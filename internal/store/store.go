package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

const (
	ColUsers               = "users"
	ColSessions            = "sessions"
	ColAPIKeys             = "api_keys"
	ColAudit               = "audit_events"
	ColPolicies            = "policies"
	ColScopes              = "scopes"
	ColResultSets          = "result_sets"
	ColTasks               = "tasks"
	ColTaskSteps           = "task_steps"
	ColTaskEvents          = "task_events"
	ColWorkItems           = "task_work_items"
	ColOutbox              = "outbox"
	ColLeases              = "leases"
	ColAssets              = "assets"
	ColObservations        = "asset_observations"
	ColScopeAssets         = "scope_asset_links"
	ColFindings            = "findings"
	ColFingerprints        = "fingerprints"
	ColMonitors            = "monitors"
	ColSchedules           = "schedules"
	ColMonitorRuns         = "monitor_runs"
	ColGitHubTasks         = "github_tasks"
	ColGitHubMonitors      = "github_monitors"
	ColGitHubFindings      = "github_findings"
	ColGitHubSeen          = "github_seen"
	ColExports             = "exports"
	ColPOCPlugins          = "poc_plugins"
	ColStats               = "stats"
	ColOrganizations       = "organizations"
	ColOrganizationDomains = "organization_domains"
	ColICPEvidence         = "icp_evidence"
	ColIPLocations         = "ip_locations"
	ColICPJobs             = "icp_jobs"
	ColGraphOutbox         = "graph_outbox"
)

type Store struct {
	client *mongo.Client
	db     *mongo.Database
}

func Open(ctx context.Context, uri, database string) (*Store, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(8 * time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}
	st := &Store{client: client, db: client.Database(database)}
	pingCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping mongo: %w", err)
	}
	return st, nil
}

func (s *Store) Close(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Disconnect(ctx)
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.client == nil {
		return errors.New("store is not open")
	}
	return s.client.Ping(ctx, readpref.Primary())
}

func (s *Store) DB() *mongo.Database { return s.db }

func (s *Store) C(name string) *mongo.Collection { return s.db.Collection(name) }

func (s *Store) EnsureIndexes(ctx context.Context) error {
	type spec struct {
		collection string
		indexes    []mongo.IndexModel
	}
	unique := func(keys ...string) mongo.IndexModel {
		d := bson.D{}
		for _, k := range keys {
			d = append(d, bson.E{Key: k, Value: 1})
		}
		return mongo.IndexModel{Keys: d, Options: options.Index().SetUnique(true)}
	}
	plain := func(keys ...string) mongo.IndexModel {
		d := bson.D{}
		for _, k := range keys {
			d = append(d, bson.E{Key: k, Value: 1})
		}
		return mongo.IndexModel{Keys: d}
	}
	desc := func(keys ...string) mongo.IndexModel {
		d := bson.D{}
		for _, k := range keys {
			d = append(d, bson.E{Key: k, Value: -1})
		}
		return mongo.IndexModel{Keys: d}
	}
	ttl := func(field string, seconds int32) mongo.IndexModel {
		return mongo.IndexModel{
			Keys:    bson.D{{Key: field, Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(seconds),
		}
	}
	specs := []spec{
		{ColUsers, []mongo.IndexModel{unique("username")}},
		{ColSessions, []mongo.IndexModel{unique("refreshHash"), plain("userId"), ttl("expiresAt", 0)}},
		{ColAPIKeys, []mongo.IndexModel{unique("keyHash"), plain("revokedAt")}},
		{ColAudit, []mongo.IndexModel{plain("actorId"), ttl("createdAt", 180*24*3600)}},
		{ColPolicies, []mongo.IndexModel{unique("name")}},
		{ColScopes, []mongo.IndexModel{plain("name"), plain("type")}},
		{ColTasks, []mongo.IndexModel{plain("status"), desc("createdAt"), plain("kind"), plain("scopeId"), plain("organizationId", "createdAt"), plain("rootDomain", "createdAt"), plain("origin")}},
		{ColTaskSteps, []mongo.IndexModel{unique("taskId", "sequence"), plain("taskId", "status")}},
		{ColTaskEvents, []mongo.IndexModel{plain("taskId", "sequence"), ttl("createdAt", 90*24*3600)}},
		{ColWorkItems, []mongo.IndexModel{plain("taskId", "status"), plain("leaseUntil")}},
		{ColOutbox, []mongo.IndexModel{plain("status", "availableAt")}},
		{ColLeases, []mongo.IndexModel{ttl("expiresAt", 0)}},
		{ColAssets, []mongo.IndexModel{unique("kind", "key"), plain("kind", "lastSeen"), plain("scopeIds")}},
		{ColObservations, []mongo.IndexModel{plain("taskId", "kind"), plain("taskId", "observedAt"), plain("assetId")}},
		{ColScopeAssets, []mongo.IndexModel{unique("scopeId", "assetId"), plain("scopeId", "kind", "lastSeen")}},
		{ColFindings, []mongo.IndexModel{unique("taskId", "kind", "dedupeKey"), plain("taskId", "kind"), plain("scopeId", "kind"), plain("target", "dedupeKey", "createdAt")}},
		{ColFingerprints, []mongo.IndexModel{unique("rule")}},
		{ColMonitors, []mongo.IndexModel{plain("status", "nextRunAt"), plain("scopeId", "kind")}},
		{ColSchedules, []mongo.IndexModel{plain("status", "nextRunAt")}},
		{ColMonitorRuns, []mongo.IndexModel{plain("monitorId", "startedAt")}},
		{ColGitHubTasks, []mongo.IndexModel{desc("createdAt"), plain("status")}},
		{ColGitHubMonitors, []mongo.IndexModel{plain("status", "nextRunAt")}},
		{ColGitHubFindings, []mongo.IndexModel{unique("hashMd5"), plain("monitorId", "createdAt"), plain("taskId")}},
		{ColGitHubSeen, []mongo.IndexModel{unique("hashMd5")}},
		{ColExports, []mongo.IndexModel{ttl("createdAt", 30*24*3600)}},
		{ColPOCPlugins, []mongo.IndexModel{unique("name"), plain("type", "category")}},
		{ColOrganizations, []mongo.IndexModel{unique("canonicalName", "region"), plain("status")}},
		{ColOrganizationDomains, []mongo.IndexModel{
			unique("organizationId", "rootDomain"),
			{Keys: bson.D{{Key: "rootDomain", Value: 1}}, Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"status": "confirmed"}).SetName("uniq_confirmed_root_domain")},
			plain("status"), plain("organizationId", "status"), plain("rootDomain", "status"),
		}},
		{ColICPEvidence, []mongo.IndexModel{plain("provider", "queryType", "queryKey"), ttl("expiresAt", 0)}},
		{ColIPLocations, []mongo.IndexModel{unique("ip")}},
		{ColICPJobs, []mongo.IndexModel{plain("status", "createdAt"), ttl("createdAt", 7*24*3600)}},
		{ColGraphOutbox, []mongo.IndexModel{plain("processedAt", "createdAt"), plain("processingAt")}},
	}
	for _, sp := range specs {
		if len(sp.indexes) == 0 {
			continue
		}
		if _, err := s.C(sp.collection).Indexes().CreateMany(ctx, sp.indexes); err != nil {
			return fmt.Errorf("create indexes for %s: %w", sp.collection, err)
		}
	}
	_, _ = s.C(ColAssets).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "data.title", Value: "text"}, {Key: "data.domain", Value: "text"}, {Key: "data.ip", Value: "text"}},
		Options: options.Index().SetName("asset_text"),
	})
	return nil
}

type PageQuery struct {
	Page     int
	PageSize int
	Sort     string
	Search   string
}

func (p PageQuery) Normalized() PageQuery {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 200 {
		p.PageSize = 200
	}
	if p.Sort == "" {
		p.Sort = "createdAt:desc"
	}
	return p
}

func SortDocument(sort string) bson.D {
	if sort == "" {
		return bson.D{{Key: "_id", Value: -1}}
	}
	field, order := sort, "desc"
	for i, c := range sort {
		if c == ':' {
			field, order = sort[:i], sort[i+1:]
			break
		}
	}
	switch field {
	case "_id", "createdAt", "updatedAt", "lastSeen", "nextRunAt", "status", "name", "kind", "progress":
	default:
		field = "createdAt"
	}
	v := -1
	if order == "asc" {
		v = 1
	}
	out := bson.D{{Key: field, Value: v}}
	if field != "_id" {
		out = append(out, bson.E{Key: "_id", Value: -1})
	}
	return out
}

func ObjectID(id string) (bson.ObjectID, error) {
	if id == "" {
		return bson.NilObjectID, errors.New("empty object id")
	}
	return bson.ObjectIDFromHex(id)
}
