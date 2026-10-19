# Scanner differences

ARL uses ProjectDiscovery Go SDKs in-process. The production image does not
contain `subfinder`, `dnsx`, `naabu`, `tlsx`, `httpx`, `katana`, `nuclei`,
`nmap`, `massdns`, or `wih` executables.

## Pinned engines

| Function | Implementation |
|---|---|
| Passive subdomains | `subfinder/v2` v2.17.0, sources `crtsh`, `rapiddns`, `alienvault` by default |
| DNS | `dnsx` v1.3.1 (A/AAAA/CNAME, wildcard filtering, CDN checks) |
| Ports and services | `naabu/v2` v2.6.1 (connect by default; optional SYN with `NET_RAW`) |
| TLS | `tlsx` v1.4.1 |
| HTTP probing | `httpx` v1.12.0 |
| Web crawling | `katana` v1.8.0 standard crawler |
| Templated checks | `nuclei/v3` v3.11.1, HTTP templates only |

## Known behavior differences

- Upstream SDK rate limiting, retries, service fingerprints, redirect handling
  and result timing differ from the previous native implementations.
- Subfinder runs first for ICANN-managed registrable domains and multi-label
  public suffixes, including inputs such as `baidu.com`, `guc.edu.cn`, `edu.cn`
  and `com.cn`. A normal subdomain such as `www.baidu.com`, a private suffix
  such as `foo.github.io`, or an unmanaged name such as `intranet.corp` is
  treated as a host target and does not trigger parent-domain enumeration.
  Inputs are not rewritten to the registrable domain.
- Passive candidates, dictionary-brute words, historical DNS results and
  AltDNS candidates are capped by `scan.maxHosts`; truncation records a task
  warning. Retained passive candidates are resolved with bounded concurrency
  using `scan.concurrency`. Subfinder applies the same result limit while
  collecting source output.
- OS information is reported only when naabu service metadata provides an OS
  type. It is not nmap OS-database parity and is omitted when unavailable.
- `nuclei` runs only the HTTP templates under `assets/nuclei`; code, file,
  headless, local-file access and interactsh/OOB callbacks are disabled by
  default. A template directory with no valid templates makes the optional
  nuclei step fail to `partial`.
- `katana` uses its standard crawler and returns in-scope HTTP/HTTPS URLs.
  Chromium is not required for crawling.
- Chromium remains optional for screenshots. It is the only non-Go runtime
  used by the application.
- SDK update checks, PDCP upload and automatic external interaction services
  are disabled. All outbound integrations must be configured explicitly.

## Rebuilding parity

The scanner versions and adapters are covered by unit and local integration
tests. Run `arl doctor` to print SDK versions, the subfinder provider config
status and the number of nuclei templates. Use `docs/PARITY.md` for the
feature-to-module mapping.
