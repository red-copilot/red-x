package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	stdlog "log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/red-copilot/red-x/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const schemaLockID int64 = 0x41524c5047 // "ARLPG"

type Options struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type Store struct {
	db    *gorm.DB
	sqlDB *sql.DB
}

func Open(ctx context.Context, opts Options) (*Store, error) {
	if strings.TrimSpace(opts.DSN) == "" {
		return nil, errors.New("postgres dsn is required")
	}
	gormLogger := logger.New(stdlog.New(os.Stderr, "", stdlog.LstdFlags), logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	})
	db, err := gorm.Open(postgres.Open(opts.DSN), &gorm.Config{
		TranslateError: true,
		Logger:         gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("postgres sql pool: %w", err)
	}
	if opts.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(opts.MaxOpenConns)
	}
	if opts.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(opts.MaxIdleConns)
	}
	if opts.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(opts.ConnMaxLifetime)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{db: db.WithContext(ctx), sqlDB: sqlDB}, nil
}

func (s *Store) Close(context.Context) error {
	if s == nil || s.sqlDB == nil {
		return nil
	}
	return s.sqlDB.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.sqlDB == nil {
		return errors.New("store is not open")
	}
	return s.sqlDB.PingContext(ctx)
}

// Migrate applies the idempotent GORM schema and PostgreSQL-specific indexes.
// The advisory lock serializes concurrent role starts against the same database.
func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("store is not open")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(?)`, schemaLockID).Error; err != nil {
			return fmt.Errorf("acquire schema lock: %w", err)
		}
		if err := tx.AutoMigrate(
			&domain.User{}, &domain.Session{}, &domain.APIKey{}, &domain.AuditEvent{},
			&domain.Policy{}, &domain.Scope{},
			&domain.Task{}, &domain.TaskStep{}, &domain.TaskEvent{},
			&domain.OutboxMessage{}, &domain.Lease{}, &ResultSet{},
			&domain.Asset{}, &domain.AssetObservation{}, &domain.ScopeAssetLink{}, &domain.Finding{},
			&FingerprintRule{}, &domain.PocPlugin{},
			&domain.Monitor{}, &domain.Schedule{}, &domain.MonitorRun{},
			&domain.GitHubTask{}, &domain.GitHubMonitor{}, &domain.GitHubFinding{}, &domain.ExportJob{},
			&domain.Organization{}, &domain.OrganizationDomain{}, &domain.ICPEvidence{},
			&domain.IPLocation{}, &domain.ICPJob{}, &domain.GraphOutbox{},
		); err != nil {
			return fmt.Errorf("automigrate postgres: %w", err)
		}
		if err := tx.Exec(`CREATE EXTENSION IF NOT EXISTS pg_trgm SCHEMA public`).Error; err != nil {
			return fmt.Errorf("enable pg_trgm: %w", err)
		}
		var trgmSchema string
		if err := tx.Raw(`SELECT n.nspname FROM pg_extension e JOIN pg_namespace n ON n.oid = e.extnamespace WHERE e.extname = 'pg_trgm'`).Scan(&trgmSchema).Error; err != nil {
			return fmt.Errorf("locate pg_trgm schema: %w", err)
		}
		trgmOpclass := pgx.Identifier{trgmSchema}.Sanitize() + ".gin_trgm_ops"
		for _, stmt := range postgresIndexes {
			if err := tx.Exec(strings.ReplaceAll(stmt, "__TRGM__", trgmOpclass)).Error; err != nil {
				return fmt.Errorf("create postgres index: %w", err)
			}
		}
		return nil
	})
}

var postgresIndexes = []string{
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_username ON users (username)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_sessions_refresh_hash ON sessions (refresh_hash)`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions (user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions (expires_at)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_api_keys_key_hash ON api_keys (key_hash)`,
	`CREATE INDEX IF NOT EXISTS idx_api_keys_revoked_at ON api_keys (revoked_at)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_events_actor_id ON audit_events (actor_id)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_events_created_at ON audit_events (created_at)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_policies_name ON policies (name)`,
	`CREATE INDEX IF NOT EXISTS idx_scopes_name ON scopes (name)`,
	`CREATE INDEX IF NOT EXISTS idx_scopes_type ON scopes (type)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks (status)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks (created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_kind ON tasks (kind)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_scope_id ON tasks (scope_id)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_org_created ON tasks (organization_id, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_root_created ON tasks (root_domain, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_origin ON tasks (origin)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_task_steps_task_sequence ON task_steps (task_id, sequence)`,
	`CREATE INDEX IF NOT EXISTS idx_task_steps_task_status ON task_steps (task_id, status)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_task_events_task_sequence ON task_events (task_id, sequence)`,
	`CREATE INDEX IF NOT EXISTS idx_task_events_created_at ON task_events (created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox (status, available_at)`,
	`CREATE INDEX IF NOT EXISTS idx_leases_expires_at ON leases (expires_at)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_assets_kind_key ON assets (kind, key)`,
	`CREATE INDEX IF NOT EXISTS idx_assets_kind_last_seen ON assets (kind, last_seen DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_assets_scope_ids ON assets USING gin (scope_ids)`,
	`CREATE INDEX IF NOT EXISTS idx_asset_observations_task_kind ON asset_observations (task_id, kind)`,
	`CREATE INDEX IF NOT EXISTS idx_asset_observations_task_time ON asset_observations (task_id, observed_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_asset_observations_asset_id ON asset_observations (asset_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_scope_asset_links ON scope_asset_links (scope_id, asset_id)`,
	`CREATE INDEX IF NOT EXISTS idx_scope_asset_links_kind_time ON scope_asset_links (scope_id, kind, last_seen DESC)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_findings_task_kind_dedupe ON findings (task_id, kind, dedupe_key)`,
	`CREATE INDEX IF NOT EXISTS idx_findings_task_kind ON findings (task_id, kind)`,
	`CREATE INDEX IF NOT EXISTS idx_findings_scope_kind ON findings (scope_id, kind)`,
	`CREATE INDEX IF NOT EXISTS idx_findings_target_created ON findings (target, dedupe_key, created_at DESC)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_fingerprints_rule ON fingerprints (rule)`,
	`CREATE INDEX IF NOT EXISTS idx_poc_plugins_type_category ON poc_plugins (type, category)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_poc_plugins_name ON poc_plugins (name)`,
	`CREATE INDEX IF NOT EXISTS idx_monitors_status_next_run ON monitors (status, next_run_at)`,
	`CREATE INDEX IF NOT EXISTS idx_monitors_scope_kind ON monitors (scope_id, kind)`,
	`CREATE INDEX IF NOT EXISTS idx_schedules_status_next_run ON schedules (status, next_run_at)`,
	`CREATE INDEX IF NOT EXISTS idx_monitor_runs_monitor_started ON monitor_runs (monitor_id, started_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_github_tasks_created_at ON github_tasks (created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_github_tasks_status ON github_tasks (status)`,
	`CREATE INDEX IF NOT EXISTS idx_github_monitors_status_next_run ON github_monitors (status, next_run_at)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_github_findings_hash ON github_findings (hash_md5)`,
	`CREATE INDEX IF NOT EXISTS idx_github_findings_monitor_created ON github_findings (monitor_id, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_github_findings_task_id ON github_findings (task_id)`,
	`CREATE INDEX IF NOT EXISTS idx_exports_created_at ON exports (created_at)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_organizations_name_region ON organizations (canonical_name, region)`,
	`CREATE INDEX IF NOT EXISTS idx_organizations_status ON organizations (status)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_organization_domains_org_root ON organization_domains (organization_id, root_domain)`,
	`DROP INDEX IF EXISTS uniq_confirmed_root_domain`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_active_confirmed_root_domain ON organization_domains (root_domain) WHERE status = 'confirmed' AND deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_organization_domains_status ON organization_domains (status)`,
	`CREATE INDEX IF NOT EXISTS idx_organization_domains_org_status ON organization_domains (organization_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_organization_domains_root_status ON organization_domains (root_domain, status)`,
	`CREATE INDEX IF NOT EXISTS idx_icp_evidence_query ON icp_evidence (provider, query_type, query_key)`,
	`CREATE INDEX IF NOT EXISTS idx_icp_evidence_expires_at ON icp_evidence (expires_at)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_ip_locations_ip ON ip_locations (ip)`,
	`CREATE INDEX IF NOT EXISTS idx_icp_jobs_status_created ON icp_jobs (status, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_graph_outbox_processed_created ON graph_outbox (processed_at, created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_graph_outbox_processing_at ON graph_outbox (processing_at)`,
	`CREATE INDEX IF NOT EXISTS idx_assets_key_trgm ON assets USING gin (key __TRGM__)`,
	`CREATE INDEX IF NOT EXISTS idx_assets_title_trgm ON assets USING gin ((data->>'title') __TRGM__)`,
	`CREATE INDEX IF NOT EXISTS idx_assets_url_trgm ON assets USING gin ((data->>'url') __TRGM__)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_name_trgm ON tasks USING gin (name __TRGM__)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_target_trgm ON tasks USING gin (target __TRGM__)`,
	`CREATE INDEX IF NOT EXISTS idx_findings_title_trgm ON findings USING gin (title __TRGM__)`,
	`CREATE INDEX IF NOT EXISTS idx_findings_target_trgm ON findings USING gin (target __TRGM__)`,
	`CREATE INDEX IF NOT EXISTS idx_organizations_name_trgm ON organizations USING gin (canonical_name __TRGM__)`,
	`CREATE INDEX IF NOT EXISTS idx_organizations_aliases ON organizations USING gin (aliases)`,
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

func OrderClause(sort string) string {
	field, direction := sort, "desc"
	if i := strings.IndexByte(sort, ':'); i >= 0 {
		field, direction = sort[:i], strings.ToLower(sort[i+1:])
	}
	columns := map[string]string{
		"id": "id", "_id": "id", "createdAt": "created_at", "updatedAt": "updated_at",
		"lastSeen": "last_seen", "nextRunAt": "next_run_at", "status": "status",
		"name": "name", "kind": "kind", "progress": "progress",
	}
	column, ok := columns[field]
	if !ok {
		column = "created_at"
	}
	if direction != "asc" {
		direction = "desc"
	}
	order := column + " " + strings.ToUpper(direction)
	if column != "id" {
		order += ", id DESC"
	}
	return order
}

func ParseID(id string) (string, error) {
	return domain.ParseID(id)
}

func ilike(query string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(query) + "%"
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func isDuplicate(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func jsonbExpr(v any) clause.Expr {
	raw, _ := json.Marshal(v)
	return gorm.Expr("?::jsonb", string(raw))
}
