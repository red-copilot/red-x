# EASM Operations

## Pages

- `/easm`: map and graph overview.
- `/easm/organizations`: organization list, CSV import, manual creation and domain attribution lookup.
- `/easm/organizations/:id`: domain review, assets, current findings, tasks and reports.
- `/easm/assets`, `/easm/findings`, `/easm/tasks`: cross-organization EASM views.

## Ownership review

Domain attribution lookup, manual domain imports and CSV imports create `candidate` rows. The lookup entry point is available from the organization list and opens the matched organization for evidence review. Candidates do not appear in the map, graph or organization statistics until an operator confirms them. Rejecting or moving a confirmed domain keeps the original row and its ICP evidence. Reassignment is transactional and writes graph outbox events for both the old and new rows.

## External integrations

Neo4j, ICP_Query and GeoIP are optional external services. Start them outside
the default Docker Compose stack, then load their endpoints into the native ARL
process:

```bash
cp deploy/.env.easm.example deploy/.env.easm
set -a
source deploy/.env.easm
set +a
mkdir -p deploy/secrets deploy/geoip
# Place an authorized Chinaz cookie at deploy/secrets/chinaz.cookie with mode 0600.
# Optionally override ARL_ICP_CHINAZ_COOKIE_FILE and the GeoIP paths for the host.
```

ARL itself does not depend on Python. ICP_Query remains an external query
service and must not be exposed outside the trusted host or management network.

## Live smoke

```bash
ARL_BASE_URL=http://127.0.0.1:5003 \
E2E_USERNAME=operator E2E_PASSWORD='...' \
EASM_LIVE_DOMAIN='authorized.example.edu.cn' \
EASM_LIVE_SCAN=1 \
./scripts/easm-live-smoke.sh
```

The smoke script uses the direct `/api/v1` contract, never prints passwords or provider cookies, and validates health, domain attribution, ICP refresh, candidate confirmation and export. Scanning is opt-in through `EASM_LIVE_SCAN=1`; use only an explicitly authorized domain and bounded scan options.

## Failure modes

- ICP cookie expired: lookup returns `icp_cookie_expired`; confirmed domains, assets and reports remain available.
- ICP_Query unavailable: refresh jobs fail while existing organization data remains visible.
- GeoIP missing: unlocated assets are counted, never plotted at fake coordinates.
- Neo4j unavailable: graph is disabled; map, lists, assets and findings remain available.
