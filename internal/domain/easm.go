package domain

import (
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

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
	ID            bson.ObjectID     `bson:"_id,omitempty" json:"id"`
	CanonicalName string            `bson:"canonicalName" json:"canonicalName"`
	Aliases       []string          `bson:"aliases,omitempty" json:"aliases,omitempty"`
	USCC          string            `bson:"uscc,omitempty" json:"uscc,omitempty"`
	Region        string            `bson:"region,omitempty" json:"region,omitempty"`
	ExternalIDs   map[string]string `bson:"externalIds,omitempty" json:"externalIds,omitempty"`
	Status        OrgStatus         `bson:"status" json:"status"`
	MergedInto    *bson.ObjectID    `bson:"mergedInto,omitempty" json:"mergedInto,omitempty"`
	CreatedAt     time.Time         `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time         `bson:"updatedAt" json:"updatedAt"`
	SchemaVersion int               `bson:"schemaVersion" json:"schemaVersion"`
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
	ID             bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	OrganizationID bson.ObjectID  `bson:"organizationId" json:"organizationId"`
	RootDomain     string         `bson:"rootDomain" json:"rootDomain"`
	SiteName       string         `bson:"siteName,omitempty" json:"siteName,omitempty"`
	HomeSite       string         `bson:"homeSite,omitempty" json:"homeSite,omitempty"`
	ICPNo          string         `bson:"icpNo,omitempty" json:"icpNo,omitempty"`
	AuditDate      string         `bson:"auditDate,omitempty" json:"auditDate,omitempty"`
	Source         string         `bson:"source" json:"source"`
	Confidence     Confidence     `bson:"confidence" json:"confidence"`
	Status         DomainStatus   `bson:"status" json:"status"`
	EvidenceID     *bson.ObjectID `bson:"evidenceId,omitempty" json:"evidenceId,omitempty"`
	FirstSeen      time.Time      `bson:"firstSeen" json:"firstSeen"`
	LastSeen       time.Time      `bson:"lastSeen" json:"lastSeen"`
	UpdatedAt      time.Time      `bson:"updatedAt" json:"updatedAt"`
	SchemaVersion  int            `bson:"schemaVersion" json:"schemaVersion"`
}

func (d *OrganizationDomain) Validate() error {
	if d == nil || d.OrganizationID.IsZero() {
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
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Provider      string        `bson:"provider" json:"provider"`
	QueryType     string        `bson:"queryType" json:"queryType"`
	QueryKey      string        `bson:"queryKey" json:"queryKey"`
	Raw           string        `bson:"raw" json:"raw"`
	FetchedAt     time.Time     `bson:"fetchedAt" json:"fetchedAt"`
	ExpiresAt     time.Time     `bson:"expiresAt" json:"expiresAt"`
	PayloadHash   string        `bson:"payloadHash" json:"payloadHash"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type IPLocation struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	IP            string        `bson:"ip" json:"ip"`
	Lat           float64       `bson:"lat" json:"lat"`
	Lon           float64       `bson:"lon" json:"lon"`
	Country       string        `bson:"country,omitempty" json:"country,omitempty"`
	Province      string        `bson:"province,omitempty" json:"province,omitempty"`
	City          string        `bson:"city,omitempty" json:"city,omitempty"`
	ASN           uint          `bson:"asn,omitempty" json:"asn,omitempty"`
	ASNOrg        string        `bson:"asnOrg,omitempty" json:"asnOrg,omitempty"`
	Source        string        `bson:"source" json:"source"`
	UpdatedAt     time.Time     `bson:"updatedAt" json:"updatedAt"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}

type ICPJob struct {
	ID             bson.ObjectID `bson:"_id,omitempty" json:"id"`
	OrganizationID bson.ObjectID `bson:"organizationId" json:"organizationId"`
	Status         ICPJobStatus  `bson:"status" json:"status"`
	RequestedBy    string        `bson:"requestedBy" json:"requestedBy"`
	ResultCount    int           `bson:"resultCount" json:"resultCount"`
	Error          string        `bson:"error,omitempty" json:"error,omitempty"`
	CreatedAt      time.Time     `bson:"createdAt" json:"createdAt"`
	FinishedAt     *time.Time    `bson:"finishedAt,omitempty" json:"finishedAt,omitempty"`
	SchemaVersion  int           `bson:"schemaVersion" json:"schemaVersion"`
}

type GraphOutbox struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	EntityType    string        `bson:"entityType" json:"entityType"`
	EntityID      string        `bson:"entityId" json:"entityId"`
	Op            GraphOutboxOp `bson:"op" json:"op"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	ProcessingAt  *time.Time    `bson:"processingAt,omitempty" json:"processingAt,omitempty"`
	ProcessedAt   *time.Time    `bson:"processedAt,omitempty" json:"processedAt,omitempty"`
	Attempts      int           `bson:"attempts" json:"attempts"`
	SchemaVersion int           `bson:"schemaVersion" json:"schemaVersion"`
}
