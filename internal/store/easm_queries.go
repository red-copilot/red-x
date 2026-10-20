package store

import (
	"context"
	"regexp"
	"strings"

	"github.com/red-copilot/red-x/internal/domain"
)

func (s *Store) ListDomainAssetsUnderRoots(ctx context.Context, roots []string) ([]domain.Asset, error) {
	roots = uniqueStrings(roots)
	if len(roots) == 0 {
		return nil, nil
	}
	var patterns []string
	for _, root := range roots {
		patterns = append(patterns, `(^|\.)`+regexp.QuoteMeta(strings.ToLower(root))+`$`)
	}
	var out []domain.Asset
	err := active(s.db.WithContext(ctx)).Where("kind = ?", domain.AssetDomain).
		Where("key ~* ?", strings.Join(patterns, "|")).Limit(5000).Find(&out).Error
	return out, err
}

func (s *Store) ListAssetsByKindKeys(ctx context.Context, kind domain.AssetKind, keys []string) ([]domain.Asset, error) {
	keys = uniqueStrings(keys)
	if len(keys) == 0 {
		return nil, nil
	}
	var out []domain.Asset
	err := active(s.db.WithContext(ctx)).Where("kind = ? AND key IN ?", kind, keys).Limit(5000).Find(&out).Error
	return out, err
}

func (s *Store) ListServiceAssetsByIPs(ctx context.Context, ips []string) ([]domain.Asset, error) {
	ips = uniqueStrings(ips)
	if len(ips) == 0 {
		return nil, nil
	}
	var out []domain.Asset
	err := active(s.db.WithContext(ctx)).Where("kind = ? AND data->>'ip' IN ?", domain.AssetService, ips).Limit(5000).Find(&out).Error
	return out, err
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
		for _, target := range batch {
			escaped = append(escaped, regexp.QuoteMeta(target))
		}
		pattern := `(?i)(^|[./])(` + strings.Join(escaped, "|") + `)([:/?#]|$)`
		var items []domain.Finding
		if err := s.db.WithContext(ctx).Where("target IN ? OR target ~ ?", batch, pattern).Limit(5000).Find(&items).Error; err != nil {
			return nil, err
		}
		for _, finding := range items {
			if !seen[finding.ID] {
				seen[finding.ID] = true
				out = append(out, finding)
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
	var out []domain.IPLocation
	err := s.db.WithContext(ctx).Where("ip IN ?", ips).Find(&out).Error
	return out, err
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
