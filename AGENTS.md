# Repository Guidelines

## Project Structure & Module Organization

ARL is a Go modular monolith with an embedded React console.

- `cmd/arl/`: single-node server plus operational CLI commands (`all`, `migrate`, `doctor`).
- `internal/app/`, `internal/httpapi/`, `internal/store/`: wiring, REST/SSE/OpenAPI, and PostgreSQL/GORM repositories.
- `internal/modules/`: identity, discovery, webintel, EASM, PoC, tasking, monitoring, GitHub, and reporting.
- `internal/platform/`: config, logging, queues, safety, notifications, and ProjectDiscovery helpers.
- `web/src/`: Vue/TypeScript source; `web/e2e/`: Playwright tests; output goes to `internal/webui/dist`.
- `assets/`: dictionaries, fingerprints, WIH rules, and nuclei templates. `deploy/` and `docs/` contain deployment and design docs.

## Build, Test, and Development Commands

Requires Go 1.27+ and Node.js 22+.

```bash
make build     # frontend and Go binary
make test      # Go tests and frontend typecheck
make race      # Go race tests
make lint      # gofmt, vet, vue-tsc
make verify    # module verification, vet, tests, typecheck
```

Run `./arl migrate --config config.yaml` before starting; production uses Redis with `./arl all`, while `queue.driver: memory` is only for throwaway local runs. From `web/`: `npm test` (Vitest), `npm run test:e2e` (Playwright).

## Coding Style & Naming Conventions

- Run `gofmt`; use lowercase packages, `PascalCase` exports, `camelCase` locals, and `%w` for errors.
- Put `context.Context` first and propagate cancellation through scanners and workers.
- Use camelCase JSON/BSON fields such as `taskId` and `scopeId`.
- Use in-process ProjectDiscovery SDKs pinned in `go.mod`; no CLI wrappers or shell-outs.
- Vue uses two spaces, `<script setup lang="ts">`, Composition API, and `PascalCase` components.
- Keep `pdtools` as the sole mapping of `netsafe` policy and logs to ProjectDiscovery config.
- Do not reintroduce Python, Flask, Celery, nmap, massdns, or PhantomJS.

## Testing Guidelines

Use Go's `testing` package with table-driven tests and `httptest`. Name tests `TestXxx`, colocate them with the package, and cover success, failure, and authorization. Integration tests use `ARL_TEST_POSTGRES_DSN`, `ARL_TEST_MONGO_AUTH_ADDR`/`ARL_TEST_MONGO_NOAUTH_ADDR`, `ARL_TEST_NEO4J_*`, or `ARL_TEST_GEOIP_*`. Add focused tests; no fixed coverage threshold. Frontend changes must pass `npm run typecheck` and `npm run build`.

## Commit & Pull Request Guidelines

Use Conventional Commits, for example `feat: add EASM wallboard` or `fix: preserve ICP ownership`. After verification, stage only task-specific files and commit automatically; exclude unrelated changes, generated artifacts, and secrets. Do not push unless asked.

PRs should cover the problem, change, verification commands, and linked issues; call out API/schema/config/security changes. Include UI screenshots and update `docs/PARITY.md` or `docs/SCANNER-DIFFERENCES.md` when behavior changes.

## Security & Configuration

Never commit secrets. Treat `config.example.yaml` as a template and prefer `ARL_*` environment overrides. Preserve blacklist/SSRF checks, CSRF protection, RBAC, and sensitive-field masking.
