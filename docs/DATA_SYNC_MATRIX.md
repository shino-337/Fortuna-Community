# Ma trận đồng bộ dữ liệu – Dashboard và các trang

*Cập nhật: 2026-02-01*

---

## 1. Nguồn dữ liệu chung (SSOT)

| Số liệu | API / Backend | Dashboard | Trang liên quan | Ghi chú |
|--------|----------------|-----------|------------------|--------|
| **Clusters** | `GET /dashboard/stats` (totalClusters), `GET /clusters`, `GET /clusters/stats` | StatCard "Clusters" | Clusters | Cùng cutoff: `last_sync >= now - 7 days`. Số trên Dashboard = số dòng trên Clusters. |
| **Security Risks** | `GET /dashboard/stats` (totalRisks), `GET /risks?type=vulnerability` | StatCard "Security Risks" | Risk Center (Insights) | Cùng định nghĩa: `insight_type = vulnerability`, `deleted_at IS NULL`. Total Risk Center = totalRisks. |
| **Pods** | `GET /dashboard/stats` (runningPods) | StatCard "Pods" | Resources (tab Pods), SBOM | `pods` table, `deleted_at IS NULL`. |
| **Agents** | `GET /dashboard/stats` (activeAgents), `GET /agents/status` | StatCard "Agents" | Monitoring (Metrics) | `agents` table, status=ready, last_seen within 10 min. |
| **Resolved (24h)** | `GET /dashboard/stats` (resolved24h) | (subtitle StatCard nếu có) | Risk Center | Insights `status=resolved`, `updated_at` trong 24h. |

---

## 2. Phân trang và total

| Trang | API | Phân trang | Total | Đồng bộ |
|-------|-----|------------|--------|--------|
| **Risk Center (Risks)** | `GET /risks?type=vulnerability&page=&pageSize=&severity=&search=` | Server-side | `total` từ API | Total = số vulnerability (trùng với Dashboard Security Risks khi filter = all, search = rỗng). |
| **Audit Logs** | `GET /audit?page=&pageSize=` | Server-side | `total` từ API | Pagination dùng total từ backend. |
| **Clusters** | `GET /clusters/stats` | Client-side (slice) | `clusters.length` | Cùng danh sách với Dashboard count (cùng cutoff). |
| **Resources** | `GET /resources?kind=Pod|ServiceAccount|Role|RoleBinding` | Client-side | `resources.length` | Một request lấy hết, slice phía client. |
| **Insights (PCE tab)** | `GET /pod-capabilities`, `GET /pod-capabilities/summary/severity` | N/A | Từ API | Độc lập với risks. |

---

## 3. API đã đồng bộ (field mapping)

### 3.1 GET /risks

- **Backend:** `dashboard_handlers.GetInsightsList` – mặc định `type=vulnerability`, filter `severity`, `status`, `search`, phân trang `page`, `pageSize`.
- **Frontend:** `api.getRisks({ page, pageSize, type?, severity?, status?, search? })` → `{ insights, total, page, pageSize }`.
- **Trang:** Dashboard dùng top risks (page 1, pageSize 50). Risk Center dùng server-side pagination và total từ API.

### 3.2 GET /dashboard/stats

- **Backend:** totalClusters, activeAgents, runningPods, totalRisks, criticalRisks, resolved24h.
- **Frontend:** `api.getStats()` → clusters, insights, critical, pods, agents, resolved24h.
- **Nhãn:** STAT_LABELS.CLUSTERS, SECURITY_RISKS, PODS, AGENTS (dashboard/constants/labels.ts).

### 3.3 GET /clusters và GET /clusters/stats

- **Backend:** Cùng `ActiveClusterCutoff` (7 ngày) với GetDashboardStats.
- **Frontend:** `getClusters()`, `getClustersStats()` – Clusters page dùng getClustersStats; Dashboard có thể dùng getClusters cho danh sách.

### 3.4 GET /audit

- **Backend:** Trả về `logs`, `total`, `page`, `pageSize`.
- **Frontend:** `api.getAuditLogs({ page, pageSize })` → `{ logs, total, page, pageSize }`. Audit page dùng server-side pagination.

---

## 4. Các trang và dữ liệu hiển thị

| Trang | Dữ liệu chính | Nguồn | Ghi chú |
|-------|----------------|-------|--------|
| **Dashboard** | Stats (4 thẻ), threat velocity, PCE trend, top risks, notifications | getStats, getClusters, getRisks(1,50), getNotifications, getThreatVelocity, getPceSummaryByCapability, getPceTrend | Số "Security Risks" = totalRisks (vulnerability). |
| **Risk Center** | Total risks, filter severity/search, bảng risks (paginated) | getRisks(page, pageSize, severity, search), getPceSummaryBySeverity, getPceCapabilities, getClusters, getStats | Total từ API = tổng vulnerability (khớp Dashboard khi không filter). |
| **Clusters** | Bảng clusters | getClustersStats | Mô tả: "Count matches Dashboard Clusters (active within 7 days)". |
| **Resources** | Pods, ServiceAccounts, Roles, RoleBindings | getResources(kind) | Tab = kind. |
| **SBOM** | Danh sách pod + chi tiết SBOM | getSbomList, getPodSbom | 1/3 danh sách, 2/3 chi tiết. |
| **Audit** | Bảng audit logs | getAuditLogs(page, pageSize) | Server-side pagination, total từ API. |
| **Notifications** | Danh sách thông báo | getNotifications | Backend stub `[]` cho đến khi có bảng notifications. |
| **Monitoring** | Certificates, agents, queue, error logs, sync status | getCertificates, getAgents, getQueueMetrics, getErrorLogs, getSyncStatus | Một số metric vẫn stub/unsupported. |

---

## 5. Kiểm tra nhanh đồng bộ

1. **Dashboard "Clusters"** = số dòng trên **Clusters** (cùng cutoff 7 ngày).
2. **Dashboard "Security Risks"** = **Risk Center** total khi filter = "all", search = rỗng (cùng type=vulnerability).
3. **Dashboard "Pods"** = tổng pod từ DB; **Resources (tab Pods)** dùng cùng bảng pods (có thể khác nếu filter cluster/namespace).
4. **Audit** total = từ `GET /audit` (total), phân trang đúng theo page/pageSize.
