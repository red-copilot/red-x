package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/easm"
	"github.com/red-copilot/red-x/internal/modules/identity"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type fakeEASM struct {
	lookup      *easm.LookupResult
	lookupErr   error
	mapSnap     *easm.MapSnapshot
	graphSnap   *easm.GraphSnapshot
	graphErr    error
	job         *domain.ICPJob
	confirmN    int
	confirmErr  error
	confirmed   []string
	health      easm.Health
	summary     *easm.OrganizationSummary
	assets      *easm.AssetPage
	asset       *easm.AssetItem
	findings    *easm.FindingPage
	history     []domain.Finding
	tasks       []domain.Task
	scanErr     error
	overview    *easm.Overview
	overviewErr error
	search      *easm.SearchResult
	searchErr   error
}

func (f *fakeEASM) LookupDomain(context.Context, string, string) (*easm.LookupResult, error) {
	return f.lookup, f.lookupErr
}
func (f *fakeEASM) EnqueueRefresh(context.Context, bson.ObjectID, string) (*domain.ICPJob, error) {
	return f.job, nil
}
func (f *fakeEASM) GetICPJob(context.Context, bson.ObjectID) (*domain.ICPJob, error) {
	return f.job, nil
}
func (f *fakeEASM) ConfirmDomains(_ context.Context, _ bson.ObjectID, domains []string, _ string) (int, error) {
	f.confirmed = domains
	return f.confirmN, f.confirmErr
}
func (f *fakeEASM) Map(context.Context, *bson.ObjectID, string) (*easm.MapSnapshot, error) {
	return f.mapSnap, nil
}
func (f *fakeEASM) Graph(context.Context, bson.ObjectID, int, int) (*easm.GraphSnapshot, error) {
	return f.graphSnap, f.graphErr
}
func (f *fakeEASM) Health(context.Context) easm.Health { return f.health }
func (f *fakeEASM) Summary(context.Context, bson.ObjectID) (*easm.OrganizationSummary, error) {
	return f.summary, nil
}
func (f *fakeEASM) Assets(context.Context, *bson.ObjectID, easm.AssetFilter) (*easm.AssetPage, error) {
	return f.assets, nil
}
func (f *fakeEASM) Asset(context.Context, *bson.ObjectID, bson.ObjectID) (*easm.AssetItem, error) {
	return f.asset, nil
}
func (f *fakeEASM) Findings(context.Context, *bson.ObjectID, easm.FindingFilter) (*easm.FindingPage, error) {
	return f.findings, nil
}
func (f *fakeEASM) FindingHistory(context.Context, bson.ObjectID, bson.ObjectID) ([]domain.Finding, error) {
	return f.history, nil
}
func (f *fakeEASM) CreateScans(context.Context, bson.ObjectID, []string, domain.TaskOptions, string, string, string) ([]domain.Task, []easm.ScanError, error) {
	return f.tasks, nil, f.scanErr
}

func withPrincipal(r *http.Request, role domain.Role) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), principalKey{}, &identity.Principal{
		Username: "alice", Role: role,
	}))
}

func TestEASMMapShape(t *testing.T) {
	s := &Server{EASM: &fakeEASM{mapSnap: &easm.MapSnapshot{
		Organizations: []easm.OrgBubble{{OrgID: "a", Name: "单位A"}},
		Points:        []easm.MapPoint{{Key: "1.1.1.1", Lat: 1, Lon: 2}},
	}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/easm/map", nil)
	s.handleEASMMap(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var body easm.MapSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Organizations) != 1 || len(body.Points) != 1 {
		t.Fatalf("unexpected map: %+v", body)
	}
}

func TestEASMGraphShape(t *testing.T) {
	id := bson.NewObjectID()
	s := &Server{EASM: &fakeEASM{graphSnap: &easm.GraphSnapshot{
		Nodes: []easm.GraphNode{{ID: "n1", Type: "domain", Label: "example.com"}},
		Edges: []easm.GraphEdge{{ID: "e1", Type: "OWNS", Source: "o1", Target: "n1"}},
	}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/easm/graph?orgId="+id.Hex()+"&depth=2&limit=300", nil)
	s.handleEASMGraph(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEASMLookupMapsProviderErrors(t *testing.T) {
	s := &Server{EASM: &fakeEASM{lookupErr: easm.ErrICPCookieExpired}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/icp/lookup", strings.NewReader(`{"domain":"guc.edu.cn"}`))
	req = withPrincipal(req, domain.RoleOperator)
	s.handleLookupDomain(rec, req)
	if rec.Code != http.StatusFailedDependency {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "icp_cookie_expired" {
		t.Fatalf("unexpected code: %q", problem.Code)
	}
}

func TestEASMLookupRejectsInvalidDomain(t *testing.T) {
	s := &Server{EASM: &fakeEASM{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/icp/lookup", strings.NewReader(`{"domain":"not a domain"}`))
	req = withPrincipal(req, domain.RoleOperator)
	s.handleLookupDomain(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestEASMConfirmRequiresWrite(t *testing.T) {
	id := bson.NewObjectID()
	fake := &fakeEASM{confirmN: 1}
	s := &Server{EASM: fake}
	router := chi.NewRouter()
	router.Post("/organizations/{id}/domains/confirm", s.requireWrite(s.handleConfirmDomains))

	body := `{"domains":["guc.edu.cn"]}`
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+id.Hex()+"/domains/confirm", strings.NewReader(body))
	req = withPrincipal(req, domain.RoleViewer)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/organizations/"+id.Hex()+"/domains/confirm", strings.NewReader(body))
	req = withPrincipal(req, domain.RoleOperator)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("operator status = %d: %s", rec.Code, rec.Body.String())
	}
	if len(fake.confirmed) != 1 {
		t.Fatalf("confirm not called: %+v", fake)
	}
}

func TestEASMHealthShape(t *testing.T) {
	s := &Server{EASM: &fakeEASM{health: easm.Health{Chinaz: "ok", ICPQuery: "ok", GeoIP: "unavailable", Neo4j: "ok"}}}
	rec := httptest.NewRecorder()
	s.handleEASMHealth(rec, httptest.NewRequest(http.MethodGet, "/api/v1/easm/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"neo4j":"ok"`) {
		t.Fatalf("unexpected health body: %s", rec.Body.String())
	}
}

func TestEASMConfirmConflictMaps409(t *testing.T) {
	id := bson.NewObjectID()
	s := &Server{EASM: &fakeEASM{confirmErr: domain.ErrDomainAlreadyConfirmed}}
	router := chi.NewRouter()
	router.Post("/organizations/{id}/domains/confirm", s.requireWrite(s.handleConfirmDomains))
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+id.Hex()+"/domains/confirm", strings.NewReader(`{"domains":["guc.edu.cn"]}`))
	req = withPrincipal(req, domain.RoleOperator)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "domain_already_confirmed") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestEASMOrganizationSummaryAndAssetsShape(t *testing.T) {
	orgID := bson.NewObjectID()
	s := &Server{EASM: &fakeEASM{
		summary: &easm.OrganizationSummary{OrganizationID: orgID.Hex(), ConfirmedDomains: 2},
		assets:  &easm.AssetPage{Items: []easm.AssetItem{{ID: bson.NewObjectID().Hex(), Kind: "domain", Key: "example.com"}}, Total: 1, Page: 1, PageSize: 20},
	}}
	for _, tc := range []struct {
		path    string
		handler http.HandlerFunc
		want    string
	}{
		{"/api/v1/easm/organizations/" + orgID.Hex() + "/summary", s.handleEASMOrganizationSummary, `"confirmedDomains":2`},
		{"/api/v1/easm/organizations/" + orgID.Hex() + "/assets?kind=domain", s.handleEASMOrganizationAssets, `"key":"example.com"`},
	} {
		rec := httptest.NewRecorder()
		req := withChiParam(httptest.NewRequest(http.MethodGet, tc.path, nil), "id", orgID.Hex())
		tc.handler(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s status=%d body=%s", tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestEASMFindingsShapeAndHistory(t *testing.T) {
	orgID, findingID := bson.NewObjectID(), bson.NewObjectID()
	s := &Server{EASM: &fakeEASM{
		findings: &easm.FindingPage{Items: []easm.FindingItem{{Finding: domain.Finding{ID: findingID, Title: "漏洞", Target: "example.com"}, Occurrences: 2}}, Total: 1},
		history:  []domain.Finding{{ID: findingID, Title: "漏洞"}},
	}}
	rec := httptest.NewRecorder()
	req := withChiParam(httptest.NewRequest(http.MethodGet, "/api/v1/easm/organizations/"+orgID.Hex()+"/findings", nil), "id", orgID.Hex())
	s.handleEASMOrganizationFindings(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"occurrences":2`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/easm/organizations/"+orgID.Hex()+"/findings/"+findingID.Hex()+"/history", nil)
	req = withChiParam(withChiParam(req, "id", orgID.Hex()), "findingId", findingID.Hex())
	s.handleEASMFindingHistory(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"title":"漏洞"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func withChiParam(r *http.Request, name, value string) *http.Request {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		rctx = chi.NewRouteContext()
	}
	rctx.URLParams.Add(name, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestEASMCreateScansShapeAndValidation(t *testing.T) {
	orgID := bson.NewObjectID()
	s := &Server{EASM: &fakeEASM{tasks: []domain.Task{{ID: bson.NewObjectID(), OrganizationID: orgID.Hex(), RootDomain: "a.edu.cn", Origin: domain.TaskOriginEASM}}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/easm/organizations/"+orgID.Hex()+"/scans", strings.NewReader(`{"rootDomains":["a.edu.cn"],"options":{"portScan":true}}`))
	req = withPrincipal(withChiParam(req, "id", orgID.Hex()), domain.RoleOperator)
	s.handleCreateEASMScans(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"origin":"easm"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/easm/organizations/"+orgID.Hex()+"/scans", strings.NewReader(`{"rootDomains":["candidate.edu.cn"]}`))
	req = withPrincipal(withChiParam(req, "id", orgID.Hex()), domain.RoleOperator)
	s.EASM = &fakeEASM{scanErr: easm.ErrDomainNotConfirmed}
	s.handleCreateEASMScans(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
