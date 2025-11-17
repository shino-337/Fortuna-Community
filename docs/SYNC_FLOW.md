# ServiceAccount Sync Flow Documentation

## Tổng quan

KSAM sử dụng một cơ chế sync hai tầng để đồng bộ dữ liệu từ Kubernetes cluster lên database và hiển thị trên Dashboard:

1. **Periodic Sync (Collector)**: Thu thập toàn bộ dữ liệu định kỳ (mặc định 30s)
2. **Real-time Sync (Watcher)**: Theo dõi và gửi updates ngay khi có thay đổi

## Flow Diagram

```
Kubernetes Cluster
       │
       ├─ ServiceAccount Created/Updated/Deleted
       │
       ▼
Agent (Watcher)
       │
       ├─ Detect change via Kubernetes Informers
       │
       ▼
Agent (gRPC/HTTP Client)
       │
       ├─ Send update to Core Controller
       │
       ▼
Core Controller (Agent Service)
       │
       ├─ Process and sync to Database
       │
       ▼
PostgreSQL Database
       │
       ├─ Data stored/updated
       │
       ▼
Dashboard (React Query)
       │
       ├─ Auto-refresh every 5 seconds
       │
       ▼
User sees updated data
```

## Components

### 1. Agent Watcher (`agent/internal/watcher/watcher.go`)

- Sử dụng Kubernetes Informers để watch real-time changes
- Khi có event (Add/Update/Delete), gọi `sendUpdate()` để gửi data lên Core
- Gửi qua gRPC client (hiện tại dùng HTTP endpoint tạm thời)

### 2. Agent Client (`agent/internal/client/grpc_client.go`)

- Gửi data lên Core Controller qua HTTP endpoint `/api/v1/agent/sync`
- Có retry logic với exponential backoff
- Convert CollectedData thành JSON format

### 3. Core Agent Handler (`core/internal/api/agent_handlers.go`)

- Nhận data từ Agent qua HTTP POST `/api/v1/agent/sync`
- Gọi `AgentService.SyncData()` để xử lý

### 4. Agent Service (`core/internal/service/agent_service.go`)

- Xử lý và sync data vào database
- Upsert logic: Update nếu đã tồn tại, Create nếu chưa có
- Sync các resources: ServiceAccounts, RoleBindings, ClusterRoleBindings, Roles, ClusterRoles, Pods

### 5. Dashboard Auto-refresh (`dashboard/src/hooks/useServiceAccounts.ts`)

- Sử dụng React Query với `refetchInterval: 5000` (5 giây)
- Tự động refetch khi window regains focus
- Invalidate queries sau khi mutation (delete, update)

## Test Cases

### Test Case 1: Create ServiceAccount

```bash
# 1. Create ServiceAccount in K8s
kubectl create serviceaccount test-sa -n default

# 2. Wait for sync (max 60s)
# Agent Watcher detects change → sends to Core → Core syncs to DB

# 3. Verify in Dashboard API
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/serviceaccounts?namespace=default

# Expected: test-sa appears in the list
```

### Test Case 2: Delete ServiceAccount

```bash
# 1. Delete ServiceAccount from K8s
kubectl delete serviceaccount test-sa -n default

# 2. Wait for sync (max 60s)
# Agent Watcher detects deletion → sends to Core → Core removes from DB

# 3. Verify in Dashboard API
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/serviceaccounts?namespace=default

# Expected: test-sa no longer appears in the list
```

### Test Case 3: Update ServiceAccount

```bash
# 1. Update ServiceAccount labels
kubectl label serviceaccount test-sa -n default env=production --overwrite

# 2. Wait for sync
# Agent Watcher detects update → sends to Core → Core updates in DB

# 3. Verify in Dashboard API
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/serviceaccounts/<id>

# Expected: Labels updated in the response
```

## Automated Testing

Sử dụng script `scripts/test-sync-flow.sh` để test tự động:

```bash
# Run test
./scripts/test-sync-flow.sh

# With custom parameters
NAMESPACE=my-namespace \
CORE_API=http://localhost:8080/api/v1 \
./scripts/test-sync-flow.sh
```

Script sẽ:
1. Tạo ServiceAccount trong K8s
2. Đợi và verify nó xuất hiện trong database (max 60s)
3. Xóa ServiceAccount
4. Đợi và verify nó bị xóa khỏi database (max 60s)

## Troubleshooting

### ServiceAccount không xuất hiện sau khi tạo

1. **Kiểm tra Agent logs**:
   ```bash
   kubectl logs -n ksam -l app=ksam-agent --tail=50
   ```
   Tìm log: "ServiceAccount added: ..."

2. **Kiểm tra Core logs**:
   ```bash
   kubectl logs -n ksam -l app=ksam-core --tail=50
   ```
   Tìm log về sync requests

3. **Kiểm tra database**:
   ```bash
   kubectl exec -n ksam -it postgres-xxx -- psql -U postgres -d ksam \
     -c "SELECT name, namespace FROM service_accounts WHERE name = 'test-sa';"
   ```

4. **Kiểm tra Agent connectivity**:
   ```bash
   kubectl exec -n ksam -it <agent-pod> -- \
     curl -X POST http://ksam-core:8080/api/v1/agent/sync \
     -H "Content-Type: application/json" \
     -d '{"clusterId":"minikube","data":{}}'
   ```

### Dashboard không auto-refresh

1. Kiểm tra browser console có errors không
2. Kiểm tra Network tab xem có requests đến `/api/v1/serviceaccounts` mỗi 5 giây không
3. Verify React Query config trong `useServiceAccounts.ts`

## Performance Considerations

- **Watcher**: Real-time updates, low latency (< 1s từ K8s event đến DB)
- **Collector**: Periodic full sync để đảm bảo consistency (mặc định 30s)
- **Dashboard**: Auto-refresh 5s để balance giữa real-time và performance

## Future Improvements

1. **WebSocket**: Thay thế polling bằng WebSocket cho real-time updates
2. **gRPC**: Implement đầy đủ gRPC với proto files thay vì HTTP
3. **Event Streaming**: Sử dụng event streaming (Kafka, NATS) cho scale
4. **Delta Sync**: Chỉ sync changes thay vì full data

