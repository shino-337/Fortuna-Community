# Risk Center – Phân tích Findings, Đối chiếu Thực tế & Phương án Cải tiến

Tài liệu này đối chiếu **10 findings** trong `Risk-Center-Component-Implement.md` với **mã nguồn và hiện trạng**, điều chỉnh đề xuất khi cần, và đưa ra **phương án triển khai** theo phase để bắt đầu cải tiến.

---

## 1. Bảng đối chiếu Findings vs Thực tế

| # | Finding | Đối chiếu mã nguồn | Kết luận | Ghi chú |
|---|--------|--------------------|----------|---------|
| **1** | Thiếu real-time push, chỉ polling | Không có WebSocket/SSE cho risks. Dashboard dùng `usePolling` + `getRisks`/`getInsightsSummary`. **Đã có** WebSocket cho Pod Detail: `core/internal/api/pod_detail_ws_hub.go` (gorilla/websocket, hub broadcast). | **Hợp lệ** | Có thể tái sử dụng pattern Pod Detail WS (hub + broadcast khi có event). |
| **2** | Risk scoring không unified (insights vs risk_scores) | `Insight` model không có `total_score`/`priority_level` (`implementation_guide.go`). Bảng `risk_scores` riêng, API `/risk/scores`. Engine tạo insight với severity; scorer.go tính risk_scores. | **Hợp lệ** | UI Risk Center sort by score đang dùng `insight.score` (có thể map từ cvss/severity) – cần xác nhận nguồn. Thống nhất cần refactor schema hoặc join. |
| **3** | Không cache cho risk queries | Handlers `GetInsightsList`, `GetInsightsSummary` query GORM trực tiếp. Config có `RedisURL` nhưng **không** dùng Redis cho risks/insights. Có cache in-memory ở CVE manager, CEL, correlator – không áp dụng cho list/summary. | **Hợp lệ** | Redis đã có trong config; thiếu layer cache cho read APIs. |
| **4** | Thiếu export/Reporting và SIEM | Không có endpoint `/risks/export`, không webhook SIEM. | **Hợp lệ** | Triển khai từ đầu. |
| **5** | Retention không configurable | `insights_cleanup_job.go`: `30*24*time.Hour` và `90*24*time.Hour` hard-coded. Không đọc env. | **Hợp lệ** | Chỉ cần thêm env và đọc trong job. |
| **6** | Observability: thiếu tracing, alerting | Metrics: `fortuna_insights_created_total`, `fortuna_insights_active_total` (metrics.go). Không OpenTelemetry spans cho pipeline; không Prometheus alert rules trong repo. | **Hợp lệ** | Có thể thêm spans từng bước; alert rules thường nằm ngoài repo (Helm/values). |
| **7** | Thiếu audit logs cho user actions Risk Center | Bảng `audit_logs` và model `AuditLog` tồn tại; dùng cho Agent sync (SA, roles, …) và một số API (handlers.go, bulk_operations, remediation). **AcknowledgeInsight, ResolveInsight, DismissInsight, GetInsight (view)** **không** ghi audit. | **Hợp lệ (một phần)** | Hạ tầng audit có sẵn; chỉ cần gọi audit khi thao tác insight (action + resource=insight, resourceId=id). |
| **8** | Multi-cluster aggregation / global view | `GetInsightsSummary` khi **không** truyền `clusterId` đã trả về tổng toàn cục (all clusters). Không có endpoint riêng `/risks/global-summary` hay aggregate **theo từng cluster** (GROUP BY cluster_id) trong một response. | **Hợp lệ (một phần)** | Global count đã có (no clusterId). Thiếu: summary **by cluster** (danh sách cluster + count từng cluster) và UI tab "Global Risks" rõ ràng. |
| **9** | PCE thiếu heatmaps, trends | Tab PCE: summary severity + bảng drill-down (Insights.tsx). Threat Velocity chart cho **risks** (7 ngày); **không** có chart cho PCE theo thời gian hay heatmap capability vs namespace. | **Hợp lệ** | Cần API `/pod-capabilities/trends` (theo ngày) và component Recharts cho PCE. |
| **10** | Rules không configurable qua UI | Rules load từ YAML (FORTUNA_RULES_DIR), engine.go; không CRUD API, không bảng `risk_rules`, không UI admin. | **Hợp lệ** | Refactor lớn nếu đưa rules vào DB + UI. |

---

## 2. Điều chỉnh Đề xuất so với Bản gốc

- **#1 (Real-time):** Giữ đề xuất WebSocket; **tái sử dụng** pattern hiện có: hub tương tự `PodDetailWSHub`, endpoint `/api/v1/ws/risks`, subscribe NATS `fortuna.insights.created` (hoặc subject tương đương) và broadcast delta (new/updated insight ids hoặc summary) cho mọi client đang mở Risk Center. Fallback: giữ polling khi WS fail.
- **#2 (Unified scoring):** Hai hướng (chọn một hoặc làm từng bước):
  - **Option A (ít xâm lấn):** Giữ hai nguồn; UI Risk Center thêm filter/sort "by risk score" bằng cách gọi `/risk/scores` và join/merge theo resource_uid trên frontend (hoặc API mới trả list insights kèm score từ risk_scores). Không đổi schema insights.
  - **Option B (unified):** Thêm cột `total_score`, `priority_level` vào `insights`; khi BatchCreateOrUpdateInsights (và khi tạo insight từ CVE/RBAC) gọi scorer MVP2 và ghi vào insight. Migration + cập nhật engine/insight_manager.
- **#7 (Audit):** Không cần bảng mới. Trong `AcknowledgeInsight`, `ResolveInsight`, `DismissInsight` (và tùy chọn khi `GetInsight` – "view"), lấy user từ Gin context (JWT middleware đã set user_id/username) và ghi `AuditLog{ Resource: "insight", ResourceID: id, Action: "acknowledge"|"resolve"|"dismiss"|"view" }`. API GET /audit-logs đã có, chỉ cần filter theo resource=insight.
- **#8 (Global):** Không bắt buộc endpoint tên "global-summary" nếu GET /insights/summary không clusterId đã đủ cho "all clusters". Cải tiến hữu ích: API **summary by cluster** (ví dụ GET /insights/summary/by-cluster hoặc query param `groupBy=cluster`) trả về `[{ clusterId, total, critical, high, ... }]` để UI vẽ "Global Risks" / dropdown theo cluster.

---

## 3. Phương án Triển khai theo Phase

### Phase 1 – Quick wins (ưu tiên cao, ít phụ thuộc)

Mục tiêu: cải thiện vận hành và compliance với ít thay đổi kiến trúc.

| Thứ tự | Công việc | Finding | Việc cụ thể | Ước lượng |
|--------|-----------|---------|-------------|-----------|
| 1.1 | Retention configurable | #5 | Thêm env `INSIGHTS_RESOLVED_RETENTION_DAYS` (default 30), `INSIGHTS_ACTIVE_RETENTION_DAYS` (default 90). Trong `InsightsCleanupJob.run()` đọc env, dùng biến thay vì literal. | 0.5–1 ngày |
| 1.2 | Audit log cho Risk Center actions | #7 | Middleware hoặc inline trong handler: khi gọi AcknowledgeInsight, ResolveInsight, DismissInsight (và tùy chọn GetInsight view), insert vào `audit_logs` (resource=insight, resourceId, action, user từ context). Đảm bảo JWT middleware đã set user. | 1–2 ngày |
| 1.3 | Export risks CSV | #4 | Endpoint GET `/api/v1/risks/export?format=csv&clusterId=...&sinceMinutes=...` (và các filter giống GetInsightsList). Dùng thư viện CSV (encoding/csv hoặc gocsv), stream response. Nút "Export CSV" trên Risk Center gọi endpoint. | 1–2 ngày |

**Kết quả Phase 1:** Retention linh hoạt, audit trail cho thao tác insight, export CSV cơ bản – có thể bắt đầu ngay.

---

### Phase 2 – Performance & UX (phụ thuộc hạ tầng nhẹ)

| Thứ tự | Công việc | Finding | Việc cụ thể | Ước lượng |
|--------|-----------|---------|-------------|-----------|
| 2.1 | Cache read APIs | #3 | Layer cache (Redis nếu REDIS_URL có; fallback in-memory TTL 60s) cho GET /risks và GET /insights/summary. Key: `risks:list:{clusterId}:{sinceMinutes}:{page}:{pageSize}:{status}:{severity}` và `insights:summary:{clusterId}:{sinceMinutes}`. Invalidate khi BatchCreateOrUpdateInsights (hoặc TTL ngắn 30–60s). | 2–3 ngày |
| 2.2 | Global / by-cluster summary | #8 | Thêm GET `/api/v1/insights/summary/by-cluster?sinceMinutes=...` trả về `[{ clusterId, clusterName?, total, critical, high, medium, low }]`. UI: tùy chọn dropdown "All clusters" vs từng cluster; có thể thêm block "Risks by cluster" dùng API này. | 1–2 ngày |
| 2.3 | Real-time push (WebSocket) | #1 | Core: endpoint GET `/api/v1/ws/risks` (hoặc `/ws/risks`), hub tương tự Pod Detail. Khi CVE matcher / risk worker gọi BatchCreateOrUpdateInsights thành công, publish event nội bộ (hoặc subscribe NATS nếu đã có subject) và hub broadcast message (ví dụ `{ "type": "insights_updated", "count": N }` hoặc danh sách id). Dashboard: Risk Center mở WS; khi nhận message, trigger refetch (getRisks/getInsightsSummary) hoặc merge delta. Giữ polling làm fallback. | 3–5 ngày |

**Kết quả Phase 2:** Giảm tải DB, global view rõ ràng, cập nhật gần real-time.

---

### Phase 3 – Unified scoring & Nâng cao tính năng

| Thứ tự | Công việc | Finding | Việc cụ thể | Ước lượng |
|--------|-----------|---------|-------------|-----------|
| 3.1 | Unified risk score (Option A nhẹ) | #2 | API mới GET `/api/v1/risks/with-scores?clusterId=...&page=...` (hoặc mở rộng GET /risks) trả về từng insight kèm `total_score`, `priority_level` từ bảng `risk_scores` (join resource_uid). UI: sort/filter by score và priority. Không đổi schema insights. | 2–3 ngày |
| 3.2 | Observability | #6 | Thêm OpenTelemetry spans cho EvaluateResource (engine), BatchCreateOrUpdateInsights (insight_manager), và CVE matcher worker. Thêm file Prometheus alert rules mẫu (ví dụ critical insight tăng đột biến) trong repo hoặc trong chart. | 2–3 ngày |
| 3.3 | Export PDF / SIEM hook | #4 | Export PDF (wkhtmltopdf hoặc lib Go); endpoint `/risks/export?format=pdf`. SIEM: khi tạo/update insight severity critical/high, publish message NATS `fortuna.siem.events` (payload JSON); adapter riêng subscribe và gọi webhook. | 2–4 ngày |

**Kết quả Phase 3:** Score thống nhất trên UI, dễ debug và alert, export đa dạng và tích hợp SIEM.

---

### Phase 4 – Tùy chọn (Lower priority)

| Công việc | Finding | Ghi chú |
|-----------|--------|--------|
| PCE trends + heatmap | #9 | API `/pod-capabilities/trends`, UI Recharts (line + heatmap). |
| Risk rules CRUD + UI | #10 | DB table risk_rules, API CRUD, engine load từ DB; UI admin. Refactor lớn. |
| Unified scoring Option B | #2 | Migration thêm cột insights.total_score, priority_level; engine/scorer ghi khi tạo insight. |

---

## 4. Thứ tự Bắt đầu Đề xuất

1. **Bắt đầu bằng Phase 1** (retention + audit + export CSV) – không phụ thuộc Redis/WS, mang lại giá trị compliance và vận hành ngay.
2. Sau đó **Phase 2.1 (cache)** nếu đã có Redis hoặc chấp nhận in-memory TTL ngắn.
3. Tiếp theo **Phase 2.2 (by-cluster summary)** và **Phase 1.2 (audit)** nếu chưa làm – audit rất quan trọng cho compliance.
4. **Phase 2.3 (WebSocket)** khi đã ổn định cache và muốn giảm latency cảm nhận.

---

## 5. Checklist Phase 1 (Đã hoàn thành)

- [x] **1.1 Retention:** `core/internal/scheduler/insights_cleanup_job.go` – đọc env `INSIGHTS_RESOLVED_RETENTION_DAYS` (default 30), `INSIGHTS_ACTIVE_RETENTION_DAYS` (default 90). Unit test: `insights_cleanup_job_test.go` (TestGetResolvedRetentionDays, TestGetActiveRetentionDays).
- [x] **1.2 Audit:** Helper `createInsightAuditLog(db, c, action, insightID, details)` trong `insights_handlers.go`; gọi trong AcknowledgeInsight, ResolveInsight, DismissInsight. Unit test: `insights_audit_test.go` (TestCreateInsightAuditLog_NoUserInContext, TestCreateInsightAuditLog_WithUserInContext).
- [x] **1.3 Export CSV:** Route GET `/api/v1/risks/export` (trước `/risks/:riskId`), handler `ExportRisksCSV` trong `dashboard_handlers.go`; limit 10k rows; Dashboard nút "Export CSV" trong Risk Center (tab risks). Unit test: `risks_export_test.go` (TestExportRisksCSV_Empty, TestExportRisksCSV_WithInsights).

Khi hoàn thành Phase 1, cập nhật lại tài liệu này (đánh dấu done) và chuyển sang Phase 2 theo thứ tự trên.

---

## 6. Đồng bộ Core / Agent / DB / Dashboard & Cleanup (sau mỗi Phase)

### 6.1 Phase 1 – Thành phần chịu ảnh hưởng

| Thành phần | Thay đổi Phase 1 | Đồng bộ / Ghi chú |
|------------|-------------------|-------------------|
| **Core** | Retention (env), audit (insight actions), export CSV | Không đổi contract với Agent. API mới: GET /risks/export. Audit ghi vào bảng `audit_logs` có sẵn. |
| **Agent** | Không thay đổi | Agent không gửi insights trực tiếp; CVE matcher / risk worker trong Core tạo insights. Không cần đổi Agent. |
| **Database** | Không migration mới | Bảng `audit_logs` đã có; `insights` không đổi schema. Retention chỉ đổi logic xóa (soft delete). |
| **Dashboard** | Nút Export CSV, gọi api.exportRisksCSV() | API base `/api/v1`; query params giống getRisks (clusterId, sinceMinutes, status, severity, search). |

### 6.2 Kiểm tra đồng bộ sau Phase 1

- **Core ↔ DB:** InsightsCleanupJob đọc env mỗi lần `run()`; audit log insert vào `audit_logs` (cột resource='insight'). Export query cùng filter với GetInsightsList (cluster, pod deleted_at, severity, status, search, sinceMinutes).
- **Dashboard ↔ Core:** `exportRisksCSV` gọi GET `/api/v1/risks/export?...` với token; Core trả CSV với Content-Disposition attachment. Cùng scope (cluster, time, status, severity) như list.
- **Audit:** GET /api/v1/audit-logs có thể filter theo `resource=insight` để xem thao tác Risk Center (acknowledge/resolve/dismiss).

### 6.3 Cleanup – Dữ liệu / Cache / Outdated

- **Unit test:** Test data trong `*_test.go` dùng in-memory SQLite hoặc test DB; không để lại dữ liệu thật. Sau khi chạy test, không cần xóa DB (test DB riêng hoặc :memory:).
- **Cached / outdated:** Risk Center không dùng Redis cache (sẽ thêm ở Phase 2). Nếu có script hoặc job cũ dùng literal 30/90 ngày, nên chuyển sang đọc env hoặc xóa script cũ.
- **Docs:** Cập nhật README hoặc deployment doc với env `INSIGHTS_RESOLVED_RETENTION_DAYS`, `INSIGHTS_ACTIVE_RETENTION_DAYS` (ví dụ trong `docs/03-components/risk-center/` hoặc env.example).

---

## 7. Checklist Phase 2 (Đã hoàn thành)

- [x] **2.1 Cache:** In-memory TTL 60s trong `core/internal/api/risks_cache.go` (MemoryRisksCache). GetInsightsListCached và GetInsightsSummaryCached dùng defaultRisksCache; khởi tạo trong SetupRoutesWithCertManager. Unit test: `risks_cache_test.go` (TestMemoryRisksCache_GetSet, TestBuildRisksListCacheKey, TestBuildInsightsSummaryCacheKey).
- [x] **2.2 By-cluster summary:** GET `/api/v1/insights/summary/by-cluster?sinceMinutes=...` trong `insights_handlers.go` (GetInsightsSummaryByCluster); trả về `{ byCluster: [{ clusterId, clusterName?, total, critical, high, medium, low }] }`. Dashboard: `api.getInsightsSummaryByCluster()`, state `risksByCluster`; khi scope = all clusters hiển thị bảng "Risks by cluster" (click cluster → setSelectedClusterId).
- [x] **2.3 WebSocket:** Hub và handler trong `risks_ws_hub.go`; route GET `/api/v1/ws/risks`. `BroadcastRisksUpdate()` để push khi có cập nhật (sẽ nối từ NATS subscriber trong main khi workers publish `fortuna.insights.updated`). Dashboard: connect tới `api.getRisksWsUrl()`, on message `type === 'insights_updated'` gọi refetch.

### 7.1 Phase 2 – Đồng bộ

- **Core:** Cache chỉ đọc; by-cluster dùng cùng bảng insights/pods/clusters. WebSocket không đổi contract Agent.
- **Agent:** Không thay đổi. (Broadcast khi tạo insight: có thể thêm publish NATS từ workers + subscriber trong main gọi `api.BroadcastRisksUpdate()` sau.)
- **Dashboard:** Gọi GET /insights/summary/by-cluster khi không chọn cluster; mở WS /ws/risks và refetch khi nhận insights_updated.

---

## 8. Checklist Phase 3 (Đã hoàn thành)

- [x] **3.1 Unified risk score (Option A):** Mở rộng GET `/api/v1/risks` với query `withScores=1` và `priorityLevel` (P0–P4). Left join bảng `risk_scores` theo `resource_uid`; trả về `totalScore`, `priorityLevel` trong từng insight. Cache key bao gồm withScores và priorityLevel. Dashboard: sort theo Risk score, filter Priority (P0–P4), cột Risk Score hiển thị score + priority. Unit test: `risks_list_with_scores_test.go` (TestGetInsightsList_WithScores_Empty, TestGetInsightsList_WithScores_IncludesScore, TestGetInsightsList_WithScores_NoMatchingScore).
- [x] **3.2 Observability:** File Prometheus alert rules mẫu `deploy/prometheus/risk-center.alerts.yaml` (critical insights cao, tốc độ tạo insight). OpenTelemetry spans tạm hoãn.
- [x] **3.3 Export PDF / SIEM:** GET `/api/v1/risks/export?format=pdf` trả về HTML tối ưu in (Print → Save as PDF); nút "Export PDF" trên Risk Center. SIEM: stream NATS `fortuna-siem`, subject `fortuna.siem.events`; CVE matcher và Risk worker gọi `PublishSIEMEvents(js, insights)` cho insight critical/high; helper `core/pkg/worker/siem.go`.

### 8.1 Phase 3 – Đồng bộ

- **Core:** GET /risks với withScores/priorityLevel; export format=pdf (HTML); stream fortuna.siem.events. Không đổi contract Agent.
- **Dashboard:** getRisks(withScores, priorityLevel), exportRisksPDF(), sort/filter by score và priority.

---

## 9. Checklist Phase 4 (Đã hoàn thành)

- [x] **PCE trends + heatmap:** API `/pod-capabilities/trends` và `/pod-capabilities/summary/namespace` đã có. Tab PCE (Risk Center): line chart "PCE Trend (7 Days)", heatmap "Exposure by Namespace" (namespace × severity). Dashboard: getPceTrend(), getPceSummaryByNamespace({ clusterId }).
- [x] **Risk rules read-only:** GET `/api/v1/risk-rules` đọc từ `FORTUNA_RULES_DIR` (YAML), trả về `{ rules: RiskRuleSummary[], total }`. `riskengine.ListRuleSummariesFromDir`, handler `GetRiskRulesList()`. Unit test: `risk_rules_test.go` (TestGetRiskRulesList_EmptyDir, TestGetRiskRulesList_WithYAML).
- **Risk rules CRUD + UI (chưa làm):** DB table risk_rules, API CRUD, engine load từ DB, UI admin – refactor lớn, để phase sau.
- **Unified scoring Option B (chưa làm):** Migration thêm cột insights; không triển khai.

### 9.1 Phase 4 – Đồng bộ

- **Core:** GET /risk-rules không cần DB; đọc file từ env. PCE APIs không đổi.

---

*Tài liệu đối chiếu với mã nguồn tại thời điểm phân tích; khi triển khai nên cập nhật theo branch hiện tại. Phase 1–4 đã triển khai; test bổ sung: risks_list_with_scores_test.go, risk_rules_test.go.*
