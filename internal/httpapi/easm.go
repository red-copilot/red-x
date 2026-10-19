package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/easm"
	"github.com/red-copilot/red-x/internal/platform/httpx"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type EASMService interface {
	LookupDomain(ctx context.Context, rawDomain, actor string) (*easm.LookupResult, error)
	EnqueueRefresh(ctx context.Context, orgID bson.ObjectID, actor string) (*domain.ICPJob, error)
	GetICPJob(ctx context.Context, id bson.ObjectID) (*domain.ICPJob, error)
	ConfirmDomains(ctx context.Context, orgID bson.ObjectID, domains []string, actor string) (int, error)
	Map(ctx context.Context, orgID *bson.ObjectID, risk string) (*easm.MapSnapshot, error)
	Graph(ctx context.Context, orgID bson.ObjectID, depth, limit int) (*easm.GraphSnapshot, error)
	Health(ctx context.Context) easm.Health
}

func (s *Server) handleListOrganizations(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	items, err := s.Store.ListOrganizations(r.Context(), r.URL.Query().Get("query"), limit)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": items}})
}

func (s *Server) handleCreateOrganization(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CanonicalName string   `json:"canonicalName"`
		Aliases       []string `json:"aliases"`
		Region        string   `json:"region"`
		USCC          string   `json:"uscc"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	org := &domain.Organization{
		CanonicalName: strings.TrimSpace(req.CanonicalName), Aliases: req.Aliases,
		Region: req.Region, USCC: req.USCC, Status: domain.OrgStatusActive,
	}
	if err := s.Store.CreateOrganization(r.Context(), org); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "organization_create_failed", err.Error())
		return
	}
	p := principalFrom(r.Context())
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: p.Username, Action: "easm.organization_create", Resource: org.ID.Hex(),
		Detail: map[string]any{"name": org.CanonicalName},
	})
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"data": org})
}

func (s *Server) handleGetOrganization(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	org, err := s.Store.GetOrganization(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if org == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "organization_not_found", "organization not found")
		return
	}
	domains, err := s.Store.ListOrganizationDomains(r.Context(), id, r.URL.Query().Get("status"))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"organization": org, "domains": domains}})
}

func (s *Server) handleLookupDomain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if _, err := easm.NormalizeRootDomain(req.Domain); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_domain", "domain is invalid")
		return
	}
	p := principalFrom(r.Context())
	out, err := s.EASM.LookupDomain(r.Context(), req.Domain, p.Username)
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (s *Server) handleRefreshICP(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	job, err := s.EASM.EnqueueRefresh(r.Context(), id, p.Username)
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"data": job})
}

func (s *Server) handleGetICPJob(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	job, err := s.EASM.GetICPJob(r.Context(), id)
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	if job == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "icp_job_not_found", "ICP job not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": job})
}

func (s *Server) handleListOrgDomains(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	items, err := s.Store.ListOrganizationDomains(r.Context(), id, r.URL.Query().Get("status"))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": items}})
}

func (s *Server) handleConfirmDomains(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Domains []string `json:"domains"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if len(req.Domains) == 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_domains", "domains are required")
		return
	}
	p := principalFrom(r.Context())
	n, err := s.EASM.ConfirmDomains(r.Context(), id, req.Domains, p.Username)
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"changed": n}})
}

func (s *Server) handleImportDomains(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Domains []string `json:"domains"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if len(req.Domains) == 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_domains", "domains are required")
		return
	}
	p := principalFrom(r.Context())
	imported := 0
	for _, raw := range req.Domains {
		root, err := easm.NormalizeRootDomain(raw)
		if err != nil {
			continue
		}
		od := &domain.OrganizationDomain{
			OrganizationID: id, RootDomain: root, Source: "manual",
			Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusConfirmed,
		}
		if err := s.Store.UpsertOrganizationDomain(r.Context(), od); err != nil {
			continue
		}
		imported++
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: p.Username, Action: "easm.domain_import", Resource: id.Hex(),
		Detail: map[string]any{"domains": req.Domains, "imported": imported},
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"imported": imported}})
}

func (s *Server) handleEASMMap(w http.ResponseWriter, r *http.Request) {
	var orgID *bson.ObjectID
	if raw := r.URL.Query().Get("orgId"); raw != "" {
		id, err := bson.ObjectIDFromHex(raw)
		if err != nil {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_org_id", "invalid organization id")
			return
		}
		orgID = &id
	}
	out, err := s.EASM.Map(r.Context(), orgID, r.URL.Query().Get("risk"))
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (s *Server) handleEASMGraph(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.URL.Query().Get("orgId"))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_org_id", "orgId is required")
		return
	}
	depth, _ := strconv.Atoi(r.URL.Query().Get("depth"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := s.EASM.Graph(r.Context(), id, depth, limit)
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (s *Server) handleEASMHealth(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": s.EASM.Health(r.Context())})
}

func writeEASMProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, easm.ErrICPCookieExpired):
		httpx.WriteProblem(w, r, http.StatusFailedDependency, "icp_cookie_expired", "ICP query credential is invalid or expired")
	case errors.Is(err, easm.ErrICPRateLimited):
		httpx.WriteProblem(w, r, http.StatusTooManyRequests, "icp_rate_limited", "ICP provider rate limited the request")
	case errors.Is(err, easm.ErrICPNoAttribution):
		httpx.WriteProblem(w, r, http.StatusNotFound, "icp_not_found", "no ICP attribution found")
	case errors.Is(err, easm.ErrICPUnavailable):
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "icp_unavailable", "ICP provider is unavailable")
	case errors.Is(err, easm.ErrOrganizationNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, "organization_not_found", "organization not found")
	case errors.Is(err, easm.ErrGraphUnavailable):
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "graph_unavailable", "relationship graph is unavailable")
	case errors.Is(err, easm.ErrInvalidDepth):
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_depth", "graph depth must be between 1 and 3")
	default:
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "easm_error", err.Error())
	}
}
