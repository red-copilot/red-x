package store

import (
	"context"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"gorm.io/gorm"
)

func (s *Store) CountUsers(ctx context.Context) (int64, error) {
	var n int64
	err := active(s.db.WithContext(ctx)).Model(&domain.User{}).Count(&n).Error
	return n, err
}

func (s *Store) CountActiveAdmins(ctx context.Context) (int64, error) {
	var count int64
	err := active(s.db.WithContext(ctx)).Model(&domain.User{}).
		Where("role = ? AND disabled = FALSE", domain.RoleAdmin).
		Count(&count).Error
	return count, err
}

func (s *Store) CreateUser(ctx context.Context, u *domain.User) error {
	now := time.Now().UTC()
	u.CreatedAt, u.UpdatedAt, u.SchemaVersion = now, now, domain.SchemaVersion
	if u.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		u.ID = id
	}
	return s.db.WithContext(ctx).Create(u).Error
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	var u domain.User
	err := active(s.db.WithContext(ctx)).Where("username = ?", strings.TrimSpace(username)).First(&u).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	var u domain.User
	err := active(s.db.WithContext(ctx)).Where("id = ?", id).First(&u).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UpdateUser(ctx context.Context, u *domain.User) error {
	u.UpdatedAt = time.Now().UTC()
	return activeUpdateResult(s.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ? AND deleted_at IS NULL", u.ID).
		Updates(map[string]any{
			"password_hash": u.PasswordHash, "role": u.Role,
			"must_change_password": u.MustChangePassword, "disabled": u.Disabled,
			"updated_at": u.UpdatedAt,
		}))
}

func (s *Store) ListUsers(ctx context.Context, q PageQuery) ([]domain.User, int64, error) {
	q = q.Normalized()
	db := active(s.db.WithContext(ctx)).Model(&domain.User{})
	if q.Search != "" {
		db = db.Where(`username ILIKE ? ESCAPE '\'`, ilike(q.Search))
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.User
	err := db.Order(OrderClause(q.Sort)).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}

func (s *Store) DeleteUser(ctx context.Context, id, actor, currentUserID string) error {
	if id == currentUserID {
		return domain.ErrSelfDelete
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user domain.User
		if err := active(tx).Where("id = ?", id).First(&user).Error; err != nil {
			if isNotFound(err) {
				return domain.ErrRecordNotFound
			}
			return err
		}
		if user.Role == domain.RoleAdmin && !user.Disabled {
			var admins int64
			if err := active(tx).Model(&domain.User{}).
				Where("role = ? AND disabled = FALSE", domain.RoleAdmin).
				Count(&admins).Error; err != nil {
				return err
			}
			if admins <= 1 {
				return domain.ErrLastAdmin
			}
		}
		return softDeleteRecordTx(tx, "user", id, actor)
	})
}

func (s *Store) CreateSession(ctx context.Context, sess *domain.Session) error {
	if sess.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		sess.ID = id
	}
	sess.CreatedAt = time.Now().UTC()
	sess.SchemaVersion = domain.SchemaVersion
	return s.db.WithContext(ctx).Create(sess).Error
}

func (s *Store) GetSessionByHash(ctx context.Context, hash string) (*domain.Session, error) {
	var sess domain.Session
	err := s.db.WithContext(ctx).Where("refresh_hash = ?", hash).First(&sess).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) RotateSession(ctx context.Context, id, newHash string, expires time.Time) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&domain.Session{}).Where("id = ?", id).Updates(map[string]any{
		"refresh_hash": newHash, "expires_at": expires, "rotated_at": now,
	}).Error
}

func (s *Store) RevokeSession(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Model(&domain.Session{}).Where("id = ?", id).Update("revoked_at", time.Now().UTC()).Error
}

func (s *Store) RevokeUserSessions(ctx context.Context, userID string) error {
	return s.db.WithContext(ctx).Model(&domain.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", time.Now().UTC()).Error
}

func (s *Store) CreateAPIKey(ctx context.Context, key *domain.APIKey) error {
	if key.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		key.ID = id
	}
	key.CreatedAt = time.Now().UTC()
	key.SchemaVersion = domain.SchemaVersion
	return s.db.WithContext(ctx).Create(key).Error
}

func (s *Store) GetAPIKeyByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	var key domain.APIKey
	err := s.db.WithContext(ctx).Where("key_hash = ? AND revoked_at IS NULL", hash).First(&key).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (s *Store) TouchAPIKey(ctx context.Context, id string) {
	_ = s.db.WithContext(ctx).Model(&domain.APIKey{}).Where("id = ?", id).Update("last_used_at", time.Now().UTC()).Error
}

func (s *Store) ListAPIKeys(ctx context.Context) ([]domain.APIKey, error) {
	var out []domain.APIKey
	err := s.db.WithContext(ctx).Where("revoked_at IS NULL").Order("created_at DESC, id DESC").Find(&out).Error
	return out, err
}

func (s *Store) RevokeAPIKey(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Model(&domain.APIKey{}).Where("id = ?", id).Update("revoked_at", time.Now().UTC()).Error
}

func (s *Store) AddAudit(ctx context.Context, e domain.AuditEvent) {
	e.CreatedAt = time.Now().UTC()
	if e.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return
		}
		e.ID = id
	}
	_ = s.db.WithContext(ctx).Create(&e).Error
}

func (s *Store) ListAudit(ctx context.Context, q PageQuery) ([]domain.AuditEvent, int64, error) {
	q = q.Normalized()
	db := s.db.WithContext(ctx).Model(&domain.AuditEvent{})
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []domain.AuditEvent
	err := db.Order(OrderClause(q.Sort)).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&out).Error
	return out, total, err
}
