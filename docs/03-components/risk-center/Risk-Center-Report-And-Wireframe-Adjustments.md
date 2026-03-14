# Risk Center – Báo cáo toàn bộ công việc đã thực hiện & Đề xuất điều chỉnh UX/UI, Wireframe

Tài liệu này: (1) báo cáo **toàn bộ công việc** đã thực hiện từ các finding và plan; (2) so sánh **wireframe vs as-built**; (3) **đề xuất điều chỉnh** UX/UI và wireframe cho phù hợp hiện trạng và backlog.

---

## 1. Tổng quan công việc đã thực hiện

### 1.1 Xuất phát từ findings & gaps

- **Finding ban đầu:** Tab/route Overview riêng; Quick Links; drawer dùng chung cross-route; đồng bộ scoring/priority trên UI; export/evidence/audit UX; verify & checklist; test; doc as-built.
- **Gaps doc:** Liệt kê PCE table drill-down, Export Heatmap, Evidence tab Audit Trail, Risk Trend/stats chỉ vulnerability, env/ops, backlog Grafana/Prometheus/Redis.
- **Plan chi tiết:** Backlog (Grafana, Prometheus, Redis) + Coming Soon; Phase 1–8 triển khai không phụ thuộc hạ tầng.

### 1.2 Công việc đã hoàn thành (theo phase)

| Phase | Nội dung | Chi tiết triển khai |
|-------|----------|----------------------|
| **Phase 1** | Data scope: Risk Trend & Dashboard stats (all insight types) | Core: GetThreatVelocity, GetDashboardStats thêm query param `byType=all` (default `vulnerability`). Dashboard: Risk Center gọi getThreatVelocity(7, clusterId, **'all'**), getStats(..., **'all'**). KPI và Risk Trend phản ánh toàn bộ findings (CVE + RBAC + misconfig). |
| **Phase 2** | Ops / Env documentation | Tạo **Risk-Center-Ops-Env.md** (PCE_CLEANUP_RETENTION_DAYS, RISKS_WS_MAX_CONNS_PER_IP, INSIGHTS_*; gợi ý dev/stg/prod). Verify-And-Deploy-Checklist link sang doc; ghi rõ copy sang helm khi có. |
| **Phase 3** | PCE: Heatmap click → filter table | Heatmap cells clickable (namespace × severity); click → set pceNamespace + pceSeverityFilter, gọi getPceCapabilities(namespace, severity), setPceDetails. Chip "Filtered by namespace X, severity Y" + nút Clear filter. Form drill-down thêm dropdown Severity; Run filters/Refresh truyền severity. |
| **Phase 4** | Evidence: Tab Audit Trail toàn cục | Trang /risks/evidence có 2 sub-tab: **Runtime Signals** (RuntimeSignalsTable + Capability Catalog) và **Audit Trail**. Audit Trail: getAuditLogs({ resource: 'insight', page, pageSize: 20 }); bảng User, Action, Finding ID, Timestamp, IP; pagination Previous/Next. AuditLog type + API mapping thêm resourceId, ip. |
| **Phase 5** | PCE Export Heatmap | Header tab PCE: tiêu đề "PCE Exposure" + nút **Export Heatmap**. Client-side CSV từ pceHeatmap (namespace, severity, count), download `pce-heatmap-YYYY-MM-DD.csv`; tooltip "Export heatmap data as CSV". |

### 1.3 Công việc đã có từ trước (được nhắc trong báo cáo)

| Hạng mục | Mô tả |
|----------|--------|
| **Overview-only /risks** | `/risks` chỉ KPI, Trend, Histogram (compact), Risk Level Overview, Risks by cluster, Quick Links (3 card), CTA "View all findings"; không có bảng. `/risks/findings` có thêm filter + bảng. |
| **Quick Links** | 3 card: View All Findings → `/risks/findings`, PCE Heatmap → `/risks/pce`, Evidence & References → `/risks/evidence`. |
| **Drawer global + ?insightId=** | Mở drawer từ URL `?insightId=<id>` (deep link); đóng xóa tham số; sync URL khi mở từ bảng Findings. |
| **Priority/score UX** | Sort mặc định score_desc; cột Level badge P0/P1; filter Priority P0–P4. |
| **Export CSV/PDF UX** | Loading spinner; tooltip "Max 10,000 rows. Current filters apply." |
| **Histogram** | GET /risk/histogram; Recharts; click bar → filter scoreBin; chip "Clear filter". |
| **Bulk actions** | POST /risk/insights/bulk; checkbox + Acknowledge/Resolve/Dismiss. |
| **Doc** | RISK-CENTER-CONPONENT-OVERVIEW §10 As-built; Risk-Center-Gaps-And-Data-Flows; Risk-Center-Implementation-Plan-Detailed; Risk-Center-Ops-Env. |

### 1.4 Chưa làm / Backlog

- **Phase 6:** Kiểm tra hiển thị [REDACTED] trong evidence JSON (chưa kiểm tra explicitly).
- **Phase 7:** E2E flow agent → worker → insights → WS → UI; E2E byType=all; UI test (Cypress/Playwright) → backlog.
- **Backlog:** Grafana dashboard, Prometheus alerting, Redis; Sidebar filter 280px; Risk Score vs Capability Severity; MITRE ATT&CK trong drawer.

---

## 2. So sánh Wireframe vs As-Built (chi tiết)

| Wireframe / Spec | As-built | Ghi chú |
|------------------|----------|--------|
| **Overview (/risks)** | ✅ | KPI 4 ô (Total, Critical, Resolved 24h, scope); Risk Trend 7 ngày; Histogram compact; Risk Level Overview (4 severity); Risks by cluster; Quick Links 3 card; CTA View all findings. Không có "Velocity" KPI riêng (chart nhỏ) – trend đã là chart. |
| **KPI Critical (P0)** | ✅ | Hiển thị số; badge P0/P1 ở cột Level bảng Findings. Wireframe ghi "87 (P0)" – có thể hiển thị thêm số P0 trên KPI nếu cần. |
| **Risk Trend click → filter theo ngày** | Một phần | Trend có; click point filter theo ngày (selectedChartDate → sinceMinutes) đã có logic trong code; có thể cần kiểm tra UX. |
| **Histogram click → filter** | ✅ | Click bar → navigate /risks/findings (nếu đang Overview) + set selectedScoreBin, refetch; chip "Clear filter". |
| **Quick Links** | ✅ | 3 card trên Overview; navigate đúng route. |
| **Findings: Scope bar, Export CSV/PDF** | ✅ | Cùng dòng Total findings, Cluster, Time; Export CSV/PDF + loading + tooltip. |
| **Findings: Filters inline** | ✅ | Severity, Workflow (status), Namespace, Type, Search, Priority P0–P4, Sort (mặc định Score high to low). |
| **Findings: Sidebar 280px** | ❌ Backlog | Wireframe "phiên bản nâng cao"; hiện filter inline. |
| **Findings: Bulk** | ✅ | Checkbox + Acknowledge / Resolve / Dismiss selected; POST /risk/insights/bulk. |
| **Drawer: 3 tab + bottom bar** | ✅ | Summary, Evidence & Audit, PCE & Attack Path; Acknowledge / Resolve / Dismiss. Không có nút Export trong drawer (wireframe có "Export"). |
| **Drawer: mở từ URL / deep link** | ✅ | ?insightId=<id> từ bất kỳ tab. |
| **PCE: Header + Export Heatmap** | ✅ | "PCE Exposure" + nút Export Heatmap (CSV). |
| **PCE: Trend 7 days** | ✅ | AreaChart (wireframe ghi Line Chart – as-built dùng Area). |
| **PCE: Heatmap + Table** | ✅ | Heatmap namespace × severity; bảng drill-down (Pod Name, UID, Namespace, Capability, Severity). **Thiếu cột Evidence, Last Seen** trong bảng (wireframe có). |
| **PCE: Click cell heatmap → filter table** | ✅ | Click → set namespace + severity, refetch table; chip + Clear. |
| **PCE: Hover pod → tooltip "Open Pod Detail"** | Chưa | Có thể thêm tooltip + link sang /resources/pods/uid/:uid. |
| **Evidence: Tabs Runtime Signals \| Audit Trail** | ✅ | 2 sub-tab; Audit Trail: User, Action, Finding ID, Timestamp, IP; pagination. |
| **Evidence: Export từng tab** | Chưa | Wireframe "Export: Nút riêng cho từng tab (khi có)". |
| **Risk Score vs Capability Severity** | ❌ Coming Soon | Wireframe "(Future)". |
| **MITRE ATT&CK trong drawer** | ❌ Coming Soon | Wireframe "nếu có". |

---

## 3. Đề xuất điều chỉnh UX/UI và Wireframe

### 3.1 Cập nhật wireframe (doc) cho khớp as-built

- **§1 Overview:** Ghi rõ "Velocity" = Risk Trend chart (không cần KPI card thứ 4 riêng "Chart nhỏ" nếu đã có Trend toàn dải). Hoặc giữ 4 KPI: Total, Critical, Resolved 24h, và ô thứ 4 là "Trend" (link tới chart) thay vì "Velocity" chart nhỏ.
- **§2 Findings:** Giữ "Sidebar filter (fixed 280px) có thể dùng trong phiên bản nâng cao" → đánh dấu **Backlog / Coming Soon**.
- **§3 PCE:**  
  - Ghi rõ: "Click cell heatmap → filter table bên dưới" **đã triển khai**.  
  - Bảng drill-down as-built: **Namespace, Pod Name, Pod UID, Capability, Severity** (chưa có cột Evidence, Last Seen). Đề xuất wireframe: hoặc bổ sung cột Evidence (truncate), Last Seen khi API/backend có; hoặc ghi trong Implementation Notes "Columns Evidence, Last Seen – optional / when available".  
  - "Export Heatmap" **đã có** (CSV).
- **§4 Evidence:** Ghi rõ "Tab Audit Trail" **đã triển khai** (User, Action, Finding ID, Timestamp, IP; pagination). "Export: Nút riêng cho từng tab (khi có)" → **Optional / backlog**.
- **§5 Drawer:** Ghi "Export" trong bottom bar là **optional** (chưa có trong as-built); hoặc thêm nút Export (single finding) khi cần.

### 3.2 Điều chỉnh UX/UI đề xuất (code / product)

| Đề xuất | Mức độ | Mô tả |
|--------|--------|--------|
| **PCE table: cột Evidence, Last Seen** | Tùy chọn | Nếu backend/API trả evidence (masked) và last_seen_at cho từng dòng PCE, thêm 2 cột (Evidence truncate, Last Seen); nếu chưa có data thì bỏ qua hoặc ẩn. |
| **PCE: Link Pod → Pod Detail** | Nên có | Cột Pod Name (hoặc Pod UID) có link tới `/resources/pods/uid/:uid` và tooltip "Open Pod Detail". |
| **Drawer: nút Export (single)** | Tùy chọn | Bottom bar thêm "Export" (export 1 finding dạng PDF/HTML) nếu product cần. |
| **Evidence tab: Export Runtime / Export Audit** | Tùy chọn | Mỗi sub-tab có nút Export (CSV) khi có dữ liệu; backlog nếu chưa ưu tiên. |
| **Overview KPI: hiển thị số P0** | Tùy chọn | Nếu muốn khớp wireframe "Critical 87 (P0)", có thể thêm (p0Count) từ summary/histogram. |
| **[REDACTED] trong evidence JSON** | Nên kiểm tra | Phase 6: đảm bảo chuỗi [REDACTED] không làm vỡ layout (monospace, word-break); cập nhật doc sau khi kiểm tra. |
| **Empty state thống nhất** | Polish | Các bảng/tab không có dữ liệu: message rõ ràng (vd "No findings in this scope", "No audit logs yet") và (nếu cần) CTA ngắn. |

### 3.3 Điều chỉnh doc Wireframe (Implementation Notes §7)

- Cập nhật bảng "Wireframe / Design | Hiện trạng (as-built)" trong **Risk-Center-UI_UX-Wireframe-Specification.md** §7:
  - **Quick Links:** Đã có block 3 card trên Overview.
  - **Overview vs Findings:** /risks chỉ Overview (KPI, Trend, Histogram, Quick Links, CTA); bảng chỉ ở /risks/findings.
  - **PCE:** Heatmap click → filter table **đã có**; Export Heatmap **đã có** (CSV). Bảng drill-down có Severity filter; chưa cột Evidence, Last Seen (optional).
  - **Evidence:** Tab Audit Trail toàn cục **đã có** (Runtime Signals | Audit Trail).
  - **Data scope:** Risk Trend và KPI dùng byType=all (all insight types).

---

## 4. Cập nhật Gaps doc (đánh dấu đã xong)

Trong **Risk-Center-Gaps-And-Data-Flows.md** §1.1 nên sửa:

- **PCE view layout:** "Click cell heatmap → filter table" **đã triển khai** (Phase 3). Bảng drill-down có; cột Evidence, Last Seen optional (khi API có).
- **PCE Export Heatmap:** **Đã có** (Phase 5).
- **Evidence view: tab Audit Trail:** **Đã có** (Phase 4) – sub-tab Audit Trail trên /risks/evidence.

---

## 5. Tóm tắt

- **Đã thực hiện:** Phase 1 (byType=all), Phase 2 (Ops env doc), Phase 3 (PCE heatmap click + chip + Severity), Phase 4 (Evidence sub-tab Audit Trail), Phase 5 (Export Heatmap CSV); cùng toàn bộ tính năng Overview/Quick Links/drawer ?insightId=/priority/export UX đã có từ trước.
- **Wireframe vs as-built:** Khớp phần lớn; còn lệch nhỏ: sidebar 280px (backlog), PCE bảng chưa cột Evidence/Last Seen (optional), Drawer chưa nút Export, Evidence chưa nút Export từng tab (optional).
- **Đề xuất:** (1) Cập nhật wireframe doc §7 và các §1–5 theo as-built và optional/backlog; (2) UX: link Pod → Pod Detail, (tuỳ chọn) cột Evidence/Last Seen, Export single/tab, kiểm tra [REDACTED]; (3) Cập nhật Gaps doc đánh dấu PCE heatmap click, Export Heatmap, Evidence Audit Trail đã xong.

---

*Báo cáo phản ánh mã nguồn và docs tại thời điểm rà soát. Khi triển khai thêm, cập nhật lại báo cáo và wireframe.*
