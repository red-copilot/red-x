package easm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"github.com/red-copilot/red-x/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type ICPServiceStore interface {
	CreateOrganization(ctx context.Context, org *domain.Organization) error
	FindOrganizationByName(ctx context.Context, name string) (*domain.Organization, error)
	GetOrganization(ctx context.Context, id bson.ObjectID) (*domain.Organization, error)
	UpsertOrganizationDomain(ctx context.Context, od *domain.OrganizationDomain) error
	ConfirmOrganizationDomains(ctx context.Context, orgID bson.ObjectID, domains []string) (int, error)
	AddICPEvidence(ctx context.Context, ev *domain.ICPEvidence) error
	CreateICPJob(ctx context.Context, job *domain.ICPJob) error
	UpdateICPJob(ctx context.Context, id bson.ObjectID, fields bson.M) error
	GetICPJob(ctx context.Context, id bson.ObjectID) (*domain.ICPJob, error)
	AddAudit(ctx context.Context, e domain.AuditEvent)
}

type LookupResult struct {
	Organization domain.Organization `json:"organization"`
	Attribution  DomainICPResult     `json:"attribution"`
	Confidence   domain.Confidence   `json:"confidence"`
}

func (s *Service) LookupDomain(ctx context.Context, rawDomain, actor string) (*LookupResult, error) {
	if s.Chinaz == nil || s.Store == nil {
		return nil, ErrICPUnavailable
	}
	root, err := NormalizeRootDomain(rawDomain)
	if err != nil {
		return nil, err
	}
	res, err := s.Chinaz.LookupByDomain(ctx, root)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	ev := &domain.ICPEvidence{
		Provider: "chinaz", QueryType: "domain", QueryKey: root, Raw: res.RawHTML,
		FetchedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour), PayloadHash: payloadHash(res.RawHTML),
	}
	_ = s.Store.AddICPEvidence(ctx, ev)
	name := strings.TrimSpace(res.OrgName)
	org, err := s.Store.FindOrganizationByName(ctx, NormalizeOrgName(name))
	if err != nil {
		return nil, err
	}
	if org == nil {
		org, err = s.Store.FindOrganizationByName(ctx, name)
		if err != nil {
			return nil, err
		}
	}
	if org == nil {
		org = &domain.Organization{
			CanonicalName: name, Aliases: []string{NormalizeOrgName(name)},
			Region: LicenseRegion(res.ICPNo), Status: domain.OrgStatusActive,
		}
		if err := s.Store.CreateOrganization(ctx, org); err != nil {
			return nil, err
		}
	}
	conf := MatchConfidence(res.OrgName, res.ICPNo, org.CanonicalName, res.ICPNo)
	od := &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: root, ICPNo: res.ICPNo, AuditDate: res.AuditDate,
		Source: "chinaz", Confidence: conf, Status: domain.DomainStatusCandidate, EvidenceID: &ev.ID,
	}
	if err := s.Store.UpsertOrganizationDomain(ctx, od); err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "easm.icp_lookup", org.ID.Hex(), map[string]any{
		"domain": root, "organization": org.CanonicalName, "confidence": conf,
	})
	return &LookupResult{Organization: *org, Attribution: res, Confidence: conf}, nil
}

func (s *Service) EnqueueRefresh(ctx context.Context, orgID bson.ObjectID, actor string) (*domain.ICPJob, error) {
	if s.Store == nil {
		return nil, ErrICPUnavailable
	}
	org, err := s.Store.GetOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, ErrOrganizationNotFound
	}
	job := &domain.ICPJob{OrganizationID: orgID, Status: domain.ICPJobQueued, RequestedBy: actor}
	if err := s.Store.CreateICPJob(ctx, job); err != nil {
		return nil, err
	}
	if s.Queue != nil {
		payload, _ := json.Marshal(map[string]string{"jobId": job.ID.Hex()})
		if err := s.Queue.Publish(ctx, "q.icp", payload); err != nil {
			_ = s.Store.UpdateICPJob(ctx, job.ID, bson.M{
				"status": domain.ICPJobFailed, "error": err.Error(), "finishedAt": time.Now().UTC(),
			})
			return nil, err
		}
	}
	s.audit(ctx, actor, "easm.icp_refresh", orgID.Hex(), map[string]any{"jobId": job.ID.Hex()})
	return job, nil
}

func (s *Service) HandleICPJob(ctx context.Context, d jobs.Delivery) error {
	if s.Store == nil || s.ICPQuery == nil {
		return ErrICPUnavailable
	}
	var payload struct {
		JobID string `json:"jobId"`
	}
	if err := json.Unmarshal(d.Envelope.Payload, &payload); err != nil {
		return err
	}
	id, err := store.ObjectID(payload.JobID)
	if err != nil {
		return err
	}
	job, err := s.Store.GetICPJob(ctx, id)
	if err != nil {
		return err
	}
	if job == nil || job.Status != domain.ICPJobQueued {
		return nil
	}
	if err := s.Store.UpdateICPJob(ctx, id, bson.M{"status": domain.ICPJobRunning}); err != nil {
		return err
	}
	org, err := s.Store.GetOrganization(ctx, job.OrganizationID)
	if err != nil {
		return s.failJob(ctx, id, err)
	}
	if org == nil {
		return s.failJob(ctx, id, ErrOrganizationNotFound)
	}
	records, err := s.ICPQuery.SearchByOrganization(ctx, org.CanonicalName)
	if err != nil {
		return s.failJob(ctx, id, err)
	}
	count := 0
	for _, rec := range records {
		conf := domain.ConfidenceLow
		if NormalizeOrgName(rec.UnitName) == NormalizeOrgName(org.CanonicalName) {
			conf = domain.ConfidenceMedium
		}
		od := &domain.OrganizationDomain{
			OrganizationID: org.ID, RootDomain: rec.Domain, SiteName: rec.ServiceName,
			ICPNo: rec.ServiceLicence, Source: "icp_query", Confidence: conf, Status: domain.DomainStatusCandidate,
		}
		if err := s.Store.UpsertOrganizationDomain(ctx, od); err != nil {
			continue
		}
		count++
	}
	now := time.Now().UTC()
	if err := s.Store.UpdateICPJob(ctx, id, bson.M{
		"status": domain.ICPJobDone, "resultCount": count, "finishedAt": now,
	}); err != nil {
		return err
	}
	return nil
}

func (s *Service) failJob(ctx context.Context, id bson.ObjectID, cause error) error {
	if s.Log != nil {
		s.Log.Warn("ICP refresh failed", "jobId", id.Hex(), "error", cause)
	}
	_ = s.Store.UpdateICPJob(ctx, id, bson.M{
		"status": domain.ICPJobFailed, "error": cause.Error(), "finishedAt": time.Now().UTC(),
	})
	return nil
}

func (s *Service) ConfirmDomains(ctx context.Context, orgID bson.ObjectID, domains []string, actor string) (int, error) {
	if s.Store == nil {
		return 0, ErrICPUnavailable
	}
	n, err := s.Store.ConfirmOrganizationDomains(ctx, orgID, domains)
	if err != nil {
		return 0, err
	}
	s.audit(ctx, actor, "easm.domain_confirm", orgID.Hex(), map[string]any{"domains": domains, "changed": n})
	return n, nil
}

func (s *Service) GetICPJob(ctx context.Context, id bson.ObjectID) (*domain.ICPJob, error) {
	if s.Store == nil {
		return nil, ErrICPUnavailable
	}
	return s.Store.GetICPJob(ctx, id)
}

func (s *Service) audit(ctx context.Context, actor, action, resource string, detail map[string]any) {
	if s.Store == nil {
		return
	}
	s.Store.AddAudit(ctx, domain.AuditEvent{Actor: actor, Action: action, Resource: resource, Detail: detail})
}

func payloadHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
