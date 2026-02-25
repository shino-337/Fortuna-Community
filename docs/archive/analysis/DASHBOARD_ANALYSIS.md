# Phân tích Dashboard Fortuna

*Cập nhật: 2026-02-02. Toàn bộ thông tin dashboard hiện tại: chức năng, thông số, cấu trúc trang, luồng xử lý và link giữa các trang.*

---

## 1. Tổng quan

### 1.1 Công nghệ & Cấu trúc

- **Framework:** React (Vite), TypeScript.
- **Router:** HashRouter (`react-router-dom`), base path `#/`.
- **Auth:** JWT lưu trong `authStore` (zustand), token gửi qua header `Authorization: Bearer <token>`.
- **API base:** `VITE_CORE_API_URL` hoặc `/api/v1` (proxy Nginx trong pod dashboard).

### 1.2 Layout chung

- **Sidebar (trái):** Logo Fortuna, menu điều hướng (Main Menu), user profile, Sign Out.
- **Header (desktop):** Tiêu đề trang hiện tại, ô Global search, Refresh interval, chuông Notifications (badge unread), badge "Production".
- **Header (mobile):** Menu hamburger, logo, Refresh interval, chuông Notifications.
- **Nội dung:** `max-w-7xl` center, padding đồng nhất, scroll độc lập.

### 1.3 Bảo vệ route

- **ProtectedRoute:** Tất cả route trong Layout yêu cầu đăng nhập; 401 → logout và redirect `/login`.
- **Redirect:** `/insights` → `/risks`, `/metrics` → `/monitoring`.

---

## 2. Danh sách trang (Routes) và chức năng

| Route | Trang | Mô tả ngắn |
|-------|--------|-------------|
| `/login` | Login | Đăng nhập (username/password → JWT). |
| `/` | Dashboard | Tổng quan: clusters, risks, pods, agents, threat velocity, PCE, top risks, notifications. |
| `/risks` | Risk Center | Insights (vulnerability), filter/pagination, tab Risks / PCE / Reference. |
| `/capabilities` | Capabilities | Capability Catalog – metadata, severity, preconditions, attack steps. |
| `/sbom` | SBOM Analysis | Danh sách pod SBOM, filter pod/namespace, chi tiết package/CVE. |
| `/attack-paths` | Attack Paths | Đồ thị D3 attack path (nodes/links từ API). |
| `/clusters` | Clusters | Danh sách cluster, connection, status, version, resources, pagination. |
| `/resources` | Resources | Tab Pod / ServiceAccount / Role / RoleBinding, bảng + pagination. |
| `/rules` | Rules | Policy-as-Code rules, tab active/disabled/templates. |
| `/certificates` | Certificates | Cert info, rotation history (stub), lifecycle. |
| `/reports` | Reports | Compliance reports, template cards, report history. |
| `/monitoring` | Monitoring | Agents, Sync status, Certificates, Error logs (preview), link Error Logs. |
| `/audit` | Audit Logs | Bảng audit logs, pagination. |
| `/error-logs` | Error Logs | Bảng error logs, filter level/source, pagination. |
| `/notifications` | Notifications | Tabs History / Channels / Rules, list notifications. |
| `/settings` | Settings | Tabs General, Users, Integrations, Audit Logs, API Keys. |

---

## 3. Chi tiết từng trang: API, thông số, cấu trúc hiển thị

### 3.1 Login (`/login`)

- **API:** `POST /auth/login` (username, password) → `{ token, user }`.
- **UI:** Form username/password, nút Login; sau khi đăng nhập → redirect `/`.
- **Link:** Chỉ vào app (không link ra trang khác).

---

### 3.2 Dashboard (`/`)

**API gọi:**
- `getStats()` → `/dashboard/stats` (totalClusters, totalRisks, criticalRisks, runningPods, activeAgents, resolved24h).
- `getClusters()`, `getRisks({ page: 1, pageSize: 50 })`, `getNotifications()`, `getThreatVelocity(7)`, `getPceSummaryByCapability()`, `getPceTrend(7)`.

**Thông số hiển thị (StatCard):**
- **Clusters** – từ stats.totalClusters.
- **Security Risks** – stats.insights, subtitle critical count.
- **Pods** – stats.pods.
- **Agents** – stats.agents.

**Cấu trúc trang:**
- 4 StatCard (Clusters, Security Risks, Pods, Agents).
- Khối “Top Critical Risks” (4 risks) + nút View All → `/risks`.
- Khối “Threat Velocity” (biểu đồ 7 ngày).
- Khối “PCE by Capability” (top 5) + biểu đồ PCE trend.
- Khối “Recent Notifications” (3 mới nhất).
- Empty state khi không có data.

**Link ra:**
- “View All Risks” → `/risks`.
- “Generate Report” → `/reports`.

**Polling:** `REFRESH_INTERVALS.STATS_CLUSTERS` (usePolling).

---

### 3.3 Risk Center (`/risks`)

**API:** `getRisks` (page, pageSize, severity, search), `getPceSummaryBySeverity()`, `getPceCapabilities()`, `getClusters()`, `getStats()` (resolved24h).

**Tabs:** Risks | PCE | Reference (CapabilityMetadataBrowser, RuntimeSignalsTable).

**Thông số:** Danh sách insights (id, title, severity, status, cluster, resources), filter severity, search, pagination. Resolved 24h hiển thị từ stats.

**Cấu trúc:** PageLayout, filter bar, bảng risks hoặc PCE/Reference nội dung.

**Link:** Không link sang trang khác trong mô tả hiện tại (có thể từ Dashboard vào).

---

### 3.4 Capabilities (`/capabilities`)

**API:** `getCapabilityMetadata()` → `/capability-metadata`.

**UI:** Tiêu đề “Capability Catalog”, 1 Card chứa `CapabilityMetadataBrowser` (search, danh sách metadata, severity, preconditions, attack steps).

**Link:** Chỉ từ sidebar.

---

### 3.5 SBOM Analysis (`/sbom`)

**API:** `getSbomList(podName?, namespace?)`, `getPodSbom(podId)`.

**Thông số:** Danh sách pod SBOM (podId, podName, namespace, image, lastScan, packageCount, vulnerabilitySummary). Chi tiết: components, CVEs, severity.

**Cấu trúc:** Layout 2 cột – trái: list pod + filter pod name/namespace; phải: chi tiết SBOM (packages, CVEs, expand).

**Link:** Chỉ từ sidebar.

---

### 3.6 Attack Paths (`/attack-paths`)

**API:** `getAttackPathsGraph()` → `/attack-paths/graph` (nodes, links).

**UI:** Đồ thị D3 (force simulation), nodes theo type (internet, loadbalancer, service, pod, database), tooltip. Nút Refresh.

**Link:** Chỉ từ sidebar.

---

### 3.7 Clusters (`/clusters`)

**API:** `getClustersStats()` → `/clusters/stats`.

**Thông số:** Cluster id/name, connection, status, version/distribution, resources (pod count, etc.). Pagination client-side (slice).

**Cấu trúc:** PageLayout, bảng, Pagination.

**Link:** Chỉ từ sidebar. Mô tả: “Count matches Dashboard Clusters (active within 7 days)”.

---

### 3.8 Resources (`/resources`)

**API:** `getResources(kind)` với kind = Pod | ServiceAccount | Role | RoleBinding.

**UI:** Tab theo kind, bảng Name, Namespace, Service Account/Pods/Rules, Status, Risk, Actions. Pagination client-side.

**Link:** Chỉ từ sidebar.

---

### 3.9 Rules (`/rules`)

**API:** `getRules()` → `/rules`.

**UI:** Tab active / disabled / templates, danh sách rule (expand chi tiết), Reload, Upload YAML.

**Link:** Chỉ từ sidebar.

---

### 3.10 Certificates (`/certificates`)

**API:** `getCertificates()` → `/certificates/info`, `getRotationHistory()` → `/certificates/rotation/history`.

**Thông số:** Cert (name, daysRemaining, status), rotation history (stub []).

**Cấu trúc:** Grid cert cards, tab/section rotation history.

**Link:** Chỉ từ sidebar.

---

### 3.11 Reports (`/reports`)

**API:** `getReports()` → `/reports`.

**UI:** Template cards (CIS, PCI-DSS, System Audit), Report History table.

**Link:** Dashboard “Generate Report” → `/reports`.

---

### 3.12 Monitoring (`/monitoring`)

**API:** `getCertificates()`, `getAgents()`, `getErrorLogs({ page: 1, pageSize: 10 })`, `getSyncStatus()` (từ `/metrics/system`).

**Thông số hiển thị:**
- **Agents** – số lượng từ `getAgents()` (bảng agents).
- **Sync** – pods synced từ `getSyncStatus().resources.pods`.
- **Agent Status** – list 5 agent (node, lastHeartbeat, status).
- **Certificate Status** – 2 cert, days remaining.
- **Error Logs** – 10 bản ghi mới nhất.

**Cấu trúc:** 2 card tổng quan (Agents, Sync), Sync Status & Performance card, cột phải: Agent Status, Certificate Status, Error Logs card.

**Link ra:**
- “View All” (Error Logs) → `/error-logs`.
- “View Full Logs” → `/error-logs`.
- Empty error logs: link “Error Logs” → `/error-logs`.

---

### 3.13 Audit Logs (`/audit`)

**API:** `getAuditLogs({ page, pageSize })` → `/audit`.

**Thông số:** Timestamp, Actor, Action, Resource, Status, Details. Pagination.

**Cấu trúc:** PageLayout, bảng, Pagination, Export CSV (button).

**Link:** Chỉ từ sidebar.

---

### 3.14 Error Logs (`/error-logs`)

**API:** `getErrorLogs({ page, pageSize, level?, source? })` → `/error-logs?page=&pageSize=&level=&source=`.

**Thông số:** Time, Level (ERROR/WARN/INFO), Source (core/agent/worker), Message. Filter dropdown level/source, pagination.

**Cấu trúc:** PageLayout, filter bar, bảng, Pagination, Refresh.

**Link:** Từ Monitoring (View All / View Full Logs / link “Error Logs”).

---

### 3.15 Notifications (`/notifications`)

**API:** `getNotifications()` → `/notifications` (bảng notifications).

**UI:** Tabs History / Channels / Rules. List notifications (title, message, severity, timestamp). “Mark all read”.

**Link:** Header chuông → `/notifications`; Layout dùng unread count cho badge.

---

### 3.16 Settings (`/settings`)

**API:** `getUsers()` (tab Users), `getAuditLogs()` (tab Audit Logs).

**Tabs:** General (theme, scan frequency), Users, Integrations, Audit Logs, API Keys.

**Cấu trúc:** Tab bar, nội dung theo tab (General form, Users table, Audit table, v.v.).

**Link:** Chỉ từ sidebar.

---

## 4. API tổng hợp (lib/api.ts)

| Method | Backend path | Mô tả |
|--------|--------------|--------|
| login | POST /auth/login | Đăng nhập |
| getStats | GET /dashboard/stats | Tổng quan số liệu |
| getClusters | GET /clusters | Danh sách cluster (active) |
| getClustersStats | GET /clusters/stats | Cluster + podCount, connectionStatus |
| getRisks | GET /risks | Insights (type, page, severity, search) |
| getResources | GET /resources?kind= | Pod/ServiceAccount/Role/RoleBinding |
| getRules | GET /rules | Security rules |
| getCertificates | GET /certificates/info | Cert info (khi TLS) |
| getAgents | GET /agents/status | Agents (bảng agents) |
| getUsers | GET /users | Users (admin khi auth) |
| getNotifications | GET /notifications | Notifications (bảng notifications) |
| getRotationHistory | GET /certificates/rotation/history | Rotation history (stub []) |
| getAuditLogs | GET /audit?page=&pageSize= | Audit logs |
| getErrorLogs | GET /error-logs?page=&pageSize=&level=&source= | Error logs (có phân trang, filter) |
| getReports | GET /reports | Reports |
| getSyncStatus | GET /metrics/system | Sync + resources (pods, sas, roles, bindings) |
| getSbomList | GET /sbom?podName=&namespace= | Danh sách pod SBOM |
| getPodSbom | GET /sbom/:podId | Chi tiết SBOM 1 pod |
| getThreatVelocity | GET /dashboard/metrics/threat-velocity?days= | Trend risk theo ngày |
| getPceSummaryByCluster | GET /pod-capabilities/summary/cluster | PCE theo cluster |
| getPceSummaryByCapability | GET /pod-capabilities/summary/capability | PCE theo capability |
| getPceSummaryByNamespace | GET /pod-capabilities/summary/namespace | PCE theo namespace |
| getPceSummaryBySeverity | GET /pod-capabilities/summary/severity | PCE theo severity |
| getPceTrend | GET /pod-capabilities/trends?days= | PCE trend |
| getPceCapabilities | GET /pod-capabilities | Danh sách capability (filter) |
| getPodCapabilities | GET /pods/:podUid/capabilities | Capabilities của 1 pod |
| getCapabilityMetadata | GET /capability-metadata | Metadata capability |
| getCapabilityMetadataById | GET /capability-metadata/:id | Chi tiết 1 metadata |
| getPodAttackSteps | GET /attack-steps/pods/:podUid | Attack steps của pod |
| getAttackStepsSummary | GET /attack-steps/summary | Tổng hợp attack steps |
| getPromotionRules | GET /promotion-rules | Promotion rules |
| getRuntimeSignals | GET /runtime-signals | Runtime signals (filter) |
| getAttackPathsGraph | GET /attack-paths/graph | Nodes + links cho đồ thị |

Tất cả dữ liệu hiển thị đều từ API (Core/DB), không còn mock.

---

## 5. Luồng xử lý

### 5.1 Auth

1. User mở app → ProtectedRoute kiểm tra `isAuthenticated` (từ authStore).
2. Chưa login → redirect `/login`.
3. Login thành công → lưu token + user vào authStore → redirect `/`.
4. Mọi request gửi kèm `Authorization: Bearer <token>`.
5. Core trả 401 → api.ts gọi logout, throw → UI có thể redirect login.

### 5.2 Refresh & Polling

- **RefreshIntervalSelector:** Lưu interval (vd 10s, 30s) trong store; các trang dùng `useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.XXX))`.
- **usePolling(fetchData, intervalMs):** Gọi fetchData định kỳ (Dashboard, Risk Center, Clusters, SBOM, Monitoring, v.v.).
- **Refresh thủ công:** Nhiều trang có nút Refresh/Reload gọi lại fetch.

### 5.3 Load dữ liệu trang

- **Dashboard:** getStats trước, sau đó song song getClusters, getRisks, getNotifications, getThreatVelocity, getPceSummaryByCapability, getPceTrend.
- **Risk Center:** getRisks + getPceSummaryBySeverity + getPceCapabilities + getClusters + getStats (resolved24h).
- **SBOM:** getSbomList; khi chọn pod → getPodSbom(podId).
- **Error Logs:** getErrorLogs({ page, pageSize, level, source }) khi đổi trang/filter.

---

## 6. Link giữa các trang (Navigation & Cross-links)

### 6.1 Sidebar (Layout) – điều hướng chính

- Dashboard → `/`
- Risk Center → `/risks`
- Capabilities → `/capabilities`
- SBOM Analysis → `/sbom`
- Attack Paths → `/attack-paths`
- Clusters → `/clusters`
- Resources → `/resources`
- Rules → `/rules`
- Certificates → `/certificates`
- Reports → `/reports`
- Monitoring → `/monitoring`
- Audit Logs → `/audit`
- Error Logs → `/error-logs`
- Settings → `/settings`

### 6.2 Header

- **Chuông:** navigate `/notifications`.
- **Global search:** ô tìm (chưa gắn route/API cụ thể trong mô tả).

### 6.3 Cross-links trong nội dung

| Từ trang | Nội dung | Link đến |
|---------|----------|----------|
| Dashboard | “View All Risks” | `/risks` |
| Dashboard | “Generate Report” | `/reports` |
| Monitoring | “View All” (Error Logs card) | `/error-logs` |
| Monitoring | “View Full Logs” | `/error-logs` |
| Monitoring | Empty error logs text “Error Logs” | `/error-logs` |

### 6.4 Redirect

- `/insights` → `/risks`
- `/metrics` → `/monitoring`

---

## 7. Component dùng chung

| Component | Vai trò |
|-----------|--------|
| PageLayout | Title, description, actions (nút) cho trang. |
| Pagination | Phân trang (page, pageSize, total, onPageChange, pageSizeOptions). |
| StatCard | Thẻ số liệu (title, value, icon, color, subtitle). |
| Card (ui) | Khung có title, description, actions. |
| Button (ui) | Primary/secondary/ghost. |
| RefreshIntervalSelector | Chọn khoảng refresh (header). |
| CapabilityMetadataBrowser | Duyệt capability metadata (Risk Center tab Reference, trang Capabilities). |
| RuntimeSignalsTable | Bảng runtime signals (Risk Center tab Reference). |
| CapabilityStateChart | Biểu đồ state capability (trong CapabilityMetadataBrowser hoặc tương tự). |
| AttackStepsTable / AttackStepsTimeline | Hiển thị attack steps (có thể dùng trong Risk Center/Attack Paths). |

---

## 8. Thông số đồng bộ giữa Dashboard và Core

- **Clusters:** Cùng cutoff “active 7 ngày” (Dashboard stats vs Clusters page).
- **Security Risks:** Cùng insight_type=vulnerability, count khớp Dashboard vs Risk Center.
- **Agents:** Bảng `agents`, last_seen 10 phút.
- **Pods:** Bảng `pods`, deleted_at IS NULL.
- **Resolved 24h:** Insights status=resolved, updated_at 24h.

Tài liệu chi tiết API/data: `docs/PENDING_TASKS_SUMMARY.md`, `docs/UI_AND_PENDING_TASKS.md`, `core/internal/api/dashboard_data_integrity.go`.
