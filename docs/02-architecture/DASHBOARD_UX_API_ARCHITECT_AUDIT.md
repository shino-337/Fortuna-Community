# Kiểm tra UX/UI Dashboard với API domain (API architect)

Version: 1.0  
Date: 2026-03  
Tham chiếu: [ROUTE_MIGRATION_MAPPING.md](./ROUTE_MIGRATION_MAPPING.md), [ROUTE_SYNC_ANALYSIS.md](./ROUTE_SYNC_ANALYSIS.md)

---

## 1. Tổng quan

Sau refactor API theo domain (inventory, runtime, risk, graph, audit, policy, cluster), dashboard phải:

- Gọi đúng path domain (không còn path legacy).
- Dùng **uid** (Kubernetes UID) cho pod/serviceaccount khi API chỉ hỗ trợ uid; không dùng id số cho pod detail.
- Dùng **id** (string, cluster/rule/insight id) đúng theo từng API.

Tài liệu này ghi lại **cách UX/UI đang xử lý** từng nhóm API và **điểm đã sửa / cần lưu ý**.

---

## 2. Inventory domain

| API (domain path) | Dashboard (api.ts) | Trang / Component | UX ghi chú |
|-------------------|--------------------|-------------------|------------|
| GET /inventory/pods | getPods() | Resources (Pods), Dashboard | ✅ Query: cluster, namespace, node, page, pageSize. Map trả về pods[].uid. |
| GET /inventory/pods/:uid | getPodByUid(uid), getPod(id) | PodDetail | ✅ PodDetail **chỉ gọi getPodByUid(idOrUid)**. Link từ list dùng **uid** → /resources/pods/uid/:uid. |
| GET /inventory/pods/:uid/spec, /capabilities, /sbom | getPodSpecYaml, getPodCapabilities, getPodSbom (podUid) | PodDetail, Sbom, CapabilityStateChart, AttackSteps* | ✅ Tất cả nhận **podUid** (string). |
| GET /inventory/sbom | getSbomList() | Sbom | ✅ Path /inventory/sbom. List trả về podId = pod UID (backend PodID := sbom.PodUID). |
| GET /inventory/pods/:uid/sbom | getPodSbom(podUid) | Sbom | ✅ Sbom dùng pod.podId (là UID từ list). |
| GET /inventory/clusters, /clusters/stats | getClusters(), getClustersStats() | Layout, Dashboard, Clusters, ClusterDetail | ✅ Path đúng. Cluster id = string (internal id). |
| GET /inventory/clusters/:id/overview, /inventory, /agents, /security-summary, /nodes/:nodeName | getClusterOverview, getClusterInventory, getClusterAgents, getClusterSecuritySummary, getClusterNode | ClusterDetail, NodeDetail | ✅ Tham số **id** = cluster id (string). getClusterInventory đã sửa path /inventory/clusters/:id/inventory. |
| GET /inventory/serviceaccounts, /:uid, /:uid/permissions | getServiceAccounts*, getServiceAccount(uid), getServiceAccountByUid(uid), getServiceAccountPermissions(uid) | Resources (ServiceAccount), IdentityDetail | ✅ IdentityDetail: lấy permissions bằng **(saData.uid ?? saData.id)** để tương thích API chỉ nhận :uid. |
| GET /inventory/pod-capabilities/* | getPceSummaryByCluster, getPceSummaryByCapability, getPceSummaryByNamespace, getPceSummaryBySeverity, getPceTrend, getPceCapabilities, getPodCapabilities(podUid) | Dashboard, Insights, Capabilities, PodDetail, CapabilityStateChart | ✅ Path /inventory/pod-capabilities/... và /inventory/pods/:uid/capabilities. |

---

## 3. Runtime domain

| API (domain path) | Dashboard (api.ts) | Trang / Component | UX ghi chú |
|-------------------|--------------------|-------------------|------------|
| GET /runtime/pods/:uid/metrics, /processes, /network, /events | getPodRuntimeMetrics, getPodProcesses, getPodNetworkConnections, getPodEvents(podUid) | PodDetail | ✅ PodDetail dùng **pod.uid**. Path: metrics (không còn runtime-metrics), network (không còn network-connections). |
| GET /runtime/signals, /runtime/pods/:uid/signals | getRuntimeSignals(), getRuntimeSignalsByPod(podUid) | Insights, RiskDetail, RuntimeSignalsTable | ✅ Query param: limit, sinceMinutes. |

---

## 4. Risk domain

| API (domain path) | Dashboard (api.ts) | Trang / Component | UX ghi chú |
|-------------------|--------------------|-------------------|------------|
| GET /risk/insights, /risk/insights/summary, /summary/by-cluster, /summary/global | getRisks(), getInsightsSummary(), getInsightsSummaryByCluster(), getInsightsSummaryGlobal() | Dashboard, Insights, RiskCenter | ✅ Query: clusterId, sinceMinutes, page, pageSize, severity, status, search, withScores, priorityLevel. |
| GET /risk/insights/:id, POST .../resolve, .../acknowledge, .../dismiss, PATCH .../:id | getInsight(id), resolveInsight(id), ... | RiskDetail, Insights | ✅ **id** = insight id (string). |
| GET /risk/insights/:id/context | getInsightContext(id) | RiskDetail, PodDetail, SBOM/CVE views | ✅ Trả về insight + pods + cluster + rules liên quan để điều hướng chéo (Risk ↔ Pod ↔ Cluster ↔ Rule). |
| GET /risk/insights/export | exportRisksCSV, exportRisksPDF | RiskCenter / export | ✅ Path /risk/insights/export. |
| GET /risk/pods/:uid/report, /risk/pods/:uid/attack-steps, GET /risk/attack-steps/summary | getPodRiskReport, getPodAttackSteps(podUid), getAttackStepsSummary() | PodDetail, RiskDetail, AttackStepsTable, AttackStepsTimeline | ✅ Pod-scoped dùng **podUid**. |
| GET /risk/rules, /risk/rules/:id, POST/PUT/DELETE | getRiskRules, getRiskRule(id), createRiskRule, updateRiskRule, deleteRiskRule(id) | Settings, Risk rules | ✅ Path /risk/rules. id = rule id. |

---

## 5. Policy domain

| API (domain path) | Dashboard (api.ts) | Trang / Component | UX ghi chú |
|-------------------|--------------------|-------------------|------------|
| GET /policy/rules, /policy/rules/:id, /reload, /:id/metrics, /:id/test | getRules, getRule(id), reloadRules, getRuleMetrics(id), testRule(id) | Rules, RuleDetail | ✅ Path /policy/rules. |

---

## 6. Cluster (infrastructure) & Certificates

| API (domain path) | Dashboard (api.ts) | Trang / Component | UX ghi chú |
|-------------------|--------------------|-------------------|------------|
| GET /cluster/certificates/info, /rotation/history, POST /rotate | getCertificates(), getRotationHistory(), rotateCertificate() | Certificates | ✅ Path /cluster/certificates/*. |
| GET /cluster/info, GET /cluster/:id/nodes | (chưa dùng trong UI) | — | Cluster domain mở rộng; dashboard vẫn dùng /inventory/clusters và /inventory/clusters/:id/nodes/:nodeName. |

---

## 7. Audit & Graph

| API (domain path) | Dashboard (api.ts) | Trang / Component | UX ghi chú |
|-------------------|--------------------|-------------------|------------|
| GET /audit/logs, /audit/reports | getAuditLogs(), getReports() | Audit, Reports | ✅ Path /audit/logs, /audit/reports. |
| GET /graph/attack-paths/graph | getAttackPathsGraph() | AttackPaths | ✅ Trả về nodes/links. |

---

## 8. Dashboard aggregate & khác

| API (domain path) | Dashboard (api.ts) | Trang / Component | UX ghi chú |
|-------------------|--------------------|-------------------|------------|
| GET /dashboard/stats, /dashboard/metrics/threat-velocity | getStats(), getThreatVelocity() | Dashboard, Insights | ✅ Aggregate; query clusterId, days, sinceMinutes. |
| GET /resources, /notifications, /agents/status, /users | getResources(), getNotifications(), getAgents(), getUsers() | Resources, Notifications, Metrics, Users | ✅ Không đổi path. |
| GET /me, POST /change-password, /auth/login | (auth store / login) | Layout, Login | ✅ Không đổi. |
| GET /ws/pod/:uid, /ws/risks | getPodDetailWsUrl(uid), getRisksWsUrl() | PodDetail, Insights | ✅ WebSocket dùng uid cho pod. |

---

## 9. Điểm đã sửa (theo API architect)

1. **Pod detail 404**  
   - **Nguyên nhân:** Link từ Resources/NodeDetail dùng **pod.id** (số) → GET /inventory/pods/2 → 404 (API chỉ có /inventory/pods/:uid).  
   - **Đã sửa:** Resources.tsx, NodeDetail.tsx navigate tới `/resources/pods/uid/${encodeURIComponent(pod.uid)}`. PodDetail luôn gọi getPodByUid(idOrUid). Khi pod không tìm thấy và param trông như số, hiển thị gợi ý "link cũ, mở pod từ Resources".

2. **Cluster inventory 404**  
   - **Nguyên nhân:** getClusterInventory gọi `/clusters/${id}/inventory` (thiếu prefix inventory).  
   - **Đã sửa:** Path thành `/inventory/clusters/${id}/inventory`.

3. **Service Account permissions**  
   - **Nguyên nhân:** IdentityDetail gọi getServiceAccountPermissions(**saData.id**) trong khi API là GET /inventory/serviceaccounts/**:uid**/permissions.  
   - **Đã sửa:** Dùng **uidForPermissions = saData.uid ?? saData.id** và gọi getServiceAccountPermissions(uidForPermissions).

4. **SBOM list/detail**  
   - Backend list trả về **PodID := sbom.PodUID** (UID). Dashboard Sbom.tsx dùng **pod.podId** cho getPodSbom → đúng (podId là UID).

---

## 10. Navigation & identifier nhất quán

| Resource | URL pattern | Identifier dùng cho API |
|----------|-------------|-------------------------|
| Pod | /resources/pods/uid/:uid | uid (Kubernetes UID) |
| Service Account | /identities/uid/:uid hoặc /identities/:id | uid cho API; id fallback cho link cũ |
| Cluster | /clusters/:id | id (cluster internal id) |
| Node | /clusters/:clusterId/nodes/:nodeName | clusterId + nodeName |
| Insight (risk) | /risks/:id | id (insight id) |
| Rule (policy) | /rules/:id | id (rule id) |
| Risk rule | Settings → rule id | id (rule id) |

---

## 11. Xử lý lỗi / empty

- **PodDetail:** Không có pod → "Pod not found". Nếu param trông như số → gợi ý "link cũ, mở từ Resources".
- **IdentityDetail:** Không có sa → "Identity not found", Back to Identities.
- **API lỗi (401/404/5xx):** api.ts throw; nhiều trang catch và set empty array / null, không crash.
- **Dashboard aggregate:** getStats, getInsightsSummary, getThreatVelocity... catch trả về default (0, [], null) để UI không đỏ.

---

*Tài liệu này nên cập nhật khi thêm route domain mới hoặc thay đổi cách dashboard gọi API.*
