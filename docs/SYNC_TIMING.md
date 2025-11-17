# Sync Timing - Agent to Frontend

## Tổng quan

Tài liệu này mô tả thời gian sync dữ liệu từ Kubernetes → Agent → Core → Database → Frontend.

## Kiến trúc Sync

```
Kubernetes → Agent (Watcher/Collector) → Core → Database → Frontend (React Query)
```

## Các Cơ Chế Sync

### 1. Agent Watcher (Real-time Events)

**Cơ chế**: Sử dụng Kubernetes Informers để watch events real-time

**Timing**: 
- **CREATE/UPDATE**: Ngay lập tức khi có event (thường < 1 giây)
- **DELETE**: Ngay lập tức khi có event (thường < 1 giây)

**Cấu hình**: 
- Không có interval, hoạt động event-driven
- Gửi data ngay khi detect event

**Logs**:
```
ServiceAccount added: default/test-sa
ServiceAccount updated: default/test-sa
ServiceAccount deleted: default/test-sa
```

### 2. Agent Collector (Full Sync)

**Cơ chế**: Collector chạy định kỳ để sync toàn bộ data

**Timing**: 
- **Interval**: 30 giây (mặc định)
- **Cấu hình**: `KSAM_SYNC_INTERVAL=30s`

**Cấu hình**:
- File: `agent/internal/config/config.go`
- Default: `30s`
- Có thể thay đổi qua env var `KSAM_SYNC_INTERVAL`

**Logs**:
```
Starting data collection...
Collection complete: 65 SAs, 17 RBs, 64 CRBs, 16 Roles, 75 CRoles, 25 Pods
Successfully sent data to core: 65 SAs, 17 RBs, 64 CRBs
```

### 3. Frontend React Query

**Cơ chế**: React Query tự động refetch data từ API

**Timing**:
- **ServiceAccounts**: `refetchInterval: 5000` (5 giây)
- **AuditLogs**: Không có auto-refetch (chỉ refetch khi user action)
- **Graph**: `staleTime: 0` (luôn refetch)

**Cấu hình**:
- File: `dashboard/src/hooks/useServiceAccounts.ts`
- File: `dashboard/src/pages/ServiceAccounts.tsx`

## Kết Quả Test Thực Tế

### Test Case: Create ServiceAccount

```
CREATE sync time: 6 seconds
```

**Breakdown**:
1. **Kubernetes → Agent Watcher**: < 1 giây (real-time event)
2. **Agent → Core**: < 1 giây (HTTP request)
3. **Core → Database**: < 1 giây (DB write)
4. **Database → Frontend**: 5 giây (React Query refetch interval)

**Tổng**: ~6 giây

### Test Case: Delete ServiceAccount

```
DELETE sync time: 10 seconds
```

**Breakdown**:
1. **Kubernetes → Agent Watcher**: < 1 giây (real-time event)
2. **Agent → Core**: < 1 giây (HTTP request)
3. **Core → Database**: Có thể cần đợi full sync để detect deletion
4. **Database → Frontend**: 5 giây (React Query refetch interval)

**Tổng**: ~10 giây (có thể cần đợi full sync 30s nếu watcher không detect)

## Timeline Chi Tiết

### CREATE Event Flow

```
T=0s:   User tạo ServiceAccount trên K8s
T=0.1s: Agent Watcher detect event
T=0.2s: Agent gửi data đến Core
T=0.3s: Core lưu vào Database
T=0.4s: Database có data mới
T=5s:   Frontend refetch (React Query interval)
T=5.1s: Frontend hiển thị data mới
```

**Tổng**: ~5-6 giây

### DELETE Event Flow

```
T=0s:   User xóa ServiceAccount trên K8s
T=0.1s: Agent Watcher detect event
T=0.2s: Agent gửi data đến Core
T=0.3s: Core xử lý (có thể cần full sync để confirm deletion)
T=5s:   Frontend refetch (React Query interval)
T=5.1s: Frontend hiển thị deletion
```

**Tổng**: ~5-10 giây (tùy vào full sync timing)

### Full Sync Flow (Collector)

```
T=0s:   Collector bắt đầu chạy (mỗi 30s)
T=2s:   Collector thu thập data từ K8s
T=3s:   Collector gửi data đến Core
T=4s:   Core sync vào Database
T=5s:   Frontend refetch (React Query interval)
T=5.1s: Frontend hiển thị data mới
```

**Tổng**: ~5 giây sau khi collector chạy (tối đa 35s từ khi có thay đổi)

## Cấu Hình

### Agent Sync Interval

**File**: `helm/ksam/values.yaml`
```yaml
agent:
  syncInterval: "30s"
```

**File**: `agent/internal/config/config.go`
```go
SyncInterval: parseDuration(getEnv("KSAM_SYNC_INTERVAL", "30s"))
```

### Frontend Refetch Interval

**File**: `dashboard/src/hooks/useServiceAccounts.ts`
```typescript
refetchInterval: options?.refetchInterval ?? 5000, // 5 seconds
```

**File**: `dashboard/src/pages/ServiceAccounts.tsx`
```typescript
refetchInterval: 5000,
refetchOnWindowFocus: true,
```

## Tối Ưu Hóa

### Để Giảm Sync Time

1. **Giảm Frontend Refetch Interval**:
   ```typescript
   refetchInterval: 2000, // 2 seconds
   ```

2. **Tăng Agent Sync Frequency**:
   ```yaml
   agent:
     syncInterval: "10s"  # Thay vì 30s
   ```

3. **Sử dụng WebSocket** (future enhancement):
   - Real-time push từ Core → Frontend
   - Không cần polling

### Trade-offs

- **Refetch thường xuyên hơn**: 
  - ✅ Data mới hơn
  - ❌ Tăng load server và network

- **Refetch ít hơn**:
  - ✅ Giảm load
  - ❌ Data có thể cũ hơn

## Monitoring

### Kiểm tra Agent Sync

```bash
# Check agent logs
kubectl logs -n kube-system -l app=ksam-agent --tail=50 | grep -i "collection\|sync"

# Check sync interval
kubectl get daemonset -n kube-system ksam-agent -o yaml | grep KSAM_SYNC_INTERVAL
```

### Kiểm tra Frontend Refetch

- Mở browser DevTools → Network tab
- Xem requests đến `/api/v1/serviceaccounts`
- Kiểm tra interval giữa các requests

### Test Sync Timing

```bash
# Chạy test script
./scripts/test-sync-timing.sh
```

## Kết Luận

### Thời Gian Sync Tổng Thể

| Event Type | Min Time | Max Time | Average |
|------------|----------|----------|---------|
| CREATE     | 5s       | 35s      | ~6s     |
| UPDATE     | 5s       | 35s      | ~6s     |
| DELETE     | 5s       | 35s      | ~10s    |

### Các Yếu Tố Ảnh Hưởng

1. **Agent Watcher**: Real-time (< 1s)
2. **Agent Collector**: Mỗi 30s
3. **Frontend Refetch**: Mỗi 5s
4. **Network Latency**: < 1s
5. **Database Write**: < 1s

### Khuyến Nghị

- **Cho real-time updates**: Giảm frontend refetch interval xuống 2-3s
- **Cho performance**: Giữ nguyên 5s
- **Cho critical updates**: Có thể thêm manual refresh button

