package store

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/red-copilot/red-x/internal/domain"
)

func newPostgresTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("ARL_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set ARL_TEST_POSTGRES_DSN to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	schema := fmt.Sprintf("arl_test_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		_ = admin.Close(context.Background())
		t.Fatalf("create test schema: %v", err)
	}
	st, err := Open(ctx, Options{DSN: dsnWithSearchPath(dsn, schema), MaxOpenConns: 10, MaxIdleConns: 5, ConnMaxLifetime: time.Minute})
	if err != nil {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = admin.Close(context.Background())
		t.Fatalf("open test store: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		_ = st.Close(context.Background())
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = admin.Close(context.Background())
		t.Fatalf("migrate test store: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close(context.Background())
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = admin.Close(context.Background())
	})
	return st, ctx
}

func dsnWithSearchPath(dsn, schema string) string {
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" {
		q := u.Query()
		q.Set("search_path", schema+",public")
		u.RawQuery = q.Encode()
		return u.String()
	}
	return dsn + " search_path=" + schema + ",public"
}

func TestUpsertAssetWithScope(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	scopeID := domain.MustID()
	if err := st.CreateScope(ctx, &domain.Scope{ID: scopeID, Name: "test", Type: "domain", Targets: []string{"example.com"}}); err != nil {
		t.Fatal(err)
	}
	taskID := domain.MustID()
	if err := st.CreateTask(ctx, &domain.Task{ID: taskID, Name: "test", Kind: domain.TaskDomain, Target: "example.com", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		asset, err := st.UpsertAsset(ctx, domain.AssetDomain, "example.com", map[string]any{"domain": "example.com"}, taskID, scopeID)
		if err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
		if err := st.ObserveAsset(ctx, taskID, asset, "test", map[string]any{"domain": "example.com"}); err != nil {
			t.Fatal(err)
		}
		if err := st.LinkAssetToScope(ctx, scopeID, asset, []string{"smoke"}); err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := st.ListScopeAssets(ctx, scopeID, "domain", PageQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("unexpected scope assets: total=%d len=%d", total, len(items))
	}
}

func TestTaskEventSequenceIsAtomic(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	taskID := domain.MustID()
	if err := st.CreateTask(ctx, &domain.Task{ID: taskID, Name: "events", Kind: domain.TaskDomain, Target: "example.com", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	const total = 24
	done := make(chan error, total)
	for i := 0; i < total; i++ {
		go func() {
			done <- st.AppendTaskEvent(ctx, &domain.TaskEvent{TaskID: taskID, Type: "test", Message: "event"})
		}()
	}
	for i := 0; i < total; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	events, err := st.ListTaskEvents(ctx, taskID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != total {
		t.Fatalf("events = %d, want %d", len(events), total)
	}
	for i, event := range events {
		if event.Sequence != int64(i+1) {
			t.Fatalf("event[%d].sequence = %d", i, event.Sequence)
		}
	}
}

func TestAcquireLeaseIsExclusive(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	ok, err := st.AcquireLease(ctx, "scheduler", "a", time.Minute)
	if err != nil || !ok {
		t.Fatalf("first acquire = %v, %v", ok, err)
	}
	ok, err = st.AcquireLease(ctx, "scheduler", "b", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("second owner acquired an active lease")
	}
	ok, err = st.AcquireLease(ctx, "scheduler", "a", time.Minute)
	if err != nil || !ok {
		t.Fatalf("owner renew = %v, %v", ok, err)
	}
}

func TestPruneExpired(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	old := time.Now().UTC().Add(-200 * 24 * time.Hour)
	userID := domain.MustID()
	if err := st.CreateUser(ctx, &domain.User{ID: userID, Username: "retention", Role: domain.RoleViewer}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, &domain.Session{UserID: userID, RefreshHash: "expired", ExpiresAt: old}); err != nil {
		t.Fatal(err)
	}
	st.AddAudit(ctx, domain.AuditEvent{Actor: "retention", Action: "old"})
	if err := st.db.WithContext(ctx).Exec(`UPDATE audit_events SET created_at = ? WHERE actor = ?`, old, "retention").Error; err != nil {
		t.Fatal(err)
	}
	if err := st.PruneExpired(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var sessions, audits int64
	_ = st.db.WithContext(ctx).Model(&domain.Session{}).Count(&sessions).Error
	_ = st.db.WithContext(ctx).Model(&domain.AuditEvent{}).Count(&audits).Error
	if sessions != 0 || audits != 0 {
		t.Fatalf("sessions=%d audits=%d, want 0", sessions, audits)
	}
}

func TestMigrateIsConcurrentAndIndexesExist(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() { errs <- st.Migrate(ctx) }()
	}
	for i := 0; i < 4; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	var indexes []string
	if err := st.db.WithContext(ctx).Raw(`SELECT indexname FROM pg_indexes WHERE schemaname = current_schema()`).Scan(&indexes).Error; err != nil {
		t.Fatal(err)
	}
	required := map[string]bool{
		"uniq_active_confirmed_root_domain": false,
		"uniq_assets_kind_key":              false,
		"uniq_findings_task_kind_dedupe":    false,
		"idx_assets_key_trgm":               false,
		"idx_organizations_name_trgm":       false,
	}
	for _, name := range indexes {
		if _, ok := required[name]; ok {
			required[name] = true
		}
	}
	for name, found := range required {
		if !found {
			t.Fatalf("required index %s missing", name)
		}
	}
}

func TestPendingOutboxIsClaimedOnce(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	if err := st.AddOutbox(ctx, "q.maintenance", []byte(`{"one":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := st.AddOutbox(ctx, "q.maintenance", []byte(`{"two":true}`)); err != nil {
		t.Fatal(err)
	}
	type result struct {
		items []domain.OutboxMessage
		err   error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			items, err := st.PendingOutbox(ctx, 10)
			results <- result{items: items, err: err}
		}()
	}
	total := 0
	for i := 0; i < 2; i++ {
		got := <-results
		if got.err != nil {
			t.Fatal(got.err)
		}
		total += len(got.items)
	}
	if total != 2 {
		t.Fatalf("claimed %d outbox messages, want 2", total)
	}
}
