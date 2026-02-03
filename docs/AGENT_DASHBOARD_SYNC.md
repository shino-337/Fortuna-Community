# Cơ chế đồng bộ Agent → Dashboard

*Cập nhật: 2026-02-02*

## 1. Mục tiêu

Đảm bảo thông tin agent **luôn cập nhật kịp thời** lên Dashboard: số lượng agent (activeAgents) và danh sách Agent Status.

## 2. Luồng dữ liệu

```
Agent (DaemonSet)                         Core                              Dashboard
─────────────────                         ────                              ─────────
1. RegisterAgent (gRPC)  ────────────────► agents table: insert/update, last_seen_at = now

2. Ping (gRPC, mỗi HEARTBEAT_INTERVAL) ─► Cập nhật last_seen_at (upsert):
   - Có agent_id  → update row (last_seen_at, node_name, status, revive nếu soft-deleted)
   - Nếu không có row → tạo mới từ Ping (dashboard cập nhật ngay, không cần chờ Register)
   - Không agent_id → fallback: update WHERE node_name = ? (limit 1)

3. GET /api/v1/agents/status  ◄────────── Core: SELECT agents (tất cả ready), mỗi agent có status = healthy/slow/disconnected theo last_seen_at
   GET /api/v1/dashboard/stats             activeAgents = count(tất cả ready) → luôn khớp số pod
                                          ► Trả về agents[], total, activeAgents (luôn mới nhất)
```

## 3. Cấu hình

### 3.1 Core

| Env | Mặc định | Mô tả |
|-----|----------|--------|
| **ACTIVE_AGENT_CUTOFF_MINUTES** | 15 | Số phút gần đây: agent có `last_seen_at` trong khoảng này mới được coi là "active" và hiển thị trên dashboard. |

### 3.2 Agent

| Env | Mặc định | Mô tả |
|-----|----------|--------|
| **HEARTBEAT_INTERVAL** | 15s | Chu kỳ gọi Ping tới Core. Càng ngắn thì `last_seen_at` càng được cập nhật thường xuyên. Tối thiểu 5s. |
| **AGENT_ID** | (NODE_NAME + "-agent") | Định danh agent; dùng trong Ping. Nếu rỗng, Agent tự set NODE_NAME+"-agent". |
| **NODE_NAME** | (từ downward API) | Tên node; dùng trong Ping (và fallback cập nhật last_seen_at khi thiếu agent_id). |

## 4. Thay đổi đã áp dụng

### 4.1 Core

- **getActiveAgentCutoff()**: cutoff "active" đọc từ env **ACTIVE_AGENT_CUTOFF_MINUTES**, mặc định **15 phút** (trước đây cố định 10 phút). Dùng trong GetAgentStatus, GetDashboardStats, dashboard_data_integrity.
- **Ping handler** (upsert):
  - Cập nhật row theo **agent_id** (last_seen_at, node_name, status=ready, revive nếu soft-deleted).
  - **Nếu không có row**: tạo mới từ Ping (agent_id, node_name, status=ready, last_seen_at) → dashboard cập nhật kịp thời dù Register chưa chạy hoặc thất bại.
  - **Fallback**: nếu request **không có agent_id** (image Agent cũ) thì cập nhật theo **node_name** (update bản ghi agent đầu tiên có `node_name` trùng).

### 4.2 Agent

- **HeartbeatInterval**: cấu hình qua env **HEARTBEAT_INTERVAL**, mặc định **15s** (tối thiểu 5s) → last_seen_at được cập nhật thường xuyên. Nên giữ **HEARTBEAT_INTERVAL** nhỏ hơn nhiều so với **ACTIVE_AGENT_CUTOFF_MINUTES** (vd. 15s vs 15 phút).
- **pingCore()**: nếu **AgentID** rỗng thì dùng **NodeName + "-agent"** trước khi gửi Ping → luôn gửi agent_id (hoặc node_name cho fallback Core).
- **Log**: chỉ log "Heartbeat OK" mỗi 20 lần thành công (~5 phút khi interval=15s) để tránh flood log.

### 4.3 Deployment

- **fortuna-agent-daemonset.yaml**: thêm env **HEARTBEAT_INTERVAL**: "15s".
- **fortuna-core-deployment.yaml**: (tùy chọn) comment gợi ý **ACTIVE_AGENT_CUTOFF_MINUTES**.

## 5. Deploy: đảm bảo dashboard cập nhật kịp thời

- **Core**: rebuild và deploy Core (Ping handler upsert). Có thể set env **ACTIVE_AGENT_CUTOFF_MINUTES** (mặc định 15) nếu cần.
- **Agent**: rebuild image Agent (có gửi agent_id trong Ping), sau đó **recreate pod** để chạy image mới:
  - `kubectl rollout restart daemonset/fortuna-agent -n fortuna` hoặc
  - `kubectl delete pod -l app.kubernetes.io/component=agent -n fortuna` (DaemonSet sẽ tạo lại).
- **Quy tắc**: Giữ **HEARTBEAT_INTERVAL** (vd. 15s) nhỏ hơn nhiều so với **ACTIVE_AGENT_CUTOFF_MINUTES** (vd. 15 phút) để agent luôn nằm trong cửa sổ "active".

## 6. Kiểm tra nhanh

```bash
# DB: last_seen_at phải gần now (vài chục giây đến vài phút)
kubectl exec -n fortuna deploy/postgres -- psql -U postgres -d fortuna -c \
  "SELECT agent_id, node_name, last_seen_at, now() - last_seen_at AS age FROM agents;"

# API: total > 0, activeAgents > 0 (sau khi login lấy JWT)
curl -s -H "Authorization: Bearer <JWT>" http://localhost:8080/api/v1/agents/status
curl -s -H "Authorization: Bearer <JWT>" http://localhost:8080/api/v1/dashboard/stats
```

Sau khi rebuild và deploy Core + Agent với cơ chế mới, Dashboard sẽ hiển thị số agent và danh sách Agent Status cập nhật kịp thời (cutoff 15 phút, heartbeat 15s, Ping upsert + fallback theo node_name).

## 7. Kiểm tra Agent Availability (Dashboard vs thực tế)

Script so sánh **số agent thực tế** (pod Running) với **số agent Dashboard dùng** (DB trong cutoff, API):

```bash
./scripts/verify-agent-availability.sh
```

- **[1] Reality**: Số pod Agent đang Running (`kubectl get pods -l app.kubernetes.io/component=agent`).
- **[2] DB**: Bảng `agents` (tất cả) và số bản ghi trong cutoff (`last_seen_at > now() - 15 phút`) – đây là điều kiện API/Dashboard dùng.
- **[3] API**: `GET /api/v1/dashboard/stats` (activeAgents) và `GET /api/v1/agents/status` (total). Cần port-forward Core (8080) và JWT nếu bật auth.

**So sánh mong đợi:**

- **Khớp**: `Reality (pods)` = `DB active` = `API total/activeAgents` → Dashboard phản ánh đúng thực tế.
- **Lệch**: Pods > 0 nhưng DB active = 0 → `last_seen_at` cũ, Ping không cập nhật kịp. Cần đảm bảo Agent gửi Ping có `agent_id`, Core cập nhật `last_seen_at`, và pod Agent chạy image mới (rebuild + restart DaemonSet).

**Gọi API khi bật auth (JWT):**

```bash
# Lấy JWT (đổi user/pass theo env Core)
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin"}' | jq -r .token)
# Chạy verify kèm JWT
JWT="$TOKEN" ./scripts/verify-agent-availability.sh
```
