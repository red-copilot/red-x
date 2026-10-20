package domain

import (
	"testing"
	"time"
)

func TestLifecycleTracksDeleteAndRestore(t *testing.T) {
	var lifecycle Lifecycle
	deletedAt := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)

	lifecycle.MarkDeleted(deletedAt, "operator")
	if !lifecycle.Deleted() {
		t.Fatal("lifecycle should be deleted")
	}
	if lifecycle.DeletedBy != "operator" || lifecycle.DeletedAt == nil || !lifecycle.DeletedAt.Equal(deletedAt) {
		t.Fatalf("unexpected delete state: %#v", lifecycle)
	}

	restoredAt := deletedAt.Add(time.Hour)
	lifecycle.MarkRestored(restoredAt, "manual")
	if lifecycle.Deleted() {
		t.Fatal("lifecycle should be active after restore")
	}
	if lifecycle.DeletedAt != nil || lifecycle.DeletedBy != "" {
		t.Fatalf("delete fields were not cleared: %#v", lifecycle)
	}
	if lifecycle.RestoredAt == nil || !lifecycle.RestoredAt.Equal(restoredAt) || lifecycle.RestoreReason != "manual" {
		t.Fatalf("unexpected restore state: %#v", lifecycle)
	}
}
