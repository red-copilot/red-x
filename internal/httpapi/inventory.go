package httpapi

import (
	"net/http"
	"strings"

	"github.com/Aabyss-Team/ARL/internal/domain"
	"github.com/Aabyss-Team/ARL/internal/platform/httpx"
)

func (s *Server) handleAssets(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(strings.ToLower(rPathParam(r, "kind")))
	switch domain.AssetKind(kind) {
	case domain.AssetDomain, domain.AssetIP, domain.AssetSite, domain.AssetURL, domain.AssetCert, domain.AssetService, domain.AssetCIDR:
	default:
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_asset_kind", "unknown asset kind")
		return
	}
	q := httpx.PageQuery(r)
	items, total, err := s.Store.ListAssets(r.Context(), kind, r.URL.Query().Get("taskId"), r.URL.Query().Get("scopeId"), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize}})
}

func (s *Server) handleFindings(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(strings.ToLower(rPathParam(r, "kind")))
	switch domain.FindingKind(kind) {
	case domain.FindingVuln, domain.FindingNuclei, domain.FindingWIH, domain.FindingFileLeak, domain.FindingProtocol:
	default:
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_finding_kind", "unknown finding kind")
		return
	}
	q := httpx.PageQuery(r)
	items, total, err := s.Store.ListFindings(r.Context(), kind, r.URL.Query().Get("taskId"), r.URL.Query().Get("scopeId"), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize}})
}

func (s *Server) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := s.Store.ListPolicies(r.Context())
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": policies}})
}

func (s *Server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	var p domain.Policy
	if !httpx.DecodeJSON(w, r, &p) {
		return
	}
	if strings.TrimSpace(p.Name) == "" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_policy", "policy name is required")
		return
	}
	if err := s.Store.CreatePolicy(r.Context(), &p); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "policy_create_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"data": p})
}

func (s *Server) handleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	p, err := s.Store.GetPolicy(r.Context(), id)
	if err != nil || p == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "policy_not_found", "policy not found")
		return
	}
	var req domain.Policy
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.ID = p.ID
	if err := s.Store.UpdatePolicy(r.Context(), &req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "policy_update_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": req})
}

func (s *Server) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.DeletePolicy(r.Context(), id); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"id": id.Hex()}})
}

func (s *Server) handleListScopes(w http.ResponseWriter, r *http.Request) {
	q := httpx.PageQuery(r)
	items, total, err := s.Store.ListScopes(r.Context(), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize}})
}

func (s *Server) handleCreateScope(w http.ResponseWriter, r *http.Request) {
	var sc domain.Scope
	if !httpx.DecodeJSON(w, r, &sc) {
		return
	}
	if strings.TrimSpace(sc.Name) == "" || len(sc.Targets) == 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_scope", "name and targets are required")
		return
	}
	if sc.Type != "ip" {
		sc.Type = "domain"
	}
	if err := s.Store.CreateScope(r.Context(), &sc); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "scope_create_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"data": sc})
}

func (s *Server) handleGetScope(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	sc, err := s.Store.GetScope(r.Context(), id)
	if err != nil || sc == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "scope_not_found", "scope not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": sc})
}

func (s *Server) handleDeleteScope(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.DeleteScope(r.Context(), id); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"id": id.Hex()}})
}

func (s *Server) handleScopeAssets(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	q := httpx.PageQuery(r)
	items, total, err := s.Store.ListScopeAssets(r.Context(), id, r.URL.Query().Get("kind"), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize}})
}
