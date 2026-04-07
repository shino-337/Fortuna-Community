# Risk Center – Kiểm tra thay đổi và Clean/Rebuild/Redeploy

**Go-Live Criteria:** Xem [Risk-Center-Go-Live-Criteria.md](./Risk-Center-Go-Live-Criteria.md) cho danh sách Must-have / Should-have và action items.

---

## Các thay đổi đã đưa vào (cần có trên image mới)

### Cơ bản (trước đây)

| Thành phần | File / API | Nội dung |
|------------|------------|----------|
| **Core** | `core/internal/api/risks_cache.go` | `BuildInsightsSummaryGlobalCacheKey` |
| **Core** | `core/internal/api/insights_handlers.go` | `GetInsightsSummaryGlobalCached`, audit "view" trong GetInsight |
| **Core** | `core/internal/api/routes.go` | GET `/insights/summary/global` |
| **Core** | `core/README.md` | Document env INSIGHTS_RESOLVED_RETENTION_DAYS, INSIGHTS_ACTIVE_RETENTION_DAYS |
| **Dashboard** | `dashboard/lib/api.ts` | `getInsightsSummaryGlobal()` |
| **E2E** | `scripts/e2e/e2e-risk-center-full.sh` | TC-05b GET /insights/summary/global |

### Phase 3 + Histogram (bổ sung)

| Thành phần | File / API | Nội dung |
|------------|------------|----------|
| **Core** | `core/internal/api/dashboard_handlers.go` | GetRiskHistogram, ExportRisksCSV streaming (chunk 500), RiskFilter.ScoreBin, roundPercent |
| **Core** | `core/internal/api/risks_cache.go` | BuildRiskHistogramCacheKey; BuildRisksListCacheKey + scoreBin |
| **Core** | `core/internal/api/risks_ws_hub.go` | ClearByPrefix("risk:histogram:"); WS rate limit (RISKS_WS_MAX_CONNS_PER_IP) |
| **Core** | `core/internal/api/routes_risk.go` | GET `/risk/histogram` |
| **Core** | `core/pkg/evidence/mask.go` | MaskSensitiveInJSON (evidence masking) |
| **Core** | `core/pkg/riskengine/insight_manager.go` | Gọi evidence.MaskSensitiveInJSON trước khi ghi Evidence/ViolatedRules |
| **Core** | `core/pkg/capability/state_controller.go` | last_seen_at trong OnConflict DoUpdates |
| **Core** | `core/internal/scheduler/pce_cleanup_job.go` | PCECleanupJob (PCE_CLEANUP_RETENTION_DAYS) |
| **Core** | `core/cmd/main.go` | Khởi chạy PCECleanupJob (goroutine) |
| **Core** | `core/pkg/metrics/metrics.go` (pkg) | RiskEvaluationDuration, InsightsBatchSize |
| **Core** | `core/pkg/models/risk_rule_history.go` | RiskRuleHistory model |
| **Core** | `core/migrations/076_add_risk_rules_history.go` | Bảng risk_rules_history |
| **Core** | `core/migrations/migrations.go` | Migration076_AddRiskRulesHistoryTable |
| **Core** | `core/internal/api/risk_rules_handlers.go` | Snapshot vào risk_rules_history trước UpdateRiskRule |
| **Dashboard** | `dashboard/lib/api.ts` | getRiskHistogram(), getRisks(scoreBin) |
| **Dashboard** | `dashboard/types.ts` | RiskHistogramBin, RiskHistogramResponse |
| **Dashboard** | `dashboard/components/RiskHistogram.tsx` | Recharts BarChart stacked, ReferenceLine, tooltip |
| **Dashboard** | `dashboard/pages/Insights.tsx` | Histogram data/state, selectedScoreBin, RiskHistogram (Overview + Findings) |

---

## Kiểm tra sau khi redeploy

1. **Core pod đang chạy image mới**
   - `kubectl get pods -n fortuna -l app=fortuna-core -o jsonpath='{.items[0].status.containerStatuses[0].imageID}'`
   - So sánh với imageID trước khi rebuild (sẽ khác nếu rebuild thành công).

2. **API global-summary**
   - Từ trong cluster: `kubectl exec -n fortuna deployment/fortuna-core -- curl -s -H "Authorization: Bearer <JWT>" http://localhost:8080/api/v1/insights/summary/global`
   - Hoặc sau khi login Dashboard, gọi GET `/api/v1/insights/summary/global` (hoặc dùng E2E TC-05b).

3. **API histogram (Phase 3)**
   - `curl -s -H "Authorization: Bearer <JWT>" "http://localhost:8080/api/v1/risk/histogram?sinceMinutes=30"`
   - Kỳ vọng: 200, JSON có `bins` (mảng 10 phần tử), `totalFindings`, `averageScore`, `p0Count`.

4. **Migrations**
   - Bảng `risk_rules_history` tồn tại: `SELECT 1 FROM risk_rules_history LIMIT 1;` (có thể 0 rows).
   - PCE cleanup job: log Core có `[PCECleanupJob] Started` (sau khi DB ready).

5. **Audit view**
   - Mở Risk Center → click một risk (mở chi tiết) → kiểm tra bảng `audit_logs`: có bản ghi `action='view'`, `resource='insight'`, `resource_id=<id>`.

6. **E2E**
   - `./scripts/e2e/e2e-risk-center-full.sh` → TC-05b PASS (và các TC khác).
   - TC-05d/05e: `GET /dashboard/stats?byType=all` và `GET /dashboard/metrics/threat-velocity?byType=all&days=7` → 200, body hợp lệ (Phase 7.2).
   - (Khi có) TC GET /risk/histogram → 200, body có bins.

7. **Dashboard**
   - `kubectl get pods -n fortuna -l app=fortuna-dashboard` → imageID mới; footer build timestamp mới.
   - Risk Center → tab Risk Findings: có block "Risk Score Distribution", click bar → bảng filter theo score bin, chip "Clear filter".

---

## Env vars (Phase 3 + go-live)

Bảng đầy đủ và gợi ý theo môi trường (dev/stg/prod): **[Risk-Center-Ops-Env.md](./Risk-Center-Ops-Env.md)**. Khi có helm/ops repo: copy bảng env vào values hoặc ops doc.

| Env | Mặc định | Mô tả |
|-----|----------|--------|
| `PCE_CLEANUP_RETENTION_DAYS` | 30 | Số ngày giữ pod_capabilities (xóa bản ghi last_seen_at cũ hơn) |
| `RISKS_WS_MAX_CONNS_PER_IP` | 10 | Số kết nối WebSocket /ws/risks tối đa mỗi IP |
| `INSIGHTS_RESOLVED_RETENTION_DAYS` | 30 | Insights resolved soft-delete sau N ngày |
| `INSIGHTS_ACTIVE_RETENTION_DAYS` | 90 | Insights active chưa cập nhật soft-delete sau N ngày |

---

## Kết quả lần chạy (sau clean + rebuild + redeploy)

- **Clean:** Đã xóa toàn bộ image fortuna (core, agent, dashboard) và prune cache.
- **Rebuild:** fortuna-core, fortuna-agent, fortuna-dashboard đã build lại (nerdctl); Core chứa GetInsightsSummaryGlobalCached, audit "view", route /insights/summary/global; Dashboard chứa getInsightsSummaryGlobal.
- **Redeploy:** Đã xóa pod Core và Dashboard để force tạo pod mới dùng image mới. Dashboard pod đang chạy image mới (imageID sha256:146b6c00bf33...). Core pod đã tạo lại (có thể đang Ready=False do khởi động/migrations).
- **Sau khi Core Ready:** Chạy `./scripts/e2e/e2e-risk-center-full.sh` để xác nhận TC-05b (GET /insights/summary/global) PASS.
