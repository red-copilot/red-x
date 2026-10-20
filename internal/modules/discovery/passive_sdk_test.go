package discovery

import (
	"testing"

	"github.com/projectdiscovery/subfinder/v2/pkg/resolve"
)

func TestPassiveDomainResults(t *testing.T) {
	entries := []*resolve.HostEntry{
		{Host: "WWW.Example.com", Source: "crtsh"},
		{Host: "www.example.com.", Source: "crtsh"},
		{Host: "api.example.com", Source: "rapiddns"},
		{Host: "example.net", Source: "alienvault"},
		{Host: "example.com", Source: "crtsh"},
	}
	got := passiveDomainResults(entries, "example.com")
	if len(got) != 2 {
		t.Fatalf("passive results = %+v", got)
	}
	if got[0].Domain != "www.example.com" || got[0].Source != "crtsh" {
		t.Fatalf("first result = %+v", got[0])
	}
	if got[1].Domain != "api.example.com" || got[1].Source != "rapiddns" {
		t.Fatalf("second result = %+v", got[1])
	}
}

func TestAppendSubfinderEntryHonorsLimit(t *testing.T) {
	var entries []*resolve.HostEntry
	if !appendSubfinderEntry(&entries, &resolve.HostEntry{Host: "one.example.com"}, 2) {
		t.Fatal("first entry should be accepted")
	}
	if !appendSubfinderEntry(&entries, &resolve.HostEntry{Host: "two.example.com"}, 2) {
		t.Fatal("second entry should be accepted")
	}
	if appendSubfinderEntry(&entries, &resolve.HostEntry{Host: "three.example.com"}, 2) {
		t.Fatal("entry over the limit should be rejected")
	}
	if len(entries) != 2 {
		t.Fatalf("collected %d entries, want 2", len(entries))
	}
}

func TestNewSubfinderEnumeratorAllSources(t *testing.T) {
	tests := []struct {
		name        string
		all         bool
		sources     []string
		wantAll     bool
		wantSources int
	}{
		{name: "explicit all", all: true, sources: []string{"crtsh"}, wantAll: true, wantSources: 0},
		{name: "all sentinel", sources: []string{"crtsh", "ALL"}, wantAll: true, wantSources: 0},
		{name: "explicit sources", sources: []string{"crtsh", "rapiddns"}, wantAll: false, wantSources: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			enumerator := NewSubfinderEnumerator(tc.all, tc.sources, 10, 30, 5, "")
			if enumerator.All != tc.wantAll {
				t.Fatalf("All = %v, want %v", enumerator.All, tc.wantAll)
			}
			if len(enumerator.Sources) != tc.wantSources {
				t.Fatalf("Sources = %v, want %d entries", enumerator.Sources, tc.wantSources)
			}
		})
	}
}
