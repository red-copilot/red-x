package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/easm"
	"github.com/red-copilot/red-x/internal/platform/httpx"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Server) handleUpdateOrganization(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		CanonicalName *string   `json:"canonicalName"`
		Aliases       *[]string `json:"aliases"`
		Region        *string   `json:"region"`
		USCC          *string   `json:"uscc"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
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
	if req.CanonicalName != nil {
		org.CanonicalName = strings.TrimSpace(*req.CanonicalName)
	}
	if req.Aliases != nil {
		org.Aliases = uniqueNonEmpty(*req.Aliases)
	}
	if req.Region != nil {
		org.Region = strings.TrimSpace(*req.Region)
	}
	if req.USCC != nil {
		org.USCC = strings.TrimSpace(*req.USCC)
	}
	if err := s.Store.UpdateOrganization(r.Context(), org); err != nil {
		httpx.WriteProblem(w, r, http.StatusConflict, "organization_update_failed", err.Error())
		return
	}
	_ = s.Store.AppendGraphOutbox(r.Context(), "organization", org.ID.Hex(), domain.OutboxOpUpsert)
	p := principalFrom(r.Context())
	s.Store.AddAudit(r.Context(), domain.AuditEvent{Actor: p.Username, Action: "easm.organization_update", Resource: org.ID.Hex(), Detail: map[string]any{"name": org.CanonicalName}})
	httpx.WriteJSON(w, http.StatusOK, org)
}

func (s *Server) handleImportOrganizations(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxOrganizationImportSize)
	if err := r.ParseMultipartForm(maxOrganizationImportSize); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_csv", "CSV upload is required")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_csv", "CSV file is required")
		return
	}
	defer file.Close()
	rows, rowErrs, err := parseOrganizationCSV(file)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_csv", err.Error())
		return
	}
	p := principalFrom(r.Context())
	result := map[string]any{"created": 0, "updated": 0, "domainsQueued": 0, "errors": rowErrs}
	for _, row := range rows {
		org, err := s.Store.FindOrganizationByNameAndRegion(r.Context(), row.CanonicalName, row.Region)
		if err != nil {
			rowErrs = append(rowErrs, organizationImportError{Row: row.Row, Message: err.Error()})
			continue
		}
		if org == nil {
			org = &domain.Organization{CanonicalName: row.CanonicalName, Aliases: row.Aliases, Region: row.Region, USCC: row.USCC, Status: domain.OrgStatusActive}
			if err := s.Store.CreateOrganization(r.Context(), org); err != nil {
				rowErrs = append(rowErrs, organizationImportError{Row: row.Row, Message: err.Error()})
				continue
			}
			result["created"] = result["created"].(int) + 1
			_ = s.Store.AppendGraphOutbox(r.Context(), "organization", org.ID.Hex(), domain.OutboxOpUpsert)
		} else {
			org.Aliases = uniqueNonEmpty(append(org.Aliases, row.Aliases...))
			if row.Region != "" {
				org.Region = row.Region
			}
			if row.USCC != "" {
				org.USCC = row.USCC
			}
			if err := s.Store.UpdateOrganization(r.Context(), org); err != nil {
				rowErrs = append(rowErrs, organizationImportError{Row: row.Row, Message: err.Error()})
				continue
			}
			result["updated"] = result["updated"].(int) + 1
			_ = s.Store.AppendGraphOutbox(r.Context(), "organization", org.ID.Hex(), domain.OutboxOpUpsert)
		}
		for _, root := range row.RootDomains {
			od := &domain.OrganizationDomain{OrganizationID: org.ID, RootDomain: root, Source: "manual", Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusCandidate}
			if err := s.Store.UpsertOrganizationDomain(r.Context(), od); err == nil {
				result["domainsQueued"] = result["domainsQueued"].(int) + 1
			} else {
				rowErrs = append(rowErrs, organizationImportError{Row: row.Row, Message: err.Error()})
			}
		}
	}
	result["errors"] = rowErrs
	s.Store.AddAudit(r.Context(), domain.AuditEvent{Actor: p.Username, Action: "easm.organization_import", Detail: result})
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (s *Server) handleRejectOrganizationDomain(w http.ResponseWriter, r *http.Request) {
	orgID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	domainID, ok := pathID(w, r, "domainId")
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	row, err := s.Store.GetOrganizationDomainByID(r.Context(), domainID)
	if err != nil || row == nil || row.OrganizationID != orgID {
		httpx.WriteProblem(w, r, http.StatusNotFound, "domain_not_found", "organization domain not found")
		return
	}
	if err := s.Store.RejectOrganizationDomain(r.Context(), domainID, orgID, principalFrom(r.Context()).Username, strings.TrimSpace(req.Reason)); err != nil {
		httpx.WriteProblem(w, r, http.StatusConflict, "domain_reject_failed", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{Actor: principalFrom(r.Context()).Username, Action: "easm.domain_reject", Resource: domainID.Hex(), Detail: map[string]any{"organizationId": orgID.Hex(), "reason": req.Reason}})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": domainID.Hex(), "status": domain.DomainStatusRejected})
}

func (s *Server) handleReassignOrganizationDomain(w http.ResponseWriter, r *http.Request) {
	orgID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	domainID, ok := pathID(w, r, "domainId")
	if !ok {
		return
	}
	var req struct {
		TargetOrganizationID string `json:"targetOrganizationId"`
		Reason               string `json:"reason"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	targetID, err := bson.ObjectIDFromHex(req.TargetOrganizationID)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_org_id", "invalid target organization id")
		return
	}
	row, err := s.Store.GetOrganizationDomainByID(r.Context(), domainID)
	if err != nil || row == nil || row.OrganizationID != orgID {
		httpx.WriteProblem(w, r, http.StatusNotFound, "domain_not_found", "organization domain not found")
		return
	}
	moved, err := s.Store.ReassignOrganizationDomain(r.Context(), domainID, targetID, principalFrom(r.Context()).Username, strings.TrimSpace(req.Reason))
	if err != nil {
		if errors.Is(err, domain.ErrDomainAlreadyConfirmed) {
			httpx.WriteProblem(w, r, http.StatusConflict, "domain_already_confirmed", "root domain already has a confirmed owner")
			return
		}
		httpx.WriteProblem(w, r, http.StatusConflict, "domain_reassign_failed", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{Actor: principalFrom(r.Context()).Username, Action: "easm.domain_reassign", Resource: domainID.Hex(), Detail: map[string]any{"fromOrganizationId": orgID.Hex(), "targetOrganizationId": targetID.Hex(), "newId": moved.ID.Hex(), "reason": req.Reason}})
	httpx.WriteJSON(w, http.StatusOK, moved)
}

func (s *Server) handleOrganizationDomainEvidence(w http.ResponseWriter, r *http.Request) {
	orgID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	domainID, ok := pathID(w, r, "domainId")
	if !ok {
		return
	}
	row, err := s.Store.GetOrganizationDomainByID(r.Context(), domainID)
	if err != nil || row == nil || row.OrganizationID != orgID {
		httpx.WriteProblem(w, r, http.StatusNotFound, "domain_not_found", "organization domain not found")
		return
	}
	p := principalFrom(r.Context())
	if p == nil || p.Role == domain.RoleViewer {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"domain": row, "evidence": nil, "redacted": true})
		return
	}
	if row.EvidenceID == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "evidence_not_found", "no ICP evidence recorded")
		return
	}
	ev, err := s.Store.GetICPEvidence(r.Context(), *row.EvidenceID)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if ev == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "evidence_not_found", "no ICP evidence recorded")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"domain": row, "evidence": ev})
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (s *Server) handleCreateEASMScans(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		RootDomains []string           `json:"rootDomains"`
		Options     domain.TaskOptions `json:"options"`
		PolicyID    string             `json:"policyId"`
		ScopeID     string             `json:"scopeId"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if len(req.RootDomains) == 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_domains", "rootDomains are required")
		return
	}
	p := principalFrom(r.Context())
	tasks, scanErrs, err := s.EASM.CreateScans(r.Context(), id, req.RootDomains, req.Options, req.PolicyID, req.ScopeID, p.Username)
	if err != nil {
		if errors.Is(err, easm.ErrDomainNotConfirmed) || errors.Is(err, easm.ErrOrganizationNotFound) {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "scan_validation_failed", err.Error())
			return
		}
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "scan_create_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"items": tasks, "errors": scanErrs, "total": len(tasks)})
}
