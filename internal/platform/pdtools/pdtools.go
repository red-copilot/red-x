package pdtools

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/platform/netsafe"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/gologger/formatter"
	"github.com/projectdiscovery/gologger/levels"
	"github.com/projectdiscovery/gologger/writer"
	"github.com/projectdiscovery/networkpolicy"
)

const (
	SubfinderVersion = "v2.17.0"
	DNSXVersion      = "v1.3.1"
	NaabuVersion     = "v2.6.1"
	TLSXVersion      = "v1.4.1"
	HTTPXVersion     = "v1.12.0"
	KatanaVersion    = "v1.8.0"
	NucleiVersion    = "v3.11.1"
)

func Versions() map[string]string {
	return map[string]string{
		"subfinder": SubfinderVersion,
		"dnsx":      DNSXVersion,
		"naabu":     NaabuVersion,
		"tlsx":      TLSXVersion,
		"httpx":     HTTPXVersion,
		"katana":    KatanaVersion,
		"nuclei":    NucleiVersion,
	}
}

func NormalizeResolvers(in []string) []string {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		scheme := "udp"
		address := raw
		if before, after, ok := strings.Cut(raw, ":"); ok {
			switch strings.ToLower(before) {
			case "udp", "tcp", "doh", "dot":
				scheme, address = strings.ToLower(before), after
			}
		}
		if _, _, err := net.SplitHostPort(address); err != nil {
			address = net.JoinHostPort(strings.Trim(address, "[]"), "53")
		}
		out = append(out, scheme+":"+address)
	}
	return out
}

func BlockedTargets(policy netsafe.Policy) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(target string) {
		if target != "" && !seen[target] {
			seen[target] = true
			out = append(out, target)
		}
	}
	for _, prefix := range policy.BlockedPrefixes {
		add(prefix.String())
	}
	if !policy.AllowLoopback {
		add("127.0.0.0/8")
		add("::1/128")
	}
	if !policy.AllowPrivate {
		for _, prefix := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "100.64.0.0/10", "fc00::/7"} {
			add(prefix)
		}
	}
	for _, prefix := range []string{
		"0.0.0.0/8",
		"169.254.0.0/16",
		"fe80::/10",
		"224.0.0.0/4",
		"ff00::/8",
		"255.255.255.255/32",
	} {
		add(prefix)
	}
	return out
}

func NewNetworkPolicy(policy netsafe.Policy) (*networkpolicy.NetworkPolicy, error) {
	np, err := networkpolicy.New(networkpolicy.Options{DenyList: BlockedTargets(policy)})
	if err != nil {
		return nil, fmt.Errorf("build projectdiscovery network policy: %w", err)
	}
	return np, nil
}

func NewSafeHTTPClient(policy netsafe.Policy, proxyURL string, timeoutSeconds int) *http.Client {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 12
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           policy.DialContext,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, // scanner inspects arbitrary endpoints
		MaxIdleConns:          128,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: time.Duration(timeoutSeconds) * time.Second,
	}
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeoutSeconds) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
			defer cancel()
			return policy.ValidateHost(ctx, req.URL.Hostname())
		},
	}
}

func ConfigureLogging(log *slog.Logger, debug bool) {
	if log == nil {
		return
	}
	gologger.DefaultLogger.SetFormatter(&formatter.JSON{})
	gologger.DefaultLogger.SetWriter(&slogWriter{log: log})
	if debug {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelDebug)
	} else {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelInfo)
	}
}

type slogWriter struct {
	log *slog.Logger
}

var _ writer.Writer = (*slogWriter)(nil)

func (w *slogWriter) Write(data []byte, level levels.Level) {
	message := strings.TrimSpace(string(data))
	if message == "" {
		return
	}
	switch level {
	case levels.LevelFatal, levels.LevelError:
		w.log.Error(message, "source", "projectdiscovery")
	case levels.LevelWarning:
		w.log.Warn(message, "source", "projectdiscovery")
	case levels.LevelDebug, levels.LevelVerbose:
		w.log.Debug(message, "source", "projectdiscovery")
	default:
		w.log.Info(message, "source", "projectdiscovery")
	}
}

func ProviderConfigPath(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = "./data"
	}
	return filepath.Join(dataDir, "subfinder-provider.yaml")
}

func EnsureProviderConfig(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	_, err = f.WriteString("{}\n")
	return err
}

// EnsureNucleiIgnore creates the default nuclei ignore file so the SDK does
// not emit a noisy error while initializing an embedded engine.
func EnsureNucleiIgnore() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(configDir, "nuclei")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	path := filepath.Join(dir, ".nuclei-ignore")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte("tags: []\nfiles: []\n"), 0o600)
}

func CountTemplates(dir string) int {
	count := 0
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".yaml", ".yml":
			count++
		}
		return nil
	})
	return count
}
