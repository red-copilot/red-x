package easm

import (
	"context"
	"testing"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type fakeProjectionStore struct {
	events   []domain.GraphOutbox
	marked   []bson.ObjectID
	orgs     map[bson.ObjectID]*domain.Organization
	ods      map[bson.ObjectID]*domain.OrganizationDomain
	assets   map[bson.ObjectID]*domain.Asset
	findings map[bson.ObjectID]*domain.Finding
	cleared  bool
}

func (f *fakeProjectionStore) ClaimGraphOutbox(context.Context, int) ([]domain.GraphOutbox, error) {
	out := f.events
	f.events = nil
	return out, nil
}
func (f *fakeProjectionStore) MarkGraphOutboxProcessed(_ context.Context, ids []bson.ObjectID) error {
	f.marked = append(f.marked, ids...)
	return nil
}
func (f *fakeProjectionStore) GetAssetByID(_ context.Context, id bson.ObjectID) (*domain.Asset, error) {
	return f.assets[id], nil
}
func (f *fakeProjectionStore) GetFindingByID(_ context.Context, id bson.ObjectID) (*domain.Finding, error) {
	return f.findings[id], nil
}
func (f *fakeProjectionStore) GetOrganizationDomainByID(_ context.Context, id bson.ObjectID) (*domain.OrganizationDomain, error) {
	return f.ods[id], nil
}
func (f *fakeProjectionStore) GetOrganization(_ context.Context, id bson.ObjectID) (*domain.Organization, error) {
	return f.orgs[id], nil
}
func (f *fakeProjectionStore) ListAllOrganizations(context.Context) ([]domain.Organization, error) {
	var out []domain.Organization
	for _, o := range f.orgs {
		out = append(out, *o)
	}
	return out, nil
}
func (f *fakeProjectionStore) ListAllOrganizationDomains(context.Context, string) ([]domain.OrganizationDomain, error) {
	var out []domain.OrganizationDomain
	for _, od := range f.ods {
		out = append(out, *od)
	}
	return out, nil
}
func (f *fakeProjectionStore) ListAssets(_ context.Context, _, _, _ string, q store.PageQuery) ([]domain.Asset, int64, error) {
	all := make([]domain.Asset, 0, len(f.assets))
	for _, a := range f.assets {
		all = append(all, *a)
	}
	return page(all, q)
}
func (f *fakeProjectionStore) ListFindings(_ context.Context, _, _, _ string, q store.PageQuery) ([]domain.Finding, int64, error) {
	all := make([]domain.Finding, 0, len(f.findings))
	for _, x := range f.findings {
		all = append(all, *x)
	}
	return page(all, q)
}
func (f *fakeProjectionStore) ClearGraphOutbox(context.Context) error { f.cleared = true; return nil }

func page[T any](all []T, q store.PageQuery) ([]T, int64, error) {
	q = q.Normalized()
	start := (q.Page - 1) * q.PageSize
	if start >= len(all) {
		return nil, int64(len(all)), nil
	}
	end := start + q.PageSize
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], int64(len(all)), nil
}

type fakeGraphWriter struct {
	calls   []string
	reset   bool
	assets  []domain.Asset
	finders []domain.Finding
}

func (f *fakeGraphWriter) MergeOrganization(context.Context, domain.Organization) error {
	f.calls = append(f.calls, "merge:organization")
	return nil
}
func (f *fakeGraphWriter) MergeOrganizationDomain(context.Context, domain.OrganizationDomain) error {
	f.calls = append(f.calls, "merge:organization_domain")
	return nil
}
func (f *fakeGraphWriter) MergeAsset(_ context.Context, a domain.Asset) error {
	f.calls = append(f.calls, "merge:"+a.Key)
	f.assets = append(f.assets, a)
	return nil
}
func (f *fakeGraphWriter) MergeFinding(_ context.Context, x domain.Finding) error {
	f.calls = append(f.calls, "merge:finding:"+x.ID.Hex())
	f.finders = append(f.finders, x)
	return nil
}
func (f *fakeGraphWriter) DeleteEntityEdges(_ context.Context, entityType, entityID string) error {
	f.calls = append(f.calls, "delete:"+entityType+":"+entityID)
	return nil
}
func (f *fakeGraphWriter) Reset(context.Context) error { f.reset = true; return nil }

func TestProcessBatchMergesAssetAndFinding(t *testing.T) {
	assetID, findingID := bson.NewObjectID(), bson.NewObjectID()
	st := &fakeProjectionStore{
		events: []domain.GraphOutbox{
			{ID: bson.NewObjectID(), EntityType: "asset", EntityID: assetID.Hex(), Op: domain.OutboxOpUpsert},
			{ID: bson.NewObjectID(), EntityType: "finding", EntityID: findingID.Hex(), Op: domain.OutboxOpUpsert},
		},
		assets:   map[bson.ObjectID]*domain.Asset{assetID: {ID: assetID, Kind: domain.AssetDomain, Key: "example.com"}},
		findings: map[bson.ObjectID]*domain.Finding{findingID: {ID: findingID, Title: "test"}},
		orgs:     map[bson.ObjectID]*domain.Organization{},
		ods:      map[bson.ObjectID]*domain.OrganizationDomain{},
	}
	w := &fakeGraphWriter{}
	p := NewProjector(st, w, nil)
	n, err := p.ProcessBatch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(w.assets) != 1 || len(w.finders) != 1 {
		t.Fatalf("unexpected projection: n=%d assets=%d findings=%d", n, len(w.assets), len(w.finders))
	}
	if len(st.marked) != 2 {
		t.Fatalf("expected 2 marked events, got %d", len(st.marked))
	}
}

func TestProcessBatchDeletesStaleEdgesBeforeMerge(t *testing.T) {
	assetID := bson.NewObjectID()
	st := &fakeProjectionStore{
		events:   []domain.GraphOutbox{{ID: bson.NewObjectID(), EntityType: "asset", EntityID: assetID.Hex(), Op: domain.OutboxOpUpsert}},
		assets:   map[bson.ObjectID]*domain.Asset{assetID: {ID: assetID, Kind: domain.AssetDomain, Key: "example.com"}},
		findings: map[bson.ObjectID]*domain.Finding{},
		orgs:     map[bson.ObjectID]*domain.Organization{},
		ods:      map[bson.ObjectID]*domain.OrganizationDomain{},
	}
	w := &fakeGraphWriter{}
	if _, err := NewProjector(st, w, nil).ProcessBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(w.calls) < 2 || w.calls[0] != "delete:asset:"+assetID.Hex() || w.calls[1] != "merge:example.com" {
		t.Fatalf("unexpected call order: %v", w.calls)
	}
}

func TestProcessBatchDeleteEventOnlyDeletes(t *testing.T) {
	st := &fakeProjectionStore{
		events:   []domain.GraphOutbox{{ID: bson.NewObjectID(), EntityType: "asset", EntityID: bson.NewObjectID().Hex(), Op: domain.OutboxOpDelete}},
		assets:   map[bson.ObjectID]*domain.Asset{},
		findings: map[bson.ObjectID]*domain.Finding{},
		orgs:     map[bson.ObjectID]*domain.Organization{},
		ods:      map[bson.ObjectID]*domain.OrganizationDomain{},
	}
	w := &fakeGraphWriter{}
	if _, err := NewProjector(st, w, nil).ProcessBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(w.assets) != 0 || len(w.calls) != 1 {
		t.Fatalf("delete event should not merge: %v", w.calls)
	}
}

func TestRebuildClearsAndReplaysAllEntities(t *testing.T) {
	orgID, odID, assetID, findingID := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	st := &fakeProjectionStore{
		orgs:     map[bson.ObjectID]*domain.Organization{orgID: {ID: orgID, CanonicalName: "某单位"}},
		ods:      map[bson.ObjectID]*domain.OrganizationDomain{odID: {ID: odID, OrganizationID: orgID, RootDomain: "example.com", Status: domain.DomainStatusConfirmed}},
		assets:   map[bson.ObjectID]*domain.Asset{assetID: {ID: assetID, Kind: domain.AssetDomain, Key: "example.com"}},
		findings: map[bson.ObjectID]*domain.Finding{findingID: {ID: findingID, Title: "test"}},
	}
	w := &fakeGraphWriter{}
	if err := NewProjector(st, w, nil).Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !w.reset || !st.cleared {
		t.Fatalf("rebuild did not reset/clear: reset=%v cleared=%v", w.reset, st.cleared)
	}
	if len(w.calls) < 4 {
		t.Fatalf("expected replay calls, got %v", w.calls)
	}
}
