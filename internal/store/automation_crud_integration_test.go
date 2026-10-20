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
