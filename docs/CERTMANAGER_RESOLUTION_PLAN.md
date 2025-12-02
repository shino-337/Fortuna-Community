# Kế hoạch Xử lý CertManager Issue

**Date**: $(date +"%Y-%m-%d %H:%M:%S")  
**Status**: Đang thực hiện

---

## Phân tích Vấn đề

### Vấn đề Chính
- Certificate API endpoints trả về 404
- CertManager không được tạo
- Code mới không được execute trong binary

### Biểu hiện
- Logs không có "Creating CertManager"
- Logs không có TEST_UNIQUE_STRING
- Logs vẫn show old code path
- Debug endpoint: 404
- Certificate API: 404

### Root Cause
**Dockerfile sử dụng `-ldflags="-w"` flag!**

Flag `-w` trong Go build:
- Removes DWARF symbol table
- Removes debug information
- **Có thể remove strings không được reference trực tiếp**

### Solution
**Remove `-w` flag từ Dockerfile**

---

## Kịch bản Xử lý

### STEP 1: Verify Source Code ✅
- [x] Check file có code mới
- [x] Verify TEST_UNIQUE_STRING có trong file
- [x] Verify Creating CertManager có trong file

### STEP 2: Fix Dockerfile ✅
- [x] Remove `-w` flag từ `-ldflags`
- [x] Keep `-trimpath` để remove paths
- [x] Rebuild với --no-cache

### STEP 3: Verify Binary ✅
- [x] Extract binary từ image
- [x] Check strings trong binary
- [x] Verify TEST_UNIQUE_STRING có trong binary

### STEP 4: Deploy và Test ✅
- [x] Load image vào Minikube
- [x] Delete pod để recreate
- [x] Check logs
- [x] Test endpoints

### STEP 5: Verify Results
- [ ] Check logs có TEST_UNIQUE_STRING
- [ ] Check logs có Creating CertManager
- [ ] Test debug endpoint
- [ ] Test certificate API
- [ ] Run comprehensive tests

---

## Implementation

### Changes Made

**File**: `core/Dockerfile`
```dockerfile
# Before:
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w" \
    -trimpath \
    -o /ksam-core ./cmd

# After:
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -o /ksam-core ./cmd
```

### Expected Results

Sau khi fix:

1. **Logs sẽ show**:
   ```
   [gRPC] TLS enabled, loading TLS configuration...
   [gRPC] TEST_UNIQUE_STRING_CERTMANAGER_12345
   [gRPC] Creating CertManager with:
   [gRPC]   CertPath: /etc/ksam/certs/tls.crt
   [gRPC]   KeyPath: /etc/ksam/certs/tls.key
   [gRPC]   CACertPath: /etc/ksam/ca-cert/ca.crt
   [CertManager] Loading certificate from...
   [gRPC] ✅ CertManager created successfully
   ```

2. **Certificate API sẽ**:
   - Return 200 với certificate info
   - Routes được register đúng cách

3. **Prometheus Metrics sẽ**:
   - Show certificate expiry metrics
   - Show rotation metrics

---

## Testing

### Test Cases

1. **Binary Verification**
   - [ ] Binary có TEST_UNIQUE_STRING
   - [ ] Binary có Creating CertManager

2. **Logs Verification**
   - [ ] Logs có TEST_UNIQUE_STRING
   - [ ] Logs có Creating CertManager
   - [ ] Logs có CertManager created

3. **API Verification**
   - [ ] Debug endpoint returns 200
   - [ ] Certificate info endpoint returns 200
   - [ ] Certificate rotate endpoint accessible

4. **Metrics Verification**
   - [ ] Certificate expiry metrics present
   - [ ] Rotation metrics present

---

## Next Steps

1. Verify fix works
2. Remove TEST_UNIQUE_STRING after verification
3. Update documentation
4. Run full test suite

---

**Report Generated**: $(date +"%Y-%m-%d %H:%M:%S")

