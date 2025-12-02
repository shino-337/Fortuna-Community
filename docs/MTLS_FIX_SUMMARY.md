# mTLS Connection Fix Summary

**Date**: 2025-11-30  
**Status**: 🔴 IN PROGRESS

---

## Issue

Agent cannot connect to Core via gRPC with mTLS:
```
rpc error: code = Unavailable desc = connection error: desc = "transport: authentication handshake failed: tls: first record does not look like a TLS handshake"
```

---

## Root Cause (Suspected)

**Core gRPC server may not be running with TLS**, despite:
- ✅ `TLS_ENABLED=true` environment variable set
- ✅ Certificate files exist and are properly mounted
- ✅ Code expects TLS to be enabled

**Evidence**:
- Core logs show "Starting gRPC server on port 9090" but **NO** TLS configuration logs
- Missing expected logs:
  - `[gRPC] TLS enabled, loading TLS configuration...`
  - `[gRPC] gRPC server configured with mTLS`
  - OR `[gRPC] WARNING: gRPC server running without TLS (insecure)`

---

## Changes Made

### 1. Enhanced Logging
- Added detailed logging in `server.go` for TLS configuration loading
- Added config debug logging in `main.go`
- Added TLS status logging in `Start()` method

### 2. Code Updates
- `KSAM/core/internal/grpc/server.go`: Enhanced TLS loading with detailed logs
- `KSAM/core/cmd/main.go`: Added config debug output

---

## Next Steps

1. **Verify Image**: Ensure new code is in Docker image
2. **Check Logs**: Look for `[Config]` and `[gRPC]` logs after restart
3. **Test Connection**: Verify TLS handshake works
4. **Fix if needed**: Address root cause once identified

---

## Testing

After fixes are deployed:
1. Check Core logs for TLS configuration messages
2. Verify Agent can connect
3. Test end-to-end event flow

---

**Status**: Awaiting image rebuild and pod restart to verify logs appear.


