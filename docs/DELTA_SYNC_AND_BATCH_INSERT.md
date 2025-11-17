# Delta Sync và Batch Insert - Giảm Tải Database

## Vấn đề

Database bị đầy và crash do:
- Agent gửi toàn bộ data mỗi 30 giây (full sync)
- Mỗi sync tạo nhiều audit logs
- Không có batch insert, mỗi audit log = 1 DB write
- Không có TTL/retention cho audit logs

## Giải pháp đã triển khai

### 1. Delta Sync

**Cơ chế**:
- **Lần đầu**: Full sync (gửi tất cả resources)
- **Sau đó**: Delta sync (chỉ gửi thay đổi)
- **Full sync định kỳ**: Mỗi 10 lần sync (~5 phút với interval 30s)

**Implementation**:
- Agent Collector track last sync state
- Chỉ gửi resources đã thay đổi hoặc mới
- Watcher events luôn là delta sync

**File**: `agent/internal/collector/collector.go`

```go
// Full sync: first sync, or every 10th sync
shouldFullSync := c.fullSyncCount == 0 || (c.fullSyncCount%10 == 0)

// Delta sync: only include if changed or new
if !fullSync {
    key := fmt.Sprintf("sa:%s", saData.UID)
    currentHash := c.hashResource("sa", saData.UID, saData)
    lastHash, exists := c.lastSyncState[key]
    
    if !exists || currentHash != lastHash {
        // Changed or new - include in delta
        data.ServiceAccounts = append(data.ServiceAccounts, saData)
        c.lastSyncState[key] = currentHash
    }
}
```

### 2. Batch Insert cho Audit Logs

**Cơ chế**:
- Buffer audit logs trong memory
- Batch insert 100 records mỗi lần
- Tự động flush khi buffer đầy hoặc khi sync kết thúc

**Implementation**:
- Buffer size: 100 records
- Auto-flush khi đạt batch size
- Thread-safe với mutex

**File**: `core/internal/service/agent_service.go`

```go
const auditLogBatchSize = 100

// Buffer audit logs
s.addAuditLogToBuffer(auditLog)

// Batch insert
s.db.CreateInBatches(s.auditLogBuffer, auditLogBatchSize)
```

### 3. Event Retention + TTL

**Cơ chế**:
- TTL: 90 ngày (mặc định)
- Tự động cleanup audit logs cũ
- Chạy định kỳ mỗi giờ

**Implementation**:
- Cleanup job chạy mỗi giờ
- Xóa logs cũ hơn 90 ngày
- Log số lượng logs đã xóa

**File**: `core/internal/service/agent_service.go`

```go
const auditLogTTLDays = 90

// Cleanup old audit logs periodically (every hour)
if time.Since(s.lastFullSync) > time.Hour {
    s.cleanupOldAuditLogs()
}
```

### 4. Giảm Frequency của Audit Log Creation

**Cơ chế**:
- Minimum interval: 5 phút giữa các audit logs cho cùng resource
- Tránh spam audit logs cho cùng resource
- Chỉ tạo log khi thực sự cần

**Implementation**:
- Check existing log trong 5 phút gần nhất
- Chỉ tạo log nếu không có log gần đây

**File**: `core/internal/service/agent_service.go`

```go
const minAuditLogInterval = 5 * time.Minute

// Check if audit log should be created (avoid spam)
func (s *AgentService) shouldCreateAuditLog(resource, resourceID, action, user string) bool {
    var existingLog models.AuditLog
    err := s.db.Where("resource = ? AND resource_id = ? AND action = ? AND \"user\" = ? AND created_at > ?",
        resource, resourceID, action, user, time.Now().Add(-minAuditLogInterval)).First(&existingLog).Error
    
    // If no recent log found, should create
    return err != nil
}
```

## Cấu hình

### Constants

**File**: `core/internal/service/agent_service.go`

```go
const (
    auditLogBatchSize = 100        // Batch size for insert
    auditLogTTLDays = 90           // TTL in days
    minAuditLogInterval = 5 * time.Minute  // Min interval between logs
)
```

### Delta Sync Frequency

**File**: `agent/internal/collector/collector.go`

```go
// Full sync: first sync, or every 10th sync
shouldFullSync := c.fullSyncCount == 0 || (c.fullSyncCount%10 == 0)
```

Có thể điều chỉnh:
- `c.fullSyncCount%10 == 0`: Mỗi 10 lần sync (~5 phút)
- `c.fullSyncCount%20 == 0`: Mỗi 20 lần sync (~10 phút)

## Lợi ích

### 1. Giảm Database Load

- **Trước**: Mỗi sync gửi ~65 SAs, ~17 RBs, ~64 CRBs, ~16 Roles, ~75 CRoles
- **Sau**: Delta sync chỉ gửi thay đổi (thường < 10 resources)
- **Giảm**: ~90% data transfer

### 2. Giảm Audit Log Writes

- **Trước**: Mỗi audit log = 1 DB write
- **Sau**: 100 audit logs = 1 batch write
- **Giảm**: ~99% DB writes

### 3. Tự động Cleanup

- **Trước**: Audit logs tích lũy vô hạn
- **Sau**: Tự động xóa logs > 90 ngày
- **Giảm**: Database size tăng chậm hơn

### 4. Tránh Spam Logs

- **Trước**: Mỗi update tạo audit log
- **Sau**: Chỉ tạo log nếu > 5 phút từ lần trước
- **Giảm**: ~80% audit logs không cần thiết

## Monitoring

### Kiểm tra Delta Sync

```bash
# Check agent logs
kubectl logs -n kube-system -l app=ksam-agent --tail=50 | grep -i "delta\|full"

# Expected output:
# Starting DELTA sync collection (changes only)...
# Collection complete (DELTA): 2 SAs, 0 RBs, 0 CRBs, ...
```

### Kiểm tra Batch Insert

```bash
# Check core logs
kubectl logs -n ksam -l app=ksam-core --tail=50 | grep -i "batch.*audit"

# Expected output:
# Batch inserted 100 audit logs
```

### Kiểm tra Cleanup

```bash
# Check core logs
kubectl logs -n ksam -l app=ksam-core --tail=50 | grep -i "cleanup.*audit"

# Expected output:
# Cleaned up 150 old audit logs (older than 90 days)
```

## Tuning

### Điều chỉnh Batch Size

```go
const auditLogBatchSize = 200  // Tăng batch size
```

### Điều chỉnh TTL

```go
const auditLogTTLDays = 30  // Giảm TTL xuống 30 ngày
```

### Điều chỉnh Min Interval

```go
const minAuditLogInterval = 10 * time.Minute  // Tăng interval
```

### Điều chỉnh Full Sync Frequency

```go
// Full sync mỗi 20 lần thay vì 10 lần
shouldFullSync := c.fullSyncCount == 0 || (c.fullSyncCount%20 == 0)
```

## Kết quả mong đợi

- **Database writes**: Giảm ~99% (batch insert)
- **Data transfer**: Giảm ~90% (delta sync)
- **Audit log spam**: Giảm ~80% (min interval)
- **Database size**: Tăng chậm hơn (TTL cleanup)

