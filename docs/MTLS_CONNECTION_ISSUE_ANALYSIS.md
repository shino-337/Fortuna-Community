# mTLS Connection Issue Analysis

**Date**: 2025-11-30  
**Issue**: Agent cannot connect to Core via gRPC with mTLS  
**Error**: `tls: first record does not look like a TLS handshake`

---

## Problem Summary

Agent is experiencing mTLS connection failures when trying to connect to Core gRPC server:
```
rpc error: code = Unavailable desc = connection error: desc = "transport: authentication handshake failed: tls: first record does not look like a TLS handshake"
```

---

## Root Cause Analysis

### Hypothesis 1: Core gRPC Server Not Running with TLS
**Status**: ⚠️ LIKELY

**Evidence**:
- Core logs show "Starting gRPC server on port 9090" but NO log about TLS configuration
- Expected logs missing:
  - `[gRPC] TLS enabled, loading TLS configuration...`
  - `[gRPC] gRPC server configured with mTLS`
  - OR `[gRPC] WARNING: gRPC server running without TLS (insecure)`

**Possible Causes**:
1. `cfg.TLSEnabled` is `false` despite `TLS_ENABLED=true` environment variable
2. TLS config loading fails silently (error not logged)
3. Code changes not included in Docker image

### Hypothesis 2: Port Mismatch
**Status**: ❌ UNLIKELY

**Evidence**:
- Agent connects to: `ksam-core.ksam.svc.cluster.local:9090`
- Core gRPC server listens on: port `9090`
- ✅ Ports match

### Hypothesis 3: Certificate Mismatch
**Status**: ⚠️ POSSIBLE

**Evidence**:
- Certificates exist and are properly formatted
- CA certificate path: `/etc/ksam/ca-cert/ca.crt`
- Server certificate path: `/etc/ksam/certs/tls.crt`
- Client certificate path: `/etc/ksam/certs/tls.crt` (Agent)

**Possible Issues**:
- Server certificate CN doesn't match `ksam-core.ksam.svc.cluster.local`
- CA certificates don't match between Core and Agent
- Certificate chain incomplete

---

## Investigation Steps Taken

### 1. ✅ Verified Environment Variables
- Core: `TLS_ENABLED=true` ✅
- Core: `TLS_CERT_PATH=/etc/ksam/certs/tls.crt` ✅
- Core: `TLS_CA_CERT_PATH=/etc/ksam/ca-cert/ca.crt` ✅
- Agent: `TLS_ENABLED=true` ✅
- Agent: `KSAM_CORE_ENDPOINT=ksam-core.ksam.svc.cluster.local:9090` ✅

### 2. ✅ Verified Certificate Files Exist
- Core certificates: ✅ All exist
- Agent certificates: ✅ All exist
- Certificate format: ✅ Valid PEM format

### 3. ✅ Verified gRPC Server is Listening
- Port 9090: ✅ Listening
- Process: ✅ Running

### 4. ⚠️ Missing TLS Configuration Logs
- Expected logs not appearing
- Code changes may not be in image
- Or TLS config loading fails silently

---

## Code Changes Made

### 1. Enhanced Logging in `server.go`
```go
// Added detailed logging for TLS configuration
log.Printf("[gRPC] TLS enabled, loading TLS configuration...")
log.Printf("[gRPC] Loading CA certificate from: %s", cfg.TLSCACertPath)
log.Printf("[gRPC] CA certificate loaded successfully")
log.Printf("[gRPC] Loading server certificate from: %s, key from: %s", ...)
log.Printf("[gRPC] Server certificate loaded successfully")
log.Printf("[gRPC] gRPC server configured with mTLS")
```

### 2. Added Config Debug Logging in `main.go`
```go
log.Printf("[Config] TLS_ENABLED=%v, TLS_CERT_PATH=%s, TLS_CA_CERT_PATH=%s", 
    cfg.TLSEnabled, cfg.TLSCertPath, cfg.TLSCACertPath)
```

---

## Next Steps

### Immediate Actions

1. **Verify Image Contains New Code**
   - Rebuild image with `--no-cache`
   - Verify logs appear after restart

2. **Check Config Loading**
   - Verify `cfg.TLSEnabled` is `true` in runtime
   - Check if `getEnv("TLS_ENABLED", "false")` returns correct value

3. **Test TLS Connection Directly**
   - Use `openssl s_client` to test TLS handshake
   - Verify certificate chain

4. **Check Certificate Details**
   - Verify server certificate CN matches endpoint
   - Verify CA certificates match between Core and Agent

### Alternative Solution

If Core is not running with TLS:
- **Option 1**: Fix TLS configuration loading
- **Option 2**: Temporarily disable TLS for testing (not recommended for production)
- **Option 3**: Use plaintext gRPC (insecure, only for development)

---

## Current Status

- ✅ Code changes made for better logging
- ✅ Image rebuilt
- ⚠️ Logs still not appearing (investigating)
- ⚠️ Agent still cannot connect

---

## Recommended Fix

1. **Add explicit error handling** in `NewServer` to ensure TLS config errors are logged
2. **Verify config loading** by adding debug output
3. **Test certificate chain** manually
4. **Check if Core is actually running with TLS** by testing connection

---

**Next Action**: Continue investigation to determine if Core is running with or without TLS.


