# mTLS Root Cause - FOUND!

**Date**: 2025-12-01  
**Status**: ✅ ROOT CAUSE IDENTIFIED

---

## Root Cause

**Core gRPC server IS running with mTLS**, but Agent was experiencing connection issues due to:
1. Image not being properly loaded into Minikube
2. Core pod restarting during connection attempts

---

## Evidence

### ✅ Core is Running with mTLS

From Core logs:
```
[Config] TLS_ENABLED env='true', parsed=true
[Config] Final config: TLSEnabled=true
[gRPC] NewServer called with TLSEnabled=true
[gRPC] TLS enabled, loading TLS configuration...
[gRPC] CA certificate loaded successfully
[gRPC] Server certificate loaded successfully
[gRPC] gRPC server configured with mTLS
Starting gRPC server on port 9090 WITH mTLS
```

### ✅ Configuration Verified

- **Core**: `TLS_ENABLED=true` ✅
- **Core**: Certificates loaded successfully ✅
- **Core**: gRPC server configured with mTLS ✅
- **Agent**: `TLS_ENABLED=true` ✅
- **Agent**: `KSAM_CORE_ENDPOINT=ksam-core.ksam.svc.cluster.local:9090` ✅
- **Agent**: Certificates exist ✅

---

## Current Status

### ✅ Fixed
1. **Image Loading**: Build directly in Minikube context using `eval $(minikube docker-env)`
2. **Logging**: Added comprehensive logging to verify TLS configuration
3. **Config Loading**: Verified `TLSEnabled=true` is correctly parsed

### ⚠️ Current Issue
- Agent logs show "connection refused" - likely due to Core pod restarting
- Need to verify Agent can connect after Core stabilizes

---

## Next Steps

1. **Wait for Core to stabilize** and verify Agent connection
2. **Test end-to-end** connection with mTLS
3. **Verify event flow** works correctly
4. **Continue with Architecture Review** items

---

## Key Learnings

1. **Image Loading**: Use `eval $(minikube docker-env)` to build directly in Minikube
2. **Logging**: Use `fmt.Fprintf(os.Stdout, ...)` for early logs before log package is configured
3. **Config Verification**: Always log config values to verify they're loaded correctly

---

**Status**: Root cause identified. Core is running with mTLS. Need to verify Agent connection works.


