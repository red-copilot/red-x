package easm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/config"
)

type Neo4jWriter struct {
	driver     neo4j.DriverWithContext
	database   string
	allowReset bool
}

// NewNeo4jWriter returns nil when Neo4j is not configured.
func NewNeo4jWriter(cfg config.Neo4jConfig) (*Neo4jWriter, error) {
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
	return &Neo4jWriter{driver: driver, database: db, allowReset: cfg.AllowRebuild}, nil
}

func (w *Neo4jWriter) Close(ctx context.Context) error {
	if w == nil || w.driver == nil {
		return nil
	}
	return w.driver.Close(ctx)
}

func (w *Neo4jWriter) exec(ctx context.Context, query string, params map[string]any) error {
	if w == nil || w.driver == nil {
		return errors.New("neo4j writer is not configured")
	}
	_, err := neo4j.ExecuteQuery[*neo4j.EagerResult](
		ctx, w.driver, query, params, neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithDatabase(w.database),
	)
	return err
}

func (w *Neo4jWriter) MergeOrganization(ctx context.Context, org domain.Organization) error {
	return w.exec(ctx, `MERGE (o:Organization {id:$id}) SET o.name=$name`,
		map[string]any{"id": org.ID, "name": org.CanonicalName})
}

func (w *Neo4jWriter) MergeOrganizationDomain(ctx context.Context, od domain.OrganizationDomain) error {
	if od.Status != domain.DomainStatusConfirmed {
		return nil
	}
	return w.exec(ctx, `
MERGE (o:Organization {id:$orgId})
MERGE (d:Domain {name:$root})
SET d.root=$root
MERGE (o)-[r:OWNS {entityId:$id}]->(d)
SET r.source=$source, r.confidence=$confidence, r.status=$status, r.icpNo=$icpNo`,
		map[string]any{
			"orgId": od.OrganizationID, "root": od.RootDomain, "id": od.ID,
			"source": od.Source, "confidence": string(od.Confidence),
			"status": string(od.Status), "icpNo": od.ICPNo,
		})
}

func (w *Neo4jWriter) MergeAsset(ctx context.Context, asset domain.Asset) error {
	switch asset.Kind {
	case domain.AssetDomain:
		root := ""
		if r, err := NormalizeRootDomain(asset.Key); err == nil {
			root = r
		}
		ips := stringSlice(asset.Data["ips"])
		cdn, cdnName := cdnOf(asset.Data)
		return w.exec(ctx, `
MERGE (d:Domain {name:$key})
SET d.root=$root, d.entityId=$id, d.cdn=$cdn
WITH d
UNWIND $ips AS ipAddr
MERGE (ip:IP {addr:ipAddr})
SET ip.cdn = CASE WHEN ip.cdn THEN true ELSE $cdn END,
    ip.cdnName = CASE WHEN ip.cdnName IS NULL OR ip.cdnName = '' THEN $cdnName ELSE ip.cdnName END
MERGE (d)-[:RESOLVES_TO]->(ip)`,
			map[string]any{
				"key": asset.Key, "root": root, "id": asset.ID,
				"cdn": cdn, "cdnName": cdnName, "ips": ips,
			})
	case domain.AssetIP:
		cdn, cdnName := cdnOf(asset.Data)
		return w.exec(ctx, `MERGE (ip:IP {addr:$key})
SET ip.entityId=$id,
    ip.cdn = CASE WHEN ip.cdn THEN true ELSE $cdn END,
    ip.cdnName = CASE WHEN ip.cdnName IS NULL OR ip.cdnName = '' THEN $cdnName ELSE ip.cdnName END`,
			map[string]any{
				"key": asset.Key, "id": asset.ID, "cdn": cdn, "cdnName": cdnName,
			})
	case domain.AssetService:
		return w.exec(ctx, `
MERGE (s:Service {key:$key})
SET s.entityId=$id, s.port=$port, s.protocol=$protocol, s.service=$service
WITH s
MERGE (ip:IP {addr:$ip})
MERGE (ip)-[:EXPOSES]->(s)`,
			map[string]any{
				"key": asset.Key, "id": asset.ID,
				"ip": stringValue(asset.Data["ip"]), "port": asset.Data["port"],
				"protocol": stringValue(asset.Data["protocol"]), "service": stringValue(asset.Data["serviceName"]),
			})
	default:
		return nil
	}
}

func (w *Neo4jWriter) MergeFinding(ctx context.Context, finding domain.Finding) error {
	label, target, port := findingTargetNode(finding.Target)
	if target == "" {
		return w.exec(ctx, `MERGE (f:Finding {id:$id}) SET f.kind=$kind, f.severity=$severity, f.title=$title, f.dedupeKey=$dedupeKey, f.createdAt=$createdAt`,
			map[string]any{
				"id": finding.ID, "kind": string(finding.Kind),
				"severity": finding.Severity, "title": finding.Title,
				"dedupeKey": finding.DedupeKey, "createdAt": finding.CreatedAt,
			})
	}
	if label == "IP" && port != "" {
		if portNum, err := strconv.Atoi(port); err == nil {
			return w.exec(ctx, `
MERGE (f:Finding {id:$id})
SET f.kind=$kind, f.severity=$severity, f.title=$title, f.dedupeKey=$dedupeKey, f.createdAt=$createdAt
WITH f
MERGE (ip:IP {addr:$target})
MERGE (f)-[:FOUND_ON]->(ip)
WITH f, ip
OPTIONAL MATCH (ip)-[:EXPOSES]->(s:Service {port:$port})
FOREACH (_ IN CASE WHEN s IS NULL THEN [] ELSE [1] END | MERGE (f)-[:FOUND_ON]->(s))`,
				map[string]any{
					"id": finding.ID, "kind": string(finding.Kind),
					"severity": finding.Severity, "title": finding.Title, "target": target,
					"dedupeKey": finding.DedupeKey, "createdAt": finding.CreatedAt, "port": portNum,
				})
		}
	}
	query := fmt.Sprintf(`
MERGE (f:Finding {id:$id})
SET f.kind=$kind, f.severity=$severity, f.title=$title, f.dedupeKey=$dedupeKey, f.createdAt=$createdAt
WITH f
MERGE (n:%s {%s:$target})
MERGE (f)-[:FOUND_ON]->(n)`, label, targetKey(label))
	return w.exec(ctx, query, map[string]any{
		"id": finding.ID, "kind": string(finding.Kind),
		"severity": finding.Severity, "title": finding.Title, "target": target,
		"dedupeKey": finding.DedupeKey, "createdAt": finding.CreatedAt,
	})
}

func (w *Neo4jWriter) DeleteEntityEdges(ctx context.Context, entityType, entityID string) error {
	switch entityType {
	case "asset":
		return w.exec(ctx, `
MATCH (n {entityId:$id})
WHERE NOT n:IP
OPTIONAL MATCH (n)-[r:RESOLVES_TO]->()
DELETE r
WITH n
OPTIONAL MATCH ()-[r2:EXPOSES]->(n)
DELETE r2`, map[string]any{"id": entityID})
	case "finding":
		return w.exec(ctx, `MATCH (f:Finding {id:$id}) DETACH DELETE f`, map[string]any{"id": entityID})
	case "organization_domain":
		return w.exec(ctx, `MATCH ()-[r:OWNS {entityId:$id}]->() DELETE r`, map[string]any{"id": entityID})
	case "organization":
		return w.exec(ctx, `MATCH (o:Organization {id:$id}) DETACH DELETE o`, map[string]any{"id": entityID})
	default:
		return nil
	}
}

func (w *Neo4jWriter) Reset(ctx context.Context) error {
	if !w.allowReset {
		return errors.New("neo4j rebuild is disabled; set neo4j.allowRebuild=true")
	}
	return w.exec(ctx, `MATCH (n) WHERE n:Organization OR n:Domain OR n:IP OR n:Service OR n:Finding DETACH DELETE n`, nil)
}

func findingTargetNode(raw string) (label, target, port string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", ""
	}
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil {
			s = u.Host
		}
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if host, p, err := net.SplitHostPort(s); err == nil {
		s, port = host, p
	}
	s = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
	if s == "" {
		return "", "", ""
	}
	if ip, err := netip.ParseAddr(s); err == nil && ip.IsValid() {
		return "IP", ip.String(), port
	}
	if strings.Contains(s, ".") {
		return "Domain", s, port
	}
	return "", "", ""
}

func targetKey(label string) string {
	if label == "IP" {
		return "addr"
	}
	return "name"
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func stringSlice(v any) []string {
	switch vv := v.(type) {
	case []string:
		return vv
	case []any:
		out := make([]string, 0, len(vv))
		for _, item := range vv {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
