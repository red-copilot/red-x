package domain

import "testing"

func TestNewIDProduces24LowercaseHex(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	if len(id) != 24 {
		t.Fatalf("len(NewID()) = %d, want 24", len(id))
	}
	if _, err := ParseID(id); err != nil {
		t.Fatalf("ParseID(NewID()) failed: %v", err)
	}
	second, err := NewID()
	if err != nil {
		t.Fatalf("NewID second: %v", err)
	}
	if second == id {
		t.Fatal("NewID returned duplicate IDs")
	}
}

func TestParseIDRejectsInvalidValues(t *testing.T) {
	for _, raw := range []string{"", "abc", "zzzzzzzzzzzzzzzzzzzzzzzz"} {
		if _, err := ParseID(raw); err == nil {
			t.Fatalf("ParseID(%q) succeeded, want error", raw)
		}
	}
}

func TestParseIDNormalizesUppercase(t *testing.T) {
	got, err := ParseID("507F1F77BCF86CD799439011")
	if err != nil || got != "507f1f77bcf86cd799439011" {
		t.Fatalf("ParseID uppercase = %q, %v", got, err)
	}
}
