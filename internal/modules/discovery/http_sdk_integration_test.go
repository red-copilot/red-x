package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Aabyss-Team/ARL/internal/platform/netsafe"
)

func TestHTTPXProbeReadsBodyAndTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "test-server")
		_, _ = w.Write([]byte(`<html><head><title>ARL Test Site</title></head><body>ok</body></html>`))
	}))
	defer srv.Close()

	probe := NewHTTPXProbe(netsafe.Policy{AllowLoopback: true}, "", 2, 3, 0, 2, 1<<20)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	sites, errs := probe.Probe(ctx, []string{srv.URL})
	if len(errs) != 0 {
		t.Fatalf("probe errors: %v", errs)
	}
	if len(sites) != 1 {
		t.Fatalf("sites = %+v", sites)
	}
	if sites[0].Title != "ARL Test Site" || sites[0].Body == "" || sites[0].HTTPServer != "test-server" {
		t.Fatalf("site = %+v", sites[0])
	}
}
