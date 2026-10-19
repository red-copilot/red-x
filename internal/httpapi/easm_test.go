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
	lookup    *easm.LookupResult
	lookupErr error
	mapSnap   *easm.MapSnapshot
	graphSnap *easm.GraphSnapshot
	graphErr  error
	job       *domain.ICPJob
	confirmN  int
	confirmed []string
	health    easm.Health
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
	return f.confirmN, nil
}
func (f *fakeEASM) Map(context.Context, *bson.ObjectID, string) (*easm.MapSnapshot, error) {
	return f.mapSnap, nil
}
func (f *fakeEASM) Graph(context.Context, bson.ObjectID, int, int) (*easm.GraphSnapshot, error) {
	return f.graphSnap, f.graphErr
}
func (f *fakeEASM) Health(context.Context) easm.Health { return f.health }

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
	var body struct {
		Data easm.MapSnapshot `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Organizations) != 1 || len(body.Data.Points) != 1 {
		t.Fatalf("unexpected map: %+v", body.Data)
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
