package httpapi

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/easm"
	"github.com/red-copilot/red-x/internal/platform/httpx"
	"github.com/red-copilot/red-x/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func queryInt(r *http.Request, key string) int {
	n, _ := strconv.Atoi(r.URL.Query().Get(key))
	return n
}

func optionalOrgID(r *http.Request, key string) (*bson.ObjectID, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, true
	}
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return nil, false
	}
	return &id, true
}

func assetFilter(r *http.Request) easm.AssetFilter {
	return easm.AssetFilter{
		Kind: r.URL.Query().Get("kind"), Query: r.URL.Query().Get("q"),
		Risk: r.URL.Query().Get("risk"), Page: queryInt(r, "page"), PageSize: queryInt(r, "pageSize"),
		Sort: r.URL.Query().Get("sort"),
	}
}

func findingFilter(r *http.Request) easm.FindingFilter {
	return easm.FindingFilter{
		State: r.URL.Query().Get("state"), Kind: r.URL.Query().Get("kind"),
		Severity: r.URL.Query().Get("severity"), Query: r.URL.Query().Get("q"),
		Page: queryInt(r, "page"), PageSize: queryInt(r, "pageSize"), Sort: r.URL.Query().Get("sort"),
	}
}

func (s *Server) handleEASMOrganizationSummary(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	out, err := s.EASM.Summary(r.Context(), id)
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (s *Server) handleEASMOrganizationAssets(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	out, err := s.EASM.Assets(r.Context(), &id, assetFilter(r))
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (s *Server) handleEASMAssets(w http.ResponseWriter, r *http.Request) {
	orgID, ok := optionalOrgID(r, "organizationId")
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_org_id", "invalid organization id")
		return
	}
	out, err := s.EASM.Assets(r.Context(), orgID, assetFilter(r))
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (s *Server) handleEASMAssetDetail(w http.ResponseWriter, r *http.Request) {
	orgID, ok := optionalOrgID(r, "organizationId")
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_org_id", "invalid organization id")
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	out, err := s.EASM.Asset(r.Context(), orgID, id)
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (s *Server) handleEASMOrganizationFindings(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	out, err := s.EASM.Findings(r.Context(), &id, findingFilter(r))
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (s *Server) handleEASMFindings(w http.ResponseWriter, r *http.Request) {
	orgID, ok := optionalOrgID(r, "organizationId")
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_org_id", "invalid organization id")
		return
	}
	out, err := s.EASM.Findings(r.Context(), orgID, findingFilter(r))
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (s *Server) handleEASMFindingHistory(w http.ResponseWriter, r *http.Request) {
	orgID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	findingID, ok := pathID(w, r, "findingId")
	if !ok {
		return
	}
	items, err := s.EASM.FindingHistory(r.Context(), orgID, findingID)
	if err != nil {
		writeEASMProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleEASMOrganizationTasks(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	q := httpx.PageQuery(r)
	items, total, err := s.Store.ListEASMTasks(r.Context(), id.Hex(), r.URL.Query().Get("status"), r.URL.Query().Get("kind"), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleEASMTasks(w http.ResponseWriter, r *http.Request) {
	orgID, ok := optionalOrgID(r, "organizationId")
	if !ok {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_org_id", "invalid organization id")
		return
	}
	q := httpx.PageQuery(r)
	filter := ""
	if orgID != nil {
		filter = orgID.Hex()
	}
	items, total, err := s.Store.ListEASMTasks(r.Context(), filter, r.URL.Query().Get("status"), r.URL.Query().Get("kind"), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleEASMOrganizationExport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Format string `json:"format"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	org, err := s.Store.GetOrganization(r.Context(), id)
	if err != nil || org == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "organization_not_found", "organization not found")
		return
	}
	domains, err := s.Store.ListOrganizationDomains(r.Context(), id, "")
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	tasks, err := s.listAllEASMTasks(r, id.Hex())
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if s.Exporter == nil {
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "export_unavailable", "report exporter is unavailable")
		return
	}
	path, err := s.Exporter.ExportOrganization(r.Context(), s.EASM, *org, domains, tasks, req.Format)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "export_failed", err.Error())
		return
	}
	name := filepath.Base(path)
	s.Store.AddAudit(r.Context(), domain.AuditEvent{Actor: principalFrom(r.Context()).Username, Action: "easm.organization_export", Resource: id.Hex(), Detail: map[string]any{"file": name, "format": req.Format}})
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"file": name, "downloadUrl": "/api/v1/exports/" + name})
}

func (s *Server) listAllEASMTasks(r *http.Request, organizationID string) ([]domain.Task, error) {
	var out []domain.Task
	for page := 1; ; page++ {
		items, total, err := s.Store.ListEASMTasks(r.Context(), organizationID, "", "", store.PageQuery{Page: page, PageSize: 200, Search: r.URL.Query().Get("q")})
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(items) == 0 || int64(page*200) >= total {
			break
		}
	}
	return out, nil
}
