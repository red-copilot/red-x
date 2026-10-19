package store

import (
	"context"
	"errors"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func DefaultPolicy() domain.Policy {
	return domain.Policy{
		Name:        "默认策略",
		Description: "系统默认资产发现策略",
		Config: domain.PolicyConfig{
			Domain: domain.DomainPolicy{DomainBrute: true, DomainBruteType: "test", AltDNS: false, ARLSearch: true, DNSQueryPlugin: false},
			IP: domain.IPPolicy{
				PortScan: true, PortScanType: "test", ServiceDetection: false, OSDetection: false,
				SSLCert: false, SkipScanCDNIP: true, HostTimeout: 900, PortParallelism: 32, PortMinRate: 60,
			},
			Site: domain.SitePolicy{SiteIdentify: true, SiteCapture: false, SearchEngines: false, SiteSpider: false, NucleiScan: false, WebInfoHunter: false},
		},
	}
}

func (s *Store) SeedDefaultPolicy(ctx context.Context) error {
	n, err := s.C(ColPolicies).CountDocuments(ctx, bson.M{})
	if err != nil || n > 0 {
		return err
	}
	p := DefaultPolicy()
	p.CreatedAt, p.UpdatedAt, p.SchemaVersion = time.Now().UTC(), time.Now().UTC(), domain.SchemaVersion
	_, err = s.C(ColPolicies).InsertOne(ctx, p)
	return err
}

func (s *Store) CreatePolicy(ctx context.Context, p *domain.Policy) error {
	now := time.Now().UTC()
	p.CreatedAt, p.UpdatedAt, p.SchemaVersion = now, now, domain.SchemaVersion
	if p.ID.IsZero() {
		p.ID = bson.NewObjectID()
	}
	_, err := s.C(ColPolicies).InsertOne(ctx, p)
	return err
}

func (s *Store) GetPolicy(ctx context.Context, id bson.ObjectID) (*domain.Policy, error) {
	var p domain.Policy
	err := s.C(ColPolicies).FindOne(ctx, bson.M{"_id": id}).Decode(&p)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &p, err
}

func (s *Store) ListPolicies(ctx context.Context) ([]domain.Policy, error) {
	cur, err := s.C(ColPolicies).Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Policy
	err = cur.All(ctx, &out)
	return out, err
}

func (s *Store) UpdatePolicy(ctx context.Context, p *domain.Policy) error {
	p.UpdatedAt = time.Now().UTC()
	_, err := s.C(ColPolicies).ReplaceOne(ctx, bson.M{"_id": p.ID}, p)
	return err
}

func (s *Store) DeletePolicy(ctx context.Context, id bson.ObjectID) error {
	_, err := s.C(ColPolicies).DeleteOne(ctx, bson.M{"_id": id})
	return err
}
