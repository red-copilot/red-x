# Playwright E2E

The suite exercises the embedded Vue console against a running ARL deployment. It signs in, navigates console pages, validates login/logout, and checks client-side task validation. It does not create tasks, assets, scopes, policies, monitors, schedules, or fingerprints; only authentication sessions and audit events are written.

## Prerequisites

- Node.js 22+
- A reachable ARL instance (`E2E_BASE_URL`, default `http://127.0.0.1:5003`)
- A dedicated E2E account with at least `viewer` role

## Run

```bash
cd web
E2E_USERNAME=arl_e2e E2E_PASSWORD='...' npm run test:e2e
```

Useful commands:

```bash
npm run test:e2e -- --headed
npm run test:e2e:report
```

If your instance is not on the default address:

```bash
E2E_BASE_URL=https://arl.example.test E2E_USERNAME=... E2E_PASSWORD=... npm run test:e2e
```

Failures retain traces, screenshots, and video under `web/test-results`; the HTML report is written to `web/playwright-report`.
