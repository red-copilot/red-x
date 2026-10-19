package store

import (
	"context"
	"errors"
	"time"

	"github.com/Aabyss-Team/ARL/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *Store) CreateMonitor(ctx context.Context, m *domain.Monitor) error {
	now := time.Now().UTC()
	m.CreatedAt, m.UpdatedAt, m.SchemaVersion = now, now, domain.SchemaVersion
	if m.ID.IsZero() {
		m.ID = bson.NewObjectID()
	}
	_, err := s.C(ColMonitors).InsertOne(ctx, m)
	return err
}

func (s *Store) GetMonitor(ctx context.Context, id bson.ObjectID) (*domain.Monitor, error) {
	var m domain.Monitor
	err := s.C(ColMonitors).FindOne(ctx, bson.M{"_id": id}).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &m, err
}

func (s *Store) ListMonitors(ctx context.Context, q PageQuery) ([]domain.Monitor, int64, error) {
	q = q.Normalized()
	filter := bson.M{}
	if q.Search != "" {
		filter["name"] = bson.M{"$regex": q.Search, "$options": "i"}
	}
	total, err := s.C(ColMonitors).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(SortDocument(q.Sort)).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColMonitors).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.Monitor
	err = cur.All(ctx, &out)
	return out, total, err
}

func (s *Store) UpdateMonitor(ctx context.Context, m *domain.Monitor) error {
	m.UpdatedAt = time.Now().UTC()
	_, err := s.C(ColMonitors).ReplaceOne(ctx, bson.M{"_id": m.ID}, m)
	return err
}

func (s *Store) DeleteMonitor(ctx context.Context, id bson.ObjectID) error {
	_, err := s.C(ColMonitors).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (s *Store) DueMonitors(ctx context.Context, now time.Time, limit int) ([]domain.Monitor, error) {
	if limit <= 0 {
		limit = 20
	}
	cur, err := s.C(ColMonitors).Find(ctx,
		bson.M{"status": "active", "nextRunAt": bson.M{"$lte": now}},
		options.Find().SetSort(bson.D{{Key: "nextRunAt", Value: 1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Monitor
	err = cur.All(ctx, &out)
	return out, err
}

func (s *Store) CreateSchedule(ctx context.Context, sc *domain.Schedule) error {
	now := time.Now().UTC()
	sc.CreatedAt, sc.UpdatedAt, sc.SchemaVersion = now, now, domain.SchemaVersion
	if sc.ID.IsZero() {
		sc.ID = bson.NewObjectID()
	}
	_, err := s.C(ColSchedules).InsertOne(ctx, sc)
	return err
}

func (s *Store) ListSchedules(ctx context.Context, q PageQuery) ([]domain.Schedule, int64, error) {
	q = q.Normalized()
	filter := bson.M{}
	if q.Search != "" {
		filter["name"] = bson.M{"$regex": q.Search, "$options": "i"}
	}
	total, err := s.C(ColSchedules).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(SortDocument(q.Sort)).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColSchedules).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.Schedule
	err = cur.All(ctx, &out)
	return out, total, err
}

func (s *Store) DueSchedules(ctx context.Context, now time.Time, limit int) ([]domain.Schedule, error) {
	if limit <= 0 {
		limit = 20
	}
	cur, err := s.C(ColSchedules).Find(ctx,
		bson.M{"status": "active", "nextRunAt": bson.M{"$lte": now}},
		options.Find().SetSort(bson.D{{Key: "nextRunAt", Value: 1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Schedule
	err = cur.All(ctx, &out)
	return out, err
}

func (s *Store) UpdateSchedule(ctx context.Context, sc *domain.Schedule) error {
	sc.UpdatedAt = time.Now().UTC()
	_, err := s.C(ColSchedules).ReplaceOne(ctx, bson.M{"_id": sc.ID}, sc)
	return err
}

func (s *Store) DeleteSchedule(ctx context.Context, id bson.ObjectID) error {
	_, err := s.C(ColSchedules).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (s *Store) CreateMonitorRun(ctx context.Context, r *domain.MonitorRun) error {
	r.StartedAt = time.Now().UTC()
	r.SchemaVersion = domain.SchemaVersion
	if r.ID.IsZero() {
		r.ID = bson.NewObjectID()
	}
	_, err := s.C(ColMonitorRuns).InsertOne(ctx, r)
	return err
}

func (s *Store) CreateGitHubTask(ctx context.Context, t *domain.GitHubTask) error {
	t.CreatedAt = time.Now().UTC()
	t.SchemaVersion = domain.SchemaVersion
	if t.ID.IsZero() {
		t.ID = bson.NewObjectID()
	}
	_, err := s.C(ColGitHubTasks).InsertOne(ctx, t)
	return err
}

func (s *Store) GetGitHubTask(ctx context.Context, id bson.ObjectID) (*domain.GitHubTask, error) {
	var t domain.GitHubTask
	err := s.C(ColGitHubTasks).FindOne(ctx, bson.M{"_id": id}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &t, err
}

func (s *Store) UpdateGitHubTask(ctx context.Context, t *domain.GitHubTask) error {
	_, err := s.C(ColGitHubTasks).ReplaceOne(ctx, bson.M{"_id": t.ID}, t)
	return err
}

func (s *Store) ListGitHubTasks(ctx context.Context, q PageQuery) ([]domain.GitHubTask, int64, error) {
	q = q.Normalized()
	filter := bson.M{}
	if q.Search != "" {
		filter["keyword"] = bson.M{"$regex": q.Search, "$options": "i"}
	}
	total, err := s.C(ColGitHubTasks).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(SortDocument(q.Sort)).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColGitHubTasks).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.GitHubTask
	err = cur.All(ctx, &out)
	return out, total, err
}

func (s *Store) CreateGitHubMonitor(ctx context.Context, m *domain.GitHubMonitor) error {
	now := time.Now().UTC()
	m.CreatedAt, m.UpdatedAt, m.SchemaVersion = now, now, domain.SchemaVersion
	if m.ID.IsZero() {
		m.ID = bson.NewObjectID()
	}
	_, err := s.C(ColGitHubMonitors).InsertOne(ctx, m)
	return err
}

func (s *Store) ListGitHubMonitors(ctx context.Context, q PageQuery) ([]domain.GitHubMonitor, int64, error) {
	q = q.Normalized()
	filter := bson.M{}
	if q.Search != "" {
		filter["keyword"] = bson.M{"$regex": q.Search, "$options": "i"}
	}
	total, err := s.C(ColGitHubMonitors).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(SortDocument(q.Sort)).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColGitHubMonitors).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.GitHubMonitor
	err = cur.All(ctx, &out)
	return out, total, err
}

func (s *Store) UpdateGitHubMonitor(ctx context.Context, m *domain.GitHubMonitor) error {
	m.UpdatedAt = time.Now().UTC()
	_, err := s.C(ColGitHubMonitors).ReplaceOne(ctx, bson.M{"_id": m.ID}, m)
	return err
}

func (s *Store) DeleteGitHubMonitor(ctx context.Context, id bson.ObjectID) error {
	_, err := s.C(ColGitHubMonitors).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (s *Store) AddGitHubFinding(ctx context.Context, f *domain.GitHubFinding) (bool, error) {
	f.CreatedAt = time.Now().UTC()
	f.SchemaVersion = domain.SchemaVersion
	if f.ID.IsZero() {
		f.ID = bson.NewObjectID()
	}
	_, err := s.C(ColGitHubFindings).InsertOne(ctx, f)
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) ListGitHubFindings(ctx context.Context, taskID, monitorID string, q PageQuery) ([]domain.GitHubFinding, int64, error) {
	q = q.Normalized()
	filter := bson.M{}
	if taskID != "" {
		filter["taskId"] = taskID
	}
	if monitorID != "" {
		filter["monitorId"] = monitorID
	}
	if q.Search != "" {
		filter["$or"] = []bson.M{
			{"repo": bson.M{"$regex": q.Search, "$options": "i"}},
			{"path": bson.M{"$regex": q.Search, "$options": "i"}},
		}
	}
	total, err := s.C(ColGitHubFindings).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(SortDocument(q.Sort)).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColGitHubFindings).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.GitHubFinding
	err = cur.All(ctx, &out)
	return out, total, err
}
