package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig       `yaml:"server"`
	Mongo       MongoConfig        `yaml:"mongo"`
	RabbitMQ    RabbitMQConfig     `yaml:"rabbitmq"`
	Queue       QueueConfig        `yaml:"queue"`
	Auth        AuthConfig         `yaml:"auth"`
	Scan        ScanConfig         `yaml:"scan"`
	Storage     StorageConfig      `yaml:"storage"`
	Tools       ToolsConfig        `yaml:"tools"`
	Scanners    ScannersConfig     `yaml:"scanners"`
	GeoIP       GeoIPConfig        `yaml:"geoip"`
	QueryPlugin map[string]any     `yaml:"queryPlugins"`
	Notify      NotificationConfig `yaml:"notifications"`
	GitHub      GitHubConfig       `yaml:"github"`
	Fofa        FofaConfig         `yaml:"fofa"`
	Neo4j       Neo4jConfig        `yaml:"neo4j"`
	ICP         ICPConfig          `yaml:"icp"`
}

type ServerConfig struct {
	Listen    string `yaml:"listen"`
	TLSCert   string `yaml:"tlsCert"`
	TLSKey    string `yaml:"tlsKey"`
	PublicURL string `yaml:"publicURL"`
	Debug     bool   `yaml:"debug"`
}

type MongoConfig struct {
	URI      string `yaml:"uri"`
	Database string `yaml:"database"`
}

type RabbitMQConfig struct {
	URL string `yaml:"url"`
}

type QueueConfig struct {
	Driver string `yaml:"driver"`
}

type AuthConfig struct {
	AccessSecret      string `yaml:"accessSecret"`
	AccessTTL         string `yaml:"accessTTL"`
	RefreshTTL        string `yaml:"refreshTTL"`
	CookieSecure      bool   `yaml:"cookieSecure"`
	BootstrapPassword string `yaml:"bootstrapPassword"`
	APIKeyPepper      string `yaml:"apiKeyPepper"`
}

type ScanConfig struct {
	BlackCIDRs       []string `yaml:"blackCIDRs"`
	ForbiddenDomains []string `yaml:"forbiddenDomains"`
	DNSServers       []string `yaml:"dnsServers"`
	Concurrency      int      `yaml:"concurrency"`
	HTTPConcurrency  int      `yaml:"httpConcurrency"`
	MaxHosts         int      `yaml:"maxHosts"`
	MaxPorts         int      `yaml:"maxPorts"`
	MaxRuntime       string   `yaml:"maxRuntime"`
	ProxyURL         string   `yaml:"proxyURL"`
	AllowPrivate     bool     `yaml:"allowPrivate"`
	AllowLoopback    bool     `yaml:"allowLoopback"`
}

type StorageConfig struct {
	DataDir        string `yaml:"dataDir"`
	ScreenshotsDir string `yaml:"screenshotsDir"`
	ExportsDir     string `yaml:"exportsDir"`
	AssetsDir      string `yaml:"assetsDir"`
}

type ToolsConfig struct {
	ChromiumPath string `yaml:"chromiumPath"`
}

type ScannersConfig struct {
	Subfinder SubfinderScannerConfig `yaml:"subfinder"`
	DNSX      DNSXScannerConfig      `yaml:"dnsx"`
	Naabu     NaabuScannerConfig     `yaml:"naabu"`
	TLSX      TLSXScannerConfig      `yaml:"tlsx"`
	HTTPX     HTTPXScannerConfig     `yaml:"httpx"`
	Katana    KatanaScannerConfig    `yaml:"katana"`
	Nuclei    NucleiScannerConfig    `yaml:"nuclei"`
}

type SubfinderScannerConfig struct {
	Sources            []string `yaml:"sources"`
	Threads            int      `yaml:"threads"`
	Timeout            int      `yaml:"timeout"`
	MaxEnumerationTime int      `yaml:"maxEnumerationTime"`
	ProviderConfig     string   `yaml:"providerConfig"`
}

type DNSXScannerConfig struct {
	Retries int `yaml:"retries"`
	Timeout int `yaml:"timeout"`
}

type NaabuScannerConfig struct {
	ScanType         string `yaml:"scanType"`
	Rate             int    `yaml:"rate"`
	Timeout          int    `yaml:"timeout"`
	Threads          int    `yaml:"threads"`
	ServiceDiscovery bool   `yaml:"serviceDiscovery"`
	ServiceVersion   bool   `yaml:"serviceVersion"`
	ExcludeCDN       bool   `yaml:"excludeCDN"`
}

type TLSXScannerConfig struct {
	ScanMode string `yaml:"scanMode"`
	Timeout  int    `yaml:"timeout"`
	Retries  int    `yaml:"retries"`
}

type HTTPXScannerConfig struct {
	Timeout      int `yaml:"timeout"`
	Retries      int `yaml:"retries"`
	MaxRedirects int `yaml:"maxRedirects"`
	MaxBodyRead  int `yaml:"maxBodyRead"`
}

type KatanaScannerConfig struct {
	MaxDepth    int `yaml:"maxDepth"`
	MaxPages    int `yaml:"maxPages"`
	RateLimit   int `yaml:"rateLimit"`
	Timeout     int `yaml:"timeout"`
	Concurrency int `yaml:"concurrency"`
}

type NucleiScannerConfig struct {
	TemplatesDir     string   `yaml:"templatesDir"`
	Protocols        []string `yaml:"protocols"`
	Timeout          int      `yaml:"timeout"`
	RateLimit        int      `yaml:"rateLimit"`
	BulkSize         int      `yaml:"bulkSize"`
	TemplateThreads  int      `yaml:"templateThreads"`
	InteractshURL    string   `yaml:"interactshURL"`
	TemplateSeverity []string `yaml:"templateSeverity"`
}

type GeoIPConfig struct {
	City string `yaml:"city"`
	ASN  string `yaml:"asn"`
}

type ICPConfig struct {
	Chinaz struct {
		CookieFile string `yaml:"cookieFile"`
		UserAgent  string `yaml:"userAgent"`
	} `yaml:"chinaz"`
	ICPQuery struct {
		BaseURL  string `yaml:"baseURL"`
		Timeout  int    `yaml:"timeout"`
		PageSize int    `yaml:"pageSize"`
	} `yaml:"icpQuery"`
}

type Neo4jConfig struct {
	URI          string `yaml:"uri"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Database     string `yaml:"database"`
	AllowRebuild bool   `yaml:"allowRebuild"`
}

type NotificationConfig struct {
	DingTalk DingTalkConfig `yaml:"dingtalk"`
	Feishu   FeishuConfig   `yaml:"feishu"`
	WeCom    WeComConfig    `yaml:"wecom"`
	SMTP     SMTPConfig     `yaml:"smtp"`
	Webhook  WebhookConfig  `yaml:"webhook"`
}

type DingTalkConfig struct {
	AccessToken string `yaml:"accessToken"`
	Secret      string `yaml:"secret"`
}

type FeishuConfig struct {
	WebhookURL string `yaml:"webhookURL"`
	Secret     string `yaml:"secret"`
}

type WeComConfig struct {
	WebhookURL string `yaml:"webhookURL"`
}

type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	To       string `yaml:"to"`
}

type WebhookConfig struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

type GitHubConfig struct {
	Token string `yaml:"token"`
}

type FofaConfig struct {
	URL      string `yaml:"url"`
	Key      string `yaml:"key"`
	MaxPage  int    `yaml:"maxPage"`
	PageSize int    `yaml:"pageSize"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Listen:    ":5003",
			PublicURL: "https://localhost:5003",
		},
		Mongo: MongoConfig{
			URI:      "mongodb://127.0.0.1:27017",
			Database: "arl",
		},
		RabbitMQ: RabbitMQConfig{
			URL: "amqp://arl:arlpassword@127.0.0.1:5672/arlv2host",
		},
		Queue: QueueConfig{Driver: "memory"},
		Auth: AuthConfig{
			AccessTTL:    "15m",
			RefreshTTL:   "168h",
			CookieSecure: true,
		},
		Scan: ScanConfig{
			BlackCIDRs: []string{
				"127.0.0.0/8",
				"0.0.0.0/8",
				"169.254.0.0/16",
				"::1/128",
				"fe80::/10",
			},
			DNSServers:      []string{"223.5.5.5:53", "1.1.1.1:53"},
			Concurrency:     200,
			HTTPConcurrency: 12,
			MaxHosts:        4096,
			MaxPorts:        65535,
			MaxRuntime:      "2h",
		},
		Storage: StorageConfig{
			DataDir:        "./data",
			ScreenshotsDir: "./data/screenshots",
			ExportsDir:     "./data/exports",
			AssetsDir:      "./assets",
		},
		Scanners: ScannersConfig{
			Subfinder: SubfinderScannerConfig{
				Sources:            []string{"crtsh", "rapiddns", "alienvault"},
				Threads:            10,
				Timeout:            30,
				MaxEnumerationTime: 5,
				ProviderConfig:     "./data/subfinder-provider.yaml",
			},
			DNSX:  DNSXScannerConfig{Retries: 3, Timeout: 4},
			Naabu: NaabuScannerConfig{ScanType: "connect", Rate: 1000, Timeout: 3, Threads: 100, ServiceDiscovery: true, ServiceVersion: true, ExcludeCDN: true},
			TLSX:  TLSXScannerConfig{ScanMode: "ctls", Timeout: 5, Retries: 1},
			HTTPX: HTTPXScannerConfig{Timeout: 10, Retries: 1, MaxRedirects: 5, MaxBodyRead: 2 << 20},
			Katana: KatanaScannerConfig{
				MaxDepth: 2, MaxPages: 100, RateLimit: 50, Timeout: 10, Concurrency: 10,
			},
			Nuclei: NucleiScannerConfig{
				TemplatesDir: "./assets/nuclei", Protocols: []string{"http"}, Timeout: 10,
				RateLimit: 100, BulkSize: 10, TemplateThreads: 10,
			},
		},
		GitHub: GitHubConfig{},
		Fofa: FofaConfig{
			URL:      "https://fofa.info",
			MaxPage:  5,
			PageSize: 2000,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("read config: %w", err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse config: %w", err)
		}
	}
	applyEnv(&cfg)
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func applyEnv(c *Config) {
	set := func(key string, dst *string) {
		if v := os.Getenv(key); v != "" {
			*dst = v
		}
	}
	set("ARL_SERVER_LISTEN", &c.Server.Listen)
	set("ARL_SERVER_PUBLIC_URL", &c.Server.PublicURL)
	set("ARL_SERVER_TLS_CERT", &c.Server.TLSCert)
	set("ARL_SERVER_TLS_KEY", &c.Server.TLSKey)
	set("ARL_MONGO_URI", &c.Mongo.URI)
	set("ARL_MONGO_DATABASE", &c.Mongo.Database)
	set("ARL_RABBITMQ_URL", &c.RabbitMQ.URL)
	set("ARL_QUEUE_DRIVER", &c.Queue.Driver)
	set("ARL_AUTH_ACCESS_SECRET", &c.Auth.AccessSecret)
	set("ARL_AUTH_ACCESS_TTL", &c.Auth.AccessTTL)
	set("ARL_AUTH_REFRESH_TTL", &c.Auth.RefreshTTL)
	set("ARL_ADMIN_PASSWORD", &c.Auth.BootstrapPassword)
	set("ARL_API_KEY_PEPPER", &c.Auth.APIKeyPepper)
	set("ARL_STORAGE_DATA_DIR", &c.Storage.DataDir)
	set("ARL_STORAGE_SCREENSHOTS_DIR", &c.Storage.ScreenshotsDir)
	set("ARL_STORAGE_EXPORTS_DIR", &c.Storage.ExportsDir)
	set("ARL_STORAGE_ASSETS_DIR", &c.Storage.AssetsDir)
	set("ARL_CHROMIUM_PATH", &c.Tools.ChromiumPath)
	set("ARL_SUBFINDER_PROVIDER_CONFIG", &c.Scanners.Subfinder.ProviderConfig)
	set("ARL_NUCLEI_TEMPLATES_DIR", &c.Scanners.Nuclei.TemplatesDir)
	set("ARL_NUCLEI_INTERACTSH_URL", &c.Scanners.Nuclei.InteractshURL)
	set("ARL_NAABU_SCAN_TYPE", &c.Scanners.Naabu.ScanType)
	set("ARL_GEOIP_CITY", &c.GeoIP.City)
	set("ARL_GEOIP_ASN", &c.GeoIP.ASN)
	set("ARL_GITHUB_TOKEN", &c.GitHub.Token)
	set("ARL_FOFA_KEY", &c.Fofa.Key)
	set("ARL_FOFA_URL", &c.Fofa.URL)
	set("ARL_PROXY_URL", &c.Scan.ProxyURL)
	set("ARL_NEO4J_URI", &c.Neo4j.URI)
	set("ARL_NEO4J_USER", &c.Neo4j.User)
	set("ARL_NEO4J_PASSWORD", &c.Neo4j.Password)
	set("ARL_NEO4J_DATABASE", &c.Neo4j.Database)
	set("ARL_ICP_CHINAZ_USER_AGENT", &c.ICP.Chinaz.UserAgent)
	set("ARL_ICP_ICPQUERY_BASE_URL", &c.ICP.ICPQuery.BaseURL)
	if v := os.Getenv("ARL_COOKIE_SECURE"); v != "" {
		c.Auth.CookieSecure, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("ARL_ALLOW_PRIVATE"); v != "" {
		c.Scan.AllowPrivate, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("ARL_ALLOW_LOOPBACK"); v != "" {
		c.Scan.AllowLoopback, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("ARL_DEBUG"); v != "" {
		c.Server.Debug, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("ARL_NEO4J_ALLOW_REBUILD"); v != "" {
		c.Neo4j.AllowRebuild, _ = strconv.ParseBool(v)
	}
}

func (c Config) Validate() error {
	var errs []string
	if c.Mongo.URI == "" || c.Mongo.Database == "" {
		errs = append(errs, "mongo.uri and mongo.database are required")
	}
	if c.Queue.Driver != "memory" && c.Queue.Driver != "rabbitmq" {
		errs = append(errs, "queue.driver must be memory or rabbitmq")
	}
	if c.Queue.Driver == "rabbitmq" && c.RabbitMQ.URL == "" {
		errs = append(errs, "rabbitmq.url is required when queue.driver=rabbitmq")
	}
	if c.Auth.AccessSecret == "" {
		errs = append(errs, "auth.accessSecret is required (set ARL_AUTH_ACCESS_SECRET)")
	}
	if _, err := time.ParseDuration(c.Auth.AccessTTL); err != nil {
		errs = append(errs, "auth.accessTTL is invalid: "+err.Error())
	}
	if _, err := time.ParseDuration(c.Auth.RefreshTTL); err != nil {
		errs = append(errs, "auth.refreshTTL is invalid: "+err.Error())
	}
	if _, err := time.ParseDuration(c.Scan.MaxRuntime); err != nil {
		errs = append(errs, "scan.maxRuntime is invalid: "+err.Error())
	}
	if c.Storage.DataDir == "" {
		errs = append(errs, "storage.dataDir is required")
	}
	validScanType := map[string]bool{"connect": true, "syn": true}
	if !validScanType[strings.ToLower(c.Scanners.Naabu.ScanType)] {
		errs = append(errs, "scanners.naabu.scanType must be connect or syn")
	}
	validTLSMode := map[string]bool{"ctls": true, "auto": true, "ztls": true, "openssl": true}
	if !validTLSMode[strings.ToLower(c.Scanners.TLSX.ScanMode)] {
		errs = append(errs, "scanners.tlsx.scanMode must be ctls, auto, ztls or openssl")
	}
	if c.Scanners.Subfinder.Threads <= 0 || c.Scanners.Subfinder.Timeout <= 0 || c.Scanners.Subfinder.MaxEnumerationTime <= 0 {
		errs = append(errs, "scanners.subfinder threads, timeout and maxEnumerationTime must be positive")
	}
	if c.Scanners.HTTPX.Timeout <= 0 || c.Scanners.HTTPX.MaxBodyRead <= 0 {
		errs = append(errs, "scanners.httpx timeout and maxBodyRead must be positive")
	}
	if c.Scanners.Nuclei.TemplatesDir == "" {
		errs = append(errs, "scanners.nuclei.templatesDir is required")
	}
	for _, protocol := range c.Scanners.Nuclei.Protocols {
		if !strings.EqualFold(protocol, "http") {
			errs = append(errs, "scanners.nuclei.protocols only supports http")
			break
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (c Config) AccessTTL() time.Duration {
	d, _ := time.ParseDuration(c.Auth.AccessTTL)
	return d
}

func (c Config) RefreshTTL() time.Duration {
	d, _ := time.ParseDuration(c.Auth.RefreshTTL)
	return d
}

func (c Config) MaxRuntime() time.Duration {
	d, _ := time.ParseDuration(c.Scan.MaxRuntime)
	if d <= 0 {
		return 2 * time.Hour
	}
	return d
}

func (c Config) EnsureDirs() error {
	dirs := []string{c.Storage.DataDir, c.Storage.ScreenshotsDir, c.Storage.ExportsDir}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Clean(d), 0o750); err != nil {
			return err
		}
	}
	return nil
}
