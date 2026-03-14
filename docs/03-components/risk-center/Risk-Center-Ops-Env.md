# Risk Center – Biến môi trường (Ops)

Tài liệu tập trung các biến môi trường liên quan Risk Center. Khi có repo **helm/ops**, copy bảng này vào values hoặc ops doc tương ứng.

---

## Bảng env

| Env | Mặc định | Mô tả | Gợi ý theo môi trường |
|-----|----------|--------|------------------------|
| `PCE_CLEANUP_RETENTION_DAYS` | `30` | Số ngày giữ bản ghi `pod_capabilities`; job xóa bản ghi có `last_seen_at` cũ hơn. | dev: 7–14; stg: 30; prod: 30–90. |
| `RISKS_WS_MAX_CONNS_PER_IP` | `10` | Số kết nối WebSocket `/ws/risks` tối đa mỗi IP (rate limit). | dev: 20; stg: 10; prod: 10. |
| `INSIGHTS_RESOLVED_RETENTION_DAYS` | `30` | Insights đã resolved: soft-delete sau N ngày (InsightsCleanupJob). | dev: 14; stg/prod: 30. |
| `INSIGHTS_ACTIVE_RETENTION_DAYS` | `90` | Insights active chưa cập nhật: soft-delete sau N ngày. | dev: 30; stg/prod: 90. |

---

## API / Metrics liên quan (Phase 3)

- **GET** `/api/v1/risk/histogram` – cache key có `clusterId`, `sinceMinutes`; invalidate khi `fortuna.insights.updated`.
- **Prometheus:** `RiskEvaluationDuration`, `InsightsBatchSize` (đã expose; dashboard Grafana → Backlog).

---

## Tham chiếu

- Core: `core/README.md`, `core/internal/scheduler/pce_cleanup_job.go`, `core/internal/api/risks_ws_hub.go`.
- Checklist: [Risk-Center-Verify-And-Deploy-Checklist.md](./Risk-Center-Verify-And-Deploy-Checklist.md).
