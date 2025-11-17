# Tóm tắt: Audit Logs không sync tới Dashboard

## Vấn đề

Dashboard không hiển thị audit logs vì **không có audit logs nào được tạo trong database** khi agent sync ServiceAccounts từ watcher events.

## Phân tích

### ✅ Các bước hoạt động đúng

1. **Watcher phát hiện SA mới**: ✅
   - Log: `ServiceAccount added: default/test-logs-1763361883`
   - Set: `IsDeltaSync: true`, `IsFullSync: false`

2. **Agent gửi đến Core**: ✅
   - Payload chứa `isDeltaSync: true`
   - HTTP POST `/api/v1/agent/sync` → 200 OK

3. **Core nhận request**: ✅
   - Handler `SyncDataFromAgent` được gọi
   - Unmarshal JSON thành công

4. **SA được sync vào DB**: ✅
   - SA được tạo/cập nhật trong database
   - Query: `SELECT * FROM service_accounts WHERE uid = '...'`

### ❌ Vấn đề

**Audit logs KHÔNG được tạo** mặc dù:
- Code đã có logic tạo audit log
- Đã thêm logging chi tiết
- Đã sửa type assertion để xử lý nhiều kiểu dữ liệu

**Logs không xuất hiện**:
- Không thấy logs về "SyncData: isDeltaSync = ..."
- Không thấy logs về "Setting action=create"
- Không thấy logs về "Added audit log"

## Nguyên nhân có thể

### 1. Logs không được in ra

**Khả năng**: Code chạy nhưng logs không được in do:
- Log level quá cao
- Logs bị filter
- Standard output không được capture

**Giải pháp**: Kiểm tra log level và cách logs được capture

### 2. Type assertion fail im lặng

**Khả năng**: `data["isDeltaSync"]` không tồn tại hoặc type không đúng, nhưng code không log được

**Giải pháp**: Đã sửa để xử lý nhiều kiểu dữ liệu, nhưng vẫn cần kiểm tra

### 3. Logic không được gọi

**Khả năng**: Code không chạy đến phần tạo audit log do:
- Early return
- Error không được log
- Panic được recover

**Giải pháp**: Thêm logging ở mọi điểm quan trọng

## Giải pháp đã thực hiện

### ✅ 1. Sửa type assertion

```go
// Xử lý nhiều kiểu dữ liệu: bool, float64, string
switch v := deltaSyncVal.(type) {
case bool:
    isDeltaSync = v
case float64:
    isDeltaSync = v != 0
case string:
    isDeltaSync = (v == "true" || v == "1")
}
```

### ✅ 2. Thêm logging chi tiết

- Log khi nhận `isDeltaSync`
- Log khi set action
- Log khi thêm audit log
- Log data keys để debug

### ✅ 3. Sửa logic tạo audit log

- Khi `isDeltaSync && !hasCreateLog`: luôn set `action = "create"`
- Khi `isDeltaSync && action == "create"`: luôn set `shouldLog = true`

## Bước tiếp theo

### 1. Kiểm tra logs thực tế

```bash
# Kiểm tra tất cả logs (không filter)
kubectl logs -n ksam -l app=ksam-core --since=5m > core-logs.txt
grep -i "syncdata\|isdeltasync\|audit" core-logs.txt
```

### 2. Test trực tiếp với curl

```bash
# Test API với payload có isDeltaSync
curl -X POST http://localhost:8080/api/v1/agent/sync \
  -H "Content-Type: application/json" \
  -d '{
    "clusterId": "minikube",
    "data": {
      "serviceAccounts": [{
        "name": "test-manual",
        "namespace": "default",
        "uid": "test-uid-123",
        "labels": {},
        "secrets": []
      }],
      "isDeltaSync": true,
      "isFullSync": false
    }
  }'
```

### 3. Kiểm tra database trực tiếp

```bash
# Kiểm tra SA mới nhất
kubectl exec -n ksam $(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d ksam -c "SELECT id, name, namespace, created_at FROM service_accounts ORDER BY created_at DESC LIMIT 5;"

# Kiểm tra audit logs
kubectl exec -n ksam $(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d ksam -c "SELECT id, action, resource, resource_id, created_at FROM audit_logs ORDER BY created_at DESC LIMIT 10;"
```

### 4. Thêm breakpoint/debug

- Thêm panic để xem code có chạy đến đâu không
- Thêm file logging để đảm bảo logs được ghi
- Kiểm tra error handling

## Kết luận

**Trạng thái**: Đã thêm logging và sửa logic, nhưng vẫn chưa thấy audit logs được tạo.

**Nguyên nhân có thể**:
1. Logs không được in ra (cần kiểm tra log level)
2. Type assertion vẫn fail (cần test trực tiếp)
3. Logic không được gọi (cần thêm breakpoint)

**Ưu tiên**: 
1. Kiểm tra logs thực tế (không filter)
2. Test trực tiếp với curl
3. Thêm file logging để đảm bảo logs được ghi

## Tài liệu liên quan

- `docs/AUDIT_LOG_DEBUG_REPORT.md` - Báo cáo debug chi tiết
- `docs/AUDIT_LOG_ROOT_CAUSE.md` - Phân tích nguyên nhân gốc rễ
- `docs/AUDIT_LOG_SYNC_FIX.md` - Giải pháp đã thực hiện

