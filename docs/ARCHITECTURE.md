# Architecture

## Runtime topology

ARL runs as one native `arl all` process backed by PostgreSQL and Redis. It
serves the embedded SPA and API, consumes Redis Streams, runs the PostgreSQL
scheduler lease and projects the graph outbox. If the worker loop or HTTP server
fails, the process exits non-zero so systemd can restart it. Docker is optional
and carries only PostgreSQL and Redis. Neo4j, ICP_Query and GeoIP are external
integrations, not part of the default Compose stack.

## Module boundaries

| Package                       | Responsibility                                                                                                  |
| ----------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `internal/modules/discovery`  | Target parsing and ProjectDiscovery SDK adapters for DNS, passive sources, ports, TLS and HTTP                  |
| `internal/modules/webintel`   | Fingerprints, WIH, file leaks, katana crawling, screenshots and nuclei SDK results                              |
| `internal/modules/poc`        | Go plugin registry, HTTP/TCP/sniffer/brute plugins                                                              |
| `internal/modules/tasking`    | Task state machine, steps, outbox, worker handlers                                                              |
| `internal/modules/monitoring` | Monitors, schedules, leader lease                                                                               |
| `internal/modules/github`     | GitHub search and monitoring                                                                                    |
| `internal/modules/reporting`  | CSV/XLSX reports                                                                                                |
| `internal/store`              | PostgreSQL/GORM repositories, AutoMigrate and indexes                                                                                |
| `internal/platform`           | Config, logging, Redis Streams, auth primitives, network safety, notifications and ProjectDiscovery SDK helpers |

Modules share the domain model but do not reach into another module's private
helpers. Repository access is centralized in `internal/store`.

## Console

The embedded console lives in `web/` and is built with React, TypeScript, Vite,
Tailwind CSS, Radix/shadcn-style primitives, TanStack Query/Table and ECharts.
`make build` writes the Vite output to `internal/webui/dist` before compiling Go,
and `go:embed` places those files in the `arl` binary. Node.js is build-time
only; the runtime serves `/`, hashed assets and SPA fallback directly from Go.
The checked-in OpenAPI document is the source for generated TypeScript types.
Shareable list filters and map/graph selection live in the URL; server state is
cached by TanStack Query. The console is organization-centric and exposes
overview, organizations, assets, findings and tasks as the primary path while
automation, intelligence and administration remain secondary navigation groups.

## Scanner pipeline

```text
subfinder -> dnsx -> naabu -> tlsx
                    |
                    v
                  httpx
                    |
        +-----------+-----------+
        |           |           |
   fingerprints    WIH        katana
        |                       |
        +-----------+-----------+
                    |
                  nuclei
```

All scanner engines run in-process. The worker passes the task context through
the SDK adapters; no scanner executable is launched. `pdtools` maps the ARL
network policy onto ProjectDiscovery's network policy and redirects SDK logs
to the application's structured logger.

## Task lifecycle

```text
POST /tasks
  -> one PostgreSQL transaction:
       tasks(status=queued) + task_steps + outbox
  -> Redis Streams
  -> worker atomically claims one step with a renewable lease
  -> task_events + observations/findings
  -> enqueue next step
  -> finalize(status=succeeded|partial|failed)
```

The outbox relay is the only publisher for task steps. A duplicate delivery
cannot execute a step while its lease is active; expired leases are reclaimable
after a crash. Critical steps return to `queued` for broker retries and become
`failed` only after the final delivery attempt.

Optional steps (screenshots, WIH, file leak, PoC, nuclei) fail to `partial`
without aborting the task. Critical steps (target validation, DNS, HTTP,
statistics) fail the task.

## Assets

- `assets`: canonical `(kind, key)` rows with JSONB scan data.
- `asset_observations`: per-task snapshots for historical task views and reports.
- `scope_asset_links`: many-to-many membership with first/last seen and tags.
- Asset management metadata (`origin`, display name/URL, notes and tags) is separate
  from scan-owned `data`; user edits never rewrite scanner facts.

Canonical assets keep global search fast; observations preserve the exact
task result set and record the producing `engine`/`engineVersion`.

## Lifecycle and CRUD

Organizations, organization domains, assets, scopes, policies, monitors,
schedules, fingerprints, GitHub monitors and users use recoverable soft deletion.
Normal reads, authentication, schedulers and EASM/graph projections ignore rows
with `deletedAt`; administrators restore records from `/api/v1/recycle-bin`.

All delete and restore operations are audited. Policy deletion is blocked while
active monitors or unfinished schedules reference it, scope deletion is blocked
while active monitors reference it, and organization deletion is blocked while a
queued or running task belongs to it. Users cannot delete themselves or the last
active administrator, and deleting a user revokes all sessions.

Assets deleted manually stay hidden even if observations continue to arrive,
except that a later scan re-observing the same `(kind, key)` atomically restores
the canonical asset and records `restoreReason=scan_rediscovery`. Manual metadata
is preserved across that recovery.

## EASM

- `organizations`: canonical identity, aliases, region and external IDs.
- `organization_domains`: confirmed/candidate ICP ownership with source and confidence.
- `icp_evidence`: raw provider evidence; never contains cookies.
- `ip_locations`: offline GeoIP cache.
- `graph_outbox` + `icp_jobs`: graph projection and async ICP refresh.

The EASM surface is organization-first. `/easm` keeps the world map and graph
overview; `/easm/organizations` handles organization and domain review,
`/easm/organizations/:id` exposes assets, current findings, tasks and reports,
and `/easm/assets`, `/easm/findings`, `/easm/tasks` provide cross-organization
views. PostgreSQL remains the source of truth; Neo4j is rebuilt from the outbox
and `arl graph rebuild` after storage migrations that change projection identifiers.

ICP attribution uses the Chinaz domain page plus its signed permit endpoint for
domain → organization. Organization → filed domains uses the external
ICP_Query sidecar (`icp.icpQuery.baseURL`). Optional EASM services are reached
through loopback ports when run with the infrastructure Compose overlay. Provider
credentials come from environment variables or read-only files and are never persisted.

## Queue topology

| Queue                 | Work                                                      |
| --------------------- | --------------------------------------------------------- |
| `q.discovery.dns`     | DNS and Fofa discovery                                    |
| `q.discovery.network` | Port/TLS scanning                                         |
| `q.web.fetch`         | HTTP probing                                              |
| `q.web.scan`          | Fingerprints, WIH, spider, file leak, screenshots, nuclei |
| `q.poc.http`          | HTTP/brute checks                                         |
| `q.poc.service`       | Protocol sniffing                                         |
| `q.monitor`           | Monitoring work                                           |
| `q.github`            | GitHub jobs                                               |
| `q.icp`               | ICP organization refresh                                  |
| `q.maintenance`       | Statistics/export/maintenance                             |

Each queue is a Redis Stream named `arl:jobs:<queue>` with consumer group
`arl-workers`. Consumers use manual acknowledgement and remove acknowledged
entries. Failed jobs are delayed in `arl:jobs:<queue>:retry` and published again
after 10 seconds; after three retries they move to
`arl:jobs:<queue>:dlq`. Unacknowledged entries are reclaimed after two minutes
of inactivity, while active handlers refresh their lease every 30 seconds.

The outbox collection is the durable handoff; the relay republishes pending
messages. Redis must run with AOF persistence and `maxmemory-policy noeviction`
so queued jobs are not evicted.

## Security boundaries

- `netsafe` remains the source of truth for blocked CIDRs, private ranges,
  loopback and URL redirects.
- When a task declares a scope, its targets must belong to that scope allowlist;
  `blackScope` always takes precedence. `scan.forbiddenDomains` applies to every
  domain and URL task.
- `pdtools` converts that policy into ProjectDiscovery `networkpolicy` options.
- nuclei disables code/file/headless protocols, local file access, interactsh
  and automatic upload/update checks by default.
- Scanner concurrency and response sizes are bounded by `scanners` config.
- Screenshot/export paths are validated against path traversal.
- Secrets are never logged; sensitive findings are masked at the UI boundary.
