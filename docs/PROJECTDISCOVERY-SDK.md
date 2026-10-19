# ProjectDiscovery SDK integration

ARL embeds the following ProjectDiscovery Go SDKs. Versions are pinned in
`go.mod`; the binary does not invoke their command-line tools.

| SDK | Version | Entry point |
|---|---|---|
| subfinder | v2.17.0 | `pkg/runner` |
| dnsx | v1.3.1 | `libs/dnsx` |
| naabu | v2.6.1 | `pkg/runner` |
| tlsx | v1.4.1 | `pkg/tlsx` |
| httpx | v1.12.0 | `runner` |
| katana | v1.8.0 | `pkg/engine/standard` |
| nuclei | v3.11.1 | `lib` |

## Runtime behavior

- `subfinder` restricts default sources to `crtsh`, `rapiddns` and
  `alienvault`. Provider credentials live in
  `data/subfinder-provider.yaml` and are never committed.
- `dnsx` uses `scan.dnsServers`, filters blocked addresses before results reach
  the asset store and enables CDN classification.
- `naabu` defaults to connect scanning. SYN scanning requires `NET_RAW`; nmap
  integration is never invoked.
- `httpx` keeps response bodies only up to `scanners.httpx.maxBodyRead` and
  emits results through a callback, not stdout.
- `katana` runs the standard crawler with same-site scope, depth/page limits
  and a response-size cap.
- `nuclei` uses one thread-safe engine per worker. It loads HTTP templates from
  `scanners.nuclei.templatesDir`, disables code/file/headless protocols,
  local-file access, interactsh, PDCP upload and automatic template updates.

## Security

`internal/platform/pdtools` converts the ARL `netsafe` policy into a
ProjectDiscovery network policy. The same blocked ranges are passed to nuclei
as excluded targets. Redirects and ancillary HTTP clients are revalidated by
`netsafe`; scanners must not bypass it with a raw external client.

SaaS control-plane calls such as FOFA use `pdtools.NewVerifiedHTTPClient`,
which keeps the `netsafe` dial/redirect checks but verifies TLS certificates.
Scanner HTTP clients intentionally allow self-signed targets and must not be
reused for API calls that carry credentials.

## Verification

```bash
CGO_ENABLED=0 go test ./...
go test -race ./...
go vet ./...
go mod verify
```

Local integration tests cover DNS, connect port scanning, self-signed TLS,
HTTPX body/title extraction, katana scope and nuclei template matching.
