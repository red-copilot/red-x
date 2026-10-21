package tasking

import (
	"context"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/discovery"
	"github.com/red-copilot/red-x/internal/platform/config"
)

func TestMergePolicy(t *testing.T) {
	opts := domain.TaskOptions{}
	cfg := domain.PolicyConfig{
		Domain: domain.DomainPolicy{DomainBrute: true, DomainBruteType: "big"},
		IP:     domain.IPPolicy{PortScan: true, PortScanType: "top100", ServiceDetection: true},
		Site:   domain.SitePolicy{SiteIdentify: true, FileLeak: true},
		POC:    []domain.PluginToggle{{PluginName: "Redis_noauth", Enable: true}},
	}
	out := MergePolicy(opts, cfg)
	if !out.DomainBrute || out.DomainBruteType != "big" || !out.PortScan || out.PortScanType != "top100" || !out.ServiceDetect || !out.FileLeak {
		t.Fatalf("policy was not merged: %+v", out)
	}
	if len(out.POCPlugins) != 1 || out.POCPlugins[0] != "Redis_noauth" {
		t.Fatalf("POC plugins not merged: %+v", out.POCPlugins)
	}
}

func TestStepPlan(t *testing.T) {
	task := &domain.Task{Kind: domain.TaskDomain, Options: domain.TaskOptions{PortScan: true, POCPlugins: []string{"Redis_noauth"}}}
	steps := StepPlan(task)
	kinds := make([]string, 0, len(steps))
	for _, s := range steps {
		kinds = append(kinds, s.Kind)
	}
	want := []string{"dns", "port", "http", "webintel", "poc", "stats"}
	if len(kinds) != len(want) {
		t.Fatalf("got %v", kinds)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("got %v", kinds)
		}
	}
}

func TestDeriveTaskOptionsSetsSubfinderOnlyForRootDomains(t *testing.T) {
	base := domain.TaskOptions{DomainBrute: true, DomainBruteType: "big", DNSQueryPlugin: true}

	root, err := deriveTaskOptions(base, domain.TaskDomain, discovery.Target{Host: "baidu.com", Kind: "domain"})
	if err != nil {
		t.Fatal(err)
	}
	if !root.DNSQueryPlugin || !root.DomainBrute || root.DomainBruteType != "big" {
		t.Fatalf("root options = %+v", root)
	}

	subdomain, err := deriveTaskOptions(base, domain.TaskDomain, discovery.Target{Host: "www.baidu.com", Kind: "domain"})
	if err != nil {
		t.Fatal(err)
	}
	if subdomain.DNSQueryPlugin || !subdomain.DomainBrute || subdomain.DomainBruteType != "big" {
		t.Fatalf("subdomain options = %+v", subdomain)
	}

	ip, err := deriveTaskOptions(base, domain.TaskIP, discovery.Target{Host: "192.0.2.10", Kind: "ip"})
	if err != nil {
		t.Fatal(err)
	}
	if !ip.DNSQueryPlugin {
		t.Fatalf("non-domain options should be unchanged: %+v", ip)
	}

	fuzz, err := deriveTaskOptions(base, domain.TaskDomain, discovery.Target{Host: "{fuzz}.example.com", Kind: "domain"})
	if err != nil {
		t.Fatalf("fuzz target should remain valid: %v", err)
	}
	if fuzz.DNSQueryPlugin {
		t.Fatalf("fuzz target should not run subfinder: %+v", fuzz)
	}
}

func TestPrepareTargetAcceptsPublicSuffixRootsWithoutDNSRecords(t *testing.T) {
	engine := Engine{}
	base := domain.TaskOptions{}
	for _, input := range []string{"guc.edu.cn", "edu.cn", "com.cn", "co.uk"} {
		t.Run(input, func(t *testing.T) {
			target := discovery.Target{Raw: input, Host: input, Kind: "domain"}
			options, err := engine.prepareTarget(context.Background(), base, domain.TaskDomain, target)
			if err != nil {
				t.Fatalf("prepareTarget(%q): %v", input, err)
			}
			if !options.DNSQueryPlugin {
				t.Fatalf("prepareTarget(%q) did not enable subfinder", input)
			}
		})
	}
}

func TestDefaultDomainOptionsStayPassiveAndEnablePortScan(t *testing.T) {
	opts := DefaultDomainOptions()
	if !opts.DNSQueryPlugin || !opts.ARLSearch || !opts.PortScan || !opts.SiteIdentify {
		t.Fatalf("expected passive discovery defaults: %+v", opts)
	}
	if opts.PortScanType != "top1000" || !opts.SkipScanCDNIP {
		t.Fatalf("unexpected port defaults: %+v", opts)
	}
	if opts.DomainBrute || opts.AltDNS || opts.ServiceDetect || opts.OSDetect || opts.SSLCert ||
		opts.SiteCapture || opts.SearchEngines || opts.SiteSpider || opts.FileLeak || opts.FindVhost ||
		opts.NucleiScan || opts.WebInfoHunter || len(opts.POCPlugins) > 0 || len(opts.BrutePlugins) > 0 {
		t.Fatalf("active options must stay disabled by default: %+v", opts)
	}
}

func TestWithSafeDefaultsOnlyAppliesToEmptyDomainOptions(t *testing.T) {
	got := withSafeDefaults(domain.TaskDomain, domain.TaskOptions{})
	if !reflect.DeepEqual(got, DefaultDomainOptions()) {
		t.Fatalf("empty domain options = %+v, want defaults", got)
	}
	explicit := domain.TaskOptions{PortScan: true}
	if got := withSafeDefaults(domain.TaskDomain, explicit); !reflect.DeepEqual(got, explicit) {
		t.Fatalf("explicit options changed: %+v", got)
	}
	risk := domain.TaskOptions{}
	if got := withSafeDefaults(domain.TaskRiskCruising, risk); !reflect.DeepEqual(got, risk) {
		t.Fatalf("risk cruising should not inherit domain defaults: %+v", got)
	}
}

func TestScopeAllowsTarget(t *testing.T) {
	tests := []struct {
		name   string
		scope  *domain.Scope
		target discovery.Target
		want   bool
	}{
		{
			name:   "domain exact match",
			scope:  &domain.Scope{Type: "domain", Targets: []string{"example.com"}},
			target: discovery.Target{Kind: "domain", Host: "example.com"},
			want:   true,
		},
		{
			name:   "domain subdomain match",
			scope:  &domain.Scope{Type: "domain", Targets: []string{"example.com"}},
			target: discovery.Target{Kind: "domain", Host: "www.example.com"},
			want:   true,
		},
		{
			name:   "domain suffix spoof rejected",
			scope:  &domain.Scope{Type: "domain", Targets: []string{"example.com"}},
			target: discovery.Target{Kind: "domain", Host: "example.com.evil"},
			want:   false,
		},
		{
			name:   "black scope overrides domain match",
			scope:  &domain.Scope{Type: "domain", Targets: []string{"example.com"}, BlackScope: []string{"blocked.example.com"}},
			target: discovery.Target{Kind: "domain", Host: "api.blocked.example.com"},
			want:   false,
		},
		{
			name:   "url host uses domain scope",
			scope:  &domain.Scope{Type: "domain", Targets: []string{"example.com"}},
			target: discovery.Target{Kind: "url", Host: "api.example.com"},
			want:   true,
		},
		{
			name:   "ip inside cidr",
			scope:  &domain.Scope{Type: "ip", Targets: []string{"10.0.0.0/8"}},
			target: discovery.Target{Kind: "ip", Host: "10.2.3.4"},
			want:   true,
		},
		{
			name:   "ip in black cidr rejected",
			scope:  &domain.Scope{Type: "ip", Targets: []string{"10.0.0.0/8"}, BlackScope: []string{"10.1.0.0/16"}},
			target: discovery.Target{Kind: "ip", Host: "10.1.2.3"},
			want:   false,
		},
		{
			name:   "cidr contained by scope",
			scope:  &domain.Scope{Type: "ip", Targets: []string{"10.0.0.0/8"}},
			target: discovery.Target{Kind: "cidr", Prefix: mustPrefix(t, "10.2.0.0/16")},
			want:   true,
		},
		{
			name:   "cidr crossing scope boundary rejected",
			scope:  &domain.Scope{Type: "ip", Targets: []string{"10.0.0.0/8"}},
			target: discovery.Target{Kind: "cidr", Prefix: mustPrefix(t, "10.0.0.0/7")},
			want:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := scopeAllowsTarget(tc.scope, tc.target); got != tc.want {
				t.Fatalf("scopeAllowsTarget() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDomainForbiddenMatchesExactAndSubdomain(t *testing.T) {
	tests := []struct {
		host    string
		blocked []string
		want    bool
	}{
		{host: "example.com", blocked: []string{"example.com"}, want: true},
		{host: "api.example.com", blocked: []string{"EXAMPLE.COM."}, want: true},
		{host: "example.com.evil", blocked: []string{"example.com"}, want: false},
		{host: "other.test", blocked: []string{"example.com"}, want: false},
	}
	for _, tc := range tests {
		if got := domainForbidden(tc.host, tc.blocked); got != tc.want {
			t.Errorf("domainForbidden(%q, %v) = %v, want %v", tc.host, tc.blocked, got, tc.want)
		}
	}
}

func TestValidateTargetRejectsForbiddenDomain(t *testing.T) {
	engine := Engine{Config: config.Default()}
	engine.Config.Scan.ForbiddenDomains = []string{"example.com"}
	err := engine.validateTarget(context.Background(), discovery.Target{
		Raw: "api.example.com", Host: "api.example.com", Kind: "domain",
	}, domain.TaskDomain, true)
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("validateTarget error = %v, want forbidden-domain error", err)
	}
}

func mustPrefix(t *testing.T, raw string) netip.Prefix {
	t.Helper()
	prefix, err := netip.ParsePrefix(raw)
	if err != nil {
		t.Fatal(err)
	}
	return prefix
}
