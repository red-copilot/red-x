package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/platform/config"
)

func TestUsageOnlyDocumentsSingleNodeServer(t *testing.T) {
	text := usageText()
	if !strings.Contains(text, "arl all") {
		t.Fatal("usage does not document the single-node server command")
	}
	for _, role := range []string{"arl web", "arl worker", "arl scheduler", "arl dev"} {
		if strings.Contains(text, role) {
			t.Fatalf("usage still documents removed role %q", role)
		}
	}
}

func TestNewHTTPServerUsesStreamingTimeouts(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Listen = "127.0.0.1:5003"
	handler := http.NewServeMux()

	srv := newHTTPServer(cfg, handler)

	if srv.Addr != cfg.Server.Listen {
		t.Fatalf("address = %q, want %q", srv.Addr, cfg.Server.Listen)
	}
	if srv.Handler != handler {
		t.Fatal("handler was not retained")
	}
	if srv.ReadHeaderTimeout != 10*time.Second {
		t.Fatalf("read header timeout = %s", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 30*time.Second {
		t.Fatalf("read timeout = %s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 0 {
		t.Fatalf("write timeout = %s, want disabled for SSE", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 90*time.Second {
		t.Fatalf("idle timeout = %s", srv.IdleTimeout)
	}
}

func TestRunComponentsReturnsWorkerFailureAndStopsServer(t *testing.T) {
	want := errors.New("worker failed")
	serverStarted := make(chan struct{})
	serverStopped := make(chan struct{})

	err := runComponents(context.Background(),
		func(context.Context) error {
			<-serverStarted
			return want
		},
		func(ctx context.Context) error {
			close(serverStarted)
			<-ctx.Done()
			close(serverStopped)
			return ctx.Err()
		},
	)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	select {
	case <-serverStopped:
	case <-time.After(time.Second):
		t.Fatal("server component was not cancelled after worker failure")
	}
}

func TestRunComponentsReturnsServerFailureAndStopsWorker(t *testing.T) {
	want := errors.New("server failed")
	workerStarted := make(chan struct{})
	workerStopped := make(chan struct{})

	err := runComponents(context.Background(),
		func(ctx context.Context) error {
			close(workerStarted)
			<-ctx.Done()
			close(workerStopped)
			return ctx.Err()
		},
		func(context.Context) error {
			<-workerStarted
			return want
		},
	)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	select {
	case <-workerStopped:
	case <-time.After(time.Second):
		t.Fatal("worker component was not cancelled after server failure")
	}
}

func TestRunComponentsTreatsCancellationAsCleanShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runComponents(ctx,
		func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
		func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	)
	if err != nil {
		t.Fatalf("cancellation returned %v, want nil", err)
	}
}
