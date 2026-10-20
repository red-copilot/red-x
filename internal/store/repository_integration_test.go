package store

import (
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
)

func TestPolicyIdentityAndSessionRepositories(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	if err := st.SeedDefaultPolicy(ctx); err != nil {
		t.Fatal(err)
	}
	policies, err := st.ListPolicies(ctx)
	if err != nil || len(policies) != 1 || policies[0].Name != "默认策略" {
		t.Fatalf("policies=%+v err=%v", policies, err)
	}
	u := &domain.User{Username: "operator", PasswordHash: "hash", Role: domain.RoleOperator}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetUserByUsername(ctx, "operator")
	if err != nil || got == nil || got.ID != u.ID {
		t.Fatalf("user=%+v err=%v", got, err)
	}
	sess := &domain.Session{UserID: u.ID, RefreshHash: "refresh-one", ExpiresAt: time.Now().Add(time.Hour)}
	if err := st.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if err := st.RotateSession(ctx, sess.ID, "refresh-two", time.Now().Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	rotated, err := st.GetSessionByHash(ctx, "refresh-two")
	if err != nil || rotated == nil || rotated.RotatedAt == nil {
		t.Fatalf("rotated session=%+v err=%v", rotated, err)
	}
	key := &domain.APIKey{Name: "automation", KeyHash: "key-hash", Scopes: []string{"scan"}, CreatedBy: u.ID}
	if err := st.CreateAPIKey(ctx, key); err != nil {
		t.Fatal(err)
	}
	keyGot, err := st.GetAPIKeyByHash(ctx, "key-hash")
	if err != nil || keyGot == nil || len(keyGot.Scopes) != 1 {
		t.Fatalf("api key=%+v err=%v", keyGot, err)
	}
}

func TestTaskPatchAndFindingDedupe(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	task := &domain.Task{Name: "jsonb", Kind: domain.TaskDomain, Target: "example.com", Status: domain.TaskQueued}
	if err := st.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	stats := domain.TaskStatistics{Domains: 2, IPs: 3}
	warnings := []string{"bounded"}
	if err := st.UpdateTaskFields(ctx, task.ID, TaskPatch{Statistics: &stats, Warnings: &warnings}); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetTask(ctx, task.ID)
	if err != nil || got.Statistics.Domains != 2 || len(got.Warnings) != 1 {
		t.Fatalf("task=%+v err=%v", got, err)
	}
	finding := &domain.Finding{TaskID: task.ID, Kind: domain.FindingVuln, DedupeKey: "same", Title: "x", Target: "example.com"}
	created, err := st.AddFinding(ctx, finding)
	if err != nil || !created {
		t.Fatalf("first finding created=%v err=%v", created, err)
	}
	created, err = st.AddFinding(ctx, &domain.Finding{TaskID: task.ID, Kind: domain.FindingVuln, DedupeKey: "same", Title: "x", Target: "example.com"})
	if err != nil || created {
		t.Fatalf("duplicate finding created=%v err=%v", created, err)
	}
}

func TestOrganizationAliasSearch(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	org := &domain.Organization{CanonicalName: "四川示例", Region: "蜀", Aliases: []string{"示例别名"}}
	if err := st.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	found, err := st.FindOrganizationByNameAndRegion(ctx, "示例别名", "蜀")
	if err != nil || found == nil || found.ID != org.ID {
		t.Fatalf("alias lookup=%+v err=%v", found, err)
	}
	list, err := st.ListOrganizations(ctx, "别名", 10)
	if err != nil || len(list) != 1 || list[0].ID != org.ID {
		t.Fatalf("alias search=%+v err=%v", list, err)
	}
}

func TestMonitoringAndGitHubRepositories(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	now := time.Now().UTC()
	monitor := &domain.Monitor{Name: "monitor", Kind: "domain", Target: "example.com", IntervalSec: 21600, Status: "active", NextRunAt: now.Add(-time.Minute)}
	if err := st.CreateMonitor(ctx, monitor); err != nil {
		t.Fatal(err)
	}
	due, err := st.DueMonitors(ctx, now, 10)
	if err != nil || len(due) != 1 {
		t.Fatalf("due monitors=%+v err=%v", due, err)
	}
	ghMonitor := &domain.GitHubMonitor{Name: "github", Keyword: "secret", IntervalSec: 3600, Status: "active", NextRunAt: now}
	if err := st.CreateGitHubMonitor(ctx, ghMonitor); err != nil {
		t.Fatal(err)
	}
	ghTask := &domain.GitHubTask{Keyword: "secret", Status: domain.TaskQueued, MonitorID: ghMonitor.ID}
	if err := st.CreateGitHubTask(ctx, ghTask); err != nil {
		t.Fatal(err)
	}
	finding := &domain.GitHubFinding{TaskID: ghTask.ID, MonitorID: ghMonitor.ID, Keyword: "secret", Repo: "org/repo", Path: "a.txt", HashMD5: "same"}
	created, err := st.AddGitHubFinding(ctx, finding)
	if err != nil || !created {
		t.Fatalf("github finding created=%v err=%v", created, err)
	}
	created, err = st.AddGitHubFinding(ctx, &domain.GitHubFinding{TaskID: ghTask.ID, HashMD5: "same"})
	if err != nil || created {
		t.Fatalf("github duplicate created=%v err=%v", created, err)
	}
}
