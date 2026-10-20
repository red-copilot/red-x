package store

import (
	"errors"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
)

func TestPolicyAndScopeDeletionRespectActiveReferences(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	scope := &domain.Scope{Name: "referenced scope", Type: "domain", Targets: []string{"example.com"}}
	if err := st.CreateScope(ctx, scope); err != nil {
		t.Fatal(err)
	}
	policy := &domain.Policy{Name: "referenced policy"}
	if err := st.CreatePolicy(ctx, policy); err != nil {
		t.Fatal(err)
	}
	monitor := &domain.Monitor{
		Name: "active monitor", Kind: "domain", ScopeID: scope.ID, Target: "example.com",
		IntervalSec: 21600, PolicyID: policy.ID, Status: "active",
	}
	if err := st.CreateMonitor(ctx, monitor); err != nil {
		t.Fatal(err)
	}
	schedule := &domain.Schedule{
		Name: "active schedule", Kind: "recurrent_scan", TaskKind: domain.TaskDomain,
		Target: "example.com", PolicyID: policy.ID, Cron: "0 2 * * *", Status: "active",
	}
	if err := st.CreateSchedule(ctx, schedule); err != nil {
		t.Fatal(err)
	}
	if err := st.DeletePolicy(ctx, policy.ID, "operator"); !errors.Is(err, domain.ErrResourceInUse) {
		t.Fatalf("policy delete error = %v", err)
	}
	if err := st.DeleteScope(ctx, scope.ID, "operator"); !errors.Is(err, domain.ErrResourceInUse) {
		t.Fatalf("scope delete error = %v", err)
	}
	if err := st.SoftDeleteRecord(ctx, "monitor", monitor.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	if err := st.SoftDeleteRecord(ctx, "schedule", schedule.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	if err := st.DeletePolicy(ctx, policy.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteScope(ctx, scope.ID, "operator"); err != nil {
		t.Fatal(err)
	}
}

func TestUserDeletionProtectsSelfAndLastAdmin(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	admin := &domain.User{Username: "last-admin", PasswordHash: "hash", Role: domain.RoleAdmin}
	viewer := &domain.User{Username: "viewer", PasswordHash: "hash", Role: domain.RoleViewer}
	for _, user := range []*domain.User{admin, viewer} {
		if err := st.CreateUser(ctx, user); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.DeleteUser(ctx, admin.ID, "last-admin", admin.ID); !errors.Is(err, domain.ErrSelfDelete) {
		t.Fatalf("self delete error = %v", err)
	}
	if err := st.DeleteUser(ctx, admin.ID, "viewer", viewer.ID); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("last admin delete error = %v", err)
	}
	session := &domain.Session{UserID: viewer.ID, RefreshHash: "delete-test-session", ExpiresAt: time.Now().UTC().Add(time.Hour)}
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteUser(ctx, viewer.ID, "last-admin", admin.ID); err != nil {
		t.Fatal(err)
	}
	active, err := st.GetUserByID(ctx, viewer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active != nil {
		t.Fatal("deleted user remained active")
	}
	storedSession, err := st.GetSessionByHash(ctx, session.RefreshHash)
	if err != nil {
		t.Fatal(err)
	}
	if storedSession == nil || storedSession.RevokedAt == nil {
		t.Fatalf("deleted user session was not revoked: %#v", storedSession)
	}
}

func TestUpdatesCannotReactivateDeletedRecords(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	org := &domain.Organization{CanonicalName: "Immutable Deleted Organization"}
	policy := &domain.Policy{Name: "immutable-deleted-policy"}
	scope := &domain.Scope{Name: "immutable-deleted-scope", Type: "domain", Targets: []string{"example.com"}}
	monitor := &domain.Monitor{Name: "immutable-deleted-monitor", Kind: "domain", ScopeID: scope.ID, Target: "example.com", IntervalSec: 21600, Status: "active"}
	schedule := &domain.Schedule{Name: "immutable-deleted-schedule", Kind: "recurrent_scan", TaskKind: domain.TaskDomain, Target: "example.com", PolicyID: policy.ID, Cron: "0 2 * * *", Status: "active"}
	githubMonitor := &domain.GitHubMonitor{Name: "immutable-deleted-github", Keyword: "example.com", IntervalSec: 3600, Status: "active"}
	user := &domain.User{Username: "immutable-deleted-user", PasswordHash: "hash", Role: domain.RoleViewer}
	fingerprint := &FingerprintRule{Name: "immutable-deleted-fingerprint", Rule: `body="deleted"`}
	for _, create := range []func() error{
		func() error { return st.CreateOrganization(ctx, org) },
		func() error { return st.CreatePolicy(ctx, policy) },
		func() error { return st.CreateScope(ctx, scope) },
		func() error { return st.CreateMonitor(ctx, monitor) },
		func() error { return st.CreateSchedule(ctx, schedule) },
		func() error { return st.CreateGitHubMonitor(ctx, githubMonitor) },
		func() error { return st.CreateUser(ctx, user) },
		func() error { return st.CreateFingerprint(ctx, fingerprint) },
	} {
		if err := create(); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		resourceType string
		id           string
		update       func() error
	}{
		{"organization", org.ID, func() error { org.CanonicalName = "changed"; return st.UpdateOrganization(ctx, org) }},
		{"policy", policy.ID, func() error { policy.Name = "changed"; return st.UpdatePolicy(ctx, policy) }},
		{"scope", scope.ID, func() error { scope.Name = "changed"; return st.UpdateScope(ctx, scope) }},
		{"monitor", monitor.ID, func() error { monitor.Name = "changed"; return st.UpdateMonitor(ctx, monitor) }},
		{"schedule", schedule.ID, func() error { schedule.Name = "changed"; return st.UpdateSchedule(ctx, schedule) }},
		{"github_monitor", githubMonitor.ID, func() error { githubMonitor.Name = "changed"; return st.UpdateGitHubMonitor(ctx, githubMonitor) }},
		{"user", user.ID, func() error { user.Disabled = true; return st.UpdateUser(ctx, user) }},
		{"fingerprint", fingerprint.ID, func() error { fingerprint.Name = "changed"; return st.UpdateFingerprint(ctx, fingerprint) }},
	}
	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			if err := st.SoftDeleteRecord(ctx, tt.resourceType, tt.id, "operator"); err != nil {
				t.Fatal(err)
			}
			if err := tt.update(); !errors.Is(err, domain.ErrRecordNotFound) {
				t.Fatalf("update error = %v, want ErrRecordNotFound", err)
			}
		})
	}
}
