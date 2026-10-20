# Architecture

## Runtime roles

The `arl` binary has three production roles:

1. `web`: serves the embedded SPA, `/api/v1`, OpenAPI, SSE and metrics. It is stateless apart from PostgreSQL.
2. `worker`: consumes Redis Streams and executes task steps. Multiple workers can run concurrently.
3. `scheduler`: wins a PostgreSQL lease and enqueues due monitors/schedules. Only one scheduler is active at a time.

`arl all` is a development convenience that runs all three roles with the in-memory queue.

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
  -> tasks(status=queued)
  -> task_steps
  -> outbox
  -> Redis Streams
  -> worker executes one step
  -> task_events + observations/findings
  -> enqueue next step
  -> finalize(status=succeeded|partial|failed)
```

Optional steps (screenshots, WIH, file leak, PoC, nuclei) fail to `partial`
without aborting the task. Critical steps (target validation, DNS, HTTP,
statistics) fail the task.

## Assets

- `assets`: canonical `(kind, key)` rows with JSONB scan data.
- `asset_observations`: per-task snapshots for historical task views and reports.
- `scope_asset_links`: many-to-many membership with first/last seen and tags.

Canonical assets keep global search fast; observations preserve the exact
task result set and record the producing `engine`/`engineVersion`.

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
ICP_Query sidecar (`icp.icpQuery.baseURL`). Provider credentials come from
environment variables or read-only files and are never persisted.

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
- `pdtools` converts that policy into ProjectDiscovery `networkpolicy` options.
- nuclei disables code/file/headless protocols, local file access, interactsh
  and automatic upload/update checks by default.
- Scanner concurrency and response sizes are bounded by `scanners` config.
- Screenshot/export paths are validated against path traversal.
- Secrets are never logged; sensitive findings are masked at the UI boundary.
