# Playwright E2E

The suite exercises the embedded React console against a running ARL deployment. It signs in (including forced first-login password change), navigates the attack-surface information architecture, validates login/logout, task creation guards, removed-route 404 behavior, global search, responsive layouts, and accessibility snapshots. Non-visual tests do not create tasks, assets, scopes, policies, monitors, schedules, or fingerprints; only authentication sessions and audit events are written.

## Prerequisites

- Node.js 22+
- A reachable ARL instance (`E2E_BASE_URL`, default `http://127.0.0.1:5003`)
- A dedicated E2E account. The route and admin suites require `admin`; the default `E2E_USERNAME=admin` is suitable for the isolated test database.

## Run

```bash
cd web
E2E_USERNAME=arl_e2e E2E_PASSWORD='...' npm run test:e2e
```

Useful commands:

```bash
npm run test:e2e -- --headed
npm run test:visual
npm run test:e2e:report
```

If your instance is not on the default address:

```bash
E2E_BASE_URL=https://arl.example.test E2E_USERNAME=... E2E_PASSWORD=... npm run test:e2e
```

Visual baselines live in `web/e2e/visual.spec.ts-snapshots`. Failures retain traces, screenshots, and video under `web/test-results`; the HTML report is written to `web/playwright-report`.
