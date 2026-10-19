package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Aabyss-Team/ARL/internal/platform/config"
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
		Data struct {
			Scanners map[string]string `json:"scanners"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Scanners["nuclei"] != "v3.11.1" || body.Data.Scanners["httpx"] != "v1.12.0" {
		t.Fatalf("scanners = %v", body.Data.Scanners)
	}
}
