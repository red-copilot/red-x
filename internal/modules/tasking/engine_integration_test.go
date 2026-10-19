package tasking

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"github.com/red-copilot/red-x/internal/platform/logx"
	"github.com/red-copilot/red-x/internal/store"
)

func TestSubmitPublicSuffixRootsEnableSubfinder(t *testing.T) {
	uri := os.Getenv("ARL_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set ARL_TEST_MONGO_URI to run MongoDB integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, err := store.Open(ctx, uri, fmt.Sprintf("arl_tasking_test_%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = st.DB().Drop(context.Background())
		_ = st.Close(context.Background())
	}()
	if err := st.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	queue := jobs.NewMemory(logx.New("error", false))
	defer queue.Close()
	cfg := config.Default()
	cfg.Scan.MaxHosts = 10
	engine := &Engine{Store: st, Queue: queue, Config: cfg, Log: logx.New("error", false)}
	created, err := engine.Submit(ctx, SubmitInput{
		Name: "mixed domain targets", Kind: "domain",
		Target: "guc.edu.cn,edu.cn,com.cn,co.uk,www.baidu.com,{fuzz}.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 6 {
		t.Fatalf("created %d tasks, want 6", len(created))
	}
	wantSubfinder := map[string]bool{
		"guc.edu.cn":         true,
		"edu.cn":             true,
		"com.cn":             true,
		"co.uk":              true,
		"www.baidu.com":      false,
		"{fuzz}.example.com": false,
	}
	for _, task := range created {
		want, ok := wantSubfinder[task.Target]
		if !ok {
			t.Fatalf("unexpected task target %q", task.Target)
		}
		if task.Options.DNSQueryPlugin != want {
			t.Fatalf("task %q DNSQueryPlugin = %v, want %v", task.Target, task.Options.DNSQueryPlugin, want)
		}
	}

	retried, err := engine.Retry(ctx, created[0].ID, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(retried) != 1 || !retried[0].Options.DNSQueryPlugin {
		t.Fatalf("retry did not preserve root classification: %+v", retried)
	}

	events := []string{}
	engine.DNS = recordingDNSResolver{events: &events}
	engine.Passive = recordingPassiveEnumerator{events: &events, err: fmt.Errorf("source unavailable")}
	if err := engine.stepDNS(ctx, &created[0], &domain.TaskStep{}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := st.GetTask(ctx, created[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Warnings) == 0 {
		t.Fatal("subfinder failure was not persisted on the task")
	}
	recorded, err := st.ListTaskEvents(ctx, created[0].ID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	foundWarning := false
	for _, event := range recorded {
		if event.Type == "task.warning" && event.Payload["kind"] == "subfinder" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("task.warning event not persisted: %+v", recorded)
	}
}
