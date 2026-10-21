package tasking

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/discovery"
	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"github.com/red-copilot/red-x/internal/platform/logx"
	"github.com/red-copilot/red-x/internal/store"
)

type countingQueue struct {
	mu        sync.Mutex
	publishes int
}

func (q *countingQueue) Publish(context.Context, string, []byte) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.publishes++
	return nil
}

func (q *countingQueue) Consume(ctx context.Context, _ []string, _ int, _ jobs.Handler) error {
	<-ctx.Done()
	return ctx.Err()
}

func (*countingQueue) Health(context.Context) error { return nil }
func (*countingQueue) Close() error                 { return nil }

func (q *countingQueue) publishCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.publishes
}

type blockingProbe struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

type failingResolver struct{}

func (failingResolver) Resolve(context.Context, string) (discovery.DomainResult, error) {
	return discovery.DomainResult{}, fmt.Errorf("resolve failed")
}

func (failingResolver) WildcardIPs(context.Context, string) []string { return nil }

func (failingResolver) Brute(context.Context, string, []string, []string, func(int, int)) []discovery.DomainResult {
	return nil
}

func (p *blockingProbe) Probe(context.Context, []string) ([]discovery.SiteResult, []error) {
	p.calls.Add(1)
	p.started <- struct{}{}
	<-p.release
	return nil, nil
}

func TestSubmitPublicSuffixRootsEnableSubfinder(t *testing.T) {
	st, ctx := newTaskingPostgresStore(t)
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

func TestSubmitCreatesExecutablePlanWithoutDirectPublish(t *testing.T) {
	st, ctx := newTaskingPostgresStore(t)
	queue := &countingQueue{}
	engine := &Engine{Store: st, Queue: queue, Config: config.Default(), Log: logx.New("error", false)}
	created, err := engine.Submit(ctx, SubmitInput{Name: "atomic plan", Kind: domain.TaskDomain, Target: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Fatalf("created = %+v", created)
	}
	if got := queue.publishCount(); got != 0 {
		t.Fatalf("Submit published %d jobs directly; outbox relay must be the sole publisher", got)
	}
	steps, err := st.ListSteps(ctx, created[0].ID)
	if err != nil || len(steps) == 0 || steps[0].Status != domain.StepQueued {
		t.Fatalf("steps=%+v err=%v", steps, err)
	}
	outbox, err := st.PendingOutbox(ctx, 10)
	if err != nil || len(outbox) != 1 {
		t.Fatalf("outbox=%+v err=%v", outbox, err)
	}
}

func TestSubmitIsAllOrNothingWhenScopeValidationFails(t *testing.T) {
	st, ctx := newTaskingPostgresStore(t)
	scope := &domain.Scope{Name: "authorized domain", Type: "domain", Targets: []string{"example.com"}}
	if err := st.CreateScope(ctx, scope); err != nil {
		t.Fatal(err)
	}
	queue := &countingQueue{}
	engine := &Engine{Store: st, Queue: queue, Config: config.Default(), Log: logx.New("error", false)}
	if _, err := engine.Submit(ctx, SubmitInput{
		Name: "partial", Kind: domain.TaskDomain, Target: "example.com,google.com", ScopeID: scope.ID,
	}); err == nil {
		t.Fatal("expected out-of-scope target error")
	}
	_, total, err := st.ListTasks(ctx, store.PageQuery{Page: 1, PageSize: 10}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("created %d tasks despite validation failure", total)
	}
}

func TestRunStepClaimsOnceForDuplicateDelivery(t *testing.T) {
	st, ctx := newTaskingPostgresStore(t)
	task := &domain.Task{
		ID: domain.MustID(), Name: "duplicate delivery", Kind: domain.TaskRiskCruising,
		Target: "http://example.com", Targets: []string{"http://example.com"},
		Status: domain.TaskQueued, Options: domain.TaskOptions{},
	}
	if err := st.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	steps := []domain.TaskStep{
		{ID: domain.MustID(), TaskID: task.ID, Sequence: 1, Kind: "http", Status: domain.StepQueued},
		{ID: domain.MustID(), TaskID: task.ID, Sequence: 2, Kind: "stats", Status: domain.StepPending},
	}
	if err := st.CreateSteps(ctx, steps); err != nil {
		t.Fatal(err)
	}
	probe := &blockingProbe{started: make(chan struct{}, 2), release: make(chan struct{})}
	engine := &Engine{Store: st, Queue: &countingQueue{}, Config: config.Default(), HTTP: probe, Log: logx.New("error", false)}
	firstDone := make(chan error, 1)
	go func() { firstDone <- engine.RunStep(ctx, task.ID, steps[0].ID) }()
	select {
	case <-probe.started:
	case <-time.After(2 * time.Second):
		t.Fatal("first step did not start")
	}
	secondDone := make(chan error, 1)
	go func() { secondDone <- engine.RunStep(ctx, task.ID, steps[0].ID) }()
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("duplicate RunStep: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		close(probe.release)
		t.Fatal("duplicate delivery started another execution")
	}
	close(probe.release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if calls := probe.calls.Load(); calls != 1 {
		t.Fatalf("probe calls = %d, want 1", calls)
	}
}

func TestHandleDeliveryDefersTaskFailureUntilFinalAttempt(t *testing.T) {
	st, ctx := newTaskingPostgresStore(t)
	task := &domain.Task{
		ID: domain.MustID(), Name: "retry failure", Kind: domain.TaskDomain,
		Target: "example.com", Targets: []string{"example.com"},
		Status: domain.TaskQueued, Options: domain.TaskOptions{},
	}
	if err := st.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	steps := []domain.TaskStep{
		{ID: domain.MustID(), TaskID: task.ID, Sequence: 1, Kind: "dns", Status: domain.StepQueued},
		{ID: domain.MustID(), TaskID: task.ID, Sequence: 2, Kind: "stats", Status: domain.StepPending},
	}
	if err := st.CreateSteps(ctx, steps); err != nil {
		t.Fatal(err)
	}
	engine := &Engine{Store: st, Queue: &countingQueue{}, Config: config.Default(), DNS: failingResolver{}, Log: logx.New("error", false)}
	payload := []byte(fmt.Sprintf(`{"taskId":%q,"stepId":%q}`, task.ID, steps[0].ID))
	if err := engine.HandleDelivery(ctx, jobs.Delivery{Envelope: jobs.Envelope{Payload: payload, Attempt: 0}}); err == nil {
		t.Fatal("expected intermediate attempt to fail")
	}
	gotTask, err := st.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotTask.Status != domain.TaskRunning {
		t.Fatalf("task status after intermediate failure = %s, want running", gotTask.Status)
	}
	gotStep, err := st.GetStep(ctx, steps[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotStep.Status != domain.StepQueued {
		t.Fatalf("step status after intermediate failure = %s, want queued", gotStep.Status)
	}
	if err := engine.HandleDelivery(ctx, jobs.Delivery{Envelope: jobs.Envelope{Payload: payload, Attempt: jobs.MaxRetries}}); err == nil {
		t.Fatal("expected final attempt to fail")
	}
	gotTask, err = st.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotTask.Status != domain.TaskFailed {
		t.Fatalf("task status after final failure = %s, want failed", gotTask.Status)
	}
	gotStep, err = st.GetStep(ctx, steps[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotStep.Status != domain.StepFailed {
		t.Fatalf("step status after final failure = %s, want failed", gotStep.Status)
	}
}
