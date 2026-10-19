package store

import "testing"

func TestSortDocumentDoesNotDuplicateID(t *testing.T) {
	got := SortDocument("_id:asc")
	if len(got) != 1 || got[0].Key != "_id" || got[0].Value != 1 {
		t.Fatalf("unexpected sort document: %#v", got)
	}
	got = SortDocument("createdAt:asc")
	if len(got) != 2 || got[0].Key != "createdAt" || got[1].Key != "_id" {
		t.Fatalf("unexpected secondary sort: %#v", got)
	}
}
