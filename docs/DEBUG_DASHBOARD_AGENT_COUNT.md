# Debug: Dashboard không hiển thị số lượng Agent

*Cập nhật: 2026-02-02*

## 1. Triệu chứng

- Dashboard (Monitoring, Stats) hiển thị **0** agent hoặc danh sách Agent trống.
- API `GET /api/v1/agents/status` trả về `agents: [], total: 0`.
- API `GET /api/v1/dashboard/stats` trả về `activeAgents: 0`.

## 2. Nguyên nhân đã xác định

### 2.1 Bộ lọc “active” (10 phút)

- **GetAgentStatus** và **GetDashboardStats** chỉ tính agent thỏa:
  - `deleted_at IS NULL`
  - `status = 'ready'`
  - `last_seen_at > (now - 10 phút) OR last_seen_at IS NULL`
- **getActiveAgentCutoff()**: mặc định **15 phút**, cấu hình qua env **ACTIVE_AGENT_CUTOFF_MINUTES** (Core). Nếu **last_seen_at** trong DB **cũ hơn cutoff** → agent bị **lọc ra** → API trả 0 agent.

### 2.2 last_seen_at không được cập nhật

- **RegisterAgent** (khi agent khởi động): ghi/update bảng `agents` và set **last_seen_at = now** (một lần).
- **Ping** (heartbeat ~30s): Core chỉ cập nhật **last_seen_at** khi request có **agent_id**.
- Nếu **Agent đang chạy image cũ** (không gửi `agent_id` trong PingRequest):
  - Core nhận Ping **không có** agent_id → **không** update last_seen_at.
  - Sau lần RegisterAgent đầu tiên, last_seen_at **không** được cập nhật nữa.
  - Sau 10 phút, bộ lọc “active” loại bỏ agent → Dashboard thấy 0.

### 2.3 Kiểm tra nhanh trong DB

```bash
kubectl exec -n fortuna deploy/postgres -- psql -U postgres -d fortuna -c \
  "SELECT agent_id, node_name, status, last_seen_at FROM agents;"
```

- Nếu **last_seen_at** cũ (vài giờ/ngày) → đúng với tình trạng “Ping không cập nhật last_seen_at”.

### 2.4 Log Core (debug)

- Code đã thêm log trong handler **Ping**:
  - Có **agent_id**: `[Agent] Ping: updated last_seen_at for agent_id=...`
  - **Không** có agent_id: `[Agent] Ping: received without agent_id (old agent image?); last_seen_at not updated`
- Nếu thấy dòng “received without agent_id” → Agent đang chạy **image cũ** (không gửi agent_id trong Ping).

## 3. Cách xử lý

### 3.1 Đảm bảo Agent chạy image mới (khuyến nghị)

- Image Agent **mới** (sau khi sửa code) gửi **AgentId** và **NodeName** trong **PingRequest**.
- Sau khi rebuild image Agent và load vào containerd, **ép pod Agent tạo lại** để dùng image mới:

```bash
kubectl delete pod -n fortuna -l app.kubernetes.io/component=agent
```

- DaemonSet sẽ tạo lại pod; pod mới dùng image **fortuna-agent:latest** (đã build mới).
- Agent mới: RegisterAgent → last_seen_at set; mỗi ~30s Ping có agent_id → Core update last_seen_at → trong vòng 10 phút API trả agent → Dashboard hiển thị đúng.

### 3.2 Workaround tạm (chỉ để test hiển thị)

- Cập nhật **last_seen_at** thủ công cho các agent trong DB (chỉ để kiểm tra UI/API):

```bash
kubectl exec -n fortuna deploy/postgres -- psql -U postgres -d fortuna -c \
  "UPDATE agents SET last_seen_at = NOW() WHERE status = 'ready';"
```

- Sau đó gọi lại `GET /api/v1/agents/status` và `GET /api/v1/dashboard/stats`: sẽ thấy activeAgents > 0 cho đến khi quá 10 phút mà Ping vẫn không gửi agent_id.

## 4. Luồng đầy đủ (sau khi sửa)

1. Agent (image mới) khởi động → **RegisterAgent** → Core insert/update `agents`, set **last_seen_at = now**.
2. Mỗi ~30s Agent gọi **Ping(AgentId, NodeName)** → Core update **agents.last_seen_at** cho đúng agent_id.
3. Dashboard gọi **GET /api/v1/agents/status** và **GET /api/v1/dashboard/stats** → Core đọc `agents` với điều kiện **last_seen_at > (now - 10 phút)** → trả về danh sách agent và **activeAgents**.
4. Dashboard hiển thị số agent và danh sách Agent Status.

## 5. Kết quả kiểm tra (2026-02-02)

- **DB**: 2 agent (k8s-master-agent, k8s-worker01-agent), **last_seen_at** cũ (2026-01-31) → bị lọc bởi điều kiện “trong 10 phút” → API trả 0 agent.
- **Workaround đã chạy**: `UPDATE agents SET last_seen_at = NOW()` → API ngay sau đó trả **2 agent**, **dashboard/stats** trả **activeAgents: 2**.
- **Pod Agent**: Đã xóa pod để DaemonSet tạo lại; sau khi tạo lại, nếu image Agent **mới** (có gửi agent_id trong Ping) thì **last_seen_at** sẽ được Core cập nhật mỗi ~30s. Nếu **last_seen_at** vẫn không đổi sau vài phút → Agent đang chạy image cũ; cần rebuild Agent (ví dụ `NO_CACHE=true`) rồi xóa pod Agent lại.

## 6. Lệnh kiểm tra nhanh

```bash
# Số agent trong DB và last_seen_at
kubectl exec -n fortuna deploy/postgres -- psql -U postgres -d fortuna -c \
  "SELECT agent_id, node_name, last_seen_at FROM agents;"

# Log Core (Ping có/không agent_id)
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=100 | grep -E '\[Agent\]|Ping'

# Log Agent (Heartbeat)
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=20

# API agents/status (cần JWT)
# Sau khi port-forward: curl -s -H "Authorization: Bearer <JWT>" http://localhost:8080/api/v1/agents/status
```
