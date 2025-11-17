# Sửa lỗi Audit Log Sync

## Vấn đề

Dashboard không hiển thị audit logs vì:
1. **Không có audit logs trong database**: Chỉ có 1 test log được tạo thủ công
2. **Logic tạo audit log không hoạt động**: Khi agent sync, audit logs không được tạo

## Nguyên nhân

1. **Khi watcher gửi add event**: SA có thể đã tồn tại trong DB từ lần sync trước (từ collector), nên logic sẽ **update** thay vì **create**
2. **Logic update**: Chỉ tạo audit log nếu `shouldCreateAuditLog` trả về true, nhưng có thể đã có audit log gần đây nên nó skip
3. **Delta sync flag**: `isDeltaSync` không được truyền đúng từ agent đến core

## Giải pháp đã thực hiện

### 1. Sửa logic tạo audit log cho delta sync

**File**: `core/internal/service/agent_service.go`

**Thay đổi**:
- Khi **delta sync** (từ watcher) và SA mới (created within 2 minutes), **luôn tạo audit log "create"**
- Khi **full sync**, vẫn dùng `shouldCreateAuditLog` để tránh spam

```go
// For delta sync (from watcher), if SA is new, always create audit log
// For full sync, use shouldCreateAuditLog to avoid spam
shouldLog := false
if isDeltaSync && isNewSA && action == "create" {
    // Delta sync from watcher: always log new SAs
    shouldLog = true
} else {
    // Full sync or update: use shouldCreateAuditLog to avoid spam
    if action == "create" {
        shouldLog = s.shouldCreateAuditLog("serviceaccount", resourceID, "create", "system")
    } else if action == "update" {
        shouldLog = s.shouldCreateAuditLog("serviceaccount", resourceID, "update", "system")
    }
}
```

### 2. Thêm logging chi tiết

- Log khi thêm audit log: `"Added audit log for {action} SA: {namespace}/{name} (ID: {id}, isDeltaSync: {bool}, isNewSA: {bool})"`
- Log khi skip audit log: `"Skipped audit log for {action} SA: {namespace}/{name} (ID: {id}) - recent log exists or not new"`

### 3. Đảm bảo isDeltaSync được truyền đúng

**File**: `agent/internal/client/grpc_client.go`

**Thay đổi**: Thêm `isDeltaSync` và `isFullSync` vào payload gửi đến core

```go
dataMap := map[string]interface{}{
    // ... other fields ...
    "isFullSync":  data.IsFullSync,
    "isDeltaSync": data.IsDeltaSync,
}
```

## Kiểm tra

### 1. Kiểm tra delta sync flag

```bash
# Kiểm tra agent logs
kubectl logs -n kube-system -l app=ksam-agent --tail=50 | grep -E "DELTA|FULL"

# Kiểm tra core logs
kubectl logs -n ksam -l app=ksam-core --tail=100 | grep -E "isDeltaSync|Added audit|Skipped audit"
```

### 2. Tạo ServiceAccount test

```bash
# Tạo SA mới
kubectl create serviceaccount test-audit-$(date +%s) -n default

# Đợi 45 giây để agent sync
sleep 45

# Kiểm tra audit logs
kubectl exec -n ksam $(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d ksam -c "SELECT id, action, resource, resource_id, created_at FROM audit_logs ORDER BY created_at DESC LIMIT 10;"
```

### 3. Kiểm tra dashboard

1. Mở dashboard: http://localhost:3000
2. Đăng nhập (nếu cần)
3. Vào trang "Audit Logs"
4. Kiểm tra xem có audit logs hiển thị không

## Vấn đề còn lại

1. **Audit logs vẫn chưa được tạo**: Cần kiểm tra xem:
   - Watcher có gửi event không?
   - Core có nhận được event không?
   - `isDeltaSync` có được set đúng không?
   - Logic có được gọi không?

2. **Delete event**: Khi watcher phát hiện delete, nó vẫn gửi SA data. Core không biết đây là delete event. Cần thêm logic để xử lý delete event từ watcher.

## Bước tiếp theo

1. ✅ Kiểm tra logs của agent và core để xem có event nào được gửi không
2. ✅ Kiểm tra xem `isDeltaSync` có được set đúng không
3. ⏳ Thêm logic xử lý delete event từ watcher
4. ⏳ Test lại với ServiceAccount mới

