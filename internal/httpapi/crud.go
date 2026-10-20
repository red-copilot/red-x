package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/httpx"
	"github.com/red-copilot/red-x/internal/store"
)

func (s *Server) handleDeleteOrganization(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.DeleteOrganization(r.Context(), id, principalFrom(r.Context()).Username); err != nil {
		switch {
		case errors.Is(err, domain.ErrRecordNotFound):
			httpx.WriteProblem(w, r, http.StatusNotFound, "organization_not_found", "organization not found")
		case errors.Is(err, domain.ErrResourceInUse):
			httpx.WriteProblem(w, r, http.StatusConflict, "organization_in_use", err.Error())
		default:
			httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		}
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "organization.delete", Resource: id,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "deleted": true})
}

func (s *Server) handleUpdateOrganizationDomain(w http.ResponseWriter, r *http.Request) {
	orgID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	domainID, ok := pathID(w, r, "domainId")
	if !ok {
		return
	}
	var req struct {
		SiteName  *string `json:"siteName"`
		HomeSite  *string `json:"homeSite"`
		ICPNo     *string `json:"icpNo"`
		AuditDate *string `json:"auditDate"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	row, err := s.Store.UpdateOrganizationDomainMetadata(r.Context(), orgID, domainID, store.OrganizationDomainPatch{
		SiteName: req.SiteName, HomeSite: req.HomeSite, ICPNo: req.ICPNo, AuditDate: req.AuditDate,
	})
	if err != nil {
		if errors.Is(err, domain.ErrRecordNotFound) {
			httpx.WriteProblem(w, r, http.StatusNotFound, "domain_not_found", "organization domain not found")
			return
		}
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "organization_domain.update", Resource: domainID,
		Detail: map[string]any{"organizationId": orgID},
	})
	httpx.WriteJSON(w, http.StatusOK, row)
}

func (s *Server) handleDeleteOrganizationDomain(w http.ResponseWriter, r *http.Request) {
	orgID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	domainID, ok := pathID(w, r, "domainId")
	if !ok {
		return
	}
	row, err := s.Store.GetOrganizationDomainByID(r.Context(), domainID)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if row == nil || row.OrganizationID != orgID {
		httpx.WriteProblem(w, r, http.StatusNotFound, "domain_not_found", "organization domain not found")
		return
	}
	if err := s.Store.SoftDeleteRecord(r.Context(), "organization_domain", domainID, principalFrom(r.Context()).Username); err != nil {
		if errors.Is(err, domain.ErrRecordNotFound) {
			httpx.WriteProblem(w, r, http.StatusNotFound, "domain_not_found", "organization domain not found")
			return
		}
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "organization_domain.delete", Resource: domainID,
		Detail: map[string]any{"organizationId": orgID},
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": domainID, "deleted": true})
}

func (s *Server) handleCreateAsset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind        domain.AssetKind `json:"kind"`
		Key         string           `json:"key"`
		DisplayName string           `json:"displayName"`
		DisplayURL  string           `json:"displayUrl"`
		Notes       string           `json:"notes"`
		Tags        []string         `json:"tags"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if !validAssetKind(req.Kind) || strings.TrimSpace(req.Key) == "" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_asset", "asset kind and key are required")
		return
	}
	asset := &domain.Asset{
		Kind: req.Kind, Key: strings.TrimSpace(req.Key), Origin: domain.AssetOriginManual,
		DisplayName: strings.TrimSpace(req.DisplayName), DisplayURL: strings.TrimSpace(req.DisplayURL),
		Notes: strings.TrimSpace(req.Notes), Tags: req.Tags,
	}
	if err := s.Store.CreateManualAsset(r.Context(), asset); err != nil {
		if errors.Is(err, domain.ErrRecordAlreadyExists) {
			httpx.WriteProblem(w, r, http.StatusConflict, "asset_already_exists", "asset already exists; restore the deleted record instead")
			return
		}
		httpx.WriteProblem(w, r, http.StatusBadRequest, "asset_create_failed", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "asset.create", Resource: asset.ID,
		Detail: map[string]any{"kind": asset.Kind, "key": asset.Key},
	})
	httpx.WriteJSON(w, http.StatusCreated, asset)
}

func (s *Server) handleUpdateAsset(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		DisplayName *string   `json:"displayName"`
		DisplayURL  *string   `json:"displayUrl"`
		Notes       *string   `json:"notes"`
		Tags        *[]string `json:"tags"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	asset, err := s.Store.UpdateAssetMetadata(r.Context(), id, store.AssetMetadataPatch{
		DisplayName: req.DisplayName, DisplayURL: req.DisplayURL, Notes: req.Notes, Tags: req.Tags,
	})
	if err != nil {
		if errors.Is(err, domain.ErrRecordNotFound) {
			httpx.WriteProblem(w, r, http.StatusNotFound, "asset_not_found", "asset not found")
			return
		}
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "asset.update", Resource: id,
	})
	httpx.WriteJSON(w, http.StatusOK, asset)
}

func (s *Server) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.SoftDeleteRecord(r.Context(), "asset", id, principalFrom(r.Context()).Username); err != nil {
		if errors.Is(err, domain.ErrRecordNotFound) {
			httpx.WriteProblem(w, r, http.StatusNotFound, "asset_not_found", "asset not found")
			return
		}
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "asset.delete", Resource: id,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "deleted": true})
}

func validAssetKind(kind domain.AssetKind) bool {
	switch kind {
	case domain.AssetDomain, domain.AssetIP, domain.AssetSite, domain.AssetURL, domain.AssetCert, domain.AssetService, domain.AssetCIDR:
		return true
	default:
		return false
	}
}
