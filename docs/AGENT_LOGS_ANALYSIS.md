# Phân tích chi tiết: Logs của Agent

## Test Case

**ServiceAccount**: `test-agent-logs-1763362072`  
**Thời gian**: 2025-11-17 06:47:52  
**Namespace**: default

## Timeline

```
06:47:52 - Watcher phát hiện: "ServiceAccount added: default/test-agent-logs-1763362072"
06:47:52 - Watcher gửi event: "Successfully sent data to core: 1 SAs, 0 RBs, 0 CRBs"
06:47:53 - FULL sync bắt đầu: "Starting FULL sync collection..."
06:47:56 - FULL sync hoàn thành: "Collection complete (FULL): 72 SAs, ..."
06:47:57 - FULL sync gửi: "Successfully sent data to core: 72 SAs, 17 RBs, 64 CRBs"
06:47:57 - Core nhận request 1: POST "/api/v1/agent/sync" (200 OK, 882ms)
06:48:28 - Core nhận request 2: POST "/api/v1/agent/sync" (200 OK, 844ms)
```

## Phân tích từng bước

### ✅ Bước 1: Watcher phát hiện SA mới

**Log**:
```
2025/11/17 06:47:52 ServiceAccount added: default/test-agent-logs-1763362072
```

**Trạng thái**: ✅ **Hoạt động đúng**

**Code**: `agent/internal/watcher/watcher.go:149`
- Watcher sử dụng Kubernetes informer
- Phát hiện SA mới ngay lập tức

### ✅ Bước 2: Watcher gửi event riêng

**Log**:
```
2025/11/17 06:47:52 Successfully sent data to core: 1 SAs, 0 RBs, 0 CRBs
```

**Trạng thái**: ✅ **Watcher đã gửi event riêng**

**Phân tích**:
- Watcher gửi **1 SA** (chỉ SA mới)
- **0 RBs, 0 CRBs** (không có RoleBindings/ClusterRoleBindings)
- Đây là **DELTA sync từ watcher** với `IsDeltaSync: true`

**Code**: `agent/internal/watcher/watcher.go:155-162`
```go
data := &types.CollectedData{
    ClusterID:       w.config.ClusterID,
    ServiceAccounts: []types.ServiceAccountData{saData},
    IsFullSync:       false,
    IsDeltaSync:      true, // ✅ Watcher events are always delta
}
w.sendUpdate(ctx, data)
```

### ⚠️ Bước 3: FULL sync chạy ngay sau đó

**Log**:
```
2025/11/17 06:47:53 Starting FULL sync collection...
2025/11/17 06:47:56 Collection complete (FULL): 72 SAs, 17 RBs, 64 CRBs, 16 Roles, 75 CRoles, 25 Pods
2025/11/17 06:47:57 Successfully sent data to core: 72 SAs, 17 RBs, 64 CRBs
```

**Trạng thái**: ⚠️ **FULL sync chạy ngay sau watcher event**

**Phân tích**:
- FULL sync chạy **1 giây sau** watcher event
- FULL sync gửi **72 SAs** (bao gồm SA mới)
- Đây là **FULL sync** với `IsFullSync: true`, `IsDeltaSync: false`

**Vấn đề**:
- Core nhận **2 requests**:
  1. DELTA sync từ watcher (1 SA, isDeltaSync: true)
  2. FULL sync từ collector (72 SAs, isFullSync: true)

### ✅ Bước 4: Core nhận cả 2 requests

**Log**:
```
[GIN] 2025/11/17 - 06:47:57 | 200 | 882.07575ms | POST "/api/v1/agent/sync"
[GIN] 2025/11/17 - 06:48:28 | 200 | 844.820375ms | POST "/api/v1/agent/sync"
```

**Trạng thái**: ✅ **Core nhận được cả 2 requests**

**Phân tích**:
- Request 1 (06:47:57): DELTA sync từ watcher (882ms)
- Request 2 (06:48:28): FULL sync từ collector (844ms)

### ✅ Bước 5: SA được sync vào DB

**Query**:
```sql
SELECT id, name, namespace, uid, created_at 
FROM service_accounts 
WHERE name LIKE '%test-agent-logs%';
```

**Kết quả**:
```
 id |            name            | namespace |                 uid                  |          created_at           
----+----------------------------+-----------+--------------------------------------+-------------------------------
 76 | test-agent-logs-1763362072 | default   | 10331bc5-f3dd-42fb-bb05-f3dc392df02f | 2025-11-17 06:47:52.924543+00
```

**Trạng thái**: ✅ **SA đã được sync vào DB**

### ❌ Bước 6: Audit Log KHÔNG được tạo

**Query**:
```sql
SELECT COUNT(*) 
FROM audit_logs 
WHERE details::text LIKE '%test-agent-logs%' 
   OR resource_id IN (SELECT id::text FROM service_accounts WHERE name LIKE '%test-agent-logs%');
```

**Kết quả**: `0`

**Trạng thái**: ❌ **KHÔNG có audit log được tạo**

### ❌ Bước 7: Logs về SyncData không xuất hiện

**Kiểm tra**:
```bash
kubectl logs -n ksam -l app=ksam-core --since=5m | grep -i "syncdata\|isdeltasync\|processing sync"
```

**Kết quả**: Không có logs

**Trạng thái**: ❌ **Logs không xuất hiện**

## Vấn đề chính

### 1. Logs không xuất hiện

**Vấn đề**: 
- Code đã có logging: `log.Printf("SyncData: isDeltaSync = %v ...")`
- Nhưng không thấy logs trong output

**Nguyên nhân có thể**:
1. Log level quá cao
2. Logs bị filter
3. Code không chạy đến phần đó (có lỗi trước đó)

### 2. Audit log không được tạo

**Vấn đề**:
- Watcher gửi DELTA sync với `isDeltaSync: true`
- Core nhận được request (200 OK)
- SA được sync vào DB
- **Nhưng audit log KHÔNG được tạo**

**Nguyên nhân có thể**:
1. `isDeltaSync` không được truyền đúng (type assertion fail)
2. Logic tạo audit log không được gọi
3. Buffer không được flush

## Giải pháp đề xuất

### 1. Kiểm tra logs thực tế (không filter)

```bash
# Lưu tất cả logs vào file
kubectl logs -n ksam -l app=ksam-core --since=10m > core-logs-full.txt

# Tìm logs về SyncData
grep -i "syncdata\|isdeltasync\|processing sync\|setting action\|added audit" core-logs-full.txt
```

### 2. Thêm file logging

Thay vì dùng `log.Printf`, thêm file logging để đảm bảo logs được ghi:

```go
import (
    "os"
    "log"
)

// Tạo file log
logFile, err := os.OpenFile("/tmp/ksam-core.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
if err == nil {
    log.SetOutput(logFile)
}
```

### 3. Test trực tiếp với curl

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

## Kết luận

**Tóm tắt**:
- ✅ Watcher phát hiện SA mới
- ✅ Watcher gửi DELTA sync event (1 SA, isDeltaSync: true)
- ✅ Core nhận được request (200 OK)
- ✅ SA được sync vào DB
- ❌ **Audit log KHÔNG được tạo**
- ❌ **Logs về SyncData không xuất hiện**

**Nguyên nhân chính**:
1. Logs không được in ra (cần kiểm tra log level)
2. `isDeltaSync` có thể không được truyền đúng (cần test trực tiếp)
3. Logic tạo audit log không được gọi (cần thêm breakpoint)

**Bước tiếp theo**:
1. Kiểm tra logs thực tế (không filter)
2. Thêm file logging để đảm bảo logs được ghi
3. Test trực tiếp với curl để xác minh API hoạt động

