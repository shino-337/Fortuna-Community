# Root Cause và Fix cho CertManager Issue

**Date**: $(date +"%Y-%m-%d %H:%M:%S")  
**Issue**: Certificate API endpoints trả về 404

---

## Root Cause Analysis

### Phát hiện

**Logs show**:
```
[gRPC] TLS enabled, loading TLS configuration...
[gRPC] Loading CA certificate from: /etc/ksam/ca-cert/ca.crt
[gRPC] Loading server certificate from: /etc/ksam/certs/tls.crt, key from: /etc/ksam/certs/tls.key
```

**Code SHOULD show**:
```
[gRPC] TLS enabled, loading TLS configuration...
[gRPC] Creating CertManager with:
[gRPC]   CertPath: /etc/ksam/certs/tls.crt
[gRPC]   KeyPath: /etc/ksam/certs/tls.key
[gRPC]   CACertPath: /etc/ksam/ca-cert/ca.crt
[CertManager] Loading certificate from...
```

### Root Cause

**Code đang SKIP từ line 39 → line 64!**

- Line 39: `log.Printf("[gRPC] TLS enabled, loading TLS configuration...")` ✅ Executed
- Lines 43-46: CertManager creation logs ❌ **SKIPPED**
- Line 47: `security.NewCertManager(...)` ❌ **NOT CALLED**
- Line 64: `os.ReadFile(cfg.TLSCACertPath)` ✅ Executed

**Vấn đề**: Code mới không được execute trong binary!

### Possible Causes

1. **Build cache issue**: Old code được cache
2. **Compilation issue**: Code mới không được compile vào binary
3. **File sync issue**: Code changes chưa được save đúng cách
4. **Binary mismatch**: Binary đang dùng code cũ

---

## Solution

### Step 1: Force Rebuild với --no-cache

```bash
cd core
docker build --no-cache -t ksam-core:latest .
```

### Step 2: Verify Binary

```bash
docker run --rm --entrypoint sh ksam-core:latest -c "strings /ksam-core | grep -i 'Creating CertManager'"
```

### Step 3: Force Reload vào Minikube

```bash
minikube image rm ksam-core:latest
minikube image load ksam-core:latest
kubectl delete pod -n ksam -l app=ksam-core
```

### Step 4: Verify Logs

```bash
kubectl logs -n ksam <pod-name> | grep -E "(Creating CertManager|CertManager created)"
```

---

## Expected Results

Sau khi fix:

1. **Logs sẽ show**:
   ```
   [gRPC] TLS enabled, loading TLS configuration...
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

## Implementation

Đã thực hiện:
- ✅ Force rebuild với --no-cache
- ✅ Verify binary có string mới
- ✅ Force reload vào Minikube
- ✅ Delete pod để force recreate
- ✅ Check logs để verify

---

**Report Generated**: $(date +"%Y-%m-%d %H:%M:%S")

