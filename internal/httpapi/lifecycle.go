package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/httpx"
	"github.com/red-copilot/red-x/internal/store"
)

func (s *Server) handleListRecycleBin(w http.ResponseWriter, r *http.Request) {
	resourceType := strings.TrimSpace(r.URL.Query().Get("type"))
	if resourceType != "" && !store.ValidLifecycleResource(resourceType) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_resource_type", "unsupported recycle-bin resource type")
		return
	}
	q := httpx.PageQuery(r)
	items, total, err := s.Store.ListDeletedRecords(r.Context(), store.DeletedRecordFilter{
		Type: resourceType, Search: r.URL.Query().Get("q"), Page: q.Page, PageSize: q.PageSize,
	})
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize,
	})
}

func (s *Server) handleRestoreRecycleBin(w http.ResponseWriter, r *http.Request) {
	resourceType := strings.TrimSpace(chi.URLParam(r, "type"))
	if !store.ValidLifecycleResource(resourceType) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_resource_type", "unsupported recycle-bin resource type")
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.RestoreRecord(r.Context(), resourceType, id, principalFrom(r.Context()).Username); err != nil {
		if errors.Is(err, domain.ErrRecordNotFound) {
			httpx.WriteProblem(w, r, http.StatusNotFound, "record_not_deleted", "deleted record not found")
			return
		}
		httpx.WriteProblem(w, r, http.StatusConflict, "restore_conflict", err.Error())
		return
	}
	now := time.Now().UTC()
	p := principalFrom(r.Context())
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: p.Username, Action: resourceType + ".restore", Resource: id,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"type": resourceType, "id": id, "restoredAt": now})
}
