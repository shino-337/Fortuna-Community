Fortuna – Risk Center Component Review
Cảm ơn bạn đã cung cấp tài liệu chi tiết qua "Risk-Center-Component-Review-Answer.md". Dựa trên thông tin này (bao gồm mô tả chức năng, luồng dữ liệu, UI, data model, processing logic, và các khía cạnh khác), tôi đã có đủ cơ sở để thực hiện đánh giá toàn diện cho Risk Center – một phần unified workspace trong Dashboard để quản lý risk findings, PCE (Pod Capability Engine), và supporting evidence.
Tôi sẽ tập trung vào các lĩnh vực chính: System Integration, UI Functionality, Data Model, Processing Logic, Performance/Scalability, Reliability/HA, Security, và Observability. Phân tích dựa trên best practices cho Kubernetes security platforms (ví dụ: so sánh với Sysdig Secure hoặc Prisma Cloud), ưu tiên tính bền vững, và chỉ ra các vấn đề cụ thể từ dữ liệu bạn cung cấp.
Tôi sẽ liệt kê các finding theo thứ tự ưu tiên (từ critical đến low), với tổng cộng 10 findings chính. Mỗi finding bao gồm description, technical root cause, impact, risk level, và recommendation cụ thể (có thể bao gồm refactor lớn nếu cần thiết).
Finding #1: Thiếu Real-Time Push Updates cho Risk Center, Chỉ Dựa Polling API
Description
Risk Center sử dụng polling interval từ Dashboard để fetch data (insights, PCE, evidence) qua REST API, không có mechanism push real-time (e.g., websocket) từ Core khi có new findings hoặc updates.
Technical Root Cause
Luồng dữ liệu là client-initiated (React gọi GET /risks, /insights/summary, etc. theo refreshIntervalStore); Core không implement server-sent events hoặc websocket để push từ NATS (e.g., fortuna-insights.created).
Impact

User phải chờ refresh để thấy new risks/CVE → delay detection critical findings (e.g., new CVE exploit) lên đến interval (có thể vài phút), giảm hiệu quả SecOps.
Tăng load API không cần thiết ở multi-user environments.

Risk Level
High
Recommendation

Refactor Dashboard để thêm websocket endpoint ở Core (/ws/risks) – subscribe NATS fortuna-insights.created và push delta updates (new/updated insights) đến connected clients.
Sử dụng library như gorilla/websocket ở Core; React dùng WebSocket API để listen và update state (e.g., append to risks list).
Để backward compatible, giữ polling làm fallback nếu WS fail.

Finding #2: Risk Scoring Không Unified Giữa Insights và Risk_Scores, Dẫn Đến Inconsistency
Description
Risk Center chủ yếu dùng insights (severity từ CVE/rule) cho list/display, nhưng có bảng risk_scores riêng (với algorithm MVP2: base_score, time_decay, etc.) và API /risk/scores – không integrate rõ ràng, có thể gây confusion giữa severity-based và score-based prioritization.
Technical Root Cause
Insights và risk_scores là hai model riêng (insights thiếu total_score; risk_scores có priority_level nhưng không link trực tiếp với insights.evidence); engine.go chỉ tạo insights, scorer.go cho risk_scores – không có unified aggregator.
Impact

User thấy discrepancy giữa UI (severity bar từ insights) và analytics (scores từ risk_scores) → khó prioritize risks chính xác.
Thiếu correlation full (e.g., CVE severity không factor vào risk_scores).

Risk Level
High
Recommendation

Merge model: Thêm fields từ risk_scores vào insights (e.g., total_score, priority_level) qua refactor InsightManager.BatchCreateOrUpdateInsights – tính score khi create insight.
Update engine để apply scorer MVP2 cho tất cả insight types (CVE, capability, RBAC).
UI: Thêm sort/filter by total_score trong RiskCenter.tsx.

Finding #3: Không Có Caching cho Risk Queries, Dẫn Đến DB Load Cao
Description
Tất cả API như /risks, /insights/summary, /pod-capabilities hit DB trực tiếp (PostgreSQL) mà không cache, dù queries phức tạp (joins với pods, filters by cluster/time).
Technical Root Cause
Core handlers (dashboard_handlers.go, insights_handlers.go) dùng GORM direct query (Where, Order, Limit) mà không layer cache (in-memory hoặc Redis).
Impact

Với large datasets (1000+ insights/cluster), latency >500ms per request → UI slow ở multi-user hoặc frequent refresh.
DB overload ở high-traffic (e.g., 50+ SecOps users).

Risk Level
Medium-High
Recommendation

Thêm Redis cache cho read endpoints (key: "insights:list:{clusterId}:{sinceMinutes}:{page}"), TTL 60s.
Invalidate cache khi BatchCreateOrUpdateInsights (publish NATS event để clear keys).
Sử dụng go-redis; benchmark trước/sau để ensure <100ms p99.

Finding #4: Thiếu Export/Reporting và SIEM Integration cho Risks
Description
Risk Center không hỗ trợ export findings (CSV/PDF) hoặc integrate với external SIEM (e.g., Splunk, ELK) – chỉ view trên UI/API.
Technical Root Cause
Không có endpoints hoặc logic cho export (e.g., /risks/export); không webhook hoặc publisher cho SIEM.
Impact

Khó audit/compliance reporting → không meet enterprise needs (e.g., SOC2 export trails).
SecOps phải manual copy data → error-prone.

Risk Level
Medium
Recommendation

Thêm API /api/v1/risks/export?format=csv/pdf – dùng library như gocsv hoặc wkhtmltopdf để generate.
Cho SIEM: Publish high-severity insights đến NATS subject "fortuna.siem.events" → webhook adapter đến external systems.
UI button "Export" trong RiskCenter.tsx gọi endpoint.

Finding #5: Retention Policy Không Configurable, Có Thể Gây Data Bloat
Description
InsightsCleanupJob hard-coded retention (resolved 30 ngày, active 90 ngày) – không configurable qua env hoặc UI.
Technical Root Cause
Logic trong insights_cleanup_job.go fixed (e.g., 3024time.Hour), không env vars như INSIGHTS_RETENTION_DAYS.
Impact

Ở large clusters, DB bloat từ old insights → slow queries, high storage costs.
Không flexible cho compliance (e.g., giữ 1 năm cho audit).

Risk Level
Medium
Recommendation

Làm configurable: Env vars INSIGHTS_RESOLVED_RETENTION_DAYS (default 30), INSIGHTS_ACTIVE_RETENTION_DAYS (90).
Thêm UI setting ở admin panel để update retention (store in DB config table).
Extend job để prune based on config.

Finding #6: Observability Gaps – Thiếu Tracing và Alerting cho Risk Pipeline
Description
Không có OpenTelemetry spans cho pipeline (CVE matcher → insights → UI), và alerting rules cho high-risk (e.g., critical CVE).
Technical Root Cause
Metrics chỉ cơ bản (insights_created_total{type,severity}); không instrument tracing ở workers/engine; không rules.yaml mẫu.
Impact

Khó debug latency (e.g., slow BatchCreateOrUpdateInsights) → MTTR cao.
Không proactive alert cho new critical risks.

Risk Level
Medium
Recommendation

Tích hợp OTel: Add spans ở engine.EvaluateResource, matcher workers (otel-go instrumentation cho NATS/HTTP/DB).
Alerting: Thêm Prometheus rules (e.g., insights_created_total{severity="critical"} >0 → alert).
Deploy Jaeger cho traces.

Finding #7: Security – Thiếu Audit Logs cho User Actions trong Risk Center
Description
Không log user actions (e.g., view/acknowledge risks) – chỉ log server-side processing.
Technical Root Cause
Không middleware audit ở API handlers (insights_handlers.go); log.Printf không capture user_id/action.
Impact

Không trace ai xem/resolve risks → compliance gaps (e.g., GDPR audit).
Rủi ro insider threats undetected.

Risk Level
Medium
Recommendation

Thêm audit middleware ở Gin: Log {user_id, action, resource_id, timestamp} đến bảng audit_logs hoặc external log (e.g., via NATS).
UI: Trigger log khi open drawer hoặc acknowledge.

Finding #8: Multi-Cluster Aggregation Không Hoàn Hảo, Thiếu Global View
Description
Aggregation dựa clusterId query param (default all), nhưng không có global metrics (e.g., cross-cluster trends) hoặc hierarchical views.
Technical Root Cause
Queries scope by pods.cluster_id; không partition hoặc aggregate functions cho multi-cluster summaries.
Impact

Ở multi-cluster setups, user khó overview tổng thể → miss correlated risks cross-clusters.

Risk Level
Low-Medium
Recommendation

Thêm API /api/v1/risks/global-summary – aggregate counts by severity across all clusters (GROUP BY severity).
UI: Thêm tab "Global Risks" với cluster filter dropdown.

Finding #9: PCE Visualization Thiếu Advanced Features (e.g., Heatmaps, Trends)
Description
Tab PCE chỉ summary severity + bảng drill-down; thiếu visualizations như heatmaps (capability vs pods) hoặc trends over time.
Technical Root Cause
UI dùng cơ bản bảng/sort/filter; không Recharts cho PCE trends (chỉ Threat Velocity dùng chart).
Impact

Khó visualize exposure patterns → phân tích PCE chậm.

Risk Level
Low
Recommendation

Extend tab PCE với Recharts Heatmap (capability severity vs namespaces) và LineChart (exposure trends 7 days).
Fetch data từ new API /pod-capabilities/trends (query pod_capabilities by time).

Finding #10: Customizable Rules Không Đầy Đủ, Thiếu UI Config
Description
Rules YAML (FORTUNA_RULES_DIR) có nhưng không user-editable qua UI/DB – chỉ dev config.
Technical Root Cause
Không endpoints/UI cho CRUD rules; chỉ load từ file/env.
Impact

User không tune priorities/thresholds → kém flexible cho custom environments.

Risk Level
Low
Recommendation

Thêm API /api/v1/risk-rules (CRUD) – lưu rules vào DB table risk_rules.
UI admin panel để edit rules (YAML editor trong React).

Kết Luận và Đề Xuất Tổng Thể
Risk Center có nền tảng tốt với unified view (insights + PCE + evidence), integration NATS cho async processing, và retention job. Tuy nhiên, để enterprise-ready, cần ưu tiên real-time (#1), unified scoring (#2), và caching (#3) – ước tính 4-6 tuần refactor. Tổng risk: Medium, chủ yếu từ UX/performance gaps.

---

## Bước tiếp theo (sau Phase 2 + NATS + WebSocket sync)

- **Đồng bộ WebSocket:** Đã kiểm tra và ghi lại trong [WebSocket-Flows-Sync.md](../WebSocket-Flows-Sync.md): luồng Risk Center (`/ws/risks`, `insights_updated`) và Pod Detail (`/ws/pod/:uid`, `type` metrics/processes/network/events); Core/Agent/Dashboard đồng bộ; Dashboard Pod Detail refetch theo đúng `type`.
- **Phase 3 (Risk-Center-Improvement-Plan.md):**
  - **3.1** Unified risk score: API GET `/api/v1/risks/with-scores` (hoặc mở rộng GET /risks) trả về insight kèm `total_score`, `priority_level` từ `risk_scores`; UI sort/filter by score.
  - **3.2** Observability: OpenTelemetry spans cho EvaluateResource, BatchCreateOrUpdateInsights, CVE matcher; Prometheus alert rules mẫu (critical insight tăng đột biến).
  - **3.3** Export PDF + SIEM: GET `/risks/export?format=pdf`; publish NATS `fortuna.siem.events` cho insight critical/high; adapter webhook.
- **Phase 4 (tùy chọn):** PCE trends/heatmap, **Risk rules CRUD + UI** (đã làm: API CRUD `/risk-rules`, migration `risk_rules`, engine `LoadRulesFromDB`, Dashboard Settings tab "Risk Rules" với bảng + Add/Edit/Delete khi `source === 'db'`), unified scoring Option B (schema migration).

---

## Kiểm tra đồng bộ Core ↔ Agent ↔ Dashboard ↔ DB (cập nhật mới)

| Thành phần | Core | Agent | Dashboard | DB |
|------------|------|-------|-----------|-----|
| **Risk rules CRUD** | GET/POST/PUT/DELETE `/risk-rules`, `GetRiskRulesList(db)` (DB trước, fallback files) | — | Settings → tab "Risk Rules": bảng, Add/Edit/Delete khi `source === 'db'`; `api.getRiskRules`, `createRiskRule`, `updateRiskRule`, `deleteRiskRule` | Bảng `risk_rules` (migration 075) |
| **Risks + scores** | GET `/risks?withScores=1&priorityLevel=...`, join `risk_scores`, cache key có withScores/priorityLevel | — | Risk Center: `getRisks({ withScores: 1 })`, sort by score/priority, cột Risk Score, filter Priority | `insights`, `risk_scores` |
| **Export risks** | GET `/risks/export` (CSV), GET `/risks/export?format=pdf` (HTML in để in PDF) | — | Risk Center: nút "Export CSV", "Export PDF" gọi `exportRisksCSV`, `exportRisksPDF` | — |
| **WebSocket risks** | `/ws/risks`, broadcast khi NATS `fortuna.insights.updated` | Workers publish sau BatchCreateOrUpdateInsights | Risk Center: `getRisksWsUrl()`, listen và refetch khi `insights_updated` | — |
| **WebSocket pod detail** | `/ws/pod/:uid`, broadcast theo `type` (metrics/processes/network/events) | Gửi runtime metrics/processes/network/events lên Core | Pod Detail: subscribe WS, refetch theo `type` | Pod runtime tables |
| **Insights / summary** | GET `/insights/summary`, `/insights/summary/by-cluster`, cache | — | Risk Center + Dashboard: `getInsightsSummary`, `getInsightsSummaryByCluster` | `insights` |
| **Clusters / pods** | GET `/clusters`, `/clusters/stats`, `/pods`, … | Sync cluster + pods → Core | Clusters, Resources, Pod Detail dùng API tương ứng | `clusters`, `pods`, … |

**Database:** Core chạy toàn bộ migrations lúc startup (gồm 075 `risk_rules`). PostgreSQL/NATS deploy cùng infra; không xóa DB khi chỉ clean + rebuild + redeploy (trừ khi dùng `--db` hoặc `--db-reset`).