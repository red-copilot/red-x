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

type ICPJobPatch struct {
	Status      *domain.ICPJobStatus
	ResultCount *int
	Error       *string
	FinishedAt  *time.Time
}

type OrganizationDomainPatch struct {
	SiteName  *string
	HomeSite  *string
	ICPNo     *string
	AuditDate *string
}

func (p ICPJobPatch) updates() map[string]any {
	out := map[string]any{}
	if p.Status != nil {
		out["status"] = *p.Status
	}
	if p.ResultCount != nil {
		out["result_count"] = *p.ResultCount
	}
	if p.Error != nil {
		out["error"] = *p.Error
	}
	if p.FinishedAt != nil {
		out["finished_at"] = *p.FinishedAt
	}
	return out
}

func (s *Store) CreateOrganization(ctx context.Context, org *domain.Organization) error {
	if err := org.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	org.CanonicalName = strings.TrimSpace(org.CanonicalName)
	org.CreatedAt, org.UpdatedAt, org.SchemaVersion = now, now, domain.SchemaVersion
	if org.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		org.ID = id
	}
	return s.db.WithContext(ctx).Create(org).Error
}

func (s *Store) GetOrganization(ctx context.Context, id string) (*domain.Organization, error) {
	var org domain.Organization
	err := active(s.db.WithContext(ctx)).Where("id = ?", id).First(&org).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func (s *Store) FindOrganizationByName(ctx context.Context, name string) (*domain.Organization, error) {
	return s.FindOrganizationByNameAndRegion(ctx, name, "")
}

func (s *Store) FindOrganizationByNameAndRegion(ctx context.Context, name, region string) (*domain.Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	aliasJSON, err := json.Marshal([]string{name})
	if err != nil {
		return nil, err
	}
	db := active(s.db.WithContext(ctx)).Where("status = ?", domain.OrgStatusActive).
		Where("(canonical_name = ? OR aliases @> ?::jsonb)", name, string(aliasJSON))
	if region = strings.TrimSpace(region); region != "" {
		db = db.Where("region = ?", region)
	}
	var org domain.Organization
	err = db.First(&org).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func (s *Store) ListOrganizations(ctx context.Context, query string, limit int) ([]domain.Organization, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 5000 {
		limit = 5000
	}
	db := active(s.db.WithContext(ctx)).Where("status = ?", domain.OrgStatusActive)
	if q := strings.TrimSpace(query); q != "" {
		pattern := ilike(q)
		db = db.Where(`(canonical_name ILIKE ? ESCAPE '\' OR EXISTS (SELECT 1 FROM jsonb_array_elements_text(aliases) AS alias WHERE alias ILIKE ? ESCAPE '\'))`, pattern, pattern)
	}
	var out []domain.Organization
	err := db.Order("canonical_name ASC, id ASC").Limit(limit).Find(&out).Error
	return out, err
}

func (s *Store) UpsertOrganizationDomain(ctx context.Context, od *domain.OrganizationDomain) error {
	if err := od.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	od.RootDomain = strings.TrimSpace(strings.ToLower(od.RootDomain))
	if od.FirstSeen.IsZero() {
		od.FirstSeen = now
	}
	od.LastSeen, od.UpdatedAt, od.SchemaVersion = now, now, domain.SchemaVersion
	if od.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		od.ID = id
	}
	var conflict error
	for attempt := 0; attempt < 2; attempt++ {
		conflict = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var org domain.Organization
			if err := active(tx).Where("id = ?", od.OrganizationID).First(&org).Error; err != nil {
				if isNotFound(err) {
					return domain.ErrRecordNotFound
				}
				return err
			}
			var existing domain.OrganizationDomain
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("organization_id = ? AND root_domain = ?", od.OrganizationID, od.RootDomain).
				First(&existing).Error
			if err == nil {
				wasDeleted := existing.Deleted()
				wasConfirmed := existing.Status == domain.DomainStatusConfirmed
				if wasDeleted && od.Status == domain.DomainStatusConfirmed {
					var conflictCount int64
					if err := tx.Model(&domain.OrganizationDomain{}).
						Where("root_domain = ? AND status = ? AND organization_id <> ? AND deleted_at IS NULL", od.RootDomain, domain.DomainStatusConfirmed, od.OrganizationID).
						Count(&conflictCount).Error; err != nil {
						return err
					}
					if conflictCount > 0 {
						return domain.ErrDomainAlreadyConfirmed
					}
				}
				existing.SiteName, existing.HomeSite, existing.ICPNo, existing.AuditDate = od.SiteName, od.HomeSite, od.ICPNo, od.AuditDate
				existing.Source, existing.Confidence, existing.EvidenceID = od.Source, od.Confidence, od.EvidenceID
				existing.LastSeen, existing.UpdatedAt, existing.SchemaVersion = now, now, domain.SchemaVersion
				if existing.Status == domain.DomainStatusCandidate {
					existing.Status = od.Status
				}
				if wasDeleted {
					existing.Lifecycle.MarkRestored(now, "reimport")
				}
				*od = existing
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
				if existing.Status == domain.DomainStatusConfirmed && (wasDeleted || !wasConfirmed) {
					return appendGraphOutboxTx(tx, "organization_domain", existing.ID, domain.OutboxOpUpsert)
				}
				return nil
			}
			if !isNotFound(err) {
				return err
			}
			if od.Status == domain.DomainStatusConfirmed {
				var n int64
				if err := tx.Model(&domain.OrganizationDomain{}).
					Where("root_domain = ? AND status = ? AND organization_id <> ? AND deleted_at IS NULL", od.RootDomain, domain.DomainStatusConfirmed, od.OrganizationID).
					Count(&n).Error; err != nil {
					return err
				}
				if n > 0 {
					return domain.ErrDomainAlreadyConfirmed
				}
			}
			od.FirstSeen, od.LastSeen, od.UpdatedAt = now, now, now
			return tx.Create(od).Error
		})
		if conflict == nil {
			return nil
		}
		if errors.Is(conflict, domain.ErrDomainAlreadyConfirmed) {
			return conflict
		}
		if !isDuplicate(conflict) {
			return conflict
		}
	}
	return fmt.Errorf("upsert organization domain %s: concurrent update conflict: %w", od.RootDomain, conflict)
}

func (s *Store) ListOrganizationDomains(ctx context.Context, orgID string, status string) ([]domain.OrganizationDomain, error) {
	db := active(s.db.WithContext(ctx)).Where("organization_id = ?", orgID).
		Where("organization_id IN (?)", s.db.WithContext(ctx).Model(&domain.Organization{}).
			Select("id").Where("deleted_at IS NULL"))
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var out []domain.OrganizationDomain
	err := db.Order("root_domain ASC, id ASC").Find(&out).Error
	return out, err
}

func (s *Store) ConfirmOrganizationDomains(ctx context.Context, orgID string, domains []string, actor string) (int, error) {
	if len(domains) == 0 {
		return 0, nil
	}
	normalized := make([]string, 0, len(domains))
	for _, item := range domains {
		if item = strings.TrimSpace(strings.ToLower(item)); item != "" {
			normalized = append(normalized, item)
		}
	}
	if len(normalized) == 0 {
		return 0, nil
	}
	var changed int
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var pending []domain.OrganizationDomain
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("organization_id = ? AND root_domain IN ? AND status = ?", orgID, normalized, domain.DomainStatusCandidate).
			Find(&pending).Error; err != nil {
			return err
		}
		if len(pending) == 0 {
			return nil
		}
		roots := make([]string, 0, len(pending))
		ids := make([]string, 0, len(pending))
		for _, item := range pending {
			roots = append(roots, item.RootDomain)
			ids = append(ids, item.ID)
		}
		var conflict domain.OrganizationDomain
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("root_domain IN ? AND status = ? AND organization_id <> ? AND deleted_at IS NULL", roots, domain.DomainStatusConfirmed, orgID).
			First(&conflict).Error
		if err == nil {
			return fmt.Errorf("%w: %s", domain.ErrDomainAlreadyConfirmed, conflict.RootDomain)
		}
		if !isNotFound(err) {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&domain.OrganizationDomain{}).Where("id IN ?", ids).Updates(map[string]any{
			"status": domain.DomainStatusConfirmed, "last_seen": now, "updated_at": now,
		}).Error; err != nil {
			if isDuplicate(err) {
				return domain.ErrDomainAlreadyConfirmed
			}
			return err
		}
		for _, item := range pending {
			if err := appendGraphOutboxTx(tx, "organization_domain", item.ID, domain.OutboxOpUpsert); err != nil {
				return err
			}
		}
		changed = len(ids)
		return nil
	})
	return changed, err
}

func (s *Store) AddICPEvidence(ctx context.Context, ev *domain.ICPEvidence) error {
	if ev == nil || strings.TrimSpace(ev.Provider) == "" || strings.TrimSpace(ev.QueryKey) == "" {
		return errors.New("invalid ICP evidence")
	}
	if ev.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		ev.ID = id
	}
	if ev.FetchedAt.IsZero() {
		ev.FetchedAt = time.Now().UTC()
	}
	ev.SchemaVersion = domain.SchemaVersion
	return s.db.WithContext(ctx).Create(ev).Error
}

func (s *Store) UpsertIPLocation(ctx context.Context, loc *domain.IPLocation) error {
	if loc == nil || strings.TrimSpace(loc.IP) == "" {
		return errors.New("IP location requires ip")
	}
	loc.UpdatedAt = time.Now().UTC()
	loc.SchemaVersion = domain.SchemaVersion
	if loc.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		loc.ID = id
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "ip"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"lat", "lon", "country", "province", "city", "asn", "asn_org", "source", "updated_at", "schema_version",
		}),
	}).Create(loc).Error
}

func (s *Store) GetIPLocation(ctx context.Context, ip string) (*domain.IPLocation, error) {
	var loc domain.IPLocation
	err := s.db.WithContext(ctx).Where("ip = ?", ip).First(&loc).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &loc, nil
}

func (s *Store) CreateICPJob(ctx context.Context, job *domain.ICPJob) error {
	if job == nil || job.OrganizationID == "" {
		return errors.New("ICP job requires organizationId")
	}
	if job.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		job.ID = id
	}
	if job.Status == "" {
		job.Status = domain.ICPJobQueued
	}
	job.CreatedAt = time.Now().UTC()
	job.SchemaVersion = domain.SchemaVersion
	return s.db.WithContext(ctx).Create(job).Error
}

func (s *Store) UpdateICPJob(ctx context.Context, id string, patch ICPJobPatch) error {
	updates := patch.updates()
	if len(updates) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&domain.ICPJob{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Store) GetICPJob(ctx context.Context, id string) (*domain.ICPJob, error) {
	var job domain.ICPJob
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&job).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) AppendGraphOutbox(ctx context.Context, entityType, entityID string, op domain.GraphOutboxOp) error {
	return appendGraphOutboxTx(s.db.WithContext(ctx), entityType, entityID, op)
}

func appendGraphOutboxTx(tx *gorm.DB, entityType, entityID string, op domain.GraphOutboxOp) error {
	if strings.TrimSpace(entityType) == "" || strings.TrimSpace(entityID) == "" {
		return errors.New("graph outbox requires entity type and id")
	}
	if !op.Valid() {
		return errors.New("invalid graph outbox op")
	}
	id, err := domain.NewID()
	if err != nil {
		return err
	}
	return tx.Create(&domain.GraphOutbox{
		ID: id, EntityType: entityType, EntityID: entityID, Op: op,
		CreatedAt: time.Now().UTC(), SchemaVersion: domain.SchemaVersion,
	}).Error
}

func (s *Store) ClaimGraphOutbox(ctx context.Context, limit int) ([]domain.GraphOutbox, error) {
	if limit <= 0 {
		limit = 100
	}
	var out []domain.GraphOutbox
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		stale := now.Add(-5 * time.Minute)
		var ids []string
		if err := tx.Raw(`
SELECT id FROM graph_outbox
WHERE processed_at IS NULL
  AND attempts < 5
  AND (processing_at IS NULL OR processing_at < ?)
ORDER BY created_at ASC
LIMIT ?
FOR UPDATE SKIP LOCKED`, stale, limit).Scan(&ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Model(&domain.GraphOutbox{}).Where("id IN ?", ids).Updates(map[string]any{
			"processing_at": now, "attempts": gorm.Expr("attempts + 1"),
		}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Order("created_at ASC, id ASC").Find(&out).Error
	})
	return out, err
}

func (s *Store) MarkGraphOutboxProcessed(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&domain.GraphOutbox{}).Where("id IN ?", ids).Updates(map[string]any{
		"processed_at": now, "processing_at": nil,
	}).Error
}

func (s *Store) GetAssetByID(ctx context.Context, id string) (*domain.Asset, error) {
	var asset domain.Asset
	err := active(s.db.WithContext(ctx)).Where("id = ?", id).First(&asset).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func (s *Store) GetFindingByID(ctx context.Context, id string) (*domain.Finding, error) {
	var finding domain.Finding
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&finding).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &finding, nil
}

func (s *Store) GetOrganizationDomainByID(ctx context.Context, id string) (*domain.OrganizationDomain, error) {
	var od domain.OrganizationDomain
	err := active(s.db.WithContext(ctx)).Where("id = ?", id).
		Where("organization_id IN (?)", s.db.WithContext(ctx).Model(&domain.Organization{}).
			Select("id").Where("deleted_at IS NULL")).
		First(&od).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &od, nil
}

func (s *Store) ListAllOrganizations(ctx context.Context) ([]domain.Organization, error) {
	var out []domain.Organization
	err := active(s.db.WithContext(ctx)).Where("status = ?", domain.OrgStatusActive).Order("canonical_name ASC, id ASC").Find(&out).Error
	return out, err
}

func (s *Store) ListAllOrganizationDomains(ctx context.Context, status string) ([]domain.OrganizationDomain, error) {
	db := active(s.db.WithContext(ctx)).Model(&domain.OrganizationDomain{})
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var out []domain.OrganizationDomain
	err := db.Order("organization_id ASC, root_domain ASC, id ASC").Find(&out).Error
	return out, err
}

func (s *Store) ClearGraphOutbox(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("1 = 1").Delete(&domain.GraphOutbox{}).Error
}

func (s *Store) ClearGraphOutboxBefore(ctx context.Context, before time.Time) error {
	return s.db.WithContext(ctx).Where("created_at < ?", before).Delete(&domain.GraphOutbox{}).Error
}

func (s *Store) UpdateOrganization(ctx context.Context, org *domain.Organization) error {
	if err := org.Validate(); err != nil {
		return err
	}
	org.CanonicalName = strings.TrimSpace(org.CanonicalName)
	org.UpdatedAt = time.Now().UTC()
	return activeUpdateResult(s.db.WithContext(ctx).Model(&domain.Organization{}).Where("id = ? AND deleted_at IS NULL", org.ID).Updates(map[string]any{
		"canonical_name": org.CanonicalName, "aliases": jsonbExpr(org.Aliases), "region": org.Region,
		"uscc": org.USCC, "status": org.Status, "merged_into": org.MergedInto,
		"updated_at": org.UpdatedAt,
	}))
}

func (s *Store) DeleteOrganization(ctx context.Context, id, actor string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var org domain.Organization
		if err := active(tx).Where("id = ?", id).First(&org).Error; err != nil {
			if isNotFound(err) {
				return domain.ErrRecordNotFound
			}
			return err
		}
		var activeTasks int64
		if err := tx.Model(&domain.Task{}).
			Where("organization_id = ? AND status IN ?", id, []domain.TaskStatus{domain.TaskQueued, domain.TaskRunning}).
			Count(&activeTasks).Error; err != nil {
			return err
		}
		if activeTasks > 0 {
			return fmt.Errorf("%w: %d queued or running task(s)", domain.ErrResourceInUse, activeTasks)
		}
		now := time.Now().UTC()
		if err := tx.Model(&domain.Organization{}).Where("id = ?", id).Updates(map[string]any{
			"deleted_at": now, "deleted_by": actor, "restored_at": nil, "restore_reason": "",
		}).Error; err != nil {
			return err
		}
		return appendGraphOutboxTx(tx, "organization", id, domain.OutboxOpDelete)
	})
}

func (s *Store) UpdateOrganizationDomainMetadata(ctx context.Context, orgID, domainID string, patch OrganizationDomainPatch) (*domain.OrganizationDomain, error) {
	updates := map[string]any{"updated_at": time.Now().UTC()}
	if patch.SiteName != nil {
		updates["site_name"] = strings.TrimSpace(*patch.SiteName)
	}
	if patch.HomeSite != nil {
		updates["home_site"] = strings.TrimSpace(*patch.HomeSite)
	}
	if patch.ICPNo != nil {
		updates["icp_no"] = strings.TrimSpace(*patch.ICPNo)
	}
	if patch.AuditDate != nil {
		updates["audit_date"] = strings.TrimSpace(*patch.AuditDate)
	}
	var row domain.OrganizationDomain
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := active(tx).Where("id = ? AND organization_id = ?", domainID, orgID).First(&row).Error; err != nil {
			if isNotFound(err) {
				return domain.ErrRecordNotFound
			}
			return err
		}
		if err := tx.Model(&domain.OrganizationDomain{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return err
		}
		return active(tx).Where("id = ?", row.ID).First(&row).Error
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) GetICPEvidence(ctx context.Context, id string) (*domain.ICPEvidence, error) {
	var ev domain.ICPEvidence
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&ev).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

func (s *Store) RejectOrganizationDomain(ctx context.Context, domainID, orgID string, actor, reason string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		res := tx.Model(&domain.OrganizationDomain{}).
			Where("id = ? AND organization_id = ? AND status IN ?", domainID, orgID, []domain.DomainStatus{domain.DomainStatusCandidate, domain.DomainStatusConfirmed}).
			Updates(map[string]any{
				"status": domain.DomainStatusRejected, "decision_reason": reason, "decided_by": actor,
				"decided_at": now, "updated_at": now, "last_seen": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("organization domain not found")
		}
		return appendGraphOutboxTx(tx, "organization_domain", domainID, domain.OutboxOpUpsert)
	})
}

func (s *Store) ReassignOrganizationDomain(ctx context.Context, domainID, targetOrgID string, actor, reason string) (*domain.OrganizationDomain, error) {
	if domainID == "" || targetOrgID == "" {
		return nil, errors.New("domain id and target organization id are required")
	}
	var moved *domain.OrganizationDomain
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old domain.OrganizationDomain
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", domainID).First(&old).Error; err != nil {
			if isNotFound(err) {
				return errors.New("organization domain not found")
			}
			return err
		}
		if old.Status != domain.DomainStatusConfirmed {
			return errors.New("only confirmed domains can be reassigned")
		}
		if old.OrganizationID == targetOrgID {
			return domain.ErrDomainAlreadyConfirmed
		}
		var target domain.Organization
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ?", targetOrgID, domain.OrgStatusActive).First(&target).Error; err != nil {
			if isNotFound(err) {
				return errors.New("target organization not found")
			}
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&domain.OrganizationDomain{}).Where("id = ?", old.ID).Updates(map[string]any{
			"status": domain.DomainStatusRejected, "reassigned_to": targetOrgID,
			"decision_reason": reason, "decided_by": actor, "decided_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		var existing domain.OrganizationDomain
		existingErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("organization_id = ? AND root_domain = ?", targetOrgID, old.RootDomain).First(&existing).Error
		if existingErr != nil && !isNotFound(existingErr) {
			return existingErr
		}
		newRow := old
		newRow.OrganizationID = targetOrgID
		newRow.Status = domain.DomainStatusConfirmed
		newRow.DecisionReason = reason
		newRow.DecidedBy = actor
		newRow.DecidedAt = &now
		newRow.ReassignedTo = nil
		newRow.LastSeen = now
		newRow.UpdatedAt = now
		newRow.SchemaVersion = domain.SchemaVersion
		if existingErr == nil {
			if existing.Status == domain.DomainStatusConfirmed {
				return domain.ErrDomainAlreadyConfirmed
			}
			newRow.ID = existing.ID
			newRow.FirstSeen = existing.FirstSeen
			if err := tx.Save(&newRow).Error; err != nil {
				if isDuplicate(err) {
					return domain.ErrDomainAlreadyConfirmed
				}
				return err
			}
		} else {
			id, err := domain.NewID()
			if err != nil {
				return err
			}
			newRow.ID = id
			if err := tx.Create(&newRow).Error; err != nil {
				if isDuplicate(err) {
					return domain.ErrDomainAlreadyConfirmed
				}
				return err
			}
		}
		if err := appendGraphOutboxTx(tx, "organization_domain", old.ID, domain.OutboxOpUpsert); err != nil {
			return err
		}
		if err := appendGraphOutboxTx(tx, "organization_domain", newRow.ID, domain.OutboxOpUpsert); err != nil {
			return err
		}
		moved = &newRow
		return nil
	})
	if err != nil {
		return nil, err
	}
	return moved, nil
}

func (s *Store) SummaryTasks(ctx context.Context, organizationID string) (int, *time.Time, error) {
	var n int64
	db := s.db.WithContext(ctx).Model(&domain.Task{}).Where("organization_id = ? AND origin = ?", organizationID, domain.TaskOriginEASM)
	if err := db.Count(&n).Error; err != nil {
		return 0, nil, err
	}
	var latest domain.Task
	err := db.Order("created_at DESC, id DESC").First(&latest).Error
	if isNotFound(err) {
		return int(n), nil, nil
	}
	if err != nil {
		return 0, nil, err
	}
	return int(n), &latest.CreatedAt, nil
}
