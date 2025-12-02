# Phương án Xử lý CertManager Issue

**Date**: $(date +"%Y-%m-%d %H:%M:%S")  
**Issue**: Certificate API endpoints trả về 404

---

## Root Cause Analysis

### Từ Logs:
- ✅ TLS_ENABLED=true
- ✅ gRPC server created successfully  
- ✅ Certificate files exist và readable
- ❌ **KHÔNG thấy logs từ CertManager.NewCertManager()**
- ❌ **KHÔNG thấy logs '[CertManager] Loading certificate...'**

### Code Flow Analysis:

```
main.go:119 → grpc.NewServer()
  └─> server.go:NewServer()
      └─> if cfg.TLSEnabled {
            └─> security.NewCertManager()  ← Được gọi
                └─> cert_manager.go:NewCertManager()
                    └─> LoadCertificate()  ← Có log "[CertManager] Loading..."
                    └─> LoadCACertificate()
          }
      └─> return &Server{certManager: certManager}

main.go:144 → certManager := grpcServer.GetCertManager()
main.go:145 → api.SetupRoutesWithCertManager(..., certManager)
  └─> routes.go:SetupRoutesWithCertManager()
      └─> if certManager != nil {
            └─> Register routes
          }
```

### Possible Issues:

1. **CertManager được tạo nhưng logs không xuất hiện**
   - Logs có thể bị filter hoặc không được output
   - CertManager.NewCertManager() có thể fail silently

2. **CertManager được tạo nhưng GetCertManager() return nil**
   - Server struct không được initialize đúng cách
   - certManager field không được set

3. **Routes được register nhưng path không đúng**
   - Path mismatch giữa test và actual routes
   - Routes nằm trong protected group với auth

4. **Auth middleware block requests**
   - Routes nằm trong `v1` group với `AuthMiddleware`
   - Requests không có token → 401 (không phải 404)

---

## Solution Plan

### Step 1: Add Comprehensive Logging

**File**: `core/cmd/main.go`
```go
// Setup routes with certificate manager (if available)
certManager := grpcServer.GetCertManager()
log.Printf("========================================")
log.Printf("[Main] CertManager Status Check")
log.Printf("[Main]   certManager == nil: %v", certManager == nil)
log.Printf("[Main]   TLS_ENABLED: %v", cfg.TLSEnabled)
if certManager != nil {
    log.Printf("[Main] ✅ CertManager available - certificate routes will be registered")
} else {
    log.Printf("[Main] ⚠️  CertManager is nil - certificate routes will NOT be registered")
    log.Printf("[Main]   This may be expected if TLS is not enabled")
}
log.Printf("========================================")
api.SetupRoutesWithCertManager(router, db, cfg, certManager)
```

**File**: `core/internal/api/routes.go`
```go
// Certificate management (if cert manager available)
if certManager != nil {
    log.Printf("[API] ========================================")
    log.Printf("[API] Registering certificate management routes")
    log.Printf("[API]   CertManager: %v", certManager != nil)
    certHandler := NewCertHandler(certManager)
    certs := v1.Group("/certificates")
    {
        certs.GET("/info", certHandler.GetCertificateInfo)
        certs.POST("/rotate", certHandler.RotateCertificate)
    }
    log.Printf("[API] ✅ Certificate routes registered:")
    log.Printf("[API]   GET  /api/v1/certificates/info")
    log.Printf("[API]   POST /api/v1/certificates/rotate")
    log.Printf("[API] ========================================")
} else {
    log.Printf("[API] ⚠️  CertManager is nil - certificate routes NOT registered")
    log.Printf("[API]   TLS_ENABLED: %v", cfg.TLSEnabled)
}
```

**File**: `core/internal/grpc/server.go`
```go
// GetCertManager returns the certificate manager (for API handlers)
func (s *Server) GetCertManager() *security.CertManager {
    log.Printf("[gRPC] GetCertManager() called")
    log.Printf("[gRPC]   certManager == nil: %v", s.certManager == nil)
    log.Printf("[gRPC]   TLS_ENABLED: %v", s.config.TLSEnabled)
    return s.certManager
}
```

### Step 2: Verify CertManager Creation

**File**: `core/internal/grpc/server.go`
```go
// Create certificate manager for dynamic loading
var err error
certManager, err = security.NewCertManager(
    cfg.TLSCertPath,
    cfg.TLSKeyPath,
    cfg.TLSCACertPath,
)
if err != nil {
    log.Printf("[gRPC] ERROR: Failed to create certificate manager: %v", err)
    return nil, fmt.Errorf("failed to create certificate manager: %w", err)
}
log.Printf("[gRPC] ✅ CertManager created successfully")
log.Printf("[gRPC]   CertManager pointer: %p", certManager)
```

### Step 3: Add Debug Endpoint

**File**: `core/internal/api/routes.go`
```go
// Debug endpoint (no auth required)
router.GET("/api/v1/debug/certmanager", func(c *gin.Context) {
    // Get certManager from context or global (need to pass it)
    // For now, return config info
    c.JSON(200, gin.H{
        "tls_enabled": cfg.TLSEnabled,
        "cert_path": cfg.TLSCertPath,
        "key_path": cfg.TLSKeyPath,
        "ca_cert_path": cfg.TLSCACertPath,
        "cert_manager_available": certManager != nil,
    })
})
```

### Step 4: Check Auth Middleware

**File**: `core/internal/api/routes.go`
```go
// Certificate management (if cert manager available)
// Note: Move outside protected group if auth is blocking
if certManager != nil {
    certHandler := NewCertHandler(certManager)
    
    // Option A: Keep in protected group (requires auth)
    certs := v1.Group("/certificates")
    {
        certs.GET("/info", certHandler.GetCertificateInfo)
        certs.POST("/rotate", certHandler.RotateCertificate)
    }
    
    // Option B: Move to public group (no auth required)
    // certs := router.Group("/api/v1/certificates")
    // {
    //     certs.GET("/info", certHandler.GetCertificateInfo)
    //     certs.POST("/rotate", certHandler.RotateCertificate)
    // }
}
```

---

## Implementation Steps

### 1. Add Logging (Immediate)
- [ ] Add logging trong `main.go` để check CertManager status
- [ ] Add logging trong `routes.go` để check routes registration
- [ ] Add logging trong `server.go:GetCertManager()` để debug

### 2. Rebuild và Deploy
- [ ] Rebuild Core image
- [ ] Deploy to Minikube
- [ ] Check logs để identify root cause

### 3. Fix Based on Root Cause
- [ ] Nếu CertManager nil → fix creation
- [ ] Nếu routes không register → fix registration
- [ ] Nếu auth blocking → move routes hoặc add auth bypass

### 4. Test và Verify
- [ ] Test certificate API endpoints
- [ ] Verify Prometheus metrics
- [ ] Check logs để confirm fix

---

## Expected Outcomes

Sau khi apply solutions:

1. **Logs sẽ show**:
   ```
   [Main] CertManager Status Check
   [Main]   certManager == nil: false
   [Main] ✅ CertManager available - certificate routes will be registered
   [API] Registering certificate management routes
   [API] ✅ Certificate routes registered:
   [API]   GET  /api/v1/certificates/info
   [API]   POST /api/v1/certificates/rotate
   ```

2. **Certificate API sẽ**:
   - Return 200 với certificate info (nếu CertManager available)
   - Return proper error message (nếu CertManager nil)
   - Show trong logs tại sao routes không được register

3. **Prometheus Metrics sẽ**:
   - Appear nếu CertManager được khởi tạo thành công
   - Show certificate expiry và rotation metrics

---

## Next Actions

1. ✅ Apply Step 1 (Add Logging)
2. ✅ Rebuild và deploy
3. ✅ Check logs để identify root cause
4. ✅ Apply fix dựa trên root cause
5. ✅ Test và verify

---

**Report Generated**: $(date +"%Y-%m-%d %H:%M:%S")

