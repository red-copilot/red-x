package easm

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/config"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestNeo4jProjectionRoundTrip(t *testing.T) {
	uri := os.Getenv("ARL_TEST_NEO4J_URI")
	if uri == "" {
		t.Skip("set ARL_TEST_NEO4J_URI to run Neo4j integration tests")
	}
	cfg := config.Neo4jConfig{URI: uri, User: os.Getenv("ARL_TEST_NEO4J_USER"), Password: os.Getenv("ARL_TEST_NEO4J_PASSWORD"), Database: os.Getenv("ARL_TEST_NEO4J_DATABASE")}
	if cfg.User == "" {
		cfg.User = "neo4j"
	}
	if cfg.Database == "" {
		cfg.Database = "neo4j"
	}
	reader, err := NewNeo4jReader(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if reader == nil {
		t.Fatal("reader is nil")
	}
	defer reader.Close(context.Background())
	writer, err := NewNeo4jWriter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if writer == nil {
		t.Fatal("writer is nil")
	}
	defer writer.Close(context.Background())
	orgID := bson.NewObjectID()
	org := domain.Organization{ID: orgID, CanonicalName: "EASM integration " + orgID.Hex(), Status: domain.OrgStatusActive}
	root := fmt.Sprintf("easm-%d.invalid", time.Now().UnixNano())
	if err := writer.MergeOrganization(context.Background(), org); err != nil {
		t.Fatal(err)
	}
	if err := writer.MergeOrganizationDomain(context.Background(), domain.OrganizationDomain{
		ID: bson.NewObjectID(), OrganizationID: orgID, RootDomain: root,
		Source: "integration", Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusConfirmed,
	}); err != nil {
		t.Fatal(err)
	}
	assetID := bson.NewObjectID()
	if err := writer.MergeAsset(context.Background(), domain.Asset{
		ID: assetID, Kind: domain.AssetDomain, Key: root, Data: map[string]any{"ips": []any{"1.1.1.1"}},
	}); err != nil {
		t.Fatal(err)
	}
	defer writer.DeleteEntityEdges(context.Background(), "organization", orgID.Hex())
	if err := reader.Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
	nodes, edges, _, err := reader.ReadOrganizationGraph(context.Background(), orgID.Hex(), 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) < 2 || len(edges) < 1 {
		t.Fatalf("unexpected graph nodes=%+v edges=%+v", nodes, edges)
	}
	foundDomain := false
	for _, node := range nodes {
		if node.Type == "domain" && node.EntityID == assetID.Hex() {
			foundDomain = true
		}
	}
	if !foundDomain {
		t.Fatalf("domain Mongo id missing from graph nodes: %+v", nodes)
	}
}
