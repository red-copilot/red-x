package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/tasking"
	"github.com/red-copilot/red-x/internal/platform/httpx"
	"github.com/red-copilot/red-x/internal/store"
)

type createTaskRequest struct {
	Name        string             `json:"name"`
	Kind        domain.TaskKind    `json:"kind"`
	Target      string             `json:"target"`
	PolicyID    string             `json:"policyId"`
	ScopeID     string             `json:"scopeId"`
	ResultSetID string             `json:"resultSetId"`
	Options     domain.TaskOptions `json:"options"`
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	p := principalFrom(r.Context())
	tasks, err := s.Engine.Submit(r.Context(), tasking.SubmitInput{
		Name: req.Name, Kind: req.Kind, Target: req.Target, Options: req.Options,
		PolicyID: req.PolicyID, ScopeID: req.ScopeID, ResultSetID: req.ResultSetID, CreatedBy: p.Username,
	})
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "task_create_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"items": tasks, "total": len(tasks)})
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	q := httpx.PageQuery(r)
	tasks, total, err := s.Store.ListTasks(r.Context(), q, r.URL.Query().Get("status"), r.URL.Query().Get("kind"))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": tasks, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	task, err := s.Store.GetTask(r.Context(), id)
	if err != nil || task == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "task_not_found", "task not found")
		return
	}
	steps, _ := s.Store.ListSteps(r.Context(), id)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"task": task, "steps": steps})
}

func (s *Server) handleTaskSteps(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	steps, err := s.Store.ListSteps(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": steps})
}

func (s *Server) handleTaskObservations(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	q := httpx.PageQuery(r)
	obs, total, err := s.Store.ListTaskObservations(r.Context(), id, r.URL.Query().Get("kind"), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": obs, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleTaskFindings(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	q := httpx.PageQuery(r)
	findings, total, err := s.Store.ListFindings(r.Context(), r.URL.Query().Get("kind"), id.Hex(), "", q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": findings, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Engine.Cancel(r.Context(), id); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "task_cancel_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id.Hex(), "status": "canceled"})
}

func (s *Server) handleRetryTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	tasks, err := s.Engine.Retry(r.Context(), id, p.Username)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "task_retry_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"items": tasks})
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	task, _ := s.Store.GetTask(r.Context(), id)
	if task != nil && !task.Status.Terminal() {
		_ = s.Engine.Cancel(r.Context(), id)
	}
	if err := s.Store.DeleteTask(r.Context(), id); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id.Hex()})
}

func (s *Server) handleSyncTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		ScopeID string `json:"scopeId"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	scopeID, err := store.ObjectID(req.ScopeID)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_scope", "invalid scope id")
		return
	}
	assets, _, err := s.Store.ListAssets(r.Context(), "", id.Hex(), "", store.PageQuery{Page: 1, PageSize: 200})
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	count := 0
	for _, a := range assets {
		if err := s.Store.LinkAssetToScope(r.Context(), scopeID, &a, nil); err == nil {
			count++
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"scopeId": req.ScopeID, "assetsLinked": count})
}

func (s *Server) handleTaskEvents(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "sse_unsupported", "streaming is not supported")
		return
	}
	after, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	if q := r.URL.Query().Get("after"); q != "" {
		after, _ = strconv.ParseInt(q, 10, 64)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		events, err := s.Store.ListTaskEvents(r.Context(), id, after, 200)
		if err == nil {
			for _, e := range events {
				after = e.Sequence
				payload, _ := json.Marshal(e)
				fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.Sequence, e.Type, payload)
			}
			flusher.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		case <-heartbeat.C:
			fmt.Fprintf(w, "event: heartbeat\ndata: {\"time\":%q}\n\n", time.Now().UTC().Format(time.RFC3339))
			flusher.Flush()
		}
	}
}
