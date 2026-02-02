# To-Do List – Agent, Core, Database Migrations, API

*Cập nhật: 2026-02-02. Tổng hợp đầy đủ: **docs/PENDING_TASKS_SUMMARY.md**.*

---

## 1. Core – API mới / sửa

| Mục | Mô tả | Trạng thái |
|-----|--------|------------|
| Resolved (24h) | `/dashboard/stats` đã trả về `resolved24h` từ bảng insights (status=resolved, updated_at 24h). | ✅ Done |
| GetAgentStatus | Đã lấy từ bảng `agents` (node_name, status, last_seen_at). | ✅ Done |
| GetWorkerMetrics | Thay hardcode 3/3 bằng dữ liệu thật (NATS/Prometheus) hoặc trả về "unknown". | ⬜ TODO |
| GetQueueMetrics | Hiện trả về 0,0,0. Thay bằng queue depth thật (NATS/worker) hoặc giữ 0 và ghi rõ. | ⬜ TODO |
| API Latency | GetSystemMetrics có avgLatency hardcode "234ms". Thay bằng metrics thật hoặc bỏ. | ⬜ TODO |
| GetNotifications | Hiện stub `{ notifications: [], total: 0 }`. Khi có bảng notifications → trả về từ DB. | ⬜ TODO |
| GET /users | Chưa có route. Implement khi có bảng users. | ⬜ TODO |
| GET /certificates/rotation/history | Chưa có route. Implement khi có bảng rotation history. | ⬜ TODO |
| Error logs API | Dashboard gọi getErrorLogs; endpoint chưa có. Thêm endpoint lấy error logs (bảng hoặc log aggregation). | ⬜ TODO |

---

## 2. Database migrations

| Bảng / Mục | Mô tả | Trạng thái |
|------------|--------|------------|
| agents | Đã có (migration 038). | ✅ Done |
| insights | Có status, resolved_at, updated_at. Dùng cho resolved24h. | ✅ Done |
| notifications | Chưa có. Migration tạo bảng nếu product cần notifications. | ⬜ TODO |
| rotation_history | Nếu cần lịch sử xoay cert → migration. | ⬜ TODO |
| error_logs | Bảng hoặc view cho error logs nếu API cần. | ⬜ TODO |

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

- **Full clean + rebuild + redeploy**: `./scripts/full-clean-rebuild-redeploy.sh`  
  - Clean: xóa toàn bộ image fortuna, cache, port-forwards.  
  - Rebuild: core, agent, dashboard.  
  - Redeploy: infra, core, agent, dashboard.  
- Tùy chọn: `--skip-clean`, `--skip-rebuild`, `--skip-deploy`, `--db` (clean E2E data trong DB).
