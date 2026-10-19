package easm

import (
	"testing"
	"time"
)

func TestFindingTargetNode(t *testing.T) {
	cases := []struct {
		target string
		label  string
		value  string
		port   string
	}{
		{"https://a.example.com/path?q=1", "Domain", "a.example.com", ""},
		{"8.8.8.8:443", "IP", "8.8.8.8", "443"},
		{"https://1.1.1.1:8443/x", "IP", "1.1.1.1", "8443"},
		{"example.com.", "Domain", "example.com", ""},
		{"not a target", "", "", ""},
	}
	for _, tc := range cases {
		label, value, port := findingTargetNode(tc.target)
		if label != tc.label || value != tc.value || port != tc.port {
			t.Fatalf("findingTargetNode(%q) = (%q,%q,%q), want (%q,%q,%q)", tc.target, label, value, port, tc.label, tc.value, tc.port)
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

func TestCapGraphKeepsOrganizationAndFiltersEdges(t *testing.T) {
	nodes := []GraphNode{
		{ID: "o", Type: "organization"},
		{ID: "d1", Type: "domain"},
		{ID: "d2", Type: "domain"},
		{ID: "f1", Type: "finding"},
	}
	edges := []GraphEdge{
		{ID: "e1", Source: "o", Target: "d1"},
		{ID: "e2", Source: "d2", Target: "f1"},
	}
	kept, out, capped := capGraph(nodes, edges, 2)
	if !capped || len(kept) != 2 || kept[0].ID != "o" {
		t.Fatalf("unexpected cap: capped=%v kept=%+v", capped, kept)
	}
	if len(out) != 1 || out[0].ID != "e1" {
		t.Fatalf("dangling edges survived cap: %+v", out)
	}
}

func TestLatestRiskByTarget(t *testing.T) {
	now := time.Now().UTC()
	risks := []findingRisk{
		{targetID: "ip:1", dedupeKey: "a", severity: "high", createdAt: now.Add(-time.Hour)},
		{targetID: "ip:1", dedupeKey: "a", severity: "low", createdAt: now},
		{targetID: "ip:1", dedupeKey: "b", severity: "critical", createdAt: now.Add(-2 * time.Hour)},
		{targetID: "ip:1", dedupeKey: "c", severity: "info", createdAt: now},
	}
	if got := latestRiskByTarget(risks)["ip:1"]; got != "critical" {
		t.Fatalf("unexpected risk: %q", got)
	}
}
