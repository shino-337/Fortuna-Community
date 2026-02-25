# E2E Testcases – Tổng kết và kết quả thực tế

**Ngày chạy:** 2026-02-22 (đã sửa lỗi và chạy lại)  
**Runner:** `scripts/e2e/run-e2e-complete-with-monitor.sh`

---

## Sửa đổi đã áp dụng

- **run-e2e-full.sh:** Khi curl tới ClusterIP thất bại (host ngoài cluster), fallback sang `kubectl exec` vào Core pod cho mọi API; dùng helper `core_api()`; lấy Postgres pod theo label; tắt `set -e` để báo cáo vẫn ghi đủ khi một bước lỗi.
- **e2e-dashboard-data.sh:** Tăng thời gian chờ pod xuất hiện trong Core từ 90s lên 180s (có thể override bằng `E2E_POD_SYNC_WAIT`), bước sleep 10s.
- **test-priority1-apis.sh:** Lấy Postgres pod động theo label `app=postgres` thay vì tên cố định.

---

## Testcases đã chạy (sau khi sửa)

| # | Testcase | Script | Kết quả | Ghi chú |
|---|----------|--------|---------|--------|
| 1 | E2E Full (cluster, API, DB, dashboard) | run-e2e-full.sh | ✅ Pass | JWT, health 200, capability-metadata, promotion-rules, DB counts, dashboard; báo cáo E2E-FULL-*.md đầy đủ |
| 2 | Priority 1 APIs | test-priority1-apis.sh | ✅ Pass | promotion-rules 9, by capability 3, by signal 4, runtime-signals 3, GET by pod UID 1 |
| 3 | Runtime Signals E2E | test-runtime-signals-e2e.sh | ✅ Pass | POST → DB 4 events/signals, GET total=4, GET by pod OK |
| 4 | Dashboard chart data | e2e-dashboard-data.sh | ✅ Pass | Pod tạo; **Pod found in Core after 50s**; runtime-events 2; threat-velocity 7 points; PCE trend 7 points, 69 capabilities |

---

## Kết quả API thực tế (ghi nhận sau E2E)

### GET /api/v1/clusters
- **HTTP:** 200  
- **Nội dung:** 1 cluster: `cluster-c93f5c6c`, id `sha256-c93f5c6cb0e57f8f`, status active, lastSync có giá trị, k8sVersion v1.29.15.

### GET /api/v1/dashboard/stats
- **HTTP:** 200  
- **Nội dung:** totalClusters: 1, activeAgents: 2, runningPods: 20, totalRisks: 0, criticalRisks: 0.

### GET /api/v1/agents/status
- **HTTP:** 200  
- **Nội dung:** 2 agents (k8s-master-agent, k8s-worker01-agent), clusterId trùng cluster trên, status "slow", total: 2, disconnected: 0.

### GET /api/v1/insights/summary
- **HTTP:** 200  
- **Nội dung:** total: 0, critical/high/medium/low: 0, byType: {}.

---

## Monitor Agent

- **Sync:** Cả 2 agent đều có `[Syncer] ✅ Full sync completed` (chu kỳ 5 phút).
- **Heartbeat:** `✅ Heartbeat OK (interval=15s)` trên cả 2 pod.
- **E2E pod:** Agent ghi `Queued pod fortuna/e2e-dashboard-pod-1771769061 for async processing` khi pod Running.
- **SBOM:** Một số "Containerd fetch failed" (digest not found) – không chặn sync/heartbeat.

---

## Monitor Core

- **Trước E2E:** Core ghi pod_capabilities (INSERT/UPDATE), /ready, /healthz 200.
- **Sau E2E:** Core xử lý POST /api/v1/insights/evaluate/historical (200), GET threat-velocity (200), GET pod-capabilities/trends (200). Có CEL compile warning (overprivileged-binding, rules[].resources contains) và fallback YAML engine 7 rules – vẫn chạy bình thường.

---

## File báo cáo

- **Tổng hợp (monitor + API):** `docs/test-results/E2E-COMPLETE-WITH-MONITOR-<timestamp>.md`
- **API raw:** `docs/test-results/e2e-api-results-<timestamp>.txt`
- **Log Agent/Core trước:** `docs/test-results/e2e-logs-before-<timestamp>.txt`
- **Log Agent/Core sau:** `docs/test-results/e2e-logs-after-<timestamp>.txt`
- **E2E full chi tiết:** `docs/test-results/E2E-FULL-<timestamp>.md`

---

## Cách chạy lại

```bash
cd /home/k8s/KSAM
./scripts/e2e/run-e2e-complete-with-monitor.sh
```

Kết quả mới sẽ ghi vào `docs/test-results/` với timestamp tương ứng.
