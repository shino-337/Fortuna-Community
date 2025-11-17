# Kết quả kiểm tra logs của Agent

## Test Case

**ServiceAccount**: `test-agent-logs-1763362072`
**Thời gian tạo**: 2025-11-17 06:47:52
**Namespace**: default

## Kết quả kiểm tra

### ✅ 1. Watcher phát hiện SA mới

**Log từ Agent**:
```
2025/11/17 06:47:52 ServiceAccount added: default/test-agent-logs-1763362072
```

**Trạng thái**: ✅ **Hoạt động đúng**

### ✅ 2. Agent gửi event đến Core

**Log từ Agent**:
```
2025/11/17 06:47:53 Starting FULL sync collection...
2025/11/17 06:47:56 Collection complete (FULL): 72 SAs, 17 RBs, 64 CRBs, 16 Roles, 75 CRoles, 25 Pods
```

**Trạng thái**: ✅ **Agent đã sync (FULL sync)**

**Lưu ý**: 
- Watcher phát hiện SA mới lúc 06:47:52
- FULL sync chạy lúc 06:47:53 (1 giây sau)
- FULL sync gửi 72 SAs (bao gồm SA mới)

### ✅ 3. SA được tạo trong Database

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

### ❌ 4. Audit Log KHÔNG được tạo

**Query**:
```sql
SELECT id, action, resource, resource_id, details::text, created_at 
FROM audit_logs 
WHERE details::text LIKE '%test-agent-logs%' 
   OR resource_id IN (SELECT id::text FROM service_accounts WHERE name LIKE '%test-agent-logs%');
```

**Kết quả**: `0 rows`

**Trạng thái**: ❌ **KHÔNG có audit log được tạo**

### ❌ 5. Logs về SyncData không xuất hiện

**Kiểm tra logs của Core**:
```bash
kubectl logs -n ksam -l app=ksam-core --since=5m | grep -i "syncdata\|isdeltasync\|processing sync"
```

**Kết quả**: Không có logs

**Trạng thái**: ❌ **Logs không xuất hiện**

### ✅ 6. Core nhận được sync request

**Log từ Core**:
```
[GIN] 2025/11/17 - 06:47:56 | 200 | ... | POST "/api/v1/agent/sync"
```

**Trạng thái**: ✅ **Core nhận được request (200 OK)**

## Phân tích

### Vấn đề chính

1. **Watcher phát hiện SA mới** ✅
2. **Agent gửi FULL sync** (không phải DELTA sync từ watcher) ⚠️
3. **SA được sync vào DB** ✅
4. **Audit log KHÔNG được tạo** ❌
5. **Logs về SyncData không xuất hiện** ❌

### Nguyên nhân có thể

#### 1. Watcher event không được gửi riêng

**Vấn đề**: 
- Watcher phát hiện SA mới lúc 06:47:52
- Nhưng FULL sync chạy ngay sau đó (06:47:53)
- Có thể watcher event bị "nuốt" bởi FULL sync

**Giải pháp**: 
- Kiểm tra xem watcher có gửi event riêng không
- Hoặc FULL sync đã bao gồm SA mới

#### 2. FULL sync không tạo audit log

**Vấn đề**:
- FULL sync gửi 72 SAs (bao gồm SA mới)
- Nhưng logic tạo audit log chỉ chạy cho DELTA sync từ watcher
- FULL sync có thể không tạo audit log cho SA mới

**Giải pháp**:
- Kiểm tra logic tạo audit log cho FULL sync
- Đảm bảo FULL sync cũng tạo audit log cho SA mới

#### 3. Logs không được in ra

**Vấn đề**:
- Code đã có logging nhưng không thấy logs
- Có thể log level quá cao hoặc logs bị filter

**Giải pháp**:
- Kiểm tra log level
- Thêm file logging để đảm bảo logs được ghi

## Kết luận

**Vấn đề**: 
- Watcher phát hiện SA mới ✅
- Agent sync SA vào DB ✅
- **Audit log KHÔNG được tạo** ❌

**Nguyên nhân có thể**:
1. Watcher event không được gửi riêng (bị FULL sync "nuốt")
2. FULL sync không tạo audit log cho SA mới
3. Logs không được in ra (cần kiểm tra log level)

**Bước tiếp theo**:
1. Kiểm tra xem watcher có gửi event riêng không
2. Sửa logic để FULL sync cũng tạo audit log cho SA mới
3. Kiểm tra log level và thêm file logging

