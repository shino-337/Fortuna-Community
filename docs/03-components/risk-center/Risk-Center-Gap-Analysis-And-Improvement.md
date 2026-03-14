# Risk Center – Phân tích chi tiết các nội dung chưa thực hiện và hướng cải tiến

Tài liệu này phân tích bốn mục còn thiếu hoặc chưa đầy đủ (theo [Risk-Center-Summary-From-Source-And-Tests.md](./Risk-Center-Summary-From-Source-And-Tests.md) dòng 157–160), dựa trên **mã nguồn thực tế**, và đưa ra hướng triển khai cụ thể để bắt đầu cải tiến.

---

## Tổng quan

| # | Nội dung | Trạng thái thực tế (sau khi rà soát code) | Ưu tiên cải tiến |
|---|----------|---------------------------------------------|-------------------|
| **#5** | Retention configurable | **Một phần:** Env đã có; thiếu document + UI/DB config | Trung bình |
| **#6** | Observability (OTel, alerting) | **Một phần:** Prometheus metrics có; thiếu OTel spans + alert rules | Trung bình |
| **#7** | Audit logs (user actions) | **Một phần:** Audit cho acknowledge/resolve/dismiss có; thiếu audit "view" risk | Thấp |
| **#8** | Global view | **Một phần:** by-cluster có; thiếu endpoint global-summary + UI tab | Thấp–Trung bình |

---

## #5 – Retention configurable

### Hiện trạng trong mã nguồn

- **File:** `core/internal/scheduler/insights_cleanup_job.go`
- **Đã có:**
  - `getResolvedRetentionDays()` đọc env **`INSIGHTS_RESOLVED_RETENTION_DAYS`** (default 30).
  - `getActiveRetentionDays()` đọc env **`INSIGHTS_ACTIVE_RETENTION_DAYS`** (default 90).
  - Job chạy mỗi 24h; soft-delete insights theo hai khoảng thời gian trên.
  - Unit test: `insights_cleanup_job_test.go` (TestGetResolvedRetentionDays, TestGetActiveRetentionDays).
- **Chưa có:**
  - Document env trong deploy/README hoặc doc Risk Center (người vận hành không biết biến).
  - UI trong admin panel để **xem / chỉnh** retention (Review đề xuất lưu trong DB config table).
  - Validation: giá trị 0 hiện fallback về default (test mong đợi vậy).

### Điều cần làm để “hoàn thành” #5

1. **Document (nhanh):**
   - Thêm mục “Insights retention” vào README Core hoặc doc deploy: mô tả `INSIGHTS_RESOLVED_RETENTION_DAYS`, `INSIGHTS_ACTIVE_RETENTION_DAYS`, default, ví dụ cấu hình.
   - Cập nhật `Risk-Center-Summary-From-Source-And-Tests.md`: ghi rõ retention **đã configurable qua env**; phần còn thiếu là document + UI.

2. **UI/DB config (tùy chọn, lớn hơn):**
   - Bảng config (ví dụ `system_config`): key `insights_resolved_retention_days`, `insights_active_retention_days`, value integer.
   - Core: khi start cleanup job, đọc config từ DB (nếu có), fallback sang env rồi default.
   - Dashboard Settings (hoặc Admin): form/số ngày + API PATCH `/api/v1/config/insights-retention` (hoặc tương đương) để ghi DB.
   - Cân nhắc: không ép dùng DB ngay; document env đủ cho nhiều môi trường.

### Tham chiếu code

- Retention env: `core/internal/scheduler/insights_cleanup_job.go` (dòng 64–81, 85–87).
- Gọi job: `core/cmd/main.go` (NewInsightsCleanupJob, Start trong goroutine).

---

## Backlog – Khi có Prometheus / Observability

**Lưu ý:** Hiện tại môi trường **chưa có Prometheus**. Các nội dung dưới đây đưa vào **backlog**, triển khai khi đã có Prometheus (hoặc hệ thống observability tương đương). Các cải tiến không phụ thuộc Prometheus (#5 document, #7 audit view, #8 global-summary) đã thực hiện xong.

| Hạng mục | Mô tả | Công sức ước lượng |
|----------|--------|----------------------|
| **Prometheus – Risk Center metrics** | Thêm `risk_queries_total`, `risk_query_duration_seconds` trong handler GET /risks, GET /insights/summary, GET /risks/export. | ~1 giờ |
| **Prometheus – Alert rules mẫu** | File YAML alert rules (ví dụ `deploy/prometheus/alert-rules-risk-center.yaml`): alert khi critical insight mới hoặc `fortuna_insights_active_total` vượt ngưỡng. | ~1 giờ |
| **OpenTelemetry spans** | Instrument GET /risks, BatchCreateOrUpdateInsights, CVE matcher worker; tracer provider + export (stdout/OTLP). | 2–4 giờ |
| **Document scrape /metrics** | Hướng dẫn cấu hình Prometheus scrape endpoint `/metrics` của Core. | ~30 phút |

Sau khi triển khai Prometheus: ưu tiên alert rules → metrics Risk Center → OTel (tùy nhu cầu).

---

## #6 – Observability (OTel, alerting) [BACKLOG khi có Prometheus]

### Hiện trạng trong mã nguồn

- **Prometheus:**
  - `core/pkg/metrics/metrics.go`: **InsightsCreatedTotal** (insight_type, severity), **InsightsActiveTotal**, **RiskScoresCalculatedTotal**, **RiskScoreDistribution**; HTTP/database/worker metrics.
  - Endpoint `/metrics` (promhttp), **MetricsMiddleware** trong router.
  - Không có metric riêng kiểu `risk_queries_total` hay `correlation_duration` cho Risk Center.
- **OpenTelemetry:**
  - `go.mod` có `go.opentelemetry.io/otel`, `otel/sdk`, `otel/trace`, `otel/metric` (indirect).
  - Chưa instrument **spans** cho pipeline risk: engine.EvaluateResource, BatchCreateOrUpdateInsights, CVE matcher workers, GET /risks handler.
- **Alerting:**
  - Không có file Prometheus alert rules mẫu cho Risk Center (ví dụ: `insights_created_total{severity="critical"}` tăng đột biến, hoặc `fortuna_insights_active_total` > ngưỡng).

### Điều cần làm để cải tiến #6

1. **Prometheus – Risk Center metrics (nhỏ):**
   - Thêm (nếu muốn): `risk_queries_total` (counter, label path=risks|summary|export), `risk_query_duration_seconds` (histogram) trong handler GET /risks, GET /insights/summary, GET /risks/export.
   - Đăng ký trong `core/pkg/metrics` và gọi từ `dashboard_handlers.go` / `insights_handlers.go`.

2. **OpenTelemetry spans (trung bình):**
   - Thêm otel-go SDK trace (không chỉ indirect). Tạo tracer provider, export to stdout hoặc OTLP.
   - Instrument:
     - **GET /risks** (và cached wrapper): span "GET /risks" với attributes (cluster_id, with_scores).
     - **BatchCreateOrUpdateInsights** (hoặc nơi tạo insight hàng loạt): span "insights.batch_create".
     - **CVE matcher worker** (khi xử lý message tạo insight): span "cve_matcher.process".
   - Không bắt buộc toàn bộ pipeline ngay; ưu tiên GET /risks và batch create.

3. **Alert rules mẫu (nhỏ):**
   - Tạo file (ví dụ `deploy/prometheus/alert-rules-risk-center.yaml` hoặc thêm nhóm vào file rules có sẵn):
     - Alert khi `increase(fortuna_insights_created_total{severity="critical"}[5m]) > 0` (hoặc > N) để báo có critical mới.
     - Alert khi `fortuna_insights_active_total` tổng vượt ngưỡng (tùy chọn).
   - Document: cần Prometheus scrape `/metrics` và load file rules này.

### Tham chiếu code

- Metrics: `core/pkg/metrics/metrics.go` (Insight/risk metrics), `core/internal/api/routes.go` (MetricsMiddleware, /metrics).
- Nơi tạo insight hàng loạt: tìm `BatchCreateOrUpdateInsights` hoặc tương đương trong pkg riskengine / worker.

---

## #7 – Audit logs (user actions trong Risk Center)

### Hiện trạng trong mã nguồn

- **Đã có:**
  - **createInsightAuditLog** trong `core/internal/api/insights_handlers.go`: ghi vào `audit_logs` với action, insight id, details; lấy user từ context (middleware auth).
  - Gọi khi: **acknowledge** (POST /insights/:id/acknowledge), **resolve** (POST /insights/:id/resolve), **dismiss** (POST /insights/:id/dismiss).
  - Model `AuditLog`: action, resource, resource_id, user_id, details, cluster_id, created_at.
  - API GET `/api/v1/audit`, `/api/v1/audit-logs` (GetAuditLogs).
- **Chưa có:**
  - Audit khi user **chỉ xem** (view) risk: mở GET /insights/:id hoặc mở drawer chi tiết risk trong Risk Center. Review yêu cầu “log user actions (e.g. view/acknowledge risks)” → phần “view” chưa được log.

### Điều cần làm để cải tiến #7

1. **Audit “view” risk (ít thay đổi):**
   - Trong handler **GetInsight** (GET /insights/:id): sau khi tìm thấy insight, gọi `createInsightAuditLog(db, c, "view", idStr, "{}")` (hoặc action `"risk_view"` nếu muốn phân biệt với resource khác).
   - Lưu ý: nếu Risk Center mở drawer mà không gọi GET /insights/:id (chỉ dùng data từ list), cần thêm 1 API nhẹ kiểu POST `/api/v1/risks/:id/view` hoặc gọi createInsightAuditLog từ frontend qua API “log view” (ít lý tưởng hơn so với audit ở backend khi GET :id).
   - Khuyến nghị: audit ngay trong GetInsight khi có :id và trả về 200; không audit khi 404.

2. **Tùy chọn:**
   - Filter GET /audit theo resource=insight và action (view, acknowledge, resolve, dismiss) để báo cáo Risk Center.
   - Dashboard: trang Audit có filter “Risk Center” hoặc “Insight actions”.

### Tham chiếu code

- createInsightAuditLog: `core/internal/api/insights_handlers.go` (dòng 18–48, 548, 596, 642).
- GetInsight: `core/internal/api/insights_handlers.go` (GetInsight, dòng 150+).
- Audit model: `core/pkg/models/models.go` (AuditLog).

---

## #8 – Global view

### Hiện trạng trong mã nguồn

- **Đã có:**
  - **GET /insights/summary/by-cluster:** trả về mảng `byCluster[]`, mỗi phần tử: clusterId, clusterName, total, critical, high, medium, low. Query GROUP BY cluster_id.
  - Dashboard Risk Center: khi scope “all clusters”, gọi getInsightsSummaryByCluster và hiển thị bảng “Risks by cluster”.
- **Chưa có:**
  - Endpoint **tổng hợp toàn cục** (không theo cluster): một object `{ total, critical, high, medium, low }` cho toàn bộ insights (có thể lọc sinceMinutes). Hiện muốn “global” thì client phải cộng từ by-cluster.
  - UI tab “Global Risks” hoặc view “global summary” (một số layout chỉ cần một số tổng, không cần bảng theo cluster).

### Điều cần làm để cải tiến #8

1. **API global-summary (nhỏ):**
   - Thêm GET **/api/v1/insights/summary/global** (hoặc GET /api/v1/risks/global-summary) với query param `sinceMinutes` (tùy chọn).
   - Logic: cùng điều kiện filter insights như summary (deleted_at IS NULL, status active), nhưng **không** GROUP BY cluster_id; trả về một bản ghi tổng: total, critical, high, medium, low.
   - Có thể dùng cache (cùng TTL với summary) với key kiểu `summary:global:{sinceMinutes}`.
   - Response shape: `{ "total": number, "critical": number, "high": number, "medium": number, "low": number }` (có thể thêm resolved nếu GET /insights/summary đã có).

2. **Dashboard:**
   - Risk Center: khi scope “all clusters”, có thể gọi thêm global-summary để hiển thị **một ô “Global total”** (total/critical/high/medium/low) ở đầu, sau đó mới tới “Risks by cluster”.
   - Hoặc thêm tab “Global” chỉ hiển thị global summary + có thể link tới filter toàn bộ risks (không chọn cluster).

3. **E2E:**
   - Thêm test case: GET /insights/summary/global (hoặc /risks/global-summary) → 200, body có total, critical, high, medium, low.

### Tham chiếu code

- By-cluster: `core/internal/api/insights_handlers.go` (GetInsightsSummaryByCluster, InsightsSummaryByClusterItem); query Raw với GROUP BY p.cluster_id.
- GetInsightsSummary (single scope): cùng file, có thể tham khảo điều kiện WHERE và aggregate.

---

## Thứ tự đề xuất khi bắt đầu cải tiến

| Bước | Nội dung | Công sức ước lượng | Ghi chú |
|------|----------|--------------------|---------|
| 1 | #5 Document env retention | ~30 phút | Cập nhật README/deploy doc + Summary. |
| 2 | #8 Endpoint global-summary + E2E | ~1–2 giờ | Core handler + cache key + 1 TC E2E. |
| 3 | #7 Audit “view” trong GetInsight | ~30 phút | Một vài dòng gọi createInsightAuditLog trong GetInsight. |
| (backlog) | #6 Prometheus / OTel / alert | Xem bảng Backlog phía trên | Khi có Prometheus. |
| (tùy chọn) | #5 UI/DB config retention | 4–8 giờ | Admin panel chỉnh retention. |

---

## Cập nhật Summary sau khi rà soát

Trong [Risk-Center-Summary-From-Source-And-Tests.md](./Risk-Center-Summary-From-Source-And-Tests.md), bảng “Đối chiếu với Findings” có thể cập nhật như sau cho đúng thực tế code:

- **#5 Retention:** “**Một phần:** Đã configurable qua env (INSIGHTS_RESOLVED_RETENTION_DAYS, INSIGHTS_ACTIVE_RETENTION_DAYS). Thiếu: document cho Ops, UI/DB config (tùy chọn).”
- **#6 Observability:** “**Một phần:** Prometheus metrics (insights, risk_scores) có. Thiếu: OTel spans cho risk pipeline, Prometheus alert rules cho Risk Center.”
- **#7 Audit:** “**Một phần:** Đã audit acknowledge, resolve, dismiss (createInsightAuditLog). Thiếu: audit khi user view risk (GET /insights/:id hoặc mở drawer).”
- **#8 Global view:** “**Một phần:** GET /insights/summary/by-cluster có. Thiếu: endpoint /insights/summary/global (hoặc /risks/global-summary) và UI tab/view global.”

Tài liệu này dùng làm đầu vào cho backlog cải tiến Risk Center và có thể gắn với [Risk-Center-Component-Implement.md](./Risk-Center-Component-Implement.md), [Risk-Center-Summary-From-Source-And-Tests.md](./Risk-Center-Summary-From-Source-And-Tests.md).
