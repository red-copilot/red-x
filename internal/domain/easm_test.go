package domain

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestOrganizationValidateRejectsEmptyName(t *testing.T) {
	if err := (&Organization{}).Validate(); err == nil {
		t.Fatal("expected empty canonical name to be rejected")
	}
	if err := (&Organization{CanonicalName: "四川信息职业技术学院"}).Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestOrganizationDomainValidateRejectsInvalidStatus(t *testing.T) {
	od := &OrganizationDomain{
		OrganizationID: bson.NewObjectID(),
		RootDomain:     "scitc.edu.cn",
		Source:         "manual",
		Confidence:     ConfidenceHigh,
		Status:         DomainStatus("bogus"),
	}
	if err := od.Validate(); err == nil {
		t.Fatal("expected invalid status to be rejected")
	}
	od.Status = DomainStatusCandidate
	if err := od.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if err := (&OrganizationDomain{OrganizationID: bson.NewObjectID(), RootDomain: "", Status: DomainStatusCandidate}).Validate(); err == nil {
		t.Fatal("expected empty root domain to be rejected")
	}
}

func TestICPJobStatusTransitions(t *testing.T) {
	tests := []struct {
		from ICPJobStatus
		to   ICPJobStatus
		ok   bool
	}{
		{ICPJobQueued, ICPJobRunning, true},
		{ICPJobRunning, ICPJobDone, true},
		{ICPJobRunning, ICPJobFailed, true},
		{ICPJobQueued, ICPJobDone, false},
		{ICPJobDone, ICPJobRunning, false},
		{ICPJobFailed, ICPJobQueued, false},
	}
	for _, tt := range tests {
		if got := tt.from.CanTransition(tt.to); got != tt.ok {
			t.Fatalf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.ok)
		}
	}
}

func TestGraphOutboxValidOps(t *testing.T) {
	if !OutboxOpUpsert.Valid() {
		t.Fatal("upsert op should be valid")
	}
	if !OutboxOpDelete.Valid() {
		t.Fatal("delete op should be valid")
	}
	if GraphOutboxOp("merge").Valid() {
		t.Fatal("unknown op should be invalid")
	}
}
