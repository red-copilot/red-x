# Organization Graph Homepage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the generic dashboard with a project-scoped EASM workspace that combines organization/domain ownership, project-isolated asset inventory, GeoIP map data, and a Neo4j-backed relationship graph.

**Architecture:** Add Project, Organization, membership, domain-attribution, project-asset, GeoIP, project-event, background-job, and graph-outbox models in MongoDB. Keep MongoDB as the source of truth, project Neo4j into an isolated per-project subgraph, enqueue graph and GeoIP work through the existing queue/outbox system, and serve a Vue workspace whose graph and map share one selection/filter store. Explicit project membership gates every project-scoped endpoint; shared physical assets never imply shared project visibility.

**Tech Stack:** Go 1.27, MongoDB 8, Neo4j 5.x, MaxMind GeoIP2 City/ASN, Vue 3, TypeScript, Pinia, Ant Design Vue, ECharts, Playwright, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-19-organization-graph-homepage-design.md`

## Global Constraints

- Every project-scoped read or write must resolve the project from the URL and authorize it server-side; a client-supplied project id is never evidence of access.
- Global `admin` is the platform super-admin and may access every project. Every other interactive user must have a `project_members` record. API keys inherit the membership and write capability of their creating user.
- Asset membership in a project is the union of assets discovered by tasks carrying that `projectId` and assets reachable from confirmed root domains owned in that project. A shared Mongo asset is visible only through a project-asset link.
- Domain ownership, attribution, findings, and risk are project-scoped. Confirming the same domain for different organizations in two projects must not alter either project's facts.
- Attribution auto-confirm is disabled by default. The feature flag is `attribution.enabled=false`; when enabled, auto-confirm still requires an existing project organization and an explicitly configured policy. Unsupported provider calls must fail closed and leave attribution pending.
- Organization aliases are project-local, reversible records. Do not merge or rewrite global `organizations` records in v1.
- Organization coverage means the project has at least one confirmed root domain for that organization.
- Risk is the highest severity among open findings whose `createdAt` is within the selected window. `open` means not closed and not marked false-positive; scanner-produced findings are treated as confirmed unless an operator changes that lifecycle state. Default window is 90 days; supported values are `30d`, `90d`, and `all`. Severity order is `critical > high > medium > low > info`.
- External ICP lookup is disabled for the MVP. Do not implement live Chinaz API/Cookie calls; return a typed disabled response until credentials and fallback behavior are separately approved.
- GeoIP uses licensed MaxMind GeoIP2 City/ASN files mounted at runtime. A city point is emitted only when city confidence is at least 80; otherwise the IP is counted as unlocated. Private and reserved IPs never get map points.
- Raw attribution responses live in a separate collection and expire after 90 days. Project viewers receive redacted attribution DTOs; project operators/admins and super-admins may fetch raw data, and every raw fetch writes an audit event.
- Legacy `Scope` records remain unchanged. The opt-in `arl legacy-import-scope` command copies data; it never deletes or rewrites the legacy scope.
- Neo4j failure must not block task creation, scanning, findings, audit, or Mongo-backed list views. The graph API returns `graph.unavailable` and the UI falls back to the project asset list.
- Do not return raw JSON blocks as the primary asset presentation. Do not add 3D/decorative visualization libraries, scanner CLI wrappers, Python, or new runtime shell dependencies.
- Never commit API keys, cookies, Neo4j passwords, GeoIP license files, generated `internal/webui/dist`, test results, or secrets.

## Review Focus

- **Project id tampering:** A non-member supplies another project's id; every project, graph, map, search, inspector, asset, event, member, import, and task endpoint must return the same not-found/forbidden behavior without leaking names or counts.
- **API-key membership:** An API key whose creator belongs to project A must not access project B, and an admin-scoped API key created by a non-member must not inherit super-admin project access.
- **Shared asset leakage:** The same IP discovered in project A must not appear in project B unless project B's own task or confirmed-root-domain traversal produced a `project_assets` link.
- **Invalid status transitions:** Confirming a rejected attribution or rejecting a confirmed attribution must fail rather than silently changing ownership; alias deletion must not mutate organizations.
- **Map honesty:** Low-confidence city data, private IPs, missing GeoIP files, and Neo4j outages must produce explicit unlocated/fallback states rather than approximate or fabricated points.
- **Time window boundaries:** A finding at the exact 90-day cutoff and one second outside it must sort deterministically; timezone input must be converted to UTC before querying.

---

### Task 1: Domain Models, Indexes, and State Machines

**Files:**
- Create: `internal/domain/project.go`
- Create: `internal/domain/project_test.go`
- Modify: `internal/domain/models.go:188-300`
- Create: `internal/store/projects.go`
- Create: `internal/store/project_integration_test.go`
- Modify: `internal/store/store.go:10-170`

**Interfaces:**
- Consumes: existing `domain.NewModel[T]`, Mongo store conventions.
- Produces:
  - `domain.Project`, `domain.ProjectMember`, `domain.Organization`, `domain.ProjectOrganization`, `domain.ProjectDomain`, `domain.OrganizationAttribution`, `domain.AttributionRaw`, `domain.ProjectAlias`, `domain.ProjectAssetLink`, `domain.IPLocation`, `domain.BackgroundJob`, `domain.ProjectEvent`
  - `OrganizationAttribution.Transition(next AttributionStatus) error`
  - `domain.ProjectAssetLink.Sources []ProjectAssetSource`
  - `domain.ProjectAssetLink.LocationKind string` with values `asset`, `edge`, `origin`, `unlocated`
  - `domain.IPLocation` containing `ip, country, province, city, latitude, longitude, asn, cityConfidence, source, databaseVersion, dataUpdatedAt, updatedAt`
  - `store.ErrProjectNotFound`
  - `store.(*Store).CreateProject`, `GetProject`, `ListProjectsForUser`, `IsProjectMember`, `AddProjectMember`, `RemoveProjectMember`, `CreateOrganization`, `UpsertOrganization`, `AddProjectOrganization`, `AddProjectDomain`, `CreateAttribution`, `TransitionAttribution`, `UpsertIPLocation`, `CreateBackgroundJob`, `UpdateBackgroundJob`, `AppendProjectEvent`
  - `domain.Task.ProjectID/OrganizationID/RootDomainID`
  - `domain.Finding.ProjectID/Status`
  - `domain.FindingStatusOpen`, `FindingStatusClosed`, `FindingStatusFalsePositive`

- [ ] **Step 1: Write failing domain tests**

Create `internal/domain/project_test.go` with table tests named `TestAttributionTransition`, `TestRiskRank`, and `TestProjectAssetSourcesAreDeduplicated`. Assertions:

```go
func TestAttributionTransition(t *testing.T) {
	pending := OrganizationAttribution{Status: AttributionPending}
	if err := pending.Transition(AttributionConfirmed); err != nil { t.Fatal(err) }
	if err := pending.Transition(AttributionRejected); err == nil { t.Fatal("confirmed must not become rejected") }
}

func TestRiskRank(t *testing.T) {
	if RiskRank("critical") <= RiskRank("high") { t.Fatal("critical must outrank high") }
	if RiskRank("unknown") != 0 { t.Fatal("unknown severity must rank zero") }
}
```

The source test must merge duplicate `{Type:"task", RefID:"a"}` and `{Type:"root_domain", RefID:"b"}` entries while preserving both distinct refs.

- [ ] **Step 2: Run the tests and verify they fail**

Run: `go test ./internal/domain -run 'TestAttributionTransition|TestRiskRank|TestProjectAssetSourcesAreDeduplicated' -v`
Expected: FAIL because the project types and functions do not exist.

- [ ] **Step 3: Implement the domain types and state machine**

Use camelCase JSON/BSON names. Add these exact model fields and enums in `project.go`:

```go
type AttributionStatus string
const (
	AttributionPending AttributionStatus = "pending"
	AttributionConfirmed AttributionStatus = "confirmed"
	AttributionRejected AttributionStatus = "rejected"
)

type ProjectAssetSource struct {
	Type string `bson:"type" json:"type"` // task or root_domain
	RefID string `bson:"refId" json:"refId"`
}

type ProjectAssetLink struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID bson.ObjectID `bson:"projectId" json:"projectId"`
	AssetID bson.ObjectID `bson:"assetId" json:"assetId"`
	Kind string `bson:"kind" json:"kind"`
	Sources []ProjectAssetSource `bson:"sources" json:"sources"`
	FirstSeen time.Time `bson:"firstSeen" json:"firstSeen"`
	LastSeen time.Time `bson:"lastSeen" json:"lastSeen"`
	SchemaVersion int `bson:"schemaVersion" json:"schemaVersion"`
}
```

`Transition` permits `pending -> confirmed`, `pending -> rejected`, and same-to-same only. It rejects transitions out of confirmed/rejected. `RiskRank` returns 4,3,2,1 for critical/high/medium/low and zero otherwise. Add `ProjectID`, `OrganizationID`, `RootDomainID` as optional strings to `Task`; add `ProjectID string` and `Status FindingStatus` to `Finding`, defaulting new findings to `open`. Add internal `Project.NextEventSequence int64` for event ordering; it is never trusted from client JSON.

- [ ] **Step 4: Implement store methods and indexes**

Add collections `projects`, `project_members`, `organizations`, `project_organizations`, `project_domains`, `organization_attributions`, `attribution_raw`, `project_aliases`, `project_assets`, `ip_locations`, `background_jobs`, `project_events`. Add these unique indexes:

```text
projects: (normalizedName)
project_members: (projectId,userId)
organizations: (normalizedName)
project_organizations: (projectId,organizationId)
project_domains: (projectId,domain)
organization_attributions: (projectId,organizationId,domain,source,queryTime)
attribution_raw: (expiresAt) TTL
project_aliases: (projectId,aliasOrganizationId)
project_assets: (projectId,assetId)
ip_locations: (ip)
project_events: (projectId,sequence)
```

Add indexes for project task/finding scans: `tasks.(projectId,createdAt)`, `findings.(projectId,status,createdAt)`, and `observations.(taskId,assetId)`. Store methods must set timestamps/schema version through `NewModel`, normalize project/organization names with lowercase whitespace collapse, and always scope lookups by both project and child id. Add an integration test that inserts one project, member, organization, domain, attribution, and project event, then proves a lookup for project B does not return project A's rows. Add an integration assertion that `project_domains` defaults to `status="pending"`, has an optional `ownerOrganizationId`, and that only an explicitly confirmed record is counted by coverage.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./internal/domain ./internal/store`
Expected: PASS. If `ARL_TEST_MONGO_URI` is unset, the integration test reports `SKIP`; run `ARL_TEST_MONGO_URI=mongodb://127.0.0.1:27017 go test ./internal/store` when Mongo is available and expect PASS.

```bash
git add internal/domain/project.go internal/domain/project_test.go internal/domain/models.go internal/store/projects.go internal/store/project_integration_test.go internal/store/store.go
git commit -m "feat: add project and organization domain model"
```

### Task 2: Project Membership Authorization and Project API

**Files:**
- Create: `internal/httpapi/project_access.go`
- Create: `internal/httpapi/project_access_test.go`
- Create: `internal/httpapi/projects.go`
- Create: `internal/httpapi/projects_test.go`
- Modify: `internal/httpapi/server.go:35-150`
- Modify: `internal/modules/identity/service.go:20-235`
- Modify: `internal/httpapi/openapi.go`

**Interfaces:**
- Consumes: Task 1 store methods; `identity.Principal`.
- Produces:
  - `identity.Principal.OwnerID bson.ObjectID`
  - `type ProjectAccess struct { Project domain.Project; Member bool; CanWrite bool; CanViewRaw bool; SuperAdmin bool }`
  - `func (s *Server) resolveProjectAccess(ctx context.Context, projectID bson.ObjectID, p *identity.Principal) (*ProjectAccess, error)`
  - `func (s *Server) projectMiddleware(next http.Handler) http.Handler`
  - `func (s *Server) requireProjectWrite(next http.HandlerFunc) http.HandlerFunc`
  - `GET/POST /api/v1/projects`
  - `GET/PATCH /api/v1/projects/{id}`
  - `GET/POST/DELETE /api/v1/projects/{id}/members`
  - `GET /api/v1/projects/{id}/summary`

- [ ] **Step 1: Write failing access tests**

Create `project_access_test.go` with pure table tests for this decision matrix: super-admin non-member reads/writes; member viewer reads but cannot write/raw; member operator reads/writes; non-member denied; API key `OwnerID` member follows owner's role. Assert `CanViewRaw` is true only for super-admin or an operator/admin member.

- [ ] **Step 2: Run the tests and verify they fail**

Run: `go test ./internal/httpapi -run 'TestProjectAccessDecision|TestProjectRouteRequiresMembership|TestAPIKeyUsesOwnerRole' -v`
Expected: FAIL because `ProjectAccess` and the middleware do not exist.

- [ ] **Step 3: Implement API-key ownership and project authorization**

In `identity.Authenticate`, set `OwnerID: key.CreatedBy` for API keys. Use `OwnerID` as the membership subject. `resolveProjectAccess` loads the owner user and uses that user's current role for super-admin decisions; API-key scopes never elevate project visibility. Return `store.ErrProjectNotFound` for non-members to prevent project enumeration. Add this router shape; chi may register `/projects` and `/projects/{projectId}` directly instead of a slash-only group if route tests expose a 301:

```go
protected.Get("/projects", s.handleListProjects)
protected.Post("/projects", s.requireWrite(s.handleCreateProject))
protected.Route("/projects/{projectId}", func(one chi.Router) {
	one.Use(s.projectMiddleware)
	one.Get("/", s.handleGetProject)
	one.Patch("/", s.requireProjectWrite(s.handlePatchProject))
	one.Get("/members", s.handleListProjectMembers)
	one.Post("/members", s.requireAdmin(s.handleAddProjectMember))
	one.Delete("/members/{userId}", s.requireAdmin(s.handleRemoveProjectMember))
	one.Get("/summary", s.handleProjectSummary)
})
```

`GET /projects` returns only accessible projects for members and all active projects for a super-admin. Project creation makes the creator a member. Reject empty names and end dates before start dates. `PATCH` permits name, customer, description, start/end dates, and status. Add RFC3339 date pointers to the API DTOs; do not expose normalized names or internal ids not already needed by the client.

- [ ] **Step 4: Add handler tests and OpenAPI paths**

Test 200 access for member, 404 for non-member using a non-existent-looking project id, 403 for a viewer POST, and 201 for project creation. Test that a key with role `admin` created by a non-admin owner still cannot access an unrelated project, and that response JSON never includes another project's name. Extend the embedded OpenAPI JSON with project, member, and summary paths; keep `TestOpenAPISpecIsJSON` passing.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./internal/httpapi ./internal/modules/identity`
Expected: PASS.

```bash
git add internal/httpapi/project_access.go internal/httpapi/project_access_test.go internal/httpapi/projects.go internal/httpapi/projects_test.go internal/httpapi/server.go internal/httpapi/openapi.go internal/modules/identity/service.go
git commit -m "feat: add project membership authorization"
```

### Task 3: Asynchronous Organization CSV Import

**Files:**
- Create: `internal/modules/projects/importer.go`
- Create: `internal/modules/projects/importer_test.go`
- Create: `internal/httpapi/project_imports.go`
- Create: `internal/httpapi/project_imports_test.go`
- Modify: `internal/store/projects.go`
- Modify: `internal/platform/jobs/jobs.go:14-27`
- Modify: `internal/app/app.go`
- Modify: `cmd/arl/main.go:120-150`

**Interfaces:**
- Consumes: Tasks 1-2 project access and store.
- Produces:
  - `projects.Parser.ParseCSV(io.Reader) ([]ImportRow, ParseReport, error)`
  - `projects.Importer.Run(ctx context.Context, projectID bson.ObjectID, jobID bson.ObjectID) error`
  - `store.(*Store).CreateBackgroundJob`, `GetBackgroundJob`, `UpdateBackgroundJob`
  - `POST /api/v1/projects/{id}/organizations/import`
  - `GET /api/v1/projects/{id}/organizations`
  - `GET /api/v1/projects/{id}/jobs/{jobId}`
  - queue `q.project`

- [ ] **Step 1: Write failing parser tests**

Test `organizationName,domains` with `四川吉利学院;"guc.edu.cn;example.edu.cn"`, Chinese aliases `单位名称,主域名`, blank organization, malformed quoted CSV, duplicate rows, URL-like domains, and uppercase/trailing-dot domains. Expected: trim/lowercase domains, strip schemes/paths, reject non-domain values, accept at least one domain per row, and report row numbers for every rejection.

- [ ] **Step 2: Run parser tests and verify they fail**

Run: `go test ./internal/modules/projects -run TestParseCSV -v`
Expected: FAIL because package `projects` does not exist.

- [ ] **Step 3: Implement the parser and async job**

`ImportRow` is `{OrganizationName string; Domains []string}`. `ParseReport` contains `AcceptedRows`, `RejectedRows`, `Warnings`. Make import idempotent through the unique project-organization and project-domain indexes. A parsed row creates or reuses a global organization, adds a project organization, adds project domains, and creates one pending `source="csv_import"` attribution per domain. It does not confirm ownership. Upload endpoint accepts multipart field `file`, max 10 MiB, creates a `BackgroundJob{Kind:"organization_import", Status:"queued"}`, stores rows in the job payload, publishes to `q.project`, and returns `202` with the job id.

Update the worker callback to dispatch `q.project` to the importer. Update job progress after each organization and preserve partial success; a duplicate is counted as `skipped`, not an error. Add an importer test using a fake job/store interface that proves a failed domain lookup leaves previously imported organizations and job progress intact. Add `GET /projects/{id}/organizations` returning each project organization, its domains, alias ids, pending/confirmed counts, and coverage flag; it must never return organizations from another project.

- [ ] **Step 4: Add handler and integration tests**

Test non-member upload/list returns 404, viewer upload returns 403, multipart over 10 MiB returns 413, valid upload returns 202, and job polling is project-scoped. With `ARL_TEST_MONGO_URI`, import the same CSV twice and assert one organization, two domains, and two pending attributions per domain at the end of the first run and unchanged counts after the second.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./internal/modules/projects ./internal/httpapi ./internal/platform/jobs`
Expected: PASS.

```bash
git add internal/modules/projects internal/httpapi/project_imports.go internal/httpapi/project_imports_test.go internal/store/projects.go internal/platform/jobs/jobs.go internal/app/app.go cmd/arl/main.go
git commit -m "feat: add asynchronous organization imports"
```

### Task 4: Manual Attribution, Aliases, and Raw-Response Access

**Files:**
- Create: `internal/modules/attribution/service.go`
- Create: `internal/modules/attribution/service_test.go`
- Create: `internal/store/attribution.go`
- Create: `internal/httpapi/attribution.go`
- Create: `internal/httpapi/attribution_test.go`
- Modify: `internal/domain/project.go`
- Modify: `internal/store/store.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/platform/config/config.go`
- Modify: `config.example.yaml`

**Interfaces:**
- Consumes: Tasks 1-3.
- Produces:
  - `attribution.Service.Confirm(ctx, projectID, orgID, attributionID, actor) error`
  - `attribution.Service.Reject(...)`
  - `attribution.Service.AddAlias(ctx, projectID, canonicalID, aliasID, actor) error`
  - `attribution.Service.RemoveAlias(...)`
  - `store.(*Store).CreateAlias`, `ListAliases`, `DeleteAlias`, `GetAttribution`, `GetRawAttribution`
  - `POST /api/v1/projects/{id}/organizations/{orgId}/attributions/{attributionId}/confirm`
  - `POST /api/v1/projects/{id}/organizations/{orgId}/attributions/{attributionId}/reject`
  - `GET /api/v1/projects/{id}/attributions/{attributionId}/raw`
  - `POST/DELETE /api/v1/projects/{id}/organizations/{orgId}/aliases`
  - `POST /api/v1/organizations/lookup` returning `503 attribution.disabled` while the feature flag is off

- [ ] **Step 1: Write failing state and alias tests**

Test confirm creates exactly one project-scoped owner fact and sets `confirmedAt/confirmedBy`; reject retains the attribution row and creates no owner fact; repeat confirm is idempotent; confirmed-to-rejected fails; alias add/delete is reversible and does not modify the global organization. Test raw access rejects viewers and records `attribution.raw_view` on success.

- [ ] **Step 2: Run the tests and verify they fail**

Run: `go test ./internal/modules/attribution ./internal/httpapi -run 'TestConfirmAttribution|TestAlias|TestRawAttribution' -v`
Expected: FAIL because the attribution service and handlers do not exist.

- [ ] **Step 3: Implement attribution state and local aliases**

Add `attribution.enabled: false`, `attribution.autoConfirm: false`, and provider credential fields to config but do not call the provider. `lookup` returns 503 with problem code `attribution.disabled`. Manual confirm/reject always works. Confirmation updates only the current project's `project_domains.ownerOrganizationId` and `organization_attributions.status`; no global organization ownership field exists.

Store raw evidence separately in `attribution_raw` with `{attributionId, projectId, contentType, body, expiresAt}` and a TTL index on `expiresAt`. The list/detail DTO omits `body` unless `CanViewRaw`; raw fetch writes an audit event containing ids and actor only, never the body. Aliases use `project_aliases{projectId, canonicalOrganizationId, aliasOrganizationId, createdBy, createdAt}` with a unique project-scoped pair.

- [ ] **Step 4: Add handler tests**

Test a member operator confirm/reject, viewer confirm returns 403, cross-project attribution id returns 404, raw fetch for viewer returns redacted DTO without `body`, and operator raw fetch writes one audit event. Test duplicate alias returns 409 and self-alias returns 400.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./internal/modules/attribution ./internal/httpapi ./internal/platform/config`
Expected: PASS.

```bash
git add internal/modules/attribution internal/store/attribution.go internal/httpapi/attribution.go internal/httpapi/attribution_test.go internal/domain/project.go internal/store/store.go internal/httpapi/server.go internal/platform/config/config.go config.example.yaml
git commit -m "feat: add project attribution workflow"
```

### Task 5: Project Asset Projection and Task Tagging

**Files:**
- Create: `internal/store/project_assets.go`
- Create: `internal/store/project_assets_integration_test.go`
- Create: `internal/modules/tasking/project_assets.go`
- Create: `internal/modules/tasking/project_assets_test.go`
- Modify: `internal/modules/tasking/engine.go:35-180`
- Modify: `internal/modules/tasking/steps.go:180-440`
- Modify: `internal/store/inventory.go:15-75`
- Modify: `internal/httpapi/tasks.go:15-60`
- Modify: `internal/domain/models.go:188-355`

**Interfaces:**
- Consumes: Tasks 1-4.
- Produces:
  - `store.(*Store).LinkProjectAsset(ctx, projectID, asset, source) error`
  - `store.(*Store).ListProjectAssets(ctx, projectID, kind string, q PageQuery)`
  - `store.(*Store).RebuildProjectAssets(ctx, projectID bson.ObjectID) error`
  - `tasks.SubmitInput.ProjectID/OrganizationID/RootDomainID`
  - `POST /api/v1/tasks` accepts and validates the three optional context fields
  - `GET /api/v1/projects/{id}/assets`

- [ ] **Step 1: Write failing projection tests**

Add `TestProjectAssetIsolation`, `TestProjectAssetSourceUnion`, `TestTaskProjectContextValidation`, and `TestProjectScale`. The isolation test creates the same global IP asset with observation/task A and task B, then asserts `ListProjectAssets(projectA)` returns one row and project B's service/finding ids are absent. The union test creates a link sourced by both task and root domain and asserts one row with two sources. Validation test rejects an unconfirmed root domain, a root domain from another project, and a task target outside that root domain. The scale test inserts 100 organizations, 300 confirmed root domains, and 5,000 linked assets, then requires the first project list page and summary query to return in under 5 seconds.

- [ ] **Step 2: Run the tests and verify they fail**

Run: `go test ./internal/modules/tasking ./internal/store -run 'TestProjectAsset|TestTaskProjectContext' -v`
Expected: FAIL because project asset linking does not exist.

- [ ] **Step 3: Implement project asset links**

Change `UpsertAsset` to accept `projectID string` as its final argument and call `LinkProjectAsset` after the global asset upsert. When `projectID` is empty, preserve legacy behavior. `LinkProjectAsset` upserts by `(projectId,assetId)`, adds a deduplicated source, updates `lastSeen`, and creates a `ProjectEvent{Type:"asset.discovered"}` only when the link is newly inserted.

For project tasks, derive `projectID`, `organizationID`, and `rootDomainID` from the validated request. A task with `rootDomainID` must target that root domain or one of its subdomains. Propagate these fields through every `UpsertAsset` call in `steps.go`; findings inherit `ProjectID`. After a task succeeds, call `RebuildProjectAssets(projectID)` so a task replay reconstructs the same links. `RebuildProjectAssets` deletes only that project's links, scans project tasks and observations, then traverses confirmed project domains and adds root-domain sources.

Add `GET /projects/{id}/assets?kind=&q=&page=&pageSize=` using the project link collection joined to global assets. Never expose an asset not linked to the authorized project.

- [ ] **Step 4: Run Mongo and task tests**

Run: `go test ./internal/modules/tasking ./internal/store ./internal/httpapi`
Expected: PASS. Then run `ARL_TEST_MONGO_URI=mongodb://127.0.0.1:27017 go test ./internal/store -run 'TestProjectAsset|TestProjectScale' -v` and expect PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/project_assets.go internal/store/project_assets_integration_test.go internal/modules/tasking/project_assets.go internal/modules/tasking/project_assets_test.go internal/modules/tasking/engine.go internal/modules/tasking/steps.go internal/store/inventory.go internal/httpapi/tasks.go internal/domain/models.go
git commit -m "feat: project-scope discovered assets"
```

### Task 6: Project Risk Summary, Events, and SSE

**Files:**
- Create: `internal/store/project_queries.go`
- Create: `internal/store/project_queries_integration_test.go`
- Create: `internal/httpapi/project_events.go`
- Create: `internal/httpapi/project_events_test.go`
- Modify: `internal/httpapi/projects.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/modules/tasking/steps.go`
- Modify: `internal/store/inventory.go`
- Modify: `internal/modules/attribution/service.go`

**Interfaces:**
- Consumes: Tasks 1-5.
- Produces:
  - `store.(*Store).ProjectSummary(ctx, projectID, since time.Time) (domain.ProjectSummary, error)`
  - `store.(*Store).ListProjectEvents(ctx, projectID, after int64, limit int)`
  - `store.(*Store).AppendProjectEvent(ctx, *domain.ProjectEvent)`
  - `GET /api/v1/projects/{id}/summary?window=30d|90d|all`
  - `GET /api/v1/projects/{id}/events?after=`

- [ ] **Step 1: Write failing summary tests**

Test coverage with one confirmed and one pending domain; risk with open critical, open high, closed critical, and a finding 91 days old; and project isolates identical finding titles. Assert severity ranking, `window=all`, and exact 90-day boundary behavior in UTC.

Test project event sequence with two concurrent appends produces unique increasing sequences. Test SSE starts after `Last-Event-ID`, emits `asset.discovered` and `finding.created`, and sends heartbeats without terminating the stream.

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./internal/store ./internal/httpapi -run 'TestProjectSummary|TestProjectEvents|TestProjectSSE' -v`
Expected: FAIL because summary and event functions do not exist.

- [ ] **Step 3: Implement summary and event persistence**

`ProjectSummary` includes `organizationCount`, `coveredOrganizationCount`, `confirmedRootDomainCount`, `pendingAttributionCount`, `assetCount`, `unlocatedAssetCount`, `riskCounts`, and `highestRisk`. Compute coverage only from `project_domains.status="confirmed"` joined to project organizations. Use a Mongo aggregation over `findings` filtered by `projectId`, `status="open"`, and `createdAt>=since`, grouping by severity and taking the max.

Append a project event after a newly linked asset and after a newly inserted finding. Use `findOneAndUpdate` on `projects` with `$inc:{nextEventSequence:1}` to allocate the sequence, then insert `project_events`. Emit `task.status`, `asset.discovered`, `finding.created`, `attribution.pending`, and `attribution.confirmed`. Implement the SSE endpoint using a 2-second poll and a 15-second heartbeat, matching the existing task SSE behavior.

- [ ] **Step 4: Add authorization and invalid-window tests**

Test non-member summary/events return 404, viewer can read, unsupported window returns 400, and `after` replay does not duplicate events. Update the project OpenAPI paths.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./internal/store ./internal/httpapi ./internal/modules/tasking`
Expected: PASS.

```bash
git add internal/store/project_queries.go internal/store/project_queries_integration_test.go internal/httpapi/project_events.go internal/httpapi/project_events_test.go internal/httpapi/projects.go internal/httpapi/server.go internal/modules/tasking/steps.go internal/store/inventory.go
git commit -m "feat: add project summary and event stream"
```

### Task 7: GeoIP Resolution and Map API

**Files:**
- Create: `internal/modules/geoip/resolver.go`
- Create: `internal/modules/geoip/resolver_test.go`
- Create: `internal/store/geo.go`
- Create: `internal/httpapi/project_map.go`
- Create: `internal/httpapi/project_map_test.go`
- Modify: `internal/platform/config/config.go:20-65`
- Modify: `internal/platform/config/config.go:200-360`
- Modify: `internal/app/app.go`
- Modify: `internal/modules/tasking/steps.go`
- Modify: `internal/store/inventory.go`
- Modify: `internal/platform/jobs/jobs.go`
- Modify: `cmd/arl/main.go`
- Modify: `config.example.yaml`

**Interfaces:**
- Consumes: Tasks 5-6 project assets and events.
- Produces:
  - `geoip.Resolver.Lookup(ctx context.Context, ip netip.Addr) (*domain.IPLocation, error)`
  - `geoip.Resolver.LookupPublic(ip string) (*domain.IPLocation, error)`
  - `store.(*Store).UpsertIPLocation`
  - queue `q.geo`
  - `GET /api/v1/projects/{id}/map?bbox=&kind=&risk=&window=&types=`
  - `MapDTO` with `points`, `clusters`, `unlocatedCount`, `degraded`

- [ ] **Step 1: Write failing resolver tests**

Test private, loopback, link-local, public with city confidence 79, public with confidence 80, missing country, ASN-only, and invalid IP. Expected: private/reserved error; confidence below 80 returns an unlocated result with no lat/lon; confidence 80 keeps country/province/city and coordinates; ASN-only still stores ASN but no map point. Use a fake `LookupFunc`; do not require a licensed database in unit tests.

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./internal/modules/geoip -run TestResolver -v`
Expected: FAIL because the geoip package does not exist.

- [ ] **Step 3: Implement MaxMind resolver and asynchronous enrichment**

Add `github.com/oschwald/maxminddb-golang/v2`. Add config:

```yaml
geoip:
  city: "/data/geoip/GeoLite2-City.mmdb"
  asn: "/data/geoip/GeoLite2-ASN.mmdb"
  cityConfidenceThreshold: 80
```

When enabled, open both files at startup; if either is missing, log a warning and set the resolver to `Unavailable`, not fail app startup. On an IP asset upsert with a project context, append an outbox message for `q.geo` containing project id and asset id. The worker looks up the public IP once, upserts `ip_locations` by IP, emits `asset.located`, and ignores duplicate queue deliveries idempotently. Add startup diagnostics for DB version and file modification time without logging license contents.

- [ ] **Step 4: Implement and test the map endpoint**

The map endpoint must authorize project membership, load only linked project IP assets, join `ip_locations`, filter to non-empty lat/lon, and return `unlocatedCount` for linked IPs without valid points. `kind=asset|edge|origin` comes from `ProjectAssetLink.Kind`; v1 may emit `edge` from existing CDN observations and `origin` only when a future relation exists. Group by country/province/city into deterministic clusters above 200 points; each point has `id,label,kind,lat,lon,country,province,city,risk,assetCount,confidence`. If GeoIP is unavailable, return 200 with `degraded:"geoip_unavailable"`, empty points, and the true unlocated count.

Test that a private IP increments `unlocatedCount`, a low-confidence city is omitted, and a project B asset cannot appear in project A's map.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./internal/modules/geoip ./internal/httpapi ./internal/platform/config ./internal/store`
Expected: PASS.

```bash
git add go.mod go.sum internal/modules/geoip internal/store/geo.go internal/httpapi/project_map.go internal/httpapi/project_map_test.go internal/platform/config/config.go internal/app/app.go internal/modules/tasking/steps.go internal/store/inventory.go internal/platform/jobs/jobs.go cmd/arl/main.go config.example.yaml
git commit -m "feat: add GeoIP enrichment and project map API"
```

### Task 8: Neo4j Projection, Outbox, Rebuild, and Verification

**Files:**
- Create: `internal/modules/graph/derive.go`
- Create: `internal/modules/graph/derive_test.go`
- Create: `internal/modules/graph/repository.go`
- Create: `internal/modules/graph/projector.go`
- Create: `internal/modules/graph/projector_test.go`
- Create: `internal/modules/graph/neo4j.go`
- Create: `internal/modules/graph/neo4j_integration_test.go`
- Create: `internal/modules/graph/rebuild.go`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/jobs/jobs.go`
- Modify: `internal/app/app.go`
- Modify: `internal/modules/tasking/engine.go`
- Modify: `cmd/arl/main.go`
- Modify: `deploy/compose.yaml`
- Modify: `deploy/config.docker.yaml`
- Modify: `config.example.yaml`

**Interfaces:**
- Consumes: Tasks 5-7; Mongo project links, assets, findings, attributions.
- Produces:
  - `graph.RelationDeriver.Derive(asset domain.Asset, related []domain.Asset) (nodes []Node, edges []Edge)`
  - `graph.Repository.UpsertProjectNode`, `UpsertEdge`, `ReplaceProject`, `Graph`, `Reconcile`
  - `graph.Projector.Handle(ctx, delivery jobs.Delivery) error`
  - `graph.Rebuilder.RebuildProject(ctx, projectID) error`
  - queue `q.graph`
  - `arl graph rebuild --config PATH [--project-id ID]`
  - `arl graph verify --config PATH --project-id ID`

- [ ] **Step 1: Write failing pure derivation tests**

Cover domain with direct IPs, CDN domain with edge IPs and CDN provider, domain-to-IP, IP-to-service, IP-to-site, site-to-URL, cert-to-domain, finding-to-asset, and task-to-asset. Assert every node/edge has project id, stable deterministic ids, and source/confidence fields. Add a case where two projects reference the same IP and assert their derived node ids differ by project namespace.

- [ ] **Step 2: Run derivation tests and verify they fail**

Run: `go test ./internal/modules/graph -run TestDerive -v`
Expected: FAIL because the graph package does not exist.

- [ ] **Step 3: Implement Neo4j repository and config**

Add `github.com/neo4j/neo4j-go-driver/v5`. Config fields:

```yaml
graph:
  enabled: true
  uri: "bolt://neo4j:7687"
  username: "neo4j"
  password: ""
```

Create constraints for `Project`, `Organization`, `Domain`, `IP`, `Service`, `Site`, `URL`, `Certificate`, `CDNProvider`, `Finding`, and `Task` on `(projectId,entityId)`. Create indexes on node id/type and relationship projectId. Use `MERGE` with stable ids and `SET` properties so duplicate delivery is idempotent. If Neo4j is unreachable at startup, log `graph disabled` and continue; never fail `app.New`. Add a Neo4j service with health check and persistent volume to `deploy/compose.yaml`; read its password from an environment override and do not put a production secret in git.

- [ ] **Step 4: Wire asset/finding/attribution events to the projector**

Add `q.graph` and include it in worker consumption. On a successful project asset link, finding insert, or attribution confirmation, append an outbox message for `q.graph`. Projector loads only the project and entity from Mongo, derives the bounded neighborhood, and upserts it. On failure return the error so the existing queue retry/DLQ logic applies. Do not publish a graph event before the Mongo write commits.

Implement `ReplaceProject` as: verify project exists; delete all nodes/edges with `projectId`; rebuild organization/domain/attribution nodes; stream assets and findings in batches; derive edges; then write. `RebuildProject` must be safe to rerun. `verify` runs Mongo count and Neo4j count queries for project assets, findings, organizations, and edges, returning nonzero if counts differ.

- [ ] **Step 5: Add integration tests and commit**

Run: `go test ./internal/modules/graph ./internal/app ./cmd/arl`
Expected: PASS. With a live Neo4j, run `ARL_TEST_NEO4J_URI=bolt://127.0.0.1:7687 ARL_TEST_NEO4J_USER=neo4j ARL_TEST_NEO4J_PASSWORD=test go test ./internal/modules/graph -run Neo4j -v` and expect idempotent duplicate writes and matching rebuild/verify counts.

```bash
git add go.mod go.sum internal/modules/graph internal/platform/config/config.go internal/platform/jobs/jobs.go internal/app/app.go internal/modules/tasking/engine.go cmd/arl/main.go deploy/compose.yaml deploy/config.docker.yaml config.example.yaml
git commit -m "feat: project Mongo graph into Neo4j"
```

### Task 9: Project Graph and Degraded List API

**Files:**
- Create: `internal/httpapi/project_graph.go`
- Create: `internal/httpapi/project_graph_test.go`
- Create: `internal/store/project_graph_fallback.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/openapi.go`
- Modify: `internal/modules/graph/repository.go`

**Interfaces:**
- Consumes: Tasks 5-8.
- Produces:
  - `GET /api/v1/projects/{id}/graph?focus=&organizationId=&city=&depth=&types=&risk=&window=&limit=`
  - `GET /api/v1/projects/{id}/list?kind=&risk=&window=&q=&organizationId=&city=&page=`
  - `GET /api/v1/projects/{id}/inspector?entityType=&entityId=`
  - `GraphDTO{Nodes []GraphNode; Edges []GraphEdge; Truncated bool; Aggregated bool; Limit int}`
  - `ListDTO` for the degraded homepage and global project search.
  - `InspectorDTO` with entity summary, attribution, evidence references, linked tasks, and linked assets.

- [ ] **Step 1: Write failing graph handler tests**

Test default depth 2 and limit 300; depth above 3 clamps to 3; limit above 500 clamps to 500; unknown type returns 400; risk `critical` returns only critical-or-higher nodes; city and organization filters apply to both graph and map; non-member returns 404; Neo4j disabled returns 503 with code `graph.unavailable`; project B nodes never appear in project A. Test list fallback returns project-linked assets and honors kind/risk/window/search. Test inspector rejects an entity id from project B and redacts raw attribution from viewers.

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./internal/httpapi -run 'TestProjectGraph|TestProjectListFallback' -v`
Expected: FAIL because graph routes do not exist.

- [ ] **Step 3: Implement bounded project graph queries**

Always start Cypher from the authorized project node matched by `(projectId:{id})`; do not accept arbitrary node ids without also matching `projectId`. Apply organization, city, type, and risk filters before `LIMIT`. With no risk filter, first return high-risk organizations/root domains and their immediately connected nodes; include lower-risk direct neighbors without labeling them high risk. If the requested focus node is not in the project, return 404. Emit `truncated=true` when the raw count exceeds the limit and aggregate high-degree nodes by organization/domain before returning. Populate `geo` only from the project's `ip_locations`; never use organization registration addresses.

Map the repository result to the DTO with no Mongo asset ids that are not visible in that project. Include an opaque project-scoped `entityId` for inspector lookup. Add `GET /projects/{id}/list` using only `project_assets`, including asset kind/key, risk, first/last seen, and location/unlocated state; the same endpoint powers the project search box and fallback. Add `GET /projects/{id}/inspector`; it verifies the entity belongs to the project before returning organization/domain/IP/service/site/URL/cert/finding/task evidence. Update OpenAPI.

- [ ] **Step 4: Run tests and commit**

Run: `go test ./internal/httpapi ./internal/modules/graph ./internal/store`
Expected: PASS.

```bash
git add internal/httpapi/project_graph.go internal/httpapi/project_graph_test.go internal/store/project_graph_fallback.go internal/httpapi/server.go internal/httpapi/openapi.go internal/modules/graph/repository.go
git commit -m "feat: add bounded project graph and fallback API"
```

### Task 10: Vue Project Shell, Selection, Members, and Import

**Files:**
- Modify: `web/package.json`
- Modify: `web/package-lock.json`
- Create: `web/src/types/project.ts`
- Create: `web/src/stores/project.ts`
- Create: `web/src/layouts/ProjectLayout.vue`
- Create: `web/src/pages/ProjectPickerPage.vue`
- Create: `web/src/pages/ProjectHomePage.vue`
- Create: `web/src/components/project/OrganizationRail.vue`
- Create: `web/src/components/project/ProjectMembersModal.vue`
- Create: `web/src/components/project/ImportOrganizationsModal.vue`
- Create: `web/src/components/project/ProjectEmptyState.vue`
- Create: `web/src/components/project/ProjectErrorState.vue`
- Create: `web/src/components/project/ProjectGraphFallback.vue`
- Create: `web/src/components/project/BatchScanDrawer.vue`
- Modify: `web/src/router.ts`
- Modify: `web/src/api.ts`
- Modify: `web/src/layouts/AppLayout.vue`

**Interfaces:**
- Consumes: Tasks 2-6 and 9 API DTOs.
- Produces:
  - routes `/projects`, `/projects/:projectId`, `/projects/:projectId/organizations`
  - Pinia `useProjectStore()` with `projects`, `current`, `filters`, `selection`, `summary`, `assets`, `loading`, `degraded`
  - shared filter keys `organizationIds`, `risk`, `window`, `types`, `mapCity`, `selectionId`
  - `api` methods for project list/create/get/update/members/summary/assets/import/job/events

- [ ] **Step 1: Write failing frontend tests**

Add Vitest tests for project-store filter invariants, last-project persistence, role-based member controls, import validation, and empty/error rendering. Assert that a viewer cannot see confirm/import/member/scan mutation controls, that a batch scan refuses unconfirmed domains, and that selecting a map city sets `mapCity` without clearing the risk/window filters.

- [ ] **Step 2: Run tests and verify they fail**

Run: `cd web && npm test -- --run`
Expected: FAIL because the project store and components do not exist.

- [ ] **Step 3: Implement the project workspace shell**

Add `echarts` and `@types/echarts` only if required by the installed ECharts version. Use `ProjectLayout` with a top project bar, 236px organization rail, central workspace, 340px right rail, and bottom event ribbon; use the provided design tokens exactly. `ProjectPickerPage` lists accessible projects and has a create action. On login, route to `/projects`; redirect to the last accessible project from `localStorage` only after verifying it is returned by `GET /projects`. If no project is accessible, show the picker, not an empty dashboard.

Use `GET /projects/{id}/summary` for coverage/risk. Build an SSE connection to `/projects/{id}/events` with replay via `Last-Event-ID`; fall back to polling every 5 seconds if EventSource errors. The import modal accepts `.csv`, displays accepted/rejected row counts from the job, and polls the job endpoint. The members modal is visible to super-admin and permits add/remove only; viewer/operator membership does not grant member-management rights.

`BatchScanDrawer` lets an operator select confirmed project domains or organizations and posts one `/tasks` request per domain with `projectId`, `organizationId`, and `rootDomainId`. It reports per-domain success/failure and never submits an unconfirmed domain. The top search box queries `/projects/{id}/list?q=` and opens the inspector for a selected result.

- [ ] **Step 4: Run typecheck and commit**

Run: `cd web && npm test -- --run && npm run typecheck`
Expected: PASS.

```bash
git add web/package.json web/package-lock.json web/src/types/project.ts web/src/stores/project.ts web/src/layouts/ProjectLayout.vue web/src/pages/ProjectPickerPage.vue web/src/pages/ProjectHomePage.vue web/src/components/project/OrganizationRail.vue web/src/components/project/ProjectMembersModal.vue web/src/components/project/ImportOrganizationsModal.vue web/src/components/project/BatchScanDrawer.vue web/src/components/project/ProjectEmptyState.vue web/src/components/project/ProjectErrorState.vue web/src/components/project/ProjectGraphFallback.vue web/src/router.ts web/src/api.ts web/src/layouts/AppLayout.vue
git commit -m "feat: add project workspace frontend shell"
```

### Task 11: ECharts Graph, Map, Inspector, and Shared Filters

**Files:**
- Create: `web/src/components/project/GraphCanvas.vue`
- Create: `web/src/components/project/MapPanel.vue`
- Create: `web/src/components/project/InspectorPanel.vue`
- Create: `web/src/components/project/EventRibbon.vue`
- Create: `web/src/components/project/RiskFilter.vue`
- Create: `web/src/components/project/GraphLegend.vue`
- Create: `web/src/components/project/GraphCanvas.test.ts`
- Create: `web/src/components/project/MapPanel.test.ts`
- Create: `web/src/assets/geo/china.json`
- Create: `web/src/assets/geo/world.json`
- Modify: `web/src/pages/ProjectHomePage.vue`
- Modify: `web/src/stores/project.ts`
- Modify: `web/src/styles.css`

**Interfaces:**
- Consumes: Task 10 shell/store and Task 9 DTOs.
- Produces:
  - `GraphCanvas` emits `nodeSelected` and `backgroundSelected`
  - `MapPanel` emits `citySelected`, `pointSelected`, and `mapReady`
  - `InspectorPanel` tabs `overview`, `evidence`, `tasks`, `attribution`, `assets`
  - filter/selection behavior shared only through `useProjectStore()`

- [ ] **Step 1: Write failing interaction tests**

Mount `GraphCanvas` and `MapPanel` with mocked ECharts instances. Assert initial graph call uses `limit=300`, map click writes `mapCity` into the store, graph click writes `selectionId` and calls map focus, changing risk/window refetches both graph and map, and Neo4j unavailable switches the center panel to `ProjectGraphFallback` while retaining the map and inspector.

- [ ] **Step 2: Run tests and verify they fail**

Run: `cd web && npm test -- --run GraphCanvas MapPanel`
Expected: FAIL because the components do not exist.

- [ ] **Step 3: Implement graph/map/inspector**

Use ECharts `graph` with category colors only for unit/domain/IP/service/risk and labels only for high-risk or selected nodes. Default request is `window=90d`, depth 2, limit 300. Map defaults to China, switches to world, uses `china.json`/`world.json`, clusters above 200 visible points, and renders asset/edge/origin with distinct but restrained symbols. Never place private or unlocated IPs at a fake coordinate; show the unlocated count in the map header.

Clicking a city sets the shared `mapCity` filter and refetches the graph; clicking a graph node sets `selectionId` and the map focuses that node if it has a valid location. The inspector fetches the selected entity only from a project-scoped endpoint, displays source/confidence/evidence, and links to tasks/assets. The event ribbon consumes the shared SSE stream and never becomes the primary visual. Update the design tokens in `styles.css`: light background, white panels, low-contrast borders, one dark blue-gray accent, red/orange/blue reserved for risk. Remove gradients from the project layout.

- [ ] **Step 4: Run frontend tests and typecheck**

Run: `cd web && npm test -- --run && npm run typecheck && npm run build`
Expected: PASS. Verify the ECharts map assets are loaded locally and no network map fetch occurs.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/project/GraphCanvas.vue web/src/components/project/MapPanel.vue web/src/components/project/InspectorPanel.vue web/src/components/project/EventRibbon.vue web/src/components/project/RiskFilter.vue web/src/components/project/GraphLegend.vue web/src/components/project/GraphCanvas.test.ts web/src/components/project/MapPanel.test.ts web/src/assets/geo/china.json web/src/assets/geo/world.json web/src/pages/ProjectHomePage.vue web/src/stores/project.ts web/src/styles.css
git commit -m "feat: add project graph and map workspace"
```

### Task 12: Project Batch Export and Evidence Download

**Files:**
- Create: `internal/modules/reporting/project_exporter.go`
- Create: `internal/modules/reporting/project_exporter_test.go`
- Create: `internal/httpapi/project_exports.go`
- Create: `internal/httpapi/project_exports_test.go`
- Modify: `internal/modules/reporting/exporter.go`
- Modify: `internal/domain/models.go`
- Modify: `internal/store/project_queries.go`
- Modify: `internal/httpapi/server.go`
- Modify: `web/src/components/project/OrganizationRail.vue`
- Modify: `web/src/pages/ProjectHomePage.vue`
- Modify: `web/src/api.ts`

**Interfaces:**
- Consumes: Tasks 5-11 project links, filtering, and UI selection.
- Produces:
  - `reporting.ProjectExporter.Export(ctx, projectID, filters, format) (exportID, error)`
  - `POST /api/v1/projects/{id}/exports` with `{format:"csv"|"xlsx", organizationIds, domainIds, kind, risk, window}`
  - `GET /api/v1/projects/{id}/exports/{exportId}` for status and download readiness
  - `ExportJob.ProjectID`, `OrganizationIDs`, `DomainIDs`, `Kind`, `Risk`, `Window`

- [ ] **Step 1: Write failing export tests**

Test CSV and XLSX exports containing only project-linked assets and findings; test an empty selection returns 400, a viewer returns 403, a non-member returns 404, and project B asset keys never appear. Verify `organizationIds` and `domainIds` are intersected with the project rather than trusted directly.

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./internal/modules/reporting ./internal/httpapi -run 'TestProjectExport' -v`
Expected: FAIL because project export does not exist.

- [ ] **Step 3: Implement project-scoped export**

Add project filters to `ExportJob` with `bson`/`json` tags. The endpoint creates a queued export record and the worker exports from `project_assets` plus project findings only. CSV headers are `organization,rootDomain,kind,key,title,severity,status,firstSeen,lastSeen,source,confidence`; XLSX adds separate `Assets`, `Findings`, and `Attributions` sheets. A user can download only an export whose `projectId` matches the authorized project and whose `createdBy` matches the caller unless the caller is super-admin.

Add an export action to the organization rail for selected units/domains. Poll status and expose download only when the export is finished.

- [ ] **Step 4: Run tests and commit**

Run: `go test ./internal/modules/reporting ./internal/httpapi`
Expected: PASS.

```bash
git add internal/modules/reporting/project_exporter.go internal/modules/reporting/project_exporter_test.go internal/httpapi/project_exports.go internal/httpapi/project_exports_test.go internal/modules/reporting/exporter.go internal/domain/models.go internal/store/project_queries.go internal/httpapi/server.go web/src/components/project/OrganizationRail.vue web/src/pages/ProjectHomePage.vue web/src/api.ts
git commit -m "feat: add project-scoped exports"
```

### Task 13: Deployment, Legacy Migration, E2E, and Documentation

**Files:**
- Create: `internal/store/legacy_migrate.go`
- Create: `internal/store/legacy_migrate_test.go`
- Modify: `cmd/arl/main.go`
- Modify: `deploy/compose.yaml`
- Modify: `deploy/config.docker.yaml`
- Modify: `deploy/Dockerfile`
- Modify: `config.example.yaml`
- Create: `web/e2e/project-home.spec.ts`
- Create: `web/e2e/project-fallback.spec.ts`
- Modify: `web/e2e/support.ts`
- Modify: `docs/PARITY.md`
- Modify: `docs/SCANNER-DIFFERENCES.md`

**Interfaces:**
- Consumes: all preceding tasks.
- Produces:
  - `arl legacy-import-scope --config PATH --scope-id ID --project-id ID --organization-id ID`
  - compose services `neo4j` and optional `geoip` volume mount
  - E2E coverage for project selection, empty state, import validation, graph/map linking, Neo4j fallback, GeoIP unavailable, and responsive widths

- [ ] **Step 1: Write failing legacy and E2E tests**

Test that `legacy-import-scope` copies targets without deleting the source scope and is idempotent. Test missing project/organization ids return a clear error. Add E2E tests that login lands on `/projects`, a last accessible project is restored after reload, no-project state shows create/import controls, viewer controls are absent, a mocked graph/map link updates the shared selection, batch scan refuses unconfirmed domains, export download is project-scoped, Neo4j-unavailable shows the list fallback, GeoIP-unavailable shows the map notice, and 1920x1080, 1440px, and 1280px layouts have no overlapping panels or horizontal overflow.

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./internal/store -run TestLegacyScopeImport -v && cd web && npm run test:e2e -- project-home.spec.ts`
Expected: FAIL because the command, fixtures, and E2E routes do not exist.

- [ ] **Step 3: Implement legacy copy, deployment wiring, and docs**

`legacy-import-scope` requires explicit `--project-id` and `--organization-id`; it copies scope target domains into that project as pending `source="legacy_scope"` attributions, copies scope-asset links into `project_assets` with a `legacy_scope` source, and never calls `DeleteScope`. It prints copied/skipped/error counts and returns nonzero on any error. It never auto-confirms ownership.

Add Neo4j and GeoIP mount examples to compose. Do not COPY MaxMind files into the image; mount a deployment-owned directory read-only. Add `ARL_NEO4J_*`, `ARL_GEOIP_*`, and attribution flags to the example config. Update `docs/PARITY.md` and `docs/SCANNER-DIFFERENCES.md` with the new project model, graph projection, external lookup disabled state, and GeoIP behavior. Generate screenshots at all three widths and store them under `docs/screenshots/project-home/` only if the repository convention permits checked-in screenshots; otherwise attach them to the change request and do not commit generated images.

- [ ] **Step 4: Run the full verification suite**

Run:

```bash
go test ./...
cd web && npm run typecheck
cd web && npm test -- --run
cd web && npm run build
cd ..
make verify
```

Expected: PASS. With Mongo and Neo4j available, additionally run:

```bash
ARL_TEST_MONGO_URI=mongodb://127.0.0.1:27017 go test ./internal/store ./internal/modules/tasking -run TestProject
ARL_TEST_NEO4J_URI=bolt://127.0.0.1:7687 go test ./internal/modules/graph
```

- [ ] **Step 5: Commit**

```bash
git add internal/store/legacy_migrate.go internal/store/legacy_migrate_test.go cmd/arl/main.go deploy/compose.yaml deploy/config.docker.yaml deploy/Dockerfile config.example.yaml web/e2e/project-home.spec.ts web/e2e/project-fallback.spec.ts web/e2e/support.ts docs/PARITY.md docs/SCANNER-DIFFERENCES.md
git commit -m "feat: ship project graph homepage"
```
