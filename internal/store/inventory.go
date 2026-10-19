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

func (s *Store) UpsertAsset(ctx context.Context, kind domain.AssetKind, key string, data map[string]any, taskID string, scopeID string) (*domain.Asset, error) {
	now := time.Now().UTC()
	scopeIDs := []string{}
	if scopeID != "" {
		scopeIDs = append(scopeIDs, scopeID)
	}
	filter := bson.M{"kind": kind, "key": key}
	update := bson.M{
		"$set": bson.M{
			"data": data, "lastSeen": now, "lastTaskId": taskID, "schemaVersion": domain.SchemaVersion,
		},
		"$setOnInsert": bson.M{"firstSeen": now},
	}
	if scopeID != "" {
		update["$addToSet"] = bson.M{"scopeIds": scopeID}
	} else {
		update["$setOnInsert"].(bson.M)["scopeIds"] = scopeIDs
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var asset domain.Asset
	err := s.C(ColAssets).FindOneAndUpdate(ctx, filter, update, opts).Decode(&asset)
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func (s *Store) ObserveAsset(ctx context.Context, taskID bson.ObjectID, asset *domain.Asset, source string, snapshot map[string]any) error {
	obs := domain.AssetObservation{
		ID: bson.NewObjectID(), TaskID: taskID, AssetID: asset.ID, Kind: asset.Kind, Key: asset.Key,
		Source: source, Snapshot: snapshot, ObservedAt: time.Now().UTC(), SchemaVersion: domain.SchemaVersion,
	}
	_, err := s.C(ColObservations).InsertOne(ctx, obs)
	return err
}

func (s *Store) AddFinding(ctx context.Context, f *domain.Finding) (bool, error) {
	f.CreatedAt = time.Now().UTC()
	f.SchemaVersion = domain.SchemaVersion
	if f.ID.IsZero() {
		f.ID = bson.NewObjectID()
	}
	_, err := s.C(ColFindings).InsertOne(ctx, f)
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) ListAssets(ctx context.Context, kind, taskID, scopeID string, q PageQuery) ([]domain.Asset, int64, error) {
	q = q.Normalized()
	filter := bson.M{}
	if kind != "" {
		filter["kind"] = kind
	}
	if taskID != "" {
		ids, err := s.assetIDsForTask(ctx, taskID)
		if err != nil {
			return nil, 0, err
		}
		filter["_id"] = bson.M{"$in": ids}
	}
	if scopeID != "" {
		filter["scopeIds"] = scopeID
	}
	if q.Search != "" {
		filter["$or"] = []bson.M{
			{"key": bson.M{"$regex": q.Search, "$options": "i"}},
			{"data.title": bson.M{"$regex": q.Search, "$options": "i"}},
			{"data.url": bson.M{"$regex": q.Search, "$options": "i"}},
		}
	}
	total, err := s.C(ColAssets).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(SortDocument(q.Sort)).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColAssets).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.Asset
	err = cur.All(ctx, &out)
	return out, total, err
}

func (s *Store) assetIDsForTask(ctx context.Context, taskID string) ([]bson.ObjectID, error) {
	id, err := ObjectID(taskID)
	if err != nil {
		return nil, err
	}
	cur, err := s.C(ColObservations).Find(ctx, bson.M{"taskId": id}, options.Find().SetProjection(bson.M{"assetId": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	seen := map[bson.ObjectID]bool{}
	var ids []bson.ObjectID
	for cur.Next(ctx) {
		var row struct {
			AssetID bson.ObjectID `bson:"assetId"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, err
		}
		if !seen[row.AssetID] {
			seen[row.AssetID] = true
			ids = append(ids, row.AssetID)
		}
	}
	return ids, cur.Err()
}

func (s *Store) ListTaskObservations(ctx context.Context, taskID bson.ObjectID, kind string, q PageQuery) ([]domain.AssetObservation, int64, error) {
	q = q.Normalized()
	filter := bson.M{"taskId": taskID}
	if kind != "" {
		filter["kind"] = kind
	}
	if q.Search != "" {
		filter["key"] = bson.M{"$regex": q.Search, "$options": "i"}
	}
	total, err := s.C(ColObservations).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(bson.D{{Key: "observedAt", Value: -1}}).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColObservations).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.AssetObservation
	err = cur.All(ctx, &out)
	return out, total, err
}

func (s *Store) ListFindings(ctx context.Context, kind, taskID, scopeID string, q PageQuery) ([]domain.Finding, int64, error) {
	q = q.Normalized()
	filter := bson.M{}
	if kind != "" {
		filter["kind"] = kind
	}
	if taskID != "" {
		id, err := ObjectID(taskID)
		if err != nil {
			return nil, 0, err
		}
		filter["taskId"] = id
	}
	if scopeID != "" {
		filter["scopeId"] = scopeID
	}
	if q.Search != "" {
		filter["$or"] = []bson.M{
			{"title": bson.M{"$regex": q.Search, "$options": "i"}},
			{"target": bson.M{"$regex": q.Search, "$options": "i"}},
		}
	}
	total, err := s.C(ColFindings).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(SortDocument(q.Sort)).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColFindings).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.Finding
	err = cur.All(ctx, &out)
	return out, total, err
}

func (s *Store) TaskStatistics(ctx context.Context, taskID bson.ObjectID) (domain.TaskStatistics, error) {
	var st domain.TaskStatistics
	countObs := func(kind domain.AssetKind) (int, error) {
		n, err := s.C(ColObservations).CountDocuments(ctx, bson.M{"taskId": taskID, "kind": kind})
		return int(n), err
	}
	countFind := func(kind domain.FindingKind) (int, error) {
		n, err := s.C(ColFindings).CountDocuments(ctx, bson.M{"taskId": taskID, "kind": kind})
		return int(n), err
	}
	var err error
	if st.Domains, err = countObs(domain.AssetDomain); err != nil {
		return st, err
	}
	if st.IPs, err = countObs(domain.AssetIP); err != nil {
		return st, err
	}
	if st.Sites, err = countObs(domain.AssetSite); err != nil {
		return st, err
	}
	if st.URLs, err = countObs(domain.AssetURL); err != nil {
		return st, err
	}
	if st.Certs, err = countObs(domain.AssetCert); err != nil {
		return st, err
	}
	if st.Services, err = countObs(domain.AssetService); err != nil {
		return st, err
	}
	if st.Vulns, err = countFind(domain.FindingVuln); err != nil {
		return st, err
	}
	if st.Nuclei, err = countFind(domain.FindingNuclei); err != nil {
		return st, err
	}
	if st.WIH, err = countFind(domain.FindingWIH); err != nil {
		return st, err
	}
	if st.FileLeaks, err = countFind(domain.FindingFileLeak); err != nil {
		return st, err
	}
	if st.Protocols, err = countFind(domain.FindingProtocol); err != nil {
		return st, err
	}
	return st, nil
}

func (s *Store) CreateScope(ctx context.Context, sc *domain.Scope) error {
	now := time.Now().UTC()
	sc.CreatedAt, sc.UpdatedAt, sc.SchemaVersion = now, now, domain.SchemaVersion
	if sc.ID.IsZero() {
		sc.ID = bson.NewObjectID()
	}
	_, err := s.C(ColScopes).InsertOne(ctx, sc)
	return err
}

func (s *Store) GetScope(ctx context.Context, id bson.ObjectID) (*domain.Scope, error) {
	var sc domain.Scope
	err := s.C(ColScopes).FindOne(ctx, bson.M{"_id": id}).Decode(&sc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &sc, err
}

func (s *Store) ListScopes(ctx context.Context, q PageQuery) ([]domain.Scope, int64, error) {
	q = q.Normalized()
	filter := bson.M{}
	if q.Search != "" {
		filter["name"] = bson.M{"$regex": q.Search, "$options": "i"}
	}
	total, err := s.C(ColScopes).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(SortDocument(q.Sort)).SetSkip(int64((q.Page - 1) * q.PageSize)).SetLimit(int64(q.PageSize))
	cur, err := s.C(ColScopes).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.Scope
	err = cur.All(ctx, &out)
	return out, total, err
}

func (s *Store) UpdateScope(ctx context.Context, sc *domain.Scope) error {
	sc.UpdatedAt = time.Now().UTC()
	_, err := s.C(ColScopes).ReplaceOne(ctx, bson.M{"_id": sc.ID}, sc)
	return err
}

func (s *Store) DeleteScope(ctx context.Context, id bson.ObjectID) error {
	_, err := s.C(ColScopes).DeleteOne(ctx, bson.M{"_id": id})
	if err == nil {
		_, _ = s.C(ColScopeAssets).DeleteMany(ctx, bson.M{"scopeId": id})
	}
	return err
}

func (s *Store) LinkAssetToScope(ctx context.Context, scopeID bson.ObjectID, asset *domain.Asset, tags []string) error {
	now := time.Now().UTC()
	filter := bson.M{"scopeId": scopeID, "assetId": asset.ID}
	update := bson.M{
		"$set":         bson.M{"kind": asset.Kind, "lastSeen": now, "schemaVersion": domain.SchemaVersion},
		"$setOnInsert": bson.M{"firstSeen": now},
	}
	if len(tags) > 0 {
		update["$addToSet"] = bson.M{"tags": bson.M{"$each": tags}}
	}
	_, err := s.C(ColScopeAssets).UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return err
}

func (s *Store) ListScopeAssets(ctx context.Context, scopeID bson.ObjectID, kind string, q PageQuery) ([]map[string]any, int64, error) {
	q = q.Normalized()
	filter := bson.M{"scopeId": scopeID}
	if kind != "" {
		filter["kind"] = kind
	}
	total, err := s.C(ColScopeAssets).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$sort", Value: bson.D{{Key: "lastSeen", Value: -1}}}},
		{{Key: "$skip", Value: int64((q.Page - 1) * q.PageSize)}},
		{{Key: "$limit", Value: int64(q.PageSize)}},
		{{Key: "$lookup", Value: bson.M{"from": ColAssets, "localField": "assetId", "foreignField": "_id", "as": "asset"}}},
		{{Key: "$unwind", Value: "$asset"}},
		{{Key: "$project", Value: bson.M{"_id": 0, "kind": 1, "tags": 1, "firstSeen": 1, "lastSeen": 1, "asset": 1}}},
	}
	cur, err := s.C(ColScopeAssets).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []map[string]any
	err = cur.All(ctx, &out)
	return out, total, err
}

func (s *Store) SearchHistoryDomains(ctx context.Context, base string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5000
	}
	filter := bson.M{"kind": domain.AssetDomain, "key": bson.M{"$regex": strings.ToLower(base) + "$", "$options": "i"}}
	cur, err := s.C(ColAssets).Find(ctx, filter, options.Find().SetLimit(int64(limit)).SetProjection(bson.M{"key": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []string
	for cur.Next(ctx) {
		var row struct {
			Key string `bson:"key"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, err
		}
		out = append(out, row.Key)
	}
	return out, cur.Err()
}

func (s *Store) CountAssets(ctx context.Context) (map[string]int64, error) {
	cur, err := s.C(ColAssets).Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.M{"_id": "$kind", "count": bson.M{"$sum": 1}}}},
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

type FingerprintRule struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string        `bson:"name" json:"name"`
	Rule      string        `bson:"rule" json:"rule"`
	CreatedAt time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time     `bson:"updatedAt" json:"updatedAt"`
}

func (s *Store) CreateFingerprint(ctx context.Context, r *FingerprintRule) error {
	now := time.Now().UTC()
	r.CreatedAt, r.UpdatedAt = now, now
	if r.ID.IsZero() {
		r.ID = bson.NewObjectID()
	}
	_, err := s.C(ColFingerprints).InsertOne(ctx, r)
	return err
}

func (s *Store) ListFingerprints(ctx context.Context) ([]FingerprintRule, error) {
	cur, err := s.C(ColFingerprints).Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []FingerprintRule
	err = cur.All(ctx, &out)
	return out, err
}

func (s *Store) DeleteFingerprint(ctx context.Context, id bson.ObjectID) error {
	_, err := s.C(ColFingerprints).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (s *Store) UpsertPOCPlugin(ctx context.Context, p *domain.PocPlugin) error {
	now := time.Now().UTC()
	p.CreatedAt, p.UpdatedAt, p.SchemaVersion = now, now, domain.SchemaVersion
	_, err := s.C(ColPOCPlugins).UpdateOne(ctx,
		bson.M{"name": p.Name},
		bson.M{"$set": p},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (s *Store) ListPOCPlugins(ctx context.Context, kind string) ([]domain.PocPlugin, error) {
	filter := bson.M{}
	if kind != "" {
		filter["type"] = kind
	}
	cur, err := s.C(ColPOCPlugins).Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "type", Value: 1}, {Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.PocPlugin
	err = cur.All(ctx, &out)
	return out, err
}

func (s *Store) DeletePOCPlugin(ctx context.Context, name string) error {
	_, err := s.C(ColPOCPlugins).DeleteOne(ctx, bson.M{"name": name})
	return err
}
