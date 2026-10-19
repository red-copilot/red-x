# EASM MVP 设计：组织优先的世界地图攻击面大屏

## 1. 摘要

ARL 首页改造为 EASM 态势大屏：世界地图是主画布，组织是第一层实体，资产图谱生长在地图之上。

用户先从组织列表选中一个单位，地图收拢到该单位的资产落点；再打开图谱，域名、IP、服务、漏洞之间的关系线浮在地图上，底图不消失。域名反查方向同样闭环：输入任一主域，查出备案主体，再拉取该主体名下的全部备案主域，确认后进入地图与图谱。

本设计取代 `2026-10-19-organization-graph-homepage-design.md` 中“首页即态势页、不做独立大屏、浅色工作台”的页面方向；项目、单位等底层模型仍然可以复用，但本期 MVP 不引入 Project。

## 2. 为什么现在做

现有控制台是通用资产管理后台：资产按 `(kind, key)` 扁平存储，地图和图谱都不存在。红队和 EASM 场景最常问的两个问题——“这家单位还有哪些备案主域”和“这些资产在地图上如何暴露、彼此什么关系”——目前都无法在同一屏回答。

已有能力可以支撑 MVP：

- 扫描链路已经把域名、IP、服务、站点、证书、漏洞写入 MongoDB；域名资产的 `data.ips`、IP 资产的 `data.ports`、服务资产的 `data.ip/port` 足以推导关系图。
- `config.geoip.city/asn` 已预留，但尚未真正解析。
- ICP 域名查询已有两个可用来源：站长 HTML（域名 → 主体）和自建 ICP_Query（主体 → 全部备案域名）。

## 3. 目标

- 世界地图作为 EASM 页主画布，组织气泡是第一层。
- 选中组织后，地图只保留该组织的资产落点，并可切换图谱关系层。
- 输入主域可反查备案主体，再反查该主体全部备案主域；所有归属带来源、时间和置信度。
- MongoDB 保持事实源；Neo4j 只做可重建的关系投影。
- GeoIP 离线富化，未定位资产有明确计数，不伪造坐标。
- Neo4j、ICP、GeoIP 任一不可用时，地图列表和既有扫描功能仍可用。

## 4. 非目标

- 不做自动漏洞利用、自动批量扫描新发现域名。
- 不做多租户、项目级成员权限、演练项目模型（Project 延后）。
- 不做组织合并 API；MVP 通过别名与重新归属处理重复组织，合并延后。
- 不做回源候选推断；CDN 边缘节点保留现有 `cdn/cdnName` 并计入组织资产，但不推断其回源地址。
- 不做 3D 地球、粒子背景、装饰性动画。
- 不做 SSE 实时推送；MVP 使用 30 秒轮询。

## 5. 反目标

- 地图只是装饰，不能筛选、不能定位。
- 一次渲染几千个节点和文字，导致不可读。
- 低置信度归属自动确认并写入事实。
- 把单位注册地址当作资产地理位置。
- Neo4j 写失败阻塞扫描主流程。
- 把 cookie、原始 HTML 或 ICP 原始响应写入前端、日志或 Neo4j。

## 6. 用户工作流

### 6.1 域名 → 组织 → 全部备案主域

1. 在 EASM 搜索框输入 `scitc.edu.cn`。
2. 系统归一化为主域，调用站长 HTML 查询，返回备案主体、备案号、审核时间。
3. 用户确认主体，系统创建或合并 Organization。
4. 系统调用 ICP_Query 按主体名查询全部备案记录，按主域去重后返回候选列表。
5. 用户勾选域名，确认后写入 `organization_domains`，投影为 `OWNS` 边。
6. 确认域名时可选勾选“同时创建扫描任务”；默认只确认归属，不创建任务。勾选后复用现有 task API 立即入队，不新增草稿任务状态。

### 6.2 组织 → 资产图谱

1. 打开 EASM 页，地图显示组织气泡；气泡大小=资产数，颜色=最高风险。
2. 点击组织，地图收拢到该组织的资产点，右侧 inspector 显示备案域名、资产统计和来源。
3. 点击“图谱”，关系线浮在地图上：组织 → 域名 → IP → 服务 → 漏洞。
4. 点击任意节点，inspector 显示证据、原始来源、扫描任务和资产详情。
5. 面包屑返回组织层。

## 7. 界面与交互

### 7.1 页面结构

- 顶部：产品名、全局搜索（域名/组织）、组织筛选、ICP/GeoIP/Neo4j 健康状态。
- 左侧：组织列表，显示资产数、高危数、ICP 覆盖状态。
- 中间：世界地图主画布。
- 右侧：inspector（选中组织或节点时出现）。
- 底部：图例、未定位资产计数、最近更新时间。

### 7.2 图层

1. **组织层**：每个组织一个气泡，按资产数缩放，按最高风险着色。
2. **资产层**：选中组织后显示该组织的域名/IP 落点；IP 才能落到地图，域名作为标签附在 IP 上。
3. **图谱层**：关系线覆盖在地图上；默认只显示深度 2、最多 300 个节点，超出按域名或 IP 聚合。

### 7.3 视觉约束

- 仅 EASM 页使用深色大屏；控制台其余页面保持现有浅色风格。
- 背景深蓝灰；资产蓝、中危琥珀、高危红；风险色只用于风险。
- 细线、低噪音；不做霓虹发光、粒子、3D。
- 尊重 `prefers-reduced-motion`；默认无入场动画。

### 7.4 空态与降级

- 无组织：显示“导入组织/域名”引导和手工创建入口。
- ICP_Query 不可用：组织详情仍展示已确认域名，显示“备案反查暂不可用”，允许手工导入。
- 站长 cookie 过期：搜索返回明确错误“ICP 查询凭证失效”，不影响地图和图谱。
- Neo4j 不可用：图谱按钮禁用并提示“关系图暂不可用”，地图和列表正常。
- GeoIP 缺失：地图显示“地理数据不可用”和未定位计数。

## 8. 数据模型

### 8.1 MongoDB 新集合

`organizations`

- `_id`、`canonicalName`、`aliases[]`
- `uscc?`（MVP 不自动采集，仅手工/后续企业数据源写入）
- `region?`（由备案号前缀推导，如“蜀ICP备” → 蜀）
- `externalIds`：`{ icpQuery?, aiqicha?, tianyancha? }`
- `status`：`active | merged`、`mergedInto?`
- `createdAt`、`updatedAt`

`organization_domains`

- `_id`、`organizationId`、`rootDomain`
- `siteName?`、`homeSite?`、`icpNo?`、`auditDate?`
- `source`：`icp_query | chinaz | manual | seed`
- `confidence`：`high | medium | low`
- `status`：`candidate | confirmed | rejected`
- `evidenceId?`、`firstSeen`、`lastSeen`、`updatedAt`
- 唯一索引：`(organizationId, rootDomain)`；普通索引：`rootDomain`、`status`

`icp_evidence`

- `_id`、`provider`、`queryType`（`domain | organization`）、`queryKey`
- `raw`（HTML 摘要或 JSON 原文）、`fetchedAt`、`expiresAt`、`payloadHash`
- 不存 cookie；原始响应只在这里保留，供人工核对。

`ip_locations`

- `_id`、`ip`、`lat`、`lon`、`country?`、`province?`、`city?`、`asn?`、`asnOrg?`
- `source`、`updatedAt`
- 唯一索引：`ip`

`graph_outbox`

- `_id`、`entityType`（`asset | finding | organization_domain | organization`）、`entityId`、`op`（`upsert | delete`）、`createdAt`、`processedAt?`、`attempts`
- 索引：`(processedAt, createdAt)`

`icp_jobs`

- `_id`、`organizationId`、`status`（`queued | running | done | failed`）、`requestedBy`、`resultCount`、`error?`、`createdAt`、`finishedAt?`
- 保留 7 天；由 worker 更新。

现有 `assets`、`asset_observations`、`findings`、`tasks`、`task_events` 不改结构。

### 8.2 Neo4j 关系

节点：

- `Organization {id, name}`
- `Domain {name, root}`
- `IP {addr, cdn, cdnName}`
- `Service {key, port, protocol, service}`
- `Finding {id, kind, severity, title}`

边：

- `(:Organization)-[:OWNS {source, confidence, status, icpNo}]->(:Domain)`
- `(:Domain)-[:RESOLVES_TO]->(:IP)`，来自域名资产 `data.ips[]`
- `(:IP)-[:EXPOSES]->(:Service)`，来自服务资产 `data.ip` + `data.port`
- `(:Finding)-[:FOUND_ON]->(:Domain|:IP|:Service)`，按 finding.target 精确匹配或解析 URL/host:port 后匹配

### 8.3 关系推导规则

- 域名 → IP：取 `asset.kind=domain` 的 `data.ips[]`。
- IP → 服务：取 `asset.kind=service` 的 `data.ip`、`data.port`、`data.protocol`。
- CDN 标记：IP 资产 `data.cdnName/cdnType` 非空，或解析到该 IP 的域名资产 `data.cdn=true`，任一成立即标记 `cdn=true`。
- 漏洞 → 资产：先按 `finding.target` 与资产 key 精确匹配；失败则解析为 URL host 或 `host:port`；仍无法匹配时计入 `unlinkedFindings`，不伪造边。
- 所有边都是派生数据，可从 MongoDB 全量重建。

## 9. 组织身份与归属

### 9.1 身份键

优先级：

1. 统一社会信用代码（若有）。
2. 规范化名称 + 备案地区。
3. 别名表。

名称规范化：去空白、全半角统一、去掉“（本部）”等括号后缀、去除常见组织形式后缀后比较；保留原始名称用于展示。

同名单位必须靠地区、备案号前缀或 USCC 区分，不能只靠名称。

### 9.2 置信度

- `high`：规范化名称完全一致，且备案号与已记录备案号一致。
- `medium`：规范化名称完全一致，且备案号前缀（地区）一致。
- `low`：仅名称模糊相似。

USCC 在 MVP 中不是必需字段；只有手工录入或后续企业数据源提供时才参与匹配。

规则：

- `high` 可自动确认，但必须写审计记录（来源、时间、匹配依据）。
- `medium` 生成默认勾选的候选，人工确认后才建立 `OWNS`。
- `low` 必须人工确认；不自动进入图谱或地图。
- 候选归属只出现在归属审核面板，不进入地图、图谱和组织统计。

### 9.3 域名 → 组织

- 一个域名允许多个候选主体，保持 `candidate`，人工确认后才建立 `OWNS`。
- MVP 中一个 rootDomain 同时最多一个 `confirmed` 组织；确需改判时由 operator 执行重新归属，旧关系保留审计记录。
- 原始证据写入 `icp_evidence`，前端可展开查看来源与时间。

### 9.4 资产归属闭包

MVP 采用以下闭包规则，避免共享基础设施污染组织视图：

- 组织拥有的是 `confirmed` 的 rootDomain。
- rootDomain 下发现的子域名（asset.kind=domain 且 key 以 `.<rootDomain>` 结尾）自动属于该组织。
- IP 只有通过该组织域名资产的 `data.ips[]` 解析关系进入组织视图；IP 上的服务与漏洞随之继承。
- 一个 IP 可以同时被多个组织引用（共享/CDN/云主机），并计入每个引用它的组织资产闭包；UI 标记“共享”。
- CDN 节点同样计入组织资产，额外标记 `cdn=true` 和 `cdnName`；地图和图谱展示时使用“CDN 边缘”标记，避免被误认为回源或主机所在地。
- 没有确认域名路径可达的 IP、服务、漏洞，不计入组织气泡和左侧统计。
- 组织气泡大小 = 闭包内去重资产数（domain + ip），颜色 = 闭包内最高风险。

### 9.5 风险口径

- 风险按“每个资产 + finding.dedupeKey 的最新一条 finding”计算，历史重复扫描不累加。
- 严重度顺序：`critical > high > medium > low > info`；`info` 不参与组织颜色。
- 无法匹配到资产的 finding 不计入组织风险，单独计入 `unlinkedFindings`。
- `risk=` 过滤器表示“至少存在该级别及以上的最新 finding”。

## 10. ICP 集成

### 10.1 Provider 接口

```go
type ICPProvider interface {
    LookupByDomain(ctx context.Context, rootDomain string) ([]DomainAttribution, error)
    SearchByOrganization(ctx context.Context, org OrganizationQuery) ([]ICPDomain, error)
}
```

错误类型化：`ErrCookieExpired`、`ErrRateLimited`、`ErrQuotaExceeded`、`ErrProviderUnavailable`、`ErrNeedsHumanVerification`。

### 10.2 站长：域名 → 主体

- 请求 `https://icp.chinaz.com/{rootDomain}`，解析服务端 HTML。
- 字段：`主办单位名称`、`主办单位性质`、`网站备案/许可证号`、`审核时间`。
- 已验证该页面不依赖浏览器 JS。
- Cookie 与 User-Agent 必须成对配置，UA 使用真实浏览器 UA。

### 10.3 ICP_Query：主体 → 全部域名

- 外部 sidecar，默认地址 `http://127.0.0.1:16181`。
- 请求 `GET /query/web?search={organizationName}&pageNum={n}&pageSize={size}`。
- 解析 `params.list[]`：`domain`、`serviceLicence`、`unitName`、`serviceName`；审核时间字段以 sidecar 实际返回为准，缺失时留空。
- 分页：第一页读取 `total/pages`，每页之间固定 5 秒；最多 20 页。
- 仅当 `unitName` 与组织规范化名称一致（或命中别名）时标记 `medium/high`，否则候选。
- 该服务独立部署；ARL 不引入 Python 运行时，也不复制其代码。

### 10.4 配置

```yaml
icp:
  cacheTTL: 168h
  chinaz:
    cookieFile: ""          # 优先于环境变量
    userAgent: ""
  icpQuery:
    baseURL: "http://127.0.0.1:16181"
    timeout: 30s
    pageSize: 50
```

环境变量：

- `ARL_ICP_CHINAZ_COOKIE`
- `ARL_ICP_CHINAZ_USER_AGENT`
- `ARL_ICP_ICPQUERY_BASE_URL`（可选覆盖）

安全：

- cookie 只从环境变量或只读文件读取，不进入 Mongo、日志、审计详情、前端。
- 健康检查只返回 `ok | cookie_expired | unavailable`，不返回 cookie 内容或长度。

### 10.5 限流、缓存与熔断

- 每 provider 一个令牌桶；站长 1 QPS，ICP_Query 1 并发。
- 同 key 请求 single-flight；缓存 7 天，手动刷新可强制重查。
- 5 次连续失败打开熔断 10 分钟；熔断期间返回 `ErrProviderUnavailable`。
- ICP_Query 分页每页间隔 5 秒，禁止在 HTTP handler 内同步跑完整分页；由 worker 执行。

## 11. GeoIP

- 读取本地 MaxMind GeoLite2 City/ASN mmdb：`config.geoip.city`、`config.geoip.asn`。
- 新增 `GeoResolver`，对公网 IP 解析经纬度、国家、省、市、ASN、ASN 组织。
- 私有地址、保留地址、解析失败 → 不写坐标，计入“未定位”。
- `ip_locations` 作为缓存；IP 资产写入或定时 reconcile 时补齐。
- 地图只画有坐标的 IP；域名通过解析关系挂到 IP 上。

## 12. Neo4j 投影

- 使用 `neo4j-go-driver/v5`。
- 使用 `graph_outbox`：`UpsertAsset`、`AddFinding`、归属确认/导入、组织变更在写 MongoDB 的同一事务或紧邻写入后追加事件。
- projector 按 `(processedAt, createdAt)` 消费事件，用 `MERGE` 幂等写入；消费成功后写 `processedAt`。
- 对 `RESOLVES_TO/EXPOSES/FOUND_ON/OWNS` 的更新，先删除该实体的旧边再重建，避免残留陈旧关系。
- `delete` 事件用于停用/移除的组织域名和资产。
- 提供 `arl graph rebuild`：清空派生图并全量重建。
- 每 24 小时做一次轻量对账，比较关键节点/边计数；差异超阈值时触发全量重建。
- 投影失败只记录状态和重试，不阻塞扫描。
- 图查询必须限制深度和节点数：`depth<=3`、`limit<=500`；超限由服务端聚合并在 DTO 标记 `truncated/aggregated`。
- Neo4j 未配置时，图谱 API 返回 `503 graph_unavailable`；地图和列表不受影响。

## 13. API

新增：

- `GET /api/v1/organizations?query=&limit=`
- `POST /api/v1/organizations`（手工创建）
- `GET /api/v1/organizations/{id}`
- `POST /api/v1/icp/lookup`，输入 `{domain}`，返回候选主体
- `POST /api/v1/organizations/{id}/icp-domains/refresh`，入队反查；返回 job id
- `GET /api/v1/icp/jobs/{id}`，返回状态、结果数和错误
- `GET /api/v1/organizations/{id}/domains`
- `POST /api/v1/organizations/{id}/domains/confirm`，输入 `{domains[]}`
- `POST /api/v1/organizations/{id}/domains/import`，手工 CSV/文本导入
- `GET /api/v1/easm/map?orgId=&risk=`
- `GET /api/v1/easm/graph?orgId=&depth=2&limit=300`
- `GET /api/v1/easm/health`

权限：

- 读接口：所有已登录角色。
- 创建组织、确认归属、导入域名、刷新反查：`operator` 或 `admin`。
- 沿用现有 CSRF、RBAC、problem+json 错误格式。

所有写接口必须审计：谁确认了哪个归属、来源是什么、时间。

## 14. 前端

- 现有 `DashboardPage` 由 `EasmPage` 取代，路由 `/` 直接进入 EASM 大屏；不保留旧 dashboard 页面。
- 旧 dashboard 中有用的任务指标和系统健康状态并入 EASM 顶部状态条或 inspector，不再单独成页。
- 使用 Vue 3 + TypeScript + Pinia；图与地图使用 ECharts + vue-echarts。
- 世界地图 GeoJSON 作为静态资源打包，不运行时请求外网。
- 状态：当前组织、选中节点、图谱开关、健康状态、轮询时间。
- 数据：30 秒轮询 map/graph；切换组织立即请求，不等待轮询。
- 组件拆分：`EasmPage`、`WorldMap`、`OrgRail`、`Inspector`、`Legend`、`HealthPill`。

## 15. MVP 里程碑

1. 组织与备案域名存储、迁移、索引、单元测试。
2. ICP provider：站长 HTML + ICP_Query 客户端、缓存、错误分类、httptest fixture。
3. GeoIP resolver 与 `ip_locations` 富化。
4. Neo4j 投影 worker + `arl graph rebuild`。
5. EASM API 与权限、审计。
6. 前端大屏：组织层 → 资产层 → 图谱层、inspector、降级状态。
7. 端到端验证：种子数据 + Playwright 冒烟。

## 16. 验证

- 单元测试：域名归一化、HTML/JSON 解析、置信度规则、GeoIP 失败回退、投影幂等。
- 集成测试：Mongo（`ARL_TEST_MONGO_URI`）；Neo4j 使用 `ARL_TEST_NEO4J_URI`，缺失时跳过。
- 前端：`npm run typecheck`、`npm run build`。
- E2E：组织列表 → 选中组织 → 打开图谱 → 返回；ICP 不可用时页面不白屏。
- 手工验收：用 `scitc.edu.cn` 验证域名→主体→多主域链路。

## 17. 风险

- ICP_Query 依赖第三方对工信部接口的自动化与验证码识别，存在 ToS 和稳定性风险；必须作为可替换 sidecar，并由部署方决定是否启用。
- 站长页面结构变更会导致解析失败；解析器需要 fixture 测试和明确的错误状态。
- 同名单位合并错误会污染图谱；低置信度必须人工确认。
- 大规模图谱可读性；MVP 以聚合和截断优先。

## 18. 留给 builder 的决定

- 地图与图谱都用 ECharts 实现；若世界地图交互不足，可换 MapLibre，但不改变 API。
- MVP 用 30 秒轮询，不做 SSE。
- 首页即 EASM；不保留 `/dashboard`，旧 DashboardPage 删除或替换。
- 不引入 Project 模型；组织是全局实体，Scope 继续承担扫描分组。
- 组织详情里的“加入扫描”只创建现有任务，不新增自动扫描逻辑。
