# Phase 3 & Phase 4 – Đánh giá tối ưu và chọn phương án

Tài liệu này đánh giá chi tiết các phương án cho Phase 3 và Phase 4 (Risk Center), chọn phương án tối ưu và ghi lại quyết định triển khai.

---

## 1. Phase 3.1 – Unified risk score

### Đánh giá phương án

| Phương án | Mô tả | Ưu | Nhược | Effort |
|-----------|--------|-----|--------|--------|
| **Option A (nhẹ)** | Giữ schema `insights`; API trả về insight kèm `total_score`, `priority_level` từ bảng `risk_scores` (join theo `resource_uid`). | Không migration; tương thích ngược; triển khai nhanh. | Mỗi request list cần join (hoặc query riêng); score có thể chưa có cho mọi resource (left join). | 2–3 ngày |
| **Option B (unified)** | Thêm cột `total_score`, `priority_level` vào bảng `insights`; engine/scorer ghi khi tạo/cập nhật insight. | Một nguồn dữ liệu; không join khi đọc. | Migration schema; sửa engine, CVE matcher, insight_manager; phải tính score mọi insight. | 4–6 ngày |

### Quyết định: **Option A**

- Không đổi schema, không rủi ro migration.
- `risk_scores` đã có `resource_uid`, `total_score`, `priority_level`; join với `insights.resource_uid` là đủ.
- Triển khai: thêm query param **`withScores=1`** cho GET `/api/v1/risks`; khi bật, left join `risk_scores` và trả về thêm `totalScore`, `priorityLevel` trong từng item (DTO hoặc map); cache key cần bao gồm `withScores`. Dashboard: sort/filter theo score và priority khi dùng API với `withScores=1`.

---

## 2. Phase 3.2 – Observability

### Đánh giá phương án

| Hạng mục | Phương án | Quyết định |
|-----------|-----------|------------|
| **OpenTelemetry spans** | Thêm spans cho EvaluateResource, BatchCreateOrUpdateInsights, CVE matcher. | Hiện **không** dùng otel trong `go.mod`. Chọn: **chỉ thêm Prometheus alert rules**; không thêm dependency otel trong phase này. Có thể thêm trace sau khi đưa otel vào dự án. |
| **Prometheus alert rules** | File rules mẫu (critical insight tăng đột biến, v.v.). | **Làm:** thêm file YAML (ví dụ `deploy/prometheus/alerts/risk-center.rules.yaml`) trong repo hoặc chart, dùng metrics có sẵn (`fortuna_insights_*`). |

### Quyết định

- **Làm ngay:** Prometheus alert rules mẫu (risk-center).
- **Tạm hoãn:** OpenTelemetry spans (triển khai khi đã có otel trong core).

---

## 3. Phase 3.3 – Export PDF và SIEM

### 3.3.1 Export PDF

| Phương án | Mô tả | Ưu | Nhược |
|-----------|--------|-----|--------|
| **A: Thư viện Go thuần (gofpdf/gopdf)** | Generate PDF trong Go, cùng endpoint `/risks/export?format=pdf`. | Không phụ thuộc binary ngoài; dễ deploy. | Cần format bảng/trang thủ công. |
| **B: HTML → PDF (wkhtmltopdf / headless)** | Render HTML rồi gọi tool chuyển PDF. | Dễ layout. | Cần cài binary; không thuần Go. |
| **C: Export “Print to PDF”** | Trả về HTML có style in sẵn; user dùng Print → Save as PDF. | Rất đơn giản; không thêm dependency. | Không trả trực tiếp file PDF. |

**Quyết định: A (gofpdf hoặc tương đương thuần Go).** Endpoint GET `/api/v1/risks/export?format=pdf` trả về file PDF (cùng filter như CSV). Dùng thư viện pure Go (ví dụ `github.com/jung-kurt/gofpdf` hoặc `github.com/signintech/gopdf`) để không phụ thuộc binary.

### 3.3.2 SIEM hook

- **Chuẩn:** Khi tạo/cập nhật insight có **severity = critical hoặc high**, publish message NATS subject **`fortuna.siem.events`** (payload JSON: insight id, severity, resource, timestamp, v.v.).
- **Vị trí gọi:** Trong `InsightManager.BatchCreateOrUpdateInsights` (sau khi lưu DB thành công) hoặc trong CVE/Risk worker sau khi gọi BatchCreateOrUpdateInsights — chọn **trong Core** (insight_manager hoặc một helper gọi từ worker) để một nơi duy nhất, dùng NATS client có sẵn.
- **Adapter webhook:** Không implement trong Core; một service riêng subscribe `fortuna.siem.events` và gọi webhook SIEM (tài liệu hướng dẫn).

---

## 4. Phase 4 – PCE trends + heatmap, Risk rules

### 4.1 PCE trends và heatmap

- **API:** GET `/api/v1/pod-capabilities/trends` **đã có** (time-series theo severity); GET `/api/v1/pod-capabilities/summary/namespace` (và summary khác) đã có.
- **Thiếu:** Trong Risk Center (tab PCE), chưa có **line chart** (trend 7 ngày) và **heatmap** (ví dụ severity × namespace hoặc capability × namespace).
- **Quyết định:**
  - Thêm **line chart** trong tab PCE (Insights.tsx) gọi `getPodCapabilitiesTrend` (đã có trong api.ts), dùng Recharts tương tự Threat Velocity.
  - Thêm **heatmap** (Recharts): dữ liệu từ `getPodCapabilitiesSummaryByNamespace` hoặc API mới (nếu cần dạng matrix). Ưu tiên: summary by namespace + severity đủ để vẽ heatmap (namespace × severity).

### 4.2 Risk rules CRUD

- **Đầy đủ (Phase 4 plan):** Bảng `risk_rules`, API CRUD, engine load từ DB, UI admin → refactor lớn.
- **Tối ưu giai đoạn 1:** Chỉ **read-only**: API GET `/api/v1/risk-rules` (và GET by id?) đọc từ **FORTUNA_RULES_DIR** (YAML), trả về danh sách rules hiện tại; không DB, không sửa engine. UI có thể chỉ “xem” rules.
- **Quyết định:** Triển khai **read-only** (GET list rules từ files); CRUD + DB để phase sau.

### 4.3 Unified scoring Option B

- **Quyết định:** **Không làm** trong phase này (schema migration + sửa engine lớn). Giữ Option A (Phase 3.1).

---

## 5. Tóm tắt phương án đã chọn

| Hạng mục | Phương án đã chọn |
|----------|--------------------|
| **3.1 Unified score** | Option A: GET /risks?withScores=1, left join risk_scores; UI sort/filter by score và priority. |
| **3.2 Observability** | Chỉ Prometheus alert rules (risk-center). Otel spans tạm hoãn. |
| **3.3 PDF** | GET /risks/export?format=pdf, generate PDF bằng thư viện Go thuần. |
| **3.3 SIEM** | Publish NATS `fortuna.siem.events` khi tạo/cập nhật insight critical/high (trong Core). |
| **4 PCE** | Line chart (trend) + heatmap (namespace × severity) trong tab PCE, dùng API có sẵn. |
| **4 Risk rules** | Read-only API GET /risk-rules từ YAML files. |
| **4 Option B** | Không làm. |

---

## 6. Thứ tự triển khai đề xuất

1. **Phase 3.1** – Risks with scores (API + cache key + Dashboard sort/filter).
2. **Phase 3.2** – Prometheus alert rules file.
3. **Phase 3.3** – Export PDF (format=pdf) + SIEM (publish fortuna.siem.events).
4. **Phase 4 PCE** – Line chart + heatmap trong tab PCE (Insights.tsx).
5. **Phase 4 Risk rules** – GET /api/v1/risk-rules (read-only từ YAML).

Sau khi hoàn thành, cập nhật Risk-Center-Improvement-Plan.md và Risk-Center-Component-Implement.md (checklist Phase 3/4).

---

## 7. Checklist triển khai (đã thực hiện)

- [x] **3.1** GET /risks?withScores=1 & priorityLevel; left join risk_scores; Dashboard sort (score/priority) + filter Priority (P0–P4); cache key gồm withScores và priorityLevel.
- [x] **3.2** Prometheus alert rules: `deploy/prometheus/risk-center.alerts.yaml`.
- [x] **3.3** Export PDF: GET /risks/export?format=pdf trả về HTML tối ưu in (Print → Save as PDF); nút "Export PDF" trên Risk Center. SIEM: stream `fortuna-siem`, subject `fortuna.siem.events`; CVE/Risk worker gọi `PublishSIEMEvents(js, insights)` cho critical/high.
- [x] **4 PCE** Tab PCE: line chart "PCE Trend (7 Days)" (getPceTrend), heatmap "Exposure by Namespace" (getPceSummaryByNamespace).
- [x] **4 Risk rules** GET /api/v1/risk-rules (read-only từ FORTUNA_RULES_DIR), `riskengine.ListRuleSummariesFromDir`, handler `GetRiskRulesList()`.
