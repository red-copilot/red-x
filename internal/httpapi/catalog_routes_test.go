package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/easm"
)

func TestUnifiedSurfaceCatalogRoutes(t *testing.T) {
	s := &Server{EASM: &fakeEASM{
		assets:   &easm.AssetPage{Items: []easm.AssetItem{{ID: "a1", Kind: "domain", Key: "example.com"}}, Total: 1, Page: 1, PageSize: 20},
		findings: &easm.FindingPage{Items: []easm.FindingItem{{Occurrences: 2}}, Total: 1, Page: 1, PageSize: 20},
	}}
	rec := httptest.NewRecorder()
	s.handleAssets(rec, httptest.NewRequest(http.MethodGet, "/api/v1/assets?view=surface", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"key":"example.com"`) {
		t.Fatalf("assets status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.handleFindings(rec, httptest.NewRequest(http.MethodGet, "/api/v1/findings?view=current", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"occurrences":2`) {
		t.Fatalf("findings status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUnifiedCatalogRejectsInvalidViews(t *testing.T) {
	s := &Server{EASM: &fakeEASM{}}
	for _, target := range []string{"/api/v1/assets?view=bad", "/api/v1/findings?view=bad", "/api/v1/tasks?origin=bad"} {
		rec := httptest.NewRecorder()
		switch {
		case strings.HasPrefix(target, "/api/v1/assets"):
			s.handleAssets(rec, httptest.NewRequest(http.MethodGet, target, nil))
		case strings.HasPrefix(target, "/api/v1/findings"):
			s.handleFindings(rec, httptest.NewRequest(http.MethodGet, target, nil))
		default:
			s.handleTasks(rec, httptest.NewRequest(http.MethodGet, target, nil))
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", target, rec.Code, rec.Body.String())
		}
	}
}

func TestAssetItemFromDomainIncludesManualMetadata(t *testing.T) {
	restoredAt := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	item := assetItemFromDomain(domain.Asset{
		ID: "asset-1", Kind: domain.AssetDomain, Key: "manual.example.com",
		Origin: domain.AssetOriginManual, DisplayName: "Manual Name",
		DisplayURL: "https://manual.example.com", Notes: "operations note",
		Tags: []string{"manual", "critical"}, Data: map[string]any{"title": "Scanned Title"},
		Lifecycle: domain.Lifecycle{RestoredAt: &restoredAt, RestoreReason: "scan_rediscovery"},
	})
	if item.Origin != "manual" || item.DisplayName != "Manual Name" ||
		item.DisplayURL != "https://manual.example.com" || item.Notes != "operations note" ||
		len(item.Tags) != 2 || item.RestoredAt == nil || item.RestoreReason != "scan_rediscovery" {
		t.Fatalf("manual asset metadata missing: %#v", item)
	}
	if item.Title != "Scanned Title" {
		t.Fatalf("scanned title = %q", item.Title)
	}
}
