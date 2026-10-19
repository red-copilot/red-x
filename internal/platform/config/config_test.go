package config

import (
	"os"
	"path/filepath"
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
