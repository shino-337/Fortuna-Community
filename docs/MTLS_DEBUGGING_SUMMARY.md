# mTLS Debugging Summary

**Date**: 2025-11-30  
**Status**: 🔴 IN PROGRESS - Logs not appearing despite code changes

---

## Problem

Agent cannot connect to Core via gRPC with mTLS:
```
tls: first record does not look like a TLS handshake
```

---

## Investigation Steps Taken

### 1. ✅ Code Changes Made
- Added detailed logging in `server.go`:
  - `[gRPC] TLS enabled, loading TLS configuration...`
  - `[gRPC] gRPC server configured with mTLS`
  - OR `[gRPC] WARNING: gRPC server running without TLS (insecure)`
- Added config debug logging in `main.go`:
  - `[Config] TLS_ENABLED=%v`
  - `[Config] TLS_CERT_PATH=%s`
  - etc.

### 2. ✅ Image Rebuilds
- Multiple rebuilds with `--no-cache`
- Removed `-s` flag from `-ldflags` to preserve strings
- Verified code exists in source files

### 3. ⚠️ Logs Not Appearing
- Expected logs (`[Config]`, `[gRPC]`) are NOT in output
- Only seeing: "Starting gRPC server on port 9090"
- No TLS configuration logs visible

### 4. ✅ Environment Verification
- `TLS_ENABLED=true` ✅
- Certificate files exist ✅
- Port 9090 listening ✅

---

## Root Cause Hypothesis

**Core gRPC server is likely running WITHOUT TLS**, despite:
- Environment variable `TLS_ENABLED=true`
- Certificate files mounted
- Code expecting TLS

**Possible reasons**:
1. `cfg.TLSEnabled` is `false` (config loading issue)
2. TLS config loading fails silently (error not logged)
3. Code changes not in binary (despite rebuilds)

---

## Next Steps

### Immediate Actions

1. **Direct Test**: Test TLS connection manually
   ```bash
   # From Agent pod
   openssl s_client -connect ksam-core.ksam.svc.cluster.local:9090 \
     -CAfile /etc/ksam/ca-cert/ca.crt \
     -cert /etc/ksam/certs/tls.crt \
     -key /etc/ksam/certs/tls.key
   ```

2. **Check Binary**: Verify binary contains new code
   ```bash
   kubectl exec -n ksam <core-pod> -- strings /root/ksam-core | grep "\[Config\]"
   ```

3. **Alternative Approach**: Add explicit error handling
   - Force log output before gRPC server creation
   - Add panic if TLS expected but not enabled
   - Test with TLS disabled to verify logs appear

### Alternative Solution

If Core is not running with TLS:
- **Option 1**: Fix config loading to ensure `TLSEnabled=true`
- **Option 2**: Temporarily disable TLS on Agent for testing
- **Option 3**: Use plaintext gRPC (development only)

---

## Current Status

- ✅ Code changes made
- ✅ Images rebuilt
- ⚠️ Logs still not appearing
- ⚠️ Agent still cannot connect
- ⚠️ Root cause not yet identified

---

## Recommendation

**Next Session**: 
1. Test TLS connection directly using openssl
2. Verify binary contains new code
3. If Core is not using TLS, fix config loading
4. If Core is using TLS but Agent fails, check certificate chain

---

**Note**: The issue may be that Core is running without TLS, causing the "first record does not look like a TLS handshake" error when Agent tries to connect with TLS.


