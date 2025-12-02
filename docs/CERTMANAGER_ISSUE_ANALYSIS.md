# Phân tích Vấn đề CertManager và Phương án Xử lý

**Date**: $(date +"%Y-%m-%d %H:%M:%S")  
**Issue**: Certificate API endpoints trả về 404 mặc dù TLS_ENABLED=true

---

## Phân tích Code Flow

### Flow hiện tại:

```
main.go (line 119)
  └─> grpc.NewServer(cfg, db, natsClient)
      └─> server.go:NewServer()
          ├─> if cfg.TLSEnabled {
          │     └─> security.NewCertManager(certPath, keyPath, caCertPath)
          │         └─> cert_manager.go:NewCertManager()
          │             ├─> LoadCertificate() - Load từ disk
          │             └─> LoadCACertificate() - Load CA cert
          │     └─> certManager.StartExpiryMonitoring()
          └─> return &Server{certManager: certManager}
      
main.go (line 144)
  └─> certManager := grpcServer.GetCertManager()
      └─> server.go:GetCertManager() - return s.certManager

main.go (line 145)
  └─> api.SetupRoutesWithCertManager(router, db, cfg, certManager)
      └─> routes.go:SetupRoutesWithCertManager()
          └─> if certManager != nil {
                └─> certHandler := NewCertHandler(certManager)
                └─> certs.GET("/info", ...)
                └─> certs.POST("/rotate", ...)
              }
```

---

## Vấn đề Phát hiện

### 1. **CertManager có thể là nil**

**Nguyên nhân**:
- Nếu `cfg.TLSEnabled == false`, `certManager` sẽ là `nil`
- Nếu `security.NewCertManager()` fail, gRPC server sẽ fail và app crash
- Nhưng nếu có silent failure hoặc CertManager được tạo nhưng không được store đúng cách

**Code trong `server.go`**:
```go
var certManager *security.CertManager
if cfg.TLSEnabled {
    certManager, err = security.NewCertManager(...)
    if err != nil {
        return nil, fmt.Errorf("failed to create certificate manager: %w", err)
    }
}
// certManager có thể là nil nếu TLS không enabled
return &Server{certManager: certManager}
```

### 2. **Certificate Files có thể không accessible**

**Kiểm tra**:
- Certificate files đã mount vào pod: ✅
- Files tồn tại: ✅
- Nhưng có thể có vấn đề về permissions hoặc file format

### 3. **Routes chỉ được register nếu certManager != nil**

**Code trong `routes.go`**:
```go
if certManager != nil {
    certHandler := NewCertHandler(certManager)
    certs := v1.Group("/certificates")
    {
        certs.GET("/info", certHandler.GetCertificateInfo)
        certs.POST("/rotate", certHandler.RotateCertificate)
    }
}
```

**Vấn đề**: Nếu `certManager == nil`, routes sẽ không được register → 404

### 4. **Protected Routes có Auth Middleware**

**Code trong `routes.go`**:
```go
v1 := router.Group("/api/v1")
if cfg.AuthEnabled {
    v1.Use(middleware.AuthMiddleware(db, cfg.JWTSecret))
}
```

**Vấn đề**: Certificate routes nằm trong `v1` group, nếu `AuthEnabled=true`, cần authentication token.

---

## Root Cause Analysis

### Scenario 1: CertManager là nil
- **Nguyên nhân**: TLS enabled nhưng CertManager không được tạo thành công
- **Triệu chứng**: Routes không được register → 404
- **Xác nhận**: Cần check logs để xem có lỗi khi tạo CertManager không

### Scenario 2: Certificate Files không readable
- **Nguyên nhân**: File permissions hoặc file format không đúng
- **Triệu chứng**: `NewCertManager()` fail → gRPC server fail → app crash
- **Xác nhận**: Cần check logs và test file read

### Scenario 3: Auth Required
- **Nguyên nhân**: `AUTH_ENABLED=true` và routes nằm trong protected group
- **Triệu chứng**: 401 Unauthorized (không phải 404)
- **Xác nhận**: Check `AUTH_ENABLED` env var

### Scenario 4: Routes được register nhưng path sai
- **Nguyên nhân**: Path mismatch giữa test và actual routes
- **Triệu chứng**: 404 với correct path
- **Xác nhận**: Check actual routes registered

---

## Phương án Xử lý

### Solution 1: Add Logging để Debug

**File**: `core/cmd/main.go`
```go
// Setup routes with certificate manager (if available)
certManager := grpcServer.GetCertManager()
if certManager == nil {
    log.Printf("[Main] WARNING: CertManager is nil - certificate routes will not be registered")
    log.Printf("[Main] TLS_ENABLED=%v", cfg.TLSEnabled)
} else {
    log.Printf("[Main] ✅ CertManager available - certificate routes will be registered")
}
api.SetupRoutesWithCertManager(router, db, cfg, certManager)
```

**File**: `core/internal/api/routes.go`
```go
// Certificate management (if cert manager available)
if certManager != nil {
    log.Printf("[API] Registering certificate management routes")
    certHandler := NewCertHandler(certManager)
    certs := v1.Group("/certificates")
    {
        certs.GET("/info", certHandler.GetCertificateInfo)
        certs.POST("/rotate", certHandler.RotateCertificate)
    }
    log.Printf("[API] ✅ Certificate routes registered: GET /api/v1/certificates/info, POST /api/v1/certificates/rotate")
} else {
    log.Printf("[API] ⚠️  CertManager is nil - certificate routes NOT registered")
}
```

### Solution 2: Make Certificate Routes Public (nếu cần)

**File**: `core/internal/api/routes.go`
```go
// Certificate management (if cert manager available)
// Note: Certificate routes are public (no auth required) for monitoring purposes
if certManager != nil {
    certHandler := NewCertHandler(certManager)
    certs := router.Group("/api/v1/certificates") // Public group, not v1
    {
        certs.GET("/info", certHandler.GetCertificateInfo)
        certs.POST("/rotate", certHandler.RotateCertificate)
    }
}
```

### Solution 3: Add Health Check cho CertManager

**File**: `core/internal/grpc/server.go`
```go
// GetCertManager returns the certificate manager (for API handlers)
func (s *Server) GetCertManager() *security.CertManager {
    if s.certManager == nil {
        log.Printf("[gRPC] WARNING: GetCertManager() called but certManager is nil")
        log.Printf("[gRPC] TLS_ENABLED=%v", s.config.TLSEnabled)
    }
    return s.certManager
}
```

### Solution 4: Verify Certificate Loading

**File**: `core/pkg/security/cert_manager.go`
```go
func NewCertManager(certPath, keyPath, caCertPath string) (*CertManager, error) {
    log.Printf("[CertManager] Creating CertManager with:")
    log.Printf("[CertManager]   CertPath: %s", certPath)
    log.Printf("[CertManager]   KeyPath: %s", keyPath)
    log.Printf("[CertManager]   CACertPath: %s", caCertPath)
    
    // Check files exist
    if _, err := os.Stat(certPath); os.IsNotExist(err) {
        return nil, fmt.Errorf("certificate file not found: %s", certPath)
    }
    if _, err := os.Stat(keyPath); os.IsNotExist(err) {
        return nil, fmt.Errorf("key file not found: %s", keyPath)
    }
    if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
        return nil, fmt.Errorf("CA certificate file not found: %s", caCertPath)
    }
    
    // ... rest of the code
}
```

### Solution 5: Add Debug Endpoint để Check CertManager Status

**File**: `core/internal/api/routes.go`
```go
// Debug endpoint to check CertManager status
router.GET("/api/v1/debug/certmanager", func(c *gin.Context) {
    certManager := grpcServer.GetCertManager() // Need to pass grpcServer or get from context
    if certManager == nil {
        c.JSON(200, gin.H{
            "status": "not_available",
            "reason": "CertManager is nil",
            "tls_enabled": cfg.TLSEnabled,
        })
        return
    }
    
    info, err := certManager.GetCertificateInfo()
    if err != nil {
        c.JSON(200, gin.H{
            "status": "error",
            "error": err.Error(),
        })
        return
    }
    
    c.JSON(200, gin.H{
        "status": "available",
        "certificate": info,
    })
})
```

---

## Recommended Action Plan

### Step 1: Add Logging (Immediate)
1. Add logging trong `main.go` để check CertManager status
2. Add logging trong `routes.go` để check routes registration
3. Add logging trong `server.go:GetCertManager()` để debug

### Step 2: Verify Certificate Loading
1. Check logs để xem CertManager có được tạo thành công không
2. Test certificate files có readable không
3. Verify certificate format

### Step 3: Fix Routes Registration
1. Nếu CertManager nil → fix nguyên nhân
2. Nếu routes không được register → add logging và fix
3. Nếu auth required → move routes ra ngoài protected group hoặc add auth bypass

### Step 4: Test và Verify
1. Rebuild image với logging mới
2. Deploy và check logs
3. Test certificate API endpoints
4. Verify Prometheus metrics

---

## Expected Outcomes

Sau khi apply các solutions:

1. **Logs sẽ show**:
   - CertManager creation status
   - Routes registration status
   - Certificate loading status

2. **Certificate API sẽ**:
   - Return 200 với certificate info (nếu CertManager available)
   - Return proper error message (nếu CertManager nil)
   - Show trong logs tại sao routes không được register

3. **Prometheus Metrics sẽ**:
   - Appear nếu CertManager được khởi tạo thành công
   - Show certificate expiry và rotation metrics

---

## Next Steps

1. ✅ Apply Solution 1 (Add Logging)
2. ✅ Apply Solution 3 (Health Check)
3. ✅ Rebuild và deploy
4. ✅ Check logs để identify root cause
5. ✅ Apply fix dựa trên root cause
6. ✅ Test và verify

---

**Report Generated**: $(date +"%Y-%m-%d %H:%M:%S")

