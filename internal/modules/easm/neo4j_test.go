package easm

import "testing"

func TestFindingTargetNode(t *testing.T) {
	cases := []struct {
		target string
		label  string
		value  string
	}{
		{"https://a.example.com/path?q=1", "Domain", "a.example.com"},
		{"8.8.8.8:443", "IP", "8.8.8.8"},
		{"example.com.", "Domain", "example.com"},
		{"not a target", "", ""},
	}
	for _, tc := range cases {
		label, value := findingTargetNode(tc.target)
		if label != tc.label || value != tc.value {
			t.Fatalf("findingTargetNode(%q) = (%q,%q), want (%q,%q)", tc.target, label, value, tc.label, tc.value)
		}
	}
}

func TestStringSlice(t *testing.T) {
	if got := stringSlice([]any{"1.1.1.1", "2.2.2.2"}); len(got) != 2 || got[1] != "2.2.2.2" {
		t.Fatalf("unexpected slice: %v", got)
	}
	if got := stringSlice(nil); len(got) != 0 {
		t.Fatalf("unexpected nil slice: %v", got)
	}
}
