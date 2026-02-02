# Port-Forward Management

*Ngày: 2026-01-29*

## Vấn đề

Nhiều scripts tự động start port-forward mà không cleanup đúng cách, dẫn đến:
- Nhiều port-forward processes chạy cùng lúc
- Port conflicts
- Khó quản lý và debug

## Giải pháp

### Script quản lý tập trung

**File**: `scripts/manage-port-forwards.sh`

Script này quản lý tất cả port-forward processes một cách tập trung.

#### Usage

```bash
# Xem trạng thái tất cả port-forwards
./scripts/manage-port-forwards.sh status

# Dừng tất cả port-forwards
./scripts/manage-port-forwards.sh stop

# Start các port-forward thông dụng (Core:8080, Dashboard:8081)
./scripts/manage-port-forwards.sh start

# Clean tất cả và verify
./scripts/manage-port-forwards.sh clean
```

### Ports được quản lý

- **8080**: Core API (fortuna-core)
- **8081**: Dashboard (fortuna-dashboard)
- **5432**: PostgreSQL (postgres)
- **9090**: Core gRPC (fortuna-core)
- **18080, 28080, 28081, 29090**: Test ports (temporary)

## Best Practices

### 1. Luôn cleanup sau khi dùng

```bash
# Trước khi chạy script test
./scripts/manage-port-forwards.sh clean

# Sau khi test xong
./scripts/manage-port-forwards.sh stop
```

### 2. Sử dụng script quản lý thay vì start thủ công

**❌ Không nên:**
```bash
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
```

**✅ Nên:**
```bash
./scripts/manage-port-forwards.sh start
# hoặc
./scripts/port-forward-dashboard.sh 8081
```

### 3. Scripts tự động cleanup

Các scripts sau đã được cập nhật để cleanup port-forward:
- `scripts/verify-dashboard-issues.sh`: Cleanup sau khi test
- `scripts/clean-rebuild-dashboard.sh`: Stop port-forwards trước khi build
- `scripts/check-full-deployment.sh`: Sử dụng temporary ports và cleanup

## Scripts cần cập nhật

Các scripts sau tự động start port-forward nhưng chưa cleanup tốt:

1. **`scripts/verify-pod-data.sh`**
   - Start postgres và core port-forward
   - **Fix**: Thêm cleanup trap hoặc cleanup ở cuối script

2. **`scripts/check-pod-risk.sh`**
   - Start postgres port-forward
   - **Fix**: Thêm cleanup hoặc check trước khi start

3. **`scripts/test-pod-sync-flow.sh`**
   - Start core port-forward
   - **Fix**: Thêm cleanup trap

4. **`scripts/test-pod-risk-flow.sh`**
   - Start postgres và core port-forward
   - **Fix**: Thêm cleanup trap

## Verification

```bash
# Check status
./scripts/manage-port-forwards.sh status

# Should show:
# ✅ No port-forward processes running
# ✅ No common port-forward ports in use
```

## Troubleshooting

### Port đã được sử dụng

```bash
# 1. Stop all port-forwards
./scripts/manage-port-forwards.sh stop

# 2. Check ports
lsof -i -P -n | grep LISTEN | grep -E "8080|8081|5432"

# 3. Kill process manually nếu cần
kill <PID>
```

### Port-forward không hoạt động

```bash
# 1. Clean all
./scripts/manage-port-forwards.sh clean

# 2. Start lại
./scripts/manage-port-forwards.sh start

# 3. Check logs
tail -f /tmp/core-pf.log
tail -f /tmp/dashboard-pf.log
```

## Summary

✅ **Script quản lý tập trung**: `scripts/manage-port-forwards.sh`  
✅ **Cleanup đã thực hiện**: Tất cả port-forwards đã được dừng  
✅ **Ports sạch**: Không còn port-forward ports nào đang sử dụng  
✅ **Best practices**: Documented cho future scripts
