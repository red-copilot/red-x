package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/red-copilot/red-x/internal/httpapi"
	"github.com/red-copilot/red-x/internal/modules/discovery"
	"github.com/red-copilot/red-x/internal/modules/github"
	"github.com/red-copilot/red-x/internal/modules/identity"
	"github.com/red-copilot/red-x/internal/modules/monitoring"
	"github.com/red-copilot/red-x/internal/modules/poc"
	"github.com/red-copilot/red-x/internal/modules/reporting"
	"github.com/red-copilot/red-x/internal/modules/tasking"
	"github.com/red-copilot/red-x/internal/modules/webintel"
	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"github.com/red-copilot/red-x/internal/platform/netsafe"
	"github.com/red-copilot/red-x/internal/platform/notify"
	"github.com/red-copilot/red-x/internal/platform/pdtools"
	"github.com/red-copilot/red-x/internal/store"
)

type App struct {
	Config    config.Config
	Log       *slog.Logger
	Store     *store.Store
	Queue     jobs.Queue
	Identity  *identity.Service
	Engine    *tasking.Engine
	Scheduler *monitoring.Scheduler
	GitHub    *github.Service
	Exporter  *reporting.Exporter
	Server    *httpapi.Server
	Policy    netsafe.Policy
	Matcher   *webintel.FingerprintMatcher
	Nuclei    *webintel.NucleiSDKScanner
}

func New(ctx context.Context, cfg config.Config, log *slog.Logger, version string) (*App, error) {
	pdtools.ConfigureLogging(log, cfg.Server.Debug)
	if err := cfg.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("create data directories: %w", err)
	}
	st, err := store.Open(ctx, cfg.Mongo.URI, cfg.Mongo.Database)
	if err != nil {
		return nil, err
	}
	if err := st.EnsureIndexes(ctx); err != nil {
		_ = st.Close(context.Background())
		return nil, err
	}
	_ = st.SeedDefaultPolicy(ctx)

	var queue jobs.Queue
	if cfg.Queue.Driver == "redis" {
		queue, err = jobs.NewRedis(ctx, cfg.Redis.URL, log)
	} else {
		queue = jobs.NewMemory(log)
	}
	if err != nil {
		_ = st.Close(context.Background())
		return nil, err
	}
	policy, err := netsafe.NewPolicy(cfg.Scan.BlackCIDRs, cfg.Scan.AllowPrivate, cfg.Scan.AllowLoopback)
	if err != nil {
		_ = queue.Close()
		_ = st.Close(context.Background())
		return nil, err
	}
	matcher := webintel.NewFingerprintMatcher()
	webappPath := filepath.Join(cfg.Storage.AssetsDir, "dicts", "webapp.json")
	if err := matcher.LoadBuiltin(webappPath); err != nil {
		log.Warn("built-in fingerprint database not loaded", "path", webappPath, "error", err)
	}
	if rules, err := st.ListFingerprints(ctx); err == nil {
		dbRules := make([]webintel.DBRule, 0, len(rules))
		for _, r := range rules {
			dbRules = append(dbRules, webintel.DBRule{Name: r.Name, Rule: r.Rule})
		}
		matcher.SetDBRules(dbRules)
	}
	wihCfg, err := webintel.LoadWIHConfig(filepath.Join(cfg.Storage.AssetsDir, "rules", "wih_rules.yml"))
	if err != nil {
		log.Warn("WIH rules not loaded", "error", err)
	}
	providerConfig := cfg.Scanners.Subfinder.ProviderConfig
	if providerConfig == "" {
		providerConfig = pdtools.ProviderConfigPath(cfg.Storage.DataDir)
	}
	if err := pdtools.EnsureProviderConfig(providerConfig); err != nil {
		log.Warn("subfinder provider config not prepared", "path", providerConfig, "error", err)
	}
	if err := pdtools.EnsureNucleiIgnore(); err != nil {
		log.Warn("nuclei ignore file not prepared", "error", err)
	}
	dnsResolver, err := discovery.NewPDDNSResolver(
		cfg.Scan.DNSServers, cfg.Scan.Concurrency, cfg.Scanners.DNSX.Retries,
		time.Duration(cfg.Scanners.DNSX.Timeout)*time.Second, policy,
	)
	if err != nil {
		_ = queue.Close()
		_ = st.Close(context.Background())
		return nil, err
	}
	portScanner := discovery.NewNaabuPortScanner(
		cfg.Scanners.Naabu.Threads, cfg.Scanners.Naabu.Rate, cfg.Scanners.Naabu.Timeout,
		cfg.Scanners.Naabu.ScanType, cfg.Scanners.Naabu.ServiceDiscovery, cfg.Scanners.Naabu.ServiceVersion,
		cfg.Scanners.Naabu.ExcludeCDN, policy,
	)
	tlsInspector, err := discovery.NewTLSXInspector(cfg.Scanners.TLSX.ScanMode, cfg.Scanners.TLSX.Timeout, cfg.Scanners.TLSX.Retries)
	if err != nil {
		_ = queue.Close()
		_ = st.Close(context.Background())
		return nil, err
	}
	prober := discovery.NewHTTPXProbe(
		policy, cfg.Scan.ProxyURL, cfg.Scan.HTTPConcurrency, cfg.Scanners.HTTPX.Timeout,
		cfg.Scanners.HTTPX.Retries, cfg.Scanners.HTTPX.MaxRedirects, cfg.Scanners.HTTPX.MaxBodyRead,
	)
	safeClient := pdtools.NewSafeHTTPClient(policy, cfg.Scan.ProxyURL, cfg.Scanners.HTTPX.Timeout)
	passive := discovery.NewSubfinderEnumerator(
		cfg.Scanners.Subfinder.Sources, cfg.Scanners.Subfinder.Threads,
		cfg.Scanners.Subfinder.Timeout, cfg.Scanners.Subfinder.MaxEnumerationTime, providerConfig,
	)
	passive.MaxResults = cfg.Scan.MaxHosts
	crawler := webintel.NewKatanaCrawler(
		cfg.Scanners.Katana.MaxDepth, cfg.Scanners.Katana.MaxPages, cfg.Scanners.Katana.RateLimit,
		cfg.Scanners.Katana.Timeout, cfg.Scanners.Katana.Concurrency,
	)
	nucleiTemplates := cfg.Scanners.Nuclei.TemplatesDir
	if nucleiTemplates == "" {
		nucleiTemplates = filepath.Join(cfg.Storage.AssetsDir, "nuclei")
	}
	nucleiScanner := webintel.NewNucleiSDKScanner(
		nucleiTemplates, cfg.Scan.ProxyURL, cfg.Scanners.Nuclei.InteractshURL,
		pdtools.BlockedTargets(policy), !cfg.Scan.AllowPrivate,
		cfg.Scanners.Nuclei.Timeout, cfg.Scanners.Nuclei.RateLimit,
		cfg.Scanners.Nuclei.BulkSize, cfg.Scanners.Nuclei.TemplateThreads,
	)
	pocClient := poc.NewHTTPClient(policy, cfg.Scan.ProxyURL, 10*time.Second)
	engine := &tasking.Engine{
		Store: st, Queue: queue, Config: cfg, Log: log, Policy: policy,
		DNS: dnsResolver, Passive: passive, Ports: portScanner, TLS: tlsInspector,
		HTTP: prober, HTTPClient: safeClient,
		ControlClient: pdtools.NewVerifiedHTTPClient(policy, cfg.Scan.ProxyURL, cfg.Scanners.HTTPX.Timeout),
		Matcher:       matcher,
		WIH:           &webintel.WIHScanner{Client: safeClient, Config: wihCfg},
		FileLeak:      webintel.NewFileLeakScanner(safeClient, 6),
		Spider:        crawler,
		Screenshot:    webintel.Screenshotter{BrowserPath: cfg.Tools.ChromiumPath, OutputDir: cfg.Storage.ScreenshotsDir},
		Nuclei:        nucleiScanner,
		POCClient:     pocClient, AssetsDir: cfg.Storage.AssetsDir,
	}
	engine.InitPlugins()
	ident := identity.New(st, cfg, log)
	gh := &github.Service{Store: st, Token: cfg.GitHub.Token, Client: &http.Client{Timeout: 30 * time.Second}, Log: log}
	sched := monitoring.New(st, engine, gh, log)
	exporter := &reporting.Exporter{Store: st, Dir: cfg.Storage.ExportsDir}
	engine.Notifier = notify.New(cfg.Notify, log)
	srv := &httpapi.Server{
		Config: cfg, Store: st, Queue: queue, Identity: ident, Engine: engine,
		Scheduler: sched, GitHub: gh, Exporter: exporter, Matcher: matcher,
		WIH: &webintel.WIHScanner{Client: safeClient, Config: wihCfg},
		Log: log, Version: version, StartedAt: time.Now(),
	}
	return &App{
		Config: cfg, Log: log, Store: st, Queue: queue, Identity: ident,
		Engine: engine, Scheduler: sched, GitHub: gh, Exporter: exporter,
		Server: srv, Policy: policy, Matcher: matcher, Nuclei: nucleiScanner,
	}, nil
}

func (a *App) Close(ctx context.Context) {
	if a.Queue != nil {
		_ = a.Queue.Close()
	}
	if a.Store != nil {
		_ = a.Store.Close(ctx)
	}
	if a.Nuclei != nil {
		a.Nuclei.Close()
	}
}
