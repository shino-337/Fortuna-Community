# Full Regression After Clean/Build/Deploy

- Timestamp: 2026-02-23T13:43:43Z
- Scope: `test-*.sh + e2e-* + verify-*`
- Raw logs: `docs/test-results/full-regression-after-deploy-20260223-134343/`
- Deployment flow executed: clean -> build -> deploy -> rollout status verified

## Deployment/Rollout Status

- `deployment/fortuna-core`: rolled out successfully
- `deployment/fortuna-dashboard`: rolled out successfully
- `daemonset/fortuna-agent`: rolled out successfully

## Regression Results

| Script | Final Exit | Status | Notes |
|---|---:|---|---|
| `e2e-dashboard-data.sh` | 0 | PASS | - |
| `e2e-risk-center-verify.sh` | 0 | PASS | - |
| `e2e-sbom-verify.sh` | 0 | PASS | - |
| `test-dashboard-consistency-e2e.sh` | 0 | PASS | - |
| `test-pce-api.sh` | 0 | PASS | - |
| `test-pce-e2e.sh` | 0 | PASS | Run with `PCE_SYNC_TIMEOUT_SECONDS=420` |
| `test-pod-critical-risk-cluster-id.sh` | 0 | PASS | - |
| `test-pod-recreate-storage.sh` | 0 | PASS | - |
| `test-pod-risk-flow.sh` | 0 | PASS | - |
| `test-pod-sync-flow.sh` | 0 | PASS | - |
| `test-priority1-apis.sh` | 0 | PASS | - |
| `test-promotion-flow.sh` | 0 | PASS | - |
| `test-runtime-probe-e2e.sh` | 0 | PASS | - |
| `test-runtime-signals-e2e.sh` | 0 | PASS | - |
| `test-sbom-pod-flow.sh` | 0 | PASS | initial run failed due local core endpoint auth unreachable; rerun passed |
| `verify-agent-availability.sh` | 0 | PASS | - |
| `verify-dashboard-api.sh` | 0 | PASS | initial run failed due local core endpoint unreachable; rerun passed |
| `verify-dashboard-apis.sh` | 0 | PASS | - |
| `verify-dashboard-issues.sh` | 0 | PASS | - |
| `verify-database-schema.sh` | 0 | PASS | - |
| `verify-pod-count.sh` | 0 | PASS | - |
| `verify-pod-data.sh` | 0 | PASS | - |
| `verify-test-data.sh` | 0 | PASS | - |

## Summary

- Total scripts: **23**
- PASS: **23**
- FAIL: **0**

## Notes

- Two scripts that depend on `http://localhost:8080` were rerun after confirming local Core endpoint was reachable.
- Core/DB stale-pod hardening remains in source and DB index is present (`idx_pods_cluster_deleted_updated`).
