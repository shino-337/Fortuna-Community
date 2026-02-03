# To-Do List – Agent, Core, Database Migrations, API

*Cập nhật: 2026-02-02. Tổng hợp đầy đủ: **docs/PENDING_TASKS_SUMMARY.md**.*

---

## 1. Core – API mới / sửa

| Mục | Mô tả | Trạng thái |
|-----|--------|------------|
| Resolved (24h) | `/dashboard/stats` đã trả về `resolved24h` từ bảng insights (status=resolved, updated_at 24h). | ✅ Done |
| GetAgentStatus | Đã lấy từ bảng `agents` (node_name, status, last_seen_at). | ✅ Done |
| GetWorkerMetrics / GetQueueMetrics / API Latency | Đã loại bỏ tạm thời (route + UI). | ✅ Removed |
| GetNotifications | Đã từ bảng notifications (migration 056). | ✅ Done |
| GET /users | Đã có route; từ bảng users (admin only khi auth). | ✅ Done |
| GET /certificates/rotation/history | Đã có route; stub trả về [] cho đến khi có bảng rotation_history. | ✅ Done |
| Error logs API | Dashboard gọi getErrorLogs; endpoint chưa có. Thêm endpoint lấy error logs (bảng hoặc log aggregation). | ⬜ TODO |

---

## 2. Database migrations

| Bảng / Mục | Mô tả | Trạng thái |
|------------|--------|------------|
| agents | Đã có (migration 038). | ✅ Done |
| insights | Có status, resolved_at, updated_at. Dùng cho resolved24h. | ✅ Done |
| notifications | Đã có (migration 056). | ✅ Done |
| rotation_history | Nếu cần lịch sử xoay cert → migration. | ⬜ TODO |
| error_logs | Đã có (migration 057); API trả về từ DB. | ✅ Done |

---

## 3. Agent

| Mục | Mô tả | Trạng thái |
|-----|--------|------------|
| Heartbeat / registration | Đảm bảo agent gửi heartbeat/register vào bảng `agents` (last_seen_at, status) để GetAgentStatus và dashboard/stats đồng bộ. | ⬜ Verify |
| Workers/queue metrics | (Tùy chọn) Agent gửi metrics workers/queue nếu agent quản lý queue. | ⬜ TODO |

---

## 4. Dashboard – Đã xong

- Resolved (24h): từ `api.getStats().resolved24h`.
- Workers / API Latency: hiển thị "—" cho đến khi Core có API.
- Queue Depth: từ `api.getQueueMetrics()` (tổng normalizer+correlator+risk) hoặc "—".
- Agents: từ `api.getAgents()` (Core đã dùng bảng agents).

---

## 5. Scripts

- **Full clean + rebuild + redeploy**: `./scripts/full-clean-database-rebuild-deploy.sh` (--db hoặc --db-reset tùy chọn)  
  - Clean: xóa toàn bộ image fortuna, cache, port-forwards.  
  - Rebuild: core, agent, dashboard.  
  - Redeploy: infra, core, agent, dashboard.  
- Tùy chọn: `--skip-clean`, `--skip-rebuild`, `--skip-deploy`, `--db` (clean E2E data trong DB).
