package easm

import (
	"context"
	"errors"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var ErrAssetNotFound = errors.New("asset not found")

type OrganizationSummary struct {
	OrganizationID   string     `json:"organizationId"`
	ConfirmedDomains int        `json:"confirmedDomains"`
	CandidateDomains int        `json:"candidateDomains"`
	DomainAssets     int        `json:"domainAssets"`
	IPAssets         int        `json:"ipAssets"`
	Services         int        `json:"services"`
	CurrentFindings  int        `json:"currentFindings"`
	CriticalFindings int        `json:"criticalFindings"`
	HighFindings     int        `json:"highFindings"`
	MediumFindings   int        `json:"mediumFindings"`
	LowFindings      int        `json:"lowFindings"`
	InfoFindings     int        `json:"infoFindings"`
	HistoryFindings  int        `json:"historyFindings"`
	UnlocatedAssets  int        `json:"unlocatedAssets"`
	LinkedTasks      int        `json:"linkedTasks"`
	LastScanAt       *time.Time `json:"lastScanAt,omitempty"`
}

type AssetFilter struct {
	Kind     string
	Query    string
	Risk     string
	Page     int
	PageSize int
	Sort     string
}

type AssetItem struct {
	ID              string         `json:"id"`
	Kind            string         `json:"kind"`
	Key             string         `json:"key"`
	Title           string         `json:"title,omitempty"`
	Risk            string         `json:"risk,omitempty"`
	FirstSeen       time.Time      `json:"firstSeen"`
	LastSeen        time.Time      `json:"lastSeen"`
	OrganizationIDs []string       `json:"organizationIds"`
	RootDomains     []string       `json:"rootDomains"`
	Data            map[string]any `json:"data,omitempty"`
}

type AssetPage struct {
	Items           []AssetItem `json:"items"`
	Total           int         `json:"total"`
	Page            int         `json:"page"`
	PageSize        int         `json:"pageSize"`
	Truncated       bool        `json:"truncated"`
	UnlocatedAssets int         `json:"unlocatedAssets"`
}

type FindingFilter struct {
	State    string
	Kind     string
	Severity string
	Query    string
	Page     int
	PageSize int
	Sort     string
}

type FindingItem struct {
	Finding         domain.Finding `json:"finding"`
	Occurrences     int            `json:"occurrences"`
	OrganizationIDs []string       `json:"organizationIds"`
	RootDomains     []string       `json:"rootDomains"`
}

type FindingPage struct {
	Items      []FindingItem `json:"items"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
	Truncated  bool          `json:"truncated"`
	Unlinked   int           `json:"unlinkedFindings"`
	ScopeError string        `json:"-"`
}

type closure struct {
	orgs          []domain.Organization
	confirmed     []domain.OrganizationDomain
	candidates    []domain.OrganizationDomain
	domains       []domain.Asset
	ips           []domain.Asset
	services      []domain.Asset
	findings      []domain.Finding
	rootByOrg     map[string][]string
	domainOrgs    map[string][]string
	ipOrgs        map[string][]string
	ipRoots       map[string][]string
	targetOrgs    map[string][]string
	targetRoots   map[string][]string
	unlocatedIPs  map[string]bool
	truncated     bool
	unlinkedCount int
}

func (s *Service) closure(ctx context.Context, orgID *bson.ObjectID) (*closure, error) {
	if s.Repo == nil {
		return nil, errors.New("EASM repository is not configured")
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
		var err error
		orgs, err = s.Repo.ListOrganizations(ctx, "", 5000)
		if err != nil {
			return nil, err
		}
	}
	active := make(map[string]bool, len(orgs))
	for _, org := range orgs {
		active[org.ID.Hex()] = true
	}
	allDomains, err := s.Repo.ListAllOrganizationDomains(ctx, "")
	if err != nil {
		return nil, err
	}
	c := &closure{
		orgs: orgs, rootByOrg: map[string][]string{}, domainOrgs: map[string][]string{},
		truncated: orgID == nil && len(orgs) >= 5000,
		ipOrgs:    map[string][]string{}, ipRoots: map[string][]string{},
		targetOrgs: map[string][]string{}, targetRoots: map[string][]string{},
		unlocatedIPs: map[string]bool{},
	}
	for _, od := range allDomains {
		oid := od.OrganizationID.Hex()
		if !active[oid] {
			continue
		}
		if od.Status == domain.DomainStatusConfirmed {
			c.confirmed = append(c.confirmed, od)
			c.rootByOrg[oid] = appendUnique(c.rootByOrg[oid], od.RootDomain)
		} else if od.Status == domain.DomainStatusCandidate {
			c.candidates = append(c.candidates, od)
		}
	}
	var roots []string
	for _, list := range c.rootByOrg {
		roots = appendUnique(roots, list...)
	}
	c.domains, err = s.Repo.ListDomainAssetsUnderRoots(ctx, roots)
	if err != nil {
		return nil, err
	}
	seenDomain := map[string]bool{}
	for _, asset := range c.domains {
		if seenDomain[asset.ID.Hex()] {
			continue
		}
		seenDomain[asset.ID.Hex()] = true
		for oid, orgRoots := range c.rootByOrg {
			for _, root := range orgRoots {
				if assetBelongsToRoot(asset.Key, root) {
					c.domainOrgs[asset.Key] = appendUnique(c.domainOrgs[asset.Key], oid)
					c.targetOrgs[asset.Key] = appendUnique(c.targetOrgs[asset.Key], oid)
					c.targetRoots[asset.Key] = appendUnique(c.targetRoots[asset.Key], root)
				}
			}
		}
		for _, ip := range stringSlice(asset.Data["ips"]) {
			for _, oid := range c.domainOrgs[asset.Key] {
				c.ipOrgs[ip] = appendUnique(c.ipOrgs[ip], oid)
			}
			for _, root := range c.targetRoots[asset.Key] {
				c.ipRoots[ip] = appendUnique(c.ipRoots[ip], root)
			}
			c.targetOrgs[ip] = appendUnique(c.targetOrgs[ip], c.ipOrgs[ip]...)
			c.targetRoots[ip] = appendUnique(c.targetRoots[ip], c.ipRoots[ip]...)
		}
	}
	ipKeys := sortedKeys(c.ipOrgs)
	c.ips, err = s.Repo.ListAssetsByKindKeys(ctx, domain.AssetIP, ipKeys)
	if err != nil {
		return nil, err
	}
	locations, err := s.Repo.ListIPLocationsByIPs(ctx, ipKeys)
	if err != nil {
		return nil, err
	}
	located := make(map[string]bool, len(locations))
	for _, loc := range locations {
		located[loc.IP] = loc.Source != "unlocated"
	}
	for _, ip := range ipKeys {
		if !located[ip] {
			c.unlocatedIPs[ip] = true
		}
	}
	c.services, err = s.Repo.ListServiceAssetsByIPs(ctx, ipKeys)
	if err != nil {
		return nil, err
	}
	var targets []string
	for _, asset := range c.domains {
		targets = appendUnique(targets, asset.Key)
	}
	targets = appendUnique(targets, ipKeys...)
	for _, svc := range c.services {
		ip := stringValue(svc.Data["ip"])
		c.targetOrgs[svc.Key] = appendUnique(c.targetOrgs[svc.Key], c.ipOrgs[ip]...)
		c.targetRoots[svc.Key] = appendUnique(c.targetRoots[svc.Key], c.ipRoots[ip]...)
		targets = appendUnique(targets, svc.Key)
	}
	c.findings, err = s.Repo.ListFindingsByTargets(ctx, targets)
	if err != nil {
		return nil, err
	}
	targetSet := make(map[string]bool, len(targets))
	for _, t := range targets {
		targetSet[t] = true
	}
	for _, f := range c.findings {
		if findingTargetKey(f.Target, targetSet) == "" {
			c.unlinkedCount++
		}
	}
	return c, nil
}

func findingTargetKey(raw string, targets map[string]bool) string {
	if targets[raw] {
		return raw
	}
	label, value, port := findingTargetNode(raw)
	if label == "IP" && targets[value] {
		return value
	}
	if label == "Domain" && targets[value] {
		return value
	}
	if label == "IP" && port != "" {
		for _, scheme := range []string{"/tcp", "/udp"} {
			key := value + ":" + port + scheme
			if targets[key] {
				return key
			}
		}
	}
	return ""
}

func (c *closure) orgsForTarget(target string) []string {
	return append([]string(nil), c.targetOrgs[target]...)
}

func (c *closure) rootsForTarget(target string) []string {
	return append([]string(nil), c.targetRoots[target]...)
}

func (s *Service) Summary(ctx context.Context, orgID bson.ObjectID) (*OrganizationSummary, error) {
	c, err := s.closure(ctx, &orgID)
	if err != nil {
		return nil, err
	}
	current, _, _ := currentFindings(c.findings, c)
	summary := &OrganizationSummary{
		OrganizationID: orgID.Hex(), ConfirmedDomains: len(c.rootByOrg[orgID.Hex()]),
		CandidateDomains: len(c.candidates), DomainAssets: len(c.domains), IPAssets: len(c.ips),
		Services: len(c.services), HistoryFindings: len(c.findings),
	}
	for _, item := range current {
		summary.CurrentFindings++
		switch strings.ToLower(item.Severity) {
		case "critical":
			summary.CriticalFindings++
		case "high":
			summary.HighFindings++
		case "medium":
			summary.MediumFindings++
		case "low":
			summary.LowFindings++
		case "info":
			summary.InfoFindings++
		}
	}
	if s.Geo == nil {
		for range c.ips {
			summary.UnlocatedAssets++
		}
	} else {
		for _, ip := range c.ips {
			addr, err := netip.ParseAddr(ip.Key)
			if err != nil {
				summary.UnlocatedAssets++
				continue
			}
			if _, err := s.Geo.Resolve(ctx, addr); err != nil {
				summary.UnlocatedAssets++
			}
		}
	}
	if lister, ok := s.Repo.(TaskSummaryRepository); ok {
		n, last, err := lister.SummaryTasks(ctx, orgID.Hex())
		if err == nil {
			summary.LinkedTasks, summary.LastScanAt = n, last
		}
	}
	return summary, nil
}

type TaskSummaryRepository interface {
	SummaryTasks(ctx context.Context, organizationID string) (int, *time.Time, error)
}

func (s *Service) Assets(ctx context.Context, orgID *bson.ObjectID, filter AssetFilter) (*AssetPage, error) {
	c, err := s.closure(ctx, orgID)
	if err != nil {
		return nil, err
	}
	risks := assetRisks(c)
	var items []AssetItem
	add := func(kind domain.AssetKind, list []domain.Asset, orgsFor func(domain.Asset) []string, rootsFor func(domain.Asset) []string) {
		for _, asset := range list {
			oidList, roots := orgsFor(asset), rootsFor(asset)
			if len(oidList) == 0 {
				continue
			}
			item := AssetItem{
				ID: asset.ID.Hex(), Kind: string(kind), Key: asset.Key,
				Title: stringValue(asset.Data["title"]), Risk: risks[asset.Key],
				FirstSeen: asset.FirstSeen, LastSeen: asset.LastSeen,
				OrganizationIDs: oidList, RootDomains: roots, Data: asset.Data,
			}
			if matchesAsset(item, filter) {
				items = append(items, item)
			}
		}
	}
	add(domain.AssetDomain, c.domains, func(a domain.Asset) []string { return c.domainOrgs[a.Key] }, func(a domain.Asset) []string { return c.targetRoots[a.Key] })
	add(domain.AssetIP, c.ips, func(a domain.Asset) []string { return c.ipOrgs[a.Key] }, func(a domain.Asset) []string { return c.ipRoots[a.Key] })
	add(domain.AssetService, c.services, func(a domain.Asset) []string {
		return c.ipOrgs[stringValue(a.Data["ip"])]
	}, func(a domain.Asset) []string {
		return c.ipRoots[stringValue(a.Data["ip"])]
	})
	sortAssets(items, filter.Sort)
	page, pageSize := normalizePage(filter.Page, filter.PageSize, len(items))
	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return &AssetPage{Items: items[start:end], Total: len(items), Page: page, PageSize: pageSize, Truncated: c.truncated, UnlocatedAssets: len(c.unlocatedIPs)}, nil
}

func (s *Service) Asset(ctx context.Context, orgID *bson.ObjectID, id bson.ObjectID) (*AssetItem, error) {
	c, err := s.closure(ctx, orgID)
	if err != nil {
		return nil, err
	}
	risks := assetRisks(c)
	for _, asset := range c.domains {
		if asset.ID == id {
			return &AssetItem{ID: asset.ID.Hex(), Kind: string(asset.Kind), Key: asset.Key, Title: stringValue(asset.Data["title"]), Risk: risks[asset.Key], FirstSeen: asset.FirstSeen, LastSeen: asset.LastSeen, OrganizationIDs: c.orgsForTarget(asset.Key), RootDomains: c.rootsForTarget(asset.Key), Data: asset.Data}, nil
		}
	}
	for _, asset := range c.ips {
		if asset.ID == id {
			return &AssetItem{ID: asset.ID.Hex(), Kind: string(asset.Kind), Key: asset.Key, Risk: risks[asset.Key], FirstSeen: asset.FirstSeen, LastSeen: asset.LastSeen, OrganizationIDs: c.orgsForTarget(asset.Key), RootDomains: c.rootsForTarget(asset.Key), Data: asset.Data}, nil
		}
	}
	for _, asset := range c.services {
		if asset.ID == id {
			return &AssetItem{ID: asset.ID.Hex(), Kind: string(asset.Kind), Key: asset.Key, Risk: risks[asset.Key], FirstSeen: asset.FirstSeen, LastSeen: asset.LastSeen, OrganizationIDs: c.orgsForTarget(asset.Key), RootDomains: c.rootsForTarget(asset.Key), Data: asset.Data}, nil
		}
	}
	return nil, ErrAssetNotFound
}

func (s *Service) Findings(ctx context.Context, orgID *bson.ObjectID, filter FindingFilter) (*FindingPage, error) {
	c, err := s.closure(ctx, orgID)
	if err != nil {
		return nil, err
	}
	current, occurrences, _ := currentFindings(c.findings, c)
	items := make([]FindingItem, 0, len(current))
	if strings.EqualFold(strings.TrimSpace(filter.State), "all") {
		for _, f := range c.findings {
			if findingTargetKey(f.Target, targetSetFromClosure(c, f)) == "" {
				continue
			}
			item := FindingItem{Finding: f, Occurrences: 1, OrganizationIDs: orgsForFinding(c, f), RootDomains: rootsForFinding(c, f)}
			if matchesFinding(item, filter) {
				items = append(items, item)
			}
		}
	} else {
		for _, f := range current {
			item := FindingItem{Finding: f, Occurrences: occurrences[findingGroupKey(f)], OrganizationIDs: orgsForFinding(c, f), RootDomains: rootsForFinding(c, f)}
			if matchesFinding(item, filter) {
				items = append(items, item)
			}
		}
	}
	sortFindings(items, filter.Sort)
	page, pageSize := normalizePage(filter.Page, filter.PageSize, len(items))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > len(items) {
		start = len(items)
	}
	if end > len(items) {
		end = len(items)
	}
	return &FindingPage{Items: items[start:end], Total: len(items), Page: page, PageSize: pageSize, Unlinked: c.unlinkedCount}, nil
}

func (s *Service) FindingHistory(ctx context.Context, orgID bson.ObjectID, findingID bson.ObjectID) ([]domain.Finding, error) {
	c, err := s.closure(ctx, &orgID)
	if err != nil {
		return nil, err
	}
	var target domain.Finding
	found := false
	for _, f := range c.findings {
		if f.ID == findingID {
			target, found = f, true
			break
		}
	}
	if !found || findingTargetKey(target.Target, targetSetFromClosure(c, target)) == "" {
		return nil, ErrAssetNotFound
	}
	var out []domain.Finding
	for _, f := range c.findings {
		if f.Target == target.Target && f.DedupeKey == target.DedupeKey {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > 200 {
		out = out[:200]
	}
	return out, nil
}

func findingGroupKey(f domain.Finding) string { return f.Target + "\x00" + f.DedupeKey }

func currentFindings(all []domain.Finding, c *closure) ([]domain.Finding, map[string]int, map[string]string) {
	latest := map[string]domain.Finding{}
	occurrences := map[string]int{}
	for _, f := range all {
		if findingTargetKey(f.Target, targetSetFromClosure(c, f)) == "" {
			continue
		}
		key := findingGroupKey(f)
		occurrences[key]++
		if cur, ok := latest[key]; !ok || f.CreatedAt.After(cur.CreatedAt) {
			latest[key] = f
		}
	}
	out := make([]domain.Finding, 0, len(latest))
	for _, f := range latest {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID.Hex() < out[j].ID.Hex()
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, occurrences, nil
}

func targetSetFromClosure(c *closure, f domain.Finding) map[string]bool {
	set := map[string]bool{}
	for _, a := range c.domains {
		set[a.Key] = true
	}
	for _, a := range c.ips {
		set[a.Key] = true
	}
	for _, a := range c.services {
		set[a.Key] = true
	}
	if findingTargetKey(f.Target, set) != "" {
		return set
	}
	label, value, port := findingTargetNode(f.Target)
	if label == "IP" && port != "" {
		set[value+":"+port+"/tcp"] = true
		set[value+":"+port+"/udp"] = true
	}
	return set
}

func orgsForFinding(c *closure, f domain.Finding) []string {
	key := findingTargetKey(f.Target, targetSetFromClosure(c, f))
	return c.orgsForTarget(key)
}

func rootsForFinding(c *closure, f domain.Finding) []string {
	key := findingTargetKey(f.Target, targetSetFromClosure(c, f))
	return c.rootsForTarget(key)
}

func assetRisks(c *closure) map[string]string {
	current, _, _ := currentFindings(c.findings, c)
	targets := map[string][]domain.Finding{}
	for _, f := range current {
		key := findingTargetKey(f.Target, targetSetFromClosure(c, f))
		if key != "" {
			targets[key] = append(targets[key], f)
		}
	}
	out := map[string]string{}
	for key, list := range targets {
		out[key] = LatestRisk(list)
	}
	for _, svc := range c.services {
		ip := stringValue(svc.Data["ip"])
		if RiskRank(out[svc.Key]) == 0 {
			out[svc.Key] = out[ip]
		} else if RiskRank(out[ip]) < RiskRank(out[svc.Key]) {
			out[ip] = out[svc.Key]
		}
	}
	return out
}

func matchesAsset(item AssetItem, f AssetFilter) bool {
	if k := strings.TrimSpace(f.Kind); k != "" && !strings.EqualFold(item.Kind, k) {
		return false
	}
	if r := strings.TrimSpace(f.Risk); r != "" && RiskRank(item.Risk) < RiskRank(r) {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(f.Query))
	return q == "" || strings.Contains(strings.ToLower(item.Key), q) || strings.Contains(strings.ToLower(item.Title), q)
}

func matchesFinding(item FindingItem, f FindingFilter) bool {
	if k := strings.TrimSpace(f.Kind); k != "" && !strings.EqualFold(string(item.Finding.Kind), k) {
		return false
	}
	if s := strings.TrimSpace(f.Severity); s != "" && !strings.EqualFold(item.Finding.Severity, s) {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(f.Query))
	return q == "" || strings.Contains(strings.ToLower(item.Finding.Title), q) ||
		strings.Contains(strings.ToLower(item.Finding.Target), q) ||
		strings.Contains(strings.ToLower(item.Finding.PluginName), q)
}

func sortAssets(items []AssetItem, order string) {
	field, desc := splitSort(order, "lastSeen")
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		var less bool
		switch field {
		case "key":
			less = a.Key < b.Key
		case "risk":
			less = RiskRank(a.Risk) < RiskRank(b.Risk)
		case "firstSeen":
			less = a.FirstSeen.Before(b.FirstSeen)
		default:
			less = a.LastSeen.Before(b.LastSeen)
		}
		if desc {
			return !less
		}
		return less
	})
}

func sortFindings(items []FindingItem, order string) {
	field, desc := splitSort(order, "createdAt")
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i].Finding, items[j].Finding
		var less bool
		switch field {
		case "severity":
			less = RiskRank(a.Severity) < RiskRank(b.Severity)
		case "title":
			less = a.Title < b.Title
		default:
			less = a.CreatedAt.Before(b.CreatedAt)
		}
		if desc {
			return !less
		}
		return less
	})
}

func splitSort(order, fallback string) (string, bool) {
	order = strings.TrimSpace(order)
	if order == "" {
		return fallback, true
	}
	field, dir, ok := strings.Cut(order, ":")
	if !ok {
		return field, true
	}
	return field, !strings.EqualFold(dir, "asc")
}

func normalizePage(page, size, total int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	_ = total
	return page, size
}
