package store

import (
	"errors"
	"testing"

	"github.com/red-copilot/red-x/internal/domain"
)

func TestSoftDeleteAndRestorePolicy(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	policy := &domain.Policy{Name: "CRUD lifecycle policy"}
	if err := st.CreatePolicy(ctx, policy); err != nil {
		t.Fatal(err)
	}

	if err := st.SoftDeleteRecord(ctx, "policy", policy.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetPolicy(ctx, policy.ID); err != nil {
		t.Fatal(err)
	}
	active, err := st.GetPolicy(ctx, policy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active != nil {
		t.Fatalf("deleted policy remained active: %#v", active)
	}
	if err := st.SoftDeleteRecord(ctx, "policy", policy.ID, "operator"); !errors.Is(err, domain.ErrRecordNotFound) {
		t.Fatalf("second delete error = %v, want ErrRecordNotFound", err)
	}

	items, total, err := st.ListDeletedRecords(ctx, DeletedRecordFilter{Type: "policy", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != policy.ID || items[0].DeletedBy != "operator" {
		t.Fatalf("unexpected recycle bin items: total=%d items=%#v", total, items)
	}

	if err := st.RestoreRecord(ctx, "policy", policy.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	active, err = st.GetPolicy(ctx, policy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active == nil {
		t.Fatal("restored policy was not found")
	}
	if active.RestoredAt == nil || active.RestoreReason != "manual" {
		t.Fatalf("restore metadata was not stored: %#v", active.Lifecycle)
	}
}

func TestLifecycleFiltersEveryMaintainableResource(t *testing.T) {
	st, ctx := newPostgresTestStore(t)
	user := &domain.User{Username: "lifecycle-user", PasswordHash: "hash", Role: domain.RoleViewer}
	if err := st.CreateUser(ctx, user); err != nil {
		t.Fatal(err)
	}
	policy := &domain.Policy{Name: "lifecycle-filter-policy"}
	if err := st.CreatePolicy(ctx, policy); err != nil {
		t.Fatal(err)
	}
	scope := &domain.Scope{Name: "lifecycle-scope", Type: "domain", Targets: []string{"example.com"}}
	if err := st.CreateScope(ctx, scope); err != nil {
		t.Fatal(err)
	}
	monitor := &domain.Monitor{Name: "lifecycle-monitor", Kind: "domain", ScopeID: scope.ID, Target: "example.com", IntervalSec: 21600, Status: "active"}
	if err := st.CreateMonitor(ctx, monitor); err != nil {
		t.Fatal(err)
	}
	schedule := &domain.Schedule{Name: "lifecycle-schedule", Kind: "recurrent_scan", TaskKind: domain.TaskDomain, Target: "example.com", PolicyID: policy.ID, Cron: "0 2 * * *", Status: "active"}
	if err := st.CreateSchedule(ctx, schedule); err != nil {
		t.Fatal(err)
	}
	fingerprint := &FingerprintRule{Name: "lifecycle-fingerprint", Rule: `body="example"`}
	if err := st.CreateFingerprint(ctx, fingerprint); err != nil {
		t.Fatal(err)
	}
	githubMonitor := &domain.GitHubMonitor{Name: "lifecycle-github", Keyword: "example.com", IntervalSec: 3600, Status: "active"}
	if err := st.CreateGitHubMonitor(ctx, githubMonitor); err != nil {
		t.Fatal(err)
	}
	org := &domain.Organization{CanonicalName: "Lifecycle Organization"}
	if err := st.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	orgDomain := &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: "lifecycle.example.com", Source: "manual",
		Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusCandidate,
	}
	if err := st.UpsertOrganizationDomain(ctx, orgDomain); err != nil {
		t.Fatal(err)
	}
	asset, err := st.UpsertAsset(ctx, domain.AssetDomain, "lifecycle.example.com", map[string]any{"title": "Lifecycle"}, "", "")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		resourceType string
		id           string
		active       func() (bool, error)
	}{
		{"user", user.ID, func() (bool, error) {
			item, err := st.GetUserByID(ctx, user.ID)
			return item != nil, err
		}},
		{"policy", policy.ID, func() (bool, error) {
			item, err := st.GetPolicy(ctx, policy.ID)
			return item != nil, err
		}},
		{"scope", scope.ID, func() (bool, error) {
			item, err := st.GetScope(ctx, scope.ID)
			return item != nil, err
		}},
		{"monitor", monitor.ID, func() (bool, error) {
			item, err := st.GetMonitor(ctx, monitor.ID)
			return item != nil, err
		}},
		{"schedule", schedule.ID, func() (bool, error) {
			items, _, err := st.ListSchedules(ctx, PageQuery{Page: 1, PageSize: 20, Search: "lifecycle-schedule"})
			return len(items) > 0, err
		}},
		{"fingerprint", fingerprint.ID, func() (bool, error) {
			items, err := st.ListFingerprints(ctx)
			for _, item := range items {
				if item.ID == fingerprint.ID {
					return true, err
				}
			}
			return false, err
		}},
		{"github_monitor", githubMonitor.ID, func() (bool, error) {
			items, _, err := st.ListGitHubMonitors(ctx, PageQuery{Page: 1, PageSize: 20})
			for _, item := range items {
				if item.ID == githubMonitor.ID {
					return true, err
				}
			}
			return false, err
		}},
		{"organization", org.ID, func() (bool, error) {
			item, err := st.GetOrganization(ctx, org.ID)
			return item != nil, err
		}},
		{"organization_domain", orgDomain.ID, func() (bool, error) {
			item, err := st.GetOrganizationDomainByID(ctx, orgDomain.ID)
			return item != nil, err
		}},
		{"asset", asset.ID, func() (bool, error) {
			item, err := st.GetAssetByID(ctx, asset.ID)
			return item != nil, err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			if err := st.SoftDeleteRecord(ctx, tt.resourceType, tt.id, "operator"); err != nil {
				t.Fatal(err)
			}
			visible, err := tt.active()
			if err != nil {
				t.Fatal(err)
			}
			if visible {
				t.Fatal("deleted record remained visible")
			}
			if err := st.RestoreRecord(ctx, tt.resourceType, tt.id, "admin"); err != nil {
				t.Fatal(err)
			}
			visible, err = tt.active()
			if err != nil {
				t.Fatal(err)
			}
			if !visible {
				t.Fatal("restored record remained hidden")
			}
		})
	}
}
