![Red Copilot — Security Engineering for the Agentic Era](https://raw.githubusercontent.com/red-copilot/.github/main/assets/red-copilot-banner.svg)

# ARL 3.0 · Asset Reconnaissance Lighthouse

**Authorization-first External Attack Surface Management.**

**授权优先的外部攻击面管理与资产侦察平台。** ARL 3.0 以组织为主线关联域名、资产、风险与任务，构建可追溯的侦察工作流。

ARL 3.0 is a Go-based rewrite for authorized asset discovery and external attack surface management. A single `arl` process serves the embedded React console, REST/SSE API, task workers, and scheduler. PostgreSQL stores application data; Redis Streams provides durable asynchronous work.

ARL 3.0 面向已获授权的资产发现与外部攻击面管理。单个 `arl` 进程承载嵌入式 React 控制台、REST/SSE API、任务 worker 与调度器；PostgreSQL 保存业务数据，Redis Streams 提供持久化异步任务队列。

> **Authorized use only:** Scan only assets, domains, IPs, and sites for which you have explicit permission. Read [Disclaimer.md](Disclaimer.md) before use.
>
> **仅限授权使用：** 仅扫描已获得明确授权的资产、域名、IP 与站点。使用前请阅读 [Disclaimer.md](Disclaimer.md)。

<p>
  <a href="docs/ARCHITECTURE.md">Architecture</a> ·
  <a href="docs/PARITY.md">Feature parity</a> ·
  <a href="docs/EASM-OPERATIONS.md">EASM operations</a> ·
  <a href="#api">API</a> ·
  <a href="#安全模型">Security model / 安全模型</a>
</p>

> **Compatibility note:** Version 3.0 is a rewrite, not an in-place upgrade. It does not migrate legacy MongoDB data and is not compatible with the old `/api`; PostgreSQL 18 is the application database.
>
> **兼容性说明：** 3.0 是重写版本，不会自动迁移旧版 MongoDB 数据，也不兼容旧 `/api`；当前业务主库为 PostgreSQL 18。

---
## 功能概览

### 组织化 EASM

- 以 `organization` 组织资产、域名、风险和任务，而不是把扫描结果散落在不同任务里。
- 域名归属工作流：ICP 查询 → 候选域名 → 人工确认/驳回/转移，保留来源、置信度和原始证据。
- 组织总览、世界地图、关系图、跨组织资产/风险/任务视图，以及 CSV / XLSX 报告。
- Neo4j 关系投影、GeoIP 地图和 ICP_Query 查询均为可选组件；缺失时自动降级，不影响资产与列表视图。

### 资产发现

- 支持域名、子域名、IP、CIDR 等目标，内置黑名单与 SSRF 防护。
- 进程内 ProjectDiscovery SDK：`subfinder` → `dnsx` → `naabu` → `tlsx` → `httpx` → `katana`。
- 端口与服务识别、TLS 证书、HTTP 标题/状态/技术栈、站点爬虫、截图（可选 Chromium）。
- 标准化资产模型：canonical asset + 每次任务的 observation；历史任务可回看，报告按任务快照生成。

### 风险与情报

- 指纹库与布尔规则 DSL，支持内置与自定义指纹。
- WIH / JavaScript 敏感信息收集、文件泄漏检测。
- 96 个 PoC / sniffer / brute 插件，覆盖常见 HTTP、SSH、MySQL、PostgreSQL、MSSQL、MongoDB、Redis、FTP、SMTP、IMAP、POP3 路径。
- 官方 `nuclei` Go SDK，默认只加载 `assets/nuclei` 下的 HTTP 模板。
- GitHub 搜索与监控、FOFA 查询、站点监控与计划任务。

### 平台与交付

- React 19 + TypeScript + Vite 控制台：TanStack Query/Table、Radix UI、Tailwind CSS、ECharts。
- OpenAPI 3.1 文档、RFC 7807 错误响应、SSE 任务事件、Prometheus 指标。
- `viewer` / `operator` / `admin` 三级 RBAC、API Key、CSRF、审计日志与结构化日志。
- 单个 Go 二进制（扫描资产外置）+ systemd 原生部署；Docker 仅用于 PostgreSQL、Redis。所有后台组件都在同一个 ARL 进程内运行。

## 架构

```text
                     ┌──────────────────────────────────────┐
  Browser / API ────▶│ web                                  │────▶ PostgreSQL 18
                     │ SPA · REST · SSE · OpenAPI · metrics │
                     └───────────────┬──────────────────────┘
                                     │ outbox
                                     ▼
                              Redis Streams
                                     │
                     ┌───────────────▼──────────────────────┐
                     │ 同一 arl 进程内的 worker 循环          │
                     │ 扫描步骤：DNS / 端口 / HTTP / 指纹 / PoC │
                     └──────────────────────────────────────┘

  同一进程内的 scheduler ── PostgreSQL lease ──▶ 到期任务
  可选外部服务：Neo4j（关系图）、MaxMind GeoIP（地图）、ICP_Query
```

- Web/API 层提供嵌入式控制台、REST、SSE、健康检查和指标。
- 同一 `arl` 进程消费 Redis Streams、执行任务步骤、派发定时任务并投影图数据。
- `arl all` 是唯一运行入口；生产使用持久化 Redis 队列。

更完整的设计见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)，旧版功能对照见 [docs/PARITY.md](docs/PARITY.md)。

## 快速开始

### 依赖

| 组件 | 版本 | 说明 |
| --- | --- | --- |
| Go | 1.27+ | 构建后端和单二进制 |
| Node.js | 22.12+ | 仅构建前端，构建后打入 `arl` |
| PostgreSQL | 18+ | 业务数据、任务租约与 outbox |
| Redis | 7.0+ | 持久化任务队列；生产默认使用 Redis |
| Chromium | 可选 | 宿主安装后可启用站点截图 |
| Docker + Compose | 可选 | 仅用于 PostgreSQL、Redis |

### 方式一：基础设施容器 + 原生 ARL（推荐）

React 控制台由 Vite 构建到 `internal/webui/dist`，随后通过 `go:embed` 进入 `arl` 二进制。运行时不启动前端容器，也不需要 Node.js。

```bash
# 1. 构建单二进制（同时构建并嵌入前端）
make build

# 2. 准备应用配置；本地 HTTP 调试关闭 Secure Cookie
cp config.example.yaml config.yaml
export ARL_AUTH_ACCESS_SECRET="$(openssl rand -base64 48)"
export ARL_API_KEY_PEPPER="$(openssl rand -base64 32)"
export ARL_ADMIN_PASSWORD='<initial-admin-password>'
export ARL_COOKIE_SECURE=false

# 3. 仅在本地容器中运行 PostgreSQL 与 Redis，端口只绑定 127.0.0.1
make infra-up

# 4. 初始化并启动单节点服务
./arl migrate --config config.yaml
./arl all --config config.yaml
```

访问 <http://localhost:5003>。`config.example.yaml` 中的数据库密码只用于本地试用；生产环境应通过 `ARL_POSTGRES_DSN`、`ARL_REDIS_URL` 和密钥管理系统覆盖，并启用 TLS（配置 `server.tlsCert` / `server.tlsKey`，或在前置反向代理终止 TLS）。

### 方式二：systemd 安装

```bash
make build
sudo ./deploy/install.sh

sudoedit /etc/arl/config.yaml /etc/arl/arl.env
sudo /opt/arl/arl migrate --config /etc/arl/config.yaml
sudo systemctl enable --now arl
```

安装脚本把二进制和外部扫描资产放到 `/opt/arl`，配置与 `arl.env` 放到 `/etc/arl`，运行数据放到 `/var/lib/arl`。系统只安装一个 `arl.service`，它运行完整的单节点服务。基础设施仍可用 `make infra-up` 启动，或改用宿主原生 PostgreSQL/Redis。

### 可选 EASM 组件

Neo4j、ICP_Query 和 GeoIP 不属于默认 Docker 基础设施。需要这些能力时，连接外部或宿主原生实例，并在启动 ARL 前加载环境变量：

```bash
cp deploy/.env.easm.example deploy/.env.easm
set -a
source deploy/.env.easm
set +a
./arl all --config config.yaml
```

详细配置与降级行为见 [docs/EASM-OPERATIONS.md](docs/EASM-OPERATIONS.md)。

## 配置

配置优先级为：`config.yaml` → `ARL_*` 环境变量覆盖。常用项：

| 配置键 | 环境变量 | 说明 |
| --- | --- | --- |
| `server.listen` | `ARL_SERVER_LISTEN` | 监听地址，默认 `:5003` |
| `server.publicURL` | `ARL_SERVER_PUBLIC_URL` | 对外访问地址 |
| `postgres.dsn` | `ARL_POSTGRES_DSN` | PostgreSQL 连接串 |
| `postgres.maxOpenConns` | `ARL_POSTGRES_MAX_OPEN_CONNS` | 最大连接数，默认 25 |
| `postgres.maxIdleConns` | `ARL_POSTGRES_MAX_IDLE_CONNS` | 空闲连接数，默认 10 |
| `postgres.connMaxLifetime` | `ARL_POSTGRES_CONN_MAX_LIFETIME` | 连接生命周期，默认 `30m` |
| `redis.url` | `ARL_REDIS_URL` | Redis 连接串 |
| `queue.driver` | `ARL_QUEUE_DRIVER` | `redis` 或 `memory` |
| `auth.accessSecret` | `ARL_AUTH_ACCESS_SECRET` | 访问令牌签名密钥，必填 |
| `auth.apiKeyPepper` | `ARL_API_KEY_PEPPER` | API Key 哈希 pepper |
| `auth.bootstrapPassword` | `ARL_ADMIN_PASSWORD` | 首次启动的管理员密码；留空则随机生成并打印 |
| `auth.cookieSecure` | `ARL_COOKIE_SECURE` | HTTPS 环境应为 `true` |
| `scan.blackCIDRs` | — | 出站黑名单 |
| `scan.allowPrivate` / `scan.allowLoopback` | `ARL_ALLOW_PRIVATE` / `ARL_ALLOW_LOOPBACK` | 仅限授权内网/本地靶场 |
| `scan.maxHosts` / `scan.maxPorts` / `scan.maxRuntime` | — | 任务规模与时长上限 |
| `scanners.nuclei.templatesDir` | `ARL_NUCLEI_TEMPLATES_DIR` | nuclei 模板目录 |
| `tools.chromiumPath` | `ARL_CHROMIUM_PATH` | Chromium 可执行文件；留空自动探测 |
| `github.token` | `ARL_GITHUB_TOKEN` | GitHub 搜索/监控所需 |
| `neo4j.uri` | `ARL_NEO4J_URI` | 启用关系图时配置 |

完整配置见 [config.example.yaml](config.example.yaml)。修改配置后运行 `./arl doctor --config config.yaml`，可检查配置、PostgreSQL、Redis、GeoIP、Neo4j、DNS 与模板状态。

生产环境的 `postgres.dsn` 应启用 TLS（如 `sslmode=require` 或 `verify-full`），并通过密钥管理系统注入密码。

**不要提交真实密钥。** `config.example.yaml` 只作为模板；systemd 部署应把敏感覆盖项写入权限为 `0640` 的 `/etc/arl/arl.env` 或外部密钥管理系统。

## 命令行

| 命令 | 说明 |
| --- | --- |
| `arl all` | 唯一运行入口：API、嵌入式控制台、任务消费、调度器与图投影 |
| `arl migrate` | 执行 PostgreSQL AutoMigrate、写入默认策略并同步 PoC 插件元数据 |
| `arl doctor` | 检查配置、依赖、队列、资源与 DNS |
| `arl health` | 进程健康检查，例如 `--url http://127.0.0.1:5003/healthz` |
| `arl admin create-user` | 创建用户，可指定 `--role viewer/operator/admin` |
| `arl admin reset-password` | 重置密码 |
| `arl poc sync` | 将插件元数据同步到 PostgreSQL |
| `arl graph rebuild` | 重建 Neo4j 关系投影（启用关系图时） |
| `arl version` | 打印版本 |

### 部署与运维

- PostgreSQL 迁移从空库开始：旧版 MongoDB 数据不会自动导入；启用 Neo4j 时升级后执行一次 `./arl graph rebuild`。
- systemd：`sudo ./deploy/install.sh` 安装单节点 `arl.service`、二进制和外部资产；配置与环境文件分别位于 `/etc/arl/config.yaml`、`/etc/arl/arl.env`，数据目录为 `/var/lib/arl`。
- Redis 切换/升级前运行 `./scripts/redis-preflight.sh --config config.yaml`，检查连通性、版本、AOF、内存策略、队列深度并执行 `arl doctor`；迁移切换可加 `--require-empty`。
- EASM 在线冒烟使用 `./scripts/easm-live-smoke.sh`；默认只做只读检查，设置 `EASM_LIVE_SCAN=1` 才会发起有界扫描。

## API

- OpenAPI 3.1：`/api/v1/openapi.json`
- 文档入口：`/api/v1/docs`
- 认证方式：
  - 浏览器：HttpOnly Cookie `arl_access`，写请求携带 `X-CSRF-Token`；
  - 自动化：`X-API-Key`，范围分为 `read` / `scan` / `admin`。
- 主要资源：
  - 任务：`/api/v1/tasks`、`/tasks/{id}/events`（SSE）
  - 资产与风险：`/assets`、`/findings`
  - 组织与 EASM：`/organizations/*`、`/domain-lookups`、`/map`
  - 扫描策略：`/policies`、`/scopes`、`/fingerprints`、`/poc/plugins`
  - 自动化：`/monitors`、`/schedules`、`/github/*`
  - 导出与审计：`/exports`、`/audit-events`、`/system/info`
- 列表接口统一支持 `page`、`pageSize`、`sort`、`q` 及资源专属过滤器。
- 错误响应使用 `application/problem+json`（RFC 7807）；健康检查为 `/healthz`、`/readyz`，指标为 `/metrics`。

## 安全模型

- Argon2id 密码哈希；默认 15 分钟访问 JWT、7 天可轮换刷新令牌；刷新令牌重放会撤销整个会话族。
- `viewer` / `operator` / `admin` 三级 RBAC；API Key 支持 `read` / `scan` / `admin` 范围。
- 扫描类出站 HTTP 使用安全拨号器，校验解析后的 IP；默认阻止回环、链路本地、云元数据地址和配置的黑名单。
- 写请求默认要求 CSRF Token；安全响应头、请求 ID、审计日志与结构化日志默认开启。
- 写操作记录审计日志；ICP 原始证据独立存储且不包含 provider cookie，候选域名在确认前不会进入地图、关系图和组织统计。
- 扫描默认拒绝私网与回环地址；仅在明确授权的内网或靶场中通过 `scan.allowPrivate` / `scan.allowLoopback` 放开。

## 开发与测试

```bash
make test       # Go 测试 + 前端类型检查 + Vitest
make race       # Go race 测试
make lint       # gofmt 检查 + vet + 前端 lint/typecheck
make verify     # go mod verify + vet + 全部测试 + 前端构建
```

前端单独开发：

```bash
cd web
npm ci
npm run dev          # Vite 开发服务器，/api 代理到 127.0.0.1:5003
npm run test:e2e     # Playwright（排除 @visual）
npm run test:visual  # Playwright 视觉回归
npm run generate:api # 根据 internal/httpapi/openapi.json 生成类型
```

CI 会构建前端产物、运行 Go 测试（含 race）、构建原生 `arl` 二进制，并校验基础设施 Compose 配置。

## 目录结构

```text
cmd/arl                CLI 入口与角色分发
internal/app           依赖装配
internal/httpapi       REST / SSE / OpenAPI / 鉴权
internal/store         PostgreSQL/GORM 仓储、索引与迁移
internal/modules       discovery、webintel、poc、tasking、monitoring、
                       github、reporting、identity、easm、system
internal/platform      config、日志、队列、netsafe、通知、ProjectDiscovery 适配
internal/webui         嵌入式控制台构建产物
web/                   React + TypeScript + Vite 源码、Vitest、Playwright
assets/                字典、指纹、WIH 规则与 nuclei 模板
deploy/                基础设施 Compose、systemd 单元、环境模板与安装脚本
docs/                  架构、功能对照、扫描器差异、EASM 运维
scripts/               Redis 预检、EASM 在线冒烟
```

## 已知限制与有意差异

- 3.0 是重写而非原地升级：**不迁移旧版 MongoDB 数据，也不兼容旧 `/api`**；升级后业务主库为 PostgreSQL 18。
- OS 识别仅使用 naabu 服务元数据，无法获得 OS 类型时省略。
- 截图依赖 Chromium；未安装浏览器时截图步骤会失败并被标记为可选步骤，不影响其他扫描。
- 96 个插件中，RDP、CobaltStrike、Shiro CBC/GCM 等仍为注册扩展点，协议实现尚需补充。
- nuclei 默认只运行 `assets/nuclei` 下的 HTTP 模板；interactsh、code、file、headless 模板默认关闭。
- 结果集/风险巡航仍为部分实现，详见 [docs/PARITY.md](docs/PARITY.md)。
- GitHub 搜索与监控需要配置 API Token。
- 所有扫描引擎与 ProjectDiscovery 都通过进程内 SDK 调用；结果时间线与指纹会与 nmap / massdns 版本存在差异，见 [docs/SCANNER-DIFFERENCES.md](docs/SCANNER-DIFFERENCES.md)。

## 文档

| 文档 | 内容 |
| --- | --- |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 运行时角色、模块边界、任务生命周期、数据模型 |
| [docs/PARITY.md](docs/PARITY.md) | 旧版 Flask 功能到新实现的对照 |
| [docs/SCANNER-DIFFERENCES.md](docs/SCANNER-DIFFERENCES.md) | ProjectDiscovery SDK 与旧扫描器的差异 |
| [docs/PROJECTDISCOVERY-SDK.md](docs/PROJECTDISCOVERY-SDK.md) | SDK 接入与 `pdtools` 映射 |
| [docs/EASM-OPERATIONS.md](docs/EASM-OPERATIONS.md) | 组织、域名归属、地图/关系图与本地 EASM 栈 |
| [docs/THIRD_PARTY_NOTICES.md](docs/THIRD_PARTY_NOTICES.md) | 第三方组件与规则许可 |

## 免责声明与许可

本项目仅用于合法的授权安全测试。使用者应自行确保目标授权、数据合规与操作边界；因不当使用造成的后果由使用者承担。完整条款见 [Disclaimer.md](Disclaimer.md)。

代码以 MIT 许可发布，见 [LICENSE.md](LICENSE.md)。第三方依赖、字典、指纹、WIH 规则和 PoC 规则的版权与许可声明保留在仓库中。
