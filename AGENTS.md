# Repository Guidelines

## Project Structure & Module Organization

ARL is a Go modular monolith with an embedded Vue 3 console.

- `cmd/arl/`: CLI (`web`, `worker`, `scheduler`, `migrate`, `doctor`).
- `internal/app/`, `httpapi/`, `store/`: wiring, API/SSE, MongoDB repositories.
- `internal/modules/`: identity, discovery, webintel, PoC, tasking, monitoring, GitHub, reporting.
- `internal/platform/`: config, logging, queues, network safety, notifications.
- `web/src/`: Vue/TypeScript; output goes to `internal/webui/dist`.
- `assets/`: dictionaries, fingerprints, WIH rules, and standard nuclei templates.
- `deploy/`, `docs/`, `legacy/`: deployment, design/parity docs, read-only historical Python.

## Build, Test, and Development Commands

Requires Go 1.27+ and Node.js 22+.

```bash
make build        # build frontend and Go binary
make test         # go test ./... + web typecheck
make race         # go test -race ./...
make lint         # gofmt, go vet, vue-tsc
make verify       # go mod verify, vet, tests, frontend typecheck
make frontend     # install and build Vue
make compose-build
```

Run `./arl migrate --config config.yaml` before web/worker/scheduler. For local single-process work, set `queue.driver: memory` and run `./arl all --config config.yaml`.

## Coding Style & Naming Conventions

- Run `gofmt`; package names lowercase. Use `PascalCase` for exports, `camelCase` locally, `%w` for errors.
- Put `context.Context` first and propagate cancellation through scanners/workers.
- Use camelCase JSON/BSON fields (`taskId`, `scopeId`) and existing module boundaries.
- Scanner engines are in-process ProjectDiscovery SDKs pinned in `go.mod`; do not add CLI wrappers or shell out to scanner binaries.
- Vue: two spaces, `<script setup lang="ts">`, Composition API, `PascalCase` components.
- Do not reintroduce Python, Flask, Celery, nmap, massdns or PhantomJS runtime dependencies.
- Keep `platform/pdtools` as the single place that maps `netsafe` policy and structured logs into ProjectDiscovery SDK configuration.

## Testing Guidelines

Use Go `testing` with table-driven tests and `httptest`. Name tests `TestXxx`, colocate with the package, and cover success, failure, and authorization. MongoDB integration tests require:

```bash
ARL_TEST_MONGO_URI=mongodb://127.0.0.1:27017 go test ./internal/store
```

No fixed coverage threshold; add focused tests for each change. Frontend changes must pass `npm run typecheck` and `npm run build`.

## Commit & Pull Request Guidelines

The branch has no commits yet. Use Conventional Commits (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`). PRs should explain the problem and change, list verification commands, link issues, and call out API/schema/config/security changes. Include UI screenshots and update `docs/PARITY.md` or `docs/SCANNER-DIFFERENCES.md` when behavior changes.

## Security & Configuration

Never commit secrets. `config.example.yaml` is a template; use `ARL_*` env overrides. Preserve blacklist/SSRF checks, CSRF and RBAC. `legacy/` is reference only.
