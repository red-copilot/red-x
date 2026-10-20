package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/httpx"
)

func (s *Server) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := s.Store.ListPolicies(r.Context())
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": policies})
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
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "policy.create", Resource: p.ID,
	})
	httpx.WriteJSON(w, http.StatusCreated, p)
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
	req.CreatedAt = p.CreatedAt
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_policy", "policy name is required")
		return
	}
	if err := s.Store.UpdatePolicy(r.Context(), &req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "policy_update_failed", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "policy.update", Resource: req.ID,
	})
	httpx.WriteJSON(w, http.StatusOK, req)
}

func (s *Server) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.DeletePolicy(r.Context(), id, principalFrom(r.Context()).Username); err != nil {
		if errors.Is(err, domain.ErrRecordNotFound) {
			httpx.WriteProblem(w, r, http.StatusNotFound, "policy_not_found", "policy not found")
			return
		}
		if errors.Is(err, domain.ErrResourceInUse) {
			httpx.WriteProblem(w, r, http.StatusConflict, "policy_in_use", err.Error())
			return
		}
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "policy.delete", Resource: id,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (s *Server) handleListScopes(w http.ResponseWriter, r *http.Request) {
	q := httpx.PageQuery(r)
	items, total, err := s.Store.ListScopes(r.Context(), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize})
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
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "scope.create", Resource: sc.ID,
	})
	httpx.WriteJSON(w, http.StatusCreated, sc)
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
	httpx.WriteJSON(w, http.StatusOK, sc)
}

func (s *Server) handleUpdateScope(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	sc, err := s.Store.GetScope(r.Context(), id)
	if err != nil || sc == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "scope_not_found", "scope not found")
		return
	}
	var req struct {
		Name       *string   `json:"name"`
		Type       *string   `json:"type"`
		Targets    *[]string `json:"targets"`
		BlackScope *[]string `json:"blackScope"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Name != nil {
		sc.Name = strings.TrimSpace(*req.Name)
	}
	if req.Type != nil {
		sc.Type = strings.ToLower(strings.TrimSpace(*req.Type))
	}
	if req.Targets != nil {
		sc.Targets = *req.Targets
	}
	if req.BlackScope != nil {
		sc.BlackScope = *req.BlackScope
	}
	if sc.Name == "" || len(sc.Targets) == 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_scope", "name and targets are required")
		return
	}
	if sc.Type != "ip" {
		sc.Type = "domain"
	}
	if err := s.Store.UpdateScope(r.Context(), sc); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "scope_update_failed", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "scope.update", Resource: sc.ID,
	})
	httpx.WriteJSON(w, http.StatusOK, sc)
}

func (s *Server) handleDeleteScope(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.DeleteScope(r.Context(), id, principalFrom(r.Context()).Username); err != nil {
		if errors.Is(err, domain.ErrRecordNotFound) {
			httpx.WriteProblem(w, r, http.StatusNotFound, "scope_not_found", "scope not found")
			return
		}
		if errors.Is(err, domain.ErrResourceInUse) {
			httpx.WriteProblem(w, r, http.StatusConflict, "scope_in_use", err.Error())
			return
		}
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "scope.delete", Resource: id,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize})
}
