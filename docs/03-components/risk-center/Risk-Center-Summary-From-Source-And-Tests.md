# Risk Center – Tổng kết theo Review và thực tế mã nguồn / test case

Tài liệu này tổng kết các nội dung đã thực hiện cho Risk Center, đối chiếu với **câu hỏi đánh giá** trong [Risk-Center_Review.md](./Risk-Center_Review.md) và **thực tế mã nguồn** (Core, Dashboard, DB) cùng **E2E test cases** trong [Risk-Center-E2E-TestCases.md](./Risk-Center-E2E-TestCases.md).

---

## 1. Thông tin tổng quan Risk Center

| Câu hỏi Review | Thực tế đã triển khai |
|-----------------|------------------------|
| **Mô tả chức năng** | Risk Center là **một phần của Dashboard** (unified workspace): (1) **Risk Findings** – danh sách insights (CVE, RBAC, …) với filter/sort/export; (2) **PCE** – Pod Capability Exposure (trend 7 ngày, heatmap namespace×severity); (3) **Evidence & References** – runtime signals, capability catalog. |
| **High-level architecture** | User → Dashboard (React) → Core API (Gin) → PostgreSQL / NATS. Core nhận dữ liệu từ Agent (sync pods, PCE, runtime), workers (CVE matcher, risk engine) ghi insights/risk_scores và publish NATS `fortuna.insights.updated` → WebSocket `/ws/risks` push cho Dashboard refetch. |
| **Use-case chính** | Ưu tiên hóa risks (severity + risk score/priority), export báo cáo (CSV/PDF), xem theo cluster, PCE trends, quản lý risk rules (CRUD khi source=db). |
| **Version / tag** | Không tách module riêng; Dashboard build từ `dashboard/`, Core từ `core/`. Image: `fortuna-dashboard:latest`, `fortuna-core:latest`. |

**Mã nguồn tham chiếu:** `dashboard/pages/Insights.tsx` (Risk Center), `dashboard/App.tsx` route `/risks`, `core/internal/api/routes.go` (risks, insights, risk-rules, pod-capabilities, ws/risks).

---

## 2. System Integration và Data Flow

| Câu hỏi Review | Thực tế đã triển khai |
|----------------|------------------------|
| **Nguồn dữ liệu** | Insights từ CVE matcher + risk engine; `risk_scores` từ engine (MVP2); PCE từ Agent sync `pod_capabilities`; runtime signals từ Agent. |
| **Luồng dữ liệu** | (1) **Real-time:** NATS `fortuna.insights.updated` → Core subscriber gọi `BroadcastRisksUpdate()` → WebSocket `/ws/risks` push `insights_updated` → Dashboard refetch. (2) **Polling:** Dashboard gọi GET `/risks`, `/insights/summary`, … theo refresh interval. (3) API → GORM → PostgreSQL; cache in-memory 60s cho GET `/risks` và GET `/insights/summary`. |
| **Integration** | Core: GET `/risks` (có `withScores=1`, `priorityLevel`), GET `/insights/summary`, `/insights/summary/by-cluster`, GET `/risks/export`, GET `/risk-rules`, GET `/pod-capabilities/trends`, GET `/pod-capabilities/summary/namespace`, GET `/ws/risks`. Dashboard: `api.getRisks`, `getInsightsSummary`, `getInsightsSummaryByCluster`, `exportRisksCSV`/`exportRisksPDF`, `getRiskRules`, `getPceTrend`, `getPceSummaryByNamespace`, `getRisksWsUrl()` + subscribe refetch. |
| **Multi-cluster** | Filter theo `clusterId` (query param); GET `/insights/summary/by-cluster` trả về mảng theo cluster (total, critical, high, …). Dashboard: scope "all clusters" hoặc chọn 1 cluster; tab Risk Findings có "Risks by cluster" khi scope = all. |

**Mã nguồn:** `core/cmd/main.go` (NATS subscriber → `BroadcastRisksUpdate`), `core/internal/api/risks_ws_hub.go`, `core/internal/api/risks_cache.go`, `dashboard/lib/api.ts`, `dashboard/pages/Insights.tsx`.  
**E2E:** TC-01 (GET /risks), TC-04 (summary), TC-05 (by-cluster), TC-16 (WebSocket /ws/risks).

---

## 3. User Interface và Functionality

| Câu hỏi Review | Thực tế đã triển khai |
|----------------|------------------------|
| **UI framework / components** | React (Vite). Risk Center: `dashboard/pages/Insights.tsx` (tabs: Risk Findings, PCE, Evidence & References). Layout: `dashboard/components/Layout.tsx` (sidebar Risk Center → `/risks`). |
| **Tính năng chính** | Filter/sort risks (severity, status, search, **priority P0–P4**, **sort by risk score**); Risk Level Overview (severity bar); Risk Trend 7 ngày (AreaChart); Risks by cluster (bảng khi all clusters); **Export CSV**, **Export PDF** (cùng dòng Total findings/Scope/Time); PCE tab: trend 7 ngày, heatmap namespace×severity; Reference: runtime signals table. |
| **Authentication/Authorization** | JWT (login); API gọi với Bearer token. RBAC theo Core (admin/user). |
| **Export/Reporting** | **Export CSV:** GET `/risks/export` → Dashboard `exportRisksCSV()` → download `risks-export.csv`. **Export PDF:** GET `/risks/export?format=pdf` → HTML in sẵn → user Print → Save as PDF. Chưa có SIEM webhook/NATS publisher riêng. |

**Mã nguồn:** `dashboard/pages/Insights.tsx` (tabs, filters, Export buttons, PCE trend/heatmap, WS refetch), `dashboard/lib/api.ts` (`exportRisksCSV`, `exportRisksPDF`).  
**E2E:** TC-02, TC-03 (withScores, priorityLevel), TC-06 (export CSV), TC-07 (export PDF), TC-13 (PCE trends), TC-14 (PCE summary/namespace).

---

## 4. Data Model và Storage

| Câu hỏi Review | Thực tế đã triển khai |
|----------------|------------------------|
| **Schema risk-related** | Bảng: `insights` (id, severity, status, cluster_id, resource_*, detected_at, …), `risk_scores` (MVP2: total_score, priority_level, …), `cve_matches`, `pod_capabilities`, runtime tables. GET `/risks` join insights + risk_scores khi `withScores=1`. |
| **PCE** | Bảng `pod_capabilities`; sync từ Agent. API: `/pod-capabilities/trends`, `/pod-capabilities/summary/namespace` (heatmap). |
| **Evidence** | Insights có evidence/violated_rules; runtime signals API; Pod Detail link từ risk. |
| **Retention** | InsightsCleanupJob: resolved 30 ngày, active 90 ngày (hard-coded trong code; chưa env config). |

**Mã nguồn:** `core/migrations/` (001, 030, 033, 058, …; mvp2/001–004 risk_scores), `core/pkg/models`, `core/internal/api/dashboard_handlers.go` (GetInsightsListCached, join risk_scores).

---

## 5. Processing và Analysis Logic

| Câu hỏi Review | Thực tế đã triển khai |
|----------------|------------------------|
| **Risk scoring** | Engine tính risk_scores (MVP2); GET `/risks?withScores=1` trả về insight kèm `totalScore`, `priorityLevel` từ join `risk_scores`. Dashboard sort/filter theo score và priority. |
| **PCE logic** | Agent sync pod capabilities; Core API trends/summary by namespace/severity. |
| **Correlation** | Insights gắn resource (pod UID, namespace); workers tạo insights từ CVE/rule; NATS `fortuna.insights.updated` để WS push. |
| **Customizable rules** | **Risk rules CRUD:** API GET/POST/PUT/DELETE `/risk-rules`, `/risk-rules/:id`; load từ DB (ưu tiên) hoặc files (`FORTUNA_RULES_DIR`). Dashboard: Settings → tab "Risk Rules" (bảng, Add/Edit/Delete khi `source === 'db'`). |

**Mã nguồn:** `core/internal/api/risk_rules_handlers.go`, `core/internal/api/risks_list_with_scores_test.go`, `dashboard/pages/Settings.tsx` (Risk Rules tab).  
**E2E:** TC-08–TC-12 (risk-rules list, get by id, POST/PUT/DELETE).

---

## 6. Performance & Scalability

| Câu hỏi Review | Thực tế đã triển khai |
|----------------|------------------------|
| **Caching** | **In-memory TTL cache** (60s) cho GET `/risks` và GET `/insights/summary`; key theo clusterId, status, severity, search, priorityLevel, sinceMinutes, page, pageSize, withScores. `core/internal/api/risks_cache.go` (MemoryRisksCache), routes.go gắn GetInsightsListCached, GetInsightsSummaryCached. |
| **Scaling / latency** | Chưa benchmark chính thức; cache giảm load DB cho list/summary. |
| **Backpressure** | Chưa throttle riêng cho Risk Center; WebSocket broadcast không block (drop nếu send buffer full). |

**Mã nguồn:** `core/internal/api/risks_cache.go`, `core/internal/api/risks_list_cached.go`, `core/internal/api/routes.go`.

---

## 7. Reliability & High Availability

| Câu hỏi Review | Thực tế đã triển khai |
|----------------|------------------------|
| **Failover** | Không fallback cache riêng khi Core down; Dashboard hiển thị lỗi khi API fail. |
| **Data consistency** | Eventual: insights/risk_scores cập nhật qua workers; WS push để UI refetch kịp thời. |
| **Backup/Restore** | Risk data nằm trong PostgreSQL; backup DB theo chiến lược infra (không logic riêng trong Risk Center). |

---

## 8. Security Model

| Câu hỏi Review | Thực tế đã triển khai |
|----------------|------------------------|
| **Access controls** | JWT auth; Core middleware cho API; WebSocket /ws/risks có thể dùng token query. |
| **Auditing** | Chưa audit log riêng cho hành động user trong Risk Center (xem/acknowledge risk). |
| **Vulnerability / mask data** | API trả về evidence theo quyền; chưa mask chi tiết process/sensitive trong tài liệu. |

---

## 9. Observability

| Câu hỏi Review | Thực tế đã triển khai |
|----------------|------------------------|
| **Metrics** | Core có metrics cơ bản (ví dụ insights); chưa Prometheus keys riêng cho risk_queries_total, correlation_duration. |
| **Logging** | Log server-side (Core); không structured audit cho từng action Risk Center. |
| **Tracing** | Chưa OpenTelemetry spans cho risk pipeline. |
| **Alerting** | Chưa Prometheus/alert rules mẫu cho critical risk tăng đột biến. |

*(Các mục 7–9 phần lớn nằm trong khuyến nghị Review chưa triển khai đầy đủ.)*

---

## 10. Thông tin bổ sung – Code và Test

### API Endpoints (Core) – đã có và có E2E

| Endpoint | Mô tả | E2E TC |
|----------|--------|--------|
| GET `/risks` | List insights, pagination, filters, withScores, priorityLevel | TC-01, TC-02, TC-03 |
| GET `/insights/summary` | Severity bar (total, critical, high, medium, low) | TC-04 |
| GET `/insights/summary/by-cluster` | By-cluster aggregation | TC-05 |
| GET `/risks/export` | CSV download | TC-06 |
| GET `/risks/export?format=pdf` | HTML for PDF | TC-07 |
| GET/POST/PUT/DELETE `/risk-rules`, `/risk-rules/:id` | CRUD risk rules (source=db) | TC-08–TC-12 |
| GET `/pod-capabilities/trends` | PCE trend 7 ngày | TC-13 |
| GET `/pod-capabilities/summary/namespace` | PCE heatmap | TC-14 |
| GET `/runtime-signals` | Runtime signals (Reference) | TC-15 |
| GET `/ws/risks` | WebSocket Risk Center updates | TC-16 |

### Dashboard (Risk Center)

- **File:** `dashboard/pages/Insights.tsx`  
- **Tính năng:** tabs (risks, pce, reference), severity bar, Risk Trend chart, Risks by cluster, Export CSV/PDF, filter Priority (P0–P4), sort by risk score, cột Risk Score, PCE trend 7 ngày, PCE heatmap, WebSocket refetch khi `insights_updated`.  
- **Kiểm tra UI:** [Risk-Center-Dashboard-Verification.md](./Risk-Center-Dashboard-Verification.md), [Risk-Center-Dashboard-Pod-Check.md](./Risk-Center-Dashboard-Pod-Check.md).

### E2E script và báo cáo

- **Script:** `scripts/e2e/e2e-risk-center-full.sh`  
- **Báo cáo:** `test-results/risk-center-e2e-YYYYMMDD-HHMMSS.md`  
- **16 test cases:** Base (TC-01, TC-04, TC-15), Phase 2.2 (TC-05), Phase 2.3 (TC-16), Phase 1.3/3.3 (TC-06, TC-07), Phase 3.1 (TC-02, TC-03), Phase 4 (TC-08–TC-14). Kết quả mẫu: 15 PASS, 1 SKIP (TC-09 khi không có rule).

### Đối chiếu với Findings trong Risk-Center-Component-Implement.md

| Finding | Khuyến nghị | Trạng thái thực tế |
|---------|-------------|---------------------|
| #1 Real-time push | WebSocket /ws/risks | **Đã có:** RisksWSHub, BroadcastRisksUpdate, NATS subscriber → WS push; Dashboard subscribe và refetch. |
| #2 Unified scoring | Merge risk_scores vào insights, UI sort/filter | **Đã có:** GET /risks?withScores=1&priorityLevel=; join risk_scores; Dashboard cột Risk Score, filter Priority, sort by score. |
| #3 Caching | Redis/in-memory cho risk queries | **Đã có:** In-memory TTL 60s cho GET /risks và GET /insights/summary. |
| #4 Export / SIEM | Export CSV/PDF, SIEM webhook | **Đã có:** Export CSV + PDF (HTML); chưa SIEM publisher. |
| #5 Retention configurable | Env / UI retention days | **Một phần:** Đã configurable qua env (`INSIGHTS_RESOLVED_RETENTION_DAYS`, `INSIGHTS_ACTIVE_RETENTION_DAYS`). Thiếu: document cho Ops, UI/DB config (tùy chọn). Chi tiết: [Risk-Center-Gap-Analysis-And-Improvement.md](./Risk-Center-Gap-Analysis-And-Improvement.md#5--retention-configurable). |
| #6 Observability | OTel, alerting | **Một phần:** Prometheus metrics (insights, risk_scores) có. Thiếu: OTel spans cho risk pipeline, Prometheus alert rules cho Risk Center. Chi tiết: [Risk-Center-Gap-Analysis-And-Improvement.md](./Risk-Center-Gap-Analysis-And-Improvement.md#6--observability-otel-alerting). |
| #7 Audit logs | Audit user actions | **Một phần:** Đã audit acknowledge, resolve, dismiss (`createInsightAuditLog`). Thiếu: audit khi user **view** risk (GET /insights/:id hoặc mở drawer). Chi tiết: [Risk-Center-Gap-Analysis-And-Improvement.md](./Risk-Center-Gap-Analysis-And-Improvement.md#7--audit-logs-user-actions-trong-risk-center). |
| #8 Global view | /risks/global-summary, tab Global | **Một phần:** GET /insights/summary/by-cluster có. Thiếu: endpoint /insights/summary/global (hoặc /risks/global-summary) và UI tab/view global. Chi tiết: [Risk-Center-Gap-Analysis-And-Improvement.md](./Risk-Center-Gap-Analysis-And-Improvement.md#8--global-view). |
| #9 PCE heatmap/trends | Recharts PCE trend + heatmap | **Đã có:** getPceTrend(7), getPceSummaryByNamespace; UI PCE tab trend + heatmap. |
| #10 Risk rules CRUD + UI | API + DB + UI admin | **Đã có:** API CRUD, migration 075 risk_rules, Settings tab Risk Rules (Add/Edit/Delete khi source=db). |

---

## Tóm tắt

- Risk Center là **một phần của Dashboard** (route `/risks`), gồm ba tab: **Risk Findings** (insights + score/priority + export CSV/PDF + risks by cluster), **PCE** (trend 7 ngày + heatmap namespace), **Evidence & References** (runtime signals).
- **Core:** API đầy đủ risks, insights/summary, by-cluster, export CSV/PDF, risk-rules CRUD, pod-capabilities/trends và summary/namespace, WebSocket `/ws/risks`; cache in-memory 60s cho list và summary; NATS → BroadcastRisksUpdate → WS push.
- **Dashboard:** Gọi đủ API trên, hiển thị filter/sort (kể cả priority và risk score), Export CSV/PDF, PCE trend/heatmap, Risk Rules trong Settings; WebSocket refetch khi có `insights_updated`.
- **E2E:** 16 test cases trong `e2e-risk-center-full.sh` phủ toàn bộ API trên; kết quả điển hình 15 PASS, 1 SKIP.
- Các mục Review **chưa làm đủ** trong mã nguồn: retention configurable, observability (tracing/alerting), audit log user actions, SIEM integration, global-summary endpoint.

Tài liệu tham chiếu: [Risk-Center_Review.md](./Risk-Center_Review.md), [Risk-Center-Component-Implement.md](./Risk-Center-Component-Implement.md), [Risk-Center-E2E-TestCases.md](./Risk-Center-E2E-TestCases.md), [Risk-Center-Dashboard-Verification.md](./Risk-Center-Dashboard-Verification.md). **Phân tích chi tiết các nội dung chưa hoàn thành và hướng cải tiến:** [Risk-Center-Gap-Analysis-And-Improvement.md](./Risk-Center-Gap-Analysis-And-Improvement.md).
