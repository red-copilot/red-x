package httpapi

import (
	"net/http"
	"strings"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/easm"
	"github.com/red-copilot/red-x/internal/platform/httpx"
)

func (s *Server) handleAssets(w http.ResponseWriter, r *http.Request) {
	view := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("view")))
	if view == "" {
		view = "surface"
	}
	orgID, ok := optionalOrgID(r, "organizationId")
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_org_id", "invalid organization id")
		return
	}
	switch view {
	case "surface":
		out, err := s.EASM.Assets(r.Context(), orgID, assetFilter(r))
		if err != nil {
			writeEASMProblem(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, out)
	case "inventory":
		if s.Store == nil {
			httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "store_unavailable", "inventory is unavailable")
			return
		}
		q := httpx.PageQuery(r)
		items, total, err := s.Store.ListAssets(r.Context(), r.URL.Query().Get("kind"), "", "", q)
		if err != nil {
			httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		page := make([]easm.AssetItem, 0, len(items))
		for _, item := range items {
			page = append(page, assetItemFromDomain(item))
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": page, "total": total, "page": q.Page, "pageSize": q.PageSize, "truncated": false, "unlocatedAssets": 0})
	default:
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_view", "view must be surface or inventory")
	}
}

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	view := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("view")))
	if view == "" {
		view = "surface"
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	orgID, ok := optionalOrgID(r, "organizationId")
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_org_id", "invalid organization id")
		return
	}
	switch view {
	case "surface":
		out, err := s.EASM.Asset(r.Context(), orgID, id)
		if err != nil {
			writeEASMProblem(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, out)
	case "inventory":
		if s.Store == nil {
			httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "store_unavailable", "inventory is unavailable")
			return
		}
		item, err := s.Store.GetAssetByID(r.Context(), id)
		if err != nil {
			httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		if item == nil {
			httpx.WriteProblem(w, r, http.StatusNotFound, "asset_not_found", "asset not found")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, assetItemFromDomain(*item))
	default:
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_view", "view must be surface or inventory")
	}
}

func (s *Server) handleFindings(w http.ResponseWriter, r *http.Request) {
	view := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("view")))
	if view == "" {
		view = "current"
	}
	orgID, ok := optionalOrgID(r, "organizationId")
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_org_id", "invalid organization id")
		return
	}
	switch view {
	case "current", "history":
		filter := findingFilter(r)
		if view == "history" {
			filter.State = "all"
		}
		out, err := s.EASM.Findings(r.Context(), orgID, filter)
		if err != nil {
			writeEASMProblem(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, out)
	case "raw":
		if s.Store == nil {
			httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "store_unavailable", "raw findings are unavailable")
			return
		}
		q := httpx.PageQuery(r)
		items, total, err := s.Store.ListFindings(r.Context(), r.URL.Query().Get("kind"), "", "", q)
		if err != nil {
			httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		page := make([]easm.FindingItem, 0, len(items))
		for _, item := range items {
			page = append(page, easm.FindingItem{Finding: item, Occurrences: 1})
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": page, "total": total, "page": q.Page, "pageSize": q.PageSize, "truncated": false, "unlinkedFindings": 0})
	default:
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_view", "view must be current, history, or raw")
	}
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	origin := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("origin")))
	if origin == "" {
		origin = "surface"
	}
	if origin != "surface" && origin != "all" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_origin", "origin must be surface or all")
		return
	}
	if s.Store == nil {
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "store_unavailable", "tasks are unavailable")
		return
	}
	q := httpx.PageQuery(r)
	status, kind := r.URL.Query().Get("status"), r.URL.Query().Get("kind")
	orgID, ok := optionalOrgID(r, "organizationId")
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_org_id", "invalid organization id")
		return
	}
	var (
		items []domain.Task
		total int64
		err   error
	)
	if origin == "surface" {
		filter := ""
		if orgID != nil {
			filter = *orgID
		}
		items, total, err = s.Store.ListEASMTasks(r.Context(), filter, status, kind, q)
	} else {
		items, total, err = s.Store.ListTasks(r.Context(), q, status, kind)
	}
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleFindingHistory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	orgID, ok := optionalOrgID(r, "organizationId")
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_org_id", "invalid organization id")
		return
	}
	if orgID == nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "organization_required", "organizationId is required for finding history")
		return
	}
	items, err := s.EASM.FindingHistory(r.Context(), *orgID, id)
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func assetItemFromDomain(item domain.Asset) easm.AssetItem {
	return easm.AssetItem{
		ID: item.ID, Kind: string(item.Kind), Key: item.Key,
		Title: stringValue(item.Data["title"]), FirstSeen: item.FirstSeen, LastSeen: item.LastSeen,
		Data: item.Data,
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
