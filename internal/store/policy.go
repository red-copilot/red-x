package store

import (
	"context"
	"fmt"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"gorm.io/gorm"
)

func DefaultPolicy() domain.Policy {
	return domain.Policy{
		Name:        "默认策略",
		Description: "系统默认资产发现策略",
		Config: domain.PolicyConfig{
			Domain: domain.DomainPolicy{DomainBrute: false, DomainBruteType: "test", AltDNS: false, ARLSearch: true, DNSQueryPlugin: true},
			IP: domain.IPPolicy{
				PortScan: true, PortScanType: "top1000", ServiceDetection: false, OSDetection: false,
				SSLCert: false, SkipScanCDNIP: true, HostTimeout: 900, PortParallelism: 32, PortMinRate: 60,
			},
			Site: domain.SitePolicy{SiteIdentify: true, SiteCapture: false, SearchEngines: false, SiteSpider: false, NucleiScan: false, WebInfoHunter: false},
		},
	}
}

func (s *Store) SeedDefaultPolicy(ctx context.Context) error {
	var n int64
	if err := s.db.WithContext(ctx).Model(&domain.Policy{}).Count(&n).Error; err != nil || n > 0 {
		return err
	}
	p := DefaultPolicy()
	p.CreatedAt, p.UpdatedAt, p.SchemaVersion = time.Now().UTC(), time.Now().UTC(), domain.SchemaVersion
	err := s.db.WithContext(ctx).Create(&p).Error
	if isDuplicate(err) {
		return nil
	}
	return err
}

func (s *Store) CreatePolicy(ctx context.Context, p *domain.Policy) error {
	now := time.Now().UTC()
	p.CreatedAt, p.UpdatedAt, p.SchemaVersion = now, now, domain.SchemaVersion
	if p.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return err
		}
		p.ID = id
	}
	return s.db.WithContext(ctx).Create(p).Error
}

func (s *Store) GetPolicy(ctx context.Context, id string) (*domain.Policy, error) {
	var p domain.Policy
	err := active(s.db.WithContext(ctx)).Where("id = ?", id).First(&p).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) ListPolicies(ctx context.Context) ([]domain.Policy, error) {
	var out []domain.Policy
	err := active(s.db.WithContext(ctx)).Order("created_at ASC, id ASC").Find(&out).Error
	return out, err
}

func (s *Store) UpdatePolicy(ctx context.Context, p *domain.Policy) error {
	p.UpdatedAt = time.Now().UTC()
	return s.db.WithContext(ctx).Save(p).Error
}

func (s *Store) DeletePolicy(ctx context.Context, id, actor string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := active(tx).Where("id = ?", id).First(&domain.Policy{}).Error; err != nil {
			if isNotFound(err) {
				return domain.ErrRecordNotFound
			}
			return err
		}
		var monitors, schedules int64
		if err := active(tx).Model(&domain.Monitor{}).Where("policy_id = ?", id).Count(&monitors).Error; err != nil {
			return err
		}
		if err := active(tx).Model(&domain.Schedule{}).
			Where("policy_id = ? AND status <> ?", id, "completed").
			Count(&schedules).Error; err != nil {
			return err
		}
		if monitors+schedules > 0 {
			return fmt.Errorf("%w: %d monitor(s), %d schedule(s)", domain.ErrResourceInUse, monitors, schedules)
		}
		return softDeleteRecordTx(tx, "policy", id, actor)
	})
}
