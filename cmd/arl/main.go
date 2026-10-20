package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/red-copilot/red-x/internal/app"
	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/easm"
	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/platform/geoip"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"github.com/red-copilot/red-x/internal/platform/logx"
	"github.com/red-copilot/red-x/internal/platform/pdtools"
	"golang.org/x/sync/errgroup"
)

var version = "3.0.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "web":
		if err := runWeb(args); err != nil {
			fatal(err)
		}
	case "worker":
		runWorker(args)
	case "scheduler":
		runScheduler(args)
	case "all", "dev":
		if err := runAll(args); err != nil {
			fatal(err)
		}
	case "migrate":
		runMigrate(args)
	case "doctor":
		runDoctor(args)
	case "health":
		runHealth(args)
	case "admin":
		runAdmin(args)
	case "poc":
		runPOC(args)
	case "graph":
		runGraph(args)
	case "version", "--version", "-v":
		fmt.Println("arl", version)
	default:
		usage()
		os.Exit(2)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "arl:", err)
	os.Exit(1)
}

func usage() {
	fmt.Println(`ARL modular monolith
Usage:
  arl web       [--config config.yaml]
  arl worker    [--config config.yaml] [--queues q.web.fetch,q.web.scan]
  arl scheduler [--config config.yaml]
  arl all       [--config config.yaml]   # single-node: web+worker+scheduler
  arl migrate   [--config config.yaml]
  arl doctor    [--config config.yaml]
  arl health    [--url http://127.0.0.1:5003/readyz]
  arl admin create-user --username NAME --password PASS [--role admin]
  arl admin reset-password --username NAME --password PASS
  arl poc sync  [--config config.yaml]
  arl graph rebuild [--config config.yaml]`)
}

func configFlag(args []string) (config.Config, string) {
	fs := flag.NewFlagSet("arl", flag.ContinueOnError)
	path := fs.String("config", "config.yaml", "path to config file")
	_ = fs.Parse(args)
	cfg, err := config.Load(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	return cfg, *path
}

func build(args []string) (*app.App, config.Config, context.Context, context.CancelFunc) {
	cfg, _ := configFlag(args)
	log := logx.New("", cfg.Server.Debug)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	a, err := app.New(ctx, cfg, log, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "startup error:", err)
		cancel()
		os.Exit(1)
	}
	return a, cfg, ctx, cancel
}

func runWeb(args []string) error {
	a, cfg, ctx, cancel := build(args)
	defer cancel()
	defer a.Close(context.Background())
	if err := a.Identity.BootstrapAdmin(ctx); err != nil {
		a.Log.Error("bootstrap admin failed", "error", err)
	}
	a.Server.EASM = newEASMService(a)
	a.Log.Info("web listening", "address", cfg.Server.Listen)
	return serveHTTP(ctx, cfg, a.Log, a.Server.Router())
}

func serveHTTP(ctx context.Context, cfg config.Config, log *slog.Logger, handler http.Handler) error {
	srv := newHTTPServer(cfg, handler)
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
		case <-stopped:
		}
	}()
	defer close(stopped)

	var err error
	if cfg.Server.TLSCert != "" && cfg.Server.TLSKey != "" {
		err = srv.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey)
	} else {
		err = srv.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if err != nil {
		log.Error("http server stopped", "error", err)
	}
	return err
}

func newHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       90 * time.Second,
	}
}

func runComponents(ctx context.Context, components ...func(context.Context) error) error {
	group, groupCtx := errgroup.WithContext(ctx)
	for _, component := range components {
		component := component
		group.Go(func() error {
			return component(groupCtx)
		})
	}
	err := group.Wait()
	if ctx.Err() != nil && (err == nil || errors.Is(err, context.Canceled)) {
		return nil
	}
	return err
}

func runWorker(args []string) {
	fs := flag.NewFlagSet("worker", flag.ContinueOnError)
	path := fs.String("config", "config.yaml", "path to config file")
	queuesFlag := fs.String("queues", strings.Join(jobs.QueueNames, ","), "comma-separated queues")
	_ = fs.Parse(args)
	cfg, err := config.Load(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	log := logx.New("", cfg.Server.Debug)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	a, err := app.New(ctx, cfg, log, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "startup error:", err)
		os.Exit(1)
	}
	defer a.Close(context.Background())
	go a.Engine.OutboxRelay(ctx)
	stopProjector := startProjector(ctx, a, log)
	if stopProjector != nil {
		defer stopProjector()
	}
	icpSvc := newEASMService(a)
	queues := splitCSV(*queuesFlag)
	log.Info("worker started", "queues", queues, "driver", cfg.Queue.Driver)
	err = a.Queue.Consume(ctx, queues, 2, func(ctx context.Context, d jobs.Delivery) error {
		if d.Queue == "q.github" {
			return fmt.Errorf("github queue jobs are handled by the API-triggered GitHub worker")
		}
		if d.Queue == "q.icp" {
			return icpSvc.HandleICPJob(ctx, d)
		}
		return a.Engine.HandleDelivery(ctx, d)
	})
	if err != nil && ctx.Err() == nil {
		log.Error("worker stopped", "error", err)
	}
}

func runScheduler(args []string) {
	a, _, ctx, cancel := build(args)
	defer cancel()
	defer a.Close(context.Background())
	if err := a.Identity.BootstrapAdmin(ctx); err != nil {
		a.Log.Error("bootstrap admin failed", "error", err)
	}
	go a.Engine.OutboxRelay(ctx)
	a.Log.Info("scheduler started", "owner", a.Scheduler.Owner)
	a.Scheduler.Run(ctx)
}

func runAll(args []string) error {
	a, cfg, ctx, cancel := build(args)
	defer cancel()
	defer a.Close(context.Background())
	if err := a.Identity.BootstrapAdmin(ctx); err != nil {
		a.Log.Error("bootstrap admin failed", "error", err)
	}
	if cfg.Queue.Driver == "memory" {
		a.Log.Warn("single-node mode uses the in-memory queue; pending jobs will be lost on restart")
	}
	go a.Engine.OutboxRelay(ctx)
	stopProjector := startProjector(ctx, a, a.Log)
	if stopProjector != nil {
		defer stopProjector()
	}
	icpSvc := newEASMService(a)
	a.Server.EASM = icpSvc
	go a.Scheduler.Run(ctx)
	a.Log.Info("single-node server listening", "address", cfg.Server.Listen, "queue", cfg.Queue.Driver)
	return runComponents(ctx,
		func(ctx context.Context) error {
			err := a.Queue.Consume(ctx, jobs.QueueNames, 2, func(ctx context.Context, d jobs.Delivery) error {
				if d.Queue == "q.github" {
					return nil
				}
				if d.Queue == "q.icp" {
					return icpSvc.HandleICPJob(ctx, d)
				}
				return a.Engine.HandleDelivery(ctx, d)
			})
			if err == nil && ctx.Err() == nil {
				return errors.New("worker stopped unexpectedly")
			}
			return err
		},
		func(ctx context.Context) error {
			return serveHTTP(ctx, cfg, a.Log, a.Server.Router())
		},
	)
}

func runMigrate(args []string) {
	a, _, ctx, cancel := build(args)
	defer cancel()
	defer a.Close(context.Background())
	if err := a.Identity.BootstrapAdmin(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap admin:", err)
		os.Exit(1)
	}
	count := 0
	for _, p := range a.Engine.Plugins() {
		m := p.Meta()
		if err := a.Store.UpsertPOCPlugin(ctx, &domain.PocPlugin{
			Name: m.Name, Type: string(m.Type), AppName: m.AppName, VulName: m.VulName,
			Category: m.Category, Schemes: m.Schemes, Enabled: true,
		}); err == nil {
			count++
		}
	}
	fmt.Printf("postgres schema ready, default policy ready, %d PoC plugins registered\n", count)
}

func runDoctor(args []string) {
	cfg, _ := configFlag(args)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	problems := 0
	if err := cfg.Validate(); err != nil {
		fmt.Println("config: FAIL:", err)
		problems++
	} else {
		fmt.Println("config: OK")
	}
	if err := cfg.EnsureDirs(); err != nil {
		fmt.Println("storage: FAIL:", err)
		problems++
	} else {
		fmt.Println("storage: OK")
	}
	for name, version := range pdtools.Versions() {
		fmt.Printf("sdk %s: %s\n", name, version)
	}
	providerConfig := cfg.Scanners.Subfinder.ProviderConfig
	if providerConfig == "" {
		providerConfig = pdtools.ProviderConfigPath(cfg.Storage.DataDir)
	}
	if _, err := os.Stat(providerConfig); err != nil {
		fmt.Println("subfinder provider config: WARN:", err)
	} else {
		fmt.Println("subfinder provider config: OK")
	}
	templatesDir := cfg.Scanners.Nuclei.TemplatesDir
	if templatesDir == "" {
		templatesDir = filepath.Join(cfg.Storage.AssetsDir, "nuclei")
	}
	if count := pdtools.CountTemplates(templatesDir); count == 0 {
		fmt.Println("nuclei templates: WARN: no YAML templates found")
	} else {
		fmt.Printf("nuclei templates: OK (%d files)\n", count)
	}
	for _, name := range []string{"subfinder", "dnsx", "naabu", "tlsx", "httpx", "katana", "nuclei"} {
		if path, err := exec.LookPath(name); err == nil {
			fmt.Printf("external CLI %s: WARN: found at %s (not used)\n", name, path)
		}
	}

	if cfg.GeoIP.City == "" || cfg.GeoIP.ASN == "" {
		fmt.Println("geoip: WARN: city or ASN database is not configured")
	} else if resolver, err := geoip.NewMaxMind(cfg.GeoIP.City, cfg.GeoIP.ASN); err != nil {
		fmt.Println("geoip: FAIL:", err)
		problems++
	} else {
		_ = resolver.Close()
		fmt.Println("geoip: OK")
	}
	if cfg.Neo4j.URI == "" {
		fmt.Println("neo4j: WARN: uri is not configured")
	} else if reader, err := easm.NewNeo4jReader(cfg.Neo4j); err != nil {
		fmt.Println("neo4j: FAIL:", err)
		problems++
	} else {
		probeCtx, probeCancel := context.WithTimeout(ctx, 3*time.Second)
		err := reader.Probe(probeCtx)
		probeCancel()
		_ = reader.Close(context.Background())
		if err != nil {
			fmt.Println("neo4j: FAIL:", err)
			problems++
		} else {
			fmt.Println("neo4j: OK")
		}
	}
	baseURL := cfg.ICP.ICPQuery.BaseURL
	if baseURL == "" {
		baseURL = "http://127.0.0.1:16181"
	}
	if provider := easm.NewICPQueryProvider(baseURL, cfg.ICP.ICPQuery.PageSize, 0, &http.Client{Timeout: 3 * time.Second}); provider != nil {
		if err := provider.Probe(ctx); err != nil {
			fmt.Println("icp-query: WARN:", err)
		} else {
			fmt.Println("icp-query: OK")
		}
	}
	cookieConfigured := strings.TrimSpace(os.Getenv("ARL_ICP_CHINAZ_COOKIE")) != ""
	if cfg.ICP.Chinaz.CookieFile != "" {
		if st, err := os.Stat(cfg.ICP.Chinaz.CookieFile); err != nil || st.Size() == 0 {
			fmt.Println("chinaz cookie: FAIL: configured file is missing or empty")
			problems++
		} else {
			cookieConfigured = true
		}
	}
	if cookieConfigured {
		fmt.Println("chinaz cookie: OK")
	} else {
		fmt.Println("chinaz cookie: WARN: not configured")
	}

	if a, err := app.New(ctx, cfg, logx.New("", cfg.Server.Debug), version); err != nil {
		fmt.Println("postgres/queue/assets: FAIL:", err)
		problems++
	} else {
		fmt.Println("postgres: OK")
		if err := a.Queue.Health(ctx); err != nil {
			fmt.Println("queue: FAIL:", err)
			problems++
		} else {
			fmt.Println("queue: OK")
		}
		if a.Matcher.Count() == 0 {
			fmt.Println("fingerprints: WARN: no built-in rules loaded")
		} else {
			fmt.Printf("fingerprints: OK (%d rules)\n", a.Matcher.Count())
		}
		go a.Close(context.Background())
	}
	if cfg.Tools.ChromiumPath != "" {
		if _, err := os.Stat(cfg.Tools.ChromiumPath); err != nil {
			fmt.Println("chromium: FAIL:", err)
			problems++
		} else {
			fmt.Println("chromium: OK")
		}
	} else {
		fmt.Println("chromium: WARN: tools.chromiumPath is empty; screenshots will auto-detect a browser")
	}
	if _, err := net.LookupHost("example.com"); err != nil {
		fmt.Println("dns: WARN:", err)
	} else {
		fmt.Println("dns: OK")
	}
	if problems > 0 {
		os.Exit(1)
	}
}

func runHealth(args []string) {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	target := fs.String("url", "http://127.0.0.1:5003/readyz", "health endpoint")
	_ = fs.Parse(args)
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(*target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "health check failed:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "health check status:", resp.StatusCode)
		os.Exit(1)
	}
	fmt.Println("ok")
}

func runAdmin(args []string) {
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	sub := args[0]
	fs := flag.NewFlagSet("admin "+sub, flag.ContinueOnError)
	path := fs.String("config", "config.yaml", "path to config file")
	username := fs.String("username", "", "username")
	password := fs.String("password", "", "password")
	role := fs.String("role", "admin", "role: viewer|operator|admin")
	_ = fs.Parse(args[1:])
	cfg, err := config.Load(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	a, err := app.New(ctx, cfg, logx.New("", cfg.Server.Debug), version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "startup error:", err)
		os.Exit(1)
	}
	defer a.Close(context.Background())
	switch sub {
	case "create-user":
		u, err := a.Identity.CreateUser(ctx, *username, *password, domain.Role(*role))
		if err != nil {
			fmt.Fprintln(os.Stderr, "create user:", err)
			os.Exit(1)
		}
		fmt.Printf("created user %s (%s)\n", u.Username, u.Role)
	case "reset-password":
		u, err := a.Store.GetUserByUsername(ctx, *username)
		if err != nil || u == nil {
			fmt.Fprintln(os.Stderr, "user not found")
			os.Exit(1)
		}
		if err := a.Identity.ResetPassword(ctx, u.ID, *password); err != nil {
			fmt.Fprintln(os.Stderr, "reset password:", err)
			os.Exit(1)
		}
		fmt.Printf("password reset for %s\n", u.Username)
	default:
		fmt.Fprintln(os.Stderr, "unknown admin subcommand")
		os.Exit(2)
	}
}

func runPOC(args []string) {
	if len(args) == 0 || args[0] != "sync" {
		usage()
		os.Exit(2)
	}
	a, _, ctx, cancel := build(args[1:])
	defer cancel()
	defer a.Close(context.Background())
	count := 0
	for _, p := range a.Engine.Plugins() {
		m := p.Meta()
		if err := a.Store.UpsertPOCPlugin(ctx, &domain.PocPlugin{
			Name: m.Name, Type: string(m.Type), AppName: m.AppName, VulName: m.VulName,
			Category: m.Category, Schemes: m.Schemes, Enabled: true,
		}); err == nil {
			count++
		}
	}
	fmt.Printf("synced %d PoC plugins\n", count)
}

func splitCSV(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(x)
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

func startProjector(ctx context.Context, a *app.App, log *slog.Logger) func() {
	writer, err := easm.NewNeo4jWriter(a.Config.Neo4j)
	if err != nil {
		log.Warn("graph projector disabled", "error", err)
		return nil
	}
	if writer == nil {
		return nil
	}
	projector := easm.NewProjector(a.Store, writer, log)
	go projector.Run(ctx)
	return func() { _ = writer.Close(context.Background()) }
}

func runGraph(args []string) {
	if len(args) == 0 || args[0] != "rebuild" {
		usage()
		os.Exit(2)
	}
	a, cfg, ctx, cancel := build(args[1:])
	defer cancel()
	defer a.Close(context.Background())
	writer, err := easm.NewNeo4jWriter(cfg.Neo4j)
	if err != nil {
		fmt.Fprintln(os.Stderr, "neo4j error:", err)
		os.Exit(1)
	}
	if writer == nil {
		fmt.Fprintln(os.Stderr, "neo4j is not configured")
		os.Exit(1)
	}
	defer func() { _ = writer.Close(context.Background()) }()
	projector := easm.NewProjector(a.Store, writer, a.Log)
	if err := projector.Rebuild(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "graph rebuild failed:", err)
		os.Exit(1)
	}
	fmt.Println("graph rebuilt")
}

func newEASMService(a *app.App) *easm.Service {
	cookie := os.Getenv("ARL_ICP_CHINAZ_COOKIE")
	if path := a.Config.ICP.Chinaz.CookieFile; path != "" {
		if b, err := os.ReadFile(path); err == nil {
			cookie = strings.TrimSpace(string(b))
		} else {
			a.Log.Warn("ICP cookie file not readable", "path", path, "error", err)
		}
	}
	baseURL := a.Config.ICP.ICPQuery.BaseURL
	if baseURL == "" {
		baseURL = "http://127.0.0.1:16181"
	}
	timeout := time.Duration(a.Config.ICP.ICPQuery.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	pageSize := a.Config.ICP.ICPQuery.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	chinaz := easm.NewChinazProvider(cookie, a.Config.ICP.Chinaz.UserAgent,
		"https://icp.chinaz.com", "https://tooldata.chinaz.com/icp/api/website/queryPermit", nil)
	icpQuery := easm.NewICPQueryProvider(baseURL, pageSize, 5*time.Second, &http.Client{Timeout: timeout})
	svc := easm.NewService(a.Store, a.Store, a.Queue, chinaz, icpQuery, a.Log)
	svc.Submitter = a.Engine
	svc.ChinazConfigured = strings.TrimSpace(cookie) != ""
	if geo, err := geoip.NewMaxMind(a.Config.GeoIP.City, a.Config.GeoIP.ASN); err == nil {
		svc.SetGeoResolver(geo)
	} else {
		a.Log.Warn("GeoIP disabled", "error", err)
	}
	if reader, err := easm.NewNeo4jReader(a.Config.Neo4j); err == nil && reader != nil {
		svc.SetGraphReader(reader)
	}
	return svc
}
