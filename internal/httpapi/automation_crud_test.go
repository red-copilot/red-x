package httpapi

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/identity"
	"github.com/red-copilot/red-x/internal/modules/monitoring"
	"github.com/red-copilot/red-x/internal/store"
)

func TestMaintainableObjectUpdateAndDeleteHandlers(t *testing.T) {
	st, ctx := newHTTPTestStore(t)
	s := &Server{Store: st, Scheduler: monitoring.New(st, nil, nil, slog.Default())}
	router := chi.NewRouter()
	router.Patch("/policies/{id}", s.requireWrite(s.handleUpdatePolicy))
	router.Delete("/policies/{id}", s.requireWrite(s.handleDeletePolicy))
	router.Patch("/scopes/{id}", s.requireWrite(s.handleUpdateScope))
	router.Delete("/scopes/{id}", s.requireWrite(s.handleDeleteScope))
	router.Get("/monitors/{id}", s.handleGetMonitor)
	router.Patch("/monitors/{id}", s.requireWrite(s.handleUpdateMonitor))
	router.Delete("/monitors/{id}", s.requireWrite(s.handleDeleteMonitor))
	router.Get("/schedules/{id}", s.handleGetSchedule)
	router.Patch("/schedules/{id}", s.requireWrite(s.handleUpdateSchedule))
	router.Delete("/schedules/{id}", s.requireWrite(s.handleDeleteSchedule))
	router.Get("/fingerprints/{id}", s.handleGetFingerprint)
	router.Patch("/fingerprints/{id}", s.requireWrite(s.handleUpdateFingerprint))
	router.Delete("/fingerprints/{id}", s.requireWrite(s.handleDeleteFingerprint))
	router.Get("/github/monitors/{id}", s.handleGetGitHubMonitor)
	router.Patch("/github/monitors/{id}", s.requireWrite(s.handleUpdateGitHubMonitor))
	router.Delete("/github/monitors/{id}", s.requireWrite(s.handleDeleteGitHubMonitor))
	router.Get("/users/{id}", s.requireAdmin(s.handleGetUser))
	router.Patch("/users/{id}", s.requireAdmin(s.handleUpdateUser))
	router.Delete("/users/{id}", s.requireAdmin(s.handleDeleteUser))

	policy := &domain.Policy{Name: "CRUD policy", Description: "initial"}
	scope := &domain.Scope{Name: "CRUD scope", Type: "domain", Targets: []string{"example.com"}}
	for _, item := range []any{policy, scope} {
		switch value := item.(type) {
		case *domain.Policy:
			if err := st.CreatePolicy(ctx, value); err != nil {
				t.Fatal(err)
			}
		case *domain.Scope:
			if err := st.CreateScope(ctx, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	monitor := &domain.Monitor{
		Name: "CRUD monitor", Kind: "domain", ScopeID: scope.ID, Target: "example.com",
		IntervalSec: 21600, PolicyID: policy.ID, Status: "active",
	}
	if err := st.CreateMonitor(ctx, monitor); err != nil {
		t.Fatal(err)
	}
	schedule := &domain.Schedule{
		Name: "CRUD schedule", Kind: "recurrent_scan", TaskKind: domain.TaskDomain,
		Target: "example.com", PolicyID: policy.ID, Cron: "0 2 * * *", Status: "active",
	}
	if err := st.CreateSchedule(ctx, schedule); err != nil {
		t.Fatal(err)
	}
	fingerprint := &store.FingerprintRule{Name: "CRUD fingerprint", Rule: `body="crud"`}
	if err := st.CreateFingerprint(ctx, fingerprint); err != nil {
		t.Fatal(err)
	}
	githubMonitor := &domain.GitHubMonitor{Name: "CRUD GitHub", Keyword: "example.com", IntervalSec: 3600, Status: "active"}
	if err := st.CreateGitHubMonitor(ctx, githubMonitor); err != nil {
		t.Fatal(err)
	}
	viewer := &domain.User{Username: "crud-viewer", PasswordHash: "hash", Role: domain.RoleViewer}
	admin := &domain.User{Username: "crud-admin", PasswordHash: "hash", Role: domain.RoleAdmin}
	for _, user := range []*domain.User{viewer, admin} {
		if err := st.CreateUser(ctx, user); err != nil {
			t.Fatal(err)
		}
	}

	assertUpdate(t, requestWithPrincipal(t, http.MethodPatch, "/policies/"+policy.ID, `{
		"name":"CRUD policy renamed","description":"updated","config":{}
	}`, router), `"name":"CRUD policy renamed"`)
	assertUpdate(t, requestWithPrincipal(t, http.MethodPatch, "/scopes/"+scope.ID, `{
		"name":"CRUD scope renamed","type":"domain","targets":["updated.example.com"]
	}`, router), `"name":"CRUD scope renamed"`)
	assertUpdate(t, requestWithPrincipal(t, http.MethodPatch, "/monitors/"+monitor.ID, `{
		"name":"CRUD monitor renamed","target":"updated.example.com","intervalSec":28800
	}`, router), `"name":"CRUD monitor renamed"`)
	assertUpdate(t, requestWithPrincipal(t, http.MethodPatch, "/schedules/"+schedule.ID, `{
		"name":"CRUD schedule renamed","cron":"0 3 * * *"
	}`, router), `"name":"CRUD schedule renamed"`)
	assertUpdate(t, requestWithPrincipal(t, http.MethodPatch, "/fingerprints/"+fingerprint.ID, `{
		"name":"CRUD fingerprint renamed","rule":"title=\"updated\""
	}`, router), `"name":"CRUD fingerprint renamed"`)
	assertUpdate(t, requestWithPrincipal(t, http.MethodPatch, "/github/monitors/"+githubMonitor.ID, `{
		"name":"CRUD GitHub renamed","keyword":"updated.example.com","intervalSec":7200
	}`, router), `"name":"CRUD GitHub renamed"`)
	assertUpdate(t, requestWithUser(t, http.MethodPatch, "/users/"+viewer.ID, `{
		"role":"operator","disabled":true
	}`, admin.Username, admin.ID, domain.RoleAdmin, router), `"role":"operator"`)

	for _, target := range []string{
		"/monitors/" + monitor.ID,
		"/schedules/" + schedule.ID,
		"/fingerprints/" + fingerprint.ID,
		"/github/monitors/" + githubMonitor.ID,
	} {
		rec := requestWithPrincipal(t, http.MethodDelete, target, "", router)
		if rec.Code != http.StatusOK {
			t.Fatalf("delete %s status=%d body=%s", target, rec.Code, rec.Body.String())
		}
	}
	assertUpdate(t, requestWithPrincipal(t, http.MethodPatch, "/policies/"+policy.ID, `{
		"name":"CRUD policy renamed","description":"updated","config":{}
	}`, router), `"name":"CRUD policy renamed"`)
	if rec := requestWithPrincipal(t, http.MethodDelete, "/policies/"+policy.ID, "", router); rec.Code != http.StatusOK {
		t.Fatalf("policy delete status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := requestWithPrincipal(t, http.MethodDelete, "/scopes/"+scope.ID, "", router); rec.Code != http.StatusOK {
		t.Fatalf("scope delete status=%d body=%s", rec.Code, rec.Body.String())
	}
	userDelete := requestWithUser(t, http.MethodDelete, "/users/"+viewer.ID, "", admin.Username, admin.ID, domain.RoleAdmin, router)
	if userDelete.Code != http.StatusOK {
		t.Fatalf("user delete status=%d body=%s", userDelete.Code, userDelete.Body.String())
	}
}

func assertUpdate(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusOK || !containsJSONValue(rec.Body.String(), want) {
		t.Fatalf("update status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func requestWithUser(t *testing.T, method, target, body, username, userID string, role domain.Role, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req = req.WithContext(context.WithValue(req.Context(), principalKey{}, &identity.Principal{
		UserID: userID, Username: username, Role: role,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
