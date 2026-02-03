# Phân tích: Dashboard không hiển thị thông tin Agent

*Cập nhật: 2026-02-02*

## 1. Luồng dữ liệu Agent → Dashboard

```
Agent (DaemonSet)                    Core                              Dashboard
─────────────────                    ────                              ─────────
1. RegisterAgent (gRPC)  ──────────► agents table (insert/update)
   → last_seen_at = now, status = ready

2. Ping (gRPC, mỗi ~30s) ──────────► [Cần] cập nhật last_seen_at
   (phải gửi agent_id)              cho đúng agent_id

3. GET /api/v1/agents/status  ◄──── Core đọc agents table
   Dashboard gọi API                 Lọc: status='ready' AND
                                     (last_seen_at > 10 phút trước OR last_seen_at IS NULL)
                                     ► Trả về agents[], total, healthy, slow, disconnected
```

## 2. Nguyên nhân Dashboard không thấy Agent

### 2.1 Bộ lọc API GetAgentStatus (Core)

- **File**: `core/internal/api/metrics_handlers.go`
- **Điều kiện**: `deleted_at IS NULL AND status = 'ready' AND (last_seen_at > tenMinutesAgo OR last_seen_at IS NULL)`
- **Hệ quả**: Chỉ agent có `last_seen_at` trong vòng **10 phút** (hoặc NULL) mới được trả về. Sau 10 phút không cập nhật → bị lọc ra → Dashboard thấy `agents: []`.

### 2.2 last_seen_at chỉ được set một lần (trước khi sửa)

- **RegisterAgent** (khi agent khởi động): Core insert/update bảng `agents` và set `last_seen_at = now`.
- **Ping** (heartbeat ~30s): Trước khi sửa, handler **không** cập nhật `last_seen_at`. Do đó sau 10 phút, điều kiện `last_seen_at > tenMinutesAgo` sai → agent biến khỏi danh sách.

### 2.3 Đã sửa trong code (cần deploy image mới)

1. **Proto + Core**  
   - `PingRequest` có thêm `agent_id`, `node_name`.  
   - Handler **Ping** trong `core/internal/grpc/handler_sbom.go`: khi `req.AgentId != ""` thì update `agents.last_seen_at` cho đúng `agent_id`.

2. **Agent**  
   - Trong `agent/cmd/main.go`, **Ping** gửi `AgentId: cfg.AgentID`, `NodeName: cfg.NodeName`.  
   - `AgentID` = env `AGENT_ID` hoặc mặc định `NODE_NAME + "-agent"` (trong deploy không set `AGENT_ID` → mỗi node có id duy nhất, ví dụ `k8s-master-agent`).

3. **Dashboard**  
   - `dashboard/lib/api.ts` map response: `agentId` → `id`, `nodeName` → `node`, `status` (healthy → up, slow → slow, disconnected → down), `lastHeartbeat`.

### 2.4 Deploy đang chạy image cũ

- Nếu cluster đang dùng **image Core/Agent cũ** (chưa có fix Ping + agent_id):
  - Ping không cập nhật `last_seen_at`.
  - Sau 10 phút GetAgentStatus trả về rỗng → Dashboard không thấy agent.
- **Cách xử lý**: Build lại image Core và Agent (nerdctl/containerd), clean image cũ, deploy lại và đợi agent register + ping vài phút.

## 3. Kiểm tra nhanh

- **Bảng agents**:  
  `kubectl exec -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -c "SELECT agent_id, node_name, status, last_seen_at FROM agents;"`

- **API**:  
  `curl -s -H "Authorization: Bearer <JWT>" http://localhost:8080/api/v1/agents/status`

- **Log Core**:  
  `kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50`  
  (khi có Ping với agent_id, có thể thấy log cập nhật nếu thêm log.)

- **Log Agent**:  
  `kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=50`  
  (xác nhận RegisterAgent và Ping thành công.)

## 4. Deployment cần đồng bộ

- **Core**: Image build từ code mới (có Ping update `last_seen_at`), dùng `fortuna-core:latest` (hoặc tag) với `imagePullPolicy: Never` khi chạy local containerd.
- **Agent**: Image build từ code mới (Ping gửi AgentId/NodeName), env `NODE_NAME` từ `fieldRef.spec.nodeName` (đã có trong `fortuna-agent-daemonset.yaml`). Không bắt buộc set `AGENT_ID` (mặc định `NODE_NAME-agent`).
- **Dashboard**: Gọi `/api/v1/agents/status` và map đúng (đã sửa trong `lib/api.ts`).

Sau khi clean image cũ, rebuild (nerdctl) và deploy lại toàn bộ, Dashboard sẽ hiển thị agent sau khi agent register và ping trong vòng 10 phút.
