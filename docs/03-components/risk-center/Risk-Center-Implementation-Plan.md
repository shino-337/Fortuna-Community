# Risk Center – Kế hoạch thực hiện (Implementation Plan)

Tài liệu này **phân tích tổng thể** ba tài liệu đánh giá/đề xuất (UX Design Specification, UI/UX Wireframe Specification, Comprehensive Review), **đối chiếu với hiện trạng** mã nguồn Risk Center, sau đó **lên kế hoạch thực hiện cụ thể** theo phase với task, effort và phụ thuộc.

---

## 1. Tổng hợp đánh giá & đề xuất từ 3 tài liệu

### 1.1 UX-Design-Specification.md

| Nội dung | Đề xuất |
|----------|---------|
| **Cấu trúc UI** | 4 trang chính + 1 drawer: Overview (`/risks`), Findings (`/risks/findings`), PCE (`/risks/pce`), Evidence (`/risks/evidence`) + Risk Detail Drawer (side panel). |
| **Overview** | Landing dashboard: KPI cards (Total Active, Critical, Resolved 24h, Velocity), Risk Trend 7d, Risks by Cluster, Quick Links sang Findings/PCE/Evidence. |
| **Findings** | Bảng mạnh: filter sidebar (Severity, Priority, Status, Cluster, Namespace, Search, Date), bulk actions (Acknowledge/Resolve/Export selected), TanStack Table virtualized, click row → Drawer. |
| **PCE** | Trend 7d + Heatmap namespace×severity + bảng drill-down (Namespace \| Pod \| Capability \| Severity \| Evidence \| Last Seen). |
| **Evidence** | Tabbed: Runtime Signals \| Audit Trail; export riêng từng tab. |
| **Drawer** | 3 tab: Summary, Evidence & Correlation, PCE & Attack Path; nút Acknowledge/Resolve/Dismiss/Export. |
| **Triển khai** | Giai đoạn 1: Giữ 1 trang, cải thiện 3 tabs + Drawer. Giai đoạn 2: Tách 4 route + delta WebSocket. Giai đoạn 3: Heatmap tương tác + bulk. Effort 3–4 tuần (2 FE + 1 BE). |

### 1.2 Risk-Center-UI_UX-Wireframe-Specification.md

| Trang / Component | Wireframe & behavior |
|-------------------|----------------------|
| **Overview** | Header (breadcrumb, cluster, time, Export All); 4 KPI cards; Risk Trend AreaChart; Risks by Cluster horizontal bar; 3 Quick Link cards. Click KPI Critical → `/risks/findings?severity=critical`; chart click → filter theo ngày. |
| **Findings** | Sidebar filter 280px; toolbar Bulk \| Export \| Refresh Live; TanStack Table virtualized; pagination; click row → Drawer 70% width. |
| **PCE** | Trend line chart; heatmap + table 50/50; click cell heatmap → filter table; hover pod → tooltip. |
| **Evidence** | Tabs Signals \| Audit; table mỗi tab; export riêng. |
| **Drawer** | 70% width; 3 tab Summary / Evidence & Correlation / PCE & Attack Path; bottom bar actions. ESC/X close. |
| **Tech** | react-router 4 routes, shared RiskTable / PCEHeatmap / RiskDrawer, Zustand (filter, selected), WebSocket refresh. |

### 1.3 Risk-Center-Comprehensive-Review.md (10 findings)

| # | Finding | Mức độ | Đề xuất kỹ thuật |
|---|---------|--------|-------------------|
| 1 | WebSocket chỉ broadcast "insights_updated" không kèm delta | High | Publish delta (changed IDs, type); client mutate cache (SWR/Zustand) thay vì full refetch. |
| 2 | Risk scoring tách rời insights vs risk_scores; PCE không vào công thức | High | Thống nhất: scorer sau createInsight hoặc cột total_score/priority_level trên insights; PCE contribution. |
| 3 | MemoryRisksCache không invalidate khi update | Medium-High | ClearByPrefix khi broadcast hoặc Redis PUB/SUB clear. |
| 4 | Export CSV/PDF không streaming, limit 10k | Medium | Streaming (StreamWriter) hoặc background job + download link. |
| 5 | PCE không state machine, không stale detection | Medium | Cột state + last_seen_at; PCECleanupJob. |
| 6 | Evidence jsonb chưa mask sensitive | Medium | Sanitizer mask password/token/key trước khi lưu. |
| 7 | Chỉ API single Acknowledge/Resolve/Dismiss | Medium | POST /risk/insights/bulk (array ids) + transaction. |
| 8 | Observability thiếu histogram latency | Medium | fortuna_risk_evaluation_duration_seconds, fortuna_insights_batch_size. |
| 9 | Rule engine không versioning | Low | risk_rules_version / risk_rules_history. |
| 10 | WebSocket chưa auth/rate limit | Low | Wrap WS với JWT; rate limit. |

---

## 2. Hiện trạng so với đề xuất (Gap Analysis)

### 2.1 Cấu trúc trang & routing

| Đề xuất | Hiện trạng | Gap |
|---------|------------|-----|
| 4 route: `/risks`, `/risks/findings`, `/risks/pce`, `/risks/evidence` | Chỉ 1 route `/risks` + 3 **tab** (risks, pce, reference) trong cùng page | Chưa tách 4 trang; chưa có `/risks/findings`, `/risks/pce`, `/risks/evidence`. |
| Risk Detail Drawer 70% width, 3 tab bên trong | Đã có drawer (panel phải, max-w-lg), nội dung: Summary + Related Capabilities + Capability Details + Runtime Evidence; chưa có 3 tab trong drawer | Drawer đã có; thiếu cấu trúc 3 tab (Summary / Evidence & Correlation / PCE & Attack Path) và nút Acknowledge/Resolve/Dismiss trong drawer. |
| Overview landing với Quick Links | Tab "risks" đã có KPI (severity bar), Trend 7d, Risks by cluster, filter, bảng; chưa có Quick Link cards sang Findings/PCE/Evidence | Thiếu Quick Links dạng card; nếu tách route thì cần trang Overview riêng. |

### 2.2 Findings page (bảng + filter)

| Đề xuất | Hiện trạng | Gap |
|---------|------------|-----|
| Filter sidebar cố định 280px (Severity, Priority, Status, Cluster, Namespace, Search, Date, Rule Type) | Filter trên bảng: Severity, Workflow (status), Search, Priority (P0–P4), Sort | Chưa có sidebar filter; thiếu filter Namespace, Date range, Rule Type. |
| Bulk actions: Select nhiều → Acknowledge All / Resolve All / Export Selected | Chỉ có action từng dòng (click vào drawer); API chỉ POST :id/acknowledge, :id/resolve, :id/dismiss | Chưa có checkbox multi-select; chưa có bulk API. |
| TanStack Table + virtualized (react-window) | Bảng render danh sách risks (không rõ TanStack Table); chưa thấy react-window | Cần xác nhận dùng TanStack Table + virtualization cho >1000 rows. |
| Right panel mini: Top 5 PCE + Evidence preview | Không có panel phải nhỏ; PCE/Evidence chỉ trong drawer | Tùy chọn (nice-to-have). |

### 2.3 PCE & Evidence

| Đề xuất | Hiện trạng | Gap |
|---------|------------|-----|
| PCE: Trend 7d + Heatmap + bảng drill-down (Namespace \| Pod \| Capability \| Sev \| Evidence \| Last Seen) | Tab PCE có trend + heatmap (getPceTrend, getPceSummaryByNamespace); có bảng PCE | Bảng drill-down có thể thiếu cột Evidence count, Last Seen; PCE không state machine (Finding #5). |
| Evidence: Tabbed Signals \| Audit Trail; export riêng | Tab "reference" có RuntimeSignalsTable; chưa thấy tab Audit Trail riêng | Thiếu tab Audit Trail và export riêng từng tab. |
| Audit Trail: User \| Action \| Finding ID \| Timestamp \| IP | Backend có audit_log (createInsightAuditLog khi view/ack/resolve/dismiss) | Cần API GET audit log cho Risk Center và UI tab Audit. |

### 2.4 Backend & real-time

| Đề xuất | Hiện trạng | Gap |
|---------|------------|-----|
| WebSocket delta (changed IDs) → client mutate cache | WS chỉ nhận `{"type":"insights_updated"}` → gọi fetchData() full refetch | Finding #1: chưa delta; refetch toàn bộ. |
| Cache invalidate khi insights updated | MemoryRisksCache TTL 60s; không clear khi broadcast | Finding #3: không invalidation. |
| Bulk API POST /risk/insights/bulk | Chỉ POST /risk/insights/:id/acknowledge, resolve, dismiss | Finding #7: chưa bulk. |
| Export streaming hoặc background job | Export limit 10k, query full trong handler | Finding #4: chưa streaming. |
| Evidence masking | Không sanitize evidence/violated_rules | Finding #6: chưa mask. |
| Unified scoring (insights + risk_scores + PCE) | Join risk_scores khi withScores=1; PCE không vào score | Finding #2: chưa thống nhất. |
| PCE state + last_seen + cleanup job | pod_capabilities sync; không state machine, không PCECleanupJob | Finding #5. |
| Observability histogram | Chưa instrument duration/batch size | Finding #8. |
| Rule versioning | risk_rules không version/history | Finding #9. |
| WebSocket auth + rate limit | WS endpoint chưa rõ JWT wrap | Finding #10. |

### 2.5 Drawer chi tiết

| Đề xuất | Hiện trạng | Gap |
|---------|------------|-----|
| 3 tab trong drawer: Summary \| Evidence & Correlation \| PCE & Attack Path | Một luồng dọc: Summary, Related Capabilities, Capability Details, Runtime Evidence; có link "Open full runtime evidence tab" | Chưa tab hóa; thiếu tab PCE & Attack Path (MITRE). |
| Nút Acknowledge / Resolve / Dismiss / Export trong drawer | Drawer không có nút workflow; có thể có ở RiskDetail page | Cần thêm 4 nút trong drawer và gọi API tương ứng. |

---

## 3. Kế hoạch thực hiện cụ thể

### Nguyên tắc

- **Phase 1:** Ổn định trải nghiệm trên 1 trang hiện tại + xử lý các finding High/Medium-High (delta WS, cache invalidation, bulk, drawer đủ chức năng).
- **Phase 2:** Tách 4 route theo wireframe + unified scoring (và PCE contribution nếu khả thi).
- **Phase 3:** Nâng cấp PCE (state, cleanup), evidence masking, export streaming, observability, rule versioning, WS auth.

Effort ước tính theo **ngày dev** (1 dev = 1 ngày). Giả định 1 BE + 1 FE song song khi cần.

---

### Phase 1: Cải thiện 1 trang + Backend critical (2–3 sprint)

**Mục tiêu:** Không đổi routing; cải thiện drawer, bulk actions, real-time và cache; sửa Finding #1, #3, #7; chuẩn bị Finding #2 (design).

| ID | Task | Loại | Effort | Phụ thuộc | Ghi chú |
|----|------|------|--------|-----------|--------|
| P1-1 | **WebSocket delta payload** — Backend publish `fortuna.insights.updated` kèm `{ changed_ids: string[], change_type?: "create" \| "update" \| "delete" }`; subscriber gọi BroadcastRisksUpdate(payload); RisksWSHub broadcast payload đầy đủ | BE | 1–2d | — | Finding #1. RiskWorker + CVE matcher sau BatchCreateOrUpdateInsights trả list IDs đổi; publish kèm payload. |
| P1-2 | **Client delta merge** — Dashboard nhận WS message có `changed_ids` thì chỉ refetch summary + patch list (fetch từng page đang hiển thị hoặc invalidate key theo filter); không refetch toàn bộ danh sách nếu có thể | FE | 1d | P1-1 | Dùng cache key (filter) + mutate: nếu changed_ids nằm trong view thì refetch 1 page đó hoặc refetch list nhưng giữ summary từ delta nếu backend gửi summary snippet. |
| P1-3 | **Cache invalidation** — Khi BroadcastRisksUpdate (hoặc khi subscriber nhận insights.updated): gọi RisksCache.ClearByPrefix("risks:list:") và ClearByPrefix("insights:summary:") (thêm interface ClearByPrefix cho MemoryRisksCache) | BE | 0.5d | — | Finding #3. |
| P1-4 | **Bulk API** — POST /risk/insights/bulk body `{ action: "acknowledge" \| "resolve" \| "dismiss", insight_ids: string[] }`; transaction loop gọi logic Acknowledge/Resolve/Dismiss; trả { success_count, failed_count, errors?: [] } | BE | 1d | — | Finding #7. |
| P1-5 | **Bulk actions UI** — Checkbox multi-select trên bảng risks; toolbar "Acknowledge selected (n)", "Resolve selected (n)", "Export selected"; gọi bulk API; sau thành công refetch list / delta | FE | 1d | P1-4 | |
| P1-6 | **Drawer: 3 tab + actions** — Trong drawer hiện tại: thêm 3 tab (Summary, Evidence & Correlation, PCE & Attack Path); chuyển nội dung hiện có vào đúng tab; thêm bottom bar: Acknowledge, Resolve, Dismiss, Export (single); gọi API POST :id/acknowledge v.v. | FE | 1–1.5d | — | Không đổi route; chỉ nâng cấp drawer. |
| P1-7 | **Evidence & Audit tab (trang chính)** — Trong tab "reference": thêm sub-tab hoặc toggle "Runtime Signals" \| "Audit Trail"; Audit Trail gọi API GET audit logs (filter by resource=insight); export riêng từng bảng | FE + BE | 1d | API audit log sẵn | Cần endpoint GET /audit hoặc /risk/insights/audit filter by insight_id nếu chưa có. |
| P1-8 | **Filter bổ sung** — Trên tab risks: thêm filter Namespace (dropdown), Date range (sinceMinutes hoặc range picker), Rule Type (insight_type); đồng bộ query params | FE | 0.5d | — | Backend đã hỗ trợ type, resource_namespace; cần API list namespaces hoặc dùng từ pods. |

**Tổng Phase 1:** ~7–8 ngày (BE ~3.5d, FE ~4.5d). Có thể gộp trong 2 sprint nếu 1 BE + 1 FE.

---

### Phase 2: Tách 4 trang + Unified scoring (2–3 sprint)

**Mục tiêu:** Cấu trúc 4 route theo UX/Wireframe; landing Overview; Findings/PCE/Evidence từng trang; Quick Links; unified scoring (design + implementation cơ bản).

| ID | Task | Loại | Effort | Phụ thuộc | Ghi chú |
|----|------|------|--------|-----------|--------|
| P2-1 | **Routing 4 trang** — Thêm route `/risks` (Overview), `/risks/findings`, `/risks/pce`, `/risks/evidence`; component tương ứng hoặc 1 RiskCenterLayout với outlet; sidebar/header "Security → Risk Center" giữ | FE | 0.5d | — | react-router nested routes. |
| P2-2 | **Trang Overview** — Nội dung: KPI cards (Total, Critical, Resolved 24h, Velocity mini); Risk Trend 7d; Risks by Cluster; 3 Quick Link cards → /risks/findings, /risks/pce, /risks/evidence. Click KPI Critical → navigate findings?severity=critical | FE | 1.5d | P2-1 | Reuse API getInsightsSummary, getThreatVelocity, getInsightsSummaryByCluster. |
| P2-3 | **Trang Findings** — Chuyển toàn bộ nội dung tab "risks" hiện tại sang /risks/findings: filter (có thể sidebar 280px), bảng, bulk, sort, pagination; click row → mở Drawer (shared component) | FE | 1d | P2-1, P1-5, P1-6 | Tách component RiskFindingsPage; shared RiskTable, RiskDrawer. |
| P2-4 | **Trang PCE** — Chuyển tab PCE sang /risks/pce: trend + heatmap + bảng drill-down; thêm cột Evidence count, Last Seen nếu API có | FE | 0.5d | P2-1 | |
| P2-5 | **Trang Evidence** — Chuyển tab reference + Audit sang /risks/evidence; tabbed Signals \| Audit; export riêng | FE | 0.5d | P2-1, P1-7 | |
| P2-6 | **Unified scoring (design + DB)** — Migration: thêm cột optional total_score, priority_level vào bảng insights (nullable); job hoặc hook sau CreateOrUpdateInsight: gọi scorer, ghi vào insights; GET /risk/insights có thể đọc từ insights trước, fallback join risk_scores | BE | 2–3d | — | Finding #2. Bước 1: dual-write; bước 2: PCE contribution vào scorer (optional). |
| P2-7 | **Drawer mở từ mọi trang** — RiskDrawer dùng global state (Zustand) hoặc URL /risks?drawer=:id để mở từ Overview/Findings/PCE (click chart, heatmap, link) | FE | 0.5d | P2-1, P1-6 | |

**Tổng Phase 2:** ~6–7 ngày. Sau Phase 2 có đủ 4 trang + drawer theo spec.

---

### Phase 3: Chất lượng dữ liệu, PCE, Observability (2–3 sprint)

**Mục tiêu:** Finding #4, #5, #6, #8, #9, #10.

| ID | Task | Loại | Effort | Phụ thuộc | Ghi chú |
|----|------|------|--------|-----------|--------|
| P3-1 | **Export streaming** — GET /risk/insights/export dùng StreamWriter hoặc chunked response; CSV từng chunk; hoặc tạo job background → file URL → download | BE | 1–1.5d | — | Finding #4. |
| P3-2 | **PCE state + last_seen** — Thêm cột state (detected/confirmed/exploited nếu cần), last_seen_at vào pod_capabilities; Agent gửi last_seen; PCECleanupJob xóa/ẩn bản ghi quá cũ (tương tự InsightsCleanupJob) | BE | 1.5–2d | — | Finding #5. |
| P3-3 | **Evidence masking** — Hàm sanitize evidence/violated_rules trước khi lưu (mask giá trị field chứa password, token, secret, key); gọi trong createInsight / BatchCreateOrUpdateInsights | BE | 1d | — | Finding #6. |
| P3-4 | **Observability** — Histogram fortuna_risk_evaluation_duration_seconds, fortuna_insights_batch_size; có thể thêm risk_queries_total | BE | 0.5d | — | Finding #8. |
| P3-5 | **Rule versioning** — Bảng risk_rules_history (snapshot); cột version, created_by trên risk_rules; mỗi update rule tạo bản ghi history | BE | 1d | — | Finding #9. |
| P3-6 | **WebSocket auth + rate limit** — Middleware kiểm tra JWT (query token hoặc cookie) trước khi upgrade WS; rate limit theo IP/user (số kết nối/phút) | BE | 0.5–1d | — | Finding #10. |

**Tổng Phase 3:** ~5.5–7 ngày.

---

## 4. Tổng quan roadmap & ưu tiên

| Phase | Nội dung chính | Effort ước tính | Sprint (1 sprint ≈ 2 tuần) |
|-------|----------------|-----------------|----------------------------|
| **Phase 1** | Delta WebSocket, cache invalidation, bulk API+UI, drawer 3 tab + actions, Evidence/Audit tab, filter bổ sung | 7–8 ngày | 1–1.5 sprint |
| **Phase 2** | 4 route, Overview landing, Findings/PCE/Evidence pages, Quick Links, unified scoring (dual-write + optional PCE) | 6–7 ngày | 1–1.5 sprint |
| **Phase 3** | Export streaming, PCE state+cleanup, evidence masking, observability, rule versioning, WS auth | 5.5–7 ngày | 1–1.5 sprint |

**Tổng:** ~19–22 ngày dev (khoảng 4–5 sprint với 1 BE + 1 FE). Có thể chạy song song FE và BE trong từng phase để rút ngắn calendar time.

---

## 5. Ràng buộc & rủi ro

- **Delta WebSocket:** Client cần strategy rõ (chỉ refetch 1 page vs full list) để tránh inconsistency khi sort/filter thay đổi.
- **Unified scoring:** Migration thêm cột insights; cần đảm bảo backward compatibility với GET /risk/insights (withScores=1) và dashboard cũ.
- **Bulk API:** Giới hạn tối đa insight_ids (ví dụ 500) để tránh timeout; trả danh sách lỗi từng id nếu cần.
- **Export streaming:** Nếu dùng background job thì cần endpoint GET /risk/insights/export/:job_id/download và retention file tạm.

---

## 6. Checklist chấp nhận (theo phase)

**Phase 1 done khi:**  
- [ ] WS payload có changed_ids; client không full refetch khi chỉ 1 insight đổi.  
- [ ] Sau insights updated, cache list/summary bị clear (hoặc TTL ngắn).  
- [ ] Bulk acknowledge/resolve/dismiss từ UI hoạt động.  
- [ ] Drawer có 3 tab và 4 nút Acknowledge/Resolve/Dismiss/Export.  
- [ ] Tab Evidence có Audit Trail và export riêng (nếu API sẵn).  

**Phase 2 done khi:**  
- [ ] /risks (Overview), /risks/findings, /risks/pce, /risks/evidence đều load đúng.  
- [ ] Quick Links từ Overview chuyển đúng trang.  
- [ ] Insights có thể hiển thị total_score/priority_level từ bảng insights (hoặc join).  

**Phase 3 done khi:**  
- [ ] Export lớn không timeout (streaming hoặc job).  
- [ ] PCE có last_seen/cleanup (nếu scope đồng ý).  
- [ ] Evidence không lộ token/password (mask).  
- [ ] Prometheus có histogram risk evaluation + batch size.  
- [ ] Rule edit lưu history.  
- [ ] WebSocket yêu cầu auth và có rate limit.  

---

*Tài liệu này là bản kế hoạch thực hiện; khi bắt đầu từng task nên tạo ticket/issue cụ thể và cập nhật effort thực tế.*
