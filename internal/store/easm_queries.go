package store

import (
	"context"
	"regexp"
	"strings"

	"github.com/red-copilot/red-x/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *Store) ListDomainAssetsUnderRoots(ctx context.Context, roots []string) ([]domain.Asset, error) {
	roots = uniqueStrings(roots)
	if len(roots) == 0 {
		return nil, nil
	}
	var or []bson.M
	for _, root := range roots {
		or = append(or, bson.M{"key": bson.M{"$regex": `(^|\.)` + regexp.QuoteMeta(root) + `$`, "$options": "i"}})
	}
	filter := bson.M{"kind": domain.AssetDomain, "$or": or}
	return s.findAssets(ctx, filter, 5000)
}

func (s *Store) ListAssetsByKindKeys(ctx context.Context, kind domain.AssetKind, keys []string) ([]domain.Asset, error) {
	keys = uniqueStrings(keys)
	if len(keys) == 0 {
		return nil, nil
	}
	return s.findAssets(ctx, bson.M{"kind": kind, "key": bson.M{"$in": keys}}, 5000)
}

func (s *Store) ListServiceAssetsByIPs(ctx context.Context, ips []string) ([]domain.Asset, error) {
	ips = uniqueStrings(ips)
	if len(ips) == 0 {
		return nil, nil
	}
	return s.findAssets(ctx, bson.M{"kind": domain.AssetService, "data.ip": bson.M{"$in": ips}}, 5000)
}

func (s *Store) ListFindingsByTargets(ctx context.Context, targets []string) ([]domain.Finding, error) {
	targets = uniqueStrings(targets)
	if len(targets) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool)
	var out []domain.Finding
	for start := 0; start < len(targets); start += 200 {
		end := start + 200
		if end > len(targets) {
			end = len(targets)
		}
		batch := targets[start:end]
		escaped := make([]string, 0, len(batch))
		for _, t := range batch {
			escaped = append(escaped, regexp.QuoteMeta(t))
		}
		re := `(?i)(^|[./])(` + strings.Join(escaped, "|") + `)([:/?#]|$)`
		filter := bson.M{"$or": []bson.M{
			{"target": bson.M{"$in": batch}},
			{"target": bson.M{"$regex": re}},
		}}
		cur, err := s.C(ColFindings).Find(ctx, filter, options.Find().SetLimit(5000))
		if err != nil {
			return nil, err
		}
		var items []domain.Finding
		if err := cur.All(ctx, &items); err != nil {
			cur.Close(ctx)
			return nil, err
		}
		_ = cur.Close(ctx)
		for _, f := range items {
			if !seen[f.ID.Hex()] {
				seen[f.ID.Hex()] = true
				out = append(out, f)
			}
		}
	}
	return out, nil
}

func (s *Store) ListIPLocationsByIPs(ctx context.Context, ips []string) ([]domain.IPLocation, error) {
	ips = uniqueStrings(ips)
	if len(ips) == 0 {
		return nil, nil
	}
	cur, err := s.C(ColIPLocations).Find(ctx, bson.M{"ip": bson.M{"$in": ips}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.IPLocation
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) findAssets(ctx context.Context, filter bson.M, limit int64) ([]domain.Asset, error) {
	cur, err := s.C(ColAssets).Find(ctx, filter, options.Find().SetLimit(limit))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Asset
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
