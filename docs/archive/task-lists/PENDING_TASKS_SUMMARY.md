# Tổng hợp task chưa hoàn thiện

*Cập nhật: 2026-02-02. Gộp từ TODO_LIST.md, UI_AND_PENDING_TASKS.md và grep TODO/FIXME trong code.*

---

## 1. Core – API / Backend

| Task | Mô tả | Nguồn |
|------|--------|--------|
| GetWorkerMetrics / GetQueueMetrics / API Latency | Đã loại bỏ tạm thời (route + UI). Dùng Prometheus khi cần. | ✅ Removed |
| GetNotifications | Đã lấy từ bảng `notifications` (migration 056). | ✅ Done |
| GET /api/v1/users | Đã có route; lấy từ bảng users (admin only khi auth). | ✅ Done |
| GET /api/v1/certificates/rotation/history | Đã có route; stub trả về [] cho đến khi có bảng rotation_history. | ✅ Done |
| Error logs API | Dashboard gọi getErrorLogs; endpoint chưa có. Thêm endpoint (bảng hoặc log aggregation). | TODO_LIST |
| gRPC handler | Replace placeholder với actual proto types sau khi generate từ proto file. | core/internal/grpc/handler.go |
| gRPC SBOM version | Get version từ build info thay vì hardcode "1.0.0". | core/internal/grpc/handler_sbom.go |
| gRPC combined | ClusterID "default" → lấy từ config. | core/internal/grpc/handler_combined.go |
| Rate limiting | Implement proper rate limiting (Redis hoặc in-memory). | core/internal/middleware/security.go |
| DLQ alert | Gửi alert tới monitoring khi dead-letter. | core/pkg/worker/dlq.go |
| CVE database update | Database update requested (not implemented yet). | core/pkg/cve/database/manager.go |
| CVE matcher RPM | Implement full RPM version comparison. | core/pkg/cve/matcher/version_comparator.go |
| Policy events | Generate insights, send alerts, remediation event processing (future). | core/pkg/policy/events.go |
| Risk analytics | Implement actual correlation calculation khi có data sources. | core/internal/api/risk/analytics_handlers.go |

---

## 2. Database migrations

| Task | Mô tả | Trạng thái |
|------|--------|------------|
| notifications | Đã có bảng (migration 056); API GetNotifications trả về từ DB. | ✅ Done |
| rotation_history | Migration cho lịch sử xoay cert nếu cần. | ⬜ TODO |
| error_logs | Đã có bảng (migration 057); API GetErrorLogs trả về từ DB. Core/Agent có thể ghi lỗi vào bảng khi cần. | ✅ Done |

---

## 3. Agent

| Task | Mô tả | Trạng thái |
|------|--------|------------|
| Heartbeat / registration | Core đã implement RegisterAgent (handler_sbom); cập nhật `agents` (last_seen_at, status). Agent gọi gRPC RegisterAgent. | ✅ Verified |
| Workers/queue metrics | (Tùy chọn) Agent gửi metrics workers/queue nếu agent quản lý queue. | ⬜ TODO |
| SBOM processor | Handle registry URLs đúng (e.g. registry.io/namespace/image:tag). | agent/internal/sbom/processor.go |
| RPM parsing | Implement full RPM database parsing. | agent/pkg/sbom/extractor/rpm.go, parsers/rpm.go |

---

## 4. Dashboard – Frontend

| Task | Mô tả | Vị trí |
|------|--------|--------|
| getUsers | API endpoint chưa có; hiện comment TODO. | dashboard/lib/api.ts |
| getRotationHistory | API endpoint chưa có; comment TODO. | dashboard/lib/api.ts |
| CapabilityStateChart | API endpoint để lấy state history chưa có. | dashboard/components/CapabilityStateChart.tsx |

---

## 5. API đã bổ sung (2026-02-02)

- `GET /api/v1/users` – từ bảng users (admin only khi AUTH_ENABLED).
- `GET /api/v1/certificates/rotation/history` – stub trả về `{ history: [], total: 0 }` cho đến khi có bảng rotation_history.

---

## 6. Generated / Unimplemented (không ưu tiên sửa code)

- `api/proto/agent/service_grpc.pb.go`: Các method RegisterAgent, Heartbeat, SendSBOMFinding, … trả về `Unimplemented` – cần implement server hoặc bỏ gọi từ client.

---

## 7. Tài liệu tham chiếu

- **TODO_LIST.md** – Chi tiết Core/DB/Agent/Dashboard.
- **UI_AND_PENDING_TASKS.md** – Kiểm tra UI và đồng bộ API.
- Script chính: `./scripts/pipeline/full-clean-database-rebuild-deploy.sh` cho clean + rebuild + redeploy (--db / --db-reset tùy chọn).

---

## 8. Dọn dẹp đã thực hiện (2026-02-02)

### Documents đã xóa (old / one-time reports)
- CLEAN_REBUILD_DEPLOY_E2E_SUMMARY, DASHBOARD_* (Build, Complete Rebuild, Data Audit, Data Timing, Fixes Complete, Image Issue, Progress Report, Rebuild Complete, Update Final), DASHBOARD_401_CHECKLIST
- DOCUMENTATION_CLEANUP_SUMMARY, DOCUMENTATION_STRUCTURE, DEPLOYMENT_STATUS
- ENVIRONMENT_AUDIT_REPORT, ENVIRONMENT_CHECK_REPORT
- JSONB_UNIFICATION_REPORT, MIGRATION_CLEANUP_REPORT
- RISK_CENTER_UI_ANALYSIS, SBOM_* (Flow, Sync Diagnosis, UI Fix), STORAGE_BEHAVIOR_ANALYSIS
- GLOBAL_PROMPT–AI_AGENT_SELF-VERIFICATI, PodCapabilityEngine_and_attack-path (trùng 03-components), PRODUCTION_READINESS_SUMMARY
- **docs/archive/** – toàn bộ (implementation-plans, fix/phase reports)
- **docs/test-results/** – tất cả file E2E-*.md (giữ README.md)

### Scripts đã xóa (one-time rebuild)
- complete-dashboard-rebuild.sh, dashboard-nuclear-rebuild.sh, force-dashboard-update.sh

### Git / code (2026-02-02)
- **File backup .OLD đã xóa:** `core/pkg/riskengine/insight_manager.go.OLD`, `agent/internal/client/grpc_client.go.OLD`, `agent/internal/client/grpc_client_combined.go.OLD`, `agent/internal/watcher/watcher.go.OLD`.
- **core/migrations/archive/ đã xóa:** toàn bộ (age/, mvp2/, mvp3/) – migration hiện dùng mvp2_migrations.go và migrations đánh số 001–055.
- **agent/docs/test-results/** – báo cáo one-time đã xóa.
- **.gitignore:** thêm `*.OLD`, `core/migrations/archive/`; bỏ tham chiếu tới docs đã xóa.
