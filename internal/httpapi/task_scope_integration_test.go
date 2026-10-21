package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/tasking"
	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"github.com/red-copilot/red-x/internal/platform/logx"
)

func TestCreateTaskRejectsTargetOutsideScope(t *testing.T) {
	st, ctx := newHTTPTestStore(t)
	scope := &domain.Scope{Name: "scope", Type: "domain", Targets: []string{"example.com"}}
	if err := st.CreateScope(ctx, scope); err != nil {
		t.Fatal(err)
	}
	queue := jobs.NewMemory(logx.New("error", false))
	defer queue.Close()
	engine := &tasking.Engine{Store: st, Queue: queue, Config: config.Default(), Log: logx.New("error", false)}
	server := &Server{Store: st, Engine: engine}
	body := []byte(`{"kind":"domain","target":"google.com","scopeId":"` + scope.ID + `"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withPrincipal(req, domain.RoleOperator)
	rec := httptest.NewRecorder()
	server.handleCreateTask(rec, req)
	if rec.Code != http.StatusForbidden || !bytes.Contains(rec.Body.Bytes(), []byte("task.scope_forbidden")) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
