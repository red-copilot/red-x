package store

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// PruneExpired replaces the Mongo TTL indexes with application-driven cleanup.
func (s *Store) PruneExpired(ctx context.Context, now time.Time) error {
	now = now.UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		statements := []struct {
			query string
			arg   any
		}{
			{`DELETE FROM sessions WHERE expires_at <= ?`, now},
			{`DELETE FROM leases WHERE expires_at <= ?`, now},
			{`DELETE FROM audit_events WHERE created_at < ?`, now.Add(-180 * 24 * time.Hour)},
			{`DELETE FROM task_events WHERE created_at < ?`, now.Add(-90 * 24 * time.Hour)},
			{`DELETE FROM exports WHERE created_at < ?`, now.Add(-30 * 24 * time.Hour)},
			{`DELETE FROM icp_evidence WHERE expires_at <= ?`, now},
			{`DELETE FROM icp_jobs WHERE created_at < ?`, now.Add(-7 * 24 * time.Hour)},
		}
		for _, stmt := range statements {
			if err := tx.Exec(stmt.query, stmt.arg).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
