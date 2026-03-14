# Fortuna – Risk Center Component Review – Câu trả lời khảo sát

Tài liệu này trả lời từng phần khảo sát trong **Risk-Center_Review.md**, dựa trên **hiện trạng mã nguồn**, **API**, và **tài liệu** trong repo (Fortuna/KSAM).

---

## 1. Thông tin Tổng quan Risk Center

### Mô tả chi tiết chức năng
- **Risk Center** là **một trang trong Dashboard** (route `/risks`), không phải module tách riêng. Component: `dashboard/pages/Insights.tsx` (`RiskCenter`).
- Chức năng chính:
  - **Hiển thị risk findings (insights):** danh sách có phân trang, lọc theo severity/status/search, cluster, time window; severity bar (Critical/High/Medium/Low); Resolved (24h).
  - **PCE visualization:** tab "Capability Exposure (PCE)" – summary theo severity từ `pod_capabilities`, drill-down theo cluster/namespace/pod/capability.
  - **Evidence correlation:** tab "Evidence & References" – runtime signals (bảng), link sang Capability Catalog; trong drawer chi tiết một risk: runtime evidence + linked capabilities + external references (từ capability metadata).
- **Total findings** chỉ đếm **Risk Findings (insights)**; PCE và Evidence **không** cộng vào Total (xem `docs/07-guides/RISK_CENTER_TOTAL_FINDINGS_LOGIC.md`).

### High-level architecture (text)
```
User (Browser)
    → Dashboard React (RiskCenter @ /risks)
    → Core API (GET /api/v1/risks, /insights/summary, /dashboard/stats, /pod-capabilities/..., /runtime-signals)
    → Core (Go): handlers → DB (PostgreSQL: insights, pods, pod_capabilities, runtime_signals, cve_matches, ...)
Ingest:
  Agent → (SBOM/CVE, runtime signals, capabilities) → Core (gRPC/NATS)
  CVE matcher worker (NATS sbom.created) → insights
  Risk worker (NATS) / Historical evaluator → insights (RBAC/capability)
  PCE: pod sync + capability evaluator → pod_capabilities
```
- **Không** có diagram PNG/SVG sẵn trong repo; có thể vẽ từ luồng trên.

### Mục tiêu kinh doanh / use-case
- Ưu tiên rủi ro cho SecOps: severity bar, sort by score/severity, filter by cluster/time.
- Compliance / audit: trạng thái workflow (active, resolved, acknowledged), Resolved (24h), Threat Velocity (7 ngày).
- Điều tra nhanh: click finding → drawer với risk summary, PCE, runtime evidence, references; link sang Pod/Identity, Attack path.

### Version / tag Risk Center
- Risk Center **không** là module tách version riêng; cùng version với Dashboard/Core (monorepo). Tag/release theo dự án Fortuna.

---

## 2. System Integration và Data Flow

### Nguồn dữ liệu cho Risk Center
- **Insights (risk findings):**
  - **CVE matcher:** từ SBOM (NATS `sbom.created`) → CVE match DB → `InsightManager.BatchCreateOrUpdateInsights` (vulnerability insights).
  - **gRPC SBOM/CVE:** Agent gửi CVE findings → `storeCVEFindings` → upsert insights (vulnerability).
  - **Risk engine (RBAC/behavior):** Rule-based engine (`core/pkg/riskengine/engine.go`) đánh giá ServiceAccount/Role/RoleBinding → insights; chạy qua **Risk Worker** (NATS) hoặc **Historical Risk Evaluator** (DB).
  - **Capability evaluator:** tạo capability insights (overprivileged, etc.) khi sync pod/capabilities.
- **Risk scores:** bảng `risk_scores` (MVP2), API `/api/v1/risk/scores` – dùng cho analytics/trends; **Risk Center UI chính** dùng **insights** (GET `/risks` = GetInsightsList), không trực tiếp hiển thị bảng risk_scores.
- **PCE:** `pod_capabilities` – từ capability evaluator (pod spec sync, runtime signals).
- **Runtime signals:** bảng `runtime_signals` – từ Agent (runtime events).

### Luồng dữ liệu
- **Real-time / async:** NATS streams (`fortuna-sbom`, `fortuna-insights`, …): SBOM created → CVE matcher worker → insights; risk worker nhận message → evaluate → BatchCreateOrUpdateInsights.
- **Polling DB:** Dashboard gọi REST API; Core đọc từ PostgreSQL (insights, pods, pod_capabilities, runtime_signals). Không có push real-time từ server tới browser.
- **End-to-end:** Agent (SBOM/CVE, signals, capabilities) → Core (gRPC/NATS) → DB; Core API (GET /risks, /insights/summary, …) → Dashboard Risk Center (polling theo interval từ `refreshIntervalStore`).

### Integration với components khác
- **Core API:** `/api/v1/risks` (GetInsightsList), `/api/v1/insights/summary`, `/api/v1/insights/:id`, `/api/v1/dashboard/stats` (resolved24h), `/api/v1/risk/scores` (risk scores), `/api/v1/pod-capabilities`, `/api/v1/runtime-signals`.
- **Dashboard:** Risk Center dùng chung cluster selector và time window (store + URL params); link "View All Risks" từ Dashboard truyền `clusterId`, `sinceMinutes`.
- **PCE:** tab PCE dùng `/api/v1/pod-capabilities/summary/severity` và `/api/v1/pod-capabilities` (filter cluster/namespace/pod/capability).

### Multi-cluster
- **clusterId** query param: khi set, chỉ insights có `resource_uid` thuộc pod trong cluster đó (`pods.cluster_id = ?`, `deleted_at IS NULL`). Summary và list dùng cùng scope. Không có API aggregate “tất cả clusters” dạng single number; UI có thể gọi không truyền clusterId = “all clusters”.

---

## 3. User Interface và Functionality

### UI framework
- **React** (Dashboard). Component chính: `dashboard/pages/Insights.tsx` (`RiskCenter`). Các phần: severity bar, bảng risks (sort/filter), Threat Velocity (Recharts AreaChart), tab PCE (summary + bảng drill-down), tab Reference (RuntimeSignalsTable + link Capability Catalog), **Risk Detail Drawer** (panel bên phải khi click một finding).

### Tính năng chính
- **Filtering/sorting risks:** Severity (all/critical/high/medium/low), Status (all/active/resolved/acknowledged), Search (title, description, resource_name), clusterId, sinceMinutes. Sort: newest/oldest, score desc/asc, severity desc, title A–Z. Backend: `GetInsightsList` trong `core/internal/api/dashboard_handlers.go`.
- **Drill-down evidence:** Click một risk → drawer: Finding Summary, Related Capability IDs, Capability Details, Runtime Evidence, External References, Impacted Resources; link "Open full runtime evidence tab" / "Open capability exposure tab".
- **PCE exposure:** Tab PCE – summary theo severity; bảng drill-down với filter cluster/namespace/pod/capability, sort severity/capability/pod/namespace.
- **Trends/analytics:** Threat Velocity (7 ngày) – daily counts by severity; click điểm trên chart → filter bảng theo ngày đó. Resolved (24h) từ `/dashboard/stats`.

### Authentication/Authorization
- Dashboard gọi API với token (login); Core có auth (JWT). **RBAC riêng cho Risk Center views** (ví dụ admin-only priorities) **không** thấy trong code – authorization ở mức API chung.

### Export/Reporting
- **Export risks (CSV/PDF)** hoặc **integration SIEM** **không** thấy trong mã nguồn hiện tại. Chỉ có xem trên UI và API GET.

---

## 4. Data Model và Storage

### Schema risk-related
- **insights:** `core/pkg/models/implementation_guide.go` (Insight). Cột chính: id, resource_type, resource_namespace, resource_name, resource_uid, insight_type, severity, title, description, recommendation, cve_id, affected_component, affected_version, fixed_version, cvss, evidence (jsonb), violated_rules (jsonb), status, detected_at, resolved_at, created_at, updated_at, deleted_at. Indexes: resource_uid, severity, insight_type, deleted_at; GIN jsonb; unique (resource_uid, cve_id, insight_type) cho batch upsert.
- **risk_scores:** Bảng riêng (MVP2): total_score, base_score, severity_weight, impact_multiplier, time_decay, factors (json), priority_level, resource_uid, cluster_id, namespace, resource_type, calculated_at, exploitability_score, business_impact_score, scorer_version, deleted_at. Indexes: total_score, resource_uid, cluster_id, namespace, priority_level, calculated_at.
- **cve_matches:** sbom_id, pod_uid, cve_id, package_name, package_version, severity, cvss, fixed_version, matched_by, …
- **pod_capabilities:** id, pod_uid, namespace, capability_id, capability_group, severity, state, confidence, evidence (jsonb), mitre (text[]), first_seen_at, last_seen_at, created_at, updated_at.
- **runtime_signals:** Bảng runtime events/signals (model trong `core/pkg/models/runtime_signal.go`); Core có migration `045_add_runtime_signals_table.go`, `046_add_capability_state_machine.go`.

### PCE-specific
- Capability exposure lưu trong **pod_capabilities**. Reconciliation/sync từ pod spec và runtime qua capability evaluator (`core/pkg/capability/evaluator.go`); state machine (detected/confirmed/exploited/chained) trong migration 046.

### Evidence handling
- **insights.evidence** (jsonb), **insights.violated_rules** (jsonb) – migration 058. Correlation: Risk Center UI liên kết finding → pod UID → runtime signals (GET by pod) và pod capabilities (GET by pod); không có bảng link trực tiếp runtime_events → insights trong schema, correlation theo resource_uid/pod_uid.

### Retention policy
- **InsightsCleanupJob** (chạy mỗi 24h): soft-delete resolved insights cũ hơn **30 ngày**; soft-delete active insights **không cập nhật trong 90 ngày**; cleanup duplicate insights (cùng description, type, severity, giữ bản mới nhất). Code: `core/internal/scheduler/insights_cleanup_job.go`.

---

## 5. Processing và Analysis Logic

### Risk scoring engine
- **Insights** (Risk Center list): không dùng một “risk score” tổng hợp CVE+PCE+runtime trong engine đơn. Severity từ rule (RBAC) hoặc từ CVE (CVE matcher/grpc). Score hiển thị trên UI có thể từ insight (cvss/severity). **risk_scores** (bảng riêng): có thuật toán MVP2 (base_score, severity_weight, impact_multiplier, time_decay, priority_level); API `/api/v1/risk/scores` và handlers trong `core/internal/api/risk/risk_handlers.go`, scorer trong `core/pkg/risk/scorer.go`.
- Rule engine: `core/pkg/riskengine/engine.go` – EvaluateResource áp dụng rules (YAML hoặc hardcoded), điều kiện (resource/expression), aggregation (AND/OR/THRESHOLD); mỗi rule có BaseScore; tạo insight với severity từ rule category.

### PCE logic
- **Capability evaluator** (`core/pkg/capability/evaluator.go`): từ pod spec sync và runtime signals; detect/expose capabilities, tạo capability insights và cập nhật pod_capabilities; resolve stale capability insights khi pod không còn có capability đó.

### Correlation
- CVE ↔ pod: qua resource_uid (pod UID); CVE matcher và insight manager gắn insight với pod. **pod_processes** và runtime signals có pod_uid; UI correlate bằng cách gọi GET runtime-signals by pod và GET pod-capabilities by pod khi mở drawer. Async workers qua NATS (CVE matcher, risk worker); không có worker riêng “correlator” ngoài logic trong Risk Center (fetch theo pod).

### Customizable rules
- Risk engine hỗ trợ **YAML rules** (FORTUNA_RULES_DIR); rule có Enabled, Conditions, Severity, BaseScore. **User config risk priorities/thresholds** qua policy CRD hoặc DB **không** thấy trong code; chỉ có rule file và env.

---

## 6. Performance & Scalability

### Scaling target
- Code không định nghĩa target rõ (ví dụ “1000+ risks/cluster”). List có phân trang (page, pageSize, mặc định 20). Query latency cho `/risk/scores` không có SLA trong code.

### Caching
- **Không** thấy cache in-memory hoặc Redis cho risk/insights trong Core; mỗi request query DB. Dashboard có polling interval (refreshIntervalStore) để giảm tần suất gọi.

### Throughput
- Số findings/sec ingest: không có metric hoặc limit cố định trong code. Batch upsert insights (BatchCreateOrUpdateInsights) dùng cho CVE matcher và risk worker.

### Backpressure
- NATS stream có giới hạn (maxMsgs, maxBytes, retention); không thấy throttle riêng cho “high-volume events” trong Risk Center pipeline trong code.

---

## 7. Reliability & High Availability

### Failover
- Nếu Core down, Risk Center (UI) chỉ có thể retry hoặc hiển thị lỗi; **không** có fallback data (cached views) phía client được mô tả trong code.

### Data consistency
- Insights: eventual (async workers, batch upsert). Read: strong consistency với DB (mỗi GET đọc DB). Không có multi-region replication được cấu hình trong repo.

### Backup/Restore
- Risk data nằm trong PostgreSQL; strategy backup/restore là **part of DB backup** (không có script riêng cho risk/insights trong repo).

---

## 8. Security Model

### Access controls
- API dùng JWT (auth); **RBAC/ABAC/ReBAC** chi tiết cho “sensitive risks” **không** thấy trong code – quyền ở mức API chung.

### Auditing
- **Log user actions** (view/edit risks) trong Risk Center **không** thấy (không audit log riêng cho risk views trong code đã kiểm tra).

### Vulnerability / evidence exposure
- Evidence (jsonb) có thể chứa thông tin nhạy cảm; **mask sensitive process data** không thấy logic cụ thể trong handlers. UI hiển thị signal type, category, date; chi tiết evidence tùy nội dung API trả về.

---

## 9. Observability

### Metrics
- Prometheus: `fortuna_insights_created_total` (labels: insight_type, severity), `fortuna_insights_active_total`, `fortuna_risk_scores_calculated_total`, `fortuna_risk_score_distribution` (`core/pkg/metrics/metrics.go`). **risk_queries_total** hoặc **correlation_duration** **không** thấy.

### Logging
- Structured log dạng `log.Printf` (ví dụ `[InsightManager]`, `[GetInsights]`, `[InsightsSummary]`, `[RiskWorker]`, `[InsightsCleanupJob]`). Không chuẩn JSON structured (ví dụ zap) toàn bộ.

### Tracing
- **OpenTelemetry spans** cho risk pipeline **không** thấy trong code đã xem.

### Alerting
- **Rules cho high-risk findings** (ví dụ critical CVE alert) **không** thấy trong repo; có thể cấu hình ngoài (Prometheus alerting) dựa trên metrics trên.

---

## 10. Thông tin Bổ sung

### Code snippets quan trọng
- **Risk engine:** `core/pkg/riskengine/engine.go` – NewEngine, EvaluateResource, createInsight.
- **Insight manager:** `core/pkg/riskengine/insight_manager.go` – BatchCreateOrUpdateInsights, batchUpsertVulnerabilityInsights.
- **PCE/capability:** `core/pkg/capability/evaluator.go` – resolve stale capability insights; handlers `core/internal/api/pod_capability_handlers.go`.
- **Risk Center API:** `GET /api/v1/risks` → `GetInsightsList` (`core/internal/api/dashboard_handlers.go`); `GET /api/v1/insights/summary` → `GetInsightsSummary` (`core/internal/api/insights_handlers.go`); `GET /api/v1/risk/scores` → `risk.GetRiskScores` (`core/internal/api/risk/risk_handlers.go`).

### Known issues
- Không có GitHub issues mở được trích trong repo. Có thể tham khảo test E2E: `e2e-risk-center-verify.sh` (seed 1 insight, verify /risks, /insights/summary, runtime-signals) và docs data integrity `dashboard_data_integrity.go` để biết các API được coi là “real” và nguồn dữ liệu.

### Benchmark reports
- **Load test** cho risk ingestion/display **không** có trong repo.

### Deployment
- Risk Center **không** deploy riêng; là phần Dashboard (React) + Core API. YAML/Helm theo deployment chung của Fortuna (Dashboard + Core); không có chart riêng “risk-center”.

---

## Tài liệu tham chiếu trong repo

| Tài liệu | Nội dung |
|---------|----------|
| `docs/07-guides/RISK_CENTER_TOTAL_FINDINGS_LOGIC.md` | Total findings = Risk Findings only; scope cluster/time; PCE & Evidence không cộng vào Total. |
| `docs/07-guides/RISK_CENTER_UI_AND_APIS.md` | Ánh xạ UI ↔ API (getRisks, getInsightsSummary, getStats, pod-capabilities, runtime-signals). |
| `core/internal/api/dashboard_handlers.go` | GetInsightsList (/risks), GetDashboardStats. |
| `core/internal/api/insights_handlers.go` | GetInsightsSummary, GetInsight, Acknowledge/Resolve/Dismiss. |
| `core/internal/scheduler/insights_cleanup_job.go` | Retention: resolved 30 ngày, active 90 ngày, dedup. |
| `scripts/e2e/e2e-risk-center-verify.sh` | E2E seed insight + verify /risks, /insights/summary. |

---

*Tài liệu được tạo từ phân tích mã nguồn và tài liệu hiện có trong repo; có thể bổ sung khi có thay đổi hoặc yêu cầu chi tiết hơn.*
