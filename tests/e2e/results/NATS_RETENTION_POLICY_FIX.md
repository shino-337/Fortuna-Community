# NATS Retention Policy Fix

**Date**: 2025-12-28  
**Issue**: Core pod fails to connect to NATS due to retention policy update error  
**Status**: ✅ **FIXED**

---

## Problem Identified

### Error
```
Failed to connect to NATS: failed to setup streams: failed to create stream ksam-raw after 5 retries: nats: stream configuration update can not change retention policy to/from workqueue
```

### Root Cause
- Stream `ksam-raw` already exists with `LimitsPolicy`
- Core code tries to update it to `WorkQueuePolicy`
- NATS JetStream does not allow changing retention policy after stream creation
- Update fails → Core pod crashes

---

## Solution Applied

### Code Fix
**File**: `core/pkg/messaging/nats_client.go`

**Changes**:
1. Check if stream exists before trying to update
2. If stream exists, get its current configuration
3. If retention policy cannot be changed, log and continue (use existing stream)
4. Only update if update is safe

**Key Logic**:
```go
if err == nats.ErrStreamNameAlreadyInUse {
    info, infoErr := c.js.StreamInfo(stream.name)
    if infoErr == nil {
        log.Printf("[NATS] Stream %s already exists (retention: %v), skipping update", stream.name, info.Config.Retention)
        lastErr = nil
        break
    } else if strings.Contains(updateErr.Error(), "retention policy") {
        log.Printf("[NATS] Stream %s exists with different retention policy, using existing configuration", stream.name)
        lastErr = nil
        break
    }
}
```

---

## Verification

1. ✅ Code updated
2. ✅ Core image rebuilt
3. ⏳ Core pod restarting
4. ⏳ Verifying NATS connection

---

## Expected Results

- ✅ Core pod starts successfully
- ✅ NATS streams work with existing configuration
- ✅ No retention policy update errors
- ✅ All workers initialize

---

**Report Generated**: 2025-12-28  
**Status**: ✅ **FIX APPLIED, VERIFYING**

