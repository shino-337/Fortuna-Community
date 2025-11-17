# Báo cáo Debug Audit Log Sync

## Tóm tắt vấn đề

Dashboard không hiển thị audit logs vì **không có audit logs nào được tạo trong database** khi agent sync ServiceAccounts.

## Phân tích chi tiết

### 1. Luồng hoạt động

```
K8s Cluster → Watcher → Agent → Core Controller → Database → Dashboard
```

### 2. Kiểm tra từng bước

#### ✅ Bước 1: Watcher phát hiện SA mới
- **Log**: `ServiceAccount added: default/test-audit-final2-1763355003`
- **Trạng thái**: ✅ Hoạt động
- **Thời gian**: 2025/11/17 04:50:03

#### ✅ Bước 2: Watcher gửi event đến Core
- **Code**: `watcher.go:162` - `w.sendUpdate(ctx, data)`
- **Data**: `IsDeltaSync: true`, `IsFullSync: false`
- **Trạng thái**: ✅ Gửi thành công

#### ✅ Bước 3: Core nhận sync request
- **Log**: `[GIN] 2025/11/17 - 05:17:57 | 200 | 506.478667ms | POST "/api/v1/agent/sync"`
- **Trạng thái**: ✅ Nhận được request (200 OK)

#### ✅ Bước 4: SA được tạo trong DB
- **Query**: `SELECT * FROM service_accounts WHERE name LIKE '%test-audit-final2%'`
- **Kết quả**: SA đã tồn tại (ID: 72, created_at: 2025-11-17 04:50:03)
- **Trạng thái**: ✅ SA được sync vào DB

#### ❌ Bước 5: Audit log KHÔNG được tạo
- **Query**: `SELECT * FROM audit_logs WHERE resource_id = '72'`
- **Kết quả**: 0 rows
- **Trạng thái**: ❌ **VẤN ĐỀ Ở ĐÂY**

### 3. Nguyên nhân có thể

#### Nguyên nhân 1: `isDeltaSync` không được truyền đúng

**Kiểm tra**:
- Watcher set `IsDeltaSync: true` trong `CollectedData`
- Agent client gửi `isDeltaSync` trong payload
- Core nhận `isDeltaSync` từ `data["isDeltaSync"]`

**Vấn đề có thể**:
- Type assertion `data["isDeltaSync"].(bool)` có thể fail nếu:
  - Key không tồn tại
  - Value không phải bool (có thể là string "true"/"false")
  - JSON unmarshal không đúng

#### Nguyên nhân 2: Logic tạo audit log không được gọi

**Logic hiện tại**:
```go
if isDeltaSync && !hasCreateLog {
    action = "create"
}
if isDeltaSync && action == "create" {
    shouldLog = true
}
```

**Vấn đề có thể**:
- `isDeltaSync` = false (không được truyền đúng)
- `hasCreateLog` = true (đã có log từ trước, nhưng không có)
- SA đã tồn tại trong DB từ collector sync trước đó, nên `isNewSA` = false

#### Nguyên nhân 3: Buffer không được flush

**Logic hiện tại**:
- Audit logs được thêm vào buffer
- Buffer flush khi đạt 100 records hoặc khi `defer flushAuditLogBuffer()` được gọi

**Vấn đề có thể**:
- Buffer chưa đạt 100 records
- `defer` không được gọi (panic hoặc early return)

### 4. Giải pháp đã thực hiện

#### ✅ Thêm logging chi tiết

1. **Log khi nhận isDeltaSync**:
   ```go
   log.Printf("SyncData: isDeltaSync = %v (from data)", isDeltaSync)
   ```

2. **Log khi set action**:
   ```go
   log.Printf("SyncData: Setting action=create for SA %s/%s - isDeltaSync=true, hasCreateLog=false", ...)
   ```

3. **Log khi thêm audit log**:
   ```go
   log.Printf("Added audit log for %s SA: %s/%s (ID: %s, isDeltaSync: %v, isNewSA: %v)", ...)
   ```

#### ✅ Sửa logic tạo audit log

- Khi `isDeltaSync && !hasCreateLog`: luôn set `action = "create"`
- Khi `isDeltaSync && action == "create"`: luôn set `shouldLog = true`

### 5. Bước tiếp theo

1. ✅ **Đã thêm logging** - Cần kiểm tra logs để xác định nguyên nhân
2. ⏳ **Tạo SA mới và kiểm tra logs** - Xem `isDeltaSync` có được truyền đúng không
3. ⏳ **Kiểm tra buffer flush** - Xem buffer có được flush không
4. ⏳ **Kiểm tra type assertion** - Xem `isDeltaSync` có được parse đúng không

### 6. Lệnh kiểm tra

```bash
# 1. Tạo SA mới
kubectl create serviceaccount test-debug-$(date +%s) -n default

# 2. Đợi watcher phát hiện (10-15 giây)
sleep 15

# 3. Kiểm tra logs của agent
kubectl logs -n kube-system -l app=ksam-agent --tail=50 | grep -E "test-debug|ServiceAccount added"

# 4. Kiểm tra logs của core
kubectl logs -n ksam -l app=ksam-core --tail=200 | grep -E "SyncData|isDeltaSync|Setting action|Added audit|Skipped audit|test-debug"

# 5. Kiểm tra database
kubectl exec -n ksam $(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d ksam -c "SELECT COUNT(*) FROM audit_logs;"

# 6. Kiểm tra SA trong DB
kubectl exec -n ksam $(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d ksam -c "SELECT id, name, namespace FROM service_accounts WHERE name LIKE '%test-debug%';"
```

## Kết luận

Vấn đề chính: **Audit logs không được tạo khi agent sync ServiceAccounts từ watcher events**.

Nguyên nhân có thể:
1. `isDeltaSync` không được truyền đúng từ agent đến core
2. Logic tạo audit log không được gọi do điều kiện không thỏa
3. Buffer không được flush

**Đã thêm logging chi tiết để debug. Cần kiểm tra logs sau khi tạo SA mới để xác định nguyên nhân chính xác.**

