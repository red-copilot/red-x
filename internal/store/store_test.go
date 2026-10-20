package store

import "testing"

func TestOrderClauseUsesWhitelist(t *testing.T) {
	tests := []struct {
		sort string
		want string
	}{
		{"_id:asc", "id ASC"},
		{"createdAt:asc", "created_at ASC, id DESC"},
		{"lastSeen:desc", "last_seen DESC, id DESC"},
		{"createdAt; DROP TABLE users:asc", "created_at ASC, id DESC"},
	}
	for _, tt := range tests {
		if got := OrderClause(tt.sort); got != tt.want {
			t.Fatalf("OrderClause(%q) = %q, want %q", tt.sort, got, tt.want)
		}
	}
}

func TestParseID(t *testing.T) {
	id := "507f1f77bcf86cd799439011"
	got, err := ParseID(id)
	if err != nil || got != id {
		t.Fatalf("ParseID(%q) = %q, %v", id, got, err)
	}
	if _, err := ParseID("not-an-id"); err == nil {
		t.Fatal("expected invalid ID to fail")
	}
}

func TestIlikeEscapesWildcards(t *testing.T) {
	got := ilike(`a%b_c\d`)
	want := `%a\%b\_c\\d%`
	if got != want {
		t.Fatalf("ilike = %q, want %q", got, want)
	}
}
