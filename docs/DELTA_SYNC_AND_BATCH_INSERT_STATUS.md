# Delta Sync và Batch Insert - Trạng thái kiểm tra

## Tổng quan

Đã thực hiện kiểm tra các tính năng:
1. **Delta Sync**: Agent chỉ gửi thay đổi thay vì full sync mỗi lần
2. **Batch Insert**: Audit logs được insert theo batch (100 records/batch)
3. **Event Retention (TTL)**: Tự động xóa audit logs cũ (> 90 ngày)
4. **Audit Log Frequency Control**: Tránh spam audit logs (min 5 phút giữa các logs cùng resource/action)

## Kết quả kiểm tra

### ✅ Delta Sync - Hoạt động

**Logs từ Agent:**
```
2025/11/17 03:53:23 Starting FULL sync collection...
2025/11/17 03:53:26 Collection complete (FULL): 66 SAs, 17 RBs, 64 CRBs, 16 Roles, 75 CRoles, 25 Pods
2025/11/17 03:53:53 Starting DELTA sync collection (changes only)...
2025/11/17 03:53:56 Collection complete (DELTA): 0 SAs, 17 RBs, 64 CRBs, 16 Roles, 75 CRoles, 25 Pods
```

**Nhận xét:**
- ✅ Agent thực hiện FULL sync lần đầu và mỗi 10 lần sync
- ✅ Agent thực hiện DELTA sync (chỉ gửi thay đổi) trong các lần sync khác
- ✅ DELTA sync chỉ gửi 0 SAs khi không có thay đổi (đúng như mong đợi)

### ⚠️ Batch Insert - Cần kiểm tra thêm

**Logs từ Core:**
- Không thấy logs về "Batch inserted X audit logs"
- Có thể do:
  1. Chưa có audit logs nào được tạo
  2. Buffer chưa đạt 100 records để flush
  3. Logic tạo audit log chưa được gọi

### ⚠️ Audit Log Creation - Cần kiểm tra thêm

**Vấn đề:**
- ServiceAccount đã được tạo trong DB (ID: 69, name: test-audit-1763352554)
- Nhưng không có audit log tương ứng trong bảng `audit_logs`

**Nguyên nhân có thể:**
1. Logic tạo audit log không được gọi (SA đã tồn tại từ lần sync trước, nên update thay vì create)
2. `shouldCreateAuditLog` trả về false (đã có audit log gần đây)
3. Có lỗi trong quá trình tạo audit log (nhưng không thấy error logs)

**Đã thêm logging để debug:**
- Log khi thêm audit log: `"Added audit log for new SA: ..."`
- Log khi skip audit log: `"Skipped audit log for new SA: ... - recent log exists"`

### ✅ Event Retention (TTL) - Chưa kiểm tra

- Cleanup chạy mỗi giờ
- Cần đợi để kiểm tra

## Các thay đổi đã thực hiện

### 1. Agent (`agent/internal/collector/collector.go`)
- ✅ Thêm `lastSyncState` để track hash của resources
- ✅ Thêm logic delta sync: chỉ gửi SAs thay đổi hoặc mới
- ✅ Thêm `IsFullSync` và `IsDeltaSync` flags trong `CollectedData`

### 2. Agent (`agent/internal/watcher/watcher.go`)
- ✅ Đánh dấu events từ watcher là `IsDeltaSync: true`

### 3. Agent (`agent/internal/client/grpc_client.go`)
- ✅ Thêm `isFullSync` và `isDeltaSync` vào payload gửi đến core

### 4. Core (`core/internal/service/agent_service.go`)
- ✅ Thêm `auditLogBuffer` và `flushAuditLogBuffer` cho batch insert
- ✅ Thêm `cleanupOldAuditLogs` cho TTL (90 ngày)
- ✅ Thêm `shouldCreateAuditLog` để tránh spam (min 5 phút)
- ✅ Thêm logic tạo audit logs cho create/update/delete/restore
- ✅ Thêm logging để debug

## Vấn đề cần giải quyết

### 1. Audit Logs không được tạo

**Triệu chứng:**
- ServiceAccount được tạo trong DB
- Nhưng không có audit log tương ứng

**Nguyên nhân có thể:**
- Khi watcher gửi add event, SA có thể đã tồn tại trong DB từ lần sync trước (từ collector)
- Logic sẽ update thay vì create, và chỉ tạo audit log nếu `shouldCreateAuditLog` trả về true
- Cần kiểm tra logs để xem logic có được gọi không

**Giải pháp:**
- Thêm logging chi tiết hơn
- Kiểm tra xem SA có được tạo mới hay update
- Kiểm tra `shouldCreateAuditLog` có trả về đúng không

### 2. Delete Event từ Watcher

**Vấn đề:**
- Khi watcher phát hiện delete event, nó vẫn gửi SA data
- Core không biết đây là delete event
- Logic delete chỉ chạy khi `isFullSync` là true

**Giải pháp đề xuất:**
- Thêm flag `isDelete` trong `CollectedData` để đánh dấu delete event
- Hoặc xử lý delete event từ watcher bằng cách kiểm tra xem SA có tồn tại trong K8s không

## Bước tiếp theo

1. ✅ Kiểm tra logs của core để xem có "Added audit log" hoặc "Skipped audit log" không
2. ✅ Kiểm tra xem `shouldCreateAuditLog` có hoạt động đúng không
3. ✅ Kiểm tra xem batch insert có được gọi khi buffer đạt 100 records không
4. ⏳ Đợi full sync tiếp theo để kiểm tra delete event
5. ⏳ Đợi 1 giờ để kiểm tra TTL cleanup

## Lệnh kiểm tra

```bash
# Kiểm tra delta sync
kubectl logs -n kube-system -l app=ksam-agent --tail=50 | grep -E "DELTA|FULL"

# Kiểm tra batch insert
kubectl logs -n ksam -l app=ksam-core --tail=100 | grep -E "Batch inserted|Added audit log|Skipped audit log"

# Kiểm tra audit logs trong DB
kubectl exec -n ksam $(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- psql -U postgres -d ksam -c "SELECT COUNT(*) FROM audit_logs;"

# Kiểm tra TTL cleanup
kubectl logs -n ksam -l app=ksam-core --tail=200 | grep -E "Cleaned up.*old audit logs"
```

