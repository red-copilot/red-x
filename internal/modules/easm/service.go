package easm

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/geoip"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	ErrOrganizationNotFound = errors.New("organization not found")
	ErrGraphUnavailable     = errors.New("graph unavailable")
	ErrInvalidDepth         = errors.New("graph depth must be between 1 and 3")
)

type Repository interface {
	ListOrganizations(ctx context.Context, query string, limit int) ([]domain.Organization, error)
	GetOrganization(ctx context.Context, id bson.ObjectID) (*domain.Organization, error)
	ListAllOrganizationDomains(ctx context.Context, status string) ([]domain.OrganizationDomain, error)
	ListDomainAssetsUnderRoots(ctx context.Context, roots []string) ([]domain.Asset, error)
	ListAssetsByKindKeys(ctx context.Context, kind domain.AssetKind, keys []string) ([]domain.Asset, error)
	ListServiceAssetsByIPs(ctx context.Context, ips []string) ([]domain.Asset, error)
	ListFindingsByTargets(ctx context.Context, targets []string) ([]domain.Finding, error)
	ListIPLocationsByIPs(ctx context.Context, ips []string) ([]domain.IPLocation, error)
	UpsertIPLocation(ctx context.Context, loc *domain.IPLocation) error
}

type GraphReader interface {
	ReadOrganizationGraph(ctx context.Context, orgID string, depth, limit int) ([]GraphNode, []GraphEdge, bool, error)
}

type DomainLookupProvider interface {
	LookupByDomain(ctx context.Context, rootDomain string) (DomainICPResult, error)
}

type OrganizationLookupProvider interface {
	SearchByOrganization(ctx context.Context, name string) ([]OrgDomainRecord, error)
}

type Service struct {
	Repo        Repository
	Geo         geoip.Resolver
	GraphReader GraphReader
	Store       ICPServiceStore
	Queue       jobs.Queue
	Chinaz      DomainLookupProvider
	ICPQuery    OrganizationLookupProvider
	Log         *slog.Logger
}

type OrgBubble struct {
	OrgID      string  `json:"orgId"`
	Name       string  `json:"name"`
	AssetCount int     `json:"assetCount"`
	Risk       string  `json:"risk"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
}

type MapPoint struct {
	AssetID string   `json:"assetId"`
	Key     string   `json:"key"`
	Kind    string   `json:"kind"`
	Lat     float64  `json:"lat"`
	Lon     float64  `json:"lon"`
	Risk    string   `json:"risk"`
	CDN     bool     `json:"cdn"`
	CDNName string   `json:"cdnName,omitempty"`
	OrgIDs  []string `json:"orgIds"`
}

type MapSnapshot struct {
	Organizations          []OrgBubble `json:"organizations"`
	Points                 []MapPoint  `json:"points"`
	UnlocatedAssets        int         `json:"unlocatedAssets"`
	UnlocatedOrganizations int         `json:"unlocatedOrganizations"`
	UnlinkedFindings       int         `json:"unlinkedFindings"`
	UpdatedAt              time.Time   `json:"updatedAt"`
}

type GraphNode struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label"`
	Risk  string `json:"risk,omitempty"`
	CDN   bool   `json:"cdn,omitempty"`
}

type GraphEdge struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	Confidence string `json:"confidence,omitempty"`
}

type GraphSnapshot struct {
	Nodes      []GraphNode `json:"nodes"`
	Edges      []GraphEdge `json:"edges"`
	Truncated  bool        `json:"truncated"`
	Aggregated bool        `json:"aggregated"`
}

type Health struct {
	Chinaz   string `json:"chinaz"`
	ICPQuery string `json:"icpQuery"`
	GeoIP    string `json:"geoip"`
	Neo4j    string `json:"neo4j"`
}

func (s *Service) Health(context.Context) Health {
	h := Health{Chinaz: "unavailable", ICPQuery: "unavailable", GeoIP: "unavailable", Neo4j: "unavailable"}
	if s.Chinaz != nil {
		h.Chinaz = "ok"
	}
	if s.ICPQuery != nil {
		h.ICPQuery = "ok"
	}
	if s.Geo != nil {
		h.GeoIP = "ok"
	}
	if s.GraphReader != nil {
		h.Neo4j = "ok"
	}
	return h
}

func (s *Service) Map(ctx context.Context, orgID *bson.ObjectID, risk string) (*MapSnapshot, error) {
	if s.Repo == nil {
		return nil, errors.New("EASM repository is not configured")
	}
	domains, err := s.Repo.ListAllOrganizationDomains(ctx, string(domain.DomainStatusConfirmed))
	if err != nil {
		return nil, err
	}
	var orgs []domain.Organization
	if orgID != nil {
		org, err := s.Repo.GetOrganization(ctx, *orgID)
		if err != nil {
			return nil, err
		}
		if org == nil {
			return nil, ErrOrganizationNotFound
		}
		orgs = []domain.Organization{*org}
	} else {
		orgs, err = s.Repo.ListOrganizations(ctx, "", 200)
		if err != nil {
			return nil, err
		}
	}
	orgByID := make(map[string]domain.Organization, len(orgs))
	for _, org := range orgs {
		orgByID[org.ID.Hex()] = org
	}
	orgRoots := make(map[string][]string)
	rootToOrg := make(map[string]string)
	for _, od := range domains {
		oid := od.OrganizationID.Hex()
		if _, ok := orgByID[oid]; !ok {
			continue
		}
		orgRoots[oid] = appendUnique(orgRoots[oid], od.RootDomain)
		rootToOrg[od.RootDomain] = oid
	}
	var roots []string
	for _, list := range orgRoots {
		for _, root := range list {
			roots = appendUnique(roots, root)
		}
	}
	domainAssets, err := s.Repo.ListDomainAssetsUnderRoots(ctx, roots)
	if err != nil {
		return nil, err
	}
	domainOrgs := make(map[string][]string)
	ipOrgs := make(map[string][]string)
	ipCDN := make(map[string]bool)
	ipCDNName := make(map[string]string)
	for _, asset := range domainAssets {
		for root, oid := range rootToOrg {
			if assetBelongsToRoot(asset.Key, root) {
				domainOrgs[asset.Key] = appendUnique(domainOrgs[asset.Key], oid)
			}
		}
		for _, ip := range stringSlice(asset.Data["ips"]) {
			for _, oid := range domainOrgs[asset.Key] {
				ipOrgs[ip] = appendUnique(ipOrgs[ip], oid)
			}
			if asset.Data["cdn"] == true {
				ipCDN[ip] = true
				if name := stringValue(asset.Data["cdnName"]); name != "" {
					ipCDNName[ip] = name
				}
			}
		}
	}
	ipKeys := sortedKeys(ipOrgs)
	ipAssets, err := s.Repo.ListAssetsByKindKeys(ctx, domain.AssetIP, ipKeys)
	if err != nil {
		return nil, err
	}
	for _, asset := range ipAssets {
		if name := stringValue(asset.Data["cdnName"]); name != "" {
			ipCDN[asset.Key] = true
			ipCDNName[asset.Key] = name
		}
	}
	services, err := s.Repo.ListServiceAssetsByIPs(ctx, ipKeys)
	if err != nil {
		return nil, err
	}
	var targets []string
	for _, asset := range domainAssets {
		targets = appendUnique(targets, asset.Key)
	}
	targets = appendUnique(targets, ipKeys...)
	for _, svc := range services {
		targets = appendUnique(targets, svc.Key)
	}
	findings, err := s.Repo.ListFindingsByTargets(ctx, targets)
	if err != nil {
		return nil, err
	}
	targetSet := make(map[string]bool, len(targets))
	for _, t := range targets {
		targetSet[t] = true
	}
	findingsByKey := make(map[string][]domain.Finding)
	unlinked := 0
	for _, f := range findings {
		key := ""
		if targetSet[f.Target] {
			key = f.Target
		} else if label, value := findingTargetNode(f.Target); label == "IP" && targetSet[value] {
			key = value
		} else if label, value := findingTargetNode(f.Target); label == "Domain" && targetSet[value] {
			key = value
		}
		if key == "" {
			unlinked++
			continue
		}
		findingsByKey[key] = append(findingsByKey[key], f)
	}
	riskByKey := make(map[string]string, len(findingsByKey))
	for key, list := range findingsByKey {
		riskByKey[key] = LatestRisk(list)
	}
	for _, svc := range services {
		ip := stringValue(svc.Data["ip"])
		if rank := RiskRank(riskByKey[svc.Key]); rank == 0 {
			riskByKey[svc.Key] = riskByKey[ip]
		} else if RiskRank(riskByKey[ip]) < rank {
			riskByKey[ip] = riskByKey[svc.Key]
		}
	}
	orgRisk := make(map[string]string, len(orgs))
	raiseRisk := func(oid, key string) {
		if RiskRank(riskByKey[key]) > RiskRank(orgRisk[oid]) {
			orgRisk[oid] = riskByKey[key]
		}
	}
	for _, asset := range domainAssets {
		for _, oid := range domainOrgs[asset.Key] {
			raiseRisk(oid, asset.Key)
		}
	}
	for _, asset := range ipAssets {
		for _, oid := range ipOrgs[asset.Key] {
			raiseRisk(oid, asset.Key)
		}
	}
	for _, svc := range services {
		ip := stringValue(svc.Data["ip"])
		for _, oid := range ipOrgs[ip] {
			raiseRisk(oid, svc.Key)
		}
	}
	minRank := RiskRank(risk)
	keep := make(map[string]bool, len(orgs))
	for _, org := range orgs {
		oid := org.ID.Hex()
		if minRank == 0 || RiskRank(orgRisk[oid]) >= minRank {
			keep[oid] = true
		}
	}
	locations, err := s.Repo.ListIPLocationsByIPs(ctx, ipKeys)
	if err != nil {
		return nil, err
	}
	locationByIP := make(map[string]domain.IPLocation, len(locations))
	for _, loc := range locations {
		locationByIP[loc.IP] = loc
	}
	if s.Geo != nil {
		for _, ip := range ipKeys {
			if _, ok := locationByIP[ip]; ok {
				continue
			}
			addr, err := netip.ParseAddr(ip)
			if err != nil {
				continue
			}
			loc, err := s.Geo.Resolve(ctx, addr)
			if errors.Is(err, geoip.ErrIPUnlocated) {
				loc = &domain.IPLocation{IP: ip, Source: "unlocated", UpdatedAt: time.Now().UTC()}
				err = nil
			}
			if err == nil && loc != nil {
				_ = s.Repo.UpsertIPLocation(ctx, loc)
				locationByIP[ip] = *loc
			}
		}
	}
	snapshot := &MapSnapshot{UnlinkedFindings: unlinked, UpdatedAt: time.Now().UTC()}
	for _, asset := range ipAssets {
		orgIDs := filterStrings(ipOrgs[asset.Key], keep)
		if len(orgIDs) == 0 {
			continue
		}
		loc, ok := locationByIP[asset.Key]
		if !ok || loc.Source == "unlocated" {
			snapshot.UnlocatedAssets++
			continue
		}
		snapshot.Points = append(snapshot.Points, MapPoint{
			AssetID: asset.ID.Hex(), Key: asset.Key, Kind: "ip",
			Lat: loc.Lat, Lon: loc.Lon, Risk: riskByKey[asset.Key],
			CDN: ipCDN[asset.Key], CDNName: ipCDNName[asset.Key], OrgIDs: orgIDs,
		})
	}
	for _, org := range orgs {
		oid := org.ID.Hex()
		if !keep[oid] {
			continue
		}
		var lat, lon float64
		var n int
		for _, p := range snapshot.Points {
			if contains(p.OrgIDs, oid) {
				lat += p.Lat
				lon += p.Lon
				n++
			}
		}
		if n == 0 {
			snapshot.UnlocatedOrganizations++
			continue
		}
		assetCount := 0
		for _, asset := range domainAssets {
			if contains(domainOrgs[asset.Key], oid) {
				assetCount++
			}
		}
		for _, asset := range ipAssets {
			if contains(ipOrgs[asset.Key], oid) {
				assetCount++
			}
		}
		snapshot.Organizations = append(snapshot.Organizations, OrgBubble{
			OrgID: oid, Name: org.CanonicalName, AssetCount: assetCount,
			Risk: orgRisk[oid], Lat: lat / float64(n), Lon: lon / float64(n),
		})
	}
	return snapshot, nil
}

func (s *Service) Graph(ctx context.Context, orgID bson.ObjectID, depth, limit int) (*GraphSnapshot, error) {
	if depth <= 0 {
		depth = 2
	}
	if depth > 3 {
		return nil, ErrInvalidDepth
	}
	if limit <= 0 {
		limit = 300
	}
	if limit > 500 {
		limit = 500
	}
	if s.GraphReader == nil {
		return nil, ErrGraphUnavailable
	}
	nodes, edges, truncated, err := s.GraphReader.ReadOrganizationGraph(ctx, orgID.Hex(), depth, limit)
	if err != nil {
		return nil, err
	}
	return &GraphSnapshot{Nodes: nodes, Edges: edges, Truncated: truncated, Aggregated: truncated}, nil
}

func assetBelongsToRoot(key, root string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	root = strings.ToLower(strings.TrimSpace(root))
	return key == root || strings.HasSuffix(key, "."+root)
}

func appendUnique(list []string, values ...string) []string {
	seen := make(map[string]bool, len(list))
	for _, v := range list {
		seen[v] = true
	}
	for _, v := range values {
		if v != "" && !seen[v] {
			seen[v] = true
			list = append(list, v)
		}
	}
	return list
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

func filterStrings(list []string, keep map[string]bool) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if keep[v] {
			out = append(out, v)
		}
	}
	return out
}

func sortedKeys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
