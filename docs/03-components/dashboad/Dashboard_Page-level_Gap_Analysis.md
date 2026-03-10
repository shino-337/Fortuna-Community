# Dashboard Page-level – Gap Analysis (Spec vs Core / Agent / DB)

Tài liệu này đối chiếu **Dashboard Page-level UX Spec** với hiện trạng **Core API**, **Agent**, **DB** để xác định đã có gì, thiếu gì, và kế hoạch bổ sung.

---

## 1. Dashboard (Home)

| Spec | Hiện có (Core/DB) | Thiếu / Ghi chú |
|------|-------------------|------------------|
| Cluster selector (global) | Không có | UI: chưa có global cluster filter. API: GET /clusters có danh sách cluster. **Plan:** Thêm dropdown cluster ở header (optional). |
| Summary: Total Risks (C/H/M/L) | GET /dashboard/stats có totalRisks, criticalRisks. GET /insights/summary có total, critical, high, medium, low. | Dashboard chưa gọi insights/summary → chưa hiển thị breakdown C/H/M/L. **Làm ngay:** Gọi GET /insights/summary, hiển thị 4 card severity, click → Risk Center (filter by severity). |
| Exposed Capabilities (PCE) | GET /pod-capabilities/summary, /summary/severity. | UI đã có PCE summary trên Dashboard. OK. |
| Affected Workloads | Không có API riêng. | Spec: "Affected Workloads" = số workload (pod) có ít nhất 1 risk. **Thiếu:** API kiểu "count distinct pods with active insights" hoặc dùng insights list + đếm unique resource (pod). **Plan:** Core thêm field vào dashboard/stats (e.g. affectedPods) hoặc GET /insights/summary trả thêm affectedPodCount. |
| Top Risks (5–10) | GET /risks với pageSize. | Đã có. OK. |
| Recent Changes / Events | GET /notifications, GET /audit. | Có API. Dashboard có thể dùng notifications hoặc audit (hiện dùng notifications). OK. |
| Click severity → Risk Center (pre-filtered) | GET /risks?severity=critical. | **Làm ngay:** Card severity click → navigate('/risks?severity=...'). |
| Refresh, Filter Cluster | Refresh có (polling). Cluster filter chưa. | UI: thêm cluster filter khi có global cluster selector. |

---

## 2. Cluster List

| Spec | Hiện có | Thiếu |
|------|---------|--------|
| Table: Cluster Name, Version, Health, **Risk Count**, **Agents** | GET /clusters/stats: name, version (k8sVersion), connectionStatus (Health), podCount, deploymentCount. | **Risk Count per cluster:** Insights không có cluster_id; cần join qua pods (resource_uid = pod.uid, pod.cluster_id). **Agents per cluster:** Bảng agents không có cluster_id; có thể suy từ node_name (pods.cluster_id + pods.node_name → distinct node_name per cluster → đếm agents theo node_name). **Plan:** Core GetClustersStats bổ sung RiskCount, AgentCount (hoặc endpoint riêng). |
| Click row → Cluster Detail | GET /clusters/:id có. | **UI:** Chưa có trang Cluster Detail với tabs. **Làm ngay:** Route /clusters/:id, trang Cluster Detail đơn giản (header + tabs Overview / Inventory / Agents / Security Summary). Dữ liệu: Overview = pod count từ stats; Inventory = distinct namespace từ pods (cần GET /clusters/:id/inventory hoặc query pods); Agents = list agents (hiện không theo cluster → hiển thị tất cả hoặc "N/A"); Security Summary = risk by severity (cần GET /clusters/:id/security-summary hoặc lọc insights). **Plan backend:** GET /clusters/:id/overview, /inventory, /agents, /security-summary. |
| Search by name, Filter Health | Chưa. | UI: filter client-side từ list clusters. |

---

## 3. Cluster Detail (tabs)

| Tab | API cần | Hiện có |
|-----|---------|---------|
| Overview | Node count, namespace count, pod count | Pod count có (từ clusters/stats). Node count: có thể COUNT DISTINCT node_name FROM pods WHERE cluster_id. Namespace count: COUNT DISTINCT namespace. **Plan:** GET /clusters/:id/overview trả về nodeCount, namespaceCount, podCount. |
| Inventory | Nodes list, Namespaces list | Không có bảng nodes riêng; nodes = distinct node_name từ pods. Namespaces = distinct namespace từ pods. **Plan:** GET /clusters/:id/inventory trả nodes[], namespaces[] (từ pods). |
| Agents | Agents cho cluster | Agents table không có cluster_id. Suy từ pods: distinct node_name → match agents.node_name. **Plan:** GET /clusters/:id/agents hoặc tham số ?clusterId= cho GET /agents/status. |
| Security Summary | Risk by severity, Capability exposure | Insights theo cluster: cần join qua pods. **Plan:** GET /clusters/:id/security-summary. |

---

## 4. Node Detail

| Spec | Hiện có | Thiếu |
|-----|---------|--------|
| Node metadata (role, OS, runtime) | Pods có node_name. Không có bảng nodes với metadata. | **DB/Agent:** Cần thu thập node metadata (role, OS, runtime) và lưu (bảng nodes hoặc từ sync). **Plan:** Agent/Core bổ sung node metadata; GET /nodes/:name hoặc /clusters/:id/nodes/:name. |
| Tabs: Overview, Workloads, Risks | — | Phụ thuộc API node. **Plan:** Sau khi có node API, trang Node Detail + tabs. |

---

## 5. Workloads / Pod List

| Spec | Hiện có | Thiếu |
|------|---------|--------|
| Table: Pod name, namespace, node, **risk count** | GET /pods, GET /resources (kind=Pod). Pods có name, namespace, node_name. | **Risk count per pod:** Cần đếm insights theo resource_uid (pod UID). **Plan:** Core GET /pods trả thêm riskCount hoặc GET /pods với join insights. |
| Click pod → Pod Detail | GET /pods/:id có. | **UI:** Trang Pod Detail với tabs (Overview, Runtime, Security Context, SBOM, RBAC, Related Risks). Một phần API đã có: GET /pods/:id, /pods/:id/capabilities, GET /sbom/:podId, GET /risks/pods/:podUid/report. **Thiếu:** GET /pods/:id/runtime, /security-context, /rbac (có thể dùng từ pod model + serviceaccounts). **Làm ngay:** Route /resources/pods/:id hoặc /pods/:id, trang Pod Detail dùng API có sẵn; tab nào chưa có API thì ẩn hoặc empty state. |

---

## 6. Pod Detail (tabs)

| Tab | API | Hiện có |
|-----|-----|---------|
| Overview | Image, labels, namespace, node | GET /pods/:id trả pod; có thể có image, namespace, node_name. |
| Runtime | Containers, ports, processes | GET /pods/:id/runtime – **kiểm tra Core có route này không.** |
| Security Context | Privileged, capabilities, seccomp | Pod model có pod_security_context, container_security_contexts. GET /pods/:id hoặc /security-context. |
| SBOM | Package list, CVE | GET /sbom/:podId, GET /sbom (list). OK. |
| RBAC Context | ServiceAccount, Roles/bindings | Từ pod.service_account + GET /serviceaccounts/:id/permissions. Có thể thêm GET /pods/:id/rbac. |
| Related Risks | Risks for pod | GET /risks/pods/:podUid/report. OK. |

---

## 7. Risk Center

| Spec | Hiện có | Thiếu |
|------|---------|--------|
| Table: Risk ID, Severity, Type, Assets, Status | GET /risks trả insights với severity, insightType, affectedResources (map từ resource), status. | **UI:** Đảm bảo cột hiển thị đúng Risk ID (cveId hoặc id), Severity, Type, Assets (số hoặc tóm tắt), Status. Click row → Risk Detail. **Làm ngay:** Link row tới /risks/:id. |
| Filter: Severity, Status, Asset type; Search | GET /risks?severity=, status=, search=. | Đã có. UI đã có filter. OK. |

---

## 8. Risk Detail

| Spec | Hiện có | Thiếu |
|------|---------|--------|
| Summary banner | GET /insights/:id trả 1 insight (title, severity, status, resource, v.v.). | **UI:** Trang Risk Detail (route /risks/:id) hiển thị insight. OK. |
| Affected Assets | 1 insight = 1 resource (resourceType, resourceName, resourceNamespace, resourceUid). | Có trong insight. Hiển thị 1 dòng; click → Pod/Identity Detail. |
| Evidence | Không có field evidence trên Insight. | **Plan:** Mở rộng model Insight (evidence JSONB) hoặc bảng evidence; API GET /risks/:id/evidence. |
| Violated Rules | Không có link insight → rule. | **Plan:** Bảng policy_violations hoặc insight_rule; GET /risks/:id/violated-rules. |
| Timeline | detectedAt, updatedAt, resolvedAt. | Có trên Insight. Hiển thị timeline đơn giản. **Làm ngay:** Dùng detectedAt, updatedAt, resolvedAt. |
| Mark as resolved | POST /insights/:id/resolve. | Có. **Làm ngay:** Nút "Mark as resolved" gọi API. |

---

## 9–10. Capabilities (PCE)

| Spec | Hiện có | Thiếu |
|------|---------|--------|
| List capabilities, severity, affected assets | GET /pod-capabilities, /pod-capabilities/summary, capability-metadata. | OK. |
| Capability Detail: description, preconditions, attack steps, affected pods | GET /capability-metadata/:id, attack-steps/pods/:podUid. | Có. UI: đảm bảo có trang Capability Detail hoặc panel. |

---

## 11–12. Identities (RBAC) / Identity Detail

| Spec | Hiện có | Thiếu |
|------|---------|--------|
| List ServiceAccounts / Roles | GET /serviceaccounts, GET /resources (kind=ServiceAccount). | Identities → Resources?tab=ServiceAccount. OK. |
| Identity Detail: Permissions matrix, bound workloads, related risks | GET /serviceaccounts/:id, GET /serviceaccounts/:id/permissions. | Có. **UI:** Trang Identity Detail (route /identities/:id hoặc /serviceaccounts/:id) hiển thị permissions, workloads (linkedPods), risks (cần API risks by SA – có thể GET /risks?search= hoặc endpoint riêng). |

---

## 13–14. Rules & Policies / Rule Detail

| Spec | Hiện có | Thiếu |
|------|---------|--------|
| Rule list | GET /rules. | OK. |
| Rule Detail: intent, definition, violations | GET /rules/:id, GET /rules/:id/matches. | Có. **UI:** Rule Detail page với link violations → Risk Detail. |

---

## 15. Attack Paths

| Spec | Hiện có | Thiếu |
|------|---------|--------|
| Graph visualization | GET /attack-paths/graph. | OK. Click node → Object Detail (Pod/Identity) – **UI:** implement. |

---

## 16. Monitoring

| Spec | Hiện có | Thiếu |
|------|---------|--------|
| Agent health, Sync status, Errors | GET /agents/status, GET /sync/status (hoặc tương đương), GET /error-logs. | OK. |

---

## 17. Global UI

| Spec | Hiện có | Thiếu |
|------|---------|--------|
| Global Cluster Selector | Chưa. | **Plan:** Dropdown cluster ở header (optional). |
| Time Range | Chưa. | Optional. |
| Manual Refresh | RefreshIntervalSelector, nút Refresh trên từng trang. | OK. |

---

## Tổng hợp: Làm ngay (chỉ UI + API có sẵn)

1. **Dashboard:** Gọi GET /insights/summary; hiển thị 4 card Critical/High/Medium/Low (click → /risks?severity=...).
2. **Cluster List:** Thêm cột Risk Count, Agents (tạm "—" hoặc placeholder) nếu chưa có API; click row → navigate('/clusters/:id').
3. **Cluster Detail:** Trang mới /clusters/:id với GET /clusters/:id (header) + Overview (pod count từ stats hoặc từ cluster object); các tab khác empty state hoặc "Coming soon" nếu chưa có API.
4. **Risk Center:** Đảm bảo cột Risk ID, Severity, Type, Assets, Status; click row → /risks/:id.
5. **Risk Detail:** Trang mới /risks/:id với GET /insights/:id; Summary, Affected Assets (1 resource), Timeline (detectedAt, updatedAt, resolvedAt), nút Mark as resolved.
6. **Pod Detail:** Trang /pods/:id hoặc /resources/pods/:id với tabs (Overview, SBOM, Related Risks từ API có sẵn); tab chưa có API thì ẩn.

---

## Plan bổ sung Backend / DB / Agent

1. **Core – Clusters:** GetClustersStats thêm RiskCount, AgentCount per cluster (risk count: đếm insights join pods theo resource_uid; agent count: distinct node_name từ pods theo cluster → đếm agents match node_name).
2. **Core – Cluster Detail:** GET /clusters/:id/overview (nodeCount, namespaceCount, podCount), GET /clusters/:id/inventory (nodes[], namespaces[] từ pods), GET /clusters/:id/agents (agents có node_name thuộc cluster), GET /clusters/:id/security-summary (risk by severity, capability summary).
3. **Core – Dashboard stats:** Thêm affectedPodCount (count distinct pods có insight active) vào GET /dashboard/stats hoặc GET /insights/summary.
4. **Core – Pods list:** GET /pods trả thêm riskCount (count insights where resource_uid = pod.uid).
5. **Core – Risk Detail:** GET /risks/:id/evidence, GET /risks/:id/violated-rules (nếu có model); hoặc mở rộng GET /insights/:id trả thêm evidence, violated_rule_ids.
6. **DB/Agent – Node metadata:** Bảng nodes (node_name, cluster_id, role, os, runtime) và sync từ agent; GET /nodes/:name hoặc /clusters/:id/nodes.

---

## Đã thực hiện (Implementation Done)

* **Core – dashboard/stats:** Đã thêm `affectedPodCount` (count distinct resource_uid từ insights active, resource_type = Pod). Dashboard gọi getStats và hiển thị card "Affected Workloads".
* **Core – clusters/stats:** Đã thêm `riskCount` (insights active join pods theo cluster_id) và `agentCount` (agents có node_name trong pods của cluster). Cluster List hiển thị 2 cột Risk Count, Agents; click row → Cluster Detail.
* **Dashboard – Home:** Gọi GET /insights/summary; hiển thị 4 card Critical/High/Medium/Low (click → /risks?severity=...) + card Affected Workloads (từ stats.affectedPodCount).
* **Cluster List:** Cột Risk Count, Agents từ API; click row → /clusters/:id.
* **Cluster Detail:** Trang /clusters/:id với tabs Overview, Inventory, Agents, Security Summary; header + summary từ GET /clusters/:id và getClustersStats (để có podCount, riskCount, agentCount); các tab Inventory/Agents/Security Summary dùng empty state cho đến khi có API.
* **Risk Center:** Filter severity từ URL (?severity=critical|high|medium|low); click row → /risks/:id (dùng id pk từ API).
* **Risk Detail:** Trang /risks/:id với GET /insights/:id; Summary, Affected Assets, Timeline (detectedAt, updatedAt, resolvedAt), nút Mark as resolved (POST /insights/:id/resolve); Evidence/Violated Rules placeholder.

**GAP còn lại – đã xử lý (tiếp):**

* **Core – Cluster Detail APIs:** GET /clusters/:id/overview (nodeCount, namespaceCount, podCount), GET /clusters/:id/inventory (nodes[], namespaces[] từ pods), GET /clusters/:id/agents (agents có node_name thuộc cluster), GET /clusters/:id/security-summary (risk by severity, capabilityCount). Dashboard Cluster Detail gọi các API này và hiển thị dữ liệu thực.
* **Core – GetPods:** Trả thêm riskCount cho mỗi pod (đếm insights active theo resource_uid). GET /pods/:id cũng trả riskCount.
* **Dashboard – Resources Pod tab:** Dùng GET /pods (có riskCount); cột Name, Namespace, Node, Risk Count; click row → Pod Detail (/resources/pods/:id). Hỗ trợ filter theo cluster khi chọn Global Cluster Selector.
* **Dashboard – Pod Detail:** Trang /resources/pods/:id với GET /pods/:id; Overview (namespace, node, serviceAccount, uid, riskCount), link Related.
* **Dashboard – Global Cluster Selector:** Dropdown ở header (Layout); danh sách cluster từ GET /clusters; lưu selectedClusterId (zustand persist); Resources Pod tab dùng selectedClusterId khi gọi GET /pods?cluster=...

**GAP hoàn thiện (tiếp):**

* **Cluster List:** Search by name (client-side), Filter by Health (connection status: all / connected / degraded / disconnected). Toolbar với input search và select health.
* **Dashboard:** Khi chọn cluster ở Global Cluster Selector, mục "Cluster Health" chỉ hiển thị cluster đó; click tên cluster → Cluster Detail (/clusters/:id).
* **Core – GetPods:** Thêm query param `node=` để lọc pod theo node name (cho Node view). Thêm GET /pods/by-uid/:uid (GetPodByUID) cho link Risk Detail → Pod Detail. Thêm GET /serviceaccounts/by-uid/:uid cho link Risk Detail → Identity Detail.
* **Pod Detail:** Hỗ trợ route /resources/pods/uid/:uid (fetch by uid). Tabs: Overview, SBOM (GET /sbom/:podUid), Related Risks (GET /risks/pods/:podUid/report). Hiển thị packages và danh sách risks, click risk → Risk Detail.
* **Risk Detail:** Affected asset click → Pod: /resources/pods/uid/:uid; ServiceAccount: /identities/uid/:uid.
* **Node view:** Trang /clusters/:clusterId/nodes/:nodeName (NodeDetail) – GET /pods?cluster=&node=, bảng workloads; click pod → Pod Detail. Cluster Detail tab Inventory: node name clickable → Node view.
* **Identity Detail:** Trang /identities/:id và /identities/uid/:uid; GET /serviceaccounts/:id hoặc GET /serviceaccounts/by-uid/:uid; GET /serviceaccounts/:id/permissions. Hiển thị Overview + Permissions.
* **Rule Detail:** Trang /rules/:id; GET /rules/:id (rule, matchCount, recentMatches). Hiển thị rule intent/definition, Recent violations (link → Risk Detail). Rules list: click row / View → Rule Detail.

---

## Đã thực hiện (bổ sung – Dashboard filter API, Risk Detail Evidence, Node metadata)

* **Core – Dashboard filter API:** GET /dashboard/stats?clusterId=, GET /insights/summary?clusterId=, GET /risks?clusterId= (khi có clusterId, mọi số liệu/risks chỉ scope theo cluster đó; join insights với pods theo resource_uid = pods.uid và pods.cluster_id). Đồng bộ với Global Cluster Selector và cơ chế refresh.
* **Core – Risk Detail Evidence / Violated Rules:** Migration 058 thêm cột `evidence`, `violated_rules` (JSONB) vào bảng insights. Model Insight có Evidence, ViolatedRules. GET /insights/:id trả về 2 trường này; Risk Detail UI hiển thị block "Evidence & Violated Rules" khi có dữ liệu.
* **Core – Node metadata & Node Detail API:** Migration 059 thêm cột role, os, runtime vào bảng nodes. Model Node có Role, OS, Runtime. GET /clusters/:id/nodes/:nodeName trả metadata (từ bảng nodes nếu có) + podCount; query ?pods=true trả thêm danh sách pods trên node (id, uid, name, namespace, riskCount).
* **Dashboard – API client + refresh theo cluster:** getStats(clusterId?), getInsightsSummary(clusterId?), getRisks({ …, clusterId? }); Dashboard.tsx và Insights.tsx (Risk Center) truyền selectedClusterId; fetchData phụ thuộc selectedClusterId để refetch khi đổi cluster.
* **Dashboard – Node Detail:** getClusterNode(clusterId, nodeName, { pods: true }); trang Node Detail dùng API này, hiển thị Node metadata (role, os, runtime, ip, kubelet) khi có và bảng Workloads (pods). Type NodeDetailResponse và route GET /clusters/:id/nodes/:nodeName đã có.

---

## Tổng kết Spec vs Hiện trạng

| Spec section | Trạng thái |
|--------------|------------|
| 1. Dashboard (Home) | ✅ Cluster selector, summary C/H/M/L + Affected Workloads, Top Risks, filter theo cluster (API + UI), refresh. |
| 2. Cluster List | ✅ Table (Name, Version, Health, Risk Count, Agents), search by name, filter Health, click row → Cluster Detail. |
| 3. Cluster Detail | ✅ Header, tabs Overview / Inventory / Agents / Security Summary; API đủ; click node → Node Detail. |
| 4. Node Detail | ✅ Metadata (role, OS, runtime) khi có từ DB; Workloads (pods); click pod → Pod Detail. (Tab Risks chưa: có thể bổ sung sau.) |
| 5. Workloads / Pod List | ✅ Table (Pod name, namespace, node, risk count), filter cluster, click → Pod Detail. |
| 6. Pod Detail | ✅ Overview, SBOM, Related Risks; link by uid; tab chưa có API ẩn/empty state. |
| 7. Risk Center | ✅ Table, filter severity/status/search, clusterId; click row → Risk Detail. |
| 8. Risk Detail | ✅ Summary, Affected Assets, Evidence, Violated Rules (từ API khi có), Timeline, Mark as resolved. |
| 9–10. Capabilities (PCE) | ✅ Theo spec. |
| 11–12. Identities / Identity Detail | ✅ Theo spec. |
| 13–14. Rules / Rule Detail | ✅ Theo spec. |
| 15. Attack Paths | ✅ Graph API. |
| 16. Monitoring | ✅ Agent health, sync, errors. |
| 17. Global UI | ✅ Global Cluster Selector, Manual Refresh. |

**Definition of Done (Gap):** Các mục trong "Plan bổ sung Backend / DB / Agent" và "GAP hoàn thiện" đã được triển khai. Dữ liệu trace được về agent/core; không tab/button dẫn tới empty page khi API đã có.
