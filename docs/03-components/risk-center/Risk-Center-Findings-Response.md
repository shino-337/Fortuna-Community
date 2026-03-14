# Risk Center – Phản hồi Findings & Trạng thái

Tài liệu này phản hồi từng finding, sửa nhầm lẫn (Finding #2 – Histogram đã có), và ghi trạng thái/roadmap.

---

## Finding #1: PCE Table Drill-down – Thiếu cột Evidence & Last Seen

| Mục | Nội dung |
|-----|----------|
| **Mô tả** | Wireframe yêu cầu Evidence (truncate) và Last Seen; as-built trước đây chỉ có Namespace \| Pod \| Capability \| Severity. |
| **Root cause** | Backend DTO chưa trả `lastSeenAt`; frontend chưa hiển thị Evidence/Last Seen. |
| **Trạng thái** | **Đã xử lý** |
| **Hành động đã làm** | (1) **Backend:** `PodCapabilityDTO` thêm field `LastSeenAt` (RFC3339); `GetPodCapabilitiesList` và `mapPodCapabilities` gán từ `cap.LastSeenAt`. (2) **Frontend:** type `PodCapabilityDetail` thêm `lastSeenAt?`; bảng PCE drill-down thêm 2 cột: **Evidence** (truncate 100 ký tự + tooltip full JSON) và **Last Seen** (relative: "Just now", "5m ago", "2d ago" hoặc date; fallback `updatedAt` nếu không có lastSeenAt). |

---

## Finding #2: Histogram Risk Score Distribution “Chưa triển khai”

| Mục | Nội dung |
|-----|----------|
| **Mô tả (trong finding)** | "as-built hiện chưa thấy component Recharts nào cho histogram". |
| **Thực tế** | **Histogram đã được triển khai.** API `GET /api/v1/risk/histogram` (GetRiskHistogram) và component `RiskHistogram` (Recharts BarChart stacked) đã có; dùng trong Insights.tsx ở Overview (compact) và Findings (full width); click bar → filter bảng theo scoreBin; chip "Clear filter". |
| **Trạng thái** | **Không cần làm** – đã có sẵn. |
| **Hành động** | Cập nhật finding/checklist để không coi Histogram là "missing"; nếu cần có thể bổ sung E2E assert cho histogram (Phase 7). |

---

## Finding #3: Export Single Finding trong Drawer & Export từng Tab Evidence

| Mục | Nội dung |
|-----|----------|
| **Mô tả** | Wireframe: nút Export trong drawer bottom bar; export riêng từng tab Evidence (Signals \| Audit). |
| **Trạng thái** | **Backlog / chưa làm** |
| **Đề xuất** | (1) Backend: GET `/risk/insights/:id/export?format=pdf` (hoặc HTML) khi cần. (2) Drawer: thêm nút "Export" (single finding PDF/HTML). (3) Evidence tab: mỗi sub-tab thêm "Export CSV". Effort 1–2 ngày. |

---

## Finding #4: Sidebar Filter 280px

| Mục | Nội dung |
|-----|----------|
| **Mô tả** | Wireframe "phiên bản nâng cao"; as-built dùng inline filter. |
| **Trạng thái** | **Backlog** (đã thống nhất trước đó). |
| **Đề xuất** | Khi làm: component RiskFilterSidebar (fixed 280px desktop, drawer mobile); sync state với URL; toggle "Advanced Filters". Effort 2–3 ngày. |

---

## Finding #5: Empty State & Loading UX

| Mục | Nội dung |
|-----|----------|
| **Mô tả** | Empty/loading state chưa thống nhất, message chưa rõ. |
| **Trạng thái** | **Một phần** – các bảng đã có message "No matching...", "No audit logs..."; chưa có component EmptyState chung + skeleton. |
| **Đề xuất** | Thêm EmptyState thống nhất (icon + message + CTA) và loading skeleton cho bảng; effort ~0.5 ngày. |

---

## Finding #6: Backlog Hạ tầng (Grafana/Prometheus/Redis)

| Mục | Nội dung |
|-----|----------|
| **Đánh giá** | Đúng – backlog tách rõ, không lẫn sprint. |
| **Đề xuất** | Thêm placeholder trong Risk Center Overview (hoặc Settings): card "Advanced Metrics & Alerting – Coming Soon with Prometheus/Grafana integration" (gray, non-clickable). Effort minimal. |

---

## Finding #7: E2E Flow Chưa Test Đầy Đủ

| Mục | Nội dung |
|-----|----------|
| **Trạng thái** | **Backlog** (Phase 7). |
| **Đề xuất** | Script bash: trigger NATS → sleep → curl GET /risk/insights → assert. Hoặc Cypress: login → /risks → assert count. Effort 2–3 ngày khi có thời gian. |

---

## Finding #8: [REDACTED] trong Evidence JSON (Phase 6)

| Mục | Nội dung |
|-----|----------|
| **Mô tả** | Chưa kiểm tra hiển thị [REDACTED] không vỡ layout. |
| **Trạng thái** | **Chưa làm** |
| **Đề xuất** | Trong drawer Evidence tab: dùng `<pre class="whitespace-pre-wrap break-words font-mono text-sm">` cho JSON; test với mock chứa [REDACTED]. Effort ~0.25 ngày. |

---

## Tổng kết & Roadmap

| Ưu tiên | Finding | Trạng thái | Ghi chú |
|--------|---------|------------|---------|
| — | #1 PCE Evidence + Last Seen | **Done** | Backend DTO + FE columns. |
| — | #2 Histogram | **Đã có sẵn** | Không cần implement; chỉ cập nhật doc. |
| Cao | #3 Export single + Export tab | Backlog | 1–2 ngày. |
| Cao | #5 Empty state polish | Một phần | 0.5 ngày. |
| Trung bình | #6 Coming Soon card | Optional | Minimal. |
| Trung bình | #4 Sidebar 280px | Backlog | 2–3 ngày. |
| Trung bình | #7 E2E | Backlog | 2–3 ngày. |
| Thấp | #8 [REDACTED] UX | Chưa | 0.25 ngày. |

**Đã hoàn thành trong đợt này:** Finding #1 (PCE Evidence + Last Seen). Finding #2 được làm rõ là đã có (Histogram).

---

*Tài liệu cập nhật theo findings và mã nguồn tại thời điểm rà soát.*
