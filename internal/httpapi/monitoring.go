package httpapi

import (
	"context"
	"net/http"
	"strings"
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

func (s *Server) handleGetMonitor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	monitor, err := s.Store.GetMonitor(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if monitor == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "monitor_not_found", "monitor not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, monitor)
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
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "monitor.create", Resource: m.ID,
	})
	httpx.WriteJSON(w, http.StatusCreated, m)
}

func (s *Server) handleUpdateMonitor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	monitor, err := s.Store.GetMonitor(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if monitor == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "monitor_not_found", "monitor not found")
		return
	}
	var req struct {
		Name        *string             `json:"name"`
		Kind        *string             `json:"kind"`
		Target      *string             `json:"target"`
		IntervalSec *int                `json:"intervalSec"`
		PolicyID    *string             `json:"policyId"`
		ScopeID     *string             `json:"scopeId"`
		Options     *domain.TaskOptions `json:"options"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Name != nil {
		monitor.Name = *req.Name
	}
	if req.Kind != nil {
		monitor.Kind = *req.Kind
	}
	if req.Target != nil {
		monitor.Target = *req.Target
	}
	if req.IntervalSec != nil {
		monitor.IntervalSec = *req.IntervalSec
	}
	if req.PolicyID != nil {
		monitor.PolicyID = *req.PolicyID
	}
	if req.ScopeID != nil {
		monitor.ScopeID = *req.ScopeID
	}
	if req.Options != nil {
		monitor.Options = *req.Options
	}
	if err := s.Scheduler.UpdateMonitor(r.Context(), monitor); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "monitor_update_failed", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "monitor.update", Resource: monitor.ID,
	})
	httpx.WriteJSON(w, http.StatusOK, monitor)
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "status": "paused"})
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "status": "active"})
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "status": "triggered"})
}

func (s *Server) handleDeleteMonitor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Scheduler.DeleteMonitor(r.Context(), id, principalFrom(r.Context()).Username); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "monitor.delete", Resource: id,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
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

func (s *Server) handleGetSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	schedule, err := s.Store.GetSchedule(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if schedule == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "schedule_not_found", "schedule not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, schedule)
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
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "schedule.create", Resource: sc.ID,
	})
	httpx.WriteJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	schedule, err := s.Store.GetSchedule(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if schedule == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "schedule_not_found", "schedule not found")
		return
	}
	var req struct {
		Name     *string          `json:"name"`
		Kind     *string          `json:"kind"`
		TaskKind *domain.TaskKind `json:"taskKind"`
		Target   *string          `json:"target"`
		PolicyID *string          `json:"policyId"`
		Cron     *string          `json:"cron"`
		StartAt  *time.Time       `json:"startAt"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Name != nil {
		schedule.Name = *req.Name
	}
	if req.Kind != nil {
		schedule.Kind = *req.Kind
	}
	if req.TaskKind != nil {
		schedule.TaskKind = *req.TaskKind
	}
	if req.Target != nil {
		schedule.Target = *req.Target
	}
	if req.PolicyID != nil {
		schedule.PolicyID = *req.PolicyID
	}
	if req.Cron != nil {
		schedule.Cron = *req.Cron
	}
	if req.StartAt != nil {
		schedule.StartAt = req.StartAt
	}
	if err := s.Scheduler.UpdateSchedule(r.Context(), schedule); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "schedule_update_failed", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "schedule.update", Resource: schedule.ID,
	})
	httpx.WriteJSON(w, http.StatusOK, schedule)
}

func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.DeleteSchedule(r.Context(), id, principalFrom(r.Context()).Username); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "schedule.delete", Resource: id,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
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

func (s *Server) handleGetGitHubMonitor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	monitor, err := s.Store.GetGitHubMonitor(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if monitor == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "github_monitor_not_found", "GitHub monitor not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, monitor)
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
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "github_monitor.create", Resource: m.ID,
	})
	httpx.WriteJSON(w, http.StatusCreated, m)
}

func (s *Server) handleUpdateGitHubMonitor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	monitor, err := s.Store.GetGitHubMonitor(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if monitor == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "github_monitor_not_found", "GitHub monitor not found")
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Keyword     *string `json:"keyword"`
		IntervalSec *int    `json:"intervalSec"`
		Status      *string `json:"status"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Name != nil {
		monitor.Name = strings.TrimSpace(*req.Name)
	}
	if req.Keyword != nil {
		monitor.Keyword = strings.TrimSpace(*req.Keyword)
	}
	if req.IntervalSec != nil {
		monitor.IntervalSec = *req.IntervalSec
	}
	if req.Status != nil {
		monitor.Status = strings.TrimSpace(*req.Status)
	}
	if monitor.Keyword == "" || monitor.IntervalSec < 3600 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_github_monitor", "keyword is required and interval must be at least one hour")
		return
	}
	if monitor.Name == "" {
		monitor.Name = "GitHub监控-" + monitor.Keyword
	}
	monitor.NextRunAt = time.Now().UTC().Add(time.Duration(monitor.IntervalSec) * time.Second)
	if err := s.Store.UpdateGitHubMonitor(r.Context(), monitor); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "github_monitor.update", Resource: monitor.ID,
	})
	httpx.WriteJSON(w, http.StatusOK, monitor)
}

func (s *Server) handleDeleteGitHubMonitor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.DeleteGitHubMonitor(r.Context(), id, principalFrom(r.Context()).Username); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "github_monitor.delete", Resource: id,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
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

func (s *Server) handleGetFingerprint(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	rule, err := s.Store.GetFingerprint(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if rule == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "fingerprint_not_found", "fingerprint not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rule)
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
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "fingerprint.create", Resource: rule.ID,
	})
	httpx.WriteJSON(w, http.StatusCreated, rule)
}

func (s *Server) handleUpdateFingerprint(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	rule, err := s.Store.GetFingerprint(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if rule == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "fingerprint_not_found", "fingerprint not found")
		return
	}
	var req struct {
		Name *string `json:"name"`
		Rule *string `json:"rule"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Name != nil {
		rule.Name = strings.TrimSpace(*req.Name)
	}
	if req.Rule != nil {
		rule.Rule = strings.TrimSpace(*req.Rule)
	}
	if rule.Name == "" || rule.Rule == "" {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_fingerprint", "name and rule are required")
		return
	}
	if _, err := webintelParse(rule.Rule); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_fingerprint", err.Error())
		return
	}
	if err := s.Store.UpdateFingerprint(r.Context(), rule); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "fingerprint_update_failed", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "fingerprint.update", Resource: rule.ID,
	})
	httpx.WriteJSON(w, http.StatusOK, rule)
}

func (s *Server) handleDeleteFingerprint(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.DeleteFingerprint(r.Context(), id, principalFrom(r.Context()).Username); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		Actor: principalFrom(r.Context()).Username, Action: "fingerprint.delete", Resource: id,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
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
