package easm

import (
	"context"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
)

func catalogRepo() (*fakeRepo, string, string) {
	repo, orgA, orgB := baseRepo()
	repo.services = []domain.Asset{
		{ID: domain.MustID(), Kind: domain.AssetService, Key: "1.1.1.1:443/tcp", Data: map[string]any{"ip": "1.1.1.1", "port": 443}},
	}
	return repo, orgA, orgB
}

func TestOrganizationSummaryCountsClosure(t *testing.T) {
	repo, orgA, _ := catalogRepo()
	now := time.Now().UTC()
	repo.findings = []domain.Finding{
		{ID: domain.MustID(), Target: "example.com", DedupeKey: "v1", Severity: "high", CreatedAt: now},
		{ID: domain.MustID(), Target: "1.1.1.1", DedupeKey: "v2", Severity: "critical", CreatedAt: now},
		{ID: domain.MustID(), Target: "1.1.1.1", DedupeKey: "v2", Severity: "low", CreatedAt: now.Add(-time.Hour)},
	}
	svc := &Service{Repo: repo}
	got, err := svc.Summary(context.Background(), orgA)
	if err != nil {
		t.Fatal(err)
	}
	if got.ConfirmedDomains != 1 || got.DomainAssets != 2 || got.IPAssets != 1 || got.Services != 1 {
		t.Fatalf("unexpected summary counts: %+v", got)
	}
	if got.CurrentFindings != 2 || got.CriticalFindings != 1 || got.HighFindings != 1 || got.HistoryFindings != 3 {
		t.Fatalf("unexpected finding counts: %+v", got)
	}
}

func TestCurrentFindingsGroupLatestAndCountHistory(t *testing.T) {
	repo, orgA, _ := catalogRepo()
	now := time.Now().UTC()
	repo.findings = []domain.Finding{
		{ID: domain.MustID(), Kind: domain.FindingNuclei, Title: "old", Target: "example.com", DedupeKey: "same", Severity: "high", CreatedAt: now.Add(-time.Hour)},
		{ID: domain.MustID(), Kind: domain.FindingNuclei, Title: "new", Target: "example.com", DedupeKey: "same", Severity: "low", CreatedAt: now},
	}
	svc := &Service{Repo: repo}
	page, err := svc.Findings(context.Background(), &orgA, FindingFilter{State: "current"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("expected one current finding, got %+v", page)
	}
	if page.Items[0].Finding.Title != "new" || page.Items[0].Occurrences != 2 {
		t.Fatalf("unexpected current finding: %+v", page.Items[0])
	}
	history, err := svc.FindingHistory(context.Background(), orgA, page.Items[0].Finding.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].Title != "new" {
		t.Fatalf("unexpected history: %+v", history)
	}
}

func TestAssetsFilterByKindRiskAndMergeSharedOrganizations(t *testing.T) {
	repo, orgA, _ := catalogRepo()
	now := time.Now().UTC()
	repo.findings = []domain.Finding{{ID: domain.MustID(), Target: "1.1.1.1", DedupeKey: "v", Severity: "high", CreatedAt: now}}
	svc := &Service{Repo: repo}
	page, err := svc.Assets(context.Background(), nil, AssetFilter{Kind: "ip", Risk: "high"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("expected one high-risk shared IP, got %+v", page)
	}
	if len(page.Items[0].OrganizationIDs) != 2 || page.Items[0].Risk != "high" {
		t.Fatalf("shared organization metadata missing: %+v", page.Items[0])
	}
	orgPage, err := svc.Assets(context.Background(), &orgA, AssetFilter{Kind: "service"})
	if err != nil {
		t.Fatal(err)
	}
	if orgPage.Total != 1 || orgPage.Items[0].Key != "1.1.1.1:443/tcp" {
		t.Fatalf("unexpected service page: %+v", orgPage)
	}
}

func TestFindingHistoryRejectsFindingOutsideOrganization(t *testing.T) {
	repo, orgA, orgB := catalogRepo()
	now := time.Now().UTC()
	other := domain.Finding{ID: domain.MustID(), Target: "outside.net", DedupeKey: "x", Severity: "high", CreatedAt: now}
	repo.findings = []domain.Finding{other}
	svc := &Service{Repo: repo}
	if _, err := svc.FindingHistory(context.Background(), orgA, other.ID); err == nil {
		t.Fatal("expected outside finding to be rejected")
	}
	_ = orgB
}
