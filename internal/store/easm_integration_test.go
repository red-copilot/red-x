package store

import (
	"context"
	"errors"
	"fmt"
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
	dbName := fmt.Sprintf("arl_test_easm_%d", time.Now().UnixNano())
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
	before, err := st.ListOrganizationDomains(ctx, org.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 {
		t.Fatalf("expected 1 row after insert, got %d", len(before))
	}
	firstSeen := before[0].FirstSeen
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

func TestConfirmRejectsSecondOwner(t *testing.T) {
	st, ctx := newEASMTestStore(t)
	orgA := &domain.Organization{CanonicalName: "单位A", Region: "蜀"}
	orgB := &domain.Organization{CanonicalName: "单位B", Region: "蜀"}
	if err := st.CreateOrganization(ctx, orgA); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateOrganization(ctx, orgB); err != nil {
		t.Fatal(err)
	}
	for _, org := range []*domain.Organization{orgA, orgB} {
		od := &domain.OrganizationDomain{
			OrganizationID: org.ID, RootDomain: "conflict.edu.cn", Source: "manual",
			Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusCandidate,
		}
		if err := st.UpsertOrganizationDomain(ctx, od); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := st.ConfirmOrganizationDomains(ctx, orgA.ID, []string{"conflict.edu.cn"}); err != nil || n != 1 {
		t.Fatalf("first confirm failed: n=%d err=%v", n, err)
	}
	if _, err := st.ConfirmOrganizationDomains(ctx, orgB.ID, []string{"conflict.edu.cn"}); !errors.Is(err, domain.ErrDomainAlreadyConfirmed) {
		t.Fatalf("expected ErrDomainAlreadyConfirmed, got %v", err)
	}
}

func TestReassignOrganizationDomainPreservesEvidence(t *testing.T) {
	st, ctx := newEASMTestStore(t)
	orgA := &domain.Organization{CanonicalName: "单位A", Region: "蜀"}
	orgB := &domain.Organization{CanonicalName: "单位B", Region: "蜀"}
	if err := st.CreateOrganization(ctx, orgA); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateOrganization(ctx, orgB); err != nil {
		t.Fatal(err)
	}
	ev := &domain.ICPEvidence{Provider: "chinaz", QueryType: "domain", QueryKey: "reassign.edu.cn", Raw: "evidence", ExpiresAt: time.Now().Add(time.Hour)}
	if err := st.AddICPEvidence(ctx, ev); err != nil {
		t.Fatal(err)
	}
	original := &domain.OrganizationDomain{
		OrganizationID: orgA.ID, RootDomain: "reassign.edu.cn", Source: "chinaz",
		Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusConfirmed,
		ICPNo: "蜀ICP备123号", EvidenceID: &ev.ID,
	}
	if err := st.UpsertOrganizationDomain(ctx, original); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListOrganizationDomains(ctx, orgA.ID, "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("list original: rows=%+v err=%v", rows, err)
	}
	moved, err := st.ReassignOrganizationDomain(ctx, rows[0].ID, orgB.ID, "alice", "归属修正")
	if err != nil {
		t.Fatal(err)
	}
	if moved.OrganizationID != orgB.ID || moved.Status != domain.DomainStatusConfirmed || moved.EvidenceID == nil || *moved.EvidenceID != ev.ID {
		t.Fatalf("unexpected moved row: %+v", moved)
	}
	oldRows, err := st.ListOrganizationDomains(ctx, orgA.ID, "")
	if err != nil || len(oldRows) != 1 {
		t.Fatalf("list old: rows=%+v err=%v", oldRows, err)
	}
	if oldRows[0].Status != domain.DomainStatusRejected || oldRows[0].ReassignedTo == nil || *oldRows[0].ReassignedTo != orgB.ID || oldRows[0].DecisionReason != "归属修正" {
		t.Fatalf("old row not retained with reassignment metadata: %+v", oldRows[0])
	}
	newRows, err := st.ListOrganizationDomains(ctx, orgB.ID, string(domain.DomainStatusConfirmed))
	if err != nil || len(newRows) != 1 || newRows[0].RootDomain != "reassign.edu.cn" {
		t.Fatalf("list new: rows=%+v err=%v", newRows, err)
	}
	count, err := st.C(ColGraphOutbox).CountDocuments(ctx, bson.M{"entityType": "organization_domain"})
	if err != nil || count < 2 {
		t.Fatalf("expected outbox events for old and new rows, count=%d err=%v", count, err)
	}
}

func TestRejectOrganizationDomainKeepsRow(t *testing.T) {
	st, ctx := newEASMTestStore(t)
	org := &domain.Organization{CanonicalName: "单位R"}
	if err := st.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	row := &domain.OrganizationDomain{OrganizationID: org.ID, RootDomain: "reject.edu.cn", Source: "manual", Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusCandidate}
	if err := st.UpsertOrganizationDomain(ctx, row); err != nil {
		t.Fatal(err)
	}
	rows, _ := st.ListOrganizationDomains(ctx, org.ID, "")
	if err := st.RejectOrganizationDomain(ctx, rows[0].ID, org.ID, "alice", "无归属证据"); err != nil {
		t.Fatal(err)
	}
	rows, _ = st.ListOrganizationDomains(ctx, org.ID, "")
	if len(rows) != 1 || rows[0].Status != domain.DomainStatusRejected || rows[0].DecisionReason != "无归属证据" {
		t.Fatalf("unexpected rejected row: %+v", rows)
	}
}
