package tasking

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/config"
)

type failingRoundTripper struct {
	calls atomic.Int32
}

func (f *failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	f.calls.Add(1)
	return nil, errors.New("scanner client must not be used")
}

func TestStepFofaUsesControlHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("key"); got != "test-key" {
			t.Errorf("FOFA key = %q, want test-key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[["example.com"]]}`))
	}))
	defer server.Close()

	scanner := &failingRoundTripper{}
	engine := Engine{
		Config:        config.Config{Fofa: config.FofaConfig{Key: "test-key", URL: server.URL, PageSize: 1}},
		HTTPClient:    &http.Client{Transport: scanner},
		ControlClient: server.Client(),
	}
	task := &domain.Task{Kind: domain.TaskIP, Target: "1.2.3.4"}

	if err := engine.stepFofa(context.Background(), task, &domain.TaskStep{}); err != nil {
		t.Fatal(err)
	}
	if got := scanner.calls.Load(); got != 0 {
		t.Fatalf("scanner client used %d times, want control client", got)
	}
}
