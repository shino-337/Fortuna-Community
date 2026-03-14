# Risk Center – Tổng quan thành phần (dựa trên mã nguồn và tài liệu)

Tài liệu này mô tả Risk Center dựa trên **mã nguồn thực tế** (Core, Dashboard, DB) và các tài liệu risk-center. Mọi thông tin, code mẫu và flow được ghi trực tiếp trong file này, không tham chiếu file ngoài.

---

## 1. Tổng quan Risk Center & Mục tiêu

### Risk Center là gì

Risk Center trong Fortuna là **một trang trong Dashboard** (route `/risks`), cung cấp workspace thống nhất để:
- Xem và quản lý **risk findings** (insights): CVE, RBAC, misconfiguration, capability exposure.
- Xem **Pod Capability Exposure (PCE)**: trend 7 ngày và heatmap namespace × severity.
- Xem **Evidence & References**: runtime signals, capability catalog.

Risk Center **không** phải micro-frontend tách riêng; nó là một page React trong cùng codebase Dashboard, gọi API Core qua `/api/v1/risk/*` và `/api/v1/ws/risks` (WebSocket).

### User persona & KPI

- **Persona:** SecOps, Platform Engineer, Compliance (xem findings, export báo cáo, filter theo severity/priority).
- **KPI thực tế trong code:** Filter theo severity (critical/high/medium/low), status (active/resolved/acknowledged), **priority level (P0–P4)** từ bảng `risk_scores`, sort theo **risk score**; export CSV/PDF; số lượng theo cluster (by-cluster summary). Chưa có metric MTTR hay % resolved trong 24h được tính sẵn trong API.

### Trang riêng hay tab

- Risk Center là **trang riêng**: URL `/#/risks`, component `RiskCenter` (export từ `dashboard/pages/Insights.tsx`).
- Trong Layout, sidebar **Security → Risk Center** trỏ tới path `/risks`.
- Trong `App.tsx`: `<Route path="risks" element={<RiskCenter />} />` và `<Route path="risks/:id" element={<RiskDetail />} />`.

---

## 2. Luồng xử lý dữ liệu End-to-End

### Nguồn tạo risk findings (insights)

1. **Risk engine (RBAC / rule-based):** Worker subscribe NATS subject `fortuna.normalized.>`. Mỗi message chứa resource đã chuẩn hóa (ServiceAccount, Role, RoleBinding, ClusterRole, ClusterRoleBinding, …). RiskWorker gọi `riskEngine.EvaluateResource(ctx, kind, normalizedData)` → so khớp với rules (load từ DB `risk_rules` hoặc YAML). Rule match → tạo `*models.Insight` → `insightMgr.BatchCreateOrUpdateInsights(insights)` → ghi bảng `insights`. Sau đó publish NATS `fortuna.insights.updated`.
2. **CVE matcher:** Worker xử lý SBOM/CVE → tạo insight type `vulnerability` (resource_uid, cve_id, severity, …) → ghi `insights` và cũng publish `fortuna.insights.updated`.
3. **PCE (Pod Capability Exposure):** Không tạo trực tiếp “insight” trong bảng insights; dữ liệu PCE nằm ở bảng `pod_capabilities` (sync từ Agent). Risk Center hiển thị PCE qua API `/risk/pod-capabilities/trends` và `/risk/pod-capabilities/summary/namespace` (trend 7 ngày, heatmap).

### Nguồn tạo PCE exposure

- **Pod capabilities** do Agent đồng bộ (sync) lên Core; lưu bảng `pod_capabilities`. API đọc từ bảng này để trả trend và summary theo namespace/severity. Không có state machine Detected→Confirmed→Exploited trong mã hiện tại; chỉ có mức severity và thống kê theo thời gian.

### Supporting evidence

- **Insights** có cột `evidence` (jsonb) và `violated_rules` (jsonb), do risk engine hoặc CVE pipeline điền khi tạo/update.
- **Runtime signals:** API GET `/risk/runtime/summary`, `/risk/pods/:uid/runtime`, `/risk/pods/:uid/runtime/events` (và các endpoint runtime khác) cung cấp dữ liệu runtime; trang Risk Center tab “Evidence & References” có thể gọi API runtime signals để hiển thị bảng.
- Liên kết finding ↔ resource: insight có `resource_uid`, `resource_type`, `resource_namespace`, `resource_name`; với Pod có thể link sang Pod Detail (Dashboard route `resources/pods/uid/:uid`).

### Thứ tự xử lý (sequence)

```
[Agent / Sync] → pods, pod_capabilities, ... (DB)
[NATS: fortuna.normalized.*] → RiskWorker.Process
    → riskEngine.EvaluateResource(kind, data)
    → []*Insight
    → insightMgr.BatchCreateOrUpdateInsights(insights)
    → INSERT/UPDATE insights (dedup by resource_uid + insight_type + title hoặc cve_id)
    → js.Publish("fortuna.insights.updated", {})
[Main: subscriber fortuna.insights.updated] → BroadcastRisksUpdate()
    → RisksWSHub.Broadcast({"type":"insights_updated"})
[Dashboard: WebSocket /ws/risks] nhận message → refetch getRisks()
[Risk score] Tính riêng: POST /risk/scores/:uid/calculate hoặc job; lưu risk_scores. GET /risk/insights?withScores=1 join risk_scores → trả insight + totalScore, priorityLevel.
```

- Finding được tạo khi worker xử lý message (real-time qua NATS). Score được tính và lưu trong `risk_scores`; khi gọi GET `/risk/insights` với `withScores=1`, Core join insights với risk_scores theo `resource_uid` và trả về thêm `totalScore`, `priorityLevel`. Evidence đã nằm trong bảng insights (cột evidence/violated_rules) hoặc lấy thêm từ API runtime.

### Real-time vs batch

- **Real-time:** NATS message → RiskWorker (và CVE matcher) → ghi insights → publish `fortuna.insights.updated` → Core subscriber gọi `BroadcastRisksUpdate()` → WebSocket `/ws/risks` gửi `{"type":"insights_updated"}` → Dashboard nhận và refetch danh sách risks.
- **Polling:** Dashboard dùng `usePolling` với interval từ store (REFRESH_INTERVALS) để gọi `getRisks`, `getInsightsSummary`, `getThreatVelocity`, v.v. Khi nhận WebSocket `insights_updated`, Dashboard cũng gọi lại `fetchData()` (refetch).

### Deduplication

- **Trong InsightManager:** Vulnerability: dedup theo `(resource_uid, cve_id)`; capability với cve_id: theo `(resource_uid, cve_id, insight_type)`; loại khác: theo `(resource_uid, insight_type, title)`. Có xử lý re-activate insight đã resolved/dismissed khi có bản ghi mới tương ứng. BatchCreateOrUpdateInsights còn dedup in-memory theo key `resource_uid:cve_id:insight_type` trước khi bulk insert/upsert.

---

## 3. Logic kinh doanh & Tính toán

### Risk scoring

- Bảng **risk_scores**: `total_score` (0–100), `base_score`, `severity_weight`, `impact_multiplier`, `time_decay`; phiên bản V2 có thêm `exploitability_score`, `business_impact_score`, `scorer_version`.
- **Priority level:** P0–P4. Trong code: `models.GetPriorityLevel(score)` (V1): ≥90 P0, ≥70 P1, ≥40 P2, còn lại P3; V2 có thêm P4 (0–9). Priority lưu trong `risk_scores.priority_level`.
- GET `/risk/insights?withScores=1` join `risk_scores` theo `resource_uid` và trả `totalScore`, `priorityLevel` kèm từng insight. Công thức chi tiết (CVSS + time decay + …) nằm trong package tính risk score (ví dụ risk calculator), không nằm trong Risk Center overview; PCE không đóng góp % cố định vào một công thức chung trong mã được mô tả ở đây.

### PCE logic

- PCE từ **pod capabilities** sync từ Agent; Core API đọc `pod_capabilities` để trả trend (theo ngày) và summary theo namespace/severity. Không có state machine Detected→Confirmed→Exploited trong mã hiện tại.

### Correlation (finding ↔ evidence)

- Insight gắn resource qua **resource_uid**, **resource_type**, **resource_namespace**, **resource_name**. Evidence lưu trong cột **evidence** (jsonb). Link sang Pod Detail: Dashboard dùng `resource_uid` để tạo link tới `/resources/pods/uid/:uid` (với resource_type Pod).

### Rule engine

- **Risk rules** (dùng cho risk engine): load từ bảng **risk_rules** (ưu tiên) hoặc từ YAML trong `FORTUNA_RULES_DIR/risk/`. Rule gồm id, name, severity, category, conditions (expression hoặc resource), aggregation (AND/OR/THRESHOLD), base_score. User có thể chỉnh rule qua UI: **Settings → Risk Rules** (tab trong Settings): Add/Edit/Delete khi source=db; Import/Export YAML.

### Threat Velocity / Risk Trends

- API **GET `/risk/trends`** hoặc tương đương (risk package) cung cấp dữ liệu trend. Dashboard gọi `api.getThreatVelocity(7, clusterId)` và hiển thị **Risk Trend (7 Days)** (AreaChart). Dữ liệu theo ngày (critical/high/medium/low count). Khi user click một ngày trên chart, có thể filter bảng risks theo ngày đó (selectedChartDate → sinceMinutesForApi từ start of day tới now).

---

## 4. UI/UX – Cấu trúc và trải nghiệm

### Cấu trúc trang Risk Center

- **File:** `dashboard/pages/Insights.tsx` (export component tên `RiskCenter`).
- **Routes:** `/risks` (Overview only), `/risks/findings`, `/risks/pce`, `/risks/evidence`; tab sync theo `location.pathname`.
- **Trang Overview (`/risks`):** Chỉ KPI (Total, scope, time), **Risk Trend (7 Days)** (AreaChart), **Risk Score Distribution** (histogram Recharts, compact), **Risk Level Overview** (4 ô severity clickable → navigate Findings với `?severity=`), **Risks by cluster** (khi all clusters), **Quick Links** (3 card: View All Findings, PCE Heatmap, Evidence & References), nút "View all findings". Không có bảng findings.
- **Trang Findings (`/risks/findings`):** Cùng KPI + Trend + Histogram (full width) + Risk Level Overview + Risks by cluster; thêm hàng filter (Severity, Workflow, Namespace, Type, Search, Priority P0–P4, **Sort** mặc định **Risk score high to low**), bulk actions, bảng findings (cột Level có badge **P0/P1** khi có priority), pagination. Click dòng → mở drawer; URL sync `?insightId=<id>`.
- **Drawer (global):** Mở từ bảng Findings (click row) hoặc từ URL **`?insightId=<id>`** (deep link từ Overview/PCE/Evidence). Đóng drawer → xóa `insightId` khỏi URL. 3 tab: Summary, Evidence & Audit, PCE & Attack Path; bottom bar Acknowledge/Resolve/Dismiss.
- **Tab PCE:** PCE Trend (7 Days) (line chart), Exposure by Namespace (heatmap) khi có dữ liệu.
- **Tab Evidence & References:** Bảng runtime signals (RuntimeSignalsTable).

### Evidence trong Risk Detail

- Drawer/chi tiết risk hiển thị thông tin insight (title, description, severity, status, resource, …). Evidence có thể từ field `evidence` (JSON) của insight; có thể format dạng thẻ hoặc raw tùy component. Có link sang Pod Detail khi resource_type là Pod (dùng resource_uid). Timeline (detected_at, resolved_at) có thể hiển thị từ dữ liệu insight.

### Filtering, pagination, bulk actions

- **Filtering:** Client gửi query params (severity, status, search, clusterId, sinceMinutes, priorityLevel, **scoreBin** 0|10|…|90) tới GET `/risk/insights`. Sort mặc định **score_desc**. Cột Level có badge **P0**/**P1** khi có priorityLevel.
- **Pagination:** Classic: params `page`, `pageSize`; API trả `total`, `page`, `pageSize`, `insights`. Component Pagination trong Insights.tsx.
- **Bulk actions:** POST `/risk/insights/bulk` (acknowledge|resolve|dismiss, insightIds[]). Export CSV/PDF có loading state và tooltip "Max 10,000 rows. Current filters apply."

### Dark mode, accessibility, mobile

- Dashboard dùng theme tối (slate-950, slate-900, …). Không có toggle dark/light trong Risk Center riêng. Accessibility (ARIA) và mobile responsiveness tùy component chung của Dashboard.

### PCE visualization

- Có **line chart** (Recharts AreaChart) cho PCE trend 7 ngày và **heatmap** (bảng namespace × severity) cho Exposure by Namespace; không chỉ bảng đơn giản.

---

## 5. Các cấu phần liên quan & Tích hợp

### Core Go packages

- **riskengine:** `Engine`, `YAMLEngine`, `EvaluateResource`, `createInsight`, `LoadRulesFromDB`, `ReloadFromDB`; `InsightManager` (`BatchCreateOrUpdateInsights`, `CreateOrUpdateInsight`); `validate`, `export` (risk rules YAML).
- **worker:** `RiskWorker` (subscribe `fortuna.normalized.>`, Process → EvaluateResource → BatchCreateOrUpdateInsights, publish `fortuna.insights.updated`); `CVE matcher` worker (cũng publish `fortuna.insights.updated`).
- **models:** `Insight`, `RiskScore`, `RiskRule`, `Pod`, `Cluster`, …
- **api:** `insights_handlers.go` (GetInsights, GetInsight, GetInsightsSummaryCached, GetInsightsSummaryByCluster, GetInsightsSummaryGlobalCached, AcknowledgeInsight, ResolveInsight, DismissInsight, UpdateInsightStatus); `dashboard_handlers.go` (GetInsightsList, GetInsightsListCached, getInsightsListData, ExportRisksCSV, RiskFilter, InsightWithScore); `risks_ws_hub.go` (RisksWSHub, BroadcastRisksUpdate, RisksWS); `risks_cache.go` (MemoryRisksCache, BuildRisksListCacheKey, BuildInsightsSummaryCacheKey); `routes_risk.go` (đăng ký tất cả route `/risk/*`).

### NATS subjects

- **fortuna.normalized.>** — RiskWorker và các worker khác subscribe; message chứa resource đã chuẩn hóa.
- **fortuna.insights.updated** — Publish sau khi tạo/cập nhật insights (RiskWorker, CVE matcher). Core subscriber trong `main.go` nhận và gọi `BroadcastRisksUpdate()`.

### DB tables chính

- **insights:** id, resource_type, resource_namespace, resource_name, resource_uid, insight_type, severity, title, description, recommendation, cve_id, cvss, affected_component, affected_version, fixed_version, evidence (jsonb), violated_rules (jsonb), status, detected_at, resolved_at, created_at, updated_at, deleted_at.
- **risk_scores:** id, resource_uid, resource_type, resource_name, namespace, cluster_id, total_score, base_score, severity_weight, impact_multiplier, time_decay, exploitability_score, business_impact_score, scorer_version, factors (jsonb), insights_count, highest_severity, priority_level, calculated_at, created_at, updated_at, deleted_at.
- **risk_rules:** rule_id, name, category, severity, description, enabled, conditions (text/json), aggregation, base_score, tags (text/json), created_at, updated_at, deleted_at.
- **pod_capabilities:** (sync từ Agent) dùng cho PCE trend và summary.
- **audit_logs:** ghi audit khi acknowledge/resolve/dismiss (và view) insight.

### React components (Dashboard)

- **Risk Center page:** `dashboard/pages/Insights.tsx` (RiskCenter) — tabs risks/pce/reference, fetch getRisks, getInsightsSummary, getInsightsSummaryByCluster, getThreatVelocity, getPceTrend, getPceSummaryByNamespace, exportRisksCSV/PDF, WebSocket subscribe refetch.
- **Risk detail:** `dashboard/pages/RiskDetail.tsx` (route `/risks/:id`).
- **Layout:** `dashboard/components/Layout.tsx` — sidebar Security → Risk Center → `/risks`.
- **Runtime signals table:** `dashboard/components/RuntimeSignalsTable.tsx` (tab Reference).

### Scheduler / Workers

- **InsightsCleanupJob** (scheduler): Chạy mỗi 24h; soft-delete insights đã resolved cũ hơn `INSIGHTS_RESOLVED_RETENTION_DAYS` (mặc định 30), và insights active cũ hơn `INSIGHTS_ACTIVE_RETENTION_DAYS` (mặc định 90). File: `core/internal/scheduler/insights_cleanup_job.go`.
- **RiskWorker:** Trong worker pool; subscribe `fortuna.normalized.>`, Process → EvaluateResource → BatchCreateOrUpdateInsights → Publish `fortuna.insights.updated`.
- **CVE matcher worker:** Tạo insight vulnerability, ghi DB, publish `fortuna.insights.updated`.

### API Risk Center

- Base path: **/api/v1/risk/** (và một số route dashboard/insights).
- Ví dụ: GET `/risk/insights` (list có cache), GET `/risk/insights/summary`, GET `/risk/insights/summary/by-cluster`, GET `/risk/insights/summary/global`, GET `/risk/insights/export` (CSV/PDF), GET `/risk/insights/:id`, POST/DELETE/PATCH cho acknowledge, resolve, dismiss, update status; GET `/risk/rules`, GET `/risk/rules/export`, POST `/risk/rules/validate`, POST `/risk/rules/import`, CRUD `/risk/rules/:id`; GET `/risk/scores`, GET `/risk/scores/:uid`, POST `/risk/scores/:uid/calculate`; GET `/risk/pod-capabilities/trends`, summary; GET `/risk/ws/risks` (WebSocket). Chi tiết đầy đủ trong `core/internal/api/routes_risk.go`.

---

## 6. Cách hệ thống xử lý (Technical Deep Dive)

### Error handling & resilience

- **Finding ingest lỗi:** RiskWorker.Process nếu unmarshal hoặc EvaluateResource lỗi thì log và return error (NATS ack/nak tùy cấu hình). BatchCreateOrUpdateInsights nếu lỗi thì fallback tạo từng insight một; nếu có ít nhất một thành công thì vẫn publish `fortuna.insights.updated`. Không có DLQ riêng cho từng insight trong mã đã xem.
- **PCE stale:** Pod capabilities do Agent sync; không có logic “stale detection” riêng trong Risk Center (nếu agent ngừng sync thì dữ liệu PCE không cập nhật).

### Consistency

- **Eventual:** Insights và risk_scores được ghi bởi workers; WebSocket broadcast để UI refetch. Không đảm bảo strong consistency giữa NATS → DB → UI.
- **Race:** Nhiều worker có thể tạo insight cho cùng resource; InsightManager dedup theo (resource_uid, insight_type, title) hoặc (resource_uid, cve_id) và dùng upsert/update để tránh duplicate. Có thể có race khi hai message cùng lúc nhưng thường một bản ghi thắng (update).

### Performance

- **Cache:** GET `/risk/insights` và GET `/risk/insights/summary` (và global) dùng **MemoryRisksCache** TTL 60s. Key theo clusterId, status, severity, search, priorityLevel, sinceMinutes, page, pageSize, withScores. Cache miss thì query DB và set cache.
- **Query nặng:** getInsightsListData filter theo cluster (subquery pods), severity, status, type, search, sinceMinutes, priorityLevel (subquery risk_scores); count + order + offset/limit; khi withScores=1 thì thêm batch lookup risk_scores theo list resource_uid. Export CSV/PDF limit 10000 dòng.

### Security & privacy

- **Evidence:** Cột evidence (jsonb) có thể chứa nội dung tùy pipeline; trong tài liệu và code chưa thấy masking cụ thể cho process command hay token. API bảo vệ bằng JWT.
- **Audit:** createInsightAuditLog ghi audit khi view, acknowledge, resolve, dismiss insight (action, insight_id, user, IP). File: `insights_handlers.go`.

### Observability

- **Prometheus:** Core có metrics (ví dụ insights created, risk scores); package `core/pkg/metrics`. Endpoint `/metrics` (promhttp). Chưa có metric riêng kiểu `risk_queries_total` hay `correlation_duration` trong mã đã xem.
- **Tracing:** Chưa instrument OpenTelemetry spans cho pipeline risk trong mã đã xem.

---

## 7. Code mẫu và flow thực thi

### (1) RiskWorker.Process — từ NATS đến insights

```go
// core/pkg/worker/risk_worker.go
func (w *RiskWorker) Process(ctx context.Context, msg *nats.Msg) error {
	var normalizedData map[string]interface{}
	if err := json.Unmarshal(msg.Data, &normalizedData); err != nil {
		return fmt.Errorf("failed to unmarshal normalized item: %w", err)
	}
	kind, _ := normalizedData["kind"].(string)
	// ...
	insights, err := w.riskEngine.EvaluateResource(ctx, kind, normalizedData)
	if err != nil {
		return fmt.Errorf("failed to evaluate risks: %w", err)
	}
	if len(insights) > 0 {
		if err := w.insightMgr.BatchCreateOrUpdateInsights(insights); err != nil {
			// fallback individual
		} else {
			if w.js != nil {
				_, _ = w.js.Publish(SubjectInsightsUpdated, []byte("{}"))  // "fortuna.insights.updated"
			}
		}
	}
	return nil
}
```

Subject subscribe: `fortuna.normalized.>`.

### (2) Risk engine EvaluateResource và createInsight

```go
// core/pkg/riskengine/engine.go
func (e *Engine) EvaluateResource(ctx context.Context, resourceType string, resourceData map[string]interface{}) ([]*models.Insight, error) {
	enrichedData := e.enrichResourceData(resourceData)
	e.mu.RLock()
	applicableRules := e.getApplicableRules(resourceType)
	e.mu.RUnlock()
	for _, rule := range applicableRules {
		if !rule.Enabled { continue }
		matched, score, err := e.evaluateRule(ctx, rule, enrichedData)
		// ...
		if matched {
			insight := e.createInsight(rule, resourceType, enrichedData, score)
			insights = append(insights, insight)
		}
	}
	return insights, nil
}

func (e *Engine) createInsight(rule Rule, resourceType string, resourceData map[string]interface{}, score float64) *models.Insight {
	clusterID, _ := resourceData["cluster_id"].(string)
	name, _ := resourceData["name"].(string)
	namespace, _ := resourceData["namespace"].(string)
	uid, _ := resourceData["uid"].(string)
	description := fmt.Sprintf("%s: %s", rule.Name, rule.Description)
	// ...
	return &models.Insight{
		ResourceType: resourceType, ResourceNamespace: namespace, ResourceName: name, ResourceUID: uid,
		InsightType: string(rule.Category), Severity: string(rule.Severity), Title: rule.Name,
		Description: description, Recommendation: recommendedAction, Status: "active", DetectedAt: time.Now(), ...
	}
}
```

### (3) InsightManager.BatchCreateOrUpdateInsights (tóm tắt)

- Dedup theo (resource_uid, cve_id, insight_type) hoặc (resource_uid, insight_type, title) tùy loại insight.
- Với vulnerability: tìm existing theo resource_uid + cve_id; update hoặc re-activate.
- Với capability có cve_id: tương tự.
- Với loại khác: tìm theo resource_uid + insight_type + title; update nếu có.
- Cuối cùng build slice deduplicated, dùng bulk INSERT ... ON CONFLICT DO UPDATE (PostgreSQL) để ghi hàng loạt. File: `core/pkg/riskengine/insight_manager.go`.

### (4) GetInsightsListCached — list risks có cache

```go
// core/internal/api/dashboard_handlers.go
func GetInsightsListCached(db *gorm.DB) gin.HandlerFunc {
	inner := GetInsightsList(db)
	return func(c *gin.Context) {
		if defaultRisksCache == nil { inner(c); return }
		var filter RiskFilter
		_ = c.ShouldBindQuery(&filter)
		// ... normalize filter, page, pageSize
		key := BuildRisksListCacheKey(clusterID, statusFilter, filter.Severity, filter.Search, filter.PriorityLevel, filter.SinceMinutes, page, pageSize, filter.WithScores)
		if b, ok := defaultRisksCache.Get(key); ok {
			c.Data(http.StatusOK, "application/json", b)
			return
		}
		result, err := getInsightsListData(db, filter, page, pageSize)
		// ...
		defaultRisksCache.Set(key, b, risksCacheTTL)
		c.Data(http.StatusOK, "application/json", b)
	}
}
```

getInsightsListData: query insights với filter (cluster, severity, status, type, search, sinceMinutes, priorityLevel); count; order detected_at DESC, offset/limit. Nếu withScores=1: lấy list resource_uid → query risk_scores WHERE resource_uid IN (...) → map uid → RiskScore → gắn TotalScore, PriorityLevel vào từng insight.

### (5) BroadcastRisksUpdate và WebSocket

```go
// core/internal/api/risks_ws_hub.go
func BroadcastRisksUpdate() {
	msg, _ := json.Marshal(map[string]string{"type": "insights_updated"})
	defaultRisksHub.Broadcast(msg)
}
```

Trong main: subscriber `fortuna.insights.updated` gọi `BroadcastRisksUpdate()`. Dashboard kết nối WebSocket tới `/api/v1/ws/risks`; khi nhận message `type: insights_updated` thì gọi lại fetchData() (getRisks, getInsightsSummary, ...).

### (6) Dashboard Risk Center fetch và tabs

```tsx
// dashboard/pages/Insights.tsx (trích)
const fetchData = useCallback(async () => {
  const risksPromise = api.getRisks({
    page: risksPage, pageSize: risksPageSize,
    severity: filter !== 'all' ? filter : undefined, status: statusFilter,
    search: searchTerm.trim() || undefined, clusterId, sinceMinutes,
    withScores: useScores ? 1 : undefined, priorityLevel: priorityLevel || undefined,
  });
  const summaryPromise = api.getInsightsSummary(clusterId ?? undefined, sinceMinutes);
  const threatPromise = api.getThreatVelocity(7, clusterId ?? undefined).catch(() => []);
  // ... setRisks, setInsightsSummary, setThreatVelocity, setRisksByCluster, setPceTrend, setPceHeatmap
}, [/* deps */]);

useEffect(() => { fetchData(); }, [fetchData]);
// WebSocket: getRisksWsUrl(), subscribe message type 'insights_updated' → fetchData()
```

Tab: `activeTab === 'risks'` → Risk Findings (severity bar, trend chart, risks by cluster, filters, bảng, Export CSV/PDF). `activeTab === 'pce'` → PCE trend + heatmap. `activeTab === 'reference'` → RuntimeSignalsTable.

---

## 8. Schema DB chính (tóm tắt)

**insights:** id, resource_type, resource_namespace, resource_name, resource_uid, insight_type, severity, title, description, recommendation, cve_id, cvss, affected_component, affected_version, fixed_version, evidence (jsonb), violated_rules (jsonb), status, detected_at, resolved_at, created_at, updated_at, deleted_at.

**risk_scores:** id, resource_type, resource_uid, resource_name, namespace, cluster_id, total_score, base_score, severity_weight, impact_multiplier, time_decay, exploitability_score, business_impact_score, scorer_version, factors (jsonb), insights_count, highest_severity, priority_level (P0–P4), calculated_at, created_at, updated_at, deleted_at.

**risk_rules:** id, rule_id, name, category, severity, description, enabled, conditions (text), aggregation, base_score, tags (text), created_at, updated_at, deleted_at.

---

## 9. Flow tổng hợp (text diagram)

```
[Agent] ── sync pods, pod_capabilities ──► DB (pods, pod_capabilities)

[NATS: fortuna.normalized.>] ──► [RiskWorker] ──► riskEngine.EvaluateResource
       │                                              │
       │                                              ▼
       │                                    []*Insight (createInsight per rule match)
       │                                              │
       │                                              ▼
       │                                    InsightManager.BatchCreateOrUpdateInsights
       │                                              │
       │                                              ▼
       │                                    INSERT/UPDATE insights (dedup)
       │                                              │
       │                                              ▼
       │                                    js.Publish("fortuna.insights.updated")
       │                                              │
       ▼                                              ▼
[Main: sub fortuna.insights.updated] ──► BroadcastRisksUpdate() ──► RisksWSHub.Broadcast
                                                                          │
[Dashboard] ◄── WebSocket /ws/risks ◄── {"type":"insights_updated"} ◄───────┘
     │
     │  refetch + polling: getRisks(), getInsightsSummary(), getThreatVelocity(), ...
     │  GET /risk/insights?clusterId=&withScores=1&priorityLevel=... (cache 60s)
     ▼
[UI: Risk Findings tab] — table, filters, Export CSV/PDF, Risk Score column, Risks by cluster
[UI: PCE tab] — getPceTrend(7), getPceSummaryByNamespace() → trend chart + heatmap
[UI: Evidence tab] — runtime signals table
```

---

## 10. As-built vs design (Phase 2 & 3)

- **Overview vs Findings:** `/risks` chỉ hiển thị KPI, Trend, Histogram, Risk Level Overview, Risks by cluster, Quick Links và CTA "View all findings". Bảng findings chỉ ở `/risks/findings`. Severity cards trên Overview click → navigate `/risks/findings?severity=...`.
- **Quick Links:** Block 3 card (View All Findings, PCE Heatmap, Evidence & References) trên Overview; mỗi card navigate tới route tương ứng.
- **Drawer global:** URL `?insightId=<id>` mở drawer từ bất kỳ tab nào; đóng drawer xóa tham số. Khi mở từ Findings (click row) URL được sync; khi load trang với `?insightId=` drawer mở và fetch insight nếu chưa có trong list.
- **Priority/score UX:** Sort mặc định Risk score high to low; cột Level có badge P0/P1; filter Priority P0–P4.
- **Export UX:** Nút Export CSV/PDF có loading spinner khi đang tải; tooltip "Max 10,000 rows. Current filters apply."
- **Phase 3 backend (đã triển khai):** Evidence masking (`evidence.MaskSensitiveInJSON`), export CSV streaming (chunk 500), PCE `last_seen_at` + PCECleanupJob, Prometheus `RiskEvaluationDuration`/`InsightsBatchSize`, rule versioning (`risk_rules_history`), WS rate limit (`RISKS_WS_MAX_CONNS_PER_IP`), GET `/risk/histogram`. Env: `PCE_CLEANUP_RETENTION_DAYS`, `RISKS_WS_MAX_CONNS_PER_IP` (xem Core README và Risk-Center-Verify-And-Deploy-Checklist).

### Công việc chưa hoàn thành & luồng chưa có dữ liệu

- Chi tiết đầy đủ: **[Risk-Center-Gaps-And-Data-Flows.md](./Risk-Center-Gaps-And-Data-Flows.md)**. Plan triển khai chi tiết: **[Risk-Center-Implementation-Plan-Detailed.md](./Risk-Center-Implementation-Plan-Detailed.md)**.
- **Backlog / Coming Soon (phụ thuộc hạ tầng):** Grafana dashboard, Prometheus alerting, Redis (cache/WS) → Backlog; tính năng liên quan (Risk metrics dashboard trong app, sidebar 280px, Risk Score vs Capability, MITRE ATT&CK) → Coming Soon. Các mục này không triển khai khi chưa có hạ tầng.
- **Tóm tắt triển khai được ngay:** (1) Data scope: Risk Trend + stats byType=all (Phase 1). (2) Ops doc env (Phase 2). (3) UI: PCE table + heatmap click, Evidence tab Audit Trail, Export Heatmap, [REDACTED] (Phase 3–6). (4) Test: E2E flow (Phase 7). (5) **Luồng dữ liệu:** Risk Trend và Dashboard stats hiện chỉ `insight_type = 'vulnerability'` → Phase 1 mở rộng byType=all; PCE/Runtime/Histogram/Audit phụ thuộc nguồn dữ liệu tương ứng.

---

Tài liệu này phản ánh đúng mã nguồn và luồng thực thi Risk Center tại thời điểm rà soát; khi code thay đổi cần cập nhật lại cho khớp.
