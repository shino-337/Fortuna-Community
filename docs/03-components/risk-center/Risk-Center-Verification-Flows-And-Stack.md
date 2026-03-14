# Risk Center – Kiểm tra chi tiết: Dashboard, Core, Agent, API, Database

**Phiên bản:** 1.0  
**Ngày:** 2026-03-10  
**Phạm vi:** Luồng dữ liệu, API, UI/UX đã bổ sung (histogram, risk_scores sync, KPI, Recalculate all scores).

---

## 1. Tổng quan luồng dữ liệu

```
┌─────────────┐     SBOM/CVE/Pod     ┌─────────────┐     insights     ┌─────────────┐
│   Agent     │ ──────────────────►  │  Core       │ ──► DB: insights │  Database   │
│ (syncer,    │   (gRPC/NATS/HTTP)   │  Workers    │     risk_scores  │  (Postgres) │
│  sbom)      │                      │  InsightMgr │ ◄── Scorer V2    │             │
└─────────────┘                      └──────┬──────┘     (auto/sync)  └──────┬──────┘
                                             │                               │
                                             │ GET /risk/insights            │
                                             │ GET /risk/histogram           │
                                             │ POST /risk/scores/sync        │
                                             ▼                               │
                                      ┌─────────────┐                        │
                                      │  Dashboard  │ ◄──────────────────────┘
                                      │  Risk Center│   (REST API + WebSocket)
                                      └─────────────┘
```

---

## 2. Database

### 2.1 Bảng liên quan Risk Center

| Bảng | Mục đích | Migration |
|------|----------|-----------|
| **insights** | Findings (vulnerability, capability, policy…); có `resource_uid`, `severity`, `status`, `detected_at` | Có sẵn |
| **risk_scores** | Điểm risk theo resource (0–100), `priority_level` P0–P4; join với insights cho histogram & withScores | 012, 013, 018 |
| **pods** | Pod inventory (cluster_id, uid); dùng filter cluster trong histogram/insights | Có sẵn |
| **pod_capabilities** | PCE (heatmap, trend); không trộn với risk_scores | Có sẵn |

### 2.2 risk_scores schema (đủ cho API & UI)

- **Cột chính:** `resource_type`, `resource_uid`, `resource_name`, `namespace`, `cluster_id`, `total_score`, `base_score`, `time_decay`, `exploitability_score`, `business_impact_score`, `scorer_version`, `priority_level`, `calculated_at`, `deleted_at`.
- **Ràng buộc:** UNIQUE(resource_type, resource_uid, cluster_id).
- **Index:** total_score, resource_uid, cluster_id, priority_level, calculated_at, deleted_at.

### 2.3 Luồng ghi risk_scores

1. **Tự động khi insight thay đổi (Core):**
   - `InsightManager.CreateOrUpdateInsight` hoặc `BatchCreateOrUpdateInsights` (sau khi transaction commit).
   - Gọi `scheduleRiskScoreCalculation(insight)` hoặc với batch: thu thập distinct `resource_uid` → với mỗi UID chạy `runRiskScoreCalculation(ctx, uid)` trong goroutine.
   - `runRiskScoreCalculation` → `risk.NewScorer(db).CalculateScore(ctx, resourceUID)` → `SaveScore(ctx, score)` → ghi/upsert vào `risk_scores`.

2. **Đồng bộ hàng loạt (theo yêu cầu):**
   - Dashboard gọi `POST /api/v1/risk/scores/sync`.
   - Core: lấy distinct `resource_uid` từ `insights` (status IN ('active','acknowledged'), deleted_at IS NULL), sau đó trong goroutine với mỗi UID: CalculateScore + SaveScore.
   - Trả về 202 Accepted, body `{ "message": "Risk score sync started", "resources": N }`.

---

## 3. Core API

### 3.1 Risk Center – endpoints đang dùng

| Method | Path | Handler | Ghi chú |
|--------|------|---------|--------|
| GET | `/api/v1/risk/insights` | GetInsightsListCached | Pagination, filter: severity, status, clusterId, sinceMinutes, **scoreBin**, **priorityLevel**, withScores=1 (join risk_scores). |
| GET | `/api/v1/risk/insights/summary` | GetInsightsSummaryCached | Tổng theo severity (total, critical, high, medium, low). |
| GET | `/api/v1/risk/insights/summary/by-cluster` | GetInsightsSummaryByCluster | Summary theo cluster (Risks by cluster). |
| GET | `/api/v1/risk/histogram` | GetRiskHistogram | Bins 0–90, count + severity breakdown; **INNER JOIN risk_scores**; query: clusterId, sinceMinutes (default 30). |
| POST | `/api/v1/risk/scores/sync` | risk.SyncRiskScores | Recalculate risk_scores cho mọi resource có insight active/acknowledged; 202 Accepted. |
| POST | `/api/v1/risk/scores/:uid/calculate` | risk.CalculateRiskScore | Tính & lưu score cho một resource. |
| GET | `/api/v1/risk/trends` | risk.GetRiskTrends | Trend 7 ngày (Dashboard dùng getThreatVelocity). |
| GET | `/api/v1/dashboard/stats` | GetDashboardStats | Có `resolved24h` (KPI Resolved 24h). |

### 3.2 GET /risk/histogram – chi tiết

- **Cache:** Key `risk:histogram:{clusterId}:{sinceMinutes}`, TTL 30s.
- **Invalidate:** Khi `BroadcastRisksUpdateWithPayload` (NATS `fortuna.insights.updated` → subscriber gọi broadcast → ClearByPrefix("risk:histogram:")).
- **Query:** insights INNER JOIN risk_scores ON resource_uid; filter: status active/NULL, deleted_at NULL, cluster (qua pods), detected_at >= sinceMinutes.
- **Response:** `bins[]` (bin 0,10,…,90; count, percent, severityBreakdown, critical_count, high_count, medium_count, low_count), `totalFindings`, `averageScore`, `p0Count`. Luôn trả 10 bins (fill 0 nếu không có dữ liệu).

### 3.3 POST /risk/scores/sync – chi tiết

- **Logic:** Pluck distinct `resource_uid` từ insights (status IN ('active','acknowledged'), deleted_at IS NULL); filter bỏ UID rỗng.
- **Response:** resources=0 → 200 + message "No resources..."; resources>0 → 202 + "Risk score sync started", "resources": N.
- **Background:** Goroutine với từng UID: scorer.CalculateScore → scorer.SaveScore; log lỗi từng UID, không trả về cho client.

---

## 4. Core – luồng insight → risk_scores

### 4.1 Nơi tạo/cập nhật insights

| Nơi | Cách gọi | Kích hoạt risk_scores |
|-----|----------|------------------------|
| CVEMatcherWorker | BatchCreateOrUpdateInsights(insights) | Có (sau commit: schedule theo từng resource_uid) |
| RiskWorker | BatchCreateOrUpdateInsights / CreateOrUpdateInsight | Có |
| HistoricalRiskEvaluator | CreateOrUpdateInsight | Có (scheduleRiskScoreCalculation) |
| Capability evaluator | CreateOrUpdateInsight | Có |
| CVE processor (scanner) | CreateOrUpdateInsight | Có |

### 4.2 InsightManager (riskengine)

- **CreateOrUpdateInsight:** Sau khi transaction thành công, gọi `scheduleRiskScoreCalculation(insight)` → goroutine `runRiskScoreCalculation(context.Background(), insight.ResourceUID)`.
- **BatchCreateOrUpdateInsights:** Sau khi transaction thành công, thu thập distinct `resource_uid` từ slice insights, với mỗi UID: `go runRiskScoreCalculation(context.Background(), uid)`.
- **runRiskScoreCalculation:** `risk.NewScorer(m.db).CalculateScore(ctx, uid)` → `SaveScore(ctx, score)`; log lỗi, không block.

### 4.3 WebSocket & cache

- NATS subject `fortuna.insights.updated`: CVEMatcherWorker và RiskWorker publish sau khi BatchCreateOrUpdateInsights/CreateOrUpdateInsight thành công.
- main.go subscribe → gọi `api.BroadcastRisksUpdateWithPayload` → ClearByPrefix("risks:list:"), ClearByPrefix("insights:summary:"), **ClearByPrefix("risk:histogram:")** → broadcast WebSocket cho Risk Center.

---

## 5. Dashboard (Risk Center UI)

### 5.1 Routes & tabs

- `/risks` – Overview (KPI, Trend, Histogram compact, Risk Level Overview, Risks by cluster, Quick Links).
- `/risks/findings` – Bảng findings + filter + histogram full width + Export CSV/PDF.
- `/risks/pce` – PCE Heatmap + table.
- `/risks/evidence` – Runtime Signals | Audit Trail.

Tab sync với path (useLocation); nút tab chuyển navigate tương ứng.

### 5.2 APIs Dashboard gọi (Risk tab)

- **fetchData (một lần load/refresh):**  
  getRisks, getInsightsSummary, getThreatVelocity(7), getPceSummaryBySeverity, getPceCapabilities, getPceTrend, getPceSummaryByNamespace, getClusters, getStats, getInsightsSummaryByCluster (khi không chọn cluster), **getRiskHistogram**.
- **Khi bấm "Recalculate all scores":** **syncRiskScores()** (POST /risk/scores/sync); sau 4s gọi lại fetchData.

### 5.3 UI/UX đã bổ sung (theo wireframe & yêu cầu)

| Thành phần | Mô tả |
|------------|--------|
| **4 KPI cards** | Total Risks (có delta vs 7d ago), Critical (kèm P0 từ histogram nếu có), Resolved 24h, Velocity (↑/↓ vs 7d ago). |
| **Risk Trend (7 days)** | AreaChart; click điểm → filter theo ngày. |
| **Risk Score Distribution** | Histogram Recharts (stacked severity); ref line P0(90), P1(70), Avg; compact ~1/3 width trên Overview; click bar → filter scoreBin, chip "Clear filter". |
| **Recalculate all scores** | Nút cạnh tiêu đề histogram; gọi POST /risk/scores/sync; loading + message "Sync started for N resources. Refreshing in a few seconds…"; khi histogram trống hiện gợi ý amber. |
| **Risk Level Overview** | 4 ô severity (critical/high/medium/low) + Resolved (24h); click severity → navigate/filter. |
| **Risks by cluster** | Bảng khi scope = all clusters. |
| **Quick Links** | View All Findings, PCE Heatmap, Evidence & References. |

### 5.4 lib/api.ts (Risk Center)

- **getRiskHistogram(params):** GET /risk/histogram, params clusterId, sinceMinutes (mặc định 30).
- **syncRiskScores():** POST /risk/scores/sync, trả về `{ message, resources }`.

---

## 6. Agent

### 6.1 Vai trò với Risk Center

- Agent gửi SBOM/Pod/PCE (syncer, collector) lên Core qua gRPC/HTTP/NATS (tùy cấu hình).
- **Insights không do Agent tạo trực tiếp:** Core workers (CVE matcher, risk worker, historical evaluator, capability evaluator, scanner) đọc SBOM/pod/rule rồi gọi **InsightManager.CreateOrUpdateInsight / BatchCreateOrUpdateInsights** → từ đó Core tự schedule risk_scores và publish insights_updated.

### 6.2 Kiểm tra

- Agent không cần gọi POST /risk/scores/sync hay GET /risk/histogram.
- Đảm bảo Agent gửi đủ dữ liệu để Core tạo insights (SBOM, pod metadata, v.v.) theo pipeline hiện tại.

---

## 7. Checklist xác nhận nhanh

| # | Hạng mục | Trạng thái |
|---|----------|------------|
| 1 | DB: bảng risk_scores có đủ cột (total_score, priority_level, deleted_at, V2 columns) | ✅ Migrations 012, 013, 018 |
| 2 | Core: GetRiskHistogram trả đủ bins + totalFindings + averageScore + p0Count | ✅ dashboard_handlers.go |
| 3 | Core: GET /risk/insights hỗ trợ scoreBin, priorityLevel, withScores | ✅ RiskFilter, getInsightsListData |
| 4 | Core: POST /risk/scores/sync trả 202, chạy scorer trong background | ✅ risk_handlers.go SyncRiskScores |
| 5 | Core: CreateOrUpdateInsight / BatchCreateOrUpdateInsights gọi schedule/runRiskScoreCalculation | ✅ insight_manager.go |
| 6 | Core: fortuna.insights.updated → BroadcastRisksUpdate → xóa cache histogram | ✅ risks_ws_hub.go, main.go |
| 7 | Dashboard: fetchData gọi getRiskHistogram, hiển thị RiskHistogram | ✅ Insights.tsx |
| 8 | Dashboard: Nút "Recalculate all scores" gọi syncRiskScores, loading + message, refetch sau 4s | ✅ Insights.tsx + api.syncRiskScores |
| 9 | Dashboard: 4 KPI cards (Total, Critical/P0, Resolved 24h, Velocity) | ✅ Insights.tsx |
| 10 | Dashboard: Gợi ý khi histogram trống (totalFindings === 0) | ✅ Insights.tsx |

---

## 8. Tài liệu tham chiếu

- **Wireframe & API spec:** `Risk-Center-UI_UX-Wireframe-Specification.md` (§6 Risk Score Distribution, §7 As-Built).
- **Tổng quan component:** `RISK-CENTER-CONPONENT-OVERVIEW.md`.
- **Báo cáo & điều chỉnh:** `Risk-Center-Report-And-Wireframe-Adjustments.md`.
