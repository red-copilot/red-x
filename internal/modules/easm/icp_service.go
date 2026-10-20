package easm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"github.com/red-copilot/red-x/internal/store"
)

type ICPServiceStore interface {
	CreateOrganization(ctx context.Context, org *domain.Organization) error
	FindOrganizationByName(ctx context.Context, name string) (*domain.Organization, error)
	FindOrganizationByNameAndRegion(ctx context.Context, name, region string) (*domain.Organization, error)
	GetOrganization(ctx context.Context, id string) (*domain.Organization, error)
	UpsertOrganizationDomain(ctx context.Context, od *domain.OrganizationDomain) error
	ListOrganizationDomains(ctx context.Context, orgID string, status string) ([]domain.OrganizationDomain, error)
	ConfirmOrganizationDomains(ctx context.Context, orgID string, domains []string, actor string) (int, error)
	AddICPEvidence(ctx context.Context, ev *domain.ICPEvidence) error
	CreateICPJob(ctx context.Context, job *domain.ICPJob) error
	UpdateICPJob(ctx context.Context, id string, fields store.ICPJobPatch) error
	GetICPJob(ctx context.Context, id string) (*domain.ICPJob, error)
	AddAudit(ctx context.Context, e domain.AuditEvent)
	AppendGraphOutbox(ctx context.Context, entityType, entityID string, op domain.GraphOutboxOp) error
}

type LookupResult struct {
	Organization domain.Organization `json:"organization"`
	Attribution  DomainICPResult     `json:"attribution"`
	Confidence   domain.Confidence   `json:"confidence"`
	Provider     string              `json:"provider"`
}

type providerLookupResult struct {
	result   DomainICPResult
	provider string
}

func (s *Service) LookupDomain(ctx context.Context, rawDomain, actor string) (*LookupResult, error) {
	if s.Store == nil {
		return nil, ErrICPUnavailable
	}
	root, err := NormalizeRootDomain(rawDomain)
	if err != nil {
		return nil, err
	}
	v, err, _ := s.lookupGroup.Do(root, func() (any, error) {
		var chinazErr error
		if s.Chinaz != nil {
			if err := s.chinazGuard.acquire(ctx); err == nil {
				out, err := s.Chinaz.LookupByDomain(ctx, root)
				if err == nil {
					s.chinazGuard.success()
					s.setHealth("chinaz", "ok")
					return providerLookupResult{result: out, provider: "chinaz"}, nil
				}
				chinazErr = err
				s.chinazGuard.failure()
				s.setHealth("chinaz", icpHealth(err))
			} else {
				chinazErr = err
			}
		}
		if s.ICPQuery != nil {
			out, err := s.lookupDomainWithICPQuery(ctx, root)
			if err == nil {
				s.setHealth("icpQuery", "ok")
				return providerLookupResult{result: out, provider: "icp_query"}, nil
			}
			if chinazErr != nil {
				return nil, chinazErr
			}
			return nil, err
		}
		if chinazErr != nil {
			return nil, chinazErr
		}
		return nil, ErrICPUnavailable
	})
	if err != nil {
		return nil, err
	}
	looked := v.(providerLookupResult)
	res, provider := looked.result, looked.provider
	now := time.Now().UTC()
	ev := &domain.ICPEvidence{
		Provider: provider, QueryType: "domain", QueryKey: root, Raw: res.RawHTML,
		FetchedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour), PayloadHash: payloadHash(res.RawHTML),
	}
	_ = s.Store.AddICPEvidence(ctx, ev)
	name := strings.TrimSpace(res.OrgName)
	region := LicenseRegion(res.ICPNo)
	org, err := s.Store.FindOrganizationByNameAndRegion(ctx, NormalizeOrgName(name), region)
	if err != nil {
		return nil, err
	}
	if org == nil {
		org, err = s.Store.FindOrganizationByNameAndRegion(ctx, name, region)
		if err != nil {
			return nil, err
		}
	}
	if org == nil {
		org = &domain.Organization{
			CanonicalName: name, Aliases: []string{NormalizeOrgName(name)},
			Region: region, Status: domain.OrgStatusActive,
		}
		if err := s.Store.CreateOrganization(ctx, org); err != nil {
			return nil, err
		}
		_ = s.Store.AppendGraphOutbox(ctx, "organization", org.ID, domain.OutboxOpUpsert)
	}
	storedLicense := ""
	if confirmed, err := s.Store.ListOrganizationDomains(ctx, org.ID, string(domain.DomainStatusConfirmed)); err == nil {
		for _, d := range confirmed {
			if d.ICPNo != "" {
				storedLicense = d.ICPNo
				break
			}
		}
	}
	conf := MatchConfidence(res.OrgName, res.ICPNo, org.CanonicalName, storedLicense)
	if conf == domain.ConfidenceLow && region != "" && region == org.Region {
		conf = domain.ConfidenceMedium
	}
	od := &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: root, ICPNo: res.ICPNo, AuditDate: res.AuditDate,
		Source: provider, Confidence: conf, Status: domain.DomainStatusCandidate, EvidenceID: &ev.ID,
	}
	if err := s.Store.UpsertOrganizationDomain(ctx, od); err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "easm.icp_lookup", org.ID, map[string]any{
		"domain": root, "organization": org.CanonicalName, "confidence": conf,
	})
	return &LookupResult{Organization: *org, Attribution: res, Confidence: conf, Provider: provider}, nil
}

func (s *Service) lookupDomainWithICPQuery(ctx context.Context, root string) (DomainICPResult, error) {
	records, err := s.ICPQuery.SearchByOrganization(ctx, root)
	if err != nil {
		return DomainICPResult{}, err
	}
	var selected *OrgDomainRecord
	for i := range records {
		normalized, err := NormalizeRootDomain(records[i].Domain)
		if err == nil && normalized == root {
			selected = &records[i]
			break
		}
	}
	if selected == nil && len(records) > 0 {
		selected = &records[0]
	}
	if selected == nil || strings.TrimSpace(selected.UnitName) == "" {
		return DomainICPResult{}, ErrICPNoAttribution
	}
	raw, _ := json.Marshal(records)
	return DomainICPResult{
		OrgName: strings.TrimSpace(selected.UnitName), ICPNo: strings.TrimSpace(selected.ServiceLicence),
		RawHTML: string(raw),
	}, nil
}

type providerGuard struct {
	mu        sync.Mutex
	interval  time.Duration
	next      time.Time
	failures  int
	openUntil time.Time
}

func (g *providerGuard) acquire(ctx context.Context) error {
	g.mu.Lock()
	now := time.Now()
	if now.Before(g.openUntil) {
		g.mu.Unlock()
		return ErrICPUnavailable
	}
	wait := time.Until(g.next)
	g.next = now.Add(g.interval)
	g.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (g *providerGuard) success() {
	g.mu.Lock()
	g.failures = 0
	g.openUntil = time.Time{}
	g.mu.Unlock()
}

func (g *providerGuard) failure() {
	g.mu.Lock()
	g.failures++
	if g.failures >= 5 {
		g.openUntil = time.Now().Add(10 * time.Minute)
		g.failures = 0
	}
	g.mu.Unlock()
}

func icpHealth(err error) string {
	switch {
	case errors.Is(err, ErrICPCookieExpired):
		return "cookie_expired"
	case errors.Is(err, ErrICPNoAttribution):
		return "ok"
	default:
		return "unavailable"
	}
}

func (s *Service) EnqueueRefresh(ctx context.Context, orgID string, actor string) (*domain.ICPJob, error) {
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
		payload, _ := json.Marshal(map[string]string{"jobId": job.ID})
		if err := s.Queue.Publish(ctx, "q.icp", payload); err != nil {
			now := time.Now().UTC()
			_ = s.Store.UpdateICPJob(ctx, job.ID, store.ICPJobPatch{Status: ptrValue(domain.ICPJobFailed), Error: ptrValue(err.Error()), FinishedAt: &now})
			return nil, err
		}
	}
	s.audit(ctx, actor, "easm.icp_refresh", orgID, map[string]any{"jobId": job.ID})
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
	id, err := domain.ParseID(payload.JobID)
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
	if err := s.Store.UpdateICPJob(ctx, id, store.ICPJobPatch{Status: ptrValue(domain.ICPJobRunning)}); err != nil {
		return err
	}
	org, err := s.Store.GetOrganization(ctx, job.OrganizationID)
	if err != nil {
		return s.failJob(ctx, id, err)
	}
	if org == nil {
		return s.failJob(ctx, id, ErrOrganizationNotFound)
	}
	if err := s.icpGuard.acquire(ctx); err != nil {
		return s.failJob(ctx, id, err)
	}
	records, err := s.ICPQuery.SearchByOrganization(ctx, org.CanonicalName)
	if err != nil {
		s.icpGuard.failure()
		s.setHealth("icpQuery", "unavailable")
		return s.failJob(ctx, id, err)
	}
	s.icpGuard.success()
	s.setHealth("icpQuery", "ok")
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
	if err := s.Store.UpdateICPJob(ctx, id, store.ICPJobPatch{Status: ptrValue(domain.ICPJobDone), ResultCount: &count, FinishedAt: &now}); err != nil {
		return err
	}
	return nil
}

func (s *Service) failJob(ctx context.Context, id string, cause error) error {
	if s.Log != nil {
		s.Log.Warn("ICP refresh failed", "jobId", id, "error", cause)
	}
	now := time.Now().UTC()
	_ = s.Store.UpdateICPJob(ctx, id, store.ICPJobPatch{Status: ptrValue(domain.ICPJobFailed), Error: ptrValue(cause.Error()), FinishedAt: &now})
	return nil
}

func (s *Service) ConfirmDomains(ctx context.Context, orgID string, domains []string, actor string) (int, error) {
	if s.Store == nil {
		return 0, ErrICPUnavailable
	}
	n, err := s.Store.ConfirmOrganizationDomains(ctx, orgID, domains, actor)
	if err != nil {
		return 0, err
	}
	s.audit(ctx, actor, "easm.domain_confirm", orgID, map[string]any{"domains": domains, "changed": n})
	return n, nil
}

func (s *Service) GetICPJob(ctx context.Context, id string) (*domain.ICPJob, error) {
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

func (s *Service) setHealth(field, value string) {
	s.healthMu.Lock()
	defer s.healthMu.Unlock()
	switch field {
	case "chinaz":
		s.health.Chinaz = value
	case "icpQuery":
		s.health.ICPQuery = value
	case "geoip":
		s.health.GeoIP = value
	case "neo4j":
		s.health.Neo4j = value
	}
}

func payloadHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func ptrValue[T any](v T) *T { return &v }
