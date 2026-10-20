package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) UpsertAsset(ctx context.Context, kind domain.AssetKind, key string, data map[string]any, taskID string, scopeID string) (*domain.Asset, error) {
	now := time.Now().UTC()
	id, err := domain.NewID()
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	scopeIDs := []string{}
	if scopeID != "" {
		scopeIDs = []string{scopeID}
	}
	scopeJSON, err := json.Marshal(scopeIDs)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Exec(`
INSERT INTO assets (id, kind, key, data, scope_ids, origin, first_seen, last_seen, last_task_id, schema_version)
VALUES (?, ?, ?, ?::jsonb, ?::jsonb, ?, ?, ?, ?, ?)
ON CONFLICT (kind, key) DO UPDATE SET
  data = EXCLUDED.data,
  last_seen = EXCLUDED.last_seen,
  last_task_id = EXCLUDED.last_task_id,
  deleted_at = NULL,
  deleted_by = '',
  restored_at = CASE WHEN assets.deleted_at IS NOT NULL THEN EXCLUDED.last_seen ELSE assets.restored_at END,
  restore_reason = CASE WHEN assets.deleted_at IS NOT NULL THEN 'scan_rediscovery' ELSE assets.restore_reason END,
  scope_ids = (
    SELECT COALESCE(jsonb_agg(DISTINCT item), '[]'::jsonb)
    FROM jsonb_array_elements_text(COALESCE(assets.scope_ids, '[]'::jsonb) || COALESCE(EXCLUDED.scope_ids, '[]'::jsonb)) AS item
  )`, id, kind, key, string(raw), string(scopeJSON), domain.AssetOriginScan, now, now, taskID, domain.SchemaVersion).Error; err != nil {
		return nil, err
	}
	var asset domain.Asset
	if err := active(s.db.WithContext(ctx)).Where("kind = ? AND key = ?", kind, key).First(&asset).Error; err != nil {
		return nil, err
	}
	_ = s.AppendGraphOutbox(ctx, "asset", asset.ID, domain.OutboxOpUpsert)
	return &asset, nil
}

func (s *Store) CreateManualAsset(ctx context.Context, asset *domain.Asset) error {
	if asset == nil || strings.TrimSpace(string(asset.Kind)) == "" || strings.TrimSpace(asset.Key) == "" {
		return errors.New("asset kind and key are required")
	}
	now := time.Now().UTC()
	if asset.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		asset.ID = id
	}
	if asset.Data == nil {
		asset.Data = map[string]any{}
	}
	if asset.Tags == nil {
		asset.Tags = []string{}
	}
	asset.Origin = domain.AssetOriginManual
	asset.FirstSeen, asset.LastSeen, asset.SchemaVersion = now, now, domain.SchemaVersion
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(asset).Error; err != nil {
			if isDuplicate(err) {
				return domain.ErrRecordAlreadyExists
			}
			return err
		}
		return appendGraphOutboxTx(tx, "asset", asset.ID, domain.OutboxOpUpsert)
	}); err != nil {
		return err
	}
	return nil
}

type AssetMetadataPatch struct {
	DisplayName *string
	DisplayURL  *string
	Notes       *string
	Tags        *[]string
}

func (s *Store) UpdateAssetMetadata(ctx context.Context, id string, patch AssetMetadataPatch) (*domain.Asset, error) {
	updates := map[string]any{}
	if patch.DisplayName != nil {
		updates["display_name"] = strings.TrimSpace(*patch.DisplayName)
	}
	if patch.DisplayURL != nil {
		updates["display_url"] = strings.TrimSpace(*patch.DisplayURL)
	}
	if patch.Notes != nil {
		updates["notes"] = strings.TrimSpace(*patch.Notes)
	}
	if patch.Tags != nil {
		updates["tags"] = jsonbExpr(uniqueNonEmptyStrings(*patch.Tags))
	}
	var asset domain.Asset
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := active(tx).Where("id = ?", id).First(&asset).Error; err != nil {
			if isNotFound(err) {
				return domain.ErrRecordNotFound
			}
			return err
		}
		if len(updates) > 0 {
			if err := tx.Model(&domain.Asset{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return err
			}
		}
		return active(tx).Where("id = ?", id).First(&asset).Error
	})
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (s *Store) ObserveAsset(ctx context.Context, taskID string, asset *domain.Asset, source string, snapshot map[string]any) error {
	id, err := domain.NewID()
	if err != nil {
		return err
	}
	obs := domain.AssetObservation{
		ID: id, TaskID: taskID, AssetID: asset.ID, Kind: asset.Kind, Key: asset.Key,
		Source: source, Snapshot: snapshot, ObservedAt: time.Now().UTC(), SchemaVersion: domain.SchemaVersion,
	}
	return s.db.WithContext(ctx).Create(&obs).Error
}

func (s *Store) AddFinding(ctx context.Context, f *domain.Finding) (bool, error) {
	f.CreatedAt = time.Now().UTC()
	f.SchemaVersion = domain.SchemaVersion
	if f.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return false, err
		}
		f.ID = id
	}
	err := s.db.WithContext(ctx).Create(f).Error
	if isDuplicate(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_ = s.AppendGraphOutbox(ctx, "finding", f.ID, domain.OutboxOpUpsert)
	return true, nil
}

func (s *Store) ListAssets(ctx context.Context, kind, taskID, scopeID string, q PageQuery) ([]domain.Asset, int64, error) {
	q = q.Normalized()
	db := active(s.db.WithContext(ctx)).Model(&domain.Asset{})
	if kind != "" {
		db = db.Where("kind = ?", kind)
	}
	if taskID != "" {
		db = db.Where("id IN (?)", s.db.Model(&domain.AssetObservation{}).Select("asset_id").Where("task_id = ?", taskID))
	}
	if scopeID != "" {
		db = db.Where("id IN (?)", s.db.Model(&domain.ScopeAssetLink{}).Select("asset_id").Where("scope_id = ?", scopeID))
	}
	if q.Search != "" {
		pattern := ilike(q.Search)
		db = db.Where(`(key ILIKE ? ESCAPE '\' OR data->>'title' ILIKE ? ESCAPE '\' OR data->>'url' ILIKE ? ESCAPE '\'
			OR display_name ILIKE ? ESCAPE '\' OR display_url ILIKE ? ESCAPE '\' OR notes ILIKE ? ESCAPE '\'
			OR tags::text ILIKE ? ESCAPE '\')`,
			pattern, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.Asset
	err := db.Order(OrderClause(q.Sort)).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}

func (s *Store) assetIDsForTask(ctx context.Context, taskID string) ([]string, error) {
	var ids []string
	err := s.db.WithContext(ctx).Table("asset_observations").
		Select("DISTINCT asset_observations.asset_id").
		Joins("JOIN assets ON assets.id = asset_observations.asset_id").
		Where("asset_observations.task_id = ? AND assets.deleted_at IS NULL", taskID).
		Pluck("asset_observations.asset_id", &ids).Error
	return ids, err
}

func (s *Store) ListTaskObservations(ctx context.Context, taskID string, kind string, q PageQuery) ([]domain.AssetObservation, int64, error) {
	q = q.Normalized()
	db := s.db.WithContext(ctx).Model(&domain.AssetObservation{}).Where("task_id = ?", taskID)
	if kind != "" {
		db = db.Where("kind = ?", kind)
	}
	if q.Search != "" {
		db = db.Where(`key ILIKE ? ESCAPE '\'`, ilike(q.Search))
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.AssetObservation
	err := db.Order("observed_at DESC, id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}

func (s *Store) ListFindings(ctx context.Context, kind, taskID, scopeID string, q PageQuery) ([]domain.Finding, int64, error) {
	q = q.Normalized()
	db := s.db.WithContext(ctx).Model(&domain.Finding{})
	if kind != "" {
		db = db.Where("kind = ?", kind)
	}
	if taskID != "" {
		db = db.Where("task_id = ?", taskID)
	}
	if scopeID != "" {
		db = db.Where("scope_id = ?", scopeID)
	}
	if q.Search != "" {
		pattern := ilike(q.Search)
		db = db.Where(`(title ILIKE ? ESCAPE '\' OR target ILIKE ? ESCAPE '\')`, pattern, pattern)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.Finding
	err := db.Order(OrderClause(q.Sort)).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}

func (s *Store) TaskStatistics(ctx context.Context, taskID string) (domain.TaskStatistics, error) {
	var st domain.TaskStatistics
	var obsRows []struct {
		Kind  string
		Count int64
	}
	if err := s.db.WithContext(ctx).Model(&domain.AssetObservation{}).
		Select("kind, count(*) AS count").Where("task_id = ?", taskID).Group("kind").Scan(&obsRows).Error; err != nil {
		return st, err
	}
	for _, row := range obsRows {
		switch domain.AssetKind(row.Kind) {
		case domain.AssetDomain:
			st.Domains = int(row.Count)
		case domain.AssetIP:
			st.IPs = int(row.Count)
		case domain.AssetSite:
			st.Sites = int(row.Count)
		case domain.AssetURL:
			st.URLs = int(row.Count)
		case domain.AssetCert:
			st.Certs = int(row.Count)
		case domain.AssetService:
			st.Services = int(row.Count)
		}
	}
	var findingRows []struct {
		Kind  string
		Count int64
	}
	if err := s.db.WithContext(ctx).Model(&domain.Finding{}).
		Select("kind, count(*) AS count").Where("task_id = ?", taskID).Group("kind").Scan(&findingRows).Error; err != nil {
		return st, err
	}
	for _, row := range findingRows {
		switch domain.FindingKind(row.Kind) {
		case domain.FindingVuln:
			st.Vulns = int(row.Count)
		case domain.FindingNuclei:
			st.Nuclei = int(row.Count)
		case domain.FindingWIH:
			st.WIH = int(row.Count)
		case domain.FindingFileLeak:
			st.FileLeaks = int(row.Count)
		case domain.FindingProtocol:
			st.Protocols = int(row.Count)
		}
	}
	return st, nil
}

func (s *Store) CreateScope(ctx context.Context, sc *domain.Scope) error {
	now := time.Now().UTC()
	sc.CreatedAt, sc.UpdatedAt, sc.SchemaVersion = now, now, domain.SchemaVersion
	if sc.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		sc.ID = id
	}
	return s.db.WithContext(ctx).Create(sc).Error
}

func (s *Store) GetScope(ctx context.Context, id string) (*domain.Scope, error) {
	var sc domain.Scope
	err := active(s.db.WithContext(ctx)).Where("id = ?", id).First(&sc).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

func (s *Store) ListScopes(ctx context.Context, q PageQuery) ([]domain.Scope, int64, error) {
	q = q.Normalized()
	db := active(s.db.WithContext(ctx)).Model(&domain.Scope{})
	if q.Search != "" {
		db = db.Where(`name ILIKE ? ESCAPE '\'`, ilike(q.Search))
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.Scope
	err := db.Order(OrderClause(q.Sort)).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}

func (s *Store) UpdateScope(ctx context.Context, sc *domain.Scope) error {
	sc.UpdatedAt = time.Now().UTC()
	return s.db.WithContext(ctx).Save(sc).Error
}

func (s *Store) DeleteScope(ctx context.Context, id, actor string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := active(tx).Where("id = ?", id).First(&domain.Scope{}).Error; err != nil {
			if isNotFound(err) {
				return domain.ErrRecordNotFound
			}
			return err
		}
		var monitors int64
		if err := active(tx).Model(&domain.Monitor{}).Where("scope_id = ?", id).Count(&monitors).Error; err != nil {
			return err
		}
		if monitors > 0 {
			return fmt.Errorf("%w: %d monitor(s)", domain.ErrResourceInUse, monitors)
		}
		return softDeleteRecordTx(tx, "scope", id, actor)
	})
}

func (s *Store) LinkAssetToScope(ctx context.Context, scopeID string, asset *domain.Asset, tags []string) error {
	now := time.Now().UTC()
	id, err := domain.NewID()
	if err != nil {
		return err
	}
	if tags == nil {
		tags = []string{}
	}
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Exec(`
INSERT INTO scope_asset_links (id, scope_id, asset_id, kind, tags, first_seen, last_seen, schema_version)
VALUES (?, ?, ?, ?, ?::jsonb, ?, ?, ?)
ON CONFLICT (scope_id, asset_id) DO UPDATE SET
  kind = EXCLUDED.kind,
  last_seen = EXCLUDED.last_seen,
  schema_version = EXCLUDED.schema_version,
  tags = (
    SELECT COALESCE(jsonb_agg(DISTINCT item), '[]'::jsonb)
    FROM jsonb_array_elements_text(COALESCE(scope_asset_links.tags, '[]'::jsonb) || COALESCE(EXCLUDED.tags, '[]'::jsonb)) AS item
  )`,
		id, scopeID, asset.ID, asset.Kind, string(tagsJSON), now, now, domain.SchemaVersion).Error
}

func (s *Store) ListScopeAssets(ctx context.Context, scopeID string, kind string, q PageQuery) ([]map[string]any, int64, error) {
	q = q.Normalized()
	db := s.db.WithContext(ctx).Model(&domain.ScopeAssetLink{}).Where("scope_id = ?", scopeID)
	if kind != "" {
		db = db.Where("kind = ?", kind)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var links []domain.ScopeAssetLink
	if err := db.Order("last_seen DESC, id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&links).Error; err != nil {
		return nil, 0, err
	}
	if len(links) == 0 {
		return []map[string]any{}, total, nil
	}
	ids := make([]string, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.AssetID)
	}
	var assets []domain.Asset
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&assets).Error; err != nil {
		return nil, 0, err
	}
	byID := make(map[string]domain.Asset, len(assets))
	for _, asset := range assets {
		byID[asset.ID] = asset
	}
	out := make([]map[string]any, 0, len(links))
	for _, link := range links {
		asset, ok := byID[link.AssetID]
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"kind": link.Kind, "tags": link.Tags,
			"firstSeen": link.FirstSeen, "lastSeen": link.LastSeen, "asset": asset,
		})
	}
	return out, total, nil
}

func (s *Store) SearchHistoryDomains(ctx context.Context, base string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5000
	}
	pattern := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(strings.TrimSpace(base)))
	var out []string
	err := active(s.db.WithContext(ctx)).Model(&domain.Asset{}).
		Where(`kind = ? AND key ILIKE ? ESCAPE '\'`, domain.AssetDomain, pattern).
		Order("key ASC").Limit(limit).Pluck("key", &out).Error
	return out, err
}

func (s *Store) CountAssets(ctx context.Context) (map[string]int64, error) {
	var rows []struct {
		Kind  string
		Count int64
	}
	if err := active(s.db.WithContext(ctx)).Model(&domain.Asset{}).Select("kind, count(*) AS count").Group("kind").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, row := range rows {
		out[row.Kind] = row.Count
	}
	return out, nil
}

type FingerprintRule struct {
	ID        string           `gorm:"primaryKey;size:24" json:"id"`
	Name      string           `json:"name"`
	Rule      string           `json:"rule"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
	Lifecycle domain.Lifecycle `gorm:"embedded"`
}

func (FingerprintRule) TableName() string { return "fingerprints" }

func (s *Store) CreateFingerprint(ctx context.Context, r *FingerprintRule) error {
	now := time.Now().UTC()
	r.CreatedAt, r.UpdatedAt = now, now
	if r.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		r.ID = id
	}
	return s.db.WithContext(ctx).Create(r).Error
}

func (s *Store) ListFingerprints(ctx context.Context) ([]FingerprintRule, error) {
	var out []FingerprintRule
	err := active(s.db.WithContext(ctx)).Order("name ASC").Find(&out).Error
	return out, err
}

func (s *Store) GetFingerprint(ctx context.Context, id string) (*FingerprintRule, error) {
	var rule FingerprintRule
	err := active(s.db.WithContext(ctx)).Where("id = ?", id).First(&rule).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (s *Store) UpdateFingerprint(ctx context.Context, rule *FingerprintRule) error {
	rule.UpdatedAt = time.Now().UTC()
	return s.db.WithContext(ctx).Model(&FingerprintRule{}).
		Where("id = ? AND deleted_at IS NULL", rule.ID).
		Updates(map[string]any{"name": rule.Name, "rule": rule.Rule, "updated_at": rule.UpdatedAt}).Error
}

func (s *Store) DeleteFingerprint(ctx context.Context, id, actor string) error {
	return s.SoftDeleteRecord(ctx, "fingerprint", id, actor)
}

func (s *Store) UpsertPOCPlugin(ctx context.Context, p *domain.PocPlugin) error {
	now := time.Now().UTC()
	p.CreatedAt, p.UpdatedAt, p.SchemaVersion = now, now, domain.SchemaVersion
	if p.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		p.ID = id
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"type", "app_name", "vul_name", "category", "schemes", "description", "enabled", "updated_at", "schema_version"}),
	}).Create(p).Error
}

func (s *Store) ListPOCPlugins(ctx context.Context, kind string) ([]domain.PocPlugin, error) {
	db := s.db.WithContext(ctx).Model(&domain.PocPlugin{})
	if kind != "" {
		db = db.Where("type = ?", kind)
	}
	var out []domain.PocPlugin
	err := db.Order("type ASC, name ASC").Find(&out).Error
	return out, err
}

func (s *Store) DeletePOCPlugin(ctx context.Context, name string) error {
	return s.db.WithContext(ctx).Delete(&domain.PocPlugin{}, "name = ?", name).Error
}
