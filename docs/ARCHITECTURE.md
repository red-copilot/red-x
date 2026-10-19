# Architecture

## Runtime roles

The `arl` binary has three production roles:

1. `web`: serves the embedded SPA, `/api/v1`, OpenAPI, SSE and metrics. It is stateless apart from MongoDB.
2. `worker`: consumes RabbitMQ queues and executes task steps. Multiple workers can run concurrently.
3. `scheduler`: wins a MongoDB lease and enqueues due monitors/schedules. Only one scheduler is active at a time.

`arl all` is a development convenience that runs all three roles with the in-memory queue.

## Module boundaries

| Package | Responsibility |
|---|---|
| `internal/modules/discovery` | Target parsing and ProjectDiscovery SDK adapters for DNS, passive sources, ports, TLS and HTTP |
| `internal/modules/webintel` | Fingerprints, WIH, file leaks, katana crawling, screenshots and nuclei SDK results |
| `internal/modules/poc` | Go plugin registry, HTTP/TCP/sniffer/brute plugins |
| `internal/modules/tasking` | Task state machine, steps, outbox, worker handlers |
| `internal/modules/monitoring` | Monitors, schedules, leader lease |
| `internal/modules/github` | GitHub search and monitoring |
| `internal/modules/reporting` | CSV/XLSX reports |
| `internal/store` | MongoDB repositories and indexes |
| `internal/platform` | Config, logging, RabbitMQ, auth primitives, network safety, notifications and ProjectDiscovery SDK helpers |

Modules share the domain model but do not reach into another module's private
helpers. Repository access is centralized in `internal/store`.

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
  -> RabbitMQ queue
  -> worker executes one step
  -> task_events + observations/findings
  -> enqueue next step
  -> finalize(status=succeeded|partial|failed)
```

Optional steps (screenshots, WIH, file leak, PoC, nuclei) fail to `partial`
without aborting the task. Critical steps (target validation, DNS, HTTP,
statistics) fail the task.

## Assets

- `assets`: canonical `(kind, key)` documents.
- `asset_observations`: per-task snapshots for historical task views and reports.
- `scope_asset_links`: many-to-many membership with first/last seen and tags.

Canonical assets keep global search fast; observations preserve the exact
task result set and record the producing `engine`/`engineVersion`.

## Queue topology

| Queue | Work |
|---|---|
| `q.discovery.dns` | DNS and Fofa discovery |
| `q.discovery.network` | Port/TLS scanning |
| `q.web.fetch` | HTTP probing |
| `q.web.scan` | Fingerprints, WIH, spider, file leak, screenshots, nuclei |
| `q.poc.http` | HTTP/brute checks |
| `q.poc.service` | Protocol sniffing |
| `q.monitor` | Monitoring work |
| `q.github` | GitHub jobs |
| `q.maintenance` | Statistics/export/maintenance |

Messages are persistent and consumers use manual ack. The outbox collection is
the durable handoff; the relay republishes pending messages. Retry queues and
`.dlq` queues are declared for every main queue.

## Security boundaries

- `netsafe` remains the source of truth for blocked CIDRs, private ranges,
  loopback and URL redirects.
- `pdtools` converts that policy into ProjectDiscovery `networkpolicy` options.
- nuclei disables code/file/headless protocols, local file access, interactsh
  and automatic upload/update checks by default.
- Scanner concurrency and response sizes are bounded by `scanners` config.
- Screenshot/export paths are validated against path traversal.
- Secrets are never logged; sensitive findings are masked at the UI boundary.
