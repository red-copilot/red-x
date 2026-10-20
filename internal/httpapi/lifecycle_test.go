package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/store"
)

func newHTTPTestStore(t *testing.T) (*store.Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("ARL_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set ARL_TEST_POSTGRES_DSN to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	schema := fmt.Sprintf("arl_http_test_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		_ = admin.Close(context.Background())
		t.Fatal(err)
	}
	testDSN := dsnWithSchema(dsn, schema)
	st, err := store.Open(ctx, store.Options{DSN: testDSN, MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: time.Minute})
	if err != nil {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = admin.Close(context.Background())
		t.Fatal(err)
	}
	if err := st.Migrate(ctx); err != nil {
		_ = st.Close(context.Background())
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = admin.Close(context.Background())
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = st.Close(context.Background())
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = admin.Close(context.Background())
	})
	return st, ctx
}

func dsnWithSchema(dsn, schema string) string {
	if parsed, err := url.Parse(dsn); err == nil && parsed.Scheme != "" {
		query := parsed.Query()
		query.Set("search_path", schema+",public")
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return dsn + " search_path=" + schema + ",public"
}

func TestRecycleBinListAndRestoreHandlers(t *testing.T) {
	st, ctx := newHTTPTestStore(t)
	policy := &domain.Policy{Name: "recycle-bin-policy"}
	if err := st.CreatePolicy(ctx, policy); err != nil {
		t.Fatal(err)
	}
	if err := st.SoftDeleteRecord(ctx, "policy", policy.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	s := &Server{Store: st}
	router := chi.NewRouter()
	router.Get("/recycle-bin", s.requireAdmin(s.handleListRecycleBin))
	router.Post("/recycle-bin/{type}/{id}/restore", s.requireAdmin(s.handleRestoreRecycleBin))

	viewer := httptest.NewRecorder()
	viewerReq := withPrincipal(httptest.NewRequest(http.MethodGet, "/recycle-bin", nil), domain.RoleViewer)
	router.ServeHTTP(viewer, viewerReq)
	if viewer.Code != http.StatusForbidden {
		t.Fatalf("viewer status = %d", viewer.Code)
	}

	list := httptest.NewRecorder()
	listReq := withPrincipal(httptest.NewRequest(http.MethodGet, "/recycle-bin?type=policy", nil), domain.RoleAdmin)
	router.ServeHTTP(list, listReq)
	if list.Code != http.StatusOK || !containsJSONValue(list.Body.String(), "recycle-bin-policy") {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	restore := httptest.NewRecorder()
	restoreReq := withPrincipal(httptest.NewRequest(http.MethodPost, "/recycle-bin/policy/"+policy.ID+"/restore", nil), domain.RoleAdmin)
	router.ServeHTTP(restore, restoreReq)
	if restore.Code != http.StatusOK || !containsJSONValue(restore.Body.String(), `"type":"policy"`) {
		t.Fatalf("restore status=%d body=%s", restore.Code, restore.Body.String())
	}
	active, err := st.GetPolicy(ctx, policy.ID)
	if err != nil || active == nil {
		t.Fatalf("restored policy active=%#v err=%v", active, err)
	}
}

func TestRecycleBinRejectsUnknownResourceType(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/recycle-bin?type=unknown", nil), domain.RoleAdmin)
	s.handleListRecycleBin(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func containsJSONValue(body, value string) bool {
	return len(body) > 0 && len(value) > 0 && strings.Contains(body, value)
}
