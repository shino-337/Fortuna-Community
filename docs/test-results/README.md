# Test Results

## Monitor chi tiết các testcase

**Danh sách đầy đủ** (script, mục đích, lệnh, tiêu chí pass): ** [docs/TESTCASE_MONITOR.md](../TESTCASE_MONITOR.md)**.

**Chạy một lệnh** để theo dõi các testcase chính (check-full-deployment, test-priority1-apis, test-runtime-signals-e2e, e2e-risk-center-verify, verify-dashboard-api, verify-agent-core-connectivity):

```bash
./scripts/monitor/monitor-testcases.sh
./scripts/monitor/monitor-testcases.sh --report docs/test-results/MONITOR-TESTCASES-$(date +%Y%m%d-%H%M%S).txt
```

## Verification flow (agent → core → database → dashboard)

1. **Connection / deployment**: `./scripts/verify/check-full-deployment.sh` — cluster, namespace, workloads, Core health, dashboard, agent.
2. **Resource / API**: `./scripts/e2e/test-priority1-apis.sh` — Core APIs (promotion-rules, runtime-signals). When Core has `AUTH_ENABLED=true`, use JWT (admin/admin123).
3. **End-to-end**: Dùng **`./scripts/e2e/run-e2e.sh`** (entry point). Ví dụ: `run-e2e.sh` (full) → E2E-FULL + risk-center-e2e + priority1; `run-e2e.sh --suite=risk-center` → 17 TCs Risk Center. Chi tiết: [docs/e2e/E2E-TestCases-And-Runner.md](../e2e/E2E-TestCases-And-Runner.md).

## Reports

- **E2E runner**: `./scripts/e2e/run-e2e.sh` (--suite=full | risk-center | full-report | …) — gọi run-e2e-full, e2e-risk-center-full, test-priority1-apis, v.v.; report theo từng script con.
- **E2E full run**: `./scripts/e2e/run-e2e-full.sh` → `E2E-FULL-<timestamp>.md` (step-by-step, API, DB, dashboard).
- **Monitor testcases**: `./scripts/monitor/monitor-testcases.sh --report <file>` → ghi log từng testcase vào file.
- **Archive**: Older reports in [docs/archive/test-results/](../archive/test-results/).
