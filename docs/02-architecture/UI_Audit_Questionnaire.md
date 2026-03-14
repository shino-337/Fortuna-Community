1. Technology Stack

Cần hiểu nền tảng UI hiện tại, vì cách refactor sẽ phụ thuộc vào framework.

Hãy cung cấp:

- Frontend framework đang dùng
- Styling system
- Component library hiện có
- Có Design Token / Theme system không?

### Trả lời

- **Frontend framework**: React SPA, routing bằng `react-router-dom`.
- **Styling**: Tailwind CSS + bộ component UI custom (`dashboard/components/ui/*`). Không dùng Ant Design / MUI.
- **Theme / tokens**: Sử dụng alias Tailwind (`text-text`, `bg-surface`, `border-border`…), chưa có file riêng `colors.ts`/`typography.ts` nhưng pattern theme tối đã rõ ràng.

---

2. Typography System

Cung cấp thông tin font hiện tại:

- Font family
- Font sizes
- Line height
- Font weight
- Vấn đề typography bạn thấy

### Trả lời

- **Font family**: sans-serif global (khả năng Inter/system), `font-mono` cho ID/IP/command.
- **Font sizes tiêu biểu**:
  - `text-[10px]`: label rất nhỏ (Service Account, badge phụ).
  - `text-[11px]`: badge severity nhỏ (Related Risks).
  - `text-xs` (~12px): section title uppercase, helper text, tooltip.
  - `text-sm` (~14px): body chính trong card, table.
  - `text-lg` (~18px): card title như “Overview”, “Runtime metrics”.
  - `text-2xl` (~24px): page title (“Dashboard”, “Risk Center”, tên Pod).
- **Font weight**: 400 (body), 500 (nhấn nhẹ), 600 (heading nhỏ, badge), 700 (page title, số KPI).
- **Vấn đề**:
  - Trước: page title không đồng nhất (PodDetail để trống). Đã chỉnh thành `title="Pod Detail"`.
  - Card title/section heading trộn `text-lg` và `text-xs uppercase`; đã chuẩn hoá dần (Related / Related Risks về section nhỏ).
  - Badge size rải rác (11–13px); đang dần đưa về `text-[11px]`/`text-xs` + `getSeverityBadgeClass`.

---

3. Layout System

Cho biết layout đang tổ chức như thế nào:

- Page container / width
- Page padding
- Grid system

### Trả lời

- **Page container**: `PageLayout` bọc header (title/description/actions) + toolbar optional + content (`max-w-full`).
- **Page width**: full width; lưới và card quyết định nội dung thực tế.
- **Padding**:
  - Cấp page: `space-y-6` giữa section.
  - Card: `p-6` cho card chính; `p-3`–`p-4` cho card phụ.
- **Grid**: CSS Grid + flexbox;
  - KPI: `grid grid-cols-2 md:grid-cols-4 gap-4`.
  - Overview Pod: `grid grid-cols-1 md:grid-cols-2 gap-6`.
  - Bảng trong card với `overflow-x-auto`.

---

4. Spacing / Padding

Hãy liệt kê spacing, và vấn đề thường gặp.

### Trả lời

- **Spacing dùng nhiều**:
  - Gap: `gap-1`, `gap-2`, `gap-3`, `gap-4`, `gap-6`.
  - Padding card: `p-3`, `p-4`, `p-6`.
  - Cell table: `px-3 py-2`; button/tab: `px-4 py-2`.
  - Margin: `mb-2`, `mb-4`, `mb-6`, `mt-4`, `mt-6`.
- **Lỗi & fix**:
  - Tabs Pod/Cluster: `flex ... w-fit` → co theo nội dung, không đẹp; đã đổi thành `inline-flex flex-wrap ...` (bỏ `w-fit`).
  - Card “Related”/“Related Risks” dùng `p-6` + title lớn → nặng; đã thu nhỏ heading (section label) và dùng `p-4` cho phần navigation.
  - Risk Score Distribution card width cứng `max-w-[480px]` → lệch so với Trend; đã chuyển sang grid 2 cột Trend (2 phần) + Histogram (1 phần).

---

5. Card / Panel Structure

Card trong hệ thống hiện tại:

- Card padding / border radius / shadow
- Cấu trúc card

### Trả lời

- **Styling**:
  - Background: `bg-slate-900/50` hoặc `bg-slate-900/70`.
  - Border: `border border-slate-800`.
  - Radius: `rounded-lg` (đa số), đôi khi `rounded-xl` cho icon.
- **Cấu trúc**:
  - Section nhỏ: heading `text-xs uppercase`, body `text-sm`/`text-xs`.
  - Card chính: `h3 text-lg font-semibold text-white mb-4` + `dl` hoặc table `text-sm`.
- **Vấn đề**: một số card phụ (vd. “Related” PodDetail) dùng `text-lg` như content chính → đã chuyển sang heading section nhỏ để rõ hierarchy.

---

6. Component Sizing

Cho biết kích thước Buttons, Inputs, Icons.

### Trả lời

- **Buttons**:
  - Default: `px-4 py-2` (~36–40px); `size="sm"` nhỏ hơn (~32–36px).
  - Icon: `w-4 h-4`.
- **Input/Select**:
  - `py-2 px-4`, `text-sm`, border `border-slate-800`, focus `focus:border-pink-500/50`.
- **Icons**:
  - Tab/title: `w-4 h-4`.
  - Section head: `w-5 h-5`.
- **Table**:
  - Row: `py-2` với `text-sm`; header `font-medium` + `bg-slate-800/80` hoặc `text-slate-400 border-b`.

---

7. Page Types

Các loại page và layout:

### Trả lời

- **Dashboard** (`Dashboard.tsx`):
  - `PageLayout` + header; grid KPI; chart trend; list top risks; PCE summary.
- **Risk Center** (`Insights.tsx`):
  - 4 routes: `/risks`, `/risks/findings`, `/risks/pce`, `/risks/evidence`.
  - Overview: KPI row, Trend + Histogram grid, Risk Level Overview, Risks by cluster, Quick Links.
  - Findings: Histogram + filter bar + table findings.
  - PCE: trend + heatmap + table.
  - Evidence: Runtime Signals + Audit Trail.
- **List pages**: Agents, Resources (Pods), Risk Rules – pattern: `PageLayout` + filter bar + table.
- **Detail pages**:
  - `PodDetail`: header (tên pod + badge risk), tabs (overview, SBOM, related risks, metrics, processes, network, events, spec), footer “Related navigation”.
  - `ClusterDetail`: overview, inventory, agents, security.
  - `RiskDetail`: Summary / Evidence & Audit / PCE & Attack Path.
- **Settings / Forms**: Risk rules, cấu hình – card + form, không sidebar.

---

8. Known UI Problems

### Trả lời

- Page title không đồng nhất (đã chỉnh PodDetail).
- Card title vs section heading chưa phân cấp rõ (đang chuyển dần sang pattern: page title 2xl, card title lg, section label xs uppercase).
- Card spacing không đều giữa card chính/phụ (đang chuẩn hoá p-6 vs p-4).
- Layout Risk Score Distribution lệch so với Trend (đã sửa thành grid 2 cột).
- Tabs Pod/Cluster dùng `w-fit`, wrap kém và không scale (đã sửa `inline-flex flex-wrap`).
- Badge + icon size chưa hoàn toàn đồng nhất, nhưng đa số đã về 16–20px icons + badge text 11–12px.

---

9. Screenshots

Chưa lưu trực tiếp trong repo; có thể tự lấy từ:

- Dashboard (`/`).
- Risk Center (`/risks`, `/risks/findings`).
- Pod Detail (`/resources/pods/uid/:uid`).
- Cluster Detail (`/clusters/:id`).

---

10. Code Example

### Trả lời

- **Card section chuẩn**:

```tsx
<Card className="p-6">
  <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">
    Risk Level Overview (Active Findings)
  </h3>
  <p className="text-xs text-slate-500 mb-2">
    Same scope as "Total findings" above: Risk Findings (insights) only.
  </p>
  {/* content */}
</Card>
```

- **Tabs chuẩn (Pod/Cluster Detail)**:

```tsx
<div className="inline-flex flex-wrap gap-1 p-1 bg-slate-900/80 rounded-lg border border-slate-800 mb-6">
  {tabs.map(({ id, label, icon }) => (
    <button
      key={id}
      onClick={() => setActiveTab(id)}
      className={clsx(
        'flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors',
        activeTab === id
          ? 'bg-pink-600 text-white shadow'
          : 'text-slate-400 hover:bg-slate-800 hover:text-white'
      )}
    >
      {icon}
      {label}
    </button>
  ))}
</div>
```

---

## Wireframe Layout cho các page chính (as-built)

Các wireframe dưới đây mô tả layout tổng thể (ASCII), dùng để align các page theo hệ thống chung.

### A. Dashboard (`/`)

```text
┌───────────────────────────────────────────────────────────────┐
│ Header: Dashboard        [View All Risks button]             │
├───────────────────────────────────────────────────────────────┤
│ KPI Row (4 cards)                                            │
│ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐  │
│ │ Clusters   │ │ Pods       │ │ Agents     │ │ Risks      │  │
│ └────────────┘ └────────────┘ └────────────┘ └────────────┘  │
├───────────────────────────────────────────────────────────────┤
│ Trend + PCE Chart (Velocity)                                 │
│ [Combined line chart: Risk count vs PCE count, 7 days]       │
├───────────────────────────────────────────────────────────────┤
│ 2 Columns (Top Risks | PCE Summary)                          │
│ ┌──────────────────────┐  ┌───────────────────────────────┐  │
│ │ Top Critical Risks   │  │ PCE Exposure Summary          │  │
│ └──────────────────────┘  └───────────────────────────────┘  │
└───────────────────────────────────────────────────────────────┘
```

### B. Risk Center Overview (`/risks`)

```text
┌───────────────────────────────────────────────────────────────┐
│ Header: Risk Center         [Tabs: Risk Findings | PCE | ...]│
├───────────────────────────────────────────────────────────────┤
│ KPI Row (4 cards) – Total Risks | Critical | Resolved 24h |  │
│ Velocity                                                         │
├───────────────────────────────────────────────────────────────┤
│ 2-Column Grid: Risk Trend (2/3) + Histogram (1/3)            │
│ ┌──────────────────────────────┐ ┌─────────────────────────┐ │
│ │ Risk Trend (AreaChart)      │ │ Risk Score Distribution │ │
│ └──────────────────────────────┘ └─────────────────────────┘ │
├───────────────────────────────────────────────────────────────┤
│ Risk Level Overview (4 severity cards)                       │
│ [Critical] [High] [Medium] [Low]                            │
├───────────────────────────────────────────────────────────────┤
│ Risks by Cluster Table                                       │
├───────────────────────────────────────────────────────────────┤
│ Quick Links (3 cards)                                        │
└───────────────────────────────────────────────────────────────┘
```

### C. Risk Findings (`/risks/findings`)

```text
┌───────────────────────────────────────────────────────────────┐
│ Header: Risk Findings  (same tabs)                           │
├───────────────────────────────────────────────────────────────┤
│ Risk Score Distribution (Histogram full width)               │
├───────────────────────────────────────────────────────────────┤
│ Filter Bar (inline)                                          │
│ [Severity] [Priority] [Status] [Namespace] [Type] [Search]   │
├───────────────────────────────────────────────────────────────┤
│ Bulk Toolbar + Table                                         │
│ [Select/Bulk actions]                                        │
│ ┌─────────────────────────────────────────────────────────┐  │
│ │ Table: Severity | Priority | Score | Title | Resource  │  │
│ └─────────────────────────────────────────────────────────┘  │
│ Pagination                                                   │
└───────────────────────────────────────────────────────────────┘
```

### D. PCE Exposure (`/risks/pce`)

```text
┌───────────────────────────────────────────────────────────────┐
│ Header: PCE Exposure (tab Risk Center)                       │
├───────────────────────────────────────────────────────────────┤
│ PCE Trend 7 Days (line chart)                               │
├───────────────────────────────────────────────────────────────┤
│ Split 50/50: Heatmap + Table                                │
│ ┌───────────────────┐  ┌─────────────────────────────────┐ │
│ │ Heatmap (ns x sev)│  │ Drill-down table                │ │
│ └───────────────────┘  └─────────────────────────────────┘ │
└───────────────────────────────────────────────────────────────┘
```

### E. Evidence & Audit (`/risks/evidence`)

```text
┌───────────────────────────────────────────────────────────────┐
│ Header: Evidence & References (tab Risk Center)              │
├───────────────────────────────────────────────────────────────┤
│ Sub-tabs: [Runtime Signals] [Audit Trail]                    │
├───────────────────────────────────────────────────────────────┤
│ Filter bar (search, type, category, date range, sort)       │
├───────────────────────────────────────────────────────────────┤
│ Content (cards list or table)                               │
│ Runtime: stacked cards with signal, category, confidence,   │
│          evidence (expandable JSON).                        │
│ Audit: table with User | Action | Finding | Timestamp | IP. │
└───────────────────────────────────────────────────────────────┘
```

### F. Pod Detail (`/resources/pods/uid/:uid`)

```text
┌───────────────────────────────────────────────────────────────┐
│ Header (PageLayout): Pod Detail   [Back to Resources]        │
├───────────────────────────────────────────────────────────────┤
│ Hero Row:                                                     │
│ [Icon] Pod name   [Risk badge]                               │
│ Namespace | Node                                             │
│ Right: [Export SBOM]                                         │
├───────────────────────────────────────────────────────────────┤
│ Overview Stat Row (3 cards) – Status, Risk count, etc.       │
├───────────────────────────────────────────────────────────────┤
│ Tabs (pill group)                                            │
│ [Overview] [SBOM] [Related Risks] [Runtime metrics] ...      │
├───────────────────────────────────────────────────────────────┤
│ Tab content:                                                 │
│ - Overview: 2-column grid card (identity + ownership)        │
│ - SBOM: filter bar + table/list                              │
│ - Related Risks: list of insights (severity badge)           │
│ - Metrics/Processes/Network/Events: table per tab            │
│ - Spec: YAML viewer                                          │
├───────────────────────────────────────────────────────────────┤
│ Footer card: Related navigation (View cluster, node, risks…) │
└───────────────────────────────────────────────────────────────┘
```

### G. Cluster Detail (`/clusters/:id`)

```text
┌───────────────────────────────────────────────────────────────┐
│ Header: Cluster Detail   [Back]                              │
├───────────────────────────────────────────────────────────────┤
│ KPI Row (4 cards): Pods | Deployments | Risks | Agents       │
├───────────────────────────────────────────────────────────────┤
│ Tabs (pill group):                                           │
│ [Overview] [Inventory] [Agents] [Security]                   │
├───────────────────────────────────────────────────────────────┤
│ Tab content:                                                 │
│ - Overview: card với grid thông tin chung (ID, status, ver,  │
│   counts).                                                   │
│ - Inventory: 2-column grid (nodes list, namespaces list).    │
│ - Agents: table Agents (node, status, heartbeat, version).   │
│ - Security: grid severity cards + nút View all risks.        │
└───────────────────────────────────────────────────────────────┘
```

### H. Risk Detail (`/risks/:id`)

```text
┌───────────────────────────────────────────────────────────────┐
│ Header: Risk Detail – [Severity badge] Title                 │
├───────────────────────────────────────────────────────────────┤
│ Tabs: [Summary] [Evidence & Audit] [PCE & Attack Path]       │
├───────────────────────────────────────────────────────────────┤
│ Summary:                                                      │
│ - Left: description / risk_explanation, recommendation,      │
│   remediation steps.                                         │
│ - Right: affected resources list, quick actions,             │
│   context links (View pod, View cluster, View rule).         │
├───────────────────────────────────────────────────────────────┤
│ Evidence & Audit: runtime signals + audit logs liên quan.    │
├───────────────────────────────────────────────────────────────┤
│ PCE & Attack Path: capabilities list + (tương lai) graph.    │
└───────────────────────────────────────────────────────────────┘
```

### I. List Pages (Agents, Resources)

```text
┌───────────────────────────────────────────────────────────────┐
│ Header: <Page Title>     [Actions/Filters]                   │
├───────────────────────────────────────────────────────────────┤
│ Filter / Toolbar bar (optional)                              │
├───────────────────────────────────────────────────────────────┤
│ Table                                                         │
│ ┌─────────────────────────────────────────────────────────┐  │
│ │ Columns:                                                │  │
│ │   - Agents: Node | Status | Last heartbeat | Version    │  │
│ │   - Pods:   Name | Namespace | Risk | Node | Status     │  │
│ └─────────────────────────────────────────────────────────┘  │
│ Pagination                                                   │
└───────────────────────────────────────────────────────────────┘
```

### J. Settings / Risk Rules

```text
┌───────────────────────────────────────────────────────────────┐
│ Header: Risk Rules      [New Rule] [Import/Export]           │
├───────────────────────────────────────────────────────────────┤
│ Filter/search bar (severity, status, text search)            │
├───────────────────────────────────────────────────────────────┤
│ Table: ID | Name | Severity | Enabled | Last updated         │
├───────────────────────────────────────────────────────────────┤
│ Detail / drawer (on row click) with tabs: Metadata, YAML,   │
│ Matches.                                                     │
└───────────────────────────────────────────────────────────────┘
```

