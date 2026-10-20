package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/webui"
)

func TestOpenAPISpecIsJSON(t *testing.T) {
	var out map[string]any
	if err := json.Unmarshal([]byte(openAPISpec), &out); err != nil {
		t.Fatal(err)
	}
	if out["openapi"] != "3.1.0" {
		t.Fatalf("unexpected spec version: %v", out["openapi"])
	}
}

func TestSafeSegment(t *testing.T) {
	for _, s := range []string{"abc", "a-b_c.png", "task1"} {
		if !safeSegment(s) {
			t.Fatalf("%q should be safe", s)
		}
	}
	for _, s := range []string{"", "..", "../x", "a/b", "a\\b"} {
		if safeSegment(s) {
			t.Fatalf("%q should not be safe", s)
		}
	}
}

func TestSystemInfoIncludesScannerVersions(t *testing.T) {
	s := &Server{
		Config:    config.Config{Queue: config.QueueConfig{Driver: "memory"}},
		Version:   "test",
		StartedAt: time.Now(),
	}
	rec := httptest.NewRecorder()
	s.handleSystemInfo(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/system/info", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Scanners map[string]string `json:"scanners"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Scanners["nuclei"] != "v3.11.1" || body.Scanners["httpx"] != "v1.12.0" {
		t.Fatalf("scanners = %v", body.Scanners)
	}
}

func TestEmbeddedFrontendIncludesBuiltAssets(t *testing.T) {
	index, err := webui.Dist.ReadFile("dist/index.html")
	if err != nil {
		t.Fatalf("read embedded index: %v", err)
	}
	if !strings.Contains(string(index), `<div id="root"></div>`) {
		t.Fatal("embedded index does not contain the React root")
	}
	assetRefs := regexp.MustCompile(`(?:src|href)="(/assets/[^"]+)"`).FindAllStringSubmatch(string(index), -1)
	if len(assetRefs) == 0 {
		t.Fatal("embedded index does not reference built assets")
	}
	for _, match := range assetRefs {
		name := "dist" + match[1]
		info, err := webui.Dist.ReadFile(name)
		if err != nil {
			t.Fatalf("read embedded asset %s: %v", name, err)
		}
		if len(info) == 0 {
			t.Fatalf("embedded asset %s is empty", name)
		}
	}
}

func TestServeSPADoesNotTreatAssetDirectoryAsFile(t *testing.T) {
	if _, err := webui.Dist.ReadFile("dist/index.html"); err != nil {
		t.Fatalf("embedded frontend build is not present: %v", err)
	}
	handler := (&Server{}).serveSPA()

	for _, target := range []string{"/assets", "/assets/"} {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", target, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Fatalf("%s content-type = %q", target, got)
		}
		if !strings.Contains(rec.Body.String(), `<div id="root">`) {
			t.Fatalf("%s did not return the SPA index", target)
		}
	}
}
