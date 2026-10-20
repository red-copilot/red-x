package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/modules/easm"
	"github.com/red-copilot/red-x/internal/platform/httpx"
	"github.com/red-copilot/red-x/internal/store"
)

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	since, ok := overviewSince(r.URL.Query().Get("range"), time.Now().UTC())
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_range", "range must be one of 24h, 7d, 30d, 90d, all")
		return
	}
	out, err := s.EASM.Overview(r.Context(), since)
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	tasks := map[string]int64{}
	if s.Store != nil {
		if counts, countErr := s.Store.CountTasksByStatus(r.Context()); countErr == nil {
			tasks = counts
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"summary": out.Summary, "severity": out.Severity,
		"topOrganizations": out.TopOrganizations, "recentFindings": out.RecentFindings,
		"tasks": tasks, "health": out.Health, "updatedAt": out.UpdatedAt, "truncated": out.Truncated,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	limit := 5
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	out, err := s.EASM.Search(r.Context(), r.URL.Query().Get("q"), limit)
	if err != nil {
		if errors.Is(err, easm.ErrSearchQueryTooShort) {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "search_query_too_short", err.Error())
			return
		}
		writeEASMProblem(w, r, err)
		return
	}
	tasks := []easm.SearchHit{}
	if s.Store != nil {
		q := httpx.PageQuery(r)
		q.Page = 1
		q.PageSize = limit
		q.Search = strings.TrimSpace(r.URL.Query().Get("q"))
		if q.Search != "" {
			items, _, listErr := s.Store.ListTasks(r.Context(), store.PageQuery{Page: 1, PageSize: limit, Search: q.Search}, "", "")
			if listErr == nil {
				for _, task := range items {
					if len(tasks) >= limit {
						break
					}
					tasks = append(tasks, easm.SearchHit{Type: "task", ID: task.ID.Hex(), Label: task.Name, Detail: task.Target})
				}
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"organizations": out.Organizations, "domains": out.Domains,
		"assets": out.Assets, "findings": out.Findings, "tasks": tasks,
	})
}

func overviewSince(raw string, now time.Time) (time.Time, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "30d":
		return now.Add(-30 * 24 * time.Hour), true
	case "24h":
		return now.Add(-24 * time.Hour), true
	case "7d":
		return now.Add(-7 * 24 * time.Hour), true
	case "90d":
		return now.Add(-90 * 24 * time.Hour), true
	case "all":
		return time.Unix(0, 0).UTC(), true
	default:
		return time.Time{}, false
	}
}
