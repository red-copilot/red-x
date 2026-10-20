package store

import (
	"errors"
	"testing"

	"github.com/red-copilot/red-x/internal/domain"
)

func TestOrganizationDeleteIsBlockedByActiveTask(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	org := &domain.Organization{CanonicalName: "Active Task Organization"}
	if err := st.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	task := &domain.Task{
		Name: "active org task", Kind: domain.TaskDomain, Target: "example.com",
		Status: domain.TaskQueued, OrganizationID: org.ID,
	}
	if err := st.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteOrganization(ctx, org.ID, "operator"); !errors.Is(err, domain.ErrResourceInUse) {
		t.Fatalf("delete error = %v, want ErrResourceInUse", err)
	}
	active, err := st.GetOrganization(ctx, org.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active == nil {
		t.Fatal("organization was deleted despite active task")
	}

	task.Status = domain.TaskSucceeded
	if err := st.UpdateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteOrganization(ctx, org.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	active, err = st.GetOrganization(ctx, org.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active != nil {
		t.Fatal("organization remained active after delete")
	}
}

func TestDeletedOrganizationHidesOwnedDomains(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	org := &domain.Organization{CanonicalName: "Hidden Domain Organization"}
	if err := st.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	row := &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: "hidden.example.com", Source: "manual",
		Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusCandidate,
	}
	if err := st.UpsertOrganizationDomain(ctx, row); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteOrganization(ctx, org.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	items, err := st.ListOrganizationDomains(ctx, org.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("deleted organization leaked %d domains", len(items))
	}
	byID, err := st.GetOrganizationDomainByID(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if byID != nil {
		t.Fatalf("deleted organization domain lookup leaked %#v", byID)
	}
	if err := st.UpsertOrganizationDomain(ctx, &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: "new-hidden.example.com", Source: "manual",
		Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusCandidate,
	}); !errors.Is(err, domain.ErrRecordNotFound) {
		t.Fatalf("upsert under deleted organization error = %v, want ErrRecordNotFound", err)
	}
}

func TestOrganizationRestoreReenqueuesConfirmedDomains(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	org := &domain.Organization{CanonicalName: "Restored Graph Organization"}
	if err := st.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	row := &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: "restore-graph.example.com", Source: "manual",
		Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusConfirmed,
	}
	if err := st.UpsertOrganizationDomain(ctx, row); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteOrganization(ctx, org.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	if err := st.ClearGraphOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if err := st.RestoreRecord(ctx, "organization", org.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	var domainUpserts int64
	if err := st.db.Model(&domain.GraphOutbox{}).
		Where("entity_id = ? AND entity_type = ? AND op = ?", row.ID, "organization_domain", domain.OutboxOpUpsert).
		Count(&domainUpserts).Error; err != nil {
		t.Fatal(err)
	}
	if domainUpserts != 1 {
		t.Fatalf("domain upserts = %d, want 1", domainUpserts)
	}
}

func TestOrganizationDomainMetadataUpdateAndRestoreConflict(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	owner := &domain.Organization{CanonicalName: "Domain Owner"}
	other := &domain.Organization{CanonicalName: "Other Domain Owner"}
	for _, org := range []*domain.Organization{owner, other} {
		if err := st.CreateOrganization(ctx, org); err != nil {
			t.Fatal(err)
		}
	}
	row := &domain.OrganizationDomain{
		OrganizationID: owner.ID, RootDomain: "shared.example.com", Source: "manual",
		Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusConfirmed,
	}
	if err := st.UpsertOrganizationDomain(ctx, row); err != nil {
		t.Fatal(err)
	}
	updated, err := st.UpdateOrganizationDomainMetadata(ctx, owner.ID, row.ID, OrganizationDomainPatch{
		SiteName: stringPtr("Updated Site"),
		ICPNo:    stringPtr("ICP-1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.SiteName != "Updated Site" || updated.ICPNo != "ICP-1" {
		t.Fatalf("metadata was not updated: %#v", updated)
	}
	if err := st.SoftDeleteRecord(ctx, "organization_domain", row.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	replacement := &domain.OrganizationDomain{
		OrganizationID: other.ID, RootDomain: row.RootDomain, Source: "manual",
		Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusConfirmed,
	}
	if err := st.UpsertOrganizationDomain(ctx, replacement); err != nil {
		t.Fatal(err)
	}
	if err := st.RestoreRecord(ctx, "organization_domain", row.ID, "admin"); !errors.Is(err, domain.ErrDomainAlreadyConfirmed) {
		t.Fatalf("restore error = %v, want ErrDomainAlreadyConfirmed", err)
	}
}

func TestDomainReimportEnqueuesGraphUpsert(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	org := &domain.Organization{CanonicalName: "Reimport Domain Organization"}
	if err := st.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	row := &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: "reimport.example.com", Source: "manual",
		Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusConfirmed,
	}
	if err := st.UpsertOrganizationDomain(ctx, row); err != nil {
		t.Fatal(err)
	}
	if err := st.SoftDeleteRecord(ctx, "organization_domain", row.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	if err := st.ClearGraphOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	reimported := &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: row.RootDomain, Source: "manual",
		Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusConfirmed,
	}
	if err := st.UpsertOrganizationDomain(ctx, reimported); err != nil {
		t.Fatal(err)
	}
	var upserts int64
	if err := st.db.Model(&domain.GraphOutbox{}).
		Where("entity_id = ? AND entity_type = ? AND op = ?", row.ID, "organization_domain", domain.OutboxOpUpsert).
		Count(&upserts).Error; err != nil {
		t.Fatal(err)
	}
	if upserts != 1 {
		t.Fatalf("graph upserts = %d, want 1", upserts)
	}
}

func TestManualAssetMetadataAndScanRediscovery(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	asset := &domain.Asset{
		Kind: domain.AssetDomain, Key: "manual.example.com", Origin: domain.AssetOriginManual,
		DisplayName: "Manual Asset", DisplayURL: "https://manual.example.com",
		Notes: "owned by operations", Tags: []string{"critical", "manual"},
	}
	if err := st.CreateManualAsset(ctx, asset); err != nil {
		t.Fatal(err)
	}
	updated, err := st.UpdateAssetMetadata(ctx, asset.ID, AssetMetadataPatch{
		DisplayName: stringPtr("Renamed Asset"),
		Notes:       stringPtr("updated note"),
		Tags:        &[]string{"manual"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.DisplayName != "Renamed Asset" || updated.Notes != "updated note" || len(updated.Tags) != 1 {
		t.Fatalf("asset metadata was not updated: %#v", updated)
	}
	if err := st.SoftDeleteRecord(ctx, "asset", asset.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateManualAsset(ctx, &domain.Asset{
		Kind: domain.AssetDomain, Key: asset.Key, Origin: domain.AssetOriginManual,
	}); !errors.Is(err, domain.ErrRecordAlreadyExists) {
		t.Fatalf("duplicate manual create error = %v, want ErrRecordAlreadyExists", err)
	}
	rediscovered, err := st.UpsertAsset(ctx, domain.AssetDomain, asset.Key, map[string]any{"title": "Scanned Title"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if rediscovered.DeletedAt != nil || rediscovered.RestoreReason != "scan_rediscovery" {
		t.Fatalf("asset was not restored on rediscovery: %#v", rediscovered.Lifecycle)
	}
	if rediscovered.Origin != domain.AssetOriginManual || rediscovered.DisplayName != "Renamed Asset" ||
		rediscovered.Notes != "updated note" || len(rediscovered.Tags) != 1 {
		t.Fatalf("manual metadata was not preserved: %#v", rediscovered)
	}
	if rediscovered.Data["title"] != "Scanned Title" {
		t.Fatalf("scan data was not updated: %#v", rediscovered.Data)
	}
	var deleteEvents int64
	if err := st.db.Model(&domain.GraphOutbox{}).
		Where("entity_id = ? AND op = ?", asset.ID, domain.OutboxOpDelete).
		Count(&deleteEvents).Error; err != nil {
		t.Fatal(err)
	}
	if deleteEvents == 0 {
		t.Fatal("asset deletion did not enqueue graph removal")
	}
}

func TestListAssetsSearchesManualMetadata(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	asset := &domain.Asset{
		Kind: domain.AssetIP, Key: "203.0.113.10", Origin: domain.AssetOriginManual,
		DisplayName: "Operations Gateway", DisplayURL: "https://gateway.example.com",
		Notes: "maintained by network team", Tags: []string{"production", "edge"},
	}
	if err := st.CreateManualAsset(ctx, asset); err != nil {
		t.Fatal(err)
	}
	for _, search := range []string{"Operations", "network team", "production"} {
		items, total, err := st.ListAssets(ctx, "", "", "", PageQuery{Page: 1, PageSize: 20, Search: search})
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 || len(items) != 1 || items[0].ID != asset.ID {
			t.Fatalf("search %q returned total=%d items=%#v", search, total, items)
		}
	}
}

func stringPtr(value string) *string {
	return &value
}
