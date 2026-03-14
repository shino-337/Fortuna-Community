# Fortuna – Risk Center UI/UX Wireframe Specification
Document Version: 1.2
Date: March 12, 2026
Reviewer: Senior Software Engineer and System Architect
Dựa trên thiết kế tổng thể đã đề xuất trước (4 trang chính + 1 Risk Detail Drawer), dưới đây là wireframe chi tiết cho từng trang.
Wireframe được mô tả bằng ASCII art + markdown layout (dễ copy-paste vào Figma hoặc code React). Mỗi trang bao gồm:

- Layout grid (Desktop 1440px)
- Các section chính
- Component quan trọng
- Hover/Click behavior

Stack: Tailwind + Recharts + TanStack Table (đang có trong Dashboard). Histogram Risk dùng Recharts (BarChart stacked), không tích hợp Grafana/Chart.js.

---

## 1. Trang 1: Risk Center Overview (/risks)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Header: Security ▶ Risk Center   | Cluster: All ▼ | Time: 7 days ▼ | Export   │
├──────────────────────────────────────────────────────────────────────────────┤
│ KPI Cards (4 ngang)                                                           │
│ ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│ │ Total Risks │  │ Critical     │  │ Resolved 24h│  │ Velocity    │          │
│ │ 1,248 (+12) │  │ 87 (P0)      │  │ 42          │  │ Chart nhỏ   │          │
│ └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘          │
├──────────────────────────────────────────────────────────────────────────────┤
│ Risk Trend 7 Days (AreaChart stacked)                                        │
│ [██████████████████ Critical ███ High ███ Medium ███ Low]                     │
├──────────────────────────────────────────────────────────────────────────────┤
│ Risk Score Distribution (Histogram – compact, ~1/3 width)                    │
│ Bins 0-10 | 10-20 | ... | 90-100   Stacked: ■Critical ■High ■Medium ■Low     │
│ Total: 1,248   P0: 87   RefLine P0(90) P1(70)  Avg score (vertical)           │
├──────────────────────────────────────────────────────────────────────────────┤
│ Risks by Cluster (Horizontal Bar)                                             │
│ Prod-Cluster1 ██████████████  456                                             │
│ Staging       ███████  289                                                    │
├──────────────────────────────────────────────────────────────────────────────┤
│ Quick Links (3 cards ngang) – optional / future                              │
│ ┌─────────────────────┐  ┌─────────────────────┐  ┌─────────────────────┐    │
│ │ View All Findings   │  │ PCE Heatmap         │  │ Evidence & Signals  │    │
│ └─────────────────────┘  └─────────────────────┘  └─────────────────────┘    │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Click behavior (Overview):**
- KPI card Critical → navigate `/risks/findings?severity=critical`
- Risk Trend: click point → filter findings theo ngày (selectedChartDate)
- Histogram: click bar → navigate /risks/findings và filter bảng theo score bin (scoreBin=0|10|…|90)
- Quick Link cards → chuyển tab/route tương ứng (khi có)


---

## 2. Trang 2: Risk Findings (/risks/findings)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Scope bar: Total findings | Cluster | Time window | Export CSV | Export PDF   │
├──────────────────────────────────────────────────────────────────────────────┤
│ Risk Score Distribution (Histogram – full width, above table)                 │
│ Bins 0-10 … 90-100   Stacked bars   P0/P1 ref lines   Avg score               │
│ [Filtered to score 10–20] Clear filter   (chỉ khi user đã click 1 bar)        │
├──────────────────────────────────────────────────────────────────────────────┤
│ Filters (inline): Severity | Priority P0–P4 | Status | Namespace | Type       │
│ Search box | Sort: Newest / Oldest / Score ↓↑ / Severity / Title               │
├──────────────────────────────────────────────────────────────────────────────┤
│ Toolbar: [Select all] Bulk: Acknowledge | Resolve | Dismiss selected          │
│ Table (virtualized)                                                            │
│ ┌──────┬─────────┬──────────┬──────────────┬────────────────────┬──────────┐  │
│ │ ☐    │ Sev     │ Priority │ Risk Score   │ Title              │ Resource │  │
│ │ 1248 │ ■ Crit  │ P0       │ 98           │ Overprivileged SA  │ pod-xyz  │  │
│ └──────┴─────────┴──────────┴──────────────┴────────────────────┴──────────┘  │
│ Pagination: 1 2 3 … 45                                                        │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Hover/Click (Findings):**
- Histogram: click bar → filter bảng theo score bin; hiện chip “Filtered to score X–Y” + “Clear filter”
- Hover row: highlight + tooltip “Click to open detail”
- Click row → mở Risk Detail Drawer (70% width)
- Bulk: checkbox header + từng row; dropdown Acknowledge / Resolve / Dismiss selected (12)
- Sidebar filter (fixed 280px) có thể dùng trong phiên bản nâng cao; hiện tại filter inline


---

## 3. Trang 3: PCE Exposure (/risks/pce)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Header: PCE Exposure   | Cluster: All ▼ | Export Heatmap                     │
├──────────────────────────────────────────────────────────────────────────────┤
│ Top: PCE Trend 7 Days (Line Chart)                                           │
│ [Line Critical ── High ── Medium]                                             │
├──────────────────────────────────────────────────────────────────────────────┤
│ Bottom: Heatmap + Table (split 50/50)                                         │
│ Heatmap (Namespace × Severity)                                               │
│ ns-prod   ■■■■■■■ Critical                                                    │
│ ns-stg    ■■ High                                                             │
│ ns-dev    ■ Medium                                                            │
│                                                                               │
│ Table drill-down                                                              │
│ Namespace | Pod | Capability | Sev | Evidence | Last Seen                     │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Interaction:**
- Click cell heatmap → filter table bên dưới
- Hover pod → tooltip “Open Pod Detail”
- (Future) Tab “Risk Score vs Capability Severity” có thể thêm histogram tương ứng


---

## 4. Trang 4: Evidence & Audit (/risks/evidence)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Tabs: Runtime Signals | Audit Trail                                           │
├──────────────────────────────────────────────────────────────────────────────┤
│ Table Runtime Signals                                                         │
│ ┌────────────┬────────────┬────────────┬────────────────────┬──────────────┐  │
│ │ Type       │ Category   │ Pod        │ Timestamp          │ Severity     │  │
│ │ Process    │ Exec       │ pod-xyz    │ 2026-03-12 14:22   │ High         │  │
│ └────────────┴────────────┴────────────┴────────────────────┴──────────────┘  │
│ Audit Trail (tab 2)                                                           │
│ User | Action | Finding ID | Timestamp | IP                                  │
└──────────────────────────────────────────────────────────────────────────────┘
```
Export: Nút riêng cho từng tab (khi có).

---

## 5. Risk Detail Drawer (side panel – 70% width)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ [X] Risk Detail – Overprivileged SA (ID 1248)                                │
├──────────────────────────────────────────────────────────────────────────────┤
│ Tab 1: Summary                                                                │
│ Severity: ■ Critical   Score: 98   Priority: P0                               │
│ Title, Description, Recommendation                                           │
│ Affected Resource: pod-xyz (link)                                             │
├──────────────────────────────────────────────────────────────────────────────┤
│ Tab 2: Evidence & Audit                                                       │
│ Evidence JSON (formatted cards)   Audit Trail (resource=insight, resourceId)  │
│ Related Runtime Signals (mini table)   Link to Pod Detail                     │
├──────────────────────────────────────────────────────────────────────────────┤
│ Tab 3: PCE & Attack Path                                                      │
│ Related Capabilities (list + severity)   MITRE ATT&CK mapping (nếu có)        │
├──────────────────────────────────────────────────────────────────────────────┤
│ Bottom bar: Acknowledge | Resolve | Dismiss | Export                          │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Behavior:**
- Mở từ bảng Findings (click row); có thể mở từ heatmap/Overview khi có deep link
- Giữ state khi chuyển tab trong drawer (Summary / Evidence & Audit / PCE & Attack Path)
- Close: ESC hoặc nút [X]

---

## 6. Risk Score Distribution – Data & API (Histogram)

| Mục | Chi tiết |
|-----|----------|
| **Endpoint** | `GET /api/v1/risk/histogram` |
| **Query params** | `clusterId` (optional), `sinceMinutes` (default 30) |
| **Response** | `bins[]` (bin 0,10,…,90; count, percent, severityBreakdown), `totalFindings`, `averageScore`, `p0Count` |
| **Cache** | Key `risk:histogram:{clusterId}:{sinceMinutes}`, TTL 30s; invalidate khi `fortuna.insights.updated` (WebSocket) |
| **Filter sync** | Click bar → gửi `scoreBin=0|10|…|90` vào GET `/risk/insights`; bảng chỉ hiển thị findings có resource nằm trong bin đó |

**Chart spec (Recharts):**
- X-axis: bins 0–10, 10–20, …, 90–100
- Y-axis: count (+ % trong tooltip)
- Bar: stacked theo severity (Critical / High / Medium / Low), màu đỏ/cam/vàng/xanh
- ReferenceLine: x=90 (P0), x=70 (P1); x = bin chứa average score (label “Avg”)
- Tooltip: bin range, count, %, breakdown Critical/High/Medium/Low
- Legend: severity colors + “Total: X findings”, “P0: Y” (khi có)

---

## 7. Implementation Notes & As-Built

| Wireframe / Design | Hiện trạng (as-built) |
|--------------------|------------------------|
| **4 routes** | `/risks`, `/risks/findings`, `/risks/pce`, `/risks/evidence` – dùng `useLocation` + tab sync trong `Insights.tsx` |
| **Overview vs Findings** | `/risks` chỉ Overview (KPI, Trend, Histogram compact, Risk Level, Risks by cluster, Quick Links, CTA). `/risks/findings` có thêm filter + bảng; histogram full width. |
| **Quick Links** | Đã có: block 3 card trên Overview (View All Findings, PCE Heatmap, Evidence & References) → navigate đúng route. |
| **Sidebar filter 280px** | Backlog – phiên bản nâng cao; hiện filter inline. |
| **Histogram** | Đã có: API `/risk/histogram`, component `RiskHistogram` (Recharts), click bar → filter `scoreBin`, chip “Clear filter” |
| **Drawer** | 3 tab: Summary, Evidence & Audit, PCE & Attack Path; bottom bar Acknowledge/Resolve/Dismiss. Mở từ URL `?insightId=<id>` (deep link). |
| **Bulk actions** | Checkbox + Acknowledge/Resolve/Dismiss selected; `POST /risk/insights/bulk`. |
| **PCE: Heatmap click → filter** | Đã có: click ô heatmap → filter bảng (namespace + severity); chip + Clear; dropdown Severity. |
| **PCE: Export Heatmap** | Đã có: nút Export Heatmap (CSV). |
| **Evidence: Tab Audit Trail** | Đã có: sub-tabs Runtime Signals \| Audit Trail; bảng User, Action, Finding ID, Timestamp, IP + pagination. |
| **Data scope (KPI/Trend)** | byType=all → Total/Critical/Trend đại diện tất cả insight types. |
| **Real-time** | WebSocket `/ws/risks` → refetch; cache invalidate khi broadcast. |

**Stack:** React, react-router-dom, Tailwind, Recharts, Zustand. Core: `GET /risk/insights`, `GET /risk/histogram`, `GET /risk/insights/summary`, threat-velocity & dashboard/stats (byType=all), `GET /inventory/pod-capabilities/*`, `GET /audit/logs` (resource=insight), WebSocket `/ws/risks`. Báo cáo & đề xuất UX/wireframe: **Risk-Center-Report-And-Wireframe-Adjustments.md**.