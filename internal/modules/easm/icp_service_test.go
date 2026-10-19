package easm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type fakeChinaz struct {
	result DomainICPResult
	err    error
}

func (f fakeChinaz) LookupByDomain(context.Context, string) (DomainICPResult, error) {
	return f.result, f.err
}

type fakeICPQuery struct {
	records []OrgDomainRecord
	err     error
}

func (f fakeICPQuery) SearchByOrganization(context.Context, string) ([]OrgDomainRecord, error) {
	return f.records, f.err
}

type fakeQueue struct {
	published []struct {
		queue   string
		payload []byte
	}
}

func (q *fakeQueue) Publish(_ context.Context, queue string, payload []byte) error {
	q.published = append(q.published, struct {
		queue   string
		payload []byte
	}{queue, payload})
	return nil
}
func (q *fakeQueue) Consume(context.Context, []string, int, jobs.Handler) error { return nil }
func (q *fakeQueue) Health(context.Context) error                               { return nil }
func (q *fakeQueue) Close() error                                               { return nil }

type fakeICPStore struct {
	orgs     map[bson.ObjectID]*domain.Organization
	byName   map[string]*domain.Organization
	domains  map[string]*domain.OrganizationDomain
	evidence []*domain.ICPEvidence
	jobs     map[bson.ObjectID]*domain.ICPJob
	audits   []domain.AuditEvent
}

func newFakeICPStore() *fakeICPStore {
	return &fakeICPStore{
		orgs: map[bson.ObjectID]*domain.Organization{}, byName: map[string]*domain.Organization{},
		domains: map[string]*domain.OrganizationDomain{}, jobs: map[bson.ObjectID]*domain.ICPJob{},
	}
}

func (f *fakeICPStore) CreateOrganization(_ context.Context, org *domain.Organization) error {
	if org.ID.IsZero() {
		org.ID = bson.NewObjectID()
	}
	f.orgs[org.ID] = org
	f.byName[org.CanonicalName] = org
	for _, alias := range org.Aliases {
		f.byName[alias] = org
	}
	return nil
}
func (f *fakeICPStore) FindOrganizationByName(_ context.Context, name string) (*domain.Organization, error) {
	return f.byName[name], nil
}
func (f *fakeICPStore) GetOrganization(_ context.Context, id bson.ObjectID) (*domain.Organization, error) {
	return f.orgs[id], nil
}
func (f *fakeICPStore) UpsertOrganizationDomain(_ context.Context, od *domain.OrganizationDomain) error {
	key := od.OrganizationID.Hex() + ":" + od.RootDomain
	if od.ID.IsZero() {
		od.ID = bson.NewObjectID()
	}
	f.domains[key] = od
	return nil
}
func (f *fakeICPStore) ConfirmOrganizationDomains(_ context.Context, orgID bson.ObjectID, domains []string) (int, error) {
	n := 0
	for _, root := range domains {
		od := f.domains[orgID.Hex()+":"+root]
		if od != nil && od.Status == domain.DomainStatusCandidate {
			od.Status = domain.DomainStatusConfirmed
			n++
		}
	}
	return n, nil
}
func (f *fakeICPStore) AddICPEvidence(_ context.Context, ev *domain.ICPEvidence) error {
	f.evidence = append(f.evidence, ev)
	return nil
}
func (f *fakeICPStore) CreateICPJob(_ context.Context, job *domain.ICPJob) error {
	if job.ID.IsZero() {
		job.ID = bson.NewObjectID()
	}
	f.jobs[job.ID] = job
	return nil
}
func (f *fakeICPStore) UpdateICPJob(_ context.Context, id bson.ObjectID, fields bson.M) error {
	job := f.jobs[id]
	if job == nil {
		return errors.New("job not found")
	}
	if v, ok := fields["status"]; ok {
		job.Status = v.(domain.ICPJobStatus)
	}
	if v, ok := fields["error"]; ok {
		job.Error, _ = v.(string)
	}
	if v, ok := fields["resultCount"]; ok {
		job.ResultCount, _ = v.(int)
	}
	if v, ok := fields["finishedAt"]; ok {
		t, _ := v.(time.Time)
		job.FinishedAt = &t
	}
	return nil
}
func (f *fakeICPStore) GetICPJob(_ context.Context, id bson.ObjectID) (*domain.ICPJob, error) {
	return f.jobs[id], nil
}
func (f *fakeICPStore) AddAudit(_ context.Context, e domain.AuditEvent) {
	f.audits = append(f.audits, e)
}

func TestLookupDomainCreatesCandidateOrganization(t *testing.T) {
	st := newFakeICPStore()
	svc := &Service{
		Store: st,
		Chinaz: fakeChinaz{result: DomainICPResult{
			RawHTML: "<html/>", OrgName: "四川信息职业技术学院", OrgType: "事业单位",
			ICPNo: "蜀ICP备12031012号-1", AuditDate: "2021-05-14",
		}},
	}
	out, err := svc.LookupDomain(context.Background(), "www.scitc.edu.cn", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if out.Confidence != domain.ConfidenceHigh {
		t.Fatalf("expected high confidence, got %s", out.Confidence)
	}
	if len(st.orgs) != 1 || len(st.domains) != 1 {
		t.Fatalf("expected org and candidate domain, got orgs=%d domains=%d", len(st.orgs), len(st.domains))
	}
	for _, od := range st.domains {
		if od.RootDomain != "scitc.edu.cn" || od.Status != domain.DomainStatusCandidate || od.ICPNo == "" {
			t.Fatalf("unexpected domain record: %+v", od)
		}
	}
	if len(st.evidence) != 1 || len(st.audits) != 1 {
		t.Fatalf("expected evidence and audit, got %d/%d", len(st.evidence), len(st.audits))
	}
}

func TestLookupDomainLowConfidenceStaysCandidate(t *testing.T) {
	st := newFakeICPStore()
	svc := &Service{
		Store:  st,
		Chinaz: fakeChinaz{result: DomainICPResult{OrgName: "某单位", ICPNo: ""}},
	}
	out, err := svc.LookupDomain(context.Background(), "example.com", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if out.Confidence != domain.ConfidenceLow {
		t.Fatalf("expected low confidence, got %s", out.Confidence)
	}
	for _, od := range st.domains {
		if od.Status != domain.DomainStatusCandidate {
			t.Fatalf("low confidence must stay candidate: %+v", od)
		}
	}
}

func TestEnqueueCreatesQueuedJob(t *testing.T) {
	st := newFakeICPStore()
	org := &domain.Organization{CanonicalName: "某单位"}
	_ = st.CreateOrganization(context.Background(), org)
	q := &fakeQueue{}
	svc := &Service{Store: st, Queue: q}
	job, err := svc.EnqueueRefresh(context.Background(), org.ID, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != domain.ICPJobQueued {
		t.Fatalf("expected queued job, got %s", job.Status)
	}
	if len(q.published) != 1 || q.published[0].queue != "q.icp" {
		t.Fatalf("unexpected queue publish: %+v", q.published)
	}
}

func TestHandleJobPersistsCandidatesAndCompletes(t *testing.T) {
	st := newFakeICPStore()
	org := &domain.Organization{CanonicalName: "四川信息职业技术学院"}
	_ = st.CreateOrganization(context.Background(), org)
	job := &domain.ICPJob{OrganizationID: org.ID, Status: domain.ICPJobQueued}
	_ = st.CreateICPJob(context.Background(), job)
	svc := &Service{
		Store: st,
		ICPQuery: fakeICPQuery{records: []OrgDomainRecord{
			{Domain: "scitc.edu.cn", UnitName: "四川信息职业技术学院", ServiceLicence: "蜀ICP备1号"},
			{Domain: "other.com", UnitName: "其他单位", ServiceLicence: "蜀ICP备2号"},
		}},
	}
	payload, _ := json.Marshal(map[string]string{"jobId": job.ID.Hex()})
	if err := svc.HandleICPJob(context.Background(), jobs.Delivery{Envelope: jobs.Envelope{Payload: payload}}); err != nil {
		t.Fatal(err)
	}
	got := st.jobs[job.ID]
	if got.Status != domain.ICPJobDone || got.ResultCount != 2 {
		t.Fatalf("unexpected job: %+v", got)
	}
	if len(st.domains) != 2 {
		t.Fatalf("expected 2 candidate domains, got %d", len(st.domains))
	}
}

func TestHandleJobProviderFailureKeepsExistingDomains(t *testing.T) {
	st := newFakeICPStore()
	org := &domain.Organization{CanonicalName: "某单位"}
	_ = st.CreateOrganization(context.Background(), org)
	existing := &domain.OrganizationDomain{OrganizationID: org.ID, RootDomain: "example.com", Source: "manual", Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusConfirmed}
	_ = st.UpsertOrganizationDomain(context.Background(), existing)
	job := &domain.ICPJob{OrganizationID: org.ID, Status: domain.ICPJobQueued}
	_ = st.CreateICPJob(context.Background(), job)
	svc := &Service{Store: st, ICPQuery: fakeICPQuery{err: ErrICPUnavailable}}
	payload, _ := json.Marshal(map[string]string{"jobId": job.ID.Hex()})
	if err := svc.HandleICPJob(context.Background(), jobs.Delivery{Envelope: jobs.Envelope{Payload: payload}}); err != nil {
		t.Fatal(err)
	}
	if st.jobs[job.ID].Status != domain.ICPJobFailed {
		t.Fatalf("expected failed job, got %s", st.jobs[job.ID].Status)
	}
	if st.domains[org.ID.Hex()+":example.com"].Status != domain.DomainStatusConfirmed {
		t.Fatal("existing confirmed domain was modified")
	}
}

func TestConfirmDomainsWritesAuditAndOutbox(t *testing.T) {
	st := newFakeICPStore()
	org := &domain.Organization{CanonicalName: "某单位"}
	_ = st.CreateOrganization(context.Background(), org)
	od := &domain.OrganizationDomain{OrganizationID: org.ID, RootDomain: "example.com", Source: "icp_query", Confidence: domain.ConfidenceMedium, Status: domain.DomainStatusCandidate}
	_ = st.UpsertOrganizationDomain(context.Background(), od)
	svc := &Service{Store: st}
	n, err := svc.ConfirmDomains(context.Background(), org.ID, []string{"example.com"}, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(st.audits) != 1 {
		t.Fatalf("expected 1 changed and 1 audit, got %d/%d", n, len(st.audits))
	}
	if st.domains[org.ID.Hex()+":example.com"].Status != domain.DomainStatusConfirmed {
		t.Fatal("domain was not confirmed")
	}
}
