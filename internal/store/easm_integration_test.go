package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func newEASMTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	uri := os.Getenv("ARL_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set ARL_TEST_MONGO_URI to run MongoDB integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	dbName := "arl_test_easm_" + time.Now().Format("20060102150405.000000000")
	st, err := Open(ctx, uri, dbName)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = st.DB().Drop(context.Background())
		_ = st.Close(context.Background())
	})
	if err := st.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	return st, ctx
}

func TestOrganizationDomainUpsertIsIdempotent(t *testing.T) {
	st, ctx := newEASMTestStore(t)
	org := &domain.Organization{CanonicalName: "四川信息职业技术学院"}
	if err := st.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	first := &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: "scitc.edu.cn", SiteName: "旧名称",
		Source: "icp_query", Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusCandidate,
	}
	if err := st.UpsertOrganizationDomain(ctx, first); err != nil {
		t.Fatal(err)
	}
	firstSeen := first.FirstSeen
	time.Sleep(5 * time.Millisecond)
	second := &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: "scitc.edu.cn", SiteName: "新名称",
		Source: "icp_query", Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusCandidate,
	}
	if err := st.UpsertOrganizationDomain(ctx, second); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListOrganizationDomains(ctx, org.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].SiteName != "新名称" {
		t.Fatalf("expected updated site name, got %q", rows[0].SiteName)
	}
	if !rows[0].FirstSeen.Equal(firstSeen) {
		t.Fatalf("firstSeen changed: %v != %v", rows[0].FirstSeen, firstSeen)
	}
}

func TestConfirmOnlyCandidateDomains(t *testing.T) {
	st, ctx := newEASMTestStore(t)
	org := &domain.Organization{CanonicalName: "四川信息职业技术学院"}
	if err := st.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	for _, od := range []*domain.OrganizationDomain{
		{OrganizationID: org.ID, RootDomain: "a-scitc.edu.cn", Source: "manual", Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusCandidate},
		{OrganizationID: org.ID, RootDomain: "b-scitc.edu.cn", Source: "manual", Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusConfirmed},
	} {
		if err := st.UpsertOrganizationDomain(ctx, od); err != nil {
			t.Fatal(err)
		}
	}
	n, err := st.ConfirmOrganizationDomains(ctx, org.ID, []string{"a-scitc.edu.cn", "b-scitc.edu.cn"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 changed domain, got %d", n)
	}
	rows, err := st.ListOrganizationDomains(ctx, org.ID, string(domain.DomainStatusConfirmed))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 confirmed domains, got %d", len(rows))
	}
	count, err := st.C(ColGraphOutbox).CountDocuments(ctx, bson.M{"entityType": "organization_domain"})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 outbox event, got %d", count)
	}
}

func TestOutboxClaimIsExclusive(t *testing.T) {
	st, ctx := newEASMTestStore(t)
	if err := st.AppendGraphOutbox(ctx, "asset", "asset-1", domain.OutboxOpUpsert); err != nil {
		t.Fatal(err)
	}
	if err := st.AppendGraphOutbox(ctx, "asset", "asset-2", domain.OutboxOpUpsert); err != nil {
		t.Fatal(err)
	}
	first, err := st.ClaimGraphOutbox(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("expected 2 claimed events, got %d", len(first))
	}
	second, err := st.ClaimGraphOutbox(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("expected no reclaim, got %d", len(second))
	}
	ids := []bson.ObjectID{first[0].ID, first[1].ID}
	if err := st.MarkGraphOutboxProcessed(ctx, ids); err != nil {
		t.Fatal(err)
	}
	third, err := st.ClaimGraphOutbox(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(third) != 0 {
		t.Fatalf("expected no processed events, got %d", len(third))
	}
}

func TestUpsertDoesNotDowngradeConfirmedDomain(t *testing.T) {
	st, ctx := newEASMTestStore(t)
	org := &domain.Organization{CanonicalName: "某单位"}
	if err := st.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	confirmed := &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: "example.com", Source: "manual",
		Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusConfirmed,
	}
	if err := st.UpsertOrganizationDomain(ctx, confirmed); err != nil {
		t.Fatal(err)
	}
	incoming := &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: "example.com", Source: "icp_query",
		Confidence: domain.ConfidenceMedium, Status: domain.DomainStatusCandidate, SiteName: "刷新名称",
	}
	if err := st.UpsertOrganizationDomain(ctx, incoming); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListOrganizationDomains(ctx, org.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Status != domain.DomainStatusConfirmed || rows[0].SiteName != "刷新名称" {
		t.Fatalf("confirmed domain was downgraded or metadata lost: %+v", rows)
	}
}
