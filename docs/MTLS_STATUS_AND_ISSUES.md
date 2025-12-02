# mTLS Status and Current Issues

**Date**: 2025-12-01  
**Status**: ✅ mTLS Working | ⚠️ Core Pod Issues

---

## ✅ mTLS Status: WORKING

**Core gRPC Server is running with mTLS**:
```
[Config] TLS_ENABLED env='true', parsed=true
[gRPC] gRPC server configured with mTLS
Starting gRPC server on port 9090 WITH mTLS
```

**Configuration verified**:
- Core: `TLSEnabled=true` ✅
- Core: Certificates loaded ✅
- Agent: `TLS_ENABLED=true` ✅
- Agent: Endpoint configured correctly ✅

---

## ⚠️ Current Issues

### 1. Core Pod OOMKilled

**Problem**: Core pod is being killed due to memory limit (512Mi)

**Evidence**:
```
Last State:     Terminated
  Reason:       OOMKilled
  Exit Code:    137
Restart Count:  6
Limits:
  memory:  512Mi
```

**Impact**: Core pod keeps restarting, causing Agent connection failures

**Solution**: Increase memory limit or optimize memory usage

---

### 2. Database JSON Error

**Problem**: `ERROR: invalid input syntax for type json (SQLSTATE 22P02)`

**Evidence**:
```
[CorrelatorWorker] Processing normalized item: kind=Pod
ERROR: invalid input syntax for type json (SQLSTATE 22P02)
INSERT INTO "pods" ("containers", ...) VALUES ('[{"env":[...]}]', ...)
```

**Root Cause**: GORM is serializing JSON incorrectly - the `containers` field contains complex nested JSON that may have encoding issues or invalid characters.

**Impact**: Pods cannot be stored in database, causing data loss

**Solution**: 
1. Fix JSON serialization in CorrelatorWorker
2. Validate JSON before inserting
3. Use proper JSON encoding/escaping

---

## Next Steps

### Priority 1: Fix Core Pod Stability
1. **Increase memory limit** to 1Gi or 2Gi
2. **Optimize memory usage** in workers
3. **Add memory monitoring** to identify leaks

### Priority 2: Fix Database JSON Error
1. **Review CorrelatorWorker** JSON serialization
2. **Add JSON validation** before database insert
3. **Test with complex pod specs** (like Istio sidecars)

### Priority 3: Verify Agent Connection
1. **Wait for Core to stabilize** after fixes
2. **Test Agent mTLS connection**
3. **Verify end-to-end event flow**

---

## Summary

- ✅ **mTLS is working** - Core is correctly configured with mTLS
- ⚠️ **Core pod unstable** - OOMKilled and JSON errors causing restarts
- ⚠️ **Agent cannot connect** - Due to Core pod restarts

**Action Required**: Fix Core pod stability issues before verifying Agent connection.


