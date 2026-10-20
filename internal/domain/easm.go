package domain

import (
	"errors"
	"strings"
	"time"
)

// ErrDomainAlreadyConfirmed is returned when a root domain already has another owner.
var ErrDomainAlreadyConfirmed = errors.New("domain already has a confirmed owner")

// Confidence describes how strongly an ICP attribution is supported.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

func (c Confidence) Valid() bool {
	return c == ConfidenceHigh || c == ConfidenceMedium || c == ConfidenceLow
}

// OrgStatus is the lifecycle state of an organization.
type OrgStatus string

const (
	OrgStatusActive OrgStatus = "active"
	OrgStatusMerged OrgStatus = "merged"
)

func (s OrgStatus) Valid() bool {
	return s == OrgStatusActive || s == OrgStatusMerged
}

// DomainStatus is the review state of an organization-domain attribution.
type DomainStatus string

const (
	DomainStatusCandidate DomainStatus = "candidate"
	DomainStatusConfirmed DomainStatus = "confirmed"
	DomainStatusRejected  DomainStatus = "rejected"
)

func (s DomainStatus) Valid() bool {
	return s == DomainStatusCandidate || s == DomainStatusConfirmed || s == DomainStatusRejected
}

// ICPJobStatus is the lifecycle state of an asynchronous ICP refresh.
type ICPJobStatus string

const (
	ICPJobQueued  ICPJobStatus = "queued"
	ICPJobRunning ICPJobStatus = "running"
	ICPJobDone    ICPJobStatus = "done"
	ICPJobFailed  ICPJobStatus = "failed"
)

// CanTransition reports whether an ICP job may move from one state to another.
func (s ICPJobStatus) CanTransition(next ICPJobStatus) bool {
	switch s {
	case ICPJobQueued:
		return next == ICPJobRunning
	case ICPJobRunning:
		return next == ICPJobDone || next == ICPJobFailed
	default:
		return false
	}
}

// GraphOutboxOp is the operation carried by a graph projection event.
type GraphOutboxOp string

const (
	OutboxOpUpsert GraphOutboxOp = "upsert"
	OutboxOpDelete GraphOutboxOp = "delete"
)

func (o GraphOutboxOp) Valid() bool {
	return o == OutboxOpUpsert || o == OutboxOpDelete
}

type Organization struct {
	ID            string            `gorm:"primaryKey;size:24" json:"id"`
	CanonicalName string            `json:"canonicalName"`
	Aliases       []string          `gorm:"type:jsonb;serializer:json" json:"aliases,omitempty"`
	USCC          string            `json:"uscc,omitempty"`
	Region        string            `json:"region,omitempty"`
	ExternalIDs   map[string]string `gorm:"type:jsonb;serializer:json" json:"externalIds,omitempty"`
	Status        OrgStatus         `json:"status"`
	MergedInto    *string           `json:"mergedInto,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
	SchemaVersion int               `json:"schemaVersion"`
	Lifecycle     `gorm:"embedded"`
}

func (o *Organization) Validate() error {
	if o == nil || strings.TrimSpace(o.CanonicalName) == "" {
		return errors.New("organization canonical name is required")
	}
	if o.Status == "" {
		o.Status = OrgStatusActive
	}
	if !o.Status.Valid() {
		return errors.New("invalid organization status")
	}
	if o.Status == OrgStatusMerged && o.MergedInto == nil {
		return errors.New("merged organization requires mergedInto")
	}
	return nil
}

type OrganizationDomain struct {
	ID             string       `gorm:"primaryKey;size:24" json:"id"`
	OrganizationID string       `json:"organizationId"`
	RootDomain     string       `json:"rootDomain"`
	SiteName       string       `json:"siteName,omitempty"`
	HomeSite       string       `json:"homeSite,omitempty"`
	ICPNo          string       `json:"icpNo,omitempty"`
	AuditDate      string       `json:"auditDate,omitempty"`
	Source         string       `json:"source"`
	Confidence     Confidence   `json:"confidence"`
	Status         DomainStatus `json:"status"`
	EvidenceID     *string      `json:"evidenceId,omitempty"`
	DecisionReason string       `json:"decisionReason,omitempty"`
	DecidedBy      string       `json:"decidedBy,omitempty"`
	DecidedAt      *time.Time   `json:"decidedAt,omitempty"`
	ReassignedTo   *string      `json:"reassignedTo,omitempty"`
	FirstSeen      time.Time    `json:"firstSeen"`
	LastSeen       time.Time    `json:"lastSeen"`
	UpdatedAt      time.Time    `json:"updatedAt"`
	SchemaVersion  int          `json:"schemaVersion"`
	Lifecycle      `gorm:"embedded"`
}

func (d *OrganizationDomain) Validate() error {
	if d == nil || d.OrganizationID == "" {
		return errors.New("organization domain requires organizationId")
	}
	if strings.TrimSpace(d.RootDomain) == "" {
		return errors.New("organization domain requires rootDomain")
	}
	if strings.TrimSpace(d.Source) == "" {
		return errors.New("organization domain requires source")
	}
	if !d.Confidence.Valid() {
		return errors.New("invalid confidence")
	}
	if !d.Status.Valid() {
		return errors.New("invalid domain status")
	}
	return nil
}

type ICPEvidence struct {
	ID            string    `gorm:"primaryKey;size:24" json:"id"`
	Provider      string    `json:"provider"`
	QueryType     string    `json:"queryType"`
	QueryKey      string    `json:"queryKey"`
	Raw           string    `json:"raw"`
	FetchedAt     time.Time `json:"fetchedAt"`
	ExpiresAt     time.Time `json:"expiresAt"`
	PayloadHash   string    `json:"payloadHash"`
	SchemaVersion int       `json:"schemaVersion"`
}

type IPLocation struct {
	ID            string    `gorm:"primaryKey;size:24" json:"id"`
	IP            string    `json:"ip"`
	Lat           float64   `json:"lat"`
	Lon           float64   `json:"lon"`
	Country       string    `json:"country,omitempty"`
	Province      string    `json:"province,omitempty"`
	City          string    `json:"city,omitempty"`
	ASN           uint      `json:"asn,omitempty"`
	ASNOrg        string    `json:"asnOrg,omitempty"`
	Source        string    `json:"source"`
	UpdatedAt     time.Time `json:"updatedAt"`
	SchemaVersion int       `json:"schemaVersion"`
}

type ICPJob struct {
	ID             string       `gorm:"primaryKey;size:24" json:"id"`
	OrganizationID string       `json:"organizationId"`
	Status         ICPJobStatus `json:"status"`
	RequestedBy    string       `json:"requestedBy"`
	ResultCount    int          `json:"resultCount"`
	Error          string       `json:"error,omitempty"`
	CreatedAt      time.Time    `json:"createdAt"`
	FinishedAt     *time.Time   `json:"finishedAt,omitempty"`
	SchemaVersion  int          `json:"schemaVersion"`
}

type GraphOutbox struct {
	ID            string        `gorm:"primaryKey;size:24" json:"id"`
	EntityType    string        `json:"entityType"`
	EntityID      string        `json:"entityId"`
	Op            GraphOutboxOp `json:"op"`
	CreatedAt     time.Time     `json:"createdAt"`
	ProcessingAt  *time.Time    `json:"processingAt,omitempty"`
	ProcessedAt   *time.Time    `json:"processedAt,omitempty"`
	Attempts      int           `json:"attempts"`
	SchemaVersion int           `json:"schemaVersion"`
}

func (User) TableName() string               { return "users" }
func (Session) TableName() string            { return "sessions" }
func (APIKey) TableName() string             { return "api_keys" }
func (AuditEvent) TableName() string         { return "audit_events" }
func (Policy) TableName() string             { return "policies" }
func (Scope) TableName() string              { return "scopes" }
func (Task) TableName() string               { return "tasks" }
func (TaskStep) TableName() string           { return "task_steps" }
func (TaskEvent) TableName() string          { return "task_events" }
func (OutboxMessage) TableName() string      { return "outbox" }
func (Lease) TableName() string              { return "leases" }
func (Asset) TableName() string              { return "assets" }
func (AssetObservation) TableName() string   { return "asset_observations" }
func (ScopeAssetLink) TableName() string     { return "scope_asset_links" }
func (Finding) TableName() string            { return "findings" }
func (Monitor) TableName() string            { return "monitors" }
func (Schedule) TableName() string           { return "schedules" }
func (MonitorRun) TableName() string         { return "monitor_runs" }
func (GitHubTask) TableName() string         { return "github_tasks" }
func (GitHubMonitor) TableName() string      { return "github_monitors" }
func (GitHubFinding) TableName() string      { return "github_findings" }
func (ExportJob) TableName() string          { return "exports" }
func (PocPlugin) TableName() string          { return "poc_plugins" }
func (Organization) TableName() string       { return "organizations" }
func (OrganizationDomain) TableName() string { return "organization_domains" }
func (ICPEvidence) TableName() string        { return "icp_evidence" }
func (IPLocation) TableName() string         { return "ip_locations" }
func (ICPJob) TableName() string             { return "icp_jobs" }
func (GraphOutbox) TableName() string        { return "graph_outbox" }
