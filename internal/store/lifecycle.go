package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"gorm.io/gorm"
)

type DeletedRecordFilter struct {
	Type     string
	Search   string
	Page     int
	PageSize int
}

type DeletedRecord struct {
	ResourceType string    `json:"type"`
	ID           string    `json:"id"`
	Label        string    `json:"label"`
	DeletedAt    time.Time `json:"deletedAt"`
	DeletedBy    string    `json:"deletedBy,omitempty"`
}

type lifecycleResource struct {
	Table string
	Label string
}

var lifecycleResourceOrder = []string{
	"organization",
	"organization_domain",
	"asset",
	"policy",
	"scope",
	"monitor",
	"schedule",
	"fingerprint",
	"github_monitor",
	"user",
}

var lifecycleResources = map[string]lifecycleResource{
	"organization":        {Table: "organizations", Label: "canonical_name"},
	"organization_domain": {Table: "organization_domains", Label: "root_domain"},
	"asset":               {Table: "assets", Label: "kind || ':' || key"},
	"policy":              {Table: "policies", Label: "name"},
	"scope":               {Table: "scopes", Label: "name"},
	"monitor":             {Table: "monitors", Label: "name"},
	"schedule":            {Table: "schedules", Label: "name"},
	"fingerprint":         {Table: "fingerprints", Label: "name"},
	"github_monitor":      {Table: "github_monitors", Label: "name"},
	"user":                {Table: "users", Label: "username"},
}

func ValidLifecycleResource(resourceType string) bool {
	_, ok := lifecycleResources[resourceType]
	return ok
}

func active(db *gorm.DB) *gorm.DB {
	return db.Where("deleted_at IS NULL")
}

func activeUpdateResult(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrRecordNotFound
	}
	return nil
}

func (s *Store) SoftDeleteRecord(ctx context.Context, resourceType, id, actor string) error {
	if !ValidLifecycleResource(resourceType) {
		return fmt.Errorf("unsupported lifecycle resource %q", resourceType)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return softDeleteRecordTx(tx, resourceType, id, actor)
	})
}

func softDeleteRecordTx(tx *gorm.DB, resourceType, id, actor string) error {
	resource := lifecycleResources[resourceType]
	now := time.Now().UTC()
	result := tx.Exec(
		fmt.Sprintf("UPDATE %s SET deleted_at = ?, deleted_by = ?, restored_at = NULL, restore_reason = '' WHERE id = ? AND deleted_at IS NULL", resource.Table),
		now, actor, id,
	)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrRecordNotFound
	}
	if resourceType == "user" {
		if err := tx.Model(&domain.Session{}).
			Where("user_id = ? AND revoked_at IS NULL", id).
			Update("revoked_at", now).Error; err != nil {
			return err
		}
	}
	switch resourceType {
	case "organization", "organization_domain", "asset":
		if err := appendGraphOutboxTx(tx, resourceType, id, domain.OutboxOpDelete); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RestoreRecord(ctx context.Context, resourceType, id, actor string) error {
	resource, ok := lifecycleResources[resourceType]
	if !ok {
		return fmt.Errorf("unsupported lifecycle resource %q", resourceType)
	}
	_ = actor
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if resourceType == "organization_domain" {
			var domainRow domain.OrganizationDomain
			if err := tx.Where("id = ? AND deleted_at IS NOT NULL", id).First(&domainRow).Error; err != nil {
				if isNotFound(err) {
					return domain.ErrRecordNotFound
				}
				return err
			}
			if domainRow.Status == domain.DomainStatusConfirmed {
				var conflicts int64
				if err := tx.Model(&domain.OrganizationDomain{}).
					Where("root_domain = ? AND status = ? AND organization_id <> ? AND deleted_at IS NULL", domainRow.RootDomain, domain.DomainStatusConfirmed, domainRow.OrganizationID).
					Count(&conflicts).Error; err != nil {
					return err
				}
				if conflicts > 0 {
					return domain.ErrDomainAlreadyConfirmed
				}
			}
		}
		now := time.Now().UTC()
		result := tx.Exec(
			fmt.Sprintf("UPDATE %s SET deleted_at = NULL, deleted_by = '', restored_at = ?, restore_reason = 'manual' WHERE id = ? AND deleted_at IS NOT NULL", resource.Table),
			now, id,
		)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrRecordNotFound
		}
		switch resourceType {
		case "organization", "organization_domain", "asset":
			if err := appendGraphOutboxTx(tx, resourceType, id, domain.OutboxOpUpsert); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ListDeletedRecords(ctx context.Context, filter DeletedRecordFilter) ([]DeletedRecord, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}
	types := lifecycleResourceOrder
	if filter.Type != "" {
		if _, ok := lifecycleResources[filter.Type]; !ok {
			return nil, 0, fmt.Errorf("unsupported lifecycle resource %q", filter.Type)
		}
		types = []string{filter.Type}
	}
	queries := make([]string, 0, len(types))
	for _, resourceType := range types {
		resource := lifecycleResources[resourceType]
		queries = append(queries, fmt.Sprintf(
			"SELECT '%s' AS resource_type, id, COALESCE(%s, '') AS label, deleted_at, deleted_by FROM %s WHERE deleted_at IS NOT NULL",
			resourceType, resource.Label, resource.Table,
		))
	}
	base := "SELECT * FROM (" + strings.Join(queries, " UNION ALL ") + ") deleted"
	args := []any{}
	if search := strings.TrimSpace(filter.Search); search != "" {
		base += ` WHERE label ILIKE ? ESCAPE '\'`
		args = append(args, "%"+escapeLike(search)+"%")
	}
	var total int64
	if err := s.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM ("+base+") counted", args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	query := base + " ORDER BY deleted_at DESC, resource_type ASC, id DESC LIMIT ? OFFSET ?"
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	var items []DeletedRecord
	if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
