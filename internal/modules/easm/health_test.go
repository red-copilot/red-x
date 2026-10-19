package easm

import (
	"context"
	"errors"
	"testing"
)

type fakeProbe struct{ err error }

func (f fakeProbe) Probe(context.Context) error { return f.err }

func TestHealthActivelyProbesNeo4jAndICPQuery(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil)
	svc.ChinazConfigured = true
	svc.ICPQuery = fakeOrganizationProbe{err: nil}
	svc.GraphReader = fakeGraphProbe{err: errors.New("down")}
	got := svc.Health(context.Background())
	if got.Chinaz != "unknown" || got.ICPQuery != "ok" || got.Neo4j != "unavailable" {
		t.Fatalf("unexpected health: %+v", got)
	}
}

type fakeOrganizationProbe struct{ err error }

func (f fakeOrganizationProbe) SearchByOrganization(context.Context, string) ([]OrgDomainRecord, error) {
	return nil, f.err
}
func (f fakeOrganizationProbe) Probe(context.Context) error { return f.err }

type fakeGraphProbe struct{ err error }

func (f fakeGraphProbe) ReadOrganizationGraph(context.Context, string, int, int) ([]GraphNode, []GraphEdge, bool, error) {
	return nil, nil, false, f.err
}
func (f fakeGraphProbe) Probe(context.Context) error { return f.err }
