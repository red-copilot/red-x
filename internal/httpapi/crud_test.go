package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/store"
)

func TestOrganizationDomainAndAssetCRUDHandlers(t *testing.T) {
	st, ctx := newHTTPTestStore(t)
	s := &Server{Store: st}
	router := chi.NewRouter()
	router.Delete("/organizations/{id}", s.requireWrite(s.handleDeleteOrganization))
	router.Patch("/organizations/{id}/domains/{domainId}", s.requireWrite(s.handleUpdateOrganizationDomain))
	router.Delete("/organizations/{id}/domains/{domainId}", s.requireWrite(s.handleDeleteOrganizationDomain))
	router.Post("/assets", s.requireWrite(s.handleCreateAsset))
	router.Patch("/assets/{id}", s.requireWrite(s.handleUpdateAsset))
	router.Delete("/assets/{id}", s.requireWrite(s.handleDeleteAsset))

	org := &domain.Organization{CanonicalName: "HTTP CRUD Organization"}
	if err := st.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	orgDomain := &domain.OrganizationDomain{
		OrganizationID: org.ID, RootDomain: "http-crud.example.com", Source: "manual",
		Confidence: domain.ConfidenceHigh, Status: domain.DomainStatusCandidate,
	}
	if err := st.UpsertOrganizationDomain(ctx, orgDomain); err != nil {
		t.Fatal(err)
	}
	patch := requestWithPrincipal(t, http.MethodPatch, "/organizations/"+org.ID+"/domains/"+orgDomain.ID, `{"siteName":"HTTP Site","icpNo":"ICP-42"}`, router)
	if patch.Code != http.StatusOK || !containsJSONValue(patch.Body.String(), `"siteName":"HTTP Site"`) {
		t.Fatalf("domain patch status=%d body=%s", patch.Code, patch.Body.String())
	}
	domainDelete := requestWithPrincipal(t, http.MethodDelete, "/organizations/"+org.ID+"/domains/"+orgDomain.ID, "", router)
	if domainDelete.Code != http.StatusOK {
		t.Fatalf("domain delete status=%d body=%s", domainDelete.Code, domainDelete.Body.String())
	}
	if item, err := st.GetOrganizationDomainByID(ctx, orgDomain.ID); err != nil || item != nil {
		t.Fatalf("deleted domain item=%#v err=%v", item, err)
	}

	create := requestWithPrincipal(t, http.MethodPost, "/assets", `{
		"kind":"domain",
		"key":"manual-http.example.com",
		"displayName":"Manual HTTP",
		"displayUrl":"https://manual-http.example.com",
		"notes":"created through API",
		"tags":["manual","critical"]
	}`, router)
	if create.Code != http.StatusCreated {
		t.Fatalf("asset create status=%d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	update := requestWithPrincipal(t, http.MethodPatch, "/assets/"+created.ID, `{"displayName":"Renamed HTTP","tags":["manual"]}`, router)
	if update.Code != http.StatusOK || !containsJSONValue(update.Body.String(), `"displayName":"Renamed HTTP"`) {
		t.Fatalf("asset patch status=%d body=%s", update.Code, update.Body.String())
	}
	assetDelete := requestWithPrincipal(t, http.MethodDelete, "/assets/"+created.ID, "", router)
	if assetDelete.Code != http.StatusOK {
		t.Fatalf("asset delete status=%d body=%s", assetDelete.Code, assetDelete.Body.String())
	}
	if item, err := st.GetAssetByID(ctx, created.ID); err != nil || item != nil {
		t.Fatalf("deleted asset item=%#v err=%v", item, err)
	}

	orgDelete := requestWithPrincipal(t, http.MethodDelete, "/organizations/"+org.ID, "", router)
	if orgDelete.Code != http.StatusOK {
		t.Fatalf("organization delete status=%d body=%s", orgDelete.Code, orgDelete.Body.String())
	}
	audit, total, err := st.ListAudit(ctx, store.PageQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if total < 6 {
		t.Fatalf("audit events = %d, want at least 6: %#v", total, audit)
	}
}

func requestWithPrincipal(t *testing.T, method, target, body string, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req = withPrincipal(req, domain.RoleOperator)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
