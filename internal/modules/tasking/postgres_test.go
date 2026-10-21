package tasking

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"github.com/red-copilot/red-x/internal/platform/logx"
	"github.com/red-copilot/red-x/internal/store"
)

func newTaskingPostgresStore(t *testing.T) (*store.Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("ARL_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set ARL_TEST_POSTGRES_DSN to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	schema := fmt.Sprintf("arl_tasking_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		_ = admin.Close(context.Background())
		t.Fatalf("create test schema: %v", err)
	}
	st, err := store.Open(ctx, store.Options{DSN: dsnWithSearchPath(dsn, schema), MaxOpenConns: 10, MaxIdleConns: 5, ConnMaxLifetime: time.Minute})
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

func TestPortsForIPUsesDiscoveredCustomPort(t *testing.T) {
	st, ctx := newTaskingPostgresStore(t)
	taskID := domain.MustID()
	if err := st.CreateTask(ctx, &domain.Task{ID: taskID, Name: "ip scan", Kind: domain.TaskIP, Target: "127.0.0.1", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"ports": []map[string]any{{"portId": 58080, "protocol": "tcp"}}}
	asset, err := st.UpsertAsset(ctx, domain.AssetIP, "127.0.0.1", data, taskID, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ObserveAsset(ctx, taskID, asset, "port", data); err != nil {
		t.Fatal(err)
	}
	engine := &Engine{Store: st}
	ports, err := engine.portsForIP(ctx, taskID, "127.0.0.1")
	if err != nil {
		t.Fatalf("portsForIP: %v", err)
	}
	if len(ports) != 1 || ports[0] != 58080 {
		t.Fatalf("ports = %v, want [58080]", ports)
	}
}

func TestSitesForIPChoosesOneSchemePerPort(t *testing.T) {
	cases := []struct {
		name string
		port int
		tls  bool
		want string
	}{
		{name: "plain custom port", port: 58081, want: "http://127.0.0.1:58081"},
		{name: "tls custom port", port: 58443, tls: true, want: "https://127.0.0.1:58443"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, ctx := newTaskingPostgresStore(t)
			taskID := domain.MustID()
			if err := st.CreateTask(ctx, &domain.Task{ID: taskID, Name: "ip scan", Kind: domain.TaskIP, Target: "127.0.0.1", Status: domain.TaskQueued}); err != nil {
				t.Fatal(err)
			}
			data := map[string]any{"ports": []map[string]any{{"portId": tc.port, "tls": tc.tls}}}
			asset, err := st.UpsertAsset(ctx, domain.AssetIP, "127.0.0.1", data, taskID, "")
			if err != nil {
				t.Fatal(err)
			}
			if err := st.ObserveAsset(ctx, taskID, asset, "port", data); err != nil {
				t.Fatal(err)
			}
			engine := &Engine{Store: st}
			sites, err := engine.sitesForIP(ctx, taskID, "127.0.0.1")
			if err != nil {
				t.Fatalf("sitesForIP: %v", err)
			}
			if len(sites) != 1 || sites[0] != tc.want {
				t.Fatalf("sites = %v, want [%s]", sites, tc.want)
			}
		})
	}
}

func TestSubmitEnforcesScopeAllowlist(t *testing.T) {
	st, ctx := newTaskingPostgresStore(t)
	scope := &domain.Scope{Name: "authorized domain", Type: "domain", Targets: []string{"example.com"}}
	if err := st.CreateScope(ctx, scope); err != nil {
		t.Fatal(err)
	}
	queue := jobs.NewMemory(logx.New("error", false))
	defer queue.Close()
	engine := &Engine{Store: st, Queue: queue, Config: config.Default(), Log: logx.New("error", false)}

	_, err := engine.Submit(ctx, SubmitInput{
		Name: "outside", Kind: domain.TaskDomain, Target: "google.com", ScopeID: scope.ID,
	})
	if err == nil {
		t.Fatal("Submit accepted a target outside the scope")
	}

	created, err := engine.Submit(ctx, SubmitInput{
		Name: "inside", Kind: domain.TaskDomain, Target: "example.com", ScopeID: scope.ID,
	})
	if err != nil {
		t.Fatalf("Submit inside scope: %v", err)
	}
	if len(created) != 1 || created[0].ScopeID != scope.ID || created[0].Options.ScopeID != scope.ID {
		t.Fatalf("created = %+v", created)
	}
}

func TestSubmitRejectsUnknownScope(t *testing.T) {
	st, ctx := newTaskingPostgresStore(t)
	queue := jobs.NewMemory(logx.New("error", false))
	defer queue.Close()
	engine := &Engine{Store: st, Queue: queue, Config: config.Default(), Log: logx.New("error", false)}
	_, err := engine.Submit(ctx, SubmitInput{
		Name: "missing scope", Kind: domain.TaskDomain, Target: "example.com", ScopeID: domain.MustID(),
	})
	if err == nil {
		t.Fatal("Submit accepted an unknown scope")
	}
}
