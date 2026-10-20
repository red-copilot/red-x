package tasking

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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
