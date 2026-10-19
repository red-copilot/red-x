# 项目级资产图谱首页 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 ARL 首页改造成项目级 EASM 工作台，支持项目与单位批量管理、域名归属查询、Neo4j 资产关系图和 GeoIP 地图联动。

**Architecture:** MongoDB 保存项目、单位、域名、归属证据、任务、观测和发现；Neo4j 通过 graph outbox 幂等投影资产关系。前端使用 Vue 3 加 ECharts，在浅色工作台中共享项目、单位、风险和时间筛选。

**Tech Stack:** Go 1.27, MongoDB 8, Neo4j 5.x, RabbitMQ, Vue 3, TypeScript, Pinia, ECharts, Vue ECharts, Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-10-19-organization-graph-homepage-design.md`

## Global Constraints

- 所有新增 JSON/BSON 字段使用 camelCase。
- `context.Context` 必须是所有新 service、provider、repository 方法的第一个参数。
- MongoDB 是项目、单位、归属证据、任务、观测和发现的真相源。
- Neo4j 只能作为可重建投影，不得成为扫描写入的阻塞依赖。
- 所有 graph/map/search 查询必须携带 projectId，并在服务端校验项目存在。
- Chinaz Cookie/APIKey 只能从环境变量或只读文件读取，不能进入前端、数据库、审计详情或普通日志。
- 外部供应商请求只允许固定 URL 和固定路径，不允许用户控制目标 URL。
- 不实现验证码绕过、签名伪造或高频抓取。
- 地图显示 IP 地理位置；CDN 边缘 IP、回源候选、直接资产 IP、未定位资产必须分开。
- 首页采用方案 A：浅色、克制、单一强调色、细边框，不使用渐变发光和装饰性动画。
- 不新增独立 wallboard 页面。
- 现有 Scope 保留为 Legacy 资产组，不自动转换成 Project。
- v1 使用现有全局 RBAC，不新增项目级成员权限；projectId 只用于查询隔离和 UI 上下文。
- 组织归属不得自动确认；候选结果必须保持 pending，等待 operator 确认或拒绝。
- 每个任务完成后只提交本任务文件，不提交无关的既有修改。

## Review Focus

- 供应商 Cookie/APIKey 过期或限流时，CSV 导入和资产发现仍能完成入库，归属进入可重试 pending 状态。
- 同一资产被多个项目发现时，项目 A 的图谱和地图不能泄露项目 B 的边、finding、域名或单位。
- 私有 IP、CDN 边缘 IP、回源候选和普通资产 IP 不能在数据模型或 UI 中混淆。
- 大规模项目（100 单位、300 主域名、数千资产）不能一次返回完整图；必须聚合、截断并可展开。
- Neo4j、GeoIP 或供应商不可用时，首页仍有可用的单位列表和错误状态，不能白屏。

## Task 1: 项目与单位领域模型和 MongoDB 存储

**Files:**
- Create: `internal/domain/projects.go`
- Modify: `internal/domain/models.go`
- Modify: `internal/store/store.go`
- Create: `internal/store/projects.go`
- Create: `internal/domain/projects_test.go`
- Create: `internal/store/projects_integration_test.go`

**Interfaces:**
- Produces `domain.Project`, `Organization`, `ProjectOrganization`, `ProjectDomain`, `OrganizationAttribution`, `IPLocation`.
- Produces store methods: `CreateProject`, `GetProject`, `ListProjects`, `UpsertOrganization`, `GetOrganizationByLegalName`, `AddOrganizationToProject`, `ListProjectOrganizations`, `AddProjectDomain`, `ListProjectDomains`, `AddAttribution`, `ListAttributions`, `ConfirmAttribution`, `RejectAttribution`, `UpsertIPLocation`, `GetIPLocation`.

- [ ] **Step 1: Write failing tests** in `internal/domain/projects_test.go`: `TestProjectValidateRejectsEmptyName`, `TestProjectDomainValidateRejectsInvalidDomain`, `TestAttributionStatusTransitions`.
- [ ] **Step 2: Run tests** `go test ./internal/domain -run 'TestProject|TestAttribution' -v`; expect FAIL.
- [ ] **Step 3: Add domain types** with the fields in the spec, all IDs as `bson.ObjectID`, timestamps as `time.Time` or `*time.Time`, plus validation methods.
- [ ] **Step 4: Add collections and indexes** in `internal/store/store.go`: `projects`, `organizations`, `project_organizations`, `project_domains`, `organization_attributions`, `ip_locations`; unique legal name, unique project/org, unique project/domain, unique IP.
- [ ] **Step 5: Implement repositories** in `internal/store/projects.go`; every create/upsert sets timestamps and `SchemaVersion`. `ConfirmAttribution` must be idempotent and only accept `pending`.
- [ ] **Step 6: Write and run Mongo integration tests** in `internal/store/projects_integration_test.go`; use `ARL_TEST_MONGO_URI`, skip when absent.
- [ ] **Step 7: Run** `ARL_TEST_MONGO_URI=mongodb://127.0.0.1:27017 go test ./internal/store -run 'TestProject|TestOrganization|TestAttribution' -v`; expect PASS or SKIP.
- [ ] **Step 8: Commit** `feat: add project and organization storage`.

## Task 2: CSV 项目导入服务

**Files:**
- Create: `internal/modules/projects/service.go`
- Create: `internal/modules/projects/import.go`
- Create: `internal/modules/projects/service_test.go`
- Modify: `internal/platform/netsafe/netsafe.go` only if a reusable domain validator is needed.

**Interfaces:**
- Produces `projects.ImportRow`, `projects.ImportResult`, `projects.Repository`, `projects.LookupScheduler`, and `projects.Service.ImportCSV(ctx, projectID, io.Reader) (ImportResult, error)`.
- `LookupScheduler.EnqueueLookup(ctx, projectID, domain) error` lets import enqueue attribution work without calling a supplier inline.
- `ImportResult` has `Organizations`, `Domains`, `QueuedLookups`, `Warnings`.

- [ ] **Step 1: Write failing tests** for multiple root domains, invalid domain rejection, idempotent import, and missing legal name producing a pending lookup.
- [ ] **Step 2: Run** `go test ./internal/modules/projects -run TestImportCSV -v`; expect FAIL.
- [ ] **Step 3: Implement CSV parsing** with `encoding/csv`, UTF-8 BOM handling, empty-line skipping, semicolon-separated domains, and `netsafe.ValidateDomain`.
- [ ] **Step 4: Implement repository-backed import** that upserts organizations, project membership, project domains, and pending attributions. If `LookupScheduler` is configured, enqueue pending lookups; do not call a supplier inline and do not start scan tasks during import.
- [ ] **Step 5: Run** `go test ./internal/modules/projects -v`; expect PASS.
- [ ] **Step 6: Commit** `feat: add CSV project import`.

## Task 3: Chinaz 组织归属 provider

**Files:**
- Create: `internal/modules/orgintel/provider.go`
- Create: `internal/modules/orgintel/chinaz_api.go`
- Create: `internal/modules/orgintel/chinaz_page.go`
- Create: `internal/modules/orgintel/service.go`
- Create: `internal/modules/orgintel/chinaz_api_test.go`
- Create: `internal/modules/orgintel/chinaz_page_test.go`
- Create: `internal/modules/orgintel/testdata/guc.edu.cn.html`

**Interfaces:**
- `orgintel.Candidate` has `Name`, `Type`, `SiteLicense`, `ServiceLicense`, `VerifyTime`, `Source`, `Confidence`, `Raw`.
- `orgintel.Provider` has `Name() string` and `Lookup(ctx, domain string) ([]Candidate, error)`.
- `orgintel.Service.EnqueueLookup(ctx, projectID, domain string) (*domain.OrganizationAttribution, error)` creates a pending attribution and publishes a lookup job.
- `orgintel.Service.Process(ctx, attributionID bson.ObjectID) error` calls the provider chain and updates the attribution.
- `orgintel.Service.Lookup(ctx, domain string) ([]Candidate, error)` is the provider-facing method used by tests and `Process`.

- [ ] **Step 1: Write failing tests** for JSON mapping, upstream failure, GUC page fixture parsing, and pending attribution persistence.
- [ ] **Step 2: Run** `go test ./internal/modules/orgintel -v`; expect FAIL.
- [ ] **Step 3: Implement official JSON provider** using fixed `https://openapi.chinaz.net/v1/1001/newicp` with `domain`, `APIKey`, `ChinazVer=1.0`; map `CompanyName`, `CompanyType`, `SiteLicense`, `ServiceLicence`, `VerifyTime`; confidence `1.0`.
- [ ] **Step 4: Implement page fallback** using fixed `https://icp.chinaz.com/{domain}` and a configured Cookie; parse `#companyName`, `主办单位性质`, `审核时间`; confidence `0.8`; never log the Cookie.
- [ ] **Step 5: Implement provider chain** API first, page fallback second; apply a per-domain cache TTL of 7 days and a per-provider minimum interval. `Process` updates pending attributions; on failure return a typed error and leave the attribution retryable. Add `q.orgintel` to `jobs.QueueNames` and enqueue lookup jobs.
- [ ] **Step 6: Run** `go test ./internal/modules/orgintel -v`; expect PASS.
- [ ] **Step 7: Commit** `feat: add chinaz organization attribution provider`.

## Task 4: 项目、单位、归属查询 HTTP API

**Files:**
- Create: `internal/httpapi/projects.go`
- Create: `internal/httpapi/organization_lookup.go`
- Create: `internal/httpapi/projects_test.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/openapi.go`
- Modify: `internal/app/app.go`

**Interfaces:**
- Routes: `GET/POST /projects`, `GET/PATCH /projects/{id}`, `POST /projects/{id}/organizations/import`, `GET /projects/{id}/organizations`, `POST /organizations/lookup`, `GET /organizations/{id}/attributions/{attributionId}`, `POST .../confirm`, `POST .../reject`.
- `POST /organizations/lookup` returns HTTP 202 with `{attributionId, status: "pending"}` unless a cached confirmed result exists.
- Uses `projects.Service` and `orgintel.Service`.

- [ ] **Step 1: Write failing handler tests** for write permission, pagination, lookup source, confirm authorization, and invalid CSV.
- [ ] **Step 2: Run** `go test ./internal/httpapi -run 'TestProject|TestLookup|TestConfirm|TestImport' -v`; expect FAIL.
- [ ] **Step 3: Implement handlers and DTOs**; all responses use `{ "data": ... }`, errors use `httpx.WriteProblem`.
- [ ] **Step 4: Wire services** into `httpapi.Server` and `app.New`; register routes before the SPA fallback.
- [ ] **Step 5: Update OpenAPI** path summaries; do not expose raw attribution in list DTOs.
- [ ] **Step 6: Run** `go test ./internal/httpapi -v`; expect PASS.
- [ ] **Step 7: Commit** `feat: expose project and organization lookup APIs`.

## Task 5: GeoIP 位置解析

**Files:**
- Create: `internal/platform/geoip/resolver.go`
- Create: `internal/platform/geoip/maxmind.go`
- Create: `internal/platform/geoip/resolver_test.go`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/app/app.go`

**Interfaces:**
- `geoip.Location` has country, province, city, latitude, longitude, ASN, ASN org, source, confidence.
- `geoip.Resolver` has `Lookup(ctx, netip.Addr) (*Location, error)` and `Health(ctx) error`.
- `geoip.NewMaxMindResolver(cityPath, asnPath string) (*MaxMindResolver, error)`.

- [ ] **Step 1: Write failing tests** for invalid IP, private IP returning nil, normalization, and missing database health.
- [ ] **Step 2: Run** `go test ./internal/platform/geoip -v`; expect FAIL.
- [ ] **Step 3: Add `geoip.enabled` config** and initialize the resolver in `app.New`; a missing database is a warning, not startup failure.
- [ ] **Step 4: Implement resolver** with injectable readers; private, loopback, link-local, and reserved addresses return `nil, nil`.
- [ ] **Step 5: Persist through `store.UpsertIPLocation`**; keep the IP as the unique key.
- [ ] **Step 6: Run** `go test ./internal/platform/geoip ./internal/platform/config -v`; expect PASS.
- [ ] **Step 7: Commit** `feat: add offline geoip resolver`.

## Task 6: Neo4j 客户端、graph outbox 和 projector

**Files:**
- Create: `internal/platform/graphdb/client.go`
- Create: `internal/platform/graphdb/schema.go`
- Create: `internal/modules/graph/projector.go`
- Create: `internal/modules/graph/events.go`
- Create: `internal/modules/graph/projector_test.go`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/jobs/jobs.go`
- Modify: `internal/store/inventory.go`
- Modify: `internal/app/app.go`
- Modify: `cmd/arl/main.go`

**Interfaces:**
- Adds `neo4j` config block and `q.graph` queue.
- `graph.Event` has `Type`, `AssetID`, `TaskID`, `ProjectID`, `OrganizationID`, `Domain`, `OccurredAt`.
- `graph.Projector.HandleDelivery`, `Rebuild`, `Health`.

- [ ] **Step 1: Write failing tests** for idempotent upsert, unknown task, event payload shape, and schema constraints; fake the graph store.
- [ ] **Step 2: Run** `go test ./internal/modules/graph ./internal/platform/graphdb -v`; expect FAIL.
- [ ] **Step 3: Add config and queue** with `ARL_NEO4J_*` environment overrides and `q.graph` in `jobs.QueueNames`.
- [ ] **Step 4: Emit outbox events** from asset/finding/attribution writes; payload contains IDs and stable keys only, never raw evidence.
- [ ] **Step 5: Implement projector** using `MERGE` on stable IDs and project membership; read task/project context from Mongo. Neo4j failure is non-blocking and retryable.
- [ ] **Step 6: Wire worker and all-in-one mode** so `q.graph` goes to the projector and `q.orgintel` goes to the attribution processor; add `arl graph rebuild` reading from Mongo.
- [ ] **Step 7: Run** `go test ./internal/modules/graph ./internal/platform/graphdb ./internal/store -v`; expect PASS or SKIP.
- [ ] **Step 8: Commit** `feat: project graph events into neo4j`.

## Task 7: 图谱、地图和项目事件 API

**Files:**
- Create: `internal/httpapi/graph.go`
- Create: `internal/httpapi/graph_test.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/openapi.go`
- Create: `internal/modules/graph/query.go`

**Interfaces:**
- Routes: `GET /projects/{id}/graph`, `GET /projects/{id}/map`, `GET /projects/{id}/events`.
- Graph DTO: `nodes`, `edges`, `truncated`, `aggregated`, `limit`.
- Map DTO: `points`, `clusters`, `unlocatedCount`.

- [ ] **Step 1: Write failing tests** for unknown project, limit truncation, private IP exclusion, project isolation, and project event payload.
- [ ] **Step 2: Run** `go test ./internal/httpapi -run 'TestGraph|TestMap|TestProjectIsolation' -v`; expect FAIL.
- [ ] **Step 3: Implement query options** `focus`, `depth`, `types`, `risk`, `limit`; default 300, max 500.
- [ ] **Step 4: Implement Cypher** starting from the requested Project; never start from a global asset. Aggregate map by city/country.
- [ ] **Step 5: Implement project SSE** with no raw evidence or credentials.
- [ ] **Step 6: Run** `go test ./internal/httpapi -v`; expect PASS.
- [ ] **Step 7: Commit** `feat: expose project graph and map APIs`.

## Task 8: 任务项目上下文

**Files:**
- Modify: `internal/domain/models.go`
- Modify: `internal/modules/tasking/engine.go`
- Modify: `internal/httpapi/tasks.go`
- Modify: `internal/store/tasks.go`
- Create/modify: `internal/modules/tasking/engine_test.go`

**Interfaces:**
- Adds optional `ProjectID`, `OrganizationID`, `RootDomainID` to Task.
- `POST /tasks` accepts these fields; `ListTasks` filters by projectId.

- [ ] **Step 1: Write failing tests** for project persistence, project filtering, unknown project rejection, and legacy scope compatibility.
- [ ] **Step 2: Run** `go test ./internal/modules/tasking ./internal/store ./internal/httpapi -run 'TestTask.*Project|TestTask.*Legacy' -v`; expect FAIL.
- [ ] **Step 3: Add fields and store filters**; old tasks remain readable.
- [ ] **Step 4: Update task API** and validate project existence before submit.
- [ ] **Step 5: Run** `go test ./internal/modules/tasking ./internal/store ./internal/httpapi -v`; expect PASS.
- [ ] **Step 6: Commit** `feat: add project context to tasks`.

## Task 9: 前端设计 token 和认证外壳

**Files:**
- Modify: `web/src/styles.css`
- Modify: `web/src/layouts/AppLayout.vue`
- Modify: `web/src/router.ts`
- Modify: `web/src/pages/LoginPage.vue`
- Create: `web/src/pages/ProjectHomePage.vue`
- Create: `web/src/layouts/AppLayout.test.ts`

**Interfaces:**
- Adds route `/projects/:projectId`; `/` redirects to last project or projects list; `/dashboard` redirects to `/`.
- Adds light design tokens: `--arl-bg`, `--arl-surface`, `--arl-text`, `--arl-muted`, `--arl-border`, `--arl-accent`, risk colors.

- [ ] **Step 1: Write failing tests** for project context rendering, navigation, and root redirect.
- [ ] **Step 2: Run** `cd web && npx vitest run src/layouts/AppLayout.test.ts`; expect FAIL.
- [ ] **Step 3: Add design tokens** and convert the shell to a light, narrow, collapsible layout.
- [ ] **Step 4: Add project route** and redirect legacy dashboard.
- [ ] **Step 5: Run** `cd web && npx vitest run src/layouts/AppLayout.test.ts && npm run typecheck`; expect PASS.
- [ ] **Step 6: Commit** `feat: add light console shell and project route`.

## Task 10: 前端项目状态和类型化 API

**Files:**
- Create: `web/src/api/projects.ts`
- Create: `web/src/types/graph.ts`
- Create: `web/src/stores/project.ts`
- Create: `web/src/stores/project.test.ts`
- Create: `web/vitest.config.ts`

**Interfaces:**
- Types: `Project`, `Organization`, `GraphNode`, `GraphEdge`, `MapPoint`, `MapCluster`.
- API: `fetchProjects`, `createProject`, `importOrganizations`, `lookupDomain`, `fetchAttribution`, `fetchProjectGraph`, `fetchProjectMap`.
- Store: `currentProjectId`, `selectedNodeId`, `filters`, `setFilter`, `selectNode`; default filters `{ risk: "high", kind: "all", timeRange: "30d", mapScope: "china" }`.

- [ ] **Step 1: Write failing tests** for default filters, graph/map filter sync, project reset, and problem code preservation.
- [ ] **Step 2: Run** `cd web && npx vitest run src/stores/project.test.ts`; expect FAIL.
- [ ] **Step 3: Implement typed API and store** using existing `api<T>()`; never include provider credentials.
- [ ] **Step 4: Run** `cd web && npx vitest run src/stores/project.test.ts && npm run typecheck`; expect PASS.
- [ ] **Step 5: Commit** `feat: add typed project store and api`.

## Task 11: 图谱和地图组件

**Files:**
- Modify: `web/package.json`
- Modify: `web/package-lock.json`
- Create: `web/src/components/graph/AssetGraph.vue`
- Create: `web/src/components/graph/AssetGraph.test.ts`
- Create: `web/src/components/map/AssetMap.vue`
- Create: `web/src/components/map/AssetMap.test.ts`
- Create: `web/src/assets/maps/china.json`
- Create: `web/src/assets/maps/world.json`

**Interfaces:**
- `AssetGraph` props `nodes`, `edges`, `selectedId`, `loading`; emits `select`, `expand`, `filter`.
- `AssetMap` props `points`, `clusters`, `scope`, `selectedId`; emits `select`, `region`, `scopeChange`.

- [ ] **Step 1: Install** `cd web && npm install echarts vue-echarts`.
- [ ] **Step 2: Write failing tests** for node selection, legend, China/world switch, region selection, and unlocated count. Mock ECharts.
- [ ] **Step 3: Implement graph** with labels only for selected/unit/domain nodes; risk colors only for risk; empty state.
- [ ] **Step 4: Implement map** with geo series; point kinds `asset`, `edge`, `origin`; no point for private IP; missing GeoIP adds to unlocated count.
- [ ] **Step 5: Document GeoJSON source/license** in `docs/ARCHITECTURE.md`; commit only licensed map data.
- [ ] **Step 6: Run** `cd web && npx vitest run src/components/graph src/components/map && npm run typecheck`; expect PASS.
- [ ] **Step 7: Commit** `feat: add asset graph and map components`.

## Task 12: 首页组合、项目导入和域名反查

**Files:**
- Modify: `web/src/pages/ProjectHomePage.vue`
- Create: `web/src/components/project/ProjectSwitcher.vue`
- Create: `web/src/components/project/ProjectImportDialog.vue`
- Create: `web/src/components/project/DomainLookupDialog.vue`
- Create: `web/src/components/project/AssetInspector.vue`
- Create: `web/src/pages/ProjectHomePage.test.ts`
- Modify: `web/src/pages/DashboardPage.vue` to redirect or remove.

**Interfaces:**
- Project homepage consumes Task 10/11. No separate wallboard.
- Layout: top project/search/actions, left units, center graph, right map/inspector/events, bottom event strip.

- [ ] **Step 1: Write failing tests** for project context, lookup candidate, import refresh, graph inspector, map filter, and Neo4j unavailable banner.
- [ ] **Step 2: Run** `cd web && npx vitest run src/pages/ProjectHomePage.test.ts`; expect FAIL.
- [ ] **Step 3: Build the layout** with small text summaries, not metric-card grids.
- [ ] **Step 4: Implement import/lookup** with CSV warnings, source/confidence, confirm/reject, and list refresh. Lookup returns pending, then polls `fetchAttribution` every 2 seconds for up to 60 seconds; show timeout and retry state.
- [ ] **Step 5: Implement degraded states** for no project, no GeoIP, and Neo4j down.
- [ ] **Step 6: Run** `cd web && npx vitest run src/pages/ProjectHomePage.test.ts && npm run typecheck && npm run build`; expect PASS.
- [ ] **Step 7: Commit** `feat: build project asset graph homepage`.

## Task 13: 部署、迁移、诊断和 E2E

**Files:**
- Modify: `deploy/compose.yaml`
- Modify: `deploy/config.docker.yaml`
- Modify: `config.example.yaml`
- Modify: `internal/platform/config/config.go`
- Modify: `cmd/arl/main.go`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/PARITY.md`
- Create: `web/e2e/home.spec.ts`
- Modify: `web/e2e/support.ts`
- Modify: `web/e2e/console.spec.ts`

- [ ] **Step 1: Add Neo4j compose service** with pinned 5.x image, volume, auth, ports 7474/7687, and healthcheck.
- [ ] **Step 2: Add config and doctor checks** for Neo4j, GeoIP files, and Chinaz configured/missing status; never print secret values.
- [ ] **Step 3: Add E2E** for login, project creation, CSV import, `guc.edu.cn -> 四川吉利学院`, confirm, graph selection, map filter, and degraded banner; use API fixtures and no real Chinaz network.
- [ ] **Step 4: Update existing E2E expectations** from `/dashboard` to project home.
- [ ] **Step 5: Run backend verification** `go test ./...`, `go test -race ./...`, `go vet ./...`.
- [ ] **Step 6: Run frontend verification** `cd web && npm run typecheck && npm run build && npx vitest run`.
- [ ] **Step 7: Run integration** with `ARL_TEST_MONGO_URI` and `ARL_TEST_NEO4J_URI`.
- [ ] **Step 8: Run E2E** with `E2E_PASSWORD`; do not output secrets.
- [ ] **Step 9: Commit** `feat: deploy project graph homepage`.

## Self-Review

- Spec coverage: project/organization model, CSV import, Chinaz provider, graph/map API, GeoIP, Neo4j projection, homepage A, legacy Scope, security, fallback, tests all have tasks.
- Type consistency: projectId is a string at API/JS boundaries and bson.ObjectID in Mongo domain; graph/map DTOs use string IDs.
- Review Focus tests are assigned to Tasks 3, 7, 11, and 12.
- Plan scope: individual legacy pages are not restyled beyond the shared shell; they follow after homepage parity.
