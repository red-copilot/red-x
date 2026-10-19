package easm

import (
	"context"
	"testing"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/tasking"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type fakeScanSubmitter struct{ inputs []tasking.SubmitInput }

func (f *fakeScanSubmitter) Submit(_ context.Context, in tasking.SubmitInput) ([]domain.Task, error) {
	f.inputs = append(f.inputs, in)
	return []domain.Task{{ID: bson.NewObjectID(), Name: in.Name, Target: in.Target, OrganizationID: in.OrganizationID, RootDomain: in.RootDomain, Origin: in.Origin}}, nil
}

func TestCreateScansRequiresConfirmedDomainsAndCreatesOneTaskEach(t *testing.T) {
	orgID := bson.NewObjectID()
	store := newFakeICPStore()
	store.orgs[orgID] = &domain.Organization{ID: orgID, CanonicalName: "单位A", Status: domain.OrgStatusActive}
	for _, row := range []*domain.OrganizationDomain{
		{ID: bson.NewObjectID(), OrganizationID: orgID, RootDomain: "a.edu.cn", Status: domain.DomainStatusConfirmed},
		{ID: bson.NewObjectID(), OrganizationID: orgID, RootDomain: "b.edu.cn", Status: domain.DomainStatusConfirmed},
		{ID: bson.NewObjectID(), OrganizationID: orgID, RootDomain: "c.edu.cn", Status: domain.DomainStatusCandidate},
	} {
		store.domains[orgID.Hex()+":"+row.RootDomain] = row
	}
	submit := &fakeScanSubmitter{}
	svc := &Service{Store: store, Submitter: submit}
	tasks, errs, err := svc.CreateScans(context.Background(), orgID, []string{"a.edu.cn", "b.edu.cn"}, domain.TaskOptions{PortScan: true}, "", "", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 0 || len(tasks) != 2 || len(submit.inputs) != 2 {
		t.Fatalf("tasks=%+v errors=%+v inputs=%+v", tasks, errs, submit.inputs)
	}
	if submit.inputs[0].Origin != domain.TaskOriginEASM || submit.inputs[0].OrganizationID != orgID.Hex() || submit.inputs[0].RootDomain != "a.edu.cn" {
		t.Fatalf("task lineage missing: %+v", submit.inputs[0])
	}
	if len(store.audits) != 1 {
		t.Fatalf("expected one batch audit, got %+v", store.audits)
	}
}

func TestCreateScansRejectsUnconfirmedBeforeCreatingTasks(t *testing.T) {
	orgID := bson.NewObjectID()
	store := newFakeICPStore()
	store.orgs[orgID] = &domain.Organization{ID: orgID, CanonicalName: "单位A", Status: domain.OrgStatusActive}
	store.domains[orgID.Hex()+":a.edu.cn"] = &domain.OrganizationDomain{ID: bson.NewObjectID(), OrganizationID: orgID, RootDomain: "a.edu.cn", Status: domain.DomainStatusConfirmed}
	submit := &fakeScanSubmitter{}
	svc := &Service{Store: store, Submitter: submit}
	_, _, err := svc.CreateScans(context.Background(), orgID, []string{"a.edu.cn", "candidate.edu.cn"}, domain.TaskOptions{}, "", "", "alice")
	if err == nil {
		t.Fatal("expected unconfirmed domain error")
	}
	if len(submit.inputs) != 0 {
		t.Fatalf("tasks were created before validation completed: %+v", submit.inputs)
	}
}
