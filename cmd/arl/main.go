package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Aabyss-Team/ARL/internal/app"
	"github.com/Aabyss-Team/ARL/internal/domain"
	"github.com/Aabyss-Team/ARL/internal/platform/config"
	"github.com/Aabyss-Team/ARL/internal/platform/jobs"
	"github.com/Aabyss-Team/ARL/internal/platform/logx"
	"github.com/Aabyss-Team/ARL/internal/platform/pdtools"
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
		runWeb(args)
	case "worker":
		runWorker(args)
	case "scheduler":
		runScheduler(args)
	case "all", "dev":
		runAll(args)
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
	case "version", "--version", "-v":
		fmt.Println("arl", version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`ARL modular monolith
Usage:
  arl web       [--config config.yaml]
  arl worker    [--config config.yaml] [--queues q.web.fetch,q.web.scan]
  arl scheduler [--config config.yaml]
  arl all       [--config config.yaml]   # development: web+worker+scheduler
  arl migrate   [--config config.yaml]
  arl doctor    [--config config.yaml]
  arl health    [--url http://127.0.0.1:5003/readyz]
  arl admin create-user --username NAME --password PASS [--role admin]
  arl admin reset-password --username NAME --password PASS
  arl poc sync  [--config config.yaml]`)
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

func runWeb(args []string) {
	a, cfg, ctx, cancel := build(args)
	defer cancel()
	defer a.Close(context.Background())
	if err := a.Identity.BootstrapAdmin(ctx); err != nil {
		a.Log.Error("bootstrap admin failed", "error", err)
	}
	srv := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           a.Server.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       90 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, c := context.WithTimeout(context.Background(), 15*time.Second)
		defer c()
		_ = srv.Shutdown(shutdownCtx)
	}()
	a.Log.Info("web listening", "address", cfg.Server.Listen)
	var err error
	if cfg.Server.TLSCert != "" && cfg.Server.TLSKey != "" {
		err = srv.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey)
	} else {
		err = srv.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		a.Log.Error("web server stopped", "error", err)
	}
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
	queues := splitCSV(*queuesFlag)
	log.Info("worker started", "queues", queues, "driver", cfg.Queue.Driver)
	err = a.Queue.Consume(ctx, queues, 2, func(ctx context.Context, d jobs.Delivery) error {
		if d.Queue == "q.github" {
			return fmt.Errorf("github queue jobs are handled by the API-triggered GitHub worker")
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

func runAll(args []string) {
	a, cfg, ctx, cancel := build(args)
	defer cancel()
	defer a.Close(context.Background())
	if err := a.Identity.BootstrapAdmin(ctx); err != nil {
		a.Log.Error("bootstrap admin failed", "error", err)
	}
	go a.Engine.OutboxRelay(ctx)
	go a.Scheduler.Run(ctx)
	go func() {
		err := a.Queue.Consume(ctx, jobs.QueueNames, 2, func(ctx context.Context, d jobs.Delivery) error {
			if d.Queue == "q.github" {
				return nil
			}
			return a.Engine.HandleDelivery(ctx, d)
		})
		if err != nil && ctx.Err() == nil {
			a.Log.Error("worker stopped", "error", err)
		}
	}()
	srv := &http.Server{Addr: cfg.Server.Listen, Handler: a.Server.Router(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, c := context.WithTimeout(context.Background(), 15*time.Second)
		defer c()
		_ = srv.Shutdown(shutdownCtx)
	}()
	a.Log.Info("development all-in-one listening", "address", cfg.Server.Listen, "queue", cfg.Queue.Driver)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		a.Log.Error("server stopped", "error", err)
	}
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
	fmt.Printf("indexes ready, default policy ready, %d PoC plugins registered\n", count)
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
	if a, err := app.New(ctx, cfg, logx.New("", cfg.Server.Debug), version); err != nil {
		fmt.Println("mongo/queue/assets: FAIL:", err)
		problems++
	} else {
		fmt.Println("mongo: OK")
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
