# Port-Forward Management

*Ngày: 2026-01-29*

## Vấn đề

Nhiều scripts tự động start port-forward mà không cleanup đúng cách, dẫn đến:
- Nhiều port-forward processes chạy cùng lúc
- Port conflicts
- Khó quản lý và debug

## Giải pháp

### Script quản lý tập trung

**File**: `scripts/utils/manage-port-forwards.sh`

Script này quản lý tất cả port-forward processes một cách tập trung.

#### Usage

```bash
# Xem trạng thái tất cả port-forwards
./scripts/utils/manage-port-forwards.sh status

# Dừng tất cả port-forwards
./scripts/utils/manage-port-forwards.sh stop

# Start các port-forward thông dụng (Core:8080, Dashboard:8081)
./scripts/utils/manage-port-forwards.sh start

# Clean tất cả và verify
./scripts/utils/manage-port-forwards.sh clean
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
./scripts/utils/manage-port-forwards.sh clean

# Sau khi test xong
./scripts/utils/manage-port-forwards.sh stop
```

### 2. Sử dụng script quản lý thay vì start thủ công

**❌ Không nên:**
```bash
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
```

**✅ Nên:**
```bash
./scripts/utils/manage-port-forwards.sh start
# hoặc
./scripts/utils/port-forward-dashboard.sh 8081
```

### 3. Scripts tự động cleanup

Các scripts sau đã được cập nhật để cleanup port-forward:
- `scripts/verify/verify-dashboard-issues.sh`: Cleanup sau khi test
- `scripts/clean/clean-rebuild-dashboard.sh`: Stop port-forwards trước khi build
- `scripts/verify/check-full-deployment.sh`: Sử dụng temporary ports và cleanup

## Trạng thái cập nhật

Các script E2E đã được chuẩn hóa để ưu tiên gọi API qua `kubectl exec` vào Core pod + JWT, giảm phụ thuộc port-forward:

1. **`scripts/e2e/test-pod-sync-flow.sh`**
   - Không còn tự start `port-forward` cho Core.
   - Dùng helper chung `scripts/e2e/common.sh`.

2. **`scripts/e2e/test-pod-risk-flow.sh`**
   - Không còn tự start `port-forward` cho Core/Postgres.
   - Dùng helper chung `scripts/e2e/common.sh`.

3. **`scripts/e2e/test-promotion-flow.sh`**, **`scripts/e2e/test-runtime-probe-e2e.sh`**
   - Đã bỏ endpoint/flow cũ phụ thuộc port-forward.
   - Chuẩn hóa auth JWT + truy vấn trực tiếp từ Core pod.

## Verification

```bash
# Check status
./scripts/utils/manage-port-forwards.sh status

# Should show:
# ✅ No port-forward processes running
# ✅ No common port-forward ports in use
```

## Troubleshooting

### Port đã được sử dụng

```bash
# 1. Stop all port-forwards
./scripts/utils/manage-port-forwards.sh stop

# 2. Check ports
lsof -i -P -n | grep LISTEN | grep -E "8080|8081|5432"

# 3. Kill process manually nếu cần
kill <PID>
```

### Port-forward không hoạt động

```bash
# 1. Clean all
./scripts/utils/manage-port-forwards.sh clean

# 2. Start lại
./scripts/utils/manage-port-forwards.sh start

# 3. Check logs
tail -f /tmp/core-pf.log
tail -f /tmp/dashboard-pf.log
```

## Summary

✅ **Script quản lý tập trung**: `scripts/utils/manage-port-forwards.sh`  
✅ **Cleanup đã thực hiện**: Tất cả port-forwards đã được dừng  
✅ **Ports sạch**: Không còn port-forward ports nào đang sử dụng  
✅ **Best practices**: Documented cho future scripts
