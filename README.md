# ARL · Asset Reconnaissance Lighthouse

ARL 是一个面向授权安全测试的资产侦察平台。当前代码库已从 Flask/Celery/Python 重构为 **Go 模块化单体 + Vue 3 控制台**。

> 仅用于已获得明确授权的资产、域名、IP 和站点测试。使用前请阅读 [Disclaimer.md](Disclaimer.md)。

## 架构

- 同一个 Go 二进制提供 `web`、`worker`、`scheduler` 三个角色。
- MongoDB 保存任务、资产、发现、监控和审计数据。
- Redis Streams 承载异步任务；开发模式可使用内存队列。
- DNS、端口、HTTP、TLS、指纹、WIH、文件泄漏和 PoC 引擎均在 Go 进程内实现。
- 前端为 Vue 3 + TypeScript + Ant Design Vue，构建后嵌入 Go 二进制。

详细设计见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)，功能对照见 [docs/PARITY.md](docs/PARITY.md)。

## 快速开始

### 依赖

- Go 1.27+
- Node.js 22+（仅构建前端）
- MongoDB 8.0
- Redis 7.0+（生产/多进程模式；Ubuntu 可 `apt install redis-server`）
- Chromium（可选，站点截图）

### 构建

```bash
cd web && npm ci && npm run build && cd ..
go build -o arl ./cmd/arl
```

### 配置

```bash
cp config.example.yaml config.yaml
export ARL_AUTH_ACCESS_SECRET="$(openssl rand -base64 48)"
export ARL_ADMIN_PASSWORD="change-this-immediately"
```

生产环境请修改 `mongo.uri`、`redis.url`、`auth.accessSecret`、`auth.apiKeyPepper` 和 TLS 配置。不要把真实密钥提交到仓库。

### 初始化与运行

```bash
./arl migrate --config config.yaml
./arl web --config config.yaml
./arl worker --config config.yaml
./arl scheduler --config config.yaml
```

本地开发可以使用单进程模式（内存队列，需将 `queue.driver` 设为 `memory`）：

```bash
ARL_QUEUE_DRIVER=memory ./arl all --config config.yaml
```

首次启动时如果未设置 `ARL_ADMIN_PASSWORD`，管理员一次性密码会打印到日志。首次登录后必须修改密码。

### Docker Compose

```bash
docker compose -f deploy/compose.yaml build
docker compose -f deploy/compose.yaml up -d
```

默认 Web 端口为 `5003`。`deploy/config.docker.yaml` 是本地示例配置，部署前必须替换其中的密钥和默认密码；如修改 Compose 的 `REDIS_PASSWORD`，需同步修改 `redis.url`。生产 TLS 请为 `server.tlsCert` / `server.tlsKey` 配置证书，或在前面放置反向代理。

默认镜像不安装 Chromium；需要截图时构建 `deploy/Dockerfile.chromium`，或在运行镜像中安装 Chromium。未安装浏览器时截图步骤会失败并被标记为可选步骤，不影响其他扫描。

### 部署预检

切换或升级前可运行 Redis 预检脚本。脚本不会打印包含密码的 Redis URL：

```bash
./scripts/redis-preflight.sh --config config.yaml
```

脚本会检查 Redis 连通性、版本、AOF/`maxmemory-policy`、写权限、`arl:jobs:*` 队列深度，并运行 `arl doctor`。迁移切换时可加 `--require-empty`；如仍保留旧 RabbitMQ，可用 `--rabbitmq-vhost arlv2host` 检查旧队列是否已排空。

## 命令

| 命令 | 说明 |
|---|---|
| `arl web` | API、SSE、OpenAPI、嵌入式前端 |
| `arl worker` | 消费 Redis Streams 任务 |
| `arl scheduler` | 监控和计划任务调度（Mongo 租约选主） |
| `arl migrate` | 创建索引、默认策略、管理员和 PoC 元数据 |
| `arl doctor` | 检查配置、Mongo、队列、资源与 DNS |
| `arl admin create-user` | 创建用户 |
| `arl admin reset-password` | 重置密码 |
| `arl poc sync` | 同步 Go 插件元数据到 MongoDB |
| `arl health` | 容器健康检查 |

## API

- OpenAPI 3.1：`/api/v1/openapi.json`
- 文档入口：`/api/v1/docs`
- 认证：`HttpOnly` Cookie `arl_access` + 写请求 `X-CSRF-Token`，或 `X-API-Key`
- 资源：`/api/v1/tasks`、`/assets/{kind}`、`/findings/{kind}`、`/scopes`、`/policies`、`/monitors`、`/schedules`、`/github/*`、`/poc/plugins`、`/exports`
- 任务实时事件：`/api/v1/tasks/{id}/events`（SSE）

列表接口统一支持 `page`、`pageSize`、`sort`、`q` 和资源专属过滤器。错误响应使用 `application/problem+json`。

## 安全模型

- Argon2id 密码哈希；15 分钟访问 JWT；7 天可轮换刷新令牌；刷新令牌重放会撤销整个会话族。
- `viewer` / `operator` / `admin` 三级 RBAC；API Key 支持 `read` / `scan` / `admin` 范围。
- 所有出站 HTTP 使用安全拨号器，校验解析后的 IP，默认阻止回环、链路本地、云元数据地址和配置的黑名单。
- 写请求默认要求 CSRF Token；安全响应头、请求 ID、审计日志和结构化日志默认开启。
- 弱口令、AK/SK 等敏感结果在日志和列表接口中脱敏；查看细节需要写权限。

## 测试

```bash
go test ./...
go test -race ./...
go vet ./...
cd web && npm run typecheck && npm run build
```

当前自动化覆盖认证密码/JWT、目标解析、端口列表、CIDR 展开、指纹 DSL、WIH 规则加载、96 个插件注册表和 Redis 协议识别。靶场集成测试需要本机 MongoDB/Redis；参见 `docs/ARCHITECTURE.md`。

## 目录

```text
cmd/arl               CLI 入口
internal/app          依赖装配
internal/domain       领域模型
internal/store        MongoDB 仓储
internal/httpapi      HTTP API、鉴权、SSE
internal/modules      业务模块
internal/platform     配置、日志、队列、安全、通知
internal/webui        嵌入式前端资源
assets/               字典、指纹、WIH 和 nuclei 模板
web/                  Vue 3 前端源码
deploy/               Docker、Compose、systemd
```

## 已知限制

- 全新部署，不迁移旧 ARL MongoDB 数据，也不兼容旧 `/api`。
- 扫描能力通过进程内 ProjectDiscovery SDK 提供：`subfinder`、`dnsx`、`naabu`、`tlsx`、`httpx`、`katana` 和 `nuclei`，版本锁定在 `go.mod`。
- OS 识别仅使用 naabu 服务元数据，无法获得 OS 类型时省略，不承诺与 nmap OS DB 字段级一致。
- 默认镜像不包含 Chromium。
- RDP、CobaltStrike、Shiro CBC/GCM 等少数暴力破解插件已注册，但需要继续补充对应协议实现；其余 HTTP/SSH/MySQL/PostgreSQL/MSSQL/MongoDB/Redis/FTP/SMTP/IMAP/POP3 路径已实现。
- `nuclei_scan` 使用官方 nuclei Go SDK，默认只加载 `assets/nuclei` 下的 HTTP 模板；interactsh、code、file 和 headless 模板默认关闭。
- 生产镜像不安装 ProjectDiscovery CLI、nmap、massdns 或其他扫描器可执行文件；Chromium 是唯一可选的非 Go 运行时。

## License

MIT。第三方依赖、字典、指纹、WIH 规则和 PoC 规则的版权与许可声明保留在仓库中。
