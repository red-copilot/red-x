package easm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type fakeRepo struct {
	orgs         []domain.Organization
	domains      []domain.OrganizationDomain
	domainAssets []domain.Asset
	ipAssets     []domain.Asset
	services     []domain.Asset
	findings     []domain.Finding
	locations    []domain.IPLocation
}

func (f *fakeRepo) ListOrganizations(context.Context, string, int) ([]domain.Organization, error) {
	return f.orgs, nil
}
func (f *fakeRepo) GetOrganization(_ context.Context, id bson.ObjectID) (*domain.Organization, error) {
	for _, o := range f.orgs {
		if o.ID == id {
			return &o, nil
		}
	}
	return nil, nil
}
func (f *fakeRepo) ListAllOrganizationDomains(context.Context, string) ([]domain.OrganizationDomain, error) {
	return f.domains, nil
}
func (f *fakeRepo) ListDomainAssetsUnderRoots(context.Context, []string) ([]domain.Asset, error) {
	return f.domainAssets, nil
}
func (f *fakeRepo) ListAssetsByKindKeys(_ context.Context, kind domain.AssetKind, keys []string) ([]domain.Asset, error) {
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	var out []domain.Asset
	if kind == domain.AssetIP {
		for _, a := range f.ipAssets {
			if set[a.Key] {
				out = append(out, a)
			}
		}
	}
	return out, nil
}
func (f *fakeRepo) ListServiceAssetsByIPs(_ context.Context, ips []string) ([]domain.Asset, error) {
	set := make(map[string]bool, len(ips))
	for _, ip := range ips {
		set[ip] = true
	}
	var out []domain.Asset
	for _, s := range f.services {
		if set[stringValue(s.Data["ip"])] {
			out = append(out, s)
		}
	}
	return out, nil
}
func (f *fakeRepo) ListFindingsByTargets(context.Context, []string) ([]domain.Finding, error) {
	return f.findings, nil
}
func (f *fakeRepo) UpsertIPLocation(_ context.Context, loc *domain.IPLocation) error {
	for i := range f.locations {
		if f.locations[i].IP == loc.IP {
			f.locations[i] = *loc
			return nil
		}
	}
	f.locations = append(f.locations, *loc)
	return nil
}
func (f *fakeRepo) ListIPLocationsByIPs(_ context.Context, ips []string) ([]domain.IPLocation, error) {
	set := make(map[string]bool, len(ips))
	for _, ip := range ips {
		set[ip] = true
	}
	var out []domain.IPLocation
	for _, l := range f.locations {
		if set[l.IP] {
			out = append(out, l)
		}
	}
	return out, nil
}

type fakeGraphReader struct {
	depth, limit int
	nodes        []GraphNode
	edges        []GraphEdge
	truncated    bool
}

func (f *fakeGraphReader) ReadOrganizationGraph(_ context.Context, _ string, depth, limit int) ([]GraphNode, []GraphEdge, bool, error) {
	f.depth, f.limit = depth, limit
	return f.nodes, f.edges, f.truncated, nil
}

func baseRepo() (*fakeRepo, bson.ObjectID, bson.ObjectID) {
	orgA, orgB := bson.NewObjectID(), bson.NewObjectID()
	f := &fakeRepo{
		orgs: []domain.Organization{
			{ID: orgA, CanonicalName: "单位A"},
			{ID: orgB, CanonicalName: "单位B"},
		},
		domains: []domain.OrganizationDomain{
			{OrganizationID: orgA, RootDomain: "example.com", Status: domain.DomainStatusConfirmed},
			{OrganizationID: orgB, RootDomain: "example.org", Status: domain.DomainStatusConfirmed},
		},
		domainAssets: []domain.Asset{
			{ID: bson.NewObjectID(), Kind: domain.AssetDomain, Key: "example.com", Data: map[string]any{"ips": []any{"1.1.1.1"}}},
			{ID: bson.NewObjectID(), Kind: domain.AssetDomain, Key: "sub.example.com", Data: map[string]any{"ips": []any{"1.1.1.1"}}},
			{ID: bson.NewObjectID(), Kind: domain.AssetDomain, Key: "example.org", Data: map[string]any{"ips": []any{"1.1.1.1"}}},
		},
		ipAssets: []domain.Asset{
			{ID: bson.NewObjectID(), Kind: domain.AssetIP, Key: "1.1.1.1", Data: map[string]any{}},
		},
		locations: []domain.IPLocation{{IP: "1.1.1.1", Lat: 39.9, Lon: 116.4}},
	}
	return f, orgA, orgB
}

func TestClosureIncludesSubdomainsAndSharedIPs(t *testing.T) {
	repo, _, _ := baseRepo()
	s := &Service{Repo: repo}
	snap, err := s.Map(context.Background(), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Organizations) != 2 || len(snap.Points) != 1 {
		t.Fatalf("unexpected snapshot: orgs=%d points=%d", len(snap.Organizations), len(snap.Points))
	}
	if len(snap.Points[0].OrgIDs) != 2 {
		t.Fatalf("shared IP should belong to both orgs: %+v", snap.Points[0])
	}
	counts := map[string]int{}
	for _, o := range snap.Organizations {
		counts[o.Name] = o.AssetCount
	}
	if counts["单位A"] != 3 || counts["单位B"] != 2 {
		t.Fatalf("unexpected asset counts: %v", counts)
	}
}

func TestClosureMarksCDN(t *testing.T) {
	repo, _, _ := baseRepo()
	repo.domainAssets = repo.domainAssets[:1]
	repo.domainAssets[0].Data["cdn"] = true
	repo.domainAssets[0].Data["cdnName"] = "CDN"
	repo.orgs = repo.orgs[:1]
	repo.domains = repo.domains[:1]
	s := &Service{Repo: repo}
	snap, err := s.Map(context.Background(), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Points) != 1 || !snap.Points[0].CDN || snap.Points[0].CDNName != "CDN" {
		t.Fatalf("unexpected CDN point: %+v", snap.Points)
	}
}

func TestClosureExcludesUnreachableIPs(t *testing.T) {
	repo, _, _ := baseRepo()
	repo.ipAssets = append(repo.ipAssets, domain.Asset{ID: bson.NewObjectID(), Kind: domain.AssetIP, Key: "2.2.2.2"})
	repo.locations = append(repo.locations, domain.IPLocation{IP: "2.2.2.2", Lat: 1, Lon: 2})
	s := &Service{Repo: repo}
	snap, err := s.Map(context.Background(), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Points) != 1 || snap.Points[0].Key != "1.1.1.1" {
		t.Fatalf("unreachable IP leaked into map: %+v", snap.Points)
	}
}

func TestRiskUsesLatestFindingPerDedupeKey(t *testing.T) {
	repo, _, _ := baseRepo()
	now := time.Now().UTC()
	repo.findings = []domain.Finding{
		{ID: bson.NewObjectID(), Target: "example.com", DedupeKey: "a", Severity: "high", CreatedAt: now.Add(-time.Hour)},
		{ID: bson.NewObjectID(), Target: "example.com", DedupeKey: "a", Severity: "low", CreatedAt: now},
		{ID: bson.NewObjectID(), Target: "example.com", DedupeKey: "b", Severity: "critical", CreatedAt: now.Add(-2 * time.Hour)},
		{ID: bson.NewObjectID(), Target: "1.1.1.1", DedupeKey: "c", Severity: "medium", CreatedAt: now},
	}
	s := &Service{Repo: repo}
	snap, err := s.Map(context.Background(), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Organizations) != 2 {
		t.Fatalf("unexpected orgs: %+v", snap.Organizations)
	}
	for _, o := range snap.Organizations {
		if o.Name == "单位A" && o.Risk != "critical" {
			t.Fatalf("expected critical org risk, got %q", o.Risk)
		}
	}
	if snap.Points[0].Risk != "medium" {
		t.Fatalf("expected medium IP risk, got %q", snap.Points[0].Risk)
	}
}

func TestMapCountsUnlocatedAndUnlinked(t *testing.T) {
	repo, _, _ := baseRepo()
	repo.locations = nil
	repo.findings = []domain.Finding{{ID: bson.NewObjectID(), Target: "unrelated.net", DedupeKey: "x", Severity: "high", CreatedAt: time.Now()}}
	s := &Service{Repo: repo}
	snap, err := s.Map(context.Background(), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if snap.UnlocatedAssets != 1 || snap.UnlocatedOrganizations != 2 {
		t.Fatalf("unexpected unlocated counts: %+v", snap)
	}
	if snap.UnlinkedFindings != 1 {
		t.Fatalf("expected 1 unlinked finding, got %d", snap.UnlinkedFindings)
	}
}

func TestMapFilterByRisk(t *testing.T) {
	repo, _, _ := baseRepo()
	repo.findings = []domain.Finding{{ID: bson.NewObjectID(), Target: "1.1.1.1", DedupeKey: "x", Severity: "high", CreatedAt: time.Now()}}
	s := &Service{Repo: repo}
	snap, err := s.Map(context.Background(), nil, "high")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Organizations) != 2 {
		t.Fatalf("shared IP should keep both orgs: %+v", snap.Organizations)
	}
	snap, err = s.Map(context.Background(), nil, "critical")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Organizations) != 0 || len(snap.Points) != 0 {
		t.Fatalf("expected empty critical view: %+v", snap)
	}
}

func TestGraphRejectsDepthOver3(t *testing.T) {
	s := &Service{Repo: &fakeRepo{}, GraphReader: &fakeGraphReader{}}
	if _, err := s.Graph(context.Background(), bson.NewObjectID(), 4, 100); !errors.Is(err, ErrInvalidDepth) {
		t.Fatalf("expected ErrInvalidDepth, got %v", err)
	}
}

func TestGraphCapsLimitAt500(t *testing.T) {
	reader := &fakeGraphReader{}
	s := &Service{Repo: &fakeRepo{}, GraphReader: reader}
	if _, err := s.Graph(context.Background(), bson.NewObjectID(), 2, 1000); err != nil {
		t.Fatal(err)
	}
	if reader.limit != 500 || reader.depth != 2 {
		t.Fatalf("unexpected reader args: depth=%d limit=%d", reader.depth, reader.limit)
	}
}
