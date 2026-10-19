# 项目级资产图谱首页设计

## 1. 摘要

ARL 的界面从通用资产管理后台调整为面向红队项目的 EASM 工作台。首页不是独立态势大屏，而是用户登录后直接进入的项目工作区：以项目、单位、主域名、子域名、IP、服务、漏洞和证据为一条连续的数据链，资产图谱与地理分布共享筛选和选择状态。

视觉方向采用已确认的方案 A：浅色、克制、单一深色强调色、细边框和低噪音。首页必须同时满足“打开即知道项目态势”和“打开即可开始研判与批量操作”两件事。

本设计的核心变化不只是 UI：

- 新增 Project 和 Organization 一级模型。
- 引入 Neo4j 作为资产关系的查询与投影层。
- 引入组织归属查询与 GeoIP 地理富化。
- 将首页重组为图谱主画布加地图切片，而不是指标卡堆叠。
- 为后续 CDN 边缘节点与回源候选关系预留完整表达。

## 2. 为什么现在做

现有控制台是通用后台结构：左侧模块导航、顶部标题、表格和 JSON 字段。它无法表达红队工作中最重要的三件事：

1. 一个演习项目下面有大量单位，单位和主域名需要批量管理。
2. 资产不是孤立记录，而是“单位拥有域名，域名解析到 IP，IP 暴露服务，服务关联漏洞”的关系网络。
3. 用户需要同时看到关系、地理分布和证据来源，并从同一屏直接进入下一步操作。

现有 `Scope` 只是资产组，不能表达项目、单位、域名归属和扫描覆盖。资产目前按 `(kind, key)` 扁平存储，缺少显式的 DNS、托管、证书、CDN、漏洞归属关系。GeoIP 配置存在，但 `geoCity` / `geoAsn` 尚未真正填充。因此地图和关系图不能只靠前端拼装。

## 3. 目标与非目标

### 目标

- 用户可创建项目，例如“2025 四川省攻防演习”，批量导入 100 家单位及主域名。
- 用户输入域名时可反查主办单位，并看到来源、时间、原始结果和置信度。
- 首页展示当前项目的单位覆盖、资产关系、地理分布和高风险事件。
- 图谱和地图共享筛选：选择地图区域会过滤图谱，选择图谱节点会定位地图位置。
- 用户可以从单位页直接发起批量扫描、查看任务状态、查看证据和资产详情。
- 组织归属结果可人工确认、拒绝、合并别名，不能无痕自动写入事实。
- Neo4j 可在不阻塞主业务的情况下重建；MongoDB 仍保存原始观测、任务、审计和证据。

### 非目标

- 不做自动漏洞利用平台。
- 不把页面抓取或验证码绕过作为唯一数据源依赖。
- 不做独立的态势大屏页面；首页就是态势页。
- 不在 v1 实现跨租户 SaaS 计费、复杂组织权限模型或完整攻击路径推演。
- 不把原始 observations 和完整 HTTP 响应塞进 Neo4j。
- 不为了视觉“科技感”引入 3D 地球、霓虹光效或无意义动画。

### 反目标

- 地图只是装饰，不能筛选，不能回答任何判断。
- 图谱一次渲染几千个节点和文字，导致不可读。
- 自动归属错误单位且没有来源和人工纠正入口。
- 将 CDN 边缘节点当成真实资产所在地。
- 首页仍以任务列表或指标卡为第一视觉。
- 双写 Mongo 和 Neo4j 却没有重建与对账机制。

## 4. 用户工作流

### 4.1 创建项目并导入单位

1. 创建项目，填写项目名称、客户或归属、起止时间、说明。
2. 上传 CSV，字段至少包含单位名称和主域名；主域名可用分号分隔。
3. 系统为单位和主域名建立项目成员关系。
4. 对没有明确单位的域名发起组织归属查询；结果进入待确认或自动确认状态。
5. 用户可批量启动单位或主域名的扫描任务。

### 4.2 域名反查组织

1. 在全局搜索框输入域名。
2. 系统返回候选组织、来源、匹配时间和置信度。
3. 用户确认后，域名和单位建立 `OWNS` 关系；拒绝则保留查询记录但不建立事实关系。
4. 同一单位可有多个别名和多个主域名；法定名称与展示名称分开保存。

### 4.3 首页研判

1. 登录后进入上次项目，或显示项目选择页。
2. 首页显示图谱主画布、地图切片、单位覆盖、实时发现和当前项目上下文。
3. 用户点击地图上的城市，只保留该城市资产；点击图谱节点，地图联动定位。
4. 打开右侧 inspector 查看证据、归属、扫描任务和资产详情。
5. 选中一批单位或域名后，直接提交批量扫描或导出。

## 5. 信息架构

```text
项目 Project
  -> 单位 Organization
    -> 主域名 Root Domain
      -> 子域名 Domain
        -> IP
          -> 服务 Service
          -> 站点 Site
            -> URL
        -> CDN Provider / Edge IP
        -> 回源候选 Origin Candidate
    -> 证书 Certificate
    -> 漏洞 Finding
    -> 归属证据 Attribution Evidence
    -> 扫描任务 Task
```

项目是默认上下文，单位是组织归属层，主域名是批量操作边界，图谱是关系查询层，证据保存在 MongoDB。

## 6. 首页设计（方案 A）

### 6.1 布局

- 顶部：项目选择、全局搜索、当前项目风险摘要、批量操作入口。
- 左侧：项目切换、单位列表、单位风险标记、资产覆盖摘要。
- 中间：资产关系画布，支持筛选、缩放、按单位展开、按风险突出。
- 右侧：地理分布地图、当前节点 inspector、实时发现事件。
- 底部：轻量事件流，不作为独立大屏。

### 6.2 视觉约束

- 背景：浅灰白；内容面板白色；边框低对比度。
- 主文字：深灰蓝；次文字：中性灰。
- 强调色：深蓝灰，只用于交互和选中态。
- 风险色：红、橙、蓝；风险色只用于风险和严重度，不用于装饰。
- 图表节点：单位、域名、IP、服务、风险五类以内；其余节点降级为小点或不显示文字。
- 字号、间距和边框统一使用 8px 栅格与固定设计 token。
- 不使用渐变发光、粒子背景、3D 地球和装饰性动画。

### 6.3 图谱交互

- 默认只展示当前项目的高风险单位、主域名和直接关联节点。
- 初始最多返回 300 至 500 个节点；超限时按单位或域名聚合，用户点击后按需展开。
- 选中单位或域名时，右侧 inspector 展示关系摘要和证据入口。
- 支持按单位、域名、IP、风险、CDN、回源候选、最近变化筛选。
- 图谱节点与地图点使用同一个 selection store；筛选条件互相共享。

### 6.4 地图交互

- 中国地图为默认视图，世界地图可切换。
- 资产点按 IP 地理位置展示，不展示单位注册地址来代替资产位置。
- 点类型至少区分：资产 IP、CDN 边缘节点、回源候选、未定位资产。
- 地图使用聚类，避免大量点重叠；点击聚类后展开或过滤图谱。
- 私有地址和无 GeoIP 结果的资产进入“未定位”计数，不在地图上伪造位置。
- 地图和图谱必须共用同一个项目、单位、风险和时间筛选。

### 6.5 空态与错误态

- 无项目：显示创建项目或导入项目的引导。
- 项目中无单位：显示 CSV 导入入口和示例格式。
- 组织归属查询中：显示 pending，不阻塞资产和任务。
- Neo4j 不可用：首页退化为单位与资产列表，显示“关系图暂不可用”，不阻塞已有任务和扫描。
- GeoIP 不可用：地图显示“地理数据不可用”，图谱与列表正常。
- 外部归属服务不可用：保留待查询队列，允许重试和手工确认。

## 7. 领域模型

### 7.1 MongoDB 中的新集合

- `projects`：项目元数据、状态、起止时间、描述、创建者。
- `project_organizations`：项目与单位的关系、项目内展示名、排序、备注、覆盖状态。
- `organizations`：单位规范身份、法定名称、别名、类型、统一社会信用代码或备案主体信息。
- `organization_attributions`：域名到单位的候选归属、来源、原始返回、置信度、确认状态、查询时间。
- `project_domains`：项目内主域名、所属单位、导入来源、启用状态。
- `ip_locations`：IP 地理位置、国家、省、市、经纬度、ASN、数据源、更新时间。

现有 `assets`、`asset_observations`、`findings`、`tasks`、`task_events` 保留。任务增加可选 `projectId`、`organizationId`、`rootDomainId` 字段。

### 7.2 关系语义

- `Project -[:CONTAINS]-> Organization`
- `Organization -[:OWNS]-> Domain`，只对显式确认的主域名建立
- `Domain -[:SUBDOMAIN_OF]-> Domain`
- `Domain -[:RESOLVES_TO]-> IP`
- `Domain -[:USES_CDN]-> CDNProvider`
- `Domain -[:HAS_EDGE_IP]-> IP`
- `Domain -[:ORIGIN_CANDIDATE]-> IP`
- `IP -[:EXPOSES]-> Service`
- `IP -[:HOSTS]-> Site`
- `Site -[:SERVES]-> URL`
- `Certificate -[:COVERS]-> Domain`
- `Finding -[:FOUND_ON]-> Asset`
- `Task -[:DISCOVERED]-> Asset`
- `Attribution -[:CONFIRMS_OWNER]-> Organization`

边属性至少包含 `source`、`confidence`、`firstSeen`、`lastSeen`、`projectId`（需要项目隔离时）。原始 observations 不进入图，只通过 MongoDB id 关联。

### 7.3 项目隔离

- 资产节点可以为多个项目复用，避免同一 IP 或域名重复存储。
- 项目可见性由项目成员边和查询条件共同保证。
- 所有图谱、地图、搜索和详情接口必须带当前 projectId，并在服务端校验项目访问权限。
- 不允许前端通过修改 projectId 读取其他项目的关系。

## 8. Neo4j 架构

- Neo4j 是关系查询层和可视化投影，不是原始证据真相源。
- MongoDB 是任务、观测、发现、审计和归因的真相源。
- 资产和发现写入 Mongo 后，通过 graph outbox 事件驱动 projector 更新 Neo4j。
- projector 必须幂等：同一事件重复消费不会生成重复节点或边。
- 提供 `arl graph rebuild` 或等价维护命令，从 Mongo 重建图。
- 提供对账任务，按批比较 Mongo 资产和 Neo4j 节点数量、最后更新时间。
- Neo4j 写入失败不阻塞扫描主流程；失败事件保留并可重试。
- 图谱查询默认限制深度和节点数，服务端负责聚合，不把整个图返回前端。

建议节点：Project、Organization、Domain、IP、Service、Site、URL、Certificate、CDNProvider、Finding、Task。建议只把高价值关系投影到 Neo4j，避免 observation 级节点爆炸。

## 9. 外部数据源

### 9.1 组织归属

已知可用路径：

- 官方 JSON API：`GET https://openapi.chinaz.net/v1/1001/newicp`，参数为 `domain`、`APIKey`、`ChinazVer=1.0`，返回 CompanyName、CompanyType、SiteLicense、VerifyTime、Domain、ServiceLicence、ContentTypeName。
- 页面级 fallback：`GET https://icp.chinaz.com/{domain}`，使用用户提供的合法会话 Cookie，从主办单位名称、主办单位性质、审核时间等字段解析。

设计约束：

- 官方 APIKey 优先。
- Cookie 只通过后端环境变量或只读文件注入，不进入前端、日志、数据库或 Git。
- 页面 fallback 不做验证码绕过，不做签名伪造，不做高频抓取。
- 查询按域名缓存，默认 TTL 7 天；人工刷新可强制重查。
- 批量查询走队列，遵守供应商建议的最小请求间隔，失败进入退避重试。
- 结果保存原始返回、来源、查询时间、置信度和人工确认状态。
- 法定名称和展示别名分开；例如法定名称可能是“四川吉利学院”，展示名可为“吉利学院”。

### 9.2 GeoIP

- 新增 `GeoResolver` 接口，默认使用本地离线库，避免为每个 IP 调外部接口。
- 首选 MaxMind GeoLite2 City/ASN；可选 ip2region 适配器提升中国城市精度。
- 数据源、数据库版本、更新时间、国家和城市置信度写入 `ip_locations`。
- 私有 IP、保留地址和无法解析的 IP 不进入地图点位。
- CDN 边缘 IP 与回源候选 IP 必须使用不同 location kind，不能混淆。

### 9.3 CDN 与回源候选

- 保留现有 DNS/CDN 识别结果，区分“域名直接解析到的资产 IP”和“CDN 边缘 IP”。
- 图谱必须显示 CDNProvider 节点和 edge 关系。
- 回源候选是带来源和置信度的推断关系，不能标成确定事实。
- v1 先完成数据模型、展示和筛选；回源发现算法可按可用数据源逐步补强。

## 10. API 与 DTO

新增或扩展的公开接口：

- `GET /api/v1/projects`
- `POST /api/v1/projects`
- `GET /api/v1/projects/{id}`
- `PATCH /api/v1/projects/{id}`
- `POST /api/v1/projects/{id}/organizations/import`
- `GET /api/v1/projects/{id}/organizations`
- `POST /api/v1/organizations/lookup`，输入域名，返回候选组织与来源
- `POST /api/v1/organizations/{id}/attributions/{attributionId}/confirm`
- `POST /api/v1/organizations/{id}/attributions/{attributionId}/reject`
- `GET /api/v1/projects/{id}/graph?focus=&depth=&types=&risk=&limit=`
- `GET /api/v1/projects/{id}/map?bbox=&risk=&types=`
- `GET /api/v1/projects/{id}/events`，SSE
- `POST /api/v1/tasks` 扩展 `projectId`、`organizationId`、`rootDomainId`

Graph DTO 至少包含：

- `nodes[]`：id、type、label、risk、geo、summary、source
- `edges[]`：id、type、source、target、confidence、kind
- `truncated`、`aggregated`、`limit`

Map DTO 至少包含：

- `points[]`：id、label、kind（asset/edge/origin）、lat、lon、country、province、city、risk、assetCount、confidence
- `clusters[]`：id、label、lat、lon、assetCount、risk
- `unlocatedCount`

所有接口必须使用现有认证、CSRF、RBAC 和 problem+json 错误格式。外部供应商凭据永远不出现在 API 响应中。

## 11. 数据流与异步

```text
CSV 导入
  -> Project + Organization + Root Domain
  -> Organization attribution job
  -> Chinaz API / cookie fallback
  -> Attribution evidence in MongoDB
  -> Graph projector
  -> Neo4j owner edges

扫描任务
  -> DNS / port / HTTP / webintel / findings
  -> assets + observations + findings in MongoDB
  -> graph outbox event
  -> Neo4j projector
  -> graph/map query API
```

- 批量导入是异步任务，返回 jobId 和进度。
- 组织归属查询和 GeoIP 富化不阻塞任务提交。
- Graph projector 消费独立队列，失败进入重试和 DLQ。
- 首页通过 SSE 或短轮询刷新项目摘要和事件。

## 12. 安全与合规

- 项目权限在服务端校验，前端 projectId 不作为授权依据。
- Chinaz Cookie 和 APIKey 只从环境变量或只读 secret 文件读取。
- 供应商请求固定域名和固定接口，禁止用户控制目标 URL。
- 所有外部请求设置超时、最大响应体、重试上限和速率限制。
- 供应商原始返回可能含个人信息或敏感备案信息，按敏感数据存储和展示权限处理。
- 归属确认、拒绝、合并和批量导入写审计事件。
- 不把 Cookie、APIKey、完整原始响应写入普通日志。

## 13. 兼容与迁移

- 现有 Scope 保留为 Legacy 资产组，不自动伪装成 Project。
- 新项目和新单位使用新集合与图关系；旧任务继续可读。
- 现有 `/assets/{kind}`、`/findings/{kind}`、`/scopes` 接口保留，但新首页不以其为主入口。
- `arl migrate` 增加 Neo4j 约束、索引和 graph outbox 初始化。
- 新增 `arl graph rebuild` 用于回填和灾难恢复。
- 对已有资产按需回填 GeoIP 和图关系，不阻塞升级。

## 14. 测试与验收

### 单元测试

- Chinaz JSON 响应字段映射和错误码处理。
- Cookie fallback HTML 解析，使用固定 fixture，不依赖真实网络。
- 组织别名、法定名称、人工确认和拒绝状态机。
- 图谱边去重、幂等 upsert 和截断规则。
- GeoIP 结果转换、私有 IP 排除和未定位计数。
- 项目隔离过滤在所有查询入口生效。

### 集成测试

- MongoDB + Neo4j Docker 环境下的项目创建、单位导入、归属确认、图谱读取。
- graph projector 重复消费同一事件不产生重复边。
- `graph rebuild` 后节点和边数量与 Mongo 对账一致。
- Neo4j 不可用时首页降级为列表视图。
- Chinaz 超时、限流、APIKey 错误和 Cookie 过期时任务进入可重试状态。
- 100 家单位、300 个主域名、数千资产的导入和首页查询在可接受时间内完成。

### 前端与 E2E

- 创建项目并导入单位。
- 输入 `guc.edu.cn` 返回“四川吉利学院”和来源信息。
- 确认归属后，单位出现在项目首页。
- 图谱地图联动：点击城市过滤图谱，点击节点定位地图。
- 切换中国/世界地图、过滤 CDN 边缘和未定位资产。
- 打开资产 inspector 查看归属、证据、任务和地图位置。
- 空项目、无 GeoIP、Neo4j 降级、供应商错误四种状态均有明确 UI。
- 1920x1080、1440px 桌面和 1280px 窄屏布局不重叠。

### 验收标准

- 首次登录后看到项目工作区，而不是通用模块后台。
- 用户可以在一个项目内管理 100 家单位和多个主域名。
- 域名反查结果可追溯、可确认、可拒绝。
- 首页图谱与地图共享筛选，CDN 边缘和回源候选不混淆。
- Neo4j 可以从 Mongo 重建，且查询失败不影响扫描与审计。
- 页面不使用原始 JSON 块作为主要资产展示。

## 15. 实施边界与默认选择

- 首页先按方案 A 实现，后续页面沿用同一设计 token 逐步改造。
- 图与地图前端使用 ECharts + GeoJSON，避免同时引入多套可视化框架。
- 默认 Neo4j 5.x，Docker Compose 增加服务、健康检查和持久卷。
- 默认使用本地 GeoIP 数据库；具体供应商可通过适配器替换。
- 官方 Chinaz JSON API 优先，Cookie 页面 fallback 仅作为过渡。
- 项目内默认单租户 RBAC；细粒度项目成员权限不在 v1 范围内。
- CDN 回源发现算法不在 v1 承诺完整实现，但数据模型、展示和置信度语义必须预留。

## 16. 已决定的实现假设

以下是设计阶段确认的默认选择，实施时不再向用户询问；若后续需要改变，应作为显式变更处理。

### 16.1 项目访问

v1 沿用现有全局 RBAC：登录用户按 viewer/operator/admin 角色访问项目，不新增项目级成员权限。项目隔离用于查询和 UI 上下文，不用于替代用户授权。项目级 ACL 留到后续版本。

### 16.2 资产归属项目

一个资产出现在项目图谱中，必须满足以下任一条件：

- 能从该项目已确认的主域名通过明确关系遍历到；
- 由携带该项目 projectId 的任务直接发现。

共享 IP、ASN 或 CDN 基础设施只有在与项目域名或项目任务存在直接关系时才进入项目视图，不能因为它在其他项目中出现过就自动带入。

### 16.3 归属与风险

- 域名归属是全局规范化事实，项目内的确认状态独立保存；同一域名可保留多条候选归属，但项目内默认只有一个主归属。
- Finding 是任务和项目级事实；同一资产在不同项目中的风险分别计算。
- 默认风险取当前项目内未关闭 Finding 的最高严重度；节点颜色和地图点位按该值计算。
- 首页时间范围默认近 30 天，按 Finding 的 firstSeen/lastSeen 过滤；切换时间范围不改变历史事实。

### 16.4 自动确认与别名

- 组织归属不得自动确认。即使供应商只返回一个高置信度候选，也只进入 pending，必须由操作员确认或拒绝。
- v1 的“合并别名”只允许在同一个 Organization 内添加展示别名；不合并两个 Organization 实体，不重写历史关系。
- 两个 Organization 的全局合并是后续管理员功能，必须有审计和可回滚设计。

### 16.5 覆盖口径

单位页和首页分别显示两个覆盖值：

- 归属覆盖：已导入主域名且至少一个域名已完成组织归属确认。
- 扫描覆盖：单位至少有一个主域名进入扫描任务，且任务状态为 succeeded、partial 或 running。

不把“扫描覆盖”折算成单一百分比掩盖两个不同问题。

### 16.6 数据源与地理

- 官方 Chinaz JSON API 优先，Cookie 页面 fallback 作为过渡；两者都必须经过统一 provider 接口、限速、缓存和审计。
- GeoIP 默认使用本地 MaxMind GeoLite2 City/ASN；中国城市精度不足时可选接入 ip2region 适配器。没有地理数据的 IP 进入未定位计数。
- CDN 边缘节点和回源候选使用不同的 location kind；v1 不承诺完整回源算法，只承诺数据模型、展示和置信度语义。

### 16.7 兼容与数据保留

- 现有 Scope 保留为 Legacy 资产组，不自动映射为 Project。
- 原始归属响应随项目保留，只有 operator/admin 可查看完整原始字段；普通 viewer 只看结构化摘要。
- 供应商 Cookie/APIKey 不落库、不进入前端、不写普通日志；只从环境变量或只读文件读取。

### 16.8 仍需用户提供的外部资源

以下不是设计决策，而是运行或验收时需要的外部资源：

- 可用的 Chinaz APIKey，或可持续更新的合法页面会话 Cookie。
- 生产用 GeoIP 数据库文件及其授权/更新方式。
- 若要求真实回源发现，提供可用的历史 DNS、被动 DNS、证书或商业数据源。

