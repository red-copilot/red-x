package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Aabyss-Team/ARL/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *Store) CountUsers(ctx context.Context) (int64, error) {
	return s.C(ColUsers).CountDocuments(ctx, bson.M{})
}

func (s *Store) CreateUser(ctx context.Context, u *domain.User) error {
	now := time.Now().UTC()
	u.CreatedAt, u.UpdatedAt, u.SchemaVersion = now, now, domain.SchemaVersion
	if u.ID.IsZero() {
		u.ID = bson.NewObjectID()
	}
	_, err := s.C(ColUsers).InsertOne(ctx, u)
	return err
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	var u domain.User
	err := s.C(ColUsers).FindOne(ctx, bson.M{"username": strings.TrimSpace(username)}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &u, err
}

func (s *Store) GetUserByID(ctx context.Context, id bson.ObjectID) (*domain.User, error) {
	var u domain.User
	err := s.C(ColUsers).FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &u, err
}

func (s *Store) UpdateUser(ctx context.Context, u *domain.User) error {
	u.UpdatedAt = time.Now().UTC()
	_, err := s.C(ColUsers).ReplaceOne(ctx, bson.M{"_id": u.ID}, u)
	return err
}

func (s *Store) ListUsers(ctx context.Context, q PageQuery) ([]domain.User, int64, error) {
	q = q.Normalized()
	filter := bson.M{}
	if q.Search != "" {
		filter["username"] = bson.M{"$regex": q.Search, "$options": "i"}
	}
	total, err := s.C(ColUsers).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(SortDocument(q.Sort)).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColUsers).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.User
	if err := cur.All(ctx, &out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *Store) DeleteUser(ctx context.Context, id bson.ObjectID) error {
	_, err := s.C(ColUsers).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (s *Store) CreateSession(ctx context.Context, sess *domain.Session) error {
	if sess.ID.IsZero() {
		sess.ID = bson.NewObjectID()
	}
	sess.CreatedAt = time.Now().UTC()
	sess.SchemaVersion = domain.SchemaVersion
	_, err := s.C(ColSessions).InsertOne(ctx, sess)
	return err
}

func (s *Store) GetSessionByHash(ctx context.Context, hash string) (*domain.Session, error) {
	var sess domain.Session
	err := s.C(ColSessions).FindOne(ctx, bson.M{"refreshHash": hash}).Decode(&sess)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &sess, err
}

func (s *Store) RotateSession(ctx context.Context, id bson.ObjectID, newHash string, expires time.Time) error {
	now := time.Now().UTC()
	_, err := s.C(ColSessions).UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{
		"refreshHash": newHash, "expiresAt": expires, "rotatedAt": now,
	}})
	return err
}

func (s *Store) RevokeSession(ctx context.Context, id bson.ObjectID) error {
	now := time.Now().UTC()
	_, err := s.C(ColSessions).UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"revokedAt": now}})
	return err
}

func (s *Store) RevokeUserSessions(ctx context.Context, userID bson.ObjectID) error {
	now := time.Now().UTC()
	_, err := s.C(ColSessions).UpdateMany(ctx, bson.M{"userId": userID, "revokedAt": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"revokedAt": now}})
	return err
}

func (s *Store) CreateAPIKey(ctx context.Context, key *domain.APIKey) error {
	if key.ID.IsZero() {
		key.ID = bson.NewObjectID()
	}
	key.CreatedAt = time.Now().UTC()
	key.SchemaVersion = domain.SchemaVersion
	_, err := s.C(ColAPIKeys).InsertOne(ctx, key)
	return err
}

func (s *Store) GetAPIKeyByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	var key domain.APIKey
	err := s.C(ColAPIKeys).FindOne(ctx, bson.M{"keyHash": hash, "revokedAt": bson.M{"$exists": false}}).Decode(&key)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &key, err
}

func (s *Store) TouchAPIKey(ctx context.Context, id bson.ObjectID) {
	now := time.Now().UTC()
	_, _ = s.C(ColAPIKeys).UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"lastUsedAt": now}})
}

func (s *Store) ListAPIKeys(ctx context.Context) ([]domain.APIKey, error) {
	cur, err := s.C(ColAPIKeys).Find(ctx, bson.M{"revokedAt": bson.M{"$exists": false}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.APIKey
	err = cur.All(ctx, &out)
	return out, err
}

func (s *Store) RevokeAPIKey(ctx context.Context, id bson.ObjectID) error {
	now := time.Now().UTC()
	_, err := s.C(ColAPIKeys).UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"revokedAt": now}})
	return err
}

func (s *Store) AddAudit(ctx context.Context, e domain.AuditEvent) {
	e.CreatedAt = time.Now().UTC()
	_, _ = s.C(ColAudit).InsertOne(ctx, e)
}

func (s *Store) ListAudit(ctx context.Context, q PageQuery) ([]domain.AuditEvent, int64, error) {
	q = q.Normalized()
	total, err := s.C(ColAudit).CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(SortDocument(q.Sort)).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColAudit).Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.AuditEvent
	err = cur.All(ctx, &out)
	return out, total, err
}
