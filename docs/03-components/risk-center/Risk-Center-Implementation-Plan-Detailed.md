# Risk Center – Plan triển khai chi tiết (gaps hoàn thành)

Plan này chia công việc còn lại thành **Backlog (phụ thuộc hạ tầng)**, **Coming Soon** (tính năng liên quan), và **các phase triển khai** có thể làm ngay (không phụ thuộc Grafana/Prometheus/Redis).

---

## 1. Backlog – Phụ thuộc hạ tầng (Grafana / Prometheus / Redis)

Các hạng mục dưới đây **không** đưa vào plan thực hiện ngay; đưa vào **backlog** và chỉ triển khai khi hạ tầng sẵn sàng.

| ID | Hạng mục | Phụ thuộc | Ghi chú |
|----|----------|------------|---------|
| B1 | **Grafana dashboard** cho Risk metrics | Grafana + Prometheus datasource | Metrics `RiskEvaluationDuration`, `InsightsBatchSize` đã expose; cần dashboard Grafana khi có hạ tầng. |
| B2 | **Prometheus alerting rules** (risk eval > Xs, batch size bất thường) | Prometheus + Alertmanager | Rule mẫu: `risk_evaluation_duration_seconds > 5`, `insights_batch_size > 1000`. |
| B3 | **Redis** (nếu chuyển cache/WS sang Redis) | Redis cluster | Hiện Core dùng in-memory cache và WS; nếu sau này chuyển sang Redis cho scale-out thì cần hạ tầng Redis. |

**Thống nhất:** Các mục B1–B3 phân tích và đưa vào backlog; không gắn vào sprint hiện tại.

---

## 2. Coming Soon – Tính năng liên quan hạ tầng

Các tính năng sau **phụ thuộc hoặc gắn với** Grafana/Prometheus/Redis; trên UI/docs đánh dấu **Coming Soon** (không ẩn, nhưng không cam kết ship khi chưa có hạ tầng).

| ID | Tính năng | Liên quan | Cách thể hiện |
|----|-----------|-----------|----------------|
| C1 | **Dashboard Risk metrics** (số liệu eval, batch) trong Dashboard/Fortuna | Grafana/Prometheus | Trong Risk Center hoặc Settings: block "Risk metrics" với text "Coming Soon – sẽ có khi tích hợp Grafana/Prometheus." |
| C2 | **Alerting Risk** (cảnh báo eval chậm, batch bất thường) | Prometheus + Alertmanager | Trong docs/checklist: "Alerting rules – Coming Soon (phụ thuộc Prometheus)." |
| C3 | **Cache/WS scale-out bằng Redis** | Redis | Không hiển thị Coming Soon trên UI; chỉ backlog kỹ thuật khi có Redis. |

**Thống nhất:** C1, C2 có thể nhắc trong doc và (tuỳ chọn) một label/note "Coming Soon" trong overview; không implement logic Grafana/Prometheus/Redis trong codebase hiện tại.

---

## 3. Plan triển khai theo phase (không phụ thuộc hạ tầng)

Thứ tự ưu tiên: **Data scope (KPI đúng) → Ops doc → UI PCE/Evidence → Export → UX polish → Test.**

### Phase 1 – Data scope: Risk Trend & Dashboard stats (all insight types) ✅ Done

**Mục tiêu:** Risk Trend (7 ngày) và KPI Total Risks / Critical đại diện **tất cả** insight types (vulnerability, RBAC, misconfiguration, …), không chỉ CVE.

| Bước | Task | Trạng thái |
|------|------|------------|
| 1.1 | Core: GetThreatVelocity – query param `byType=all` (default `vulnerability`) | ✅ |
| 1.2 | Dashboard: getThreatVelocity(7, clusterId, **'all'**) từ Risk Center | ✅ |
| 1.3 | Core: GetDashboardStats – query param `byType=all` cho totalRisks & critical | ✅ |
| 1.4 | Dashboard: getStats(..., **'all'**) từ Risk Center | ✅ |
| 1.5 | Doc: overview + Gaps cập nhật; E2E byType optional | Doc done; E2E optional |

**Deliverable:** Risk Center dùng `byType=all`; Risk Trend và KPI phản ánh toàn bộ findings.

---

### Phase 2 – Ops / Env documentation ✅ Done

**Mục tiêu:** Document env Risk Center ở một nơi; chuẩn bị cho helm/ops khi có.

| Bước | Task | Trạng thái |
|------|------|------------|
| 2.1 | Tạo **Risk-Center-Ops-Env.md**: bảng env, default, gợi ý dev/stg/prod | ✅ |
| 2.2 | Verify-And-Deploy-Checklist link sang Risk-Center-Ops-Env.md | (optional: thêm link trong checklist) |

**Deliverable:** [Risk-Center-Ops-Env.md](./Risk-Center-Ops-Env.md) đã tạo; sẵn sàng copy sang helm khi có.

---

### Phase 3 – PCE: Table drill-down + Heatmap click → filter

**Mục tiêu:** Tab PCE có bảng drill-down (Namespace | Pod | Capability | Sev | Evidence | Last Seen); click ô heatmap → filter bảng.

| Bước | Task | Chủ thể | Chi tiết | Ước lượng |
|------|------|---------|----------|-----------|
| 3.1 | getPceCapabilities đã có namespace, severity | — | API sẵn có. | — |
| 3.2 | Heatmap cells clickable | Dashboard | Click cell → set pceNamespace + pceSeverityFilter, gọi getPceCapabilities(namespace, severity), setPceDetails. | ✅ |
| 3.3 | Chip "Filtered by ns, severity" + Clear | Dashboard | pceHeatmapFilter state; Clear refetch without filter. | ✅ |
| 3.4 | Severity dropdown trong form drill-down | Dashboard | Thêm cột Severity; Run filters/Refresh truyền pceSeverityFilter. | ✅ |

**Deliverable:** PCE tab: click heatmap → filter table; chip + Clear filter.

---

### Phase 4 – Evidence: Tab Audit Trail toàn cục ✅ Done

**Mục tiêu:** Trang /risks/evidence có 2 tab: "Runtime Signals" và "Audit Trail" (User | Action | Finding ID | Timestamp | IP).

| Bước | Task | Chủ thể | Chi tiết | Ước lượng |
|------|------|---------|----------|-----------|
| 4.1 | getAuditLogs(resource: 'insight') | — | API đã hỗ trợ; không truyền resourceId = toàn bộ insight logs. | ✅ |
| 4.2 | Sub-tabs Runtime Signals \| Audit Trail | Dashboard | evidenceSubTab state; khi audit gọi getAuditLogs({ resource: 'insight', page, pageSize: 20 }); bảng User, Action, Finding ID, Timestamp, IP. | ✅ |
| 4.3 | Pagination Audit Trail | Dashboard | evidenceAuditPage, Previous/Next khi total > 20. | ✅ |
| 4.4 | AuditLog type + resourceId, ip | Dashboard | types + api mapping. | ✅ |

**Deliverable:** /risks/evidence có tab Audit Trail toàn cục.

---

### Phase 5 – PCE Export Heatmap

**Mục tiêu:** Nút "Export Heatmap" trên tab PCE (CSV hoặc image).

| Bước | Task | Chủ thể | Chi tiết | Ước lượng |
|------|------|---------|----------|-----------|
| 5.1 | Client-side CSV | Dashboard | Build CSV từ pceHeatmap (namespace, severity, count); download blob. | ✅ |
| 5.2 | Nút Export Heatmap | Dashboard | Header PCE tab: nút "Export Heatmap"; tooltip "Export heatmap data as CSV". | ✅ |

**Deliverable:** User export PCE heatmap as CSV.

---

### Phase 6 – UX: [REDACTED] & polish

**Mục tiêu:** Evidence/JSON hiển thị [REDACTED] ổn định; một số polish nhỏ.

| Bước | Task | Chủ thể | Chi tiết | Ước lượng |
|------|------|---------|----------|-----------|
| 6.1 | Kiểm tra hiển thị [REDACTED] | Dashboard | Trong drawer Evidence & Audit và mọi chỗ render evidence JSON/code: kiểm tra chuỗi [REDACTED] không làm vỡ layout; nếu cần thêm class/style (monospace, word-break). | 0.25d |
| 6.2 | Doc | Doc | Gaps: đánh dấu "[REDACTED] display" đã kiểm tra/điều chỉnh. | 0.25d |

**Deliverable:** Evidence có [REDACTED] hiển thị đúng; doc cập nhật.

---

### Phase 7 – Test (E2E & UI)

**Mục tiêu:** E2E flow cơ bản (ingest → insights → WS → UI); có thể thêm UI test sau.

| Bước | Task | Chủ thể | Chi tiết | Ước lượng |
|------|------|---------|----------|-----------|
| 7.1 | E2E script: agent → worker → insights → WS → refetch | Scripts | Script (bash/curl hoặc e2e framework): tạo dữ liệu chuẩn (hoặc dùng fixture) → trigger worker / NATS → đợi insights trong DB → gọi WS hoặc poll GET /risk/insights → assert count/title. | 1d |
| 7.2 | E2E byType=all | Scripts | Thêm TC: GET threat-velocity?byType=all, GET dashboard/stats?byType=all → 200, body hợp lệ. | 0.25d |
| 7.3 | UI test (Cypress/Playwright) | Backlog optional | Routes /risks, /risks/findings; bulk actions; histogram click → filter. Đánh dấu backlog nếu chưa có framework. | 1d (backlog) |

**Deliverable:** E2E flow ingest → UI có trong script; byType=all có TC; UI test trong backlog nếu chưa làm.

---

### Phase 8 – Backlog rõ ràng (không làm ngay)

| Mục | Ghi chú |
|-----|--------|
| Sidebar filter 280px | Wireframe "phiên bản nâng cao"; backlog. |
| Risk Score vs Capability Severity | Wireframe "(Future)"; backlog. |
| MITRE ATT&CK mapping trong drawer | "Nếu có"; backlog. |
| Grafana dashboard, Prometheus alerting, Redis | Đã đưa vào §1–2. |

---

## 4. Tổng hợp thứ tự & ước lượng

| Phase | Nội dung | Ước lượng (ngày) |
|-------|----------|-------------------|
| 1 | Data scope (Risk Trend + stats all types) | 1.75 |
| 2 | Ops env doc | 0.75 |
| 3 | PCE table + heatmap click filter | 1.75 |
| 4 | Evidence tab Audit Trail | 1 |
| 5 | PCE Export Heatmap | 0.5 |
| 6 | [REDACTED] UX + doc | 0.5 |
| 7 | E2E (flow + byType) | 1.25 |
| **Tổng (triển khai)** | | **~7.5 ngày** |

Backlog (không tính): B1–B3, C1–C2, sidebar filter, Risk Score vs Capability, MITRE ATT&CK, UI test (Cypress/Playwright) khi chưa có framework.

---

## 5. Bắt đầu từ Phase 1

Ưu tiên bắt đầu: **Phase 1 (Data scope)** để KPI và Risk Trend phản ánh đúng toàn bộ findings. Các bước cụ thể:

1. **Core:** GetThreatVelocity – thêm `byType` query param (all | vulnerability), khi `all` bỏ filter insight_type.
2. **Core:** GetDashboardStats – thêm `byType` (all | vulnerability), khi `all` đếm mọi insight_type cho totalRisks và critical.
3. **Dashboard:** getThreatVelocity(..., byType: 'all'), getStats(..., byType: 'all') (có thể mặc định `all` cho Risk Center).
4. **Doc:** Cập nhật overview + Gaps; E2E (optional) byType=all.

Sau khi Phase 1 xong, lần lượt Phase 2 → 3 → 4 → 5 → 6 → 7 theo năng lực.

---

*Tài liệu này thống nhất Backlog (Grafana/Prometheus/Redis), Coming Soon cho tính năng liên quan, và plan chi tiết để hoàn thành các công việc không phụ thuộc hạ tầng.*
