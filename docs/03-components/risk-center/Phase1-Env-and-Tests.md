# Risk Center Phase 1 – Env vars & Unit tests

## Env vars (Core)

| Biến | Mặc định | Mô tả |
|------|----------|--------|
| `INSIGHTS_RESOLVED_RETENTION_DAYS` | 30 | Số ngày sau đó insights đã **resolved** bị soft-delete bởi InsightsCleanupJob. |
| `INSIGHTS_ACTIVE_RETENTION_DAYS` | 90 | Số ngày không cập nhật sau đó insights **active** bị soft-delete. |

Đặt trong deployment Core (ConfigMap/Secret hoặc env của container). Job chạy mỗi 24h.

## Unit tests (sau mỗi Phase)

- **Scheduler:** `core/internal/scheduler/insights_cleanup_job_test.go` – TestGetResolvedRetentionDays, TestGetActiveRetentionDays (env default và override).
- **API – Audit:** `core/internal/api/insights_audit_test.go` – TestCreateInsightAuditLog_NoUserInContext, TestCreateInsightAuditLog_WithUserInContext (ghi audit_logs).
- **API – Export:** `core/internal/api/risks_export_test.go` – TestExportRisksCSV_Empty, TestExportRisksCSV_WithInsights (CSV format và filter).

Chạy:

```bash
cd core
go test ./internal/scheduler/ -run 'TestGetResolvedRetentionDays|TestGetActiveRetentionDays' -v
go test ./internal/api/ -run 'TestCreateInsightAuditLog|TestExportRisksCSV' -v
```

## Cleanup

- Unit test dùng SQLite in-memory (`:memory:`) hoặc test DB; không để lại dữ liệu thật.
- Không có cache Risk Center ở Phase 1; cleanup image/cache theo quy trình CI/deploy của dự án.
