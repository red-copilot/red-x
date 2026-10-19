package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/github"
	"github.com/red-copilot/red-x/internal/modules/tasking"
	"github.com/red-copilot/red-x/internal/store"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Scheduler struct {
	Store  *store.Store
	Engine *tasking.Engine
	GitHub *github.Service
	Log    *slog.Logger
	Owner  string
}

func New(st *store.Store, engine *tasking.Engine, gh *github.Service, log *slog.Logger) *Scheduler {
	return &Scheduler{Store: st, Engine: engine, GitHub: gh, Log: log, Owner: uuid.NewString()}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = s.Store.ReleaseLease(context.Background(), "scheduler", s.Owner)
			return
		case <-ticker.C:
			ok, err := s.Store.AcquireLease(ctx, "scheduler", s.Owner, 30*time.Second)
			if err != nil || !ok {
				continue
			}
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now().UTC()
	monitors, err := s.Store.DueMonitors(ctx, now, 20)
	if err == nil {
		for _, m := range monitors {
			if err := s.RunMonitor(ctx, m); err != nil {
				s.Log.Warn("monitor run failed", "monitor", m.ID.Hex(), "error", err)
			}
		}
	}
	schedules, err := s.Store.DueSchedules(ctx, now, 20)
	if err == nil {
		for _, sc := range schedules {
			if err := s.RunSchedule(ctx, sc); err != nil {
				s.Log.Warn("schedule run failed", "schedule", sc.ID.Hex(), "error", err)
			}
		}
	}
	gh, _, err := s.Store.ListGitHubMonitors(ctx, store.PageQuery{Page: 1, PageSize: 200})
	if err == nil {
		for _, m := range gh {
			if m.Status == "active" && !m.NextRunAt.After(now) {
				go func(m domain.GitHubMonitor) {
					runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
					defer cancel()
					if err := s.GitHub.RunMonitor(runCtx, m); err != nil {
						s.Log.Warn("github monitor failed", "monitor", m.ID.Hex(), "error", err)
					}
				}(m)
			}
		}
	}
}

func (s *Scheduler) RunMonitor(ctx context.Context, m domain.Monitor) error {
	kind := domain.TaskDomain
	switch m.Kind {
	case "ip":
		kind = domain.TaskIP
	case "site_update":
		kind = domain.TaskAssetSiteSync
	case "wih_update":
		kind = domain.TaskWIHUpdate
	}
	opts := m.Options
	opts.MonitorID = m.ID.Hex()
	tasks, err := s.Engine.Submit(ctx, tasking.SubmitInput{
		Name: fmt.Sprintf("监控-%s-%s", m.Name, time.Now().Format("20060102-150405")),
		Kind: kind, Target: m.Target, Options: opts, PolicyID: m.PolicyID,
		ScopeID: m.ScopeID, CreatedBy: "scheduler",
	})
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	m.LastRunAt = &now
	m.NextRunAt = now.Add(time.Duration(m.IntervalSec) * time.Second)
	if len(tasks) > 0 {
		m.LastRunID = tasks[0].ID.Hex()
	}
	if err := s.Store.UpdateMonitor(ctx, &m); err != nil {
		return err
	}
	_ = s.Store.CreateMonitorRun(ctx, &domain.MonitorRun{
		MonitorID: m.ID, TaskID: m.LastRunID, Status: "started", StartedAt: now,
	})
	return nil
}

func (s *Scheduler) RunSchedule(ctx context.Context, sc domain.Schedule) error {
	taskKind := sc.TaskKind
	if taskKind == "" {
		taskKind = domain.TaskDomain
	}
	tasks, err := s.Engine.Submit(ctx, tasking.SubmitInput{
		Name: fmt.Sprintf("定时任务-%s-%d", sc.Name, sc.RunCount+1),
		Kind: taskKind, Target: sc.Target, PolicyID: sc.PolicyID, CreatedBy: "scheduler",
	})
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	sc.RunCount++
	sc.LastRunAt = &now
	if sc.Kind == "future_scan" {
		sc.Status = "completed"
		sc.NextRunAt = nil
	} else if sc.Cron != "" {
		if next, err := NextCron(sc.Cron, now); err == nil {
			sc.NextRunAt = &next
		}
	}
	_ = tasks
	return s.Store.UpdateSchedule(ctx, &sc)
}

func NextCron(spec string, from time.Time) (time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(spec)
	if err != nil {
		return time.Time{}, err
	}
	return schedule.Next(from), nil
}

func (s *Scheduler) CreateMonitor(ctx context.Context, m *domain.Monitor) error {
	if m.IntervalSec < 21600 {
		return fmt.Errorf("monitor interval must be at least 6 hours")
	}
	if m.Status == "" {
		m.Status = "active"
	}
	if m.NextRunAt.IsZero() {
		m.NextRunAt = time.Now().UTC().Add(30 * time.Second)
	}
	if _, err := store.ObjectID(m.ScopeID); err != nil {
		return fmt.Errorf("invalid scope id")
	}
	scopeID, _ := store.ObjectID(m.ScopeID)
	if scope, err := s.Store.GetScope(ctx, scopeID); err != nil || scope == nil {
		return fmt.Errorf("scope not found")
	}
	return s.Store.CreateMonitor(ctx, m)
}

func (s *Scheduler) CreateSchedule(ctx context.Context, sc *domain.Schedule) error {
	if sc.PolicyID == "" {
		return fmt.Errorf("policyId is required")
	}
	policyID, err := store.ObjectID(sc.PolicyID)
	if err != nil {
		return fmt.Errorf("invalid policy id")
	}
	if policy, err := s.Store.GetPolicy(ctx, policyID); err != nil || policy == nil {
		return fmt.Errorf("policy not found")
	}
	if sc.Status == "" {
		sc.Status = "active"
	}
	now := time.Now().UTC()
	if sc.Kind == "future_scan" {
		if sc.StartAt == nil {
			return fmt.Errorf("startAt is required for future_scan")
		}
		sc.NextRunAt = sc.StartAt
	} else {
		if sc.Cron == "" {
			return fmt.Errorf("cron is required for recurrent_scan")
		}
		next, err := NextCron(sc.Cron, now)
		if err != nil {
			return err
		}
		sc.NextRunAt = &next
	}
	return s.Store.CreateSchedule(ctx, sc)
}

func (s *Scheduler) PauseMonitor(ctx context.Context, id bson.ObjectID, paused bool) error {
	m, err := s.Store.GetMonitor(ctx, id)
	if err != nil || m == nil {
		return fmt.Errorf("monitor not found")
	}
	if paused {
		m.Status = "paused"
	} else {
		m.Status = "active"
		if m.NextRunAt.Before(time.Now().UTC()) {
			m.NextRunAt = time.Now().UTC().Add(30 * time.Second)
		}
	}
	return s.Store.UpdateMonitor(ctx, m)
}

func (s *Scheduler) DeleteMonitor(ctx context.Context, id bson.ObjectID) error {
	return s.Store.DeleteMonitor(ctx, id)
}

func (s *Scheduler) RunMonitorNow(ctx context.Context, id bson.ObjectID) error {
	m, err := s.Store.GetMonitor(ctx, id)
	if err != nil || m == nil {
		return fmt.Errorf("monitor not found")
	}
	return s.RunMonitor(ctx, *m)
}
