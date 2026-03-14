# Risk Center – Công việc chưa hoàn thành & Luồng chưa có dữ liệu

Tài liệu này liệt kê **các công việc chưa hoàn thành** (so với plan 3 phase + design/wireframe) và **các luồng logic chưa có dữ liệu hoặc dữ liệu hạn chế**, dựa trên mã nguồn và tài liệu Risk Center hiện tại.

---

## 1. Công việc chưa hoàn thành

### 1.1 UI/UX (so với wireframe & spec)

| Mục | Mô tả | Trạng thái |
|-----|--------|------------|
| **PCE view layout** | Wireframe: Heatmap + Table drill-down. Click cell heatmap → filter table. | **Đã có (Phase 3):** heatmap click → filter; chip "Filtered by ns, severity" + Clear; bảng có Namespace, Pod, Capability, Severity. Cột Evidence, Last Seen optional (khi API có). |
| **PCE Export Heatmap** | Wireframe header PCE: "Export Heatmap". | **Đã có (Phase 5):** nút Export Heatmap, CSV client-side. |
| **Evidence view: tab Audit Trail** | Wireframe trang Evidence: 2 tab – "Runtime Signals" và "Audit Trail" (User \| Action \| Finding ID \| Timestamp \| IP). | **Đã có (Phase 4):** sub-tabs Runtime Signals \| Audit Trail trên /risks/evidence; bảng Audit Trail + pagination. |
| **Sidebar filter 280px** | Wireframe Findings: "Sidebar filter (fixed 280px) có thể dùng trong phiên bản nâng cao". | **Backlog** – phiên bản nâng cao. |
| **Risk Score vs Capability Severity** | Wireframe PCE: "(Future) Tab Risk Score vs Capability Severity". | **Coming Soon** – backlog. |
| **MITRE ATT&CK trong drawer** | Wireframe drawer tab "PCE & Attack Path": "MITRE ATT&CK mapping (nếu có)". | **Coming Soon** – backlog. |
| **Hiển thị [REDACTED]** | Evidence masking backend đã có; UI cần đảm bảo JSON viewer/code block hiển thị `[REDACTED]` ổn định, không vỡ layout. | Chưa kiểm tra explicitly; nếu vỡ cần style/sanitization. |

### 1.2 Backend / Ops / Observability

| Mục | Mô tả | Trạng thái |
|-----|--------|------------|
| **Grafana/Prometheus dashboard** | Metrics Phase 3: `RiskEvaluationDuration`, `InsightsBatchSize` đã expose; chưa có dashboard Grafana tương ứng. | **Backlog** – phụ thuộc hạ tầng Grafana/Prometheus. |
| **Alerting rules** | VD: risk eval > X giây, batch size bất thường. | **Backlog** – phụ thuộc Prometheus/Alertmanager. |
| **Redis** (cache/WS scale-out) | Nếu sau này chuyển cache hoặc WS sang Redis. | **Backlog** – phụ thuộc hạ tầng Redis. |
| **Env trong Helm/ops** | `PCE_CLEANUP_RETENTION_DAYS`, `RISKS_WS_MAX_CONNS_PER_IP` đã dùng trong Core; cần document trong ops/helm và giá trị default/tuned theo môi trường (dev/stg/prod). | Một phần: có trong Verify-And-Deploy-Checklist và Core README; plan chi tiết: Risk-Center-Ops-Env.md (Phase 2). |

### 1.3 Test

| Mục | Mô tả | Trạng thái |
|-----|--------|------------|
| **E2E flow** | Luồng: agent → risk worker → insights → WS → dashboard UI (từ ingest đến hiển thị). | Chưa có E2E test cho flow này. |
| **UI-level test** | Cypress/Playwright cho các route `/risks/*`: bulk actions + delta WebSocket + cache invalidation; PCE metrics + view + cleanup không tạo "zombie state". | Chưa có. |
| **Histogram E2E** | TC GET /risk/histogram đã có trong script; có thể bổ sung assert UI (click bar → filter). | Đã có TC-05c; UI assert tùy chọn. |

### 1.4 Document

| Mục | Mô tả | Trạng thái |
|-----|--------|------------|
| **As-built vs design** | Đã thêm §10 trong RISK-CENTER-CONPONENT-OVERVIEW; có thể bổ sung thêm mục "Gaps" và "Data flows" (tham chiếu doc này). | Một phần xong. |
| **Bulk actions paragraph** | Dòng 124 overview còn đoạn thừa (curly-quote); cần xóa tay hoặc sed. | Chưa xóa. |
| **API/env/migrations Phase 3** | Ghi rõ tên API mới, env, migrations, tên metrics trong overview hoặc checklist. | Đã có trong checklist + overview §10; có thể gộp bảng env/metrics vào một nơi. |

---

## 2. Luồng logic chưa có dữ liệu / dữ liệu hạn chế

Các luồng dưới đây **phụ thuộc nguồn dữ liệu** hoặc **scope API hạn chế**. Khi nguồn không có hoặc chưa chạy, UI/API trả về rỗng hoặc 0.

### 2.1 Risk Trend (7 Days) – đã hỗ trợ all types (Phase 1)

- **API:** `GET /dashboard/metrics/threat-velocity` (GetThreatVelocity). Query param **`byType=all`** (default `vulnerability`).
- **Logic:** Khi `byType=all`: đếm mọi insight_type, group theo ngày + severity. Risk Center gọi với `byType=all`.
- **Trước đây:** Chỉ CVE → trend = 0 khi không có CVE. **Đã triển khai:** byType=all cho Risk Trend.

### 2.2 Dashboard stats: Total Risks & Critical – đã hỗ trợ all types (Phase 1)

- **API:** `GET /dashboard/stats` (GetDashboardStats). Query param **`byType=all`** (default `vulnerability`).
- **Logic:** Khi `byType=all`: đếm mọi insight_type cho Total Risks và Critical. Risk Center gọi với `byType=all`.
- **Ghi chú:** Resolved 24h và Affected Workloads không filter theo insight_type.

### 2.3 PCE (trend + heatmap)

- **Nguồn:** Bảng `pod_capabilities` (sync từ Agent).
- **API:** GET `/inventory/pod-capabilities/trends`, summary by namespace/severity (và các endpoint PCE khác).
- **Hệ quả:** Cluster chưa có agent hoặc agent chưa sync capabilities → PCE trend và heatmap rỗng.

### 2.4 Evidence tab – Runtime signals

- **API:** GET `/runtime/signals` (và /runtime/pods/:uid/signals cho drawer).
- **Nguồn:** Runtime pipeline (signals lưu DB).
- **Hệ quả:** Chưa có pipeline/source thu thập runtime signals → bảng Evidence tab rỗng.

### 2.5 Risks by cluster

- **API:** GET `/risk/insights/summary/by-cluster`.
- **Hệ quả:** Không có insights hoặc cluster chưa có pods/insights → số theo cluster = 0; vẫn trả về danh sách cluster với count 0.

### 2.6 Risk Score Distribution (histogram)

- **API:** GET `/risk/histogram`.
- **Logic:** Từ insights + join risk_scores (bin 0–10, 10–20, …, 90–100).
- **Hệ quả:** Không có insights hoặc chưa tính risk_scores cho resource → bins toàn 0.

### 2.7 Attack path graph

- **API:** GET `/graph/attack-paths/graph` (getAttackPathsGraph).
- **Hệ quả:** Fail hoặc không có dữ liệu graph → trả về `{ nodes: [], links: [] }`. Trong Risk Center drawer tab "PCE & Attack Path" chủ yếu hiển thị Related Capabilities + references (URL), không render graph attack path đầy đủ.

### 2.8 Audit Trail (drawer)

- **API:** getAuditLogs({ resource: 'insight', resourceId }).
- **Hệ quả:** Chỉ có bản ghi khi đã có action view/acknowledge/resolve/dismiss. Finding mới chưa ai xem → "No audit logs for this finding yet."

### 2.9 Risk scores / priority trên từng finding

- **Logic:** GET `/risk/insights?withScores=1` join `risk_scores` theo resource_uid → totalScore, priorityLevel.
- **Hệ quả:** Nếu chưa chạy scorer (POST `/risk/scores/:uid/calculate` hoặc job) → risk_scores trống → cột Risk Score và badge P0/P1 không có hoặc rỗng cho nhiều dòng.

---

## 3. Backlog (phụ thuộc hạ tầng) & Coming Soon

- **Backlog (không triển khai khi chưa có hạ tầng):** Grafana dashboard cho Risk metrics; Prometheus alerting rules; Redis (nếu chuyển cache/WS). Các tính năng gắn với chúng: **Coming Soon** (có thể nhắc trong UI/docs, không cam kết ship).
- **Coming Soon (tính năng liên quan):** Dashboard Risk metrics trong app (khi có Grafana/Prometheus); Alerting Risk; Sidebar filter 280px; Risk Score vs Capability Severity; MITRE ATT&CK mapping.
- **Plan chi tiết** để hoàn thành các công việc **không** phụ thuộc hạ tầng: **[Risk-Center-Implementation-Plan-Detailed.md](./Risk-Center-Implementation-Plan-Detailed.md)** (Phase 1–8, bắt đầu từ Data scope → Ops doc → PCE/Evidence UI → Export → UX → Test).

## 4. Tóm tắt ưu tiên (triển khai được ngay)

| Ưu tiên | Nhóm | Việc |
|--------|------|------|
| Cao | Data scope | Mở rộng Risk Trend và Dashboard stats (byType=all) – Phase 1. |
| Cao | Ops | Document env trong Risk-Center-Ops-Env.md; tham chiếu helm khi có – Phase 2. |
| Trung bình | UI | PCE: table drill-down + click heatmap → filter – Phase 3; Evidence: tab Audit Trail toàn cục – Phase 4. |
| Trung bình | UI | PCE Export Heatmap – Phase 5. |
| Trung bình | Test | E2E flow + byType=all – Phase 7. |
| Thấp | UX | [REDACTED] display + doc – Phase 6. |
| Backlog | Observability | Grafana dashboard + alerting – Backlog. |
| Backlog | UI | Sidebar 280px; Risk Score vs Capability; MITRE ATT&CK – Coming Soon. |

---

*Tài liệu cập nhật theo mã nguồn và docs Risk Center tại thời điểm rà soát. Khi triển khai thay đổi, cập nhật lại doc cho khớp.*
