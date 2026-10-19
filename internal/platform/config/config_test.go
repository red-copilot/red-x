package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScannersDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Scanners.Subfinder.Threads != 10 {
		t.Fatalf("subfinder threads = %d, want 10", cfg.Scanners.Subfinder.Threads)
	}
	if got := cfg.Scanners.Subfinder.Sources; len(got) != 3 || got[0] != "crtsh" {
		t.Fatalf("subfinder sources = %v", got)
	}
	if cfg.Scanners.Naabu.ScanType != "connect" {
		t.Fatalf("naabu scan type = %q, want connect", cfg.Scanners.Naabu.ScanType)
	}
	if cfg.Scanners.HTTPX.MaxBodyRead != 2<<20 {
		t.Fatalf("httpx max body = %d, want %d", cfg.Scanners.HTTPX.MaxBodyRead, 2<<20)
	}
	if len(cfg.Scanners.Nuclei.Protocols) != 1 || cfg.Scanners.Nuclei.Protocols[0] != "http" {
		t.Fatalf("nuclei protocols = %v", cfg.Scanners.Nuclei.Protocols)
	}
}

func TestScannersYAMLOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`
auth:
  accessSecret: test-secret
scanners:
  subfinder:
    sources: [crtsh]
    threads: 2
  nuclei:
    templatesDir: /tmp/templates
    rateLimit: 5
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Scanners.Subfinder.Sources) != 1 || cfg.Scanners.Subfinder.Sources[0] != "crtsh" {
		t.Fatalf("sources = %v", cfg.Scanners.Subfinder.Sources)
	}
	if cfg.Scanners.Subfinder.Threads != 2 {
		t.Fatalf("threads = %d", cfg.Scanners.Subfinder.Threads)
	}
	if cfg.Scanners.Nuclei.TemplatesDir != "/tmp/templates" || cfg.Scanners.Nuclei.RateLimit != 5 {
		t.Fatalf("nuclei config = %+v", cfg.Scanners.Nuclei)
	}
	if cfg.Scanners.Subfinder.Timeout != 30 {
		t.Fatalf("untouched default timeout = %d, want 30", cfg.Scanners.Subfinder.Timeout)
	}
}

func TestQueueConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		redis   string
		wantErr string
	}{
		{name: "memory", driver: "memory"},
		{name: "redis", driver: "redis", redis: "redis://127.0.0.1:6379/0"},
		{name: "redis missing url", driver: "redis", wantErr: "redis.url is required"},
		{name: "rabbitmq removed", driver: "rabbitmq", wantErr: "queue.driver must be memory or redis"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Default()
			cfg.Auth.AccessSecret = "test-secret"
			cfg.Queue.Driver = tc.driver
			cfg.Redis.URL = tc.redis
			err := cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestQueueEnvironmentOverrides(t *testing.T) {
	t.Setenv("ARL_QUEUE_DRIVER", "redis")
	t.Setenv("ARL_REDIS_URL", "redis://example.test:6380/2")
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`
auth:
  accessSecret: test-secret
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Queue.Driver != "redis" {
		t.Fatalf("queue driver = %q, want redis", cfg.Queue.Driver)
	}
	if cfg.Redis.URL != "redis://example.test:6380/2" {
		t.Fatalf("redis url = %q", cfg.Redis.URL)
	}
}
