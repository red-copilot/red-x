package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const SchemaVersion = 1

type Role string

const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

type TaskStatus string

const (
	TaskQueued    TaskStatus = "queued"
	TaskRunning   TaskStatus = "running"
	TaskSucceeded TaskStatus = "succeeded"
	TaskPartial   TaskStatus = "partial"
	TaskFailed    TaskStatus = "failed"
	TaskCanceled  TaskStatus = "canceled"
)

func (s TaskStatus) Terminal() bool {
	return s == TaskSucceeded || s == TaskPartial || s == TaskFailed || s == TaskCanceled
}

type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepQueued    StepStatus = "queued"
	StepRunning   StepStatus = "running"
	StepSucceeded StepStatus = "succeeded"
	StepFailed    StepStatus = "failed"
	StepCanceled  StepStatus = "canceled"
	StepSkipped   StepStatus = "skipped"
)

type TaskKind string

const (
	TaskDomain        TaskKind = "domain"
	TaskIP            TaskKind = "ip"
	TaskRiskCruising  TaskKind = "risk_cruising"
	TaskAssetSiteAdd  TaskKind = "asset_site_add"
	TaskAssetSiteSync TaskKind = "asset_site_update"
	TaskWIHUpdate     TaskKind = "asset_wih_update"
	TaskFofa          TaskKind = "fofa"
	TaskGitHub        TaskKind = "github"
)

type AssetKind string

const (
	AssetDomain  AssetKind = "domain"
	AssetIP      AssetKind = "ip"
	AssetSite    AssetKind = "site"
	AssetURL     AssetKind = "url"
	AssetCert    AssetKind = "cert"
	AssetService AssetKind = "service"
	AssetCIDR    AssetKind = "cidr"
)

type FindingKind string

const (
	FindingVuln     FindingKind = "vuln"
	FindingNuclei   FindingKind = "nuclei"
	FindingWIH      FindingKind = "wih"
	FindingFileLeak FindingKind = "file_leak"
	FindingProtocol FindingKind = "protocol"
)

type User struct {
	ID                 bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Username           string        `bson:"username" json:"username"`
	PasswordHash       string        `bson:"passwordHash" json:"-"`
	Role               Role          `bson:"role" json:"role"`
	MustChangePassword bool          `bson:"mustChangePassword" json:"mustChangePassword"`
	Disabled           bool          `bson:"disabled" json:"disabled"`
	CreatedAt          time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt          time.Time     `bson:"updatedAt" json:"updatedAt"`
	SchemaVersion      int           `bson:"schemaVersion" json:"schemaVersion"`
}

type Session struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID        bson.ObjectID `bson:"userId" json:"userId"`
	RefreshHash   string        `bson:"refreshHash" json:"-"`
	UserAgent     string        `bson:"userAgent" json:"userAgent"`
	IP            string        `bson:"ip" json:"ip"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	ExpiresAt     time.Time     `bson:"expiresAt" json:"expiresAt"`
	RevokedAt     *time.Time    `bson:"revokedAt,omitempty" json:"revokedAt,omitempty"`
	RotatedAt     *time.Time    `bson:"rotatedAt,omitempty" json:"rotatedAt,omitempty"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type APIKey struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name          string        `bson:"name" json:"name"`
	KeyHash       string        `bson:"keyHash" json:"-"`
	Scopes        []string      `bson:"scopes" json:"scopes"`
	CreatedBy     bson.ObjectID `bson:"createdBy" json:"createdBy"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	LastUsedAt    *time.Time    `bson:"lastUsedAt,omitempty" json:"lastUsedAt,omitempty"`
	RevokedAt     *time.Time    `bson:"revokedAt,omitempty" json:"revokedAt,omitempty"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type AuditEvent struct {
	ID        bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	ActorID   bson.ObjectID  `bson:"actorId,omitempty" json:"actorId,omitempty"`
	Actor     string         `bson:"actor" json:"actor"`
	Action    string         `bson:"action" json:"action"`
	Resource  string         `bson:"resource" json:"resource"`
	Detail    map[string]any `bson:"detail,omitempty" json:"detail,omitempty"`
	IP        string         `bson:"ip" json:"ip"`
	CreatedAt time.Time      `bson:"createdAt" json:"createdAt"`
}

type DomainPolicy struct {
	DomainBrute     bool   `bson:"domainBrute" json:"domainBrute"`
	DomainBruteType string `bson:"domainBruteType" json:"domainBruteType"`
	AltDNS          bool   `bson:"altDns" json:"altDns"`
	ARLSearch       bool   `bson:"arlSearch" json:"arlSearch"`
	DNSQueryPlugin  bool   `bson:"dnsQueryPlugin" json:"dnsQueryPlugin"`
}

type IPPolicy struct {
	PortScan         bool   `bson:"portScan" json:"portScan"`
	PortScanType     string `bson:"portScanType" json:"portScanType"`
	PortCustom       string `bson:"portCustom" json:"portCustom"`
	ServiceDetection bool   `bson:"serviceDetection" json:"serviceDetection"`
	OSDetection      bool   `bson:"osDetection" json:"osDetection"`
	SSLCert          bool   `bson:"sslCert" json:"sslCert"`
	SkipScanCDNIP    bool   `bson:"skipScanCdnIp" json:"skipScanCdnIp"`
	HostTimeout      int    `bson:"hostTimeout" json:"hostTimeout"`
	PortParallelism  int    `bson:"portParallelism" json:"portParallelism"`
	PortMinRate      int    `bson:"portMinRate" json:"portMinRate"`
	ExcludePorts     string `bson:"excludePorts" json:"excludePorts"`
}

type SitePolicy struct {
	SiteIdentify  bool `bson:"siteIdentify" json:"siteIdentify"`
	SiteCapture   bool `bson:"siteCapture" json:"siteCapture"`
	SearchEngines bool `bson:"searchEngines" json:"searchEngines"`
	SiteSpider    bool `bson:"siteSpider" json:"siteSpider"`
	NucleiScan    bool `bson:"nucleiScan" json:"nucleiScan"`
	WebInfoHunter bool `bson:"webInfoHunter" json:"webInfoHunter"`
	FileLeak      bool `bson:"fileLeak" json:"fileLeak"`
	FindVhost     bool `bson:"findVhost" json:"findVhost"`
}

type PluginToggle struct {
	PluginName string `bson:"pluginName" json:"pluginName"`
	VulName    string `bson:"vulName,omitempty" json:"vulName,omitempty"`
	Enable     bool   `bson:"enable" json:"enable"`
}

type PolicyConfig struct {
	Domain         DomainPolicy   `bson:"domain" json:"domain"`
	IP             IPPolicy       `bson:"ip" json:"ip"`
	Site           SitePolicy     `bson:"site" json:"site"`
	POC            []PluginToggle `bson:"poc" json:"poc"`
	Brute          []PluginToggle `bson:"brute" json:"brute"`
	ServiceDetect  bool           `bson:"serviceDetect" json:"serviceDetect"`
	RelatedScopeID string         `bson:"relatedScopeId,omitempty" json:"relatedScopeId,omitempty"`
}

type Policy struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name          string        `bson:"name" json:"name"`
	Description   string        `bson:"description" json:"description"`
	Config        PolicyConfig  `bson:"config" json:"config"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time     `bson:"updatedAt" json:"updatedAt"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type Scope struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name          string        `bson:"name" json:"name"`
	Type          string        `bson:"type" json:"type"`
	Targets       []string      `bson:"targets" json:"targets"`
	BlackScope    []string      `bson:"blackScope" json:"blackScope"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time     `bson:"updatedAt" json:"updatedAt"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type TaskOptions struct {
	DomainBrute     bool     `bson:"domainBrute" json:"domainBrute"`
	DomainBruteType string   `bson:"domainBruteType" json:"domainBruteType"`
	AltDNS          bool     `bson:"altDns" json:"altDns"`
	ARLSearch       bool     `bson:"arlSearch" json:"arlSearch"`
	DNSQueryPlugin  bool     `bson:"dnsQueryPlugin" json:"dnsQueryPlugin"`
	PortScan        bool     `bson:"portScan" json:"portScan"`
	PortScanType    string   `bson:"portScanType" json:"portScanType"`
	PortCustom      string   `bson:"portCustom" json:"portCustom"`
	ServiceDetect   bool     `bson:"serviceDetection" json:"serviceDetection"`
	OSDetect        bool     `bson:"osDetection" json:"osDetection"`
	SSLCert         bool     `bson:"sslCert" json:"sslCert"`
	SkipScanCDNIP   bool     `bson:"skipScanCdnIp" json:"skipScanCdnIp"`
	SiteIdentify    bool     `bson:"siteIdentify" json:"siteIdentify"`
	SiteCapture     bool     `bson:"siteCapture" json:"siteCapture"`
	SearchEngines   bool     `bson:"searchEngines" json:"searchEngines"`
	SiteSpider      bool     `bson:"siteSpider" json:"siteSpider"`
	FileLeak        bool     `bson:"fileLeak" json:"fileLeak"`
	FindVhost       bool     `bson:"findVhost" json:"findVhost"`
	NucleiScan      bool     `bson:"nucleiScan" json:"nucleiScan"`
	WebInfoHunter   bool     `bson:"webInfoHunter" json:"webInfoHunter"`
	POCPlugins      []string `bson:"pocPlugins" json:"pocPlugins"`
	BrutePlugins    []string `bson:"brutePlugins" json:"brutePlugins"`
	ScopeID         string   `bson:"scopeId,omitempty" json:"scopeId,omitempty"`
	RelatedScopeID  string   `bson:"relatedScopeId,omitempty" json:"relatedScopeId,omitempty"`
	ResultSetID     string   `bson:"resultSetId,omitempty" json:"resultSetId,omitempty"`
	MonitorID       string   `bson:"monitorId,omitempty" json:"monitorId,omitempty"`
	MaxHosts        int      `bson:"maxHosts,omitempty" json:"maxHosts,omitempty"`
	MaxPorts        int      `bson:"maxPorts,omitempty" json:"maxPorts,omitempty"`
	MaxHTTPRequests int      `bson:"maxHttpRequests,omitempty" json:"maxHttpRequests,omitempty"`
	MaxRuntime      string   `bson:"maxRuntime,omitempty" json:"maxRuntime,omitempty"`
	MaxFindings     int      `bson:"maxFindings,omitempty" json:"maxFindings,omitempty"`
}

type TaskStatistics struct {
	Domains   int `bson:"domains" json:"domains"`
	IPs       int `bson:"ips" json:"ips"`
	Sites     int `bson:"sites" json:"sites"`
	URLs      int `bson:"urls" json:"urls"`
	Certs     int `bson:"certs" json:"certs"`
	Services  int `bson:"services" json:"services"`
	Vulns     int `bson:"vulns" json:"vulns"`
	Nuclei    int `bson:"nuclei" json:"nuclei"`
	WIH       int `bson:"wih" json:"wih"`
	FileLeaks int `bson:"fileLeaks" json:"fileLeaks"`
	Protocols int `bson:"protocols" json:"protocols"`
}

type Task struct {
	ID            bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name          string         `bson:"name" json:"name"`
	Kind          TaskKind       `bson:"kind" json:"kind"`
	Target        string         `bson:"target" json:"target"`
	Targets       []string       `bson:"targets" json:"targets"`
	Status        TaskStatus     `bson:"status" json:"status"`
	Progress      int            `bson:"progress" json:"progress"`
	Options       TaskOptions    `bson:"options" json:"options"`
	PolicyID      string         `bson:"policyId,omitempty" json:"policyId,omitempty"`
	ScopeID       string         `bson:"scopeId,omitempty" json:"scopeId,omitempty"`
	Statistics    TaskStatistics `bson:"statistics" json:"statistics"`
	Warnings      []string       `bson:"warnings,omitempty" json:"warnings,omitempty"`
	Error         string         `bson:"error,omitempty" json:"error,omitempty"`
	CreatedBy     string         `bson:"createdBy" json:"createdBy"`
	CreatedAt     time.Time      `bson:"createdAt" json:"createdAt"`
	StartedAt     *time.Time     `bson:"startedAt,omitempty" json:"startedAt,omitempty"`
	FinishedAt    *time.Time     `bson:"finishedAt,omitempty" json:"finishedAt,omitempty"`
	SchemaVersion int            `bson:"schemaVersion" json:"schemaVersion"`
}

type TaskStep struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	TaskID        bson.ObjectID `bson:"taskId" json:"taskId"`
	Sequence      int           `bson:"sequence" json:"sequence"`
	Kind          string        `bson:"kind" json:"kind"`
	Status        StepStatus    `bson:"status" json:"status"`
	Optional      bool          `bson:"optional" json:"optional"`
	Attempts      int           `bson:"attempts" json:"attempts"`
	Progress      int           `bson:"progress" json:"progress"`
	Message       string        `bson:"message,omitempty" json:"message,omitempty"`
	Error         string        `bson:"error,omitempty" json:"error,omitempty"`
	StartedAt     *time.Time    `bson:"startedAt,omitempty" json:"startedAt,omitempty"`
	FinishedAt    *time.Time    `bson:"finishedAt,omitempty" json:"finishedAt,omitempty"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time     `bson:"updatedAt" json:"updatedAt"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type TaskEvent struct {
	ID        bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	TaskID    bson.ObjectID  `bson:"taskId" json:"taskId"`
	Sequence  int64          `bson:"sequence" json:"sequence"`
	Type      string         `bson:"type" json:"type"`
	Message   string         `bson:"message" json:"message"`
	Payload   map[string]any `bson:"payload,omitempty" json:"payload,omitempty"`
	CreatedAt time.Time      `bson:"createdAt" json:"createdAt"`
}

type OutboxMessage struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	RoutingKey  string        `bson:"routingKey" json:"routingKey"`
	Payload     []byte        `bson:"payload" json:"payload"`
	Status      string        `bson:"status" json:"status"`
	Attempts    int           `bson:"attempts" json:"attempts"`
	AvailableAt time.Time     `bson:"availableAt" json:"availableAt"`
	CreatedAt   time.Time     `bson:"createdAt" json:"createdAt"`
	PublishedAt *time.Time    `bson:"publishedAt,omitempty" json:"publishedAt,omitempty"`
}

type WorkItem struct {
	ID         bson.ObjectID `bson:"_id,omitempty" json:"id"`
	TaskID     bson.ObjectID `bson:"taskId" json:"taskId"`
	StepID     bson.ObjectID `bson:"stepId" json:"stepId"`
	Kind       string        `bson:"kind" json:"kind"`
	Shard      int           `bson:"shard" json:"shard"`
	Status     StepStatus    `bson:"status" json:"status"`
	Attempts   int           `bson:"attempts" json:"attempts"`
	LeaseOwner string        `bson:"leaseOwner,omitempty" json:"leaseOwner,omitempty"`
	LeaseUntil *time.Time    `bson:"leaseUntil,omitempty" json:"leaseUntil,omitempty"`
	CreatedAt  time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time     `bson:"updatedAt" json:"updatedAt"`
}

type Asset struct {
	ID            bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	Kind          AssetKind      `bson:"kind" json:"kind"`
	Key           string         `bson:"key" json:"key"`
	Data          map[string]any `bson:"data" json:"data"`
	ScopeIDs      []string       `bson:"scopeIds,omitempty" json:"scopeIds,omitempty"`
	FirstSeen     time.Time      `bson:"firstSeen" json:"firstSeen"`
	LastSeen      time.Time      `bson:"lastSeen" json:"lastSeen"`
	LastTaskID    string         `bson:"lastTaskId,omitempty" json:"lastTaskId,omitempty"`
	SchemaVersion int            `bson:"schemaVersion" json:"schemaVersion"`
}

type AssetObservation struct {
	ID            bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	TaskID        bson.ObjectID  `bson:"taskId" json:"taskId"`
	AssetID       bson.ObjectID  `bson:"assetId" json:"assetId"`
	Kind          AssetKind      `bson:"kind" json:"kind"`
	Key           string         `bson:"key" json:"key"`
	Source        string         `bson:"source" json:"source"`
	Snapshot      map[string]any `bson:"snapshot" json:"snapshot"`
	ObservedAt    time.Time      `bson:"observedAt" json:"observedAt"`
	SchemaVersion int            `bson:"schemaVersion" json:"schemaVersion"`
}

type ScopeAssetLink struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	ScopeID       bson.ObjectID `bson:"scopeId" json:"scopeId"`
	AssetID       bson.ObjectID `bson:"assetId" json:"assetId"`
	Kind          AssetKind     `bson:"kind" json:"kind"`
	Tags          []string      `bson:"tags,omitempty" json:"tags,omitempty"`
	FirstSeen     time.Time     `bson:"firstSeen" json:"firstSeen"`
	LastSeen      time.Time     `bson:"lastSeen" json:"lastSeen"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type Finding struct {
	ID            bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	TaskID        bson.ObjectID  `bson:"taskId" json:"taskId"`
	ScopeID       string         `bson:"scopeId,omitempty" json:"scopeId,omitempty"`
	Kind          FindingKind    `bson:"kind" json:"kind"`
	DedupeKey     string         `bson:"dedupeKey" json:"dedupeKey"`
	Title         string         `bson:"title" json:"title"`
	Severity      string         `bson:"severity" json:"severity"`
	Target        string         `bson:"target" json:"target"`
	PluginName    string         `bson:"pluginName,omitempty" json:"pluginName,omitempty"`
	Data          map[string]any `bson:"data" json:"data"`
	CreatedAt     time.Time      `bson:"createdAt" json:"createdAt"`
	SchemaVersion int            `bson:"schemaVersion" json:"schemaVersion"`
}

type Monitor struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name          string        `bson:"name" json:"name"`
	Kind          string        `bson:"kind" json:"kind"` // domain, ip, site_update, wih_update
	ScopeID       string        `bson:"scopeId" json:"scopeId"`
	Target        string        `bson:"target" json:"target"`
	IntervalSec   int           `bson:"intervalSec" json:"intervalSec"`
	PolicyID      string        `bson:"policyId,omitempty" json:"policyId,omitempty"`
	Options       TaskOptions   `bson:"options" json:"options"`
	Status        string        `bson:"status" json:"status"`
	NextRunAt     time.Time     `bson:"nextRunAt" json:"nextRunAt"`
	LastRunAt     *time.Time    `bson:"lastRunAt,omitempty" json:"lastRunAt,omitempty"`
	LastRunID     string        `bson:"lastRunId,omitempty" json:"lastRunId,omitempty"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time     `bson:"updatedAt" json:"updatedAt"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type Schedule struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name          string        `bson:"name" json:"name"`
	Kind          string        `bson:"kind" json:"kind"` // recurrent_scan, future_scan
	TaskKind      TaskKind      `bson:"taskKind" json:"taskKind"`
	Target        string        `bson:"target" json:"target"`
	PolicyID      string        `bson:"policyId" json:"policyId"`
	ResultSetID   string        `bson:"resultSetId,omitempty" json:"resultSetId,omitempty"`
	Cron          string        `bson:"cron,omitempty" json:"cron,omitempty"`
	StartAt       *time.Time    `bson:"startAt,omitempty" json:"startAt,omitempty"`
	NextRunAt     *time.Time    `bson:"nextRunAt,omitempty" json:"nextRunAt,omitempty"`
	Status        string        `bson:"status" json:"status"`
	RunCount      int           `bson:"runCount" json:"runCount"`
	LastRunAt     *time.Time    `bson:"lastRunAt,omitempty" json:"lastRunAt,omitempty"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time     `bson:"updatedAt" json:"updatedAt"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type MonitorRun struct {
	ID            bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	MonitorID     bson.ObjectID  `bson:"monitorId" json:"monitorId"`
	TaskID        string         `bson:"taskId" json:"taskId"`
	Status        string         `bson:"status" json:"status"`
	NewAssets     int            `bson:"newAssets" json:"newAssets"`
	Detail        map[string]any `bson:"detail,omitempty" json:"detail,omitempty"`
	StartedAt     time.Time      `bson:"startedAt" json:"startedAt"`
	FinishedAt    *time.Time     `bson:"finishedAt,omitempty" json:"finishedAt,omitempty"`
	SchemaVersion int            `bson:"schemaVersion" json:"schemaVersion"`
}

type GitHubTask struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Keyword       string        `bson:"keyword" json:"keyword"`
	Status        TaskStatus    `bson:"status" json:"status"`
	MonitorID     string        `bson:"monitorId,omitempty" json:"monitorId,omitempty"`
	ResultCount   int           `bson:"resultCount" json:"resultCount"`
	Error         string        `bson:"error,omitempty" json:"error,omitempty"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	StartedAt     *time.Time    `bson:"startedAt,omitempty" json:"startedAt,omitempty"`
	FinishedAt    *time.Time    `bson:"finishedAt,omitempty" json:"finishedAt,omitempty"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type GitHubMonitor struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name          string        `bson:"name" json:"name"`
	Keyword       string        `bson:"keyword" json:"keyword"`
	IntervalSec   int           `bson:"intervalSec" json:"intervalSec"`
	Status        string        `bson:"status" json:"status"`
	NextRunAt     time.Time     `bson:"nextRunAt" json:"nextRunAt"`
	LastRunAt     *time.Time    `bson:"lastRunAt,omitempty" json:"lastRunAt,omitempty"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time     `bson:"updatedAt" json:"updatedAt"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type GitHubFinding struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	TaskID        string        `bson:"taskId" json:"taskId"`
	MonitorID     string        `bson:"monitorId,omitempty" json:"monitorId,omitempty"`
	Keyword       string        `bson:"keyword" json:"keyword"`
	Repo          string        `bson:"repo" json:"repo"`
	Path          string        `bson:"path" json:"path"`
	HTMLURL       string        `bson:"htmlUrl" json:"htmlUrl"`
	GitURL        string        `bson:"gitUrl" json:"gitUrl"`
	HashMD5       string        `bson:"hashMd5" json:"hashMd5"`
	CommitDate    string        `bson:"commitDate" json:"commitDate"`
	HumanContent  string        `bson:"humanContent" json:"humanContent"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type ExportJob struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	TaskID        string        `bson:"taskId,omitempty" json:"taskId,omitempty"`
	ScopeID       string        `bson:"scopeId,omitempty" json:"scopeId,omitempty"`
	Format        string        `bson:"format" json:"format"`
	Kind          string        `bson:"kind" json:"kind"`
	Status        string        `bson:"status" json:"status"`
	Path          string        `bson:"path,omitempty" json:"path,omitempty"`
	Error         string        `bson:"error,omitempty" json:"error,omitempty"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	FinishedAt    *time.Time    `bson:"finishedAt,omitempty" json:"finishedAt,omitempty"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type PocPlugin struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name          string        `bson:"name" json:"name"`
	Type          string        `bson:"type" json:"type"`
	AppName       string        `bson:"appName" json:"appName"`
	VulName       string        `bson:"vulName" json:"vulName"`
	Category      string        `bson:"category" json:"category"`
	Schemes       []string      `bson:"schemes" json:"schemes"`
	Description   string        `bson:"description,omitempty" json:"description,omitempty"`
	Enabled       bool          `bson:"enabled" json:"enabled"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time     `bson:"updatedAt" json:"updatedAt"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type Lease struct {
	ID        string    `bson:"_id" json:"id"`
	Owner     string    `bson:"owner" json:"owner"`
	ExpiresAt time.Time `bson:"expiresAt" json:"expiresAt"`
}

func NewModel[T any](v T) T {
	now := time.Now().UTC()
	switch x := any(v).(type) {
	case User:
		x.CreatedAt, x.UpdatedAt, x.SchemaVersion = now, now, SchemaVersion
		return any(x).(T)
	case Policy:
		x.CreatedAt, x.UpdatedAt, x.SchemaVersion = now, now, SchemaVersion
		return any(x).(T)
	case Scope:
		x.CreatedAt, x.UpdatedAt, x.SchemaVersion = now, now, SchemaVersion
		return any(x).(T)
	case Task:
		x.CreatedAt, x.SchemaVersion = now, SchemaVersion
		return any(x).(T)
	case TaskStep:
		x.CreatedAt, x.UpdatedAt, x.SchemaVersion = now, now, SchemaVersion
		return any(x).(T)
	case Asset:
		x.FirstSeen, x.LastSeen, x.SchemaVersion = now, now, SchemaVersion
		return any(x).(T)
	case Finding:
		x.CreatedAt, x.SchemaVersion = now, SchemaVersion
		return any(x).(T)
	default:
		return v
	}
}
