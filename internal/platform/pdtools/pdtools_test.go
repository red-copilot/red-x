package pdtools

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/red-copilot/red-x/internal/platform/netsafe"
)

func TestNormalizeResolvers(t *testing.T) {
	got := NormalizeResolvers([]string{"1.1.1.1", "8.8.8.8:53", "udp:9.9.9.9:53", "tcp:1.0.0.1"})
	want := []string{"udp:1.1.1.1:53", "udp:8.8.8.8:53", "udp:9.9.9.9:53", "tcp:1.0.0.1:53"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeResolvers returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeResolvers[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNetworkPolicyUsesARLPolicy(t *testing.T) {
	policy, err := netsafe.NewPolicy([]string{"203.0.113.0/24"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	np, err := NewNetworkPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"127.0.0.1", "169.254.169.254", "10.1.2.3", "203.0.113.9"} {
		if np.Validate(target) {
			t.Errorf("network policy allowed blocked target %s", target)
		}
	}
	if !np.Validate("198.51.100.10") {
		t.Error("network policy blocked a public target")
	}
}

func TestSafeHTTPClientRevalidatesRedirects(t *testing.T) {
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer blocked.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, blocked.URL, http.StatusFound)
	}))
	defer redirector.Close()

	policy := netsafe.Policy{AllowLoopback: true}
	client := NewSafeHTTPClient(policy, "", 2)
	resp, err := client.Get(redirector.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("redirect status = %d, want 200", resp.StatusCode)
	}

	policy = netsafe.Policy{}
	client = NewSafeHTTPClient(policy, "", 2)
	if _, err := client.Get(redirector.URL); err == nil {
		t.Fatal("expected loopback request to be blocked")
	}
}

func TestVerifiedHTTPClientRejectsSelfSignedCertificate(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()

	client := NewVerifiedHTTPClient(netsafe.Policy{AllowLoopback: true}, "", 2)
	_, err := client.Get(server.URL)
	if err == nil {
		t.Fatal("expected self-signed certificate to be rejected")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "certificate") {
		t.Fatalf("expected certificate verification error, got %v", err)
	}
}

func TestVersionsArePinned(t *testing.T) {
	versions := Versions()
	for _, name := range []string{"subfinder", "dnsx", "naabu", "tlsx", "httpx", "katana", "nuclei"} {
		if versions[name] == "" {
			t.Errorf("missing version for %s", name)
		}
	}
	if versions["nuclei"] != "v3.11.1" {
		t.Fatalf("unexpected nuclei version %q", versions["nuclei"])
	}
}

func TestPolicyDialContextBlocksPrivateAddress(t *testing.T) {
	policy := netsafe.Policy{}
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		return policy.DialContext(ctx, network, address)
	}
	if _, err := dial(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("expected blocked dial to fail")
	}
}

func TestCountTemplates(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("id: a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.yml"), []byte("id: b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("ignore"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := CountTemplates(dir); got != 2 {
		t.Fatalf("CountTemplates = %d, want 2", got)
	}
}

func TestEnsureNucleiIgnore(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	if err := EnsureNucleiIgnore(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(configHome, "nuclei", ".nuclei-ignore")); err != nil {
		t.Fatal(err)
	}
}

func TestBlockedTargetsHonorsAllowedLoopbackAndPrivate(t *testing.T) {
	policy, err := netsafe.NewPolicy([]string{"127.0.0.0/8", "::1/128", "10.0.0.0/8"}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	blocked := BlockedTargets(policy)
	for _, wantAllowed := range []string{"127.0.0.0/8", "::1/128", "10.0.0.0/8"} {
		for _, target := range blocked {
			if target == wantAllowed {
				t.Errorf("BlockedTargets kept %s despite allowLoopback/allowPrivate", wantAllowed)
			}
		}
	}
}
