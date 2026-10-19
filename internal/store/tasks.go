package store

import (
	"context"
	"errors"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *Store) CreateTask(ctx context.Context, t *domain.Task) error {
	now := time.Now().UTC()
	t.CreatedAt, t.SchemaVersion = now, domain.SchemaVersion
	if t.ID.IsZero() {
		t.ID = bson.NewObjectID()
	}
	_, err := s.C(ColTasks).InsertOne(ctx, t)
	return err
}

func (s *Store) GetTask(ctx context.Context, id bson.ObjectID) (*domain.Task, error) {
	var t domain.Task
	err := s.C(ColTasks).FindOne(ctx, bson.M{"_id": id}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &t, err
}

func (s *Store) ListTasks(ctx context.Context, q PageQuery, status, kind string) ([]domain.Task, int64, error) {
	q = q.Normalized()
	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}
	if kind != "" {
		filter["kind"] = kind
	}
	if q.Search != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": q.Search, "$options": "i"}},
			{"target": bson.M{"$regex": q.Search, "$options": "i"}},
		}
	}
	total, err := s.C(ColTasks).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(SortDocument(q.Sort)).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColTasks).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.Task
	err = cur.All(ctx, &out)
	return out, total, err
}

func (s *Store) UpdateTask(ctx context.Context, t *domain.Task) error {
	_, err := s.C(ColTasks).ReplaceOne(ctx, bson.M{"_id": t.ID}, t)
	return err
}

func (s *Store) UpdateTaskFields(ctx context.Context, id bson.ObjectID, fields bson.M) error {
	_, err := s.C(ColTasks).UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": fields})
	return err
}

func (s *Store) DeleteTask(ctx context.Context, id bson.ObjectID) error {
	_, err := s.C(ColTasks).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (s *Store) CreateSteps(ctx context.Context, steps []domain.TaskStep) error {
	if len(steps) == 0 {
		return nil
	}
	now := time.Now().UTC()
	docs := make([]any, 0, len(steps))
	for i := range steps {
		steps[i].CreatedAt, steps[i].UpdatedAt = now, now
		if steps[i].ID.IsZero() {
			steps[i].ID = bson.NewObjectID()
		}
		docs = append(docs, steps[i])
	}
	_, err := s.C(ColTaskSteps).InsertMany(ctx, docs)
	return err
}

func (s *Store) ListSteps(ctx context.Context, taskID bson.ObjectID) ([]domain.TaskStep, error) {
	cur, err := s.C(ColTaskSteps).Find(ctx, bson.M{"taskId": taskID}, options.Find().SetSort(bson.D{{Key: "sequence", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.TaskStep
	err = cur.All(ctx, &out)
	return out, err
}

func (s *Store) GetStep(ctx context.Context, id bson.ObjectID) (*domain.TaskStep, error) {
	var step domain.TaskStep
	err := s.C(ColTaskSteps).FindOne(ctx, bson.M{"_id": id}).Decode(&step)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &step, err
}

func (s *Store) UpdateStepFields(ctx context.Context, id bson.ObjectID, fields bson.M) error {
	fields["updatedAt"] = time.Now().UTC()
	_, err := s.C(ColTaskSteps).UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": fields})
	return err
}

func (s *Store) GetNextStep(ctx context.Context, taskID bson.ObjectID, sequence int) (*domain.TaskStep, error) {
	var step domain.TaskStep
	err := s.C(ColTaskSteps).FindOne(ctx, bson.M{"taskId": taskID, "sequence": bson.M{"$gt": sequence}}, options.FindOne().SetSort(bson.D{{Key: "sequence", Value: 1}})).Decode(&step)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &step, err
}

func (s *Store) AppendTaskEvent(ctx context.Context, e *domain.TaskEvent) error {
	e.CreatedAt = time.Now().UTC()
	if e.Sequence == 0 {
		last := struct {
			Sequence int64 `bson:"sequence"`
		}{}
		opts := options.FindOne().SetSort(bson.D{{Key: "sequence", Value: -1}})
		_ = s.C(ColTaskEvents).FindOne(ctx, bson.M{"taskId": e.TaskID}, opts).Decode(&last)
		e.Sequence = last.Sequence + 1
	}
	if e.ID.IsZero() {
		e.ID = bson.NewObjectID()
	}
	_, err := s.C(ColTaskEvents).InsertOne(ctx, e)
	return err
}

func (s *Store) ListTaskEvents(ctx context.Context, taskID bson.ObjectID, after int64, limit int) ([]domain.TaskEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	cur, err := s.C(ColTaskEvents).Find(ctx,
		bson.M{"taskId": taskID, "sequence": bson.M{"$gt": after}},
		options.Find().SetSort(bson.D{{Key: "sequence", Value: 1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.TaskEvent
	err = cur.All(ctx, &out)
	return out, err
}

func (s *Store) AddOutbox(ctx context.Context, routingKey string, payload []byte) error {
	now := time.Now().UTC()
	msg := domain.OutboxMessage{
		ID: bson.NewObjectID(), RoutingKey: routingKey, Payload: payload, Status: "pending",
		AvailableAt: now, CreatedAt: now,
	}
	_, err := s.C(ColOutbox).InsertOne(ctx, msg)
	return err
}

func (s *Store) PendingOutbox(ctx context.Context, limit int) ([]domain.OutboxMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	cur, err := s.C(ColOutbox).Find(ctx,
		bson.M{"status": "pending", "availableAt": bson.M{"$lte": time.Now().UTC()}},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.OutboxMessage
	err = cur.All(ctx, &out)
	return out, err
}

func (s *Store) MarkOutbox(ctx context.Context, id bson.ObjectID, status string, attempts int) error {
	fields := bson.M{"status": status, "attempts": attempts}
	if status == "published" {
		fields["publishedAt"] = time.Now().UTC()
	} else {
		fields["availableAt"] = time.Now().UTC().Add(time.Duration(attempts*attempts) * time.Second)
	}
	_, err := s.C(ColOutbox).UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": fields})
	return err
}

func (s *Store) CountTasksByStatus(ctx context.Context) (map[string]int64, error) {
	cur, err := s.C(ColTasks).Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.M{"_id": "$status", "count": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID    string `bson:"_id"`
		Count int64  `bson:"count"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[r.ID] = r.Count
	}
	return out, nil
}

func (s *Store) AcquireLease(ctx context.Context, name, owner string, ttl time.Duration) (bool, error) {
	now := time.Now().UTC()
	filter := bson.M{
		"_id": name,
		"$or": []bson.M{
			{"expiresAt": bson.M{"$lt": now}},
			{"owner": owner},
		},
	}
	update := bson.M{"$set": bson.M{"owner": owner, "expiresAt": now.Add(ttl)}}
	res, err := s.C(ColLeases).UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return false, err
	}
	return res.UpsertedID != nil || res.ModifiedCount > 0 || res.MatchedCount > 0, nil
}

func (s *Store) ReleaseLease(ctx context.Context, name, owner string) error {
	_, err := s.C(ColLeases).DeleteOne(ctx, bson.M{"_id": name, "owner": owner})
	return err
}

func (s *Store) CreateResultSet(ctx context.Context, kind string, items []string) (bson.ObjectID, error) {
	id := bson.NewObjectID()
	_, err := s.C(ColResultSets).InsertOne(ctx, bson.M{
		"_id": id, "kind": kind, "items": items, "total": len(items), "createdAt": time.Now().UTC(),
	})
	return id, err
}

func (s *Store) GetResultSet(ctx context.Context, id bson.ObjectID) ([]string, error) {
	var doc struct {
		Items []string `bson:"items"`
	}
	err := s.C(ColResultSets).FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return doc.Items, err
}

func (s *Store) ListEASMTasks(ctx context.Context, organizationID, status, kind string, q PageQuery) ([]domain.Task, int64, error) {
	q = q.Normalized()
	filter := bson.M{"origin": domain.TaskOriginEASM}
	if organizationID != "" {
		filter["organizationId"] = organizationID
	}
	if status != "" {
		filter["status"] = status
	}
	if kind != "" {
		filter["kind"] = kind
	}
	if q.Search != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": q.Search, "$options": "i"}},
			{"target": bson.M{"$regex": q.Search, "$options": "i"}},
			{"rootDomain": bson.M{"$regex": q.Search, "$options": "i"}},
		}
	}
	total, err := s.C(ColTasks).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(SortDocument(q.Sort)).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColTasks).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.Task
	err = cur.All(ctx, &out)
	return out, total, err
}
