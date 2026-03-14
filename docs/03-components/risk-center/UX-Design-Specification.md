# Fortuna – Risk Center UI/UX Design Specification
Document Version: 1.0
Review Date: March 12, 2026
Reviewer: Senior Software Engineer and System Architect
Project: Fortuna – Kubernetes Security Asset Management (KSAM) Platform
Dựa trên phân tích toàn diện từ tài liệu bạn cung cấp (Risk-Center-Overview-Answer.md), tôi đã làm rõ và thiết kế chi tiết UI/UX cho Risk Center. Hiện tại Risk Center chỉ là 1 trang duy nhất (/risks) với 3 tab.
Tôi đề xuất tối ưu hóa thành 4 trang chính + 1 drawer để nâng cao trải nghiệm người dùng (SecOps, Platform Engineer, Compliance Officer), giảm cognitive load, hỗ trợ drill-down nhanh và real-time. Thiết kế tuân thủ Material Design 3 + Tailwind (đang dùng trong Dashboard), dark mode mặc định, responsive 100% (mobile-first cho on-call).
1. Tổng quan cấu trúc UI/UX (4 trang chính + 1 drawer)
Số trang cần thiết: 4 trang chính (không phải micro-frontend, vẫn nằm trong Dashboard React):

Risk Center Overview (/risks) – Trang chính, landing page
Risk Findings (/risks/findings) – Danh sách chi tiết + filter mạnh
PCE Exposure (/risks/pce) – Visualization chuyên sâu cho Pod Capability
Evidence & Audit (/risks/evidence) – Supporting evidence + audit trail

+ 1 Risk Detail Drawer (side panel, không phải page riêng) mở từ bất kỳ trang nào khi click finding.
2. Chi tiết từng trang – Thông tin hiển thị & Giao diện thiết kế
Trang 1: Risk Center Overview (/risks) – Dashboard-style landing
Mục đích: Tổng quan nhanh, Threat Velocity, KPI chính → click vào bất kỳ card nào để drill-down.
Giao diện (Desktop layout – 3 cột):

Header: Breadcrumb “Security → Risk Center”, Cluster selector (multi-select), Time window (7/30/90 days), Export All button (CSV/PDF).
Top row – KPI Cards (4 cards ngang):
Total Active Risks (số + % change 24h)
Critical Risks (P0)
Resolved in 24h
Threat Velocity (7 days chart nhỏ)

Middle row – 2 charts lớn:
Risk Trend (7 days) – AreaChart Recharts (stacked severity)
Risks by Cluster – Horizontal Bar Chart

Bottom row – Quick Links:
Card “View All Findings” → chuyển sang /risks/findings
Card “PCE Exposure Heatmap” → /risks/pce
Card “Evidence & Signals” → /risks/evidence


Mobile: Stack cards + charts scroll ngang.
Trang 2: Risk Findings (/risks/findings) – Bảng chi tiết mạnh nhất
Thông tin hiển thị:

Cột: ID, Severity (color badge), Priority (P0–P4 pill), Risk Score (0-100), Title, Resource (pod/sa/role + link), Detected At, Status (active/resolved), Actions (Acknowledge/Resolve/Dismiss).
Filter sidebar (fixed left): Severity, Priority, Status, Cluster, Namespace, Search, Date range, Rule Type.
Bulk actions: Select multiple → Acknowledge All / Resolve All / Export Selected.

Giao diện:

Data table (TanStack Table) với infinite scroll hoặc pagination 20/50/100.
Click row → mở Risk Detail Drawer (70% width).
Right panel mini: Top 5 related PCE + Evidence preview.

Trang 3: PCE Exposure (/risks/pce) – Visualization chuyên sâu
Thông tin hiển thị:

PCE Trend (7 days) – Line chart (Recharts) theo severity.
Exposure Heatmap – Namespace × Severity (color intensity theo số capability).
Bảng drill-down: Namespace | Pod | Capability | Severity | Evidence count | Last Seen.
Filter: Severity, Namespace, Capability Group.

Giao diện:

2 phần ngang: Chart trên (60%), Heatmap + bảng dưới.
Hover heatmap cell → tooltip chi tiết capabilities.
Click pod → mở Risk Detail Drawer với tab PCE focus.

Trang 4: Evidence & Audit (/risks/evidence)
Thông tin hiển thị:

Bảng Runtime Signals (type, category, pod, timestamp, severity).
Audit Trail table: User | Action (view/ack/resolve) | Finding ID | Timestamp.
Search + filter theo finding hoặc pod.

Giao diện: Tabbed table (Signals | Audit Log). Export riêng từng tab.
Risk Detail Drawer (mở từ mọi trang)
Cấu trúc 3 tab bên trong drawer:

Summary: Title, Severity, Score, Priority, Description, Recommendation.
Evidence & Correlation: JSON formatted đẹp (cards) + link Pod Detail + Runtime Signals list.
PCE & Attack Path: Related capabilities + MITRE links (nếu có).

Nút: Acknowledge / Resolve / Dismiss / Export single.
3. Thiết kế tổng thể & UX Best Practices áp dụng

Color system:
Critical: #ef4444, High: #f59e0b, Medium: #eab308, Low: #84cc16.
Priority P0: red badge, P4: gray.
Real-time feedback: WebSocket badge “Live” (xanh) khi có update; toast “1 new critical finding” + auto refresh affected rows.
Accessibility: ARIA labels đầy đủ, keyboard navigation, high contrast (WCAG 2.1 AA).
Performance: All tables dùng virtualized list (react-window); cache GET requests 30s.
Responsive:
Desktop: 3-column
Tablet: 2-column
Mobile: Vertical stack + bottom sheet cho drawer.
Dark mode: Đã hỗ trợ (slate-950 background, slate-100 text).

4. Recommendation triển khai (ưu tiên)
Giai đoạn 1 (1 sprint):

Giữ 1 trang hiện tại (/risks) nhưng tách thành 3 tabs rõ ràng hơn + thêm Risk Detail Drawer.

Giai đoạn 2 (2 sprint):

Tách thành 4 route riêng như trên + implement delta WebSocket (chỉ update rows thay đổi).

Giai đoạn 3:

Thêm heatmap tương tác + bulk actions.

Tổng effort ước tính: 3-4 tuần (2 dev Frontend + 1 Backend).