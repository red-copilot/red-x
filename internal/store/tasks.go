package store

import (
	"context"
	"errors"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"gorm.io/gorm"
)

type TaskPatch struct {
	Status         *domain.TaskStatus
	Error          *string
	Progress       *int
	Warnings       *[]string
	Statistics     *domain.TaskStatistics
	StartedAt      *time.Time
	FinishedAt     *time.Time
	LastScreenshot *string
}

func (p TaskPatch) updates() map[string]any {
	out := map[string]any{}
	if p.Status != nil {
		out["status"] = *p.Status
	}
	if p.Error != nil {
		out["error"] = *p.Error
	}
	if p.Progress != nil {
		out["progress"] = *p.Progress
	}
	if p.Warnings != nil {
		out["warnings"] = jsonbExpr(*p.Warnings)
	}
	if p.Statistics != nil {
		out["statistics"] = jsonbExpr(*p.Statistics)
	}
	if p.StartedAt != nil {
		out["started_at"] = *p.StartedAt
	}
	if p.FinishedAt != nil {
		out["finished_at"] = *p.FinishedAt
	}
	if p.LastScreenshot != nil {
		out["last_screenshot"] = *p.LastScreenshot
	}
	return out
}

type StepPatch struct {
	Status     *domain.StepStatus
	Progress   *int
	Attempts   *int
	Message    *string
	Error      *string
	StartedAt  *time.Time
	FinishedAt *time.Time
}

func (p StepPatch) updates() map[string]any {
	out := map[string]any{"updated_at": time.Now().UTC()}
	if p.Status != nil {
		out["status"] = *p.Status
	}
	if p.Progress != nil {
		out["progress"] = *p.Progress
	}
	if p.Attempts != nil {
		out["attempts"] = *p.Attempts
	}
	if p.Message != nil {
		out["message"] = *p.Message
	}
	if p.Error != nil {
		out["error"] = *p.Error
	}
	if p.StartedAt != nil {
		out["started_at"] = *p.StartedAt
	}
	if p.FinishedAt != nil {
		out["finished_at"] = *p.FinishedAt
	}
	return out
}

type ResultSet struct {
	ID        string    `gorm:"primaryKey;size:24" json:"id"`
	Kind      string    `json:"kind"`
	Items     []string  `gorm:"type:jsonb;serializer:json" json:"items"`
	Total     int       `json:"total"`
	CreatedAt time.Time `json:"createdAt"`
}

func (ResultSet) TableName() string { return "result_sets" }

func (s *Store) CreateTask(ctx context.Context, t *domain.Task) error {
	now := time.Now().UTC()
	t.CreatedAt, t.SchemaVersion = now, domain.SchemaVersion
	if t.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		t.ID = id
	}
	return s.db.WithContext(ctx).Create(t).Error
}

func (s *Store) GetTask(ctx context.Context, id string) (*domain.Task, error) {
	var t domain.Task
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&t).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) ListTasks(ctx context.Context, q PageQuery, status, kind string) ([]domain.Task, int64, error) {
	q = q.Normalized()
	db := s.db.WithContext(ctx).Model(&domain.Task{})
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if kind != "" {
		db = db.Where("kind = ?", kind)
	}
	if q.Search != "" {
		pattern := ilike(q.Search)
		db = db.Where(`(name ILIKE ? ESCAPE '\' OR target ILIKE ? ESCAPE '\')`, pattern, pattern)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.Task
	err := db.Order(OrderClause(q.Sort)).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}

func (s *Store) UpdateTask(ctx context.Context, t *domain.Task) error {
	return s.db.WithContext(ctx).Save(t).Error
}

func (s *Store) UpdateTaskFields(ctx context.Context, id string, patch TaskPatch) error {
	updates := patch.updates()
	if len(updates) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&domain.Task{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Store) DeleteTask(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", id).Delete(&domain.TaskStep{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_id = ?", id).Delete(&domain.TaskEvent{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&domain.Task{}).Error
	})
}

func (s *Store) CreateSteps(ctx context.Context, steps []domain.TaskStep) error {
	if len(steps) == 0 {
		return nil
	}
	now := time.Now().UTC()
	for i := range steps {
		steps[i].CreatedAt, steps[i].UpdatedAt = now, now
		steps[i].SchemaVersion = domain.SchemaVersion
		if steps[i].ID == "" {
			id, err := domain.NewID()
			if err != nil {
				return err
			}
			steps[i].ID = id
		}
	}
	return s.db.WithContext(ctx).Create(&steps).Error
}

func (s *Store) ListSteps(ctx context.Context, taskID string) ([]domain.TaskStep, error) {
	var out []domain.TaskStep
	err := s.db.WithContext(ctx).Where("task_id = ?", taskID).Order("sequence ASC").Find(&out).Error
	return out, err
}

func (s *Store) GetStep(ctx context.Context, id string) (*domain.TaskStep, error) {
	var step domain.TaskStep
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&step).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &step, nil
}

func (s *Store) UpdateStepFields(ctx context.Context, id string, patch StepPatch) error {
	updates := patch.updates()
	return s.db.WithContext(ctx).Model(&domain.TaskStep{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Store) GetNextStep(ctx context.Context, taskID string, sequence int) (*domain.TaskStep, error) {
	var step domain.TaskStep
	err := s.db.WithContext(ctx).Where("task_id = ? AND sequence > ?", taskID, sequence).Order("sequence ASC").First(&step).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &step, nil
}

func (s *Store) AppendTaskEvent(ctx context.Context, e *domain.TaskEvent) error {
	e.CreatedAt = time.Now().UTC()
	if e.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		e.ID = id
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e.Sequence == 0 {
			var seq int64
			if err := tx.Raw(`UPDATE tasks SET event_seq = event_seq + 1 WHERE id = ? RETURNING event_seq`, e.TaskID).Row().Scan(&seq); err != nil {
				return err
			}
			e.Sequence = seq
		}
		return tx.Create(e).Error
	})
}

func (s *Store) ListTaskEvents(ctx context.Context, taskID string, after int64, limit int) ([]domain.TaskEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var out []domain.TaskEvent
	err := s.db.WithContext(ctx).
		Where("task_id = ? AND sequence > ?", taskID, after).
		Order("sequence ASC").Limit(limit).Find(&out).Error
	return out, err
}

func (s *Store) AddOutbox(ctx context.Context, routingKey string, payload []byte) error {
	now := time.Now().UTC()
	id, err := domain.NewID()
	if err != nil {
		return err
	}
	msg := domain.OutboxMessage{
		ID: id, RoutingKey: routingKey, Payload: payload, Status: "pending",
		AvailableAt: now, CreatedAt: now,
	}
	return s.db.WithContext(ctx).Create(&msg).Error
}

func (s *Store) PendingOutbox(ctx context.Context, limit int) ([]domain.OutboxMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	var out []domain.OutboxMessage
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		stale := now.Add(-5 * time.Minute)
		var ids []string
		if err := tx.Raw(`
SELECT id FROM outbox
WHERE (status = 'pending' AND available_at <= ?)
   OR (status = 'processing' AND processing_at < ?)
ORDER BY created_at ASC
LIMIT ?
FOR UPDATE SKIP LOCKED`, now, stale, limit).Scan(&ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Model(&domain.OutboxMessage{}).Where("id IN ?", ids).Updates(map[string]any{
			"status": "processing", "processing_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Order("created_at ASC").Find(&out).Error
	})
	return out, err
}

func (s *Store) MarkOutbox(ctx context.Context, id, status string, attempts int) error {
	fields := map[string]any{"status": status, "attempts": attempts, "processing_at": nil}
	if status == "published" {
		fields["published_at"] = time.Now().UTC()
	} else {
		fields["available_at"] = time.Now().UTC().Add(time.Duration(attempts*attempts) * time.Second)
	}
	return s.db.WithContext(ctx).Model(&domain.OutboxMessage{}).Where("id = ?", id).Updates(fields).Error
}

func (s *Store) CountTasksByStatus(ctx context.Context) (map[string]int64, error) {
	var rows []struct {
		Status string
		Count  int64
	}
	if err := s.db.WithContext(ctx).Model(&domain.Task{}).Select("status, count(*) AS count").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, row := range rows {
		out[row.Status] = row.Count
	}
	return out, nil
}

func (s *Store) AcquireLease(ctx context.Context, name, owner string, ttl time.Duration) (bool, error) {
	now := time.Now().UTC()
	expires := now.Add(ttl)
	var got string
	err := s.db.WithContext(ctx).Raw(`
INSERT INTO leases (id, owner, expires_at) VALUES (?, ?, ?)
ON CONFLICT (id) DO UPDATE SET owner = EXCLUDED.owner, expires_at = EXCLUDED.expires_at
WHERE leases.expires_at < ? OR leases.owner = ?
RETURNING id`, name, owner, expires, now, owner).Scan(&got).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return got != "", nil
}

func (s *Store) ReleaseLease(ctx context.Context, name, owner string) error {
	return s.db.WithContext(ctx).Where("id = ? AND owner = ?", name, owner).Delete(&domain.Lease{}).Error
}

func (s *Store) CreateResultSet(ctx context.Context, kind string, items []string) (string, error) {
	id, err := domain.NewID()
	if err != nil {
		return "", err
	}
	rs := ResultSet{ID: id, Kind: kind, Items: items, Total: len(items), CreatedAt: time.Now().UTC()}
	if err := s.db.WithContext(ctx).Create(&rs).Error; err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) GetResultSet(ctx context.Context, id string) ([]string, error) {
	var rs ResultSet
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&rs).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rs.Items, nil
}

func (s *Store) ListEASMTasks(ctx context.Context, organizationID, status, kind string, q PageQuery) ([]domain.Task, int64, error) {
	q = q.Normalized()
	db := s.db.WithContext(ctx).Model(&domain.Task{}).Where("origin = ?", domain.TaskOriginEASM)
	if organizationID != "" {
		db = db.Where("organization_id = ?", organizationID)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if kind != "" {
		db = db.Where("kind = ?", kind)
	}
	if q.Search != "" {
		pattern := ilike(q.Search)
		db = db.Where(`(name ILIKE ? ESCAPE '\' OR target ILIKE ? ESCAPE '\' OR root_domain ILIKE ? ESCAPE '\')`, pattern, pattern, pattern)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.Task
	err := db.Order(OrderClause(q.Sort)).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}
