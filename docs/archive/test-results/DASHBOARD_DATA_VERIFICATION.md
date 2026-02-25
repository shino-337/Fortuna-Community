# Kiểm tra chi tiết dữ liệu Dashboard

**Thời gian:** 2026-02-02  
**Script chạy:** `./scripts/e2e/run-dashboard-data-tests.sh`

---

## 1. Kết quả chạy E2E

| Bước | Nội dung | Kết quả |
|------|----------|---------|
| Step 1 | Load CVE data | ✅ Đã load 74561 CVE, 34036 package vulnerabilities |
| Step 1b | Deploy E2E vuln pod (debian:10) | ✅ namespace fortuna-e2e + pod fortuna-e2e-vuln-debian10 |
| Step 2 | e2e-dashboard-data.sh | ✅ Runtime events processed: 2; Threat Velocity 7 pts (14 risks); PCE Trend 7 pts (69 caps) |
| Step 3 | test-pce-e2e.sh | ⚠️ Pod chưa thấy trong Core sau 90s → không có capabilities mới (PCE Trend vẫn dùng data cũ) |
| Step 4 | test-sbom-pod-flow.sh | ✅ SBOM có trong API sau 15s |
| Step 5 | verify-dashboard-apis.sh | ✅ Tất cả API có response |

---

## 2. API dùng cho Dashboard

### 2.1 GET /api/v1/dashboard/stats

```json
{
  "totalClusters": 1,
  "activeAgents": 2,
  "runningPods": 20,
  "totalRisks": 15,
  "criticalRisks": 0,
  "resolved24h": 0,
  "affectedPodCount": 15
}
```

→ Dashboard dùng: số cluster, agents, pods, risks, affected workloads.

### 2.2 GET /api/v1/dashboard/metrics/threat-velocity?days=7 (Threat Velocity chart)

| Ngày | critical | high | medium | low | **Tổng** |
|------|----------|------|--------|-----|----------|
| 2026-01-27 | 0 | 0 | 0 | 0 | 0 |
| 2026-01-28 | 0 | 0 | 0 | 0 | 0 |
| 2026-01-29 | 0 | 0 | 0 | 0 | 0 |
| 2026-01-30 | 0 | 0 | 0 | 0 | 0 |
| 2026-01-31 | 0 | 0 | 4 | 0 | **4** |
| 2026-02-01 | 0 | 0 | 1 | 0 | **1** |
| 2026-02-02 | 0 | 0 | 10 | 0 | **10** |

→ **Tổng risks trong 7 ngày: 15** (insights type vulnerability, detected_at trong 7 ngày).

### 2.3 GET /api/v1/pod-capabilities/trends?days=7 (PCE Trend chart)

| Ngày | critical | high | medium | low | **Tổng** |
|------|----------|------|--------|-----|----------|
| 2026-01-27 .. 30 | 0 | 0 | 0 | 0 | 0 |
| 2026-01-31 | 15 | 12 | 42 | 0 | **69** |
| 2026-02-01 .. 02 | 0 | 0 | 0 | 0 | 0 |

→ Biểu đồ PCE Trend có **1 ngày có dữ liệu (69 capabilities)**; các ngày khác 0.

### 2.4 GET /api/v1/runtime-signals (Risk Center – Runtime / Escape)

- **total:** 7 signals  
- **Mẫu:** PROC_ROOT_PIVOT (3), FS_ESCAPE_ATTEMPT (3), UNKNOWN (1)  
- Evidence: syscall openat/mount, target /proc/1/root, /proc  

→ Risk Center card "Runtime / Escape signals" có dữ liệu để hiển thị.

### 2.5 GET /api/v1/insights/summary

```json
{
  "total": 16,
  "critical": 1,
  "high": 0,
  "medium": 15,
  "low": 0,
  "byType": { "rbac": 1, "vulnerability": 15 }
}
```

→ Risk Center dùng: 15 vulnerability insights (medium), 1 rbac (critical).

---

## 3. DB kiểm tra nhanh

| Bảng | Điều kiện | Kết quả |
|------|-----------|---------|
| insights | type=vulnerability, detected_at last 7d | 15 medium |
| pod_capabilities | created_at last 7d | 69 rows (2026-01-31) |
| runtime_signals | - | 7 total (3 FS_ESCAPE_ATTEMPT, 3 PROC_ROOT_PIVOT, 1 UNKNOWN) |

---

## 4. Kết luận

- **Threat Velocity (7 Days):** Có 7 điểm, 3 ngày có dữ liệu (4+1+10 = 15 risks). Chart hiển thị được.
- **PCE Trend (7 Days):** Có 7 điểm, 1 ngày có dữ liệu (69). Chart hiển thị được.
- **Runtime / Escape (Risk Center):** 7 signals, PROC_ROOT_PIVOT và FS_ESCAPE_ATTEMPT từ E2E runtime-events.
- **Dashboard stats & Risk list:** totalRisks 15, affectedPodCount 15, insights 15.

**Lưu ý:** Pod từ E2E (e2e-dashboard-pod, pce-test-pod) có thể chưa xuất hiện trong Core /pods sau 90s (agent sync chậm hoặc cluster đặc thù). Runtime-events vẫn được xử lý (runtime_signals tạo bằng pod_uid từ request). PCE Trend đang dùng dữ liệu pod_capabilities cũ (ngày 2026-01-31).

**Cách xem trên Dashboard:** Port-forward dashboard 8081:80 → mở http://localhost:8081 → đăng nhập admin/admin123 → refresh; kiểm tra 2 chart và Risk Center → Runtime / Escape signals.
