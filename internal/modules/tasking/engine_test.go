package tasking

import (
	"context"
	"reflect"
	"testing"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/discovery"
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
