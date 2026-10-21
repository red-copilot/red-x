package tasking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/discovery"
	"github.com/red-copilot/red-x/internal/modules/poc"
	"github.com/red-copilot/red-x/internal/modules/webintel"
	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"github.com/red-copilot/red-x/internal/platform/netsafe"
	"github.com/red-copilot/red-x/internal/store"
)

type SubmitInput struct {
	Name           string             `json:"name"`
	Kind           domain.TaskKind    `json:"kind"`
	Target         string             `json:"target"`
	Options        domain.TaskOptions `json:"options"`
	OrganizationID string             `json:"organizationId,omitempty"`
	RootDomain     string             `json:"rootDomain,omitempty"`
	Origin         domain.TaskOrigin  `json:"origin,omitempty"`
	PolicyID       string             `json:"policyId"`
	ScopeID        string             `json:"scopeId"`
	ResultSetID    string             `json:"resultSetId"`
	CreatedBy      string             `json:"-"`
}

type Engine struct {
	Store         *store.Store
	Queue         jobs.Queue
	Config        config.Config
	Log           *slog.Logger
	Policy        netsafe.Policy
	DNS           discovery.Resolver
	Passive       discovery.PassiveEnumerator
	Ports         discovery.PortScanner
	TLS           discovery.TLSInspector
	HTTP          discovery.HTTPProbe
	HTTPClient    *http.Client
	ControlClient *http.Client
	Matcher       *webintel.FingerprintMatcher
	WIH           *webintel.WIHScanner
	FileLeak      *webintel.FileLeakScanner
	Spider        webintel.Crawler
	Screenshot    webintel.Screenshotter
	Nuclei        webintel.NucleiScanner
	POCClient     *poc.HTTPClient
	AssetsDir     string
	Notifier      Notifier
	pluginsOnce   sync.Once
	plugins       []poc.Plugin
	pluginByID    map[string]poc.Plugin
}

type Notifier interface {
	Send(ctx context.Context, title, markdown, html string) error
}

type jobPayload struct {
	TaskID string `json:"taskId"`
	StepID string `json:"stepId"`
}

func (e *Engine) InitPlugins() {
	e.pluginsOnce.Do(func() {
		e.plugins = poc.Builtin(e.POCClient, poc.DefaultDictDir(e.AssetsDir))
		e.pluginByID = map[string]poc.Plugin{}
		for _, p := range e.plugins {
			poc.Register(p)
			e.pluginByID[p.Meta().Name] = p
		}
	})
}

func (e *Engine) Plugins() []poc.Plugin {
	e.InitPlugins()
	return append([]poc.Plugin(nil), e.plugins...)
}

func DefaultDomainOptions() domain.TaskOptions {
	return domain.TaskOptions{
		DNSQueryPlugin: true,
		ARLSearch:      true,
		PortScan:       true,
		PortScanType:   "top1000",
		SkipScanCDNIP:  true,
		SiteIdentify:   true,
	}
}

func isZeroOptions(opts domain.TaskOptions) bool {
	return reflect.DeepEqual(opts, domain.TaskOptions{})
}

func withSafeDefaults(kind domain.TaskKind, opts domain.TaskOptions) domain.TaskOptions {
	if kind == domain.TaskDomain && isZeroOptions(opts) {
		return DefaultDomainOptions()
	}
	return opts
}

func (e *Engine) Submit(ctx context.Context, in SubmitInput) ([]domain.Task, error) {
	if strings.TrimSpace(in.Name) == "" {
		in.Name = "任务-" + time.Now().Format("20060102-150405")
	}
	if in.Kind == "" {
		in.Kind = domain.TaskDomain
	}
	in.Options = withSafeDefaults(in.Kind, in.Options)
	if in.PolicyID != "" {
		pid, err := domain.ParseID(in.PolicyID)
		if err != nil {
			return nil, fmt.Errorf("invalid policy id: %w", err)
		}
		p, err := e.Store.GetPolicy(ctx, pid)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, errors.New("policy not found")
		}
		in.Options = MergePolicy(in.Options, p.Config)
	}
	targets, err := discovery.ParseTargets(in.Target)
	if err != nil {
		return nil, err
	}
	if len(targets) > e.Config.Scan.MaxHosts {
		return nil, fmt.Errorf("too many targets: %d > %d", len(targets), e.Config.Scan.MaxHosts)
	}
	var scope *domain.Scope
	if in.ScopeID != "" {
		scopeID, err := domain.ParseID(in.ScopeID)
		if err != nil {
			return nil, fmt.Errorf("invalid scope id: %w", err)
		}
		scope, err = e.Store.GetScope(ctx, scopeID)
		if err != nil {
			return nil, fmt.Errorf("load scope: %w", err)
		}
		if scope == nil {
			return nil, errors.New("scope not found")
		}
		in.Options.ScopeID = in.ScopeID
	}
	if in.ResultSetID != "" {
		in.Options.ResultSetID = in.ResultSetID
	}
	plans := make([]store.TaskPlan, 0, len(targets))
	for _, target := range targets {
		if scope != nil && !scopeAllowsTarget(scope, target) {
			return nil, fmt.Errorf("%w: target %q is outside scope %q", ErrTargetOutsideScope, target.Raw, scope.Name)
		}
		options, err := e.prepareTarget(ctx, in.Options, in.Kind, target)
		if err != nil {
			return nil, err
		}
		taskID, err := domain.NewID()
		if err != nil {
			return nil, err
		}
		t := &domain.Task{
			ID: taskID, Name: in.Name, Kind: in.Kind, Target: target.Raw, Targets: []string{target.Raw},
			Status: domain.TaskQueued, Options: options, PolicyID: in.PolicyID,
			ScopeID: in.ScopeID, OrganizationID: in.OrganizationID, RootDomain: in.RootDomain,
			Origin: in.Origin, CreatedBy: in.CreatedBy,
		}
		steps := StepPlan(t)
		for i := range steps {
			stepID, err := domain.NewID()
			if err != nil {
				return nil, err
			}
			steps[i].ID = stepID
		}
		plan := store.TaskPlan{Task: t, Steps: steps}
		if len(steps) > 0 {
			steps[0].Status = domain.StepQueued
			payload, err := json.Marshal(jobPayload{TaskID: t.ID, StepID: steps[0].ID})
			if err != nil {
				return nil, err
			}
			plan.OutboxRoutingKey = queueForStep(steps[0].Kind)
			plan.OutboxPayload = payload
		}
		plans = append(plans, plan)
	}
	if err := e.Store.CreateTaskPlans(ctx, plans); err != nil {
		return nil, err
	}
	created := make([]domain.Task, 0, len(plans))
	for _, plan := range plans {
		t := plan.Task
		_ = e.Store.AppendTaskEvent(ctx, &domain.TaskEvent{
			TaskID: t.ID, Type: "task.status", Message: "任务已创建",
			Payload: map[string]any{"status": string(domain.TaskQueued), "target": t.Target},
		})
		created = append(created, *t)
	}
	return created, nil
}

func (e *Engine) prepareTarget(ctx context.Context, base domain.TaskOptions, kind domain.TaskKind, target discovery.Target) (domain.TaskOptions, error) {
	options, err := deriveTaskOptions(base, kind, target)
	if err != nil {
		return base, err
	}
	if err := e.validateTarget(ctx, target, kind, options.DNSQueryPlugin); err != nil {
		return base, err
	}
	return options, nil
}

func deriveTaskOptions(base domain.TaskOptions, kind domain.TaskKind, target discovery.Target) (domain.TaskOptions, error) {
	if kind != domain.TaskDomain || target.Kind != "domain" {
		return base, nil
	}
	if strings.Contains(target.Host, "{fuzz}") {
		base.DNSQueryPlugin = false
		return base, nil
	}
	root, err := discovery.IsRootDomain(target.Host)
	if err != nil {
		return base, fmt.Errorf("classify domain %q: %w", target.Host, err)
	}
	base.DNSQueryPlugin = root
	return base, nil
}

func (e *Engine) validateTarget(ctx context.Context, t discovery.Target, kind domain.TaskKind, enumerateSubdomains bool) error {
	switch t.Kind {
	case "domain":
		if domainForbidden(t.Host, e.Config.Scan.ForbiddenDomains) {
			return fmt.Errorf("domain %q is forbidden", t.Host)
		}
		if strings.Contains(t.Host, "{fuzz}") {
			return nil
		}
		if _, err := netsafe.ValidateDomain(t.Host); err != nil {
			return fmt.Errorf("invalid domain %q: %w", t.Host, err)
		}
		if enumerateSubdomains {
			return nil
		}
		return e.Policy.ValidateHost(ctx, t.Host)
	case "ip":
		return e.Policy.ValidateHost(ctx, t.Host)
	case "cidr":
		if t.Prefix.Bits() < 16 {
			return fmt.Errorf("CIDR %s is too large; minimum prefix is /16", t.Raw)
		}
		return nil
	case "url":
		if domainForbidden(t.Host, e.Config.Scan.ForbiddenDomains) {
			return fmt.Errorf("domain %q is forbidden", t.Host)
		}
		_, err := netsafe.ValidateURL(t.Raw, e.Policy)
		return err
	default:
		return fmt.Errorf("unsupported target kind %q", t.Kind)
	}
}

func StepPlan(t *domain.Task) []domain.TaskStep {
	add := func(list []domain.TaskStep, kind string, optional bool) []domain.TaskStep {
		list = append(list, domain.TaskStep{
			TaskID: t.ID, Sequence: len(list) + 1, Kind: kind, Status: domain.StepPending, Optional: optional,
		})
		return list
	}
	var steps []domain.TaskStep
	switch t.Kind {
	case domain.TaskDomain:
		steps = add(steps, "dns", false)
		if t.Options.PortScan {
			steps = add(steps, "port", false)
		}
		steps = add(steps, "http", false)
		steps = add(steps, "webintel", true)
		if len(t.Options.POCPlugins) > 0 || len(t.Options.BrutePlugins) > 0 {
			steps = add(steps, "poc", true)
		}
	case domain.TaskIP:
		if t.Options.PortScan {
			steps = add(steps, "port", false)
		}
		steps = add(steps, "http", false)
		steps = add(steps, "webintel", true)
		if len(t.Options.POCPlugins) > 0 || len(t.Options.BrutePlugins) > 0 {
			steps = add(steps, "poc", true)
		}
	case domain.TaskRiskCruising:
		steps = add(steps, "http", false)
		if len(t.Options.POCPlugins) > 0 || len(t.Options.BrutePlugins) > 0 {
			steps = add(steps, "poc", true)
		}
	case domain.TaskAssetSiteAdd, domain.TaskAssetSiteSync:
		steps = add(steps, "http", false)
		steps = add(steps, "webintel", true)
	case domain.TaskWIHUpdate:
		steps = add(steps, "wih", true)
	case domain.TaskFofa:
		steps = add(steps, "fofa", false)
		steps = add(steps, "port", false)
		steps = add(steps, "http", false)
		steps = add(steps, "webintel", true)
	case domain.TaskGitHub:
		steps = add(steps, "github", false)
	default:
		steps = add(steps, "http", false)
	}
	steps = add(steps, "stats", false)
	return steps
}

func MergePolicy(opts domain.TaskOptions, p domain.PolicyConfig) domain.TaskOptions {
	opts.DomainBrute = opts.DomainBrute || p.Domain.DomainBrute
	if opts.DomainBruteType == "" {
		opts.DomainBruteType = p.Domain.DomainBruteType
	}
	opts.AltDNS = opts.AltDNS || p.Domain.AltDNS
	opts.ARLSearch = opts.ARLSearch || p.Domain.ARLSearch
	opts.DNSQueryPlugin = opts.DNSQueryPlugin || p.Domain.DNSQueryPlugin
	opts.PortScan = opts.PortScan || p.IP.PortScan
	if opts.PortScanType == "" {
		opts.PortScanType = p.IP.PortScanType
	}
	if opts.PortCustom == "" {
		opts.PortCustom = p.IP.PortCustom
	}
	opts.ServiceDetect = opts.ServiceDetect || p.IP.ServiceDetection
	opts.OSDetect = opts.OSDetect || p.IP.OSDetection
	opts.SSLCert = opts.SSLCert || p.IP.SSLCert
	opts.SiteIdentify = opts.SiteIdentify || p.Site.SiteIdentify
	opts.SiteCapture = opts.SiteCapture || p.Site.SiteCapture
	opts.SearchEngines = opts.SearchEngines || p.Site.SearchEngines
	opts.SiteSpider = opts.SiteSpider || p.Site.SiteSpider
	opts.FileLeak = opts.FileLeak || p.Site.FileLeak
	opts.NucleiScan = opts.NucleiScan || p.Site.NucleiScan
	opts.WebInfoHunter = opts.WebInfoHunter || p.Site.WebInfoHunter
	opts.FindVhost = opts.FindVhost || p.Site.FindVhost
	if len(opts.POCPlugins) == 0 {
		for _, x := range p.POC {
			if x.Enable {
				opts.POCPlugins = append(opts.POCPlugins, x.PluginName)
			}
		}
	}
	if len(opts.BrutePlugins) == 0 {
		for _, x := range p.Brute {
			if x.Enable {
				opts.BrutePlugins = append(opts.BrutePlugins, x.PluginName)
			}
		}
	}
	opts.RelatedScopeID = p.RelatedScopeID
	return opts
}

func (e *Engine) enqueueStep(ctx context.Context, taskID, stepID string) error {
	payload, err := json.Marshal(jobPayload{TaskID: taskID, StepID: stepID})
	if err != nil {
		return err
	}
	queue := queueForStep("x")
	step, _ := e.Store.GetStep(ctx, stepID)
	if step != nil {
		queue = queueForStep(step.Kind)
	}
	return e.Store.AddOutbox(ctx, queue, payload)
}

func queueForStep(kind string) string {
	switch kind {
	case "dns", "fofa":
		return "q.discovery.dns"
	case "port":
		return "q.discovery.network"
	case "http":
		return "q.web.fetch"
	case "webintel", "wih":
		return "q.web.scan"
	case "poc":
		return "q.poc.http"
	case "github":
		return "q.github"
	case "stats":
		return "q.maintenance"
	default:
		return "q.maintenance"
	}
}

func (e *Engine) HandleDelivery(ctx context.Context, d jobs.Delivery) error {
	var p jobPayload
	if err := json.Unmarshal(d.Envelope.Payload, &p); err != nil {
		return err
	}
	taskID, err := domain.ParseID(p.TaskID)
	if err != nil {
		return err
	}
	stepID, err := domain.ParseID(p.StepID)
	if err != nil {
		return err
	}
	err = e.RunStep(ctx, taskID, stepID)
	if err != nil && d.Envelope.Attempt >= jobs.MaxRetries {
		_ = e.FailStep(ctx, taskID, stepID, err)
	}
	return err
}

func (e *Engine) RunStep(ctx context.Context, taskID, stepID string) error {
	task, err := e.Store.GetTask(ctx, taskID)
	if err != nil || task == nil {
		return fmt.Errorf("task not found")
	}
	if task.Status.Terminal() {
		if task.Status == domain.TaskCanceled {
			_ = e.Store.UpdateStepFields(ctx, stepID, store.StepPatch{Status: ptr(domain.StepCanceled), Message: ptr("task canceled")})
		}
		return nil
	}
	step, err := e.Store.GetStep(ctx, stepID)
	if err != nil || step == nil {
		return fmt.Errorf("step not found")
	}
	if step.Status == domain.StepSucceeded || step.Status == domain.StepCanceled || step.Status == domain.StepSkipped {
		return nil
	}
	owner, err := domain.NewID()
	if err != nil {
		return err
	}
	claimed, err := e.Store.ClaimStep(ctx, taskID, stepID, owner, stepLeaseTTL)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	stopLease := e.keepStepLease(ctx, stepID, owner)
	defer stopLease()

	now := time.Now().UTC()
	if task.Status == domain.TaskQueued {
		if err := e.Store.UpdateTaskFields(ctx, taskID, store.TaskPatch{Status: ptr(domain.TaskRunning), StartedAt: &now}); err != nil {
			return err
		}
	}
	if err := e.Store.AppendTaskEvent(ctx, &domain.TaskEvent{
		TaskID: taskID, Type: "step.status", Message: "步骤开始: " + step.Kind,
		Payload: map[string]any{"stepId": stepID, "kind": step.Kind, "status": "running"},
	}); err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, e.Config.MaxRuntime())
	defer cancel()
	err = e.executeStep(runCtx, task, step)
	finished := time.Now().UTC()
	if err != nil {
		if step.Optional {
			updated, updateErr := e.Store.UpdateClaimedStepFields(ctx, stepID, owner, store.StepPatch{Status: ptr(domain.StepFailed), Error: ptr(err.Error()), FinishedAt: &finished})
			if updateErr != nil {
				return updateErr
			}
			if !updated {
				return errors.New("step lease lost")
			}
			task.Warnings = append(task.Warnings, fmt.Sprintf("%s: %v", step.Kind, err))
			if updateErr := e.Store.UpdateTaskFields(ctx, taskID, store.TaskPatch{Warnings: ptr(task.Warnings)}); updateErr != nil {
				return updateErr
			}
			_ = e.Store.AppendTaskEvent(ctx, &domain.TaskEvent{
				TaskID: taskID, Type: "step.status", Message: "可选步骤失败: " + step.Kind,
				Payload: map[string]any{"stepId": stepID, "kind": step.Kind, "status": "failed", "error": err.Error()},
			})
			return e.advance(ctx, task, step)
		}
		updated, updateErr := e.Store.UpdateClaimedStepFields(ctx, stepID, owner, store.StepPatch{Status: ptr(domain.StepQueued), Error: ptr(err.Error())})
		if updateErr != nil {
			return updateErr
		}
		if !updated {
			return errors.New("step lease lost")
		}
		_ = e.Store.AppendTaskEvent(ctx, &domain.TaskEvent{
			TaskID: taskID, Type: "step.status", Message: "步骤失败，等待重试: " + step.Kind,
			Payload: map[string]any{"stepId": stepID, "kind": step.Kind, "status": "retrying", "error": err.Error()},
		})
		return err
	}
	updated, err := e.Store.UpdateClaimedStepFields(ctx, stepID, owner, store.StepPatch{Status: ptr(domain.StepSucceeded), Progress: ptr(100), FinishedAt: &finished, Message: ptr("ok")})
	if err != nil {
		return err
	}
	if !updated {
		return errors.New("step lease lost")
	}
	_ = e.Store.AppendTaskEvent(ctx, &domain.TaskEvent{
		TaskID: taskID, Type: "step.status", Message: "步骤完成: " + step.Kind,
		Payload: map[string]any{"stepId": stepID, "kind": step.Kind, "status": "succeeded"},
	})
	return e.advance(ctx, task, step)
}

const (
	stepLeaseTTL     = 2 * time.Minute
	stepLeaseRefresh = 30 * time.Second
)

func (e *Engine) keepStepLease(ctx context.Context, stepID, owner string) func() {
	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(stepLeaseRefresh)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				refreshCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				ok, err := e.Store.RefreshStepLease(refreshCtx, stepID, owner, stepLeaseTTL)
				cancel()
				if err != nil || !ok {
					if e.Log != nil {
						e.Log.Warn("refresh step lease failed", "stepId", stepID, "error", err)
					}
					return
				}
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

func (e *Engine) FailStep(ctx context.Context, taskID, stepID string, cause error) error {
	now := time.Now().UTC()
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	if err := e.Store.UpdateStepFields(ctx, stepID, store.StepPatch{
		Status: ptr(domain.StepFailed), Error: ptr(message), FinishedAt: &now,
	}); err != nil {
		return err
	}
	if err := e.Store.UpdateTaskFields(ctx, taskID, store.TaskPatch{
		Status: ptr(domain.TaskFailed), Error: ptr(message), FinishedAt: &now,
	}); err != nil {
		return err
	}
	_ = e.Store.AppendTaskEvent(ctx, &domain.TaskEvent{
		TaskID: taskID, Type: "task.status", Message: "任务失败",
		Payload: map[string]any{"status": "failed", "error": message},
	})
	return nil
}

func (e *Engine) advance(ctx context.Context, task *domain.Task, step *domain.TaskStep) error {
	next, err := e.Store.GetNextStep(ctx, task.ID, step.Sequence)
	if err != nil {
		return err
	}
	if next == nil {
		return e.finalize(ctx, task.ID)
	}
	if err := e.enqueueStep(ctx, task.ID, next.ID); err != nil {
		return err
	}
	_ = e.Store.UpdateStepFields(ctx, next.ID, store.StepPatch{Status: ptr(domain.StepQueued)})
	total, err := e.Store.ListSteps(ctx, task.ID)
	if err == nil && len(total) > 0 {
		progress := int(float64(step.Sequence) / float64(len(total)) * 100)
		_ = e.Store.UpdateTaskFields(ctx, task.ID, store.TaskPatch{Progress: ptr(progress)})
	}
	return nil
}

func (e *Engine) finalize(ctx context.Context, taskID string) error {
	task, err := e.Store.GetTask(ctx, taskID)
	if err != nil || task == nil {
		return err
	}
	stats, err := e.Store.TaskStatistics(ctx, taskID)
	if err == nil {
		task.Statistics = stats
	}
	status := domain.TaskSucceeded
	if len(task.Warnings) > 0 {
		status = domain.TaskPartial
	}
	now := time.Now().UTC()
	task.Status = status
	task.Progress = 100
	task.FinishedAt = &now
	if err := e.Store.UpdateTask(ctx, task); err != nil {
		return err
	}
	_ = e.Store.AppendTaskEvent(ctx, &domain.TaskEvent{
		TaskID: taskID, Type: "task.status", Message: "任务完成",
		Payload: map[string]any{"status": string(status), "statistics": stats},
	})
	if task.Options.MonitorID != "" && task.ScopeID != "" {
		if monitorID, err := domain.ParseID(task.Options.MonitorID); err == nil {
			_ = e.Store.CreateMonitorRun(ctx, &domain.MonitorRun{
				MonitorID: monitorID, TaskID: task.ID, Status: string(status),
				NewAssets:  stats.Domains + stats.IPs + stats.Sites,
				Detail:     map[string]any{"statistics": stats},
				FinishedAt: &now,
			})
		}
	}
	if e.Notifier != nil {
		markdown := fmt.Sprintf("### %s\n\n- 目标: `%s`\n- 状态: `%s`\n- 域名: %d，IP: %d，站点: %d\n- 漏洞: %d，nuclei: %d，WIH: %d，文件泄漏: %d",
			task.Name, task.Target, status, stats.Domains, stats.IPs, stats.Sites, stats.Vulns, stats.Nuclei, stats.WIH, stats.FileLeaks)
		html := fmt.Sprintf("<h3>%s</h3><p>目标: %s</p><p>状态: %s</p><p>域名 %d，IP %d，站点 %d，漏洞 %d</p>",
			task.Name, task.Target, status, stats.Domains, stats.IPs, stats.Sites, stats.Vulns)
		go func() {
			nctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = e.Notifier.Send(nctx, "[ARL] "+task.Name, markdown, html)
		}()
	}
	return nil
}

func (e *Engine) Cancel(ctx context.Context, taskID string) error {
	task, err := e.Store.GetTask(ctx, taskID)
	if err != nil || task == nil {
		return errors.New("task not found")
	}
	now := time.Now().UTC()
	task.Status = domain.TaskCanceled
	task.FinishedAt = &now
	if err := e.Store.UpdateTask(ctx, task); err != nil {
		return err
	}
	steps, _ := e.Store.ListSteps(ctx, taskID)
	for _, step := range steps {
		if step.Status == domain.StepPending || step.Status == domain.StepQueued || step.Status == domain.StepRunning {
			_ = e.Store.UpdateStepFields(ctx, step.ID, store.StepPatch{Status: ptr(domain.StepCanceled), Message: ptr("task canceled"), FinishedAt: &now})
		}
	}
	_ = e.Store.AppendTaskEvent(ctx, &domain.TaskEvent{
		TaskID: taskID, Type: "task.status", Message: "任务已取消", Payload: map[string]any{"status": "canceled"},
	})
	return nil
}

func (e *Engine) Retry(ctx context.Context, taskID string, createdBy string) ([]domain.Task, error) {
	task, err := e.Store.GetTask(ctx, taskID)
	if err != nil || task == nil {
		return nil, errors.New("task not found")
	}
	return e.Submit(ctx, SubmitInput{
		Name: "重新运行-" + task.Name, Kind: task.Kind, Target: task.Target,
		Options: task.Options, PolicyID: task.PolicyID, ScopeID: task.ScopeID,
		OrganizationID: task.OrganizationID, RootDomain: task.RootDomain, Origin: task.Origin, CreatedBy: createdBy,
	})
}

func (e *Engine) OutboxRelay(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msgs, err := e.Store.PendingOutbox(ctx, 50)
			if err != nil {
				continue
			}
			for _, msg := range msgs {
				if err := e.Queue.Publish(ctx, msg.RoutingKey, msg.Payload); err != nil {
					_ = e.Store.MarkOutbox(ctx, msg.ID, "pending", msg.Attempts+1)
					continue
				}
				_ = e.Store.MarkOutbox(ctx, msg.ID, "published", msg.Attempts+1)
			}
		}
	}
}

func ptr[T any](v T) *T { return &v }
