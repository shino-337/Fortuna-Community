# Test Results

## Verification flow (agent → core → database → dashboard)

1. **Connection / deployment**: `./scripts/verify/check-full-deployment.sh` — cluster, namespace, workloads, Core health, dashboard, agent.
2. **Resource / API**: `./scripts/e2e/test-priority1-apis.sh` — Core APIs (promotion-rules, runtime-signals). When Core has `AUTH_ENABLED=true`, calls without `Authorization` header return 401; use dashboard or token for authenticated checks.
3. **End-to-end**: `./scripts/e2e/run-e2e-full.sh` — writes `E2E-FULL-<timestamp>.md` with cluster/pods, Core health, API responses (may show 401 if auth on), DB row counts, dashboard.

## Reports

- **E2E full run**: `./scripts/e2e/run-e2e-full.sh` → `docs/test-results/E2E-FULL-<timestamp>.md` (step-by-step, API responses, DB state, dashboard).
- **Archive**: Older dated reports (E2E-*, CLEAN-*, DEBUG-*, etc.) are in [docs/archive/test-results/](../archive/test-results/).
