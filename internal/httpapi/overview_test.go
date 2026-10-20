package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/modules/easm"
)

func (f *fakeEASM) Overview(context.Context, time.Time) (*easm.Overview, error) {
	return f.overview, f.overviewErr
}

func (f *fakeEASM) Search(context.Context, string, int) (*easm.SearchResult, error) {
	return f.search, f.searchErr
}

func TestOverviewReturnsDirectReadModel(t *testing.T) {
	s := &Server{EASM: &fakeEASM{overview: &easm.Overview{
		Summary:   easm.OverviewSummary{Organizations: 2, CurrentFindings: 1},
		Severity:  easm.SeverityCounts{Critical: 1},
		UpdatedAt: time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC),
	}}}
	rec := httptest.NewRecorder()
	s.handleOverview(rec, httptest.NewRequest(http.MethodGet, "/api/v1/overview?range=7d", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"data"`) {
		t.Fatalf("overview must not use legacy data envelope: %s", rec.Body.String())
	}
	var body struct {
		Summary easm.OverviewSummary `json:"summary"`
		Tasks   map[string]int64     `json:"tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.Organizations != 2 || body.Tasks == nil {
		t.Fatalf("unexpected overview: %+v", body)
	}
}

func TestOverviewRejectsInvalidRange(t *testing.T) {
	s := &Server{EASM: &fakeEASM{}}
	rec := httptest.NewRecorder()
	s.handleOverview(rec, httptest.NewRequest(http.MethodGet, "/api/v1/overview?range=forever", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSearchReturnsGroupedHitsAndRejectsShortQuery(t *testing.T) {
	s := &Server{EASM: &fakeEASM{search: &easm.SearchResult{
		Organizations: []easm.SearchHit{{Type: "organization", ID: "o1", Label: "单位A"}},
		Domains:       []easm.SearchHit{{Type: "domain", ID: "d1", Label: "example.com"}},
	}}}
	rec := httptest.NewRecorder()
	s.handleSearch(rec, httptest.NewRequest(http.MethodGet, "/api/v1/search?q=example", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"domains"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	s.EASM = &fakeEASM{searchErr: easm.ErrSearchQueryTooShort}
	rec = httptest.NewRecorder()
	s.handleSearch(rec, httptest.NewRequest(http.MethodGet, "/api/v1/search?q=a", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOverviewSinceAllUsesPastInsteadOfZero(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	got, ok := overviewSince("all", now)
	if !ok {
		t.Fatal("all range should be accepted")
	}
	if got.IsZero() || !got.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("all range should use a far-past sentinel, got %s", got)
	}
}
