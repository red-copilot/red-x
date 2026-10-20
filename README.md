# ARL · 资产侦察灯塔

**Authorization-first external attack surface management and asset reconnaissance.**

ARL 是面向**已授权安全测试**的外部攻击面管理（EASM）与资产侦察平台。它以组织为主线，把域名归属、资产、风险、任务和报告串成一条可追溯的链路；除可选的 Chromium 截图外，所有扫描引擎都在 Go 进程内运行，不需要安装 nmap、massdns 或 ProjectDiscovery CLI。

当前版本是 3.0：一个 Go 二进制提供 `web`、`worker`、`scheduler` 三种角色，React 控制台构建后直接嵌入二进制，PostgreSQL 18 保存业务数据，Redis Streams 承载异步任务。

> **仅用于已获得明确授权的资产、域名、IP 和站点测试。** 使用前请阅读 [Disclaimer.md](Disclaimer.md)。

> **与旧版 ARL 的关系：** 3.0 是 Go 重写而非原地升级，不迁移旧版 MongoDB 数据，也不兼容旧 `/api`；当前版本以 PostgreSQL 18 作为唯一业务主库。旧功能与新实现的对照见 [docs/PARITY.md](docs/PARITY.md)。

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
- Docker Compose、systemd、单二进制部署；`worker` 可水平扩展，`scheduler` 通过 PostgreSQL 租约选主。

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
                     │ worker × N                           │
                     │ 扫描步骤：DNS / 端口 / HTTP / 指纹 / PoC │
                     └──────────────────────────────────────┘

  scheduler ── PostgreSQL lease ──▶ 到期的 monitor / schedule
  可选：Neo4j（关系图）、MaxMind GeoIP（地图）、ICP_Query sidecar
```

- **web**：无状态，除 PostgreSQL 外不保存会话；提供控制台、API、SSE、健康检查和指标。
- **worker**：消费 Redis Streams，执行单个任务步骤；可并发运行多个实例。
- **scheduler**：通过 PostgreSQL 租约保证同一时间只有一个实例派发定时任务。
- **all / dev**：本地开发模式，在一个进程里运行三种角色，可配合内存队列。

更完整的设计见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)，旧版功能对照见 [docs/PARITY.md](docs/PARITY.md)。

## 快速开始

### 依赖

| 组件 | 版本 | 说明 |
| --- | --- | --- |
| Go | 1.27+ | 构建后端 |
| Node.js | 22.12+ | 仅构建前端 |
| PostgreSQL | 18+ | 业务数据、任务租约与 outbox |
| Redis | 7.0+ | 多进程/生产队列；本地单进程可用内存队列 |
| Chromium | 可选 | 站点截图 |

### 方式一：Docker Compose

先修改 `deploy/config.docker.yaml`，至少替换 `auth.accessSecret`、`auth.apiKeyPepper`、`bootstrapPassword`，并通过 `POSTGRES_PASSWORD` / `REDIS_PASSWORD` 设置基础设施密码。示例配置中的默认密码**只能用于本地试用**。

```bash
# 自定义 Redis/PostgreSQL 密码时，Compose 会同步注入连接串
export REDIS_PASSWORD='<strong-redis-password>'
export POSTGRES_PASSWORD='<strong-postgres-password>'

docker compose -f deploy/compose.yaml up -d --build

# 注册 PoC 插件元数据（可选）
docker compose -f deploy/compose.yaml exec web arl migrate --config /etc/arl/config.yaml
```

访问 <http://localhost:5003>。默认用户名为 `admin`，密码为 `bootstrapPassword` 的值；若留空，一次性密码会打印在日志中，首次登录后必须修改。

生产部署请启用 TLS（配置 `server.tlsCert` / `server.tlsKey`，或在前置反向代理终止 TLS），并保持 `auth.cookieSecure: true`。

### 方式二：从源码运行

```bash
make build

cp config.example.yaml config.yaml
export ARL_AUTH_ACCESS_SECRET="$(openssl rand -base64 48)"
export ARL_API_KEY_PEPPER="$(openssl rand -base64 32)"
export ARL_ADMIN_PASSWORD='<initial-admin-password>'

# 本地单进程开发：内存队列 + 允许 HTTP 访问
export ARL_QUEUE_DRIVER=memory
export ARL_COOKIE_SECURE=false

./arl migrate --config config.yaml
./arl all --config config.yaml
```

PostgreSQL 需要先运行；启动角色会在 advisory lock 保护下执行幂等 AutoMigrate。生产环境建议拆分为三个角色：

```bash
./arl web --config config.yaml
./arl worker --config config.yaml
./arl scheduler --config config.yaml
```

### 可选 EASM 组件

需要关系图、ICP 归属证据或地图时，叠加 `deploy/compose.easm.yaml`，并准备 Neo4j、GeoIP MMDB 与授权使用的 ICP cookie：

```bash
cp deploy/.env.easm.example deploy/.env.easm
mkdir -p deploy/secrets deploy/geoip
docker compose --env-file deploy/.env.easm \
  -f deploy/compose.yaml -f deploy/compose.easm.yaml up -d --build
```

详细步骤与降级行为见 [docs/EASM-OPERATIONS.md](docs/EASM-OPERATIONS.md)。

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

**不要提交真实密钥。** `deploy/config.docker.yaml` 中的默认值只用于本地演示，部署前必须替换。

## 命令行

| 命令 | 说明 |
| --- | --- |
| `arl web` | API、SSE、OpenAPI 与嵌入式控制台 |
| `arl worker` | 消费 Redis Streams 任务，可加 `--queues` 限定队列 |
| `arl scheduler` | 监控器与计划任务调度（PostgreSQL 租约选主） |
| `arl all` / `arl dev` | 开发模式，同时运行 web、worker、scheduler |
| `arl migrate` | 执行 PostgreSQL AutoMigrate、写入默认策略并同步 PoC 插件元数据 |
| `arl doctor` | 检查配置、依赖、队列、资源与 DNS |
| `arl health` | 容器健康检查，例如 `--url http://127.0.0.1:5003/healthz` |
| `arl admin create-user` | 创建用户，可指定 `--role viewer/operator/admin` |
| `arl admin reset-password` | 重置密码 |
| `arl poc sync` | 将插件元数据同步到 PostgreSQL |
| `arl graph rebuild` | 重建 Neo4j 关系投影（启用关系图时） |
| `arl version` | 打印版本 |

### 部署与运维

- PostgreSQL 迁移从空库开始：旧版 MongoDB 数据不会自动导入；启用 Neo4j 时升级后执行一次 `./arl graph rebuild`。
- systemd：`sudo ./deploy/install.sh` 安装到 `/opt/arl`，配置到 `/etc/arl`，数据目录为 `/var/lib/arl`。
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

CI 会构建前端、运行 Go 测试（含 race）、构建镜像，并检查镜像内不存在 `nmap`、`massdns`、`nuclei`、`python` 等外部扫描器/运行时。

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
deploy/                Docker、Compose、systemd、安装脚本
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
