package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *Store) CreateOrganization(ctx context.Context, org *domain.Organization) error {
	if err := org.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if org.ID.IsZero() {
		org.ID = bson.NewObjectID()
	}
	org.CanonicalName = strings.TrimSpace(org.CanonicalName)
	org.CreatedAt = now
	org.UpdatedAt = now
	org.SchemaVersion = domain.SchemaVersion
	_, err := s.C(ColOrganizations).InsertOne(ctx, org)
	return err
}

func (s *Store) GetOrganization(ctx context.Context, id bson.ObjectID) (*domain.Organization, error) {
	var org domain.Organization
	err := s.C(ColOrganizations).FindOne(ctx, bson.M{"_id": id}).Decode(&org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &org, err
}

func (s *Store) FindOrganizationByName(ctx context.Context, name string) (*domain.Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	var org domain.Organization
	err := s.C(ColOrganizations).FindOne(ctx, bson.M{
		"status": domain.OrgStatusActive,
		"$or":    bson.A{bson.M{"canonicalName": name}, bson.M{"aliases": name}},
	}).Decode(&org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &org, err
}

func (s *Store) ListOrganizations(ctx context.Context, query string, limit int) ([]domain.Organization, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	filter := bson.M{"status": domain.OrgStatusActive}
	if q := strings.TrimSpace(query); q != "" {
		re := regexp.QuoteMeta(q)
		filter["$or"] = bson.A{
			bson.M{"canonicalName": bson.M{"$regex": re, "$options": "i"}},
			bson.M{"aliases": bson.M{"$regex": re, "$options": "i"}},
		}
	}
	opts := options.Find().SetSort(bson.D{{Key: "canonicalName", Value: 1}}).SetLimit(int64(limit))
	cur, err := s.C(ColOrganizations).Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Organization
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) UpsertOrganizationDomain(ctx context.Context, od *domain.OrganizationDomain) error {
	if err := od.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if od.ID.IsZero() {
		od.ID = bson.NewObjectID()
	}
	od.RootDomain = strings.TrimSpace(strings.ToLower(od.RootDomain))
	od.LastSeen = now
	od.UpdatedAt = now
	od.SchemaVersion = domain.SchemaVersion
	filter := bson.M{"organizationId": od.OrganizationID, "rootDomain": od.RootDomain}
	update := bson.M{
		"$setOnInsert": bson.M{"_id": od.ID, "firstSeen": now},
		"$set": bson.M{
			"siteName": od.SiteName, "homeSite": od.HomeSite, "icpNo": od.ICPNo,
			"auditDate": od.AuditDate, "source": od.Source, "confidence": od.Confidence,
			"status": od.Status, "evidenceId": od.EvidenceID, "lastSeen": od.LastSeen,
			"updatedAt": od.UpdatedAt, "schemaVersion": od.SchemaVersion,
		},
	}
	_, err := s.C(ColOrganizationDomains).UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return err
}

func (s *Store) ListOrganizationDomains(ctx context.Context, orgID bson.ObjectID, status string) ([]domain.OrganizationDomain, error) {
	filter := bson.M{"organizationId": orgID}
	if status != "" {
		filter["status"] = status
	}
	opts := options.Find().SetSort(bson.D{{Key: "rootDomain", Value: 1}})
	cur, err := s.C(ColOrganizationDomains).Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.OrganizationDomain
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ConfirmOrganizationDomains(ctx context.Context, orgID bson.ObjectID, domains []string) (int, error) {
	if len(domains) == 0 {
		return 0, nil
	}
	normalized := make([]string, 0, len(domains))
	for _, d := range domains {
		if d = strings.TrimSpace(strings.ToLower(d)); d != "" {
			normalized = append(normalized, d)
		}
	}
	if len(normalized) == 0 {
		return 0, nil
	}
	filter := bson.M{"organizationId": orgID, "rootDomain": bson.M{"$in": normalized}, "status": domain.DomainStatusCandidate}
	cur, err := s.C(ColOrganizationDomains).Find(ctx, filter)
	if err != nil {
		return 0, err
	}
	var pending []domain.OrganizationDomain
	if err := cur.All(ctx, &pending); err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil
	}
	ids := make([]bson.ObjectID, 0, len(pending))
	for _, od := range pending {
		ids = append(ids, od.ID)
	}
	now := time.Now().UTC()
	if _, err := s.C(ColOrganizationDomains).UpdateMany(ctx,
		bson.M{"_id": bson.M{"$in": ids}},
		bson.M{"$set": bson.M{"status": domain.DomainStatusConfirmed, "lastSeen": now, "updatedAt": now}},
	); err != nil {
		return 0, err
	}
	for _, od := range pending {
		if err := s.AppendGraphOutbox(ctx, "organization_domain", od.ID.Hex(), domain.OutboxOpUpsert); err != nil {
			return len(ids), fmt.Errorf("append outbox for %s: %w", od.RootDomain, err)
		}
	}
	return len(ids), nil
}

func (s *Store) AddICPEvidence(ctx context.Context, ev *domain.ICPEvidence) error {
	if ev == nil || strings.TrimSpace(ev.Provider) == "" || strings.TrimSpace(ev.QueryKey) == "" {
		return errors.New("invalid ICP evidence")
	}
	if ev.ID.IsZero() {
		ev.ID = bson.NewObjectID()
	}
	if ev.FetchedAt.IsZero() {
		ev.FetchedAt = time.Now().UTC()
	}
	ev.SchemaVersion = domain.SchemaVersion
	_, err := s.C(ColICPEvidence).InsertOne(ctx, ev)
	return err
}

func (s *Store) UpsertIPLocation(ctx context.Context, loc *domain.IPLocation) error {
	if loc == nil || strings.TrimSpace(loc.IP) == "" {
		return errors.New("IP location requires ip")
	}
	loc.UpdatedAt = time.Now().UTC()
	loc.SchemaVersion = domain.SchemaVersion
	_, err := s.C(ColIPLocations).UpdateOne(ctx, bson.M{"ip": loc.IP}, bson.M{
		"$set": bson.M{
			"lat": loc.Lat, "lon": loc.Lon, "country": loc.Country, "province": loc.Province,
			"city": loc.City, "asn": loc.ASN, "asnOrg": loc.ASNOrg, "source": loc.Source,
			"updatedAt": loc.UpdatedAt, "schemaVersion": loc.SchemaVersion,
		},
		"$setOnInsert": bson.M{"_id": loc.ID},
	}, options.UpdateOne().SetUpsert(true))
	return err
}

func (s *Store) GetIPLocation(ctx context.Context, ip string) (*domain.IPLocation, error) {
	var loc domain.IPLocation
	err := s.C(ColIPLocations).FindOne(ctx, bson.M{"ip": ip}).Decode(&loc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &loc, err
}

func (s *Store) CreateICPJob(ctx context.Context, job *domain.ICPJob) error {
	if job == nil || job.OrganizationID.IsZero() {
		return errors.New("ICP job requires organizationId")
	}
	if job.ID.IsZero() {
		job.ID = bson.NewObjectID()
	}
	if job.Status == "" {
		job.Status = domain.ICPJobQueued
	}
	job.CreatedAt = time.Now().UTC()
	job.SchemaVersion = domain.SchemaVersion
	_, err := s.C(ColICPJobs).InsertOne(ctx, job)
	return err
}

func (s *Store) UpdateICPJob(ctx context.Context, id bson.ObjectID, fields bson.M) error {
	if len(fields) == 0 {
		return nil
	}
	_, err := s.C(ColICPJobs).UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": fields})
	return err
}

func (s *Store) GetICPJob(ctx context.Context, id bson.ObjectID) (*domain.ICPJob, error) {
	var job domain.ICPJob
	err := s.C(ColICPJobs).FindOne(ctx, bson.M{"_id": id}).Decode(&job)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &job, err
}

func (s *Store) AppendGraphOutbox(ctx context.Context, entityType, entityID string, op domain.GraphOutboxOp) error {
	if strings.TrimSpace(entityType) == "" || strings.TrimSpace(entityID) == "" {
		return errors.New("graph outbox requires entity type and id")
	}
	if !op.Valid() {
		return errors.New("invalid graph outbox op")
	}
	ev := &domain.GraphOutbox{
		ID:            bson.NewObjectID(),
		EntityType:    entityType,
		EntityID:      entityID,
		Op:            op,
		CreatedAt:     time.Now().UTC(),
		SchemaVersion: domain.SchemaVersion,
	}
	_, err := s.C(ColGraphOutbox).InsertOne(ctx, ev)
	return err
}

func (s *Store) ClaimGraphOutbox(ctx context.Context, limit int) ([]domain.GraphOutbox, error) {
	if limit <= 0 {
		limit = 100
	}
	now := time.Now().UTC()
	stale := now.Add(-5 * time.Minute)
	var out []domain.GraphOutbox
	for i := 0; i < limit; i++ {
		filter := bson.M{
			"processedAt": nil,
			"attempts":    bson.M{"$lt": 5},
			"$or": bson.A{
				bson.M{"processingAt": nil},
				bson.M{"processingAt": bson.M{"$lt": stale}},
			},
		}
		opts := options.FindOneAndUpdate().
			SetSort(bson.D{{Key: "createdAt", Value: 1}}).
			SetReturnDocument(options.After)
		var ev domain.GraphOutbox
		err := s.C(ColGraphOutbox).FindOneAndUpdate(ctx, filter, bson.M{
			"$set": bson.M{"processingAt": now},
			"$inc": bson.M{"attempts": 1},
		}, opts).Decode(&ev)
		if errors.Is(err, mongo.ErrNoDocuments) {
			break
		}
		if err != nil {
			return out, err
		}
		out = append(out, ev)
	}
	return out, nil
}

func (s *Store) MarkGraphOutboxProcessed(ctx context.Context, ids []bson.ObjectID) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now().UTC()
	_, err := s.C(ColGraphOutbox).UpdateMany(ctx, bson.M{"_id": bson.M{"$in": ids}}, bson.M{
		"$set":   bson.M{"processedAt": now},
		"$unset": bson.M{"processingAt": ""},
	})
	return err
}

func (s *Store) GetAssetByID(ctx context.Context, id bson.ObjectID) (*domain.Asset, error) {
	var a domain.Asset
	err := s.C(ColAssets).FindOne(ctx, bson.M{"_id": id}).Decode(&a)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &a, err
}

func (s *Store) GetFindingByID(ctx context.Context, id bson.ObjectID) (*domain.Finding, error) {
	var f domain.Finding
	err := s.C(ColFindings).FindOne(ctx, bson.M{"_id": id}).Decode(&f)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &f, err
}

func (s *Store) GetOrganizationDomainByID(ctx context.Context, id bson.ObjectID) (*domain.OrganizationDomain, error) {
	var od domain.OrganizationDomain
	err := s.C(ColOrganizationDomains).FindOne(ctx, bson.M{"_id": id}).Decode(&od)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &od, err
}

func (s *Store) ListAllOrganizations(ctx context.Context) ([]domain.Organization, error) {
	cur, err := s.C(ColOrganizations).Find(ctx, bson.M{"status": domain.OrgStatusActive})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Organization
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListAllOrganizationDomains(ctx context.Context, status string) ([]domain.OrganizationDomain, error) {
	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}
	cur, err := s.C(ColOrganizationDomains).Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.OrganizationDomain
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ClearGraphOutbox(ctx context.Context) error {
	_, err := s.C(ColGraphOutbox).DeleteMany(ctx, bson.M{})
	return err
}
