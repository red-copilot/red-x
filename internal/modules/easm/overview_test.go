package easm

import (
	"context"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
)

func TestOverviewAggregatesCurrentRiskAndCoverage(t *testing.T) {
	repo, _, _ := catalogRepo()
	now := time.Now().UTC()
	repo.findings = []domain.Finding{
		{ID: domain.MustID(), Target: "example.com", DedupeKey: "critical", Severity: "critical", Title: "critical", CreatedAt: now},
		{ID: domain.MustID(), Target: "1.1.1.1", DedupeKey: "high", Severity: "high", Title: "high", CreatedAt: now.Add(-time.Hour)},
		{ID: domain.MustID(), Target: "example.com", DedupeKey: "old", Severity: "high", Title: "old", CreatedAt: now.Add(-60 * 24 * time.Hour)},
	}
	svc := &Service{Repo: repo}

	got, err := svc.Overview(context.Background(), now.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary.Organizations != 2 || got.Summary.ConfirmedDomains != 2 {
		t.Fatalf("unexpected coverage: %+v", got.Summary)
	}
	if got.Summary.DomainAssets != 3 || got.Summary.IPAssets != 1 || got.Summary.Services != 1 {
		t.Fatalf("unexpected asset counts: %+v", got.Summary)
	}
	if got.Summary.CurrentFindings != 2 || got.Severity.Critical != 1 || got.Severity.High != 1 {
		t.Fatalf("unexpected current risk counts: %+v severity=%+v", got.Summary, got.Severity)
	}
	if len(got.RecentFindings) != 2 {
		t.Fatalf("range filter should exclude old finding: %+v", got.RecentFindings)
	}
	if len(got.TopOrganizations) != 2 || got.TopOrganizations[0].Risk != "critical" {
		t.Fatalf("expected risk-ranked organizations: %+v", got.TopOrganizations)
	}
}

func TestSearchGroupsResultsAndHonorsLimit(t *testing.T) {
	repo, _, _ := catalogRepo()
	repo.orgs[0].CanonicalName = "Example Org"
	now := time.Now().UTC()
	repo.findings = []domain.Finding{
		{ID: domain.MustID(), Target: "example.com", DedupeKey: "v", Severity: "high", Title: "Example exposed", CreatedAt: now},
	}
	svc := &Service{Repo: repo}

	got, err := svc.Search(context.Background(), "example", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Organizations) != 1 || len(got.Domains) != 1 || len(got.Assets) != 1 || len(got.Findings) != 1 {
		t.Fatalf("unexpected grouped results: %+v", got)
	}
	if got.Domains[0].Label != "example.com" || got.Findings[0].Risk != "high" {
		t.Fatalf("unexpected search hits: domains=%+v findings=%+v", got.Domains, got.Findings)
	}
}

func TestSearchRejectsShortQuery(t *testing.T) {
	svc := &Service{Repo: catalogRepoForShortSearch()}
	if _, err := svc.Search(context.Background(), "a", 5); err == nil {
		t.Fatal("expected short query to be rejected")
	}
}

func catalogRepoForShortSearch() *fakeRepo {
	repo, _, _ := catalogRepo()
	return repo
}
