# Route Migration Mapping (Legacy → Domain-Architect)

Version: 1.0  
Date: 2026-03  
Reference: [API-ARCHITECT_AND_ROUTE_STANDARD.md](./API-ARCHITECT_AND_ROUTE_STANDARD.md)

Tài liệu này ghi lại **đầy đủ** mọi route đã xóa hoặc điều chỉnh khi chuyển sang kiến trúc domain. Dùng để tra cứu khi debug, viết client mới, hoặc hỗ trợ migration.

---

## Quy ước

- **Old (Legacy):** Path cũ trước refactor (base `/api/v1`).
- **New (Domain):** Path mới theo domain (base `/api/v1`).
- **Status:** `MOVED` | `REMOVED` | `UNCHANGED` | `RENAMED`.
- **Param:** Tham số path (ví dụ `:id` → `:uid`).

---

## 1. Inventory domain (`/api/v1/inventory`)

### Pods

| Old (Legacy) | New (Domain) | Status | Param / Ghi chú |
|--------------|--------------|--------|------------------|
| GET /pods | GET /inventory/pods | MOVED | List pods. |
| GET /pods/by-id/:id | — | REMOVED | Không expose DB id. Dùng GET /inventory/pods/:uid. |
| GET /pods/by-uid/:uid | GET /inventory/pods/:uid | MOVED | Pod detail by Kubernetes UID. Param: :uid. |
| GET /pods/:podUid/spec | GET /inventory/pods/:uid/spec | MOVED | Param: :podUid → :uid. |
| GET /pods/:podUid/capabilities | GET /inventory/pods/:uid/capabilities | MOVED | Param: :podUid → :uid. |
| GET /sbom | GET /inventory/sbom | MOVED | List SBOM (optional; có thể giữ /inventory/pods cho list rồi SBOM theo pod). |
| GET /sbom/:podUid | GET /inventory/pods/:uid/sbom | MOVED | SBOM detail by pod UID. Param: :podUid → :uid. |

### ServiceAccounts

| Old (Legacy) | New (Domain) | Status | Param / Ghi chú |
|--------------|--------------|--------|------------------|
| GET /serviceaccounts | GET /inventory/serviceaccounts | MOVED | List. |
| GET /serviceaccounts/by-uid/:saUid | GET /inventory/serviceaccounts/:uid | MOVED | Param: :saUid → :uid. |
| GET /serviceaccounts/:id | — | REMOVED | Không expose DB id. Dùng GET /inventory/serviceaccounts/:uid. |
| GET /serviceaccounts/:id/permissions | GET /inventory/serviceaccounts/:uid/permissions | MOVED | Param: :id → :uid. |
| PUT /serviceaccounts/:id | PUT /inventory/serviceaccounts/:uid | MOVED | Param: :id → :uid. |
| DELETE /serviceaccounts/:id | DELETE /inventory/serviceaccounts/:uid | MOVED | Param: :id → :uid. |
| POST /serviceaccounts/bulk/disable | POST /inventory/serviceaccounts/bulk/disable | MOVED | Admin. |
| POST /serviceaccounts/bulk/delete | POST /inventory/serviceaccounts/bulk/delete | MOVED | Admin. |
| POST /serviceaccounts/disable-inactive | POST /inventory/serviceaccounts/disable-inactive | MOVED | Admin. |

### Clusters (inventory)

| Old (Legacy) | New (Domain) | Status | Param / Ghi chú |
|--------------|--------------|--------|------------------|
| GET /clusters | GET /inventory/clusters | MOVED | |
| GET /clusters/stats | GET /inventory/clusters/stats | MOVED | |
| GET /clusters/:id/overview | GET /inventory/clusters/:id/overview | MOVED | Cluster id (internal) giữ :id. |
| GET /clusters/:id/inventory | GET /inventory/clusters/:id/inventory | MOVED | |
| GET /clusters/:id/agents | GET /inventory/clusters/:id/agents | MOVED | |
| GET /clusters/:id/security-summary | GET /inventory/clusters/:id/security-summary | MOVED | |
| GET /clusters/:id/nodes/:nodeName | GET /inventory/clusters/:id/nodes/:nodeName | MOVED | |
| GET /clusters/:id | GET /inventory/clusters/:id | MOVED | |

### Deployments / ReplicaSets (inventory)

| Old (Legacy) | New (Domain) | Status | Param / Ghi chú |
|--------------|--------------|--------|------------------|
| GET /deployments | GET /inventory/deployments | MOVED | |
| GET /deployments/:id | GET /inventory/deployments/:id | MOVED | |
| GET /replicasets | GET /inventory/replicasets | MOVED | |
| GET /replicasets/:id | GET /inventory/replicasets/:id | MOVED | |

---

## 2. Runtime domain (`/api/v1/runtime`)

| Old (Legacy) | New (Domain) | Status | Param / Ghi chú |
|--------------|--------------|--------|------------------|
| GET /pods/:podUid/processes | GET /runtime/pods/:uid/processes | MOVED | Param: :podUid → :uid. |
| GET /pods/:podUid/network-connections | GET /runtime/pods/:uid/network | MOVED | Path đổi: network-connections → network. Param: :uid. |
| GET /pods/:podUid/events | GET /runtime/pods/:uid/events | MOVED | Param: :podUid → :uid. |
| GET /pods/:podUid/runtime-metrics | GET /runtime/pods/:uid/metrics | MOVED | Path: runtime-metrics → metrics. Param: :uid. |
| POST /runtime-events | POST /runtime/events | MOVED | |
| GET /runtime-signals | GET /runtime/signals | MOVED | List. |
| GET /runtime-signals/pods/:podUid | GET /runtime/pods/:uid/signals | MOVED | Param: :podUid → :uid. |
| GET /runtime-risk/pods/:podUid | GET /risk/pods/:uid/runtime | MOVED | Xem mục Risk. |
| GET /runtime-risk/pods/:podUid/events | GET /risk/pods/:uid/runtime/events | MOVED | Xem mục Risk. |
| GET /runtime-risk/summary | GET /risk/runtime/summary | MOVED | |
| GET /runtime-risk/top | GET /risk/runtime/top | MOVED | |

---

## 3. Risk domain (`/api/v1/risk`)

| Old (Legacy) | New (Domain) | Status | Param / Ghi chú |
|--------------|--------------|--------|------------------|
| GET /risk/analytics/trends | GET /risk/analytics/trends | UNCHANGED | |
| GET /risk/analytics/comparison | GET /risk/analytics/comparison | UNCHANGED | |
| GET /risk/analytics/correlation | GET /risk/analytics/correlation | UNCHANGED | |
| GET /risk/priorities | GET /risk/priorities | UNCHANGED | |
| GET /risk/top | GET /risk/top | UNCHANGED | |
| GET /risk/grouped | GET /risk/grouped | UNCHANGED | |
| GET /risk/scores | GET /risk/scores | UNCHANGED | |
| GET /risk/scores/:uid | GET /risk/scores/:uid | UNCHANGED | |
| POST /risk/scores/:uid/calculate | POST /risk/scores/:uid/calculate | UNCHANGED | |
| GET /risk/trends | GET /risk/trends | UNCHANGED | |
| GET /risk-rules | GET /risk/rules | RENAMED | Nhóm dưới risk. |
| GET /risk-rules/:id | GET /risk/rules/:id | RENAMED | |
| POST /risk-rules | POST /risk/rules | RENAMED | |
| PUT /risk-rules/:id | PUT /risk/rules/:id | RENAMED | |
| DELETE /risk-rules/:id | DELETE /risk/rules/:id | RENAMED | |
| GET /risks/export | GET /risk/insights/export | RENAMED | Insights export. |
| GET /risks | GET /risk/insights | RENAMED | List insights. |
| PATCH /risks/:riskId | PATCH /risk/insights/:riskId | RENAMED | |
| GET /risks/pods/:podUid/report | GET /risk/pods/:uid/report | MOVED | Param: :podUid → :uid. |
| GET /attack-steps/pods/:podUid | GET /risk/pods/:uid/attack-steps | MOVED | Param: :podUid → :uid. |
| GET /attack-steps/summary | GET /risk/attack-steps/summary | MOVED | |

---

## 4. Graph domain (`/api/v1/graph`)

| Old (Legacy) | New (Domain) | Status | Param / Ghi chú |
|--------------|--------------|--------|------------------|
| GET /graph | GET /graph | UNCHANGED | |
| GET /graph/blast-radius/:id | GET /graph/blast-radius/:uid | RENAMED | Param: :id → :uid. |
| GET /graph/shortest-path | GET /graph/shortest-path | UNCHANGED | |
| GET /graph/accessible/:id | GET /graph/accessible/:uid | RENAMED | Param: :id → :uid. |
| POST /graph/query | POST /graph/query | UNCHANGED | |
| GET /graph/attack-paths/:uid | GET /graph/attack-paths/:uid | UNCHANGED | |
| GET /graph/permissions/:uid | GET /graph/permissions/:uid | UNCHANGED | |
| GET /graph/risky-pods | GET /graph/risky-pods | UNCHANGED | |
| GET /attack-paths/graph | GET /graph/attack-paths/graph | RENAMED | Attack path visualization. |

---

## 5. Audit domain (`/api/v1/audit`)

| Old (Legacy) | New (Domain) | Status | Param / Ghi chú |
|--------------|--------------|--------|------------------|
| GET /audit | GET /audit/logs | RENAMED | Chỉ còn /audit/logs. |
| GET /audit-logs | GET /audit/logs | MOVED | Gộp vào /audit/logs. |
| GET /reports | GET /audit/reports | MOVED | Gộp vào /audit/reports. |
| GET /audit/reports | GET /audit/reports | UNCHANGED | |

---

## 6. Routes giữ nguyên (không đổi path) — sau khi áp dụng Nhóm C

- **Auth:** POST /api/v1/auth/login, POST /api/v1/auth/register  
- **Agent ingest:** POST /api/v1/agent/sync, pod-runtime-metrics, pod-processes, pod-network-connections, pod-events  
- **Health / User:** GET /api/v1/health/dashboard-data-integrity, GET /api/v1/me, POST /api/v1/change-password, GET /api/v1/users  
- **Dashboard:** GET /api/v1/dashboard/stats, GET /api/v1/dashboard/metrics/threat-velocity  
- **WebSocket:** GET /api/v1/ws/pod/:uid, GET /api/v1/ws/risks  
- **Capability metadata / Promotion rules:** GET /api/v1/capability-metadata, /promotion-rules/... (giữ v1 root tạm)  
- **Metrics / Agents / Resources:** GET /api/v1/metrics/system, /error-logs, /agents/status, /resources, /notifications, /monitoring/agents  

**Đã chuyển sang domain (Nhóm C):**

- **Pod capabilities** → GET /api/v1/inventory/pod-capabilities, /inventory/pod-capabilities/summary/*, /inventory/pod-capabilities/trends  
- **Insights (CRUD)** → GET/POST/DELETE/PATCH /api/v1/risk/insights, /risk/insights/:id, .../resolve, .../acknowledge, .../dismiss, .../evaluate  
- **Rules** → GET/POST/PUT/DELETE /api/v1/policy/rules, /policy/rules/:id, .../reload, .../metrics, .../matches, .../test  
- **Policy templates/instances** → /api/v1/policy/templates/*, /api/v1/policy/instances/*  
- **Certificates** → GET/POST /api/v1/cluster/certificates/info, /cluster/certificates/rotate, /cluster/certificates/rotation/history  

---

## 7. Handler param thay đổi

| Handler (hoặc file) | Param cũ | Param mới |
|---------------------|----------|-----------|
| GetPodByUID | :uid | :uid (giữ) |
| GetPod (by id) | :id | REMOVED |
| GetPodSpecYAMLByUID | :podUid | :uid |
| GetPodCapabilities (by UID) | :podUid | :uid |
| GetSBOMDetail | :podUid | :uid |
| GetPodProcessesByUID | :podUid | :uid |
| GetPodNetworkConnectionsByUID | :podUid | :uid |
| GetPodEventsByUID | :podUid | :uid |
| GetPodRuntimeMetricsByUID | :podUid | :uid |
| GetPodRiskReport | :podUid | :uid |
| GetPodRiskProfile / GetPodRuntimeEvents | :podUid | :uid |
| GetPodAttackSteps | :podUid | :uid |
| GetRuntimeSignalsByPod | :podUid | :uid |
| GetServiceAccountByUID | :saUid | :uid |
| GetServiceAccountPermissions | :id | :uid |
| GetBlastRadius | :id | :uid |
| GetAccessibleResources | :id | :uid |
| PodDetailWS | :podUid | :uid |

---

## 8. Dashboard / Client cập nhật

- **Pod list/detail:** Gọi GET /api/v1/inventory/pods, GET /api/v1/inventory/pods/:uid.  
- **Pod spec, capabilities, SBOM:** GET /api/v1/inventory/pods/:uid/spec, .../capabilities, .../sbom.  
- **Pod processes, network, events, metrics:** GET /api/v1/runtime/pods/:uid/processes, .../network, .../events, .../metrics.  
- **Pod risk report, runtime risk:** GET /api/v1/risk/pods/:uid/report, GET /api/v1/risk/pods/:uid/runtime.  
- **Pod attack steps, runtime signals:** GET /api/v1/risk/pods/:uid/attack-steps, GET /api/v1/runtime/pods/:uid/signals.  
- **ServiceAccount:** GET /api/v1/inventory/serviceaccounts/:uid, .../permissions.  
- **Audit:** GET /api/v1/audit/logs, GET /api/v1/audit/reports.  
- **Clusters / Deployments / ReplicaSets:** prefix /api/v1/inventory/...; **cluster domain:** GET /api/v1/cluster/info, GET /api/v1/cluster/:id/nodes (infrastructure view).  
- **Graph:** GET /api/v1/graph/..., param :uid thống nhất.  

**Dashboard APIs** (GET /dashboard/stats, GET /dashboard/metrics/*): **aggregate endpoints** — compose nhiều nguồn, có thể cache; **not stable for external clients**. Chi tiết: [API_ARCHITECTURE_RECOMMENDATIONS.md §6.2](./API_ARCHITECTURE_RECOMMENDATIONS.md).  

---

*Tài liệu này được cập nhật khi hoàn tất refactor domain. Mọi route legacy đã bị xóa và chỉ còn path mới trong code.*

---

## 9. Trạng thái thực hiện (2026-03)

- **Backend:** Đã chuyển sang domain modules: `registerInventoryRoutes`, `registerRuntimeRoutes`, `registerRiskRoutes`, `registerGraphRoutes`, `registerAuditRoutes`. Legacy route (pods/by-id, pods/:podUid/*, serviceaccounts/by-uid, clusters, sbom, risks/*, attack-steps, runtime-risk, graph, audit, audit-logs, reports) đã xóa.
- **Handlers:** Param thống nhất `:uid`; handler đọc `c.Param("uid")` (đã cập nhật GetServiceAccountByUID, GetPodSpecYAMLByUID, GetPodCapabilities, GetSBOMDetail, pod_detail_services, attack_steps, runtime_risk, runtime_signals, risk_reports, graph blast-radius/accessible).
- **Dashboard:** `dashboard/lib/api.ts` đã cập nhật sang path mới (inventory/*, runtime/*, risk/*, graph/*, audit/*).
- **ServiceAccount ByUID:** Đã thêm GetServiceAccountPermissionsByUID, UpdateServiceAccountByUID, DeleteServiceAccountByUID; GetServiceAccountByUID dùng param `uid`.
- **Scripts / CLI / E2E:** Các script trong `scripts/verify/`, `scripts/e2e/`, `scripts/monitor/` đã chuyển sang route domain (inventory/*, runtime/*, risk/*, policy/*, cluster/*, audit/*).
- **Cluster domain mở rộng:** GET /cluster/info (danh sách cluster cơ bản), GET /cluster/:id/nodes (danh sách node của cluster); cùng với /cluster/certificates/*. Chi tiết: [API_ARCHITECTURE_RECOMMENDATIONS.md §6.1](./API_ARCHITECTURE_RECOMMENDATIONS.md).
