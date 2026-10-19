# EASM Operations

## Pages

- `/easm`: map and graph overview.
- `/easm/organizations`: organization list, CSV import and manual creation.
- `/easm/organizations/:id`: domain review, assets, current findings, tasks and reports.
- `/easm/assets`, `/easm/findings`, `/easm/tasks`: cross-organization EASM views.

## Ownership review

Manual and CSV domain imports create `candidate` rows. They do not appear in the map, graph or organization statistics until an operator confirms them. Rejecting or moving a confirmed domain keeps the original row and its ICP evidence. Reassignment is transactional and writes graph outbox events for both the old and new rows.

## Local real-integration stack

```bash
cp deploy/.env.easm.example deploy/.env.easm
mkdir -p deploy/secrets deploy/geoip
# Place an authorized Chinaz cookie at deploy/secrets/chinaz.cookie
# Place MaxMind-compatible City and ASN MMDB files in deploy/geoip/ (GeoLite2 or DB-IP Lite work)
docker compose --env-file deploy/.env.easm -f deploy/compose.yaml -f deploy/compose.easm.yaml up -d --build
```

The overlay pins Neo4j and ICP_Query images. ICP_Query is an external Python sidecar; ARL itself does not depend on Python. Do not expose its port outside the local host.

## Live smoke

```bash
ARL_BASE_URL=http://127.0.0.1:5003 \
E2E_USERNAME=operator E2E_PASSWORD='...' \
EASM_LIVE_DOMAIN='authorized.example.edu.cn' \
EASM_LIVE_SCAN=1 \
./scripts/easm-live-smoke.sh
```

The smoke script never prints passwords or provider cookies. Scanning is opt-in through `EASM_LIVE_SCAN=1`; use only an explicitly authorized domain and bounded scan options.

## Failure modes

- ICP cookie expired: lookup returns `icp_cookie_expired`; confirmed domains, assets and reports remain available.
- ICP_Query unavailable: refresh jobs fail while existing organization data remains visible.
- GeoIP missing: unlocated assets are counted, never plotted at fake coordinates.
- Neo4j unavailable: graph is disabled; map, lists, assets and findings remain available.
