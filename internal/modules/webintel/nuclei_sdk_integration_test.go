package webintel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestNucleiSDKScannerFindsTemplateMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/actuator/env" || r.URL.Path == "/env" {
			_, _ = w.Write([]byte(`{"propertySources": [], "activeProfiles": []}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	scanner := NewNucleiSDKScanner(
		filepath.Join("..", "..", "..", "assets", "nuclei"),
		"", "", nil, false, 5, 10, 2, 2,
	)
	defer scanner.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	findings, err := scanner.Scan(ctx, []string{srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		t.Fatal("expected nuclei finding")
	}
	if findings[0].TemplateID == "" || findings[0].Name == "" {
		t.Fatalf("finding = %+v", findings[0])
	}
}
