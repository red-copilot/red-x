package tasking

import (
	"testing"

	"github.com/red-copilot/red-x/internal/domain"
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
