package easm

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/red-copilot/red-x/internal/platform/config"
)

type Neo4jReader struct {
	driver   neo4j.DriverWithContext
	database string
}

// NewNeo4jReader returns nil when Neo4j is not configured.
func NewNeo4jReader(cfg config.Neo4jConfig) (*Neo4jReader, error) {
	if strings.TrimSpace(cfg.URI) == "" {
		return nil, nil
	}
	user := cfg.User
	if user == "" {
		user = "neo4j"
	}
	db := cfg.Database
	if db == "" {
		db = "neo4j"
	}
	driver, err := neo4j.NewDriverWithContext(cfg.URI, neo4j.BasicAuth(user, cfg.Password, ""))
	if err != nil {
		return nil, err
	}
	return &Neo4jReader{driver: driver, database: db}, nil
}

func (r *Neo4jReader) Close(ctx context.Context) error {
	if r == nil || r.driver == nil {
		return nil
	}
	return r.driver.Close(ctx)
}

func (r *Neo4jReader) query(ctx context.Context, query string, params map[string]any) ([]*neo4j.Record, error) {
	if r == nil || r.driver == nil {
		return nil, ErrGraphUnavailable
	}
	res, err := neo4j.ExecuteQuery[*neo4j.EagerResult](
		ctx, r.driver, query, params, neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithDatabase(r.database),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGraphUnavailable, err)
	}
	return res.Records, nil
}

func (r *Neo4jReader) ReadOrganizationGraph(ctx context.Context, orgID string, depth, limit int) ([]GraphNode, []GraphEdge, bool, error) {
	if depth > 3 {
		return nil, nil, false, ErrInvalidDepth
	}
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	structure, err := r.query(ctx, `
MATCH (o:Organization {id:$orgId})
OPTIONAL MATCH (o)-[:OWNS]->(d:Domain)
OPTIONAL MATCH (d)-[:RESOLVES_TO]->(ip:IP)
OPTIONAL MATCH (ip)-[:EXPOSES]->(s:Service)
RETURN o.id AS orgId, o.name AS orgName,
       d.name AS domain, ip.addr AS ip, ip.cdn AS cdn,
       s.key AS service, s.port AS port, s.service AS serviceName
LIMIT $limit`, map[string]any{"orgId": orgID, "limit": limit})
	if err != nil {
		return nil, nil, false, err
	}
	truncated := len(structure) >= limit
	nodeByID := make(map[string]GraphNode)
	edgeByID := make(map[string]GraphEdge)
	riskByTarget := make(map[string]string)
	addNode := func(n GraphNode) { nodeByID[n.ID] = n }
	addEdge := func(e GraphEdge) { edgeByID[e.ID] = e }

	orgName := ""
	for _, rec := range structure {
		oid := recordString(rec, "orgId")
		if oid == "" {
			continue
		}
		if n := recordString(rec, "orgName"); n != "" {
			orgName = n
		}
		addNode(GraphNode{ID: oid, Type: "organization", Label: orgName})
		domain := recordString(rec, "domain")
		if domain == "" {
			continue
		}
		domainID := "domain:" + domain
		addNode(GraphNode{ID: domainID, Type: "domain", Label: domain})
		addEdge(GraphEdge{ID: "owns:" + oid + ":" + domainID, Type: "OWNS", Source: oid, Target: domainID})
		ip := recordString(rec, "ip")
		if ip == "" {
			continue
		}
		ipID := "ip:" + ip
		addNode(GraphNode{ID: ipID, Type: "ip", Label: ip, CDN: recordBool(rec, "cdn")})
		addEdge(GraphEdge{ID: "res:" + domainID + ":" + ipID, Type: "RESOLVES_TO", Source: domainID, Target: ipID})
		service := recordString(rec, "service")
		if service == "" {
			continue
		}
		serviceID := "service:" + service
		addNode(GraphNode{ID: serviceID, Type: "service", Label: service})
		addEdge(GraphEdge{ID: "exp:" + ipID + ":" + serviceID, Type: "EXPOSES", Source: ipID, Target: serviceID})
	}

	findings, err := r.query(ctx, `
MATCH (o:Organization {id:$orgId})-[:OWNS]->(d:Domain)
MATCH (f:Finding)-[:FOUND_ON]->(d)
RETURN f.id AS id, f.kind AS kind, f.severity AS severity, f.title AS title, d.name AS target
LIMIT $limit`, map[string]any{"orgId": orgID, "limit": limit})
	if err != nil {
		return nil, nil, false, err
	}
	if len(findings) >= limit {
		truncated = true
	}
	for _, rec := range findings {
		addFindingNode(nodeByID, edgeByID, riskByTarget, rec, "domain")
	}
	ipFindings, err := r.query(ctx, `
MATCH (o:Organization {id:$orgId})-[:OWNS]->(:Domain)-[:RESOLVES_TO]->(ip:IP)
MATCH (f:Finding)-[:FOUND_ON]->(ip)
RETURN f.id AS id, f.kind AS kind, f.severity AS severity, f.title AS title, ip.addr AS target
LIMIT $limit`, map[string]any{"orgId": orgID, "limit": limit})
	if err != nil {
		return nil, nil, false, err
	}
	if len(ipFindings) >= limit {
		truncated = true
	}
	for _, rec := range ipFindings {
		addFindingNode(nodeByID, edgeByID, riskByTarget, rec, "ip")
	}

	nodes := make([]GraphNode, 0, len(nodeByID))
	for _, n := range nodeByID {
		if r := riskByTarget[n.ID]; r != "" {
			n.Risk = r
		}
		nodes = append(nodes, n)
	}
	edges := make([]GraphEdge, 0, len(edgeByID))
	for _, e := range edgeByID {
		edges = append(edges, e)
	}
	return nodes, edges, truncated, nil
}

func addFindingNode(nodes map[string]GraphNode, edges map[string]GraphEdge, riskByTarget map[string]string, rec *neo4j.Record, targetType string) {
	id := recordString(rec, "id")
	target := recordString(rec, "target")
	if id == "" || target == "" {
		return
	}
	nodeID := "finding:" + id
	nodes[nodeID] = GraphNode{ID: nodeID, Type: "finding", Label: recordString(rec, "title")}
	targetID := targetType + ":" + target
	if _, ok := nodes[targetID]; !ok {
		nodes[targetID] = GraphNode{ID: targetID, Type: targetType, Label: target}
	}
	edges["found:"+nodeID+":"+targetID] = GraphEdge{
		ID: "found:" + nodeID + ":" + targetID, Type: "FOUND_ON", Source: nodeID, Target: targetID,
	}
	sev := recordString(rec, "severity")
	if RiskRank(sev) > RiskRank(riskByTarget[targetID]) {
		riskByTarget[targetID] = strings.ToLower(sev)
	}
}

func recordString(rec *neo4j.Record, key string) string {
	v, ok := rec.Get(key)
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

func recordBool(rec *neo4j.Record, key string) bool {
	v, ok := rec.Get(key)
	if !ok || v == nil {
		return false
	}
	b, _ := v.(bool)
	return b
}
