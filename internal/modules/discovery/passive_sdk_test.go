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
