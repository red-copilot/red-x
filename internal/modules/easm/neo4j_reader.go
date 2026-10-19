package easm

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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

type findingRisk struct {
	targetID  string
	dedupeKey string
	severity  string
	createdAt time.Time
}

func (r *Neo4jReader) ReadOrganizationGraph(ctx context.Context, orgID string, depth, limit int) ([]GraphNode, []GraphEdge, bool, error) {
	if depth <= 0 {
		depth = 2
	}
	if depth > 3 {
		return nil, nil, false, ErrInvalidDepth
	}
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	structureQuery := `
MATCH (o:Organization {id:$orgId})
OPTIONAL MATCH (o)-[:OWNS]->(d:Domain)
RETURN o.id AS orgId, o.name AS orgName, d.name AS domain,
       null AS ip, null AS cdn, null AS service, null AS port, null AS serviceName
LIMIT $limit`
	if depth >= 2 {
		structureQuery = `
MATCH (o:Organization {id:$orgId})
OPTIONAL MATCH (o)-[:OWNS]->(d:Domain)
OPTIONAL MATCH (d)-[:RESOLVES_TO]->(ip:IP)
RETURN o.id AS orgId, o.name AS orgName, d.name AS domain,
       ip.addr AS ip, ip.cdn AS cdn, null AS service, null AS port, null AS serviceName
LIMIT $limit`
	}
	if depth >= 3 {
		structureQuery = `
MATCH (o:Organization {id:$orgId})
OPTIONAL MATCH (o)-[:OWNS]->(d:Domain)
OPTIONAL MATCH (d)-[:RESOLVES_TO]->(ip:IP)
OPTIONAL MATCH (ip)-[:EXPOSES]->(s:Service)
RETURN o.id AS orgId, o.name AS orgName, d.name AS domain,
       ip.addr AS ip, ip.cdn AS cdn, s.key AS service, s.port AS port, s.service AS serviceName
LIMIT $limit`
	}
	structure, err := r.query(ctx, structureQuery, map[string]any{"orgId": orgID, "limit": limit})
	if err != nil {
		return nil, nil, false, err
	}
	truncated := len(structure) >= limit
	nodeByID := make(map[string]GraphNode)
	edgeByID := make(map[string]GraphEdge)
	risks := make([]findingRisk, 0)
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
		domain := recordString(rec, "domain")
		ip := recordString(rec, "ip")
		service := recordString(rec, "service")
		if ip != "" {
			// At depth >= 2 every row carries an IP; the org/domain nodes are still unique.
			_ = domain
		}
		orgNodeID := oid
		addNode(GraphNode{ID: orgNodeID, Type: "organization", Label: orgName})
		if domain == "" {
			continue
		}
		domainID := "domain:" + domain
		addNode(GraphNode{ID: domainID, Type: "domain", Label: domain})
		addEdge(GraphEdge{ID: "owns:" + orgNodeID + ":" + domainID, Type: "OWNS", Source: orgNodeID, Target: domainID})
		if ip == "" {
			continue
		}
		ipID := "ip:" + ip
		addNode(GraphNode{ID: ipID, Type: "ip", Label: ip, CDN: recordBool(rec, "cdn")})
		addEdge(GraphEdge{ID: "res:" + domainID + ":" + ipID, Type: "RESOLVES_TO", Source: domainID, Target: ipID})
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
RETURN f.id AS id, f.kind AS kind, f.severity AS severity, f.title AS title,
       f.dedupeKey AS dedupeKey, f.createdAt AS createdAt, d.name AS target
LIMIT $limit`, map[string]any{"orgId": orgID, "limit": limit})
	if err != nil {
		return nil, nil, false, err
	}
	if len(findings) >= limit {
		truncated = true
	}
	for _, rec := range findings {
		addFindingNode(nodeByID, edgeByID, &risks, rec, "domain")
	}
	if depth >= 2 {
		ipFindings, err := r.query(ctx, `
MATCH (o:Organization {id:$orgId})-[:OWNS]->(:Domain)-[:RESOLVES_TO]->(ip:IP)
MATCH (f:Finding)-[:FOUND_ON]->(ip)
RETURN f.id AS id, f.kind AS kind, f.severity AS severity, f.title AS title,
       f.dedupeKey AS dedupeKey, f.createdAt AS createdAt, ip.addr AS target
LIMIT $limit`, map[string]any{"orgId": orgID, "limit": limit})
		if err != nil {
			return nil, nil, false, err
		}
		if len(ipFindings) >= limit {
			truncated = true
		}
		for _, rec := range ipFindings {
			addFindingNode(nodeByID, edgeByID, &risks, rec, "ip")
		}
	}

	riskByTarget := latestRiskByTarget(risks)
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
	nodes, edges, capped := capGraph(nodes, edges, limit)
	return nodes, edges, truncated || capped, nil
}

func addFindingNode(nodes map[string]GraphNode, edges map[string]GraphEdge, risks *[]findingRisk, rec *neo4j.Record, targetType string) {
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
	*risks = append(*risks, findingRisk{
		targetID: targetID, dedupeKey: recordString(rec, "dedupeKey"),
		severity: recordString(rec, "severity"), createdAt: recordTime(rec, "createdAt"),
	})
}

// latestRiskByTarget mirrors the map rule: newest finding per dedupe key, then max severity.
func latestRiskByTarget(risks []findingRisk) map[string]string {
	latest := make(map[string]findingRisk)
	for _, r := range risks {
		key := r.targetID + "|" + r.dedupeKey
		if cur, ok := latest[key]; !ok || r.createdAt.After(cur.createdAt) {
			latest[key] = r
		}
	}
	out := make(map[string]string)
	for _, r := range latest {
		if RiskRank(r.severity) <= RiskRank("info") {
			continue
		}
		if RiskRank(r.severity) > RiskRank(out[r.targetID]) {
			out[r.targetID] = strings.ToLower(r.severity)
		}
	}
	return out
}

type nodePriority int

func priorityOf(typ string) nodePriority {
	switch typ {
	case "organization":
		return 0
	case "domain":
		return 1
	case "ip":
		return 2
	case "service":
		return 3
	default:
		return 4
	}
}

func capGraph(nodes []GraphNode, edges []GraphEdge, limit int) ([]GraphNode, []GraphEdge, bool) {
	if limit <= 0 || len(nodes) <= limit {
		return nodes, edges, false
	}
	sort.Slice(nodes, func(i, j int) bool {
		pi, pj := priorityOf(nodes[i].Type), priorityOf(nodes[j].Type)
		if pi != pj {
			return pi < pj
		}
		return nodes[i].ID < nodes[j].ID
	})
	kept := nodes[:limit]
	keep := make(map[string]bool, len(kept))
	for _, n := range kept {
		keep[n.ID] = true
	}
	out := make([]GraphEdge, 0, len(edges))
	for _, e := range edges {
		if keep[e.Source] && keep[e.Target] {
			out = append(out, e)
		}
	}
	return kept, out, true
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

func recordTime(rec *neo4j.Record, key string) time.Time {
	v, ok := rec.Get(key)
	if !ok || v == nil {
		return time.Time{}
	}
	switch x := v.(type) {
	case time.Time:
		return x
	case string:
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.999999999Z0700"} {
			if t, err := time.Parse(layout, x); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}
