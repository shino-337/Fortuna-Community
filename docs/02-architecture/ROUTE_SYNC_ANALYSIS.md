# Phân tích Route: Phương thức cũ còn lại & Đồng bộ Hệ thống

Version: 1.0  
Date: 2026-03  
Tham chiếu: [ROUTE_MIGRATION_MAPPING.md](./ROUTE_MIGRATION_MAPPING.md), [API-ARCHITECT_AND_ROUTE_STANDARD.md](./API-ARCHITECT_AND_ROUTE_STANDARD.md)

---

## 1. Routes vẫn đăng ký theo “phong cách cũ” (không nằm dưới domain)

Các route sau **cố ý giữ** dưới base `/api/v1` (không prefix `/inventory`, `/runtime`, `/risk`, `/graph`, `/audit`) theo mục “Routes giữ nguyên” trong ROUTE_MIGRATION_MAPPING.md.

| Nhóm | Path (base /api/v1) | Lý do không chuyển (hiện tại) |
|------|---------------------|--------------------------------|
| **Auth** | POST /auth/login, POST /auth/register | Chuẩn industry; tách riêng auth không gắn domain nghiệp vụ. |
| **Agent ingest** | POST /agent/sync, /agent/pod-runtime-metrics, /agent/pod-processes, /agent/pod-network-connections, /agent/pod-events | Agent (bên ngoài core) gọi cố định; đổi path phải đổi agent. Giữ ổn định cho tương thích. |
| **Health / User** | GET /health/dashboard-data-integrity, /me, POST /change-password, GET /users | Cross-cutting, không thuộc inventory/runtime/risk. |
| **Dashboard** | GET /dashboard/stats, /dashboard/metrics/threat-velocity | Tổng hợp nhiều nguồn; có thể chuyển sang /dashboard/* rõ ràng hơn sau. |
| **Pod capabilities (aggregate)** | GET /pod-capabilities, /pod-capabilities/summary, .../cluster, .../capability, .../namespace, .../severity, .../trends | API tổng hợp (list/tổng), không theo resource :uid. Có thể chuyển sang /inventory/pod-capabilities sau. |
| **WebSocket** | GET /ws/pod/:uid, GET /ws/risks | Đã dùng :uid; path ngắn cho WS. |
| **Capability metadata** | GET /capability-metadata, /capability-metadata/:capabilityId | Metadata toàn cục; giữ v1 root tạm. |
| **Promotion rules** | GET /promotion-rules, .../capability/:id, .../signal/:type | Tương tự metadata. |
| **Insights (CRUD)** | GET/POST/DELETE /insights, /insights/summary/*, /insights/:id, POST .../evaluate, .../acknowledge, .../resolve, .../dismiss | Mapping ghi “có thể map sang /risk/insights sau”. Lần refactor này giữ /insights để tránh đụng risk list (đã là /risk/insights). |
| **Rules** | GET/POST/PUT/DELETE /rules, .../reload, .../metrics, .../matches | Quản lý rule engine; tách domain riêng sau nếu cần. |
| **Policies** | /policies/templates, /policies/instances | Module policy riêng. |
| **Certificates** | GET /certificates/info, POST /certificates/rotate, GET /certificates/rotation/history | Hạ tầng TLS. |
| **Metrics / Agents / Resources** | GET /metrics/system, /metrics/policy-evaluation-cost, /error-logs, /agents/status, /resources, /notifications, /monitoring/agents | Hệ thống & vận hành; không thuộc domain inventory/risk. |

**Kết luận:** Không phải “quên” chuyển mà là **chủ đích giữ** path cũ cho các nhóm trên; khi nào muốn thống nhất domain có thể chuyển từng nhóm (ví dụ pod-capabilities → /inventory/pod-capabilities, insights CRUD → /risk/insights).

---

## 2. Tổng quan route đã thay đổi (domain)

| Domain | Path mới (base /api/v1) | Đã xóa legacy |
|--------|-------------------------|----------------|
| **Inventory** | /inventory/pods, /inventory/pods/:uid, .../spec, .../capabilities, .../sbom, /inventory/sbom, /inventory/serviceaccounts, .../:uid, .../permissions, PUT/DELETE :uid, bulk, /inventory/clusters, .../stats, .../:id/overview, .../inventory, .../agents, .../security-summary, .../nodes/:nodeName, /inventory/deployments, /inventory/replicasets | /pods, /pods/by-id, /pods/by-uid, /pods/:podUid/*, /serviceaccounts, /serviceaccounts/by-uid, /serviceaccounts/:id, /clusters/*, /deployments/*, /replicasets/*, /sbom, /sbom/:podUid |
| **Runtime** | /runtime/pods/:uid/processes, .../network, .../events, .../metrics, .../signals, POST /runtime/events, GET /runtime/signals | /pods/:podUid/processes, .../network-connections, .../events, .../runtime-metrics, POST /runtime-events, /runtime-signals, /runtime-signals/pods/:podUid |
| **Risk** | /risk/analytics/*, /risk/scores/*, /risk/trends, /risk/rules, /risk/insights, /risk/insights/export, /risk/pods/:uid/report, .../attack-steps, .../runtime, .../runtime/events, /risk/attack-steps/summary, /risk/runtime/summary, /risk/runtime/top | /risk-rules, /risks, /risks/export, PATCH /risks/:riskId, /risks/pods/:podUid/report, /attack-steps/*, /runtime-risk/* |
| **Graph** | /graph, /graph/blast-radius/:uid, /graph/shortest-path, /graph/accessible/:uid, POST /graph/query, /graph/attack-paths/:uid, /graph/attack-paths/graph, /graph/permissions/:uid, /graph/risky-pods | /graph với :id (đổi param), /attack-paths/graph |
| **Audit** | /audit/logs, /audit/reports | /audit (đổi thành /audit/logs), /audit-logs, /reports |

---

## 3. Đồng bộ Agent / Core / Database / Dashboard

### 3.1 Agent

| Thành phần | URL gọi | Chuẩn domain | Đồng bộ? |
|------------|---------|----------------|----------|
| Sync | POST /api/v1/agent/sync | Giữ nguyên (section 6) | Có |
| Pod runtime metrics | POST /api/v1/agent/pod-runtime-metrics | Giữ nguyên | Có |
| Pod processes | POST /api/v1/agent/pod-processes | Giữ nguyên | Có |
| Pod network connections | POST /api/v1/agent/pod-network-connections | Giữ nguyên | Có |
| Pod events | POST /api/v1/agent/pod-events | Giữ nguyên | Có |
| **Runtime events** | POST **/api/v1/runtime-events** | **POST /api/v1/runtime/events** | **Chưa** – agent vẫn gọi path cũ; core đã chỉ còn /runtime/events |

**Hành động:** Cập nhật agent gửi runtime events sang `POST /api/v1/runtime/events`.

### 3.2 Core (backend)

| Thành phần | Trạng thái |
|------------|------------|
| routes.go | Chỉ đăng ký domain modules + routes giữ nguyên; legacy đã xóa. |
| registerInventoryRoutes, registerRuntimeRoutes, registerRiskRoutes, registerGraphRoutes, registerAuditRoutes | Đã dùng đúng path domain. |
| Handlers | Đã chuyển param sang :uid (podUid/saUid/id → uid) cho route domain. |
| **dashboard_data_integrity.go** | Đã cập nhật danh sách endpoint theo path domain (inventory/*, risk/*, audit/*, graph/*, cluster/*, dashboard/*). |

### 3.3 Database

| Ghi chú |
|--------|
| Migration route **không đổi schema DB**. Bảng pods, service_accounts, clusters, audit_logs, insights, v.v. giữ nguyên. Chỉ URL API và param path thay đổi. **Database đồng bộ** với core (core vẫn đọc/ghi cùng DB). |

### 3.4 Dashboard (Frontend)

| API (lib/api.ts) | Path gọi | Path backend thực tế | Đồng bộ? |
|------------------|----------|----------------------|----------|
| getPods, getPod, getPodByUid | /inventory/pods | /inventory/pods | Có |
| getPodRuntimeMetrics, getPodProcesses, getPodNetworkConnections, getPodEvents | /runtime/pods/:uid/... | /runtime/pods/:uid/... | Có |
| getPodSpecYaml, getPodSpecYamlBlob, getPodCapabilities, getPodSbom | /inventory/pods/:uid/... | /inventory/pods/:uid/... | Có |
| getClusterOverview, getClusters, getClustersStats, getCluster, getClusterAgents, getClusterNode | /inventory/clusters/... | /inventory/clusters/... | Có |
| getClusterInventory | /inventory/clusters/:id/inventory | /inventory/clusters/:id/inventory | **Đã đồng bộ** |
| getClusterSecuritySummary | /inventory/clusters/:id/security-summary | /inventory/clusters/:id/security-summary | **Đã đồng bộ** |
| getServiceAccount*, getServiceAccountPermissions | /inventory/serviceaccounts/:uid | /inventory/serviceaccounts/:uid | Có |
| getRisks, exportRisksCSV, exportRisksPDF | /risk/insights, /risk/insights/export | /risk/insights, /risk/insights/export | Có |
| getPodRiskReport, getPodAttackSteps, getAttackStepsSummary | /risk/pods/:uid/..., /risk/attack-steps/summary | Đúng | Có |
| getRuntimeSignals, getRuntimeSignalsByPod | /runtime/signals, /runtime/pods/:uid/signals | Đúng | Có |
| getPodSbom, getSbomList | /inventory/pods/:uid/sbom, /inventory/sbom | Đúng | Có |
| getReports | /audit/reports | /audit/reports | Có |
| getAuditLogs | /audit/logs | /audit/logs | **Đã đồng bộ** |
| getAttackPathsGraph | /graph/attack-paths/graph | /graph/attack-paths/graph | Có |
| risk rules CRUD | /risk/rules | /risk/rules | Có |

---

## 4. Tóm tắt đồng bộ

| Thành phần | Đồng bộ? | Việc cần làm |
|------------|----------|--------------|
| **Agent** | Đồng bộ | Đã đổi POST runtime events sang `/api/v1/runtime/events`. |
| **Core** | Đúng domain | Đã cập nhật `dashboard_data_integrity.go` danh sách path (inventory/clusters, risk/insights, inventory/sbom, audit/logs, audit/reports, graph/attack-paths/graph). |
| **Database** | Đồng bộ | Không thay đổi. |
| **Dashboard** | Đồng bộ | Đã sửa getAuditLogs → `/audit/logs`; getClusterInventory và getClusterSecuritySummary → `/inventory/clusters/...`. |

---

## 5. Unit / integration test (core)

| Test file | Path dùng trong test | Ghi chú |
|-----------|----------------------|--------|
| handlers_pod_test.go | GET /api/v1/pods/by-id/:id, GET /api/v1/pods/by-uid/:uid | Test gắn handler trực tiếp lên router test; **không** dùng SetupRoutes. Route thật đã xóa by-id/by-uid (chỉ còn /inventory/pods/:uid). Test vẫn kiểm tra handler GetPod/GetPodByUID; nếu muốn test đúng route production nên đăng ký /inventory/pods/:uid và gọi path đó. |
| risk_rules_test.go | /api/v1/risk-rules | Route thật là /risk/rules. Test dùng router riêng; cần đăng ký /risk/rules hoặc sửa request path thành /api/v1/risk/rules. |
| risks_list_with_scores_test.go | /api/v1/risks | Route thật là /risk/insights. Cần sửa path trong test thành /api/v1/risk/insights. |
| risks_export_test.go | /api/v1/risks/export | Route thật là /risk/insights/export. Cần sửa path trong test. |

---

## 6. Checklist đã làm / cần làm

- [x] Backend: domain routes đăng ký; legacy xóa.
- [x] Handlers: param :uid thống nhất.
- [x] Dashboard: đa số api.ts đã sang path domain.
- [x] **Agent:** POST runtime events → /api/v1/runtime/events.
- [x] **Dashboard:** getAuditLogs → /audit/logs; getClusterInventory → /inventory/clusters/:id/inventory (đã sửa path sai /clusters/:id/inventory); getClusterSecuritySummary → /inventory/clusters/....
- [x] **Core:** dashboard_data_integrity.go cập nhật danh sách path (+ cluster/info, cluster/:id/nodes).
- [x] **Scripts:** verify-dashboard-apis.sh (inventory/clusters, risk/insights, inventory/pod-capabilities); test-sbom-pod-flow.sh (inventory/sbom, inventory/pods/:uid/sbom).
- [ ] **Tests:** cập nhật path trong test (risk-rules, risks, risks/export, và tùy chọn pod test dùng /inventory/pods/:uid).

### Trước khi chạy clean + rebuild

| Thành phần | Trạng thái đồng bộ |
|------------|---------------------|
| **Core** | Chỉ đăng ký domain routes; không còn legacy path. Agent ingest /api/v1/agent/* giữ nguyên. |
| **Agent** | POST /api/v1/agent/sync, pod-runtime-metrics, pod-processes, pod-network-connections, pod-events; POST /api/v1/runtime/events. |
| **Dashboard** | api.ts gọi /inventory/*, /runtime/*, /risk/*, /cluster/*, /policy/*, /audit/*, /graph/*; getClusterInventory dùng /inventory/clusters/:id/inventory. |
| **DB** | Schema không đổi; core/agent cùng bảng (pods, clusters, insights, runtime_*, …). |
| **Scripts** | verify/e2e/monitor dùng path domain (đã cập nhật). |
