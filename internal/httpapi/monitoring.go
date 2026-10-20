package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/httpx"
	"github.com/red-copilot/red-x/internal/store"
)

func (s *Server) handleListMonitors(w http.ResponseWriter, r *http.Request) {
	q := httpx.PageQuery(r)
	items, total, err := s.Store.ListMonitors(r.Context(), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleCreateMonitor(w http.ResponseWriter, r *http.Request) {
	var m domain.Monitor
	if !httpx.DecodeJSON(w, r, &m) {
		return
	}
	if m.Kind == "" {
		m.Kind = "domain"
	}
	if m.Name == "" {
		m.Name = "监控-" + time.Now().Format("20060102-150405")
	}
	if err := s.Scheduler.CreateMonitor(r.Context(), &m); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "monitor_create_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, m)
}

func (s *Server) handlePauseMonitor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Scheduler.PauseMonitor(r.Context(), id, true); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "monitor_pause_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id.Hex(), "status": "paused"})
}

func (s *Server) handleResumeMonitor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Scheduler.PauseMonitor(r.Context(), id, false); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "monitor_resume_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id.Hex(), "status": "active"})
}

func (s *Server) handleRunMonitor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Scheduler.RunMonitorNow(r.Context(), id); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "monitor_run_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id.Hex(), "status": "triggered"})
}

func (s *Server) handleDeleteMonitor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Scheduler.DeleteMonitor(r.Context(), id); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id.Hex()})
}

func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	q := httpx.PageQuery(r)
	items, total, err := s.Store.ListSchedules(r.Context(), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var sc domain.Schedule
	if !httpx.DecodeJSON(w, r, &sc) {
		return
	}
	if sc.Kind == "" {
		sc.Kind = "recurrent_scan"
	}
	if sc.Name == "" {
		sc.Name = "计划任务-" + time.Now().Format("20060102-150405")
	}
	if err := s.Scheduler.CreateSchedule(r.Context(), &sc); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "schedule_create_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.DeleteSchedule(r.Context(), id); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id.Hex()})
}

func (s *Server) handleListGitHubTasks(w http.ResponseWriter, r *http.Request) {
	q := httpx.PageQuery(r)
	items, total, err := s.Store.ListGitHubTasks(r.Context(), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleCreateGitHubTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Keyword string `json:"keyword"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	t, err := s.GitHub.CreateTask(r.Context(), req.Keyword, "")
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "github_task_create_failed", err.Error())
		return
	}
	go func() {
		runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_ = s.GitHub.RunTask(runCtx, t.ID)
	}()
	httpx.WriteJSON(w, http.StatusCreated, t)
}

func (s *Server) handleListGitHubMonitors(w http.ResponseWriter, r *http.Request) {
	q := httpx.PageQuery(r)
	items, total, err := s.Store.ListGitHubMonitors(r.Context(), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleCreateGitHubMonitor(w http.ResponseWriter, r *http.Request) {
	var m domain.GitHubMonitor
	if !httpx.DecodeJSON(w, r, &m) {
		return
	}
	if m.IntervalSec < 3600 {
		m.IntervalSec = 21600
	}
	if m.Status == "" {
		m.Status = "active"
	}
	if m.Name == "" {
		m.Name = "GitHub监控-" + m.Keyword
	}
	m.NextRunAt = time.Now().UTC().Add(time.Duration(m.IntervalSec) * time.Second)
	if err := s.Store.CreateGitHubMonitor(r.Context(), &m); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "github_monitor_create_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, m)
}

func (s *Server) handleDeleteGitHubMonitor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.DeleteGitHubMonitor(r.Context(), id); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id.Hex()})
}

func (s *Server) handleGitHubFindings(w http.ResponseWriter, r *http.Request) {
	q := httpx.PageQuery(r)
	items, total, err := s.Store.ListGitHubFindings(r.Context(), r.URL.Query().Get("taskId"), r.URL.Query().Get("monitorId"), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleListFingerprints(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListFingerprints(r.Context())
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateFingerprint(w http.ResponseWriter, r *http.Request) {
	var rule store.FingerprintRule
	if !httpx.DecodeJSON(w, r, &rule) {
		return
	}
	if _, err := webintelParse(rule.Rule); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_fingerprint", err.Error())
		return
	}
	if err := s.Store.CreateFingerprint(r.Context(), &rule); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "fingerprint_create_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, rule)
}

func (s *Server) handleDeleteFingerprint(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.DeleteFingerprint(r.Context(), id); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id.Hex()})
}

func (s *Server) handleImportFingerprints(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	var rules []struct {
		Name string `yaml:"name" json:"name"`
		Rule string `yaml:"rule" json:"rule"`
	}
	if err := yamlNewDecoder(r).Decode(&rules); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_import", err.Error())
		return
	}
	okCount, errCount := 0, 0
	for _, x := range rules {
		if _, err := webintelParse(x.Rule); err != nil {
			errCount++
			continue
		}
		rule := store.FingerprintRule{Name: x.Name, Rule: x.Rule}
		if err := s.Store.CreateFingerprint(r.Context(), &rule); err != nil {
			errCount++
			continue
		}
		okCount++
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"successCount": okCount, "errorCount": errCount})
}
