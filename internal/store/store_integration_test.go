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
	var sessions, audits, sessionTokens int64
	_ = st.db.WithContext(ctx).Model(&domain.Session{}).Count(&sessions).Error
	_ = st.db.WithContext(ctx).Model(&domain.AuditEvent{}).Count(&audits).Error
	_ = st.db.WithContext(ctx).Model(&domain.SessionToken{}).Count(&sessionTokens).Error
	if sessions != 0 || audits != 0 || sessionTokens != 0 {
		t.Fatalf("sessions=%d audits=%d sessionTokens=%d, want 0", sessions, audits, sessionTokens)
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

func TestListAssetsDefaultOrderUsesLastSeen(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	taskID := domain.MustID()
	if err := st.CreateTask(ctx, &domain.Task{ID: taskID, Name: "assets", Kind: domain.TaskDomain, Target: "example.com", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertAsset(ctx, domain.AssetDomain, "first.example.com", map[string]any{"domain": "first.example.com"}, taskID, ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	second, err := st.UpsertAsset(ctx, domain.AssetDomain, "second.example.com", map[string]any{"domain": "second.example.com"}, taskID, "")
	if err != nil {
		t.Fatal(err)
	}
	items, total, err := st.ListAssets(ctx, "", "", "", PageQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list assets with default sort: %v", err)
	}
	if total != 2 || len(items) != 2 || items[0].ID != second.ID {
		t.Fatalf("assets total=%d items=%+v want newest asset first", total, items)
	}
}

func TestClaimStepIsExclusiveAndReclaimableAfterLease(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	task := &domain.Task{Name: "step claim", Kind: domain.TaskDomain, Target: "example.com", Status: domain.TaskQueued}
	if err := st.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	step := domain.TaskStep{TaskID: task.ID, Sequence: 1, Kind: "http", Status: domain.StepQueued}
	if err := st.CreateSteps(ctx, []domain.TaskStep{step}); err != nil {
		t.Fatal(err)
	}
	steps, err := st.ListSteps(ctx, task.ID)
	if err != nil || len(steps) != 1 {
		t.Fatalf("steps=%+v err=%v", steps, err)
	}
	stepID := steps[0].ID

	claimed, err := st.ClaimStep(ctx, task.ID, stepID, "owner-a", 80*time.Millisecond)
	if err != nil || !claimed {
		t.Fatalf("first claim = %v, %v", claimed, err)
	}
	claimed, err = st.ClaimStep(ctx, task.ID, stepID, "owner-b", 80*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("second owner claimed an active step lease")
	}
	failure := "previous attempt"
	if updated, err := st.UpdateClaimedStepFields(ctx, stepID, "owner-a", StepPatch{Error: &failure}); err != nil || !updated {
		t.Fatalf("record previous failure = %v, %v", updated, err)
	}
	time.Sleep(120 * time.Millisecond)
	claimed, err = st.ClaimStep(ctx, task.ID, stepID, "owner-b", time.Minute)
	if err != nil || !claimed {
		t.Fatalf("reclaim after expiry = %v, %v", claimed, err)
	}
	reclaimed, err := st.GetStep(ctx, stepID)
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed.Error != "" {
		t.Fatalf("reclaimed step retained stale error %q", reclaimed.Error)
	}
	if updated, err := st.UpdateClaimedStepFields(ctx, stepID, "owner-a", StepPatch{Progress: ptrInt(50)}); err != nil || updated {
		t.Fatalf("stale owner update = %v, %v", updated, err)
	}
	if updated, err := st.UpdateClaimedStepFields(ctx, stepID, "owner-b", StepPatch{Progress: ptrInt(50)}); err != nil || !updated {
		t.Fatalf("current owner update = %v, %v", updated, err)
	}
}

func TestCreateTaskPlansCommitsTaskStepsAndOutbox(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	task := &domain.Task{ID: domain.MustID(), Name: "atomic submit", Kind: domain.TaskDomain, Target: "example.com", Status: domain.TaskQueued}
	steps := []domain.TaskStep{{ID: domain.MustID(), TaskID: task.ID, Sequence: 1, Kind: "dns", Status: domain.StepQueued}}
	if err := st.CreateTaskPlans(ctx, []TaskPlan{{
		Task: task, Steps: steps, OutboxRoutingKey: "q.discovery.dns", OutboxPayload: []byte(`{"taskId":"` + task.ID + `"}`),
	}}); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetTask(ctx, task.ID)
	if err != nil || got == nil {
		t.Fatalf("task=%+v err=%v", got, err)
	}
	gotSteps, err := st.ListSteps(ctx, task.ID)
	if err != nil || len(gotSteps) != 1 || gotSteps[0].Status != domain.StepQueued {
		t.Fatalf("steps=%+v err=%v", gotSteps, err)
	}
	outbox, err := st.PendingOutbox(ctx, 10)
	if err != nil || len(outbox) != 1 {
		t.Fatalf("outbox=%+v err=%v", outbox, err)
	}
}

func TestCreateTaskPlansRollsBackEverythingOnStepFailure(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	task := &domain.Task{ID: domain.MustID(), Name: "rollback", Kind: domain.TaskDomain, Target: "example.com", Status: domain.TaskQueued}
	steps := []domain.TaskStep{
		{ID: domain.MustID(), TaskID: task.ID, Sequence: 1, Kind: "dns", Status: domain.StepQueued},
		{ID: domain.MustID(), TaskID: task.ID, Sequence: 1, Kind: "http", Status: domain.StepPending},
	}
	err := st.CreateTaskPlans(ctx, []TaskPlan{{
		Task: task, Steps: steps, OutboxRoutingKey: "q.discovery.dns", OutboxPayload: []byte(`{}`),
	}})
	if err == nil {
		t.Fatal("expected duplicate step sequence to fail the transaction")
	}
	got, getErr := st.GetTask(ctx, task.ID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if got != nil {
		t.Fatalf("task was committed despite failed plan: %+v", got)
	}
	outbox, getErr := st.PendingOutbox(ctx, 10)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if len(outbox) != 0 {
		t.Fatalf("outbox committed despite failed plan: %+v", outbox)
	}
}

func ptrInt(v int) *int { return &v }
