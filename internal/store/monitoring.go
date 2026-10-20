package store

import (
	"context"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
)

func (s *Store) CreateMonitor(ctx context.Context, m *domain.Monitor) error {
	now := time.Now().UTC()
	m.CreatedAt, m.UpdatedAt, m.SchemaVersion = now, now, domain.SchemaVersion
	if m.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		m.ID = id
	}
	return s.db.WithContext(ctx).Create(m).Error
}

func (s *Store) GetMonitor(ctx context.Context, id string) (*domain.Monitor, error) {
	var m domain.Monitor
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) ListMonitors(ctx context.Context, q PageQuery) ([]domain.Monitor, int64, error) {
	q = q.Normalized()
	db := s.db.WithContext(ctx).Model(&domain.Monitor{})
	if q.Search != "" {
		db = db.Where(`name ILIKE ? ESCAPE '\'`, ilike(q.Search))
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.Monitor
	err := db.Order(OrderClause(q.Sort)).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}

func (s *Store) UpdateMonitor(ctx context.Context, m *domain.Monitor) error {
	m.UpdatedAt = time.Now().UTC()
	return s.db.WithContext(ctx).Save(m).Error
}

func (s *Store) DeleteMonitor(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&domain.Monitor{}, "id = ?", id).Error
}

func (s *Store) DueMonitors(ctx context.Context, now time.Time, limit int) ([]domain.Monitor, error) {
	if limit <= 0 {
		limit = 20
	}
	var out []domain.Monitor
	err := s.db.WithContext(ctx).Where("status = ? AND next_run_at <= ?", "active", now).
		Order("next_run_at ASC, id ASC").Limit(limit).Find(&out).Error
	return out, err
}

func (s *Store) CreateSchedule(ctx context.Context, sc *domain.Schedule) error {
	now := time.Now().UTC()
	sc.CreatedAt, sc.UpdatedAt, sc.SchemaVersion = now, now, domain.SchemaVersion
	if sc.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		sc.ID = id
	}
	return s.db.WithContext(ctx).Create(sc).Error
}

func (s *Store) ListSchedules(ctx context.Context, q PageQuery) ([]domain.Schedule, int64, error) {
	q = q.Normalized()
	db := s.db.WithContext(ctx).Model(&domain.Schedule{})
	if q.Search != "" {
		db = db.Where(`name ILIKE ? ESCAPE '\'`, ilike(q.Search))
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.Schedule
	err := db.Order(OrderClause(q.Sort)).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}

func (s *Store) DueSchedules(ctx context.Context, now time.Time, limit int) ([]domain.Schedule, error) {
	if limit <= 0 {
		limit = 20
	}
	var out []domain.Schedule
	err := s.db.WithContext(ctx).Where("status = ? AND next_run_at <= ?", "active", now).
		Order("next_run_at ASC, id ASC").Limit(limit).Find(&out).Error
	return out, err
}

func (s *Store) UpdateSchedule(ctx context.Context, sc *domain.Schedule) error {
	sc.UpdatedAt = time.Now().UTC()
	return s.db.WithContext(ctx).Save(sc).Error
}

func (s *Store) DeleteSchedule(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&domain.Schedule{}, "id = ?", id).Error
}

func (s *Store) CreateMonitorRun(ctx context.Context, r *domain.MonitorRun) error {
	r.StartedAt = time.Now().UTC()
	r.SchemaVersion = domain.SchemaVersion
	if r.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		r.ID = id
	}
	return s.db.WithContext(ctx).Create(r).Error
}

func (s *Store) CreateGitHubTask(ctx context.Context, t *domain.GitHubTask) error {
	t.CreatedAt = time.Now().UTC()
	t.SchemaVersion = domain.SchemaVersion
	if t.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		t.ID = id
	}
	return s.db.WithContext(ctx).Create(t).Error
}

func (s *Store) GetGitHubTask(ctx context.Context, id string) (*domain.GitHubTask, error) {
	var t domain.GitHubTask
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&t).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) UpdateGitHubTask(ctx context.Context, t *domain.GitHubTask) error {
	return s.db.WithContext(ctx).Save(t).Error
}

func (s *Store) ListGitHubTasks(ctx context.Context, q PageQuery) ([]domain.GitHubTask, int64, error) {
	q = q.Normalized()
	db := s.db.WithContext(ctx).Model(&domain.GitHubTask{})
	if q.Search != "" {
		db = db.Where(`keyword ILIKE ? ESCAPE '\'`, ilike(q.Search))
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.GitHubTask
	err := db.Order(OrderClause(q.Sort)).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}

func (s *Store) CreateGitHubMonitor(ctx context.Context, m *domain.GitHubMonitor) error {
	now := time.Now().UTC()
	m.CreatedAt, m.UpdatedAt, m.SchemaVersion = now, now, domain.SchemaVersion
	if m.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		m.ID = id
	}
	return s.db.WithContext(ctx).Create(m).Error
}

func (s *Store) ListGitHubMonitors(ctx context.Context, q PageQuery) ([]domain.GitHubMonitor, int64, error) {
	q = q.Normalized()
	db := s.db.WithContext(ctx).Model(&domain.GitHubMonitor{})
	if q.Search != "" {
		db = db.Where(`keyword ILIKE ? ESCAPE '\'`, ilike(q.Search))
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.GitHubMonitor
	err := db.Order(OrderClause(q.Sort)).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}

func (s *Store) UpdateGitHubMonitor(ctx context.Context, m *domain.GitHubMonitor) error {
	m.UpdatedAt = time.Now().UTC()
	return s.db.WithContext(ctx).Save(m).Error
}

func (s *Store) DeleteGitHubMonitor(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&domain.GitHubMonitor{}, "id = ?", id).Error
}

func (s *Store) AddGitHubFinding(ctx context.Context, f *domain.GitHubFinding) (bool, error) {
	f.CreatedAt = time.Now().UTC()
	f.SchemaVersion = domain.SchemaVersion
	if f.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return false, err
		}
		f.ID = id
	}
	err := s.db.WithContext(ctx).Create(f).Error
	if isDuplicate(err) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) ListGitHubFindings(ctx context.Context, taskID, monitorID string, q PageQuery) ([]domain.GitHubFinding, int64, error) {
	q = q.Normalized()
	db := s.db.WithContext(ctx).Model(&domain.GitHubFinding{})
	if taskID != "" {
		db = db.Where("task_id = ?", taskID)
	}
	if monitorID != "" {
		db = db.Where("monitor_id = ?", monitorID)
	}
	if q.Search != "" {
		pattern := ilike(q.Search)
		db = db.Where(`(repo ILIKE ? ESCAPE '\' OR path ILIKE ? ESCAPE '\')`, pattern, pattern)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.GitHubFinding
	err := db.Order(OrderClause(q.Sort)).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}
