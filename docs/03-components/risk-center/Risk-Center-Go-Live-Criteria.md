# Risk Center – Go-Live Criteria

Tài liệu này định nghĩa **tiêu chí go-live** cho Risk Center: mục bắt buộc (Must-have), khuyến nghị trước khi ra production (Should-have) và cải tiến sau launch (Nice-to-have). Kèm trạng thái hiện tại và action items.

**Tham chiếu:** [Risk-Center-Verify-And-Deploy-Checklist.md](./Risk-Center-Verify-And-Deploy-Checklist.md) (kiểm tra sau deploy), [Risk-Center-UI_UX-Wireframe-Specification.md](./Risk-Center-UI_UX-Wireframe-Specification.md) (wireframe as-built).

---

## 1. Must-have (Blocker – không go-live nếu chưa xong)

| # | Mục | Mô tả | Trạng thái | Ghi chú |
|---|-----|--------|------------|---------|
| M1 | **Core API – Insights & List** | GET /risk/insights với filter, pagination, cache | ✅ Done | GetInsightsListCached, RiskFilter (severity, status, clusterId, namespace, type, sinceMinutes, priorityLevel, scoreBin) |
| M2 | **Core API – Summary & Export** | GET summary (by-cluster, global), GET export CSV (streaming) | ✅ Done | Summary cached; export chunked 500, limit 10k |
| M3 | **Real-time** | WebSocket /ws/risks + cache invalidation khi insights update | ✅ Done | BroadcastRisksUpdateWithPayload; ClearByPrefix risks:list, insights:summary, risk:histogram |
| M4 | **Dashboard – 4 routes** | /risks, /risks/findings, /risks/pce, /risks/evidence | ✅ Done | useLocation + tab sync trong Insights.tsx |
| M5 | **Dashboard – Findings** | Bảng findings, filter, sort, pagination, click row → drawer | ✅ Done | Bulk actions (Acknowledge/Resolve/Dismiss selected) |
| M6 | **Drawer** | Risk Detail Drawer 3 tab (Summary, Evidence & Audit, PCE & Attack Path) + bottom bar actions | ✅ Done | Audit Trail trong tab Evidence & Audit |
| M7 | **Auth & WS** | WebSocket dùng auth khi bật; rate limit theo IP | ✅ Done | RisksWS trong group v1 (AuthMiddleware khi cfg.AuthEnabled); RISKS_WS_MAX_CONNS_PER_IP |
| M8 | **Migrations** | DB migrations chạy không lỗi (insights, risk_scores, risk_rules, risk_rules_history, pod_capabilities) | ✅ Done | 076 risk_rules_history; 046 capability state/last_seen |
| M9 | **E2E cơ bản** | Ít nhất 1 E2E script chạy qua: GET /risk/insights, summary, export, WS | ✅ Done | e2e-risk-center-full.sh (16 TCs); TC-05b global summary |

**Kết luận Must-have:** Tất cả đã đáp ứng. Không còn blocker kỹ thuật cho go-live.

---

## 2. Should-have (Khuyến nghị trước go-live)

| # | Mục | Mô tả | Trạng thái | Action |
|---|-----|--------|------------|--------|
| S1 | **Env & Ops doc** | Document env: PCE_CLEANUP_RETENTION_DAYS, RISKS_WS_MAX_CONNS_PER_IP, INSIGHTS_*_RETENTION_DAYS | ✅ Done | Core README đã có; Verify-And-Deploy-Checklist có bảng env |
| S2 | **Histogram E2E** | E2E test GET /risk/histogram → 200, body có bins, totalFindings | ✅ Done | TC-05c trong e2e-risk-center-full.sh |
| S3 | **Export filter đồng bộ** | Export CSV/PDF dùng đúng filter (namespace, type) như list | ✅ Done | ResourceNamespace trong ExportRisksCSV |
| S4 | **Evidence masking** | Không lưu plaintext password/token trong evidence | ✅ Done | evidence.MaskSensitiveInJSON trong createInsightTx |
| S5 | **PCE cleanup** | Job xóa pod_capabilities cũ (last_seen_at) | ✅ Done | PCECleanupJob, PCE_CLEANUP_RETENTION_DAYS |
| S6 | **Observability** | Prometheus metrics cho risk pipeline | ✅ Done | RiskEvaluationDuration, InsightsBatchSize (pkg/metrics) |
| S7 | **Rule versioning** | Lưu lịch sử khi sửa risk rule | ✅ Done | risk_rules_history, snapshot trước UpdateRiskRule |

**Action nên làm trước go-live:** Đã đủ (S1 doc env trong Core README + Checklist; S2 TC-05c E2E histogram).

---

## 3. Nice-to-have (Sau go-live)

| # | Mục | Mô tả | Trạng thái |
|---|-----|--------|------------|
| N1 | **Delta WebSocket** | WS gửi payload changed_ids; client chỉ refetch/merge thay vì full refetch | ⬜ Open |
| N2 | **Quick Links** | Block 3 card trên Overview: View Findings / PCE Heatmap / Evidence | ⬜ Open |
| N3 | **Sidebar filter** | Filter cố định 280px bên trái Findings (thay filter inline) | ⬜ Open |
| N4 | **Grafana dashboard** | Dashboard mẫu cho fortuna_risk_*, fortuna_insights_batch_size | ⬜ Open |
| N5 | **Unified scoring** | PCE đóng góp vào total_score; hoặc cột score trực tiếp trên insights | ⬜ Open |
| N6 | **Histogram từ Overview** | Click bar trên Overview → navigate /risks/findings + set scoreBin | 🟡 Partial | Hiện click bar set state; nếu đang /risks có thể navigate + set state |

---

## 4. Checklist theo khu vực

### 4.1 Backend (Core)

- [x] GET /risk/insights (list, filter, pagination, withScores, scoreBin)
- [x] GET /risk/insights/summary, /by-cluster, /global
- [x] GET /risk/histogram (bins, totalFindings, averageScore, p0Count)
- [x] GET /risk/insights/export (CSV streaming, PDF/HTML)
- [x] POST /risk/insights/bulk (acknowledge/resolve/dismiss)
- [x] WebSocket /ws/risks + cache invalidation (list, summary, histogram)
- [x] Evidence masking (MaskSensitiveInJSON)
- [x] PCE last_seen_at + PCECleanupJob
- [x] Rule versioning (risk_rules_history)
- [x] WS rate limit (per IP)
- [x] Prometheus: RiskEvaluationDuration, InsightsBatchSize

### 4.2 Dashboard (React)

- [x] 4 routes: /risks, /risks/findings, /risks/pce, /risks/evidence
- [x] Risk Findings: table, filters, sort, bulk actions, Export CSV/PDF
- [x] Risk Score Distribution histogram (Recharts), click bar → filter scoreBin
- [x] Risk Detail Drawer: 3 tabs, Audit Trail, bottom bar Acknowledge/Resolve/Dismiss
- [x] WebSocket refetch khi insights_updated
- [ ] (Optional) Quick Links trên Overview
- [ ] (Optional) Navigate to Findings khi click histogram từ /risks

### 4.3 Ops & Deploy

- [x] Migrations 075, 076 chạy trong RunMigrations
- [x] PCE cleanup job chạy trong main (goroutine)
- [ ] Document env: PCE_CLEANUP_RETENTION_DAYS, RISKS_WS_MAX_CONNS_PER_IP
- [ ] (Optional) Helm values / Runbook cập nhật

### 4.4 Test

- [x] Unit: risks_cache, insights_audit, risks_export, evidence/mask, risk_rules, scheduler (insights + PCE cleanup)
- [x] E2E: e2e-risk-center-full.sh (TC-01–TC-16)
- [ ] E2E: thêm TC GET /risk/histogram
- [ ] (Optional) E2E: bulk action (POST /risk/insights/bulk)

---

## 5. Action items (để đạt go-live đầy đủ)

| Ưu tiên | Action | Owner | Effort |
|---------|--------|-------|--------|
| P0 | ~~Document env vars~~ (đã có trong Core README + Risk-Center-Verify-And-Deploy-Checklist) | — | Done |
| P1 | ~~Thêm E2E test GET /risk/histogram~~ (đã có TC-05c) | — | Done |
| P2 | Cập nhật Risk-Center-Verify-And-Deploy-Checklist với Phase 3 + histogram (migrations, API mới) | Dev | ~20 phút |

---

## 6. Sign-off go-live (mẫu)

| Role | Tên | Ký (ngày) | Ghi chú |
|------|-----|-----------|---------|
| Dev Lead | | | Must-have + Should-have S1,S2 done |
| QA | | | E2E pass; histogram TC added |
| Ops | | | Env documented; migrations verified |

---

*Phiên bản: 1.0. Cập nhật khi bổ sung tiêu chí hoặc thay đổi trạng thái.*
