package domain

import (
	"time"
)

const SchemaVersion = 1

// Lifecycle is embedded by user-maintained records so deletes remain
// recoverable while scans, schedulers and credential checks continue to see
// only active rows.
type Lifecycle struct {
	DeletedAt     *time.Time `gorm:"index" json:"deletedAt,omitempty"`
	DeletedBy     string     `json:"deletedBy,omitempty"`
	RestoredAt    *time.Time `json:"restoredAt,omitempty"`
	RestoreReason string     `json:"restoreReason,omitempty"`
}

func (l Lifecycle) Deleted() bool {
	return l.DeletedAt != nil
}

func (l *Lifecycle) MarkDeleted(at time.Time, actor string) {
	l.DeletedAt = &at
	l.DeletedBy = actor
	l.RestoredAt = nil
	l.RestoreReason = ""
}

func (l *Lifecycle) MarkRestored(at time.Time, reason string) {
	l.DeletedAt = nil
	l.DeletedBy = ""
	l.RestoredAt = &at
	l.RestoreReason = reason
}

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

type TaskOrigin string

const (
	TaskOriginManual TaskOrigin = "manual"
	TaskOriginEASM   TaskOrigin = "easm"
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

type AssetOrigin string

const (
	AssetOriginScan   AssetOrigin = "scan"
	AssetOriginManual AssetOrigin = "manual"
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
	ID                 string    `gorm:"primaryKey;size:24" json:"id"`
	Username           string    `json:"username"`
	PasswordHash       string    `json:"-"`
	Role               Role      `json:"role"`
	MustChangePassword bool      `json:"mustChangePassword"`
	Disabled           bool      `json:"disabled"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
	SchemaVersion      int       `json:"schemaVersion"`
	Lifecycle          `gorm:"embedded"`
}

type Session struct {
	ID            string     `gorm:"primaryKey;size:24" json:"id"`
	UserID        string     `json:"userId"`
	RefreshHash   string     `json:"-"`
	UserAgent     string     `json:"userAgent"`
	IP            string     `json:"ip"`
	CreatedAt     time.Time  `json:"createdAt"`
	ExpiresAt     time.Time  `json:"expiresAt"`
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
	RotatedAt     *time.Time `json:"rotatedAt,omitempty"`
	SchemaVersion int        `json:"schemaVersion"`
}

type APIKey struct {
	ID            string     `gorm:"primaryKey;size:24" json:"id"`
	Name          string     `json:"name"`
	KeyHash       string     `json:"-"`
	Scopes        []string   `gorm:"type:jsonb;serializer:json" json:"scopes"`
	CreatedBy     string     `json:"createdBy"`
	CreatedAt     time.Time  `json:"createdAt"`
	LastUsedAt    *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
	SchemaVersion int        `json:"schemaVersion"`
}

type AuditEvent struct {
	ID        string         `gorm:"primaryKey;size:24" json:"id"`
	ActorID   string         `json:"actorId,omitempty"`
	Actor     string         `json:"actor"`
	Action    string         `json:"action"`
	Resource  string         `json:"resource"`
	Detail    map[string]any `gorm:"type:jsonb;serializer:json" json:"detail,omitempty"`
	IP        string         `json:"ip"`
	CreatedAt time.Time      `json:"createdAt"`
}

type DomainPolicy struct {
	DomainBrute     bool   `json:"domainBrute"`
	DomainBruteType string `json:"domainBruteType"`
	AltDNS          bool   `json:"altDns"`
	ARLSearch       bool   `json:"arlSearch"`
	DNSQueryPlugin  bool   `json:"dnsQueryPlugin"`
}

type IPPolicy struct {
	PortScan         bool   `json:"portScan"`
	PortScanType     string `json:"portScanType"`
	PortCustom       string `json:"portCustom"`
	ServiceDetection bool   `json:"serviceDetection"`
	OSDetection      bool   `json:"osDetection"`
	SSLCert          bool   `json:"sslCert"`
	SkipScanCDNIP    bool   `json:"skipScanCdnIp"`
	HostTimeout      int    `json:"hostTimeout"`
	PortParallelism  int    `json:"portParallelism"`
	PortMinRate      int    `json:"portMinRate"`
	ExcludePorts     string `json:"excludePorts"`
}

type SitePolicy struct {
	SiteIdentify  bool `json:"siteIdentify"`
	SiteCapture   bool `json:"siteCapture"`
	SearchEngines bool `json:"searchEngines"`
	SiteSpider    bool `json:"siteSpider"`
	NucleiScan    bool `json:"nucleiScan"`
	WebInfoHunter bool `json:"webInfoHunter"`
	FileLeak      bool `json:"fileLeak"`
	FindVhost     bool `json:"findVhost"`
}

type PluginToggle struct {
	PluginName string `json:"pluginName"`
	VulName    string `json:"vulName,omitempty"`
	Enable     bool   `json:"enable"`
}

type PolicyConfig struct {
	Domain         DomainPolicy   `json:"domain"`
	IP             IPPolicy       `json:"ip"`
	Site           SitePolicy     `json:"site"`
	POC            []PluginToggle `json:"poc"`
	Brute          []PluginToggle `json:"brute"`
	ServiceDetect  bool           `json:"serviceDetect"`
	RelatedScopeID string         `json:"relatedScopeId,omitempty"`
}

type Policy struct {
	ID            string       `gorm:"primaryKey;size:24" json:"id"`
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	Config        PolicyConfig `gorm:"type:jsonb;serializer:json" json:"config"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
	SchemaVersion int          `json:"schemaVersion"`
	Lifecycle     `gorm:"embedded"`
}

type Scope struct {
	ID            string    `gorm:"primaryKey;size:24" json:"id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	Targets       []string  `gorm:"type:jsonb;serializer:json" json:"targets"`
	BlackScope    []string  `gorm:"type:jsonb;serializer:json" json:"blackScope"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	SchemaVersion int       `json:"schemaVersion"`
	Lifecycle     `gorm:"embedded"`
}

type TaskOptions struct {
	DomainBrute     bool     `json:"domainBrute"`
	DomainBruteType string   `json:"domainBruteType"`
	AltDNS          bool     `json:"altDns"`
	ARLSearch       bool     `json:"arlSearch"`
	DNSQueryPlugin  bool     `json:"dnsQueryPlugin"`
	PortScan        bool     `json:"portScan"`
	PortScanType    string   `json:"portScanType"`
	PortCustom      string   `json:"portCustom"`
	ServiceDetect   bool     `json:"serviceDetection"`
	OSDetect        bool     `json:"osDetection"`
	SSLCert         bool     `json:"sslCert"`
	SkipScanCDNIP   bool     `json:"skipScanCdnIp"`
	SiteIdentify    bool     `json:"siteIdentify"`
	SiteCapture     bool     `json:"siteCapture"`
	SearchEngines   bool     `json:"searchEngines"`
	SiteSpider      bool     `json:"siteSpider"`
	FileLeak        bool     `json:"fileLeak"`
	FindVhost       bool     `json:"findVhost"`
	NucleiScan      bool     `json:"nucleiScan"`
	WebInfoHunter   bool     `json:"webInfoHunter"`
	POCPlugins      []string `gorm:"type:jsonb;serializer:json" json:"pocPlugins"`
	BrutePlugins    []string `gorm:"type:jsonb;serializer:json" json:"brutePlugins"`
	ScopeID         string   `json:"scopeId,omitempty"`
	RelatedScopeID  string   `json:"relatedScopeId,omitempty"`
	ResultSetID     string   `json:"resultSetId,omitempty"`
	MonitorID       string   `json:"monitorId,omitempty"`
	MaxHosts        int      `json:"maxHosts,omitempty"`
	MaxPorts        int      `json:"maxPorts,omitempty"`
	MaxHTTPRequests int      `json:"maxHttpRequests,omitempty"`
	MaxRuntime      string   `json:"maxRuntime,omitempty"`
	MaxFindings     int      `json:"maxFindings,omitempty"`
}

type TaskStatistics struct {
	Domains   int `json:"domains"`
	IPs       int `json:"ips"`
	Sites     int `json:"sites"`
	URLs      int `json:"urls"`
	Certs     int `json:"certs"`
	Services  int `json:"services"`
	Vulns     int `json:"vulns"`
	Nuclei    int `json:"nuclei"`
	WIH       int `json:"wih"`
	FileLeaks int `json:"fileLeaks"`
	Protocols int `json:"protocols"`
}

type Task struct {
	ID             string         `gorm:"primaryKey;size:24" json:"id"`
	Name           string         `json:"name"`
	Kind           TaskKind       `json:"kind"`
	Target         string         `json:"target"`
	Targets        []string       `gorm:"type:jsonb;serializer:json" json:"targets"`
	Status         TaskStatus     `json:"status"`
	Progress       int            `json:"progress"`
	Options        TaskOptions    `gorm:"type:jsonb;serializer:json" json:"options"`
	PolicyID       string         `json:"policyId,omitempty"`
	ScopeID        string         `json:"scopeId,omitempty"`
	OrganizationID string         `json:"organizationId,omitempty"`
	RootDomain     string         `json:"rootDomain,omitempty"`
	Origin         TaskOrigin     `json:"origin,omitempty"`
	Statistics     TaskStatistics `gorm:"type:jsonb;serializer:json" json:"statistics"`
	Warnings       []string       `gorm:"type:jsonb;serializer:json" json:"warnings,omitempty"`
	Error          string         `json:"error,omitempty"`
	CreatedBy      string         `json:"createdBy"`
	CreatedAt      time.Time      `json:"createdAt"`
	StartedAt      *time.Time     `json:"startedAt,omitempty"`
	FinishedAt     *time.Time     `json:"finishedAt,omitempty"`
	LastScreenshot string         `json:"lastScreenshot,omitempty"`
	SchemaVersion  int            `json:"schemaVersion"`
	EventSeq       int64          `gorm:"not null;default:0" json:"-"`
}

type TaskStep struct {
	ID            string     `gorm:"primaryKey;size:24" json:"id"`
	TaskID        string     `json:"taskId"`
	Sequence      int        `json:"sequence"`
	Kind          string     `json:"kind"`
	Status        StepStatus `json:"status"`
	Optional      bool       `json:"optional"`
	Attempts      int        `json:"attempts"`
	Progress      int        `json:"progress"`
	Message       string     `json:"message,omitempty"`
	Error         string     `json:"error,omitempty"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	SchemaVersion int        `json:"schemaVersion"`
}

type TaskEvent struct {
	ID        string         `gorm:"primaryKey;size:24" json:"id"`
	TaskID    string         `json:"taskId"`
	Sequence  int64          `json:"sequence"`
	Type      string         `json:"type"`
	Message   string         `json:"message"`
	Payload   map[string]any `gorm:"type:jsonb;serializer:json" json:"payload,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

type OutboxMessage struct {
	ID           string     `gorm:"primaryKey;size:24" json:"id"`
	RoutingKey   string     `json:"routingKey"`
	Payload      []byte     `gorm:"type:bytea" json:"payload"`
	Status       string     `json:"status"`
	Attempts     int        `json:"attempts"`
	AvailableAt  time.Time  `json:"availableAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	PublishedAt  *time.Time `json:"publishedAt,omitempty"`
	ProcessingAt *time.Time `json:"processingAt,omitempty"`
}

type WorkItem struct {
	ID         string     `gorm:"primaryKey;size:24" json:"id"`
	TaskID     string     `json:"taskId"`
	StepID     string     `json:"stepId"`
	Kind       string     `json:"kind"`
	Shard      int        `json:"shard"`
	Status     StepStatus `json:"status"`
	Attempts   int        `json:"attempts"`
	LeaseOwner string     `json:"leaseOwner,omitempty"`
	LeaseUntil *time.Time `json:"leaseUntil,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type Asset struct {
	ID            string         `gorm:"primaryKey;size:24" json:"id"`
	Kind          AssetKind      `json:"kind"`
	Key           string         `json:"key"`
	Data          map[string]any `gorm:"type:jsonb;serializer:json" json:"data"`
	ScopeIDs      []string       `gorm:"type:jsonb;serializer:json" json:"scopeIds,omitempty"`
	Origin        AssetOrigin    `gorm:"not null;default:scan" json:"origin"`
	DisplayName   string         `json:"displayName,omitempty"`
	DisplayURL    string         `json:"displayUrl,omitempty"`
	Notes         string         `json:"notes,omitempty"`
	Tags          []string       `gorm:"type:jsonb;serializer:json" json:"tags,omitempty"`
	FirstSeen     time.Time      `json:"firstSeen"`
	LastSeen      time.Time      `json:"lastSeen"`
	LastTaskID    string         `json:"lastTaskId,omitempty"`
	SchemaVersion int            `json:"schemaVersion"`
	Lifecycle     `gorm:"embedded"`
}

type AssetObservation struct {
	ID            string         `gorm:"primaryKey;size:24" json:"id"`
	TaskID        string         `json:"taskId"`
	AssetID       string         `json:"assetId"`
	Kind          AssetKind      `json:"kind"`
	Key           string         `json:"key"`
	Source        string         `json:"source"`
	Snapshot      map[string]any `gorm:"type:jsonb;serializer:json" json:"snapshot"`
	ObservedAt    time.Time      `json:"observedAt"`
	SchemaVersion int            `json:"schemaVersion"`
}

type ScopeAssetLink struct {
	ID            string    `gorm:"primaryKey;size:24" json:"id"`
	ScopeID       string    `json:"scopeId"`
	AssetID       string    `json:"assetId"`
	Kind          AssetKind `json:"kind"`
	Tags          []string  `gorm:"type:jsonb;serializer:json" json:"tags,omitempty"`
	FirstSeen     time.Time `json:"firstSeen"`
	LastSeen      time.Time `json:"lastSeen"`
	SchemaVersion int       `json:"schemaVersion"`
}

type Finding struct {
	ID            string         `gorm:"primaryKey;size:24" json:"id"`
	TaskID        string         `json:"taskId"`
	ScopeID       string         `json:"scopeId,omitempty"`
	Kind          FindingKind    `json:"kind"`
	DedupeKey     string         `json:"dedupeKey"`
	Title         string         `json:"title"`
	Severity      string         `json:"severity"`
	Target        string         `json:"target"`
	PluginName    string         `json:"pluginName,omitempty"`
	Data          map[string]any `gorm:"type:jsonb;serializer:json" json:"data"`
	CreatedAt     time.Time      `json:"createdAt"`
	SchemaVersion int            `json:"schemaVersion"`
}

type Monitor struct {
	ID            string      `gorm:"primaryKey;size:24" json:"id"`
	Name          string      `json:"name"`
	Kind          string      `json:"kind"` // domain, ip, site_update, wih_update
	ScopeID       string      `json:"scopeId"`
	Target        string      `json:"target"`
	IntervalSec   int         `json:"intervalSec"`
	PolicyID      string      `json:"policyId,omitempty"`
	Options       TaskOptions `gorm:"type:jsonb;serializer:json" json:"options"`
	Status        string      `json:"status"`
	NextRunAt     time.Time   `json:"nextRunAt"`
	LastRunAt     *time.Time  `json:"lastRunAt,omitempty"`
	LastRunID     string      `json:"lastRunId,omitempty"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	SchemaVersion int         `json:"schemaVersion"`
	Lifecycle     `gorm:"embedded"`
}

type Schedule struct {
	ID            string     `gorm:"primaryKey;size:24" json:"id"`
	Name          string     `json:"name"`
	Kind          string     `json:"kind"` // recurrent_scan, future_scan
	TaskKind      TaskKind   `json:"taskKind"`
	Target        string     `json:"target"`
	PolicyID      string     `json:"policyId"`
	ResultSetID   string     `json:"resultSetId,omitempty"`
	Cron          string     `json:"cron,omitempty"`
	StartAt       *time.Time `json:"startAt,omitempty"`
	NextRunAt     *time.Time `json:"nextRunAt,omitempty"`
	Status        string     `json:"status"`
	RunCount      int        `json:"runCount"`
	LastRunAt     *time.Time `json:"lastRunAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	SchemaVersion int        `json:"schemaVersion"`
	Lifecycle     `gorm:"embedded"`
}

type MonitorRun struct {
	ID            string         `gorm:"primaryKey;size:24" json:"id"`
	MonitorID     string         `json:"monitorId"`
	TaskID        string         `json:"taskId"`
	Status        string         `json:"status"`
	NewAssets     int            `json:"newAssets"`
	Detail        map[string]any `gorm:"type:jsonb;serializer:json" json:"detail,omitempty"`
	StartedAt     time.Time      `json:"startedAt"`
	FinishedAt    *time.Time     `json:"finishedAt,omitempty"`
	SchemaVersion int            `json:"schemaVersion"`
}

type GitHubTask struct {
	ID            string     `gorm:"primaryKey;size:24" json:"id"`
	Keyword       string     `json:"keyword"`
	Status        TaskStatus `json:"status"`
	MonitorID     string     `json:"monitorId,omitempty"`
	ResultCount   int        `json:"resultCount"`
	Error         string     `json:"error,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	SchemaVersion int        `json:"schemaVersion"`
}

type GitHubMonitor struct {
	ID            string     `gorm:"primaryKey;size:24" json:"id"`
	Name          string     `json:"name"`
	Keyword       string     `json:"keyword"`
	IntervalSec   int        `json:"intervalSec"`
	Status        string     `json:"status"`
	NextRunAt     time.Time  `json:"nextRunAt"`
	LastRunAt     *time.Time `json:"lastRunAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	SchemaVersion int        `json:"schemaVersion"`
	Lifecycle     `gorm:"embedded"`
}

type GitHubFinding struct {
	ID            string    `gorm:"primaryKey;size:24" json:"id"`
	TaskID        string    `json:"taskId"`
	MonitorID     string    `json:"monitorId,omitempty"`
	Keyword       string    `json:"keyword"`
	Repo          string    `json:"repo"`
	Path          string    `json:"path"`
	HTMLURL       string    `json:"htmlUrl"`
	GitURL        string    `json:"gitUrl"`
	HashMD5       string    `json:"hashMd5"`
	CommitDate    string    `json:"commitDate"`
	HumanContent  string    `json:"humanContent"`
	CreatedAt     time.Time `json:"createdAt"`
	SchemaVersion int       `json:"schemaVersion"`
}

type ExportJob struct {
	ID            string     `gorm:"primaryKey;size:24" json:"id"`
	TaskID        string     `json:"taskId,omitempty"`
	ScopeID       string     `json:"scopeId,omitempty"`
	Format        string     `json:"format"`
	Kind          string     `json:"kind"`
	Status        string     `json:"status"`
	Path          string     `json:"path,omitempty"`
	Error         string     `json:"error,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	SchemaVersion int        `json:"schemaVersion"`
}

type PocPlugin struct {
	ID            string    `gorm:"primaryKey;size:24" json:"id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	AppName       string    `json:"appName"`
	VulName       string    `json:"vulName"`
	Category      string    `json:"category"`
	Schemes       []string  `gorm:"type:jsonb;serializer:json" json:"schemes"`
	Description   string    `json:"description,omitempty"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	SchemaVersion int       `json:"schemaVersion"`
}

type Lease struct {
	ID        string    `gorm:"primaryKey;size:24" json:"id"`
	Owner     string    `json:"owner"`
	ExpiresAt time.Time `json:"expiresAt"`
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
