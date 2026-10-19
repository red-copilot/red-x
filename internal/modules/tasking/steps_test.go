package tasking

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/discovery"
)

type recordingDNSResolver struct {
	events     *[]string
	results    map[string]discovery.DomainResult
	bruteCount *int
}

func (r recordingDNSResolver) Resolve(_ context.Context, name string) (discovery.DomainResult, error) {
	*r.events = append(*r.events, "resolve:"+name)
	if result, ok := r.results[name]; ok {
		return result, nil
	}
	return discovery.DomainResult{Domain: name, Type: "NONE"}, nil
}

func (r recordingDNSResolver) WildcardIPs(context.Context, string) []string { return nil }
func (r recordingDNSResolver) Brute(_ context.Context, base string, words []string, _ []string, _ func(int, int)) []discovery.DomainResult {
	if r.bruteCount != nil {
		*r.bruteCount = len(words)
	}
	*r.events = append(*r.events, "brute:"+base)
	return nil
}

type recordingPassiveEnumerator struct {
	events *[]string
	result discovery.PassiveEnumeration
	err    error
}

type concurrencyDNSResolver struct {
	active *atomic.Int32
	max    *atomic.Int32
}

func (r concurrencyDNSResolver) Resolve(_ context.Context, name string) (discovery.DomainResult, error) {
	current := r.active.Add(1)
	for {
		observed := r.max.Load()
		if current <= observed || r.max.CompareAndSwap(observed, current) {
			break
		}
	}
	time.Sleep(10 * time.Millisecond)
	r.active.Add(-1)
	return discovery.DomainResult{Domain: name, Type: "NONE"}, nil
}

func (r concurrencyDNSResolver) WildcardIPs(context.Context, string) []string { return nil }
func (r concurrencyDNSResolver) Brute(context.Context, string, []string, []string, func(int, int)) []discovery.DomainResult {
	return nil
}

func (r recordingPassiveEnumerator) Enumerate(_ context.Context, name string) (discovery.PassiveEnumeration, error) {
	*r.events = append(*r.events, "subfinder:"+name)
	return r.result, r.err
}

func TestStepDNSRunsSubfinderBeforeResolvingRootDomain(t *testing.T) {
	events := []string{}
	engine := Engine{
		DNS:       recordingDNSResolver{events: &events},
		AssetsDir: "../../../assets",
		Passive: recordingPassiveEnumerator{
			events: &events,
			result: discovery.PassiveEnumeration{Domains: []discovery.DomainResult{{Domain: "www.baidu.com", Source: "crtsh"}}},
		},
	}
	task := &domain.Task{Kind: domain.TaskDomain, Target: "baidu.com", Options: domain.TaskOptions{DNSQueryPlugin: true, DomainBrute: true}}

	if err := engine.stepDNS(context.Background(), task, &domain.TaskStep{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"subfinder:baidu.com", "resolve:baidu.com", "resolve:www.baidu.com", "brute:baidu.com"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestStepDNSSkipsSubfinderForHostTarget(t *testing.T) {
	events := []string{}
	engine := Engine{
		DNS:     recordingDNSResolver{events: &events},
		Passive: recordingPassiveEnumerator{events: &events},
	}
	task := &domain.Task{Kind: domain.TaskDomain, Target: "www.baidu.com"}

	if err := engine.stepDNS(context.Background(), task, &domain.TaskStep{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"resolve:www.baidu.com"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestStepDNSRecordsSubfinderFailureAsWarning(t *testing.T) {
	events := []string{}
	engine := Engine{
		DNS:     recordingDNSResolver{events: &events},
		Passive: recordingPassiveEnumerator{events: &events, err: errors.New("source unavailable")},
	}
	task := &domain.Task{Kind: domain.TaskDomain, Target: "baidu.com", Options: domain.TaskOptions{DNSQueryPlugin: true}}

	if err := engine.stepDNS(context.Background(), task, &domain.TaskStep{}); err != nil {
		t.Fatal(err)
	}
	if len(task.Warnings) != 1 {
		t.Fatalf("warnings = %v", task.Warnings)
	}
}

func TestStepDNSCapsPassiveResults(t *testing.T) {
	events := []string{}
	engine := Engine{
		DNS: recordingDNSResolver{events: &events},
		Passive: recordingPassiveEnumerator{
			events: &events,
			result: discovery.PassiveEnumeration{
				Domains: []discovery.DomainResult{
					{Domain: "one.baidu.com", Source: "crtsh"},
					{Domain: "two.baidu.com", Source: "crtsh"},
					{Domain: "three.baidu.com", Source: "crtsh"},
				},
			},
		},
	}
	engine.Config.Scan.MaxHosts = 2
	engine.Config.Scan.Concurrency = 1
	task := &domain.Task{Kind: domain.TaskDomain, Target: "baidu.com", Options: domain.TaskOptions{DNSQueryPlugin: true}}

	if err := engine.stepDNS(context.Background(), task, &domain.TaskStep{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"subfinder:baidu.com", "resolve:baidu.com", "resolve:one.baidu.com", "resolve:two.baidu.com"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	if len(task.Warnings) != 1 {
		t.Fatalf("warnings = %v, want truncation warning", task.Warnings)
	}
}

func TestStepDNSBoundsPassiveResolutionConcurrency(t *testing.T) {
	events := []string{}
	var active atomic.Int32
	var max atomic.Int32
	engine := Engine{
		DNS: concurrencyDNSResolver{active: &active, max: &max},
		Passive: recordingPassiveEnumerator{
			events: &events,
			result: discovery.PassiveEnumeration{
				Domains: []discovery.DomainResult{
					{Domain: "one.baidu.com", Source: "crtsh"},
					{Domain: "two.baidu.com", Source: "crtsh"},
					{Domain: "three.baidu.com", Source: "crtsh"},
					{Domain: "four.baidu.com", Source: "crtsh"},
				},
			},
		},
	}
	engine.Config.Scan.Concurrency = 2
	task := &domain.Task{Kind: domain.TaskDomain, Target: "baidu.com", Options: domain.TaskOptions{DNSQueryPlugin: true}}

	if err := engine.stepDNS(context.Background(), task, &domain.TaskStep{}); err != nil {
		t.Fatal(err)
	}
	if got := max.Load(); got != 2 {
		t.Fatalf("max concurrent DNS resolutions = %d, want 2", got)
	}
}

func TestStepDNSReportsSubfinderTruncation(t *testing.T) {
	events := []string{}
	engine := Engine{
		DNS: recordingDNSResolver{events: &events},
		Passive: recordingPassiveEnumerator{
			events: &events,
			result: discovery.PassiveEnumeration{
				Domains:   []discovery.DomainResult{{Domain: "one.baidu.com", Source: "crtsh"}},
				Truncated: true,
			},
		},
	}
	engine.Config.Scan.MaxHosts = 1
	task := &domain.Task{Kind: domain.TaskDomain, Target: "baidu.com", Options: domain.TaskOptions{DNSQueryPlugin: true}}

	if err := engine.stepDNS(context.Background(), task, &domain.TaskStep{}); err != nil {
		t.Fatal(err)
	}
	if len(task.Warnings) != 1 {
		t.Fatalf("warnings = %v, want truncation warning", task.Warnings)
	}
}

func TestStepDNSCapsDomainBruteDictionary(t *testing.T) {
	events := []string{}
	words := -1
	engine := Engine{
		DNS:       recordingDNSResolver{events: &events, bruteCount: &words},
		AssetsDir: "../../../assets",
	}
	engine.Config.Scan.MaxHosts = 2
	task := &domain.Task{
		Kind: domain.TaskDomain, Target: "edu.cn",
		Options: domain.TaskOptions{DomainBrute: true, DomainBruteType: "big"},
	}

	if err := engine.stepDNS(context.Background(), task, &domain.TaskStep{}); err != nil {
		t.Fatal(err)
	}
	if words != 2 {
		t.Fatalf("brute dictionary size = %d, want 2", words)
	}
	if len(task.Warnings) != 1 {
		t.Fatalf("warnings = %v, want dictionary truncation warning", task.Warnings)
	}
}
