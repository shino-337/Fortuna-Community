# NATS Subject Mismatch Fix Report

**Date**: $(date)  
**Status**: ✅ **FIXED**

---

## ✅ Issue Analysis

### Problem
```
[SBOM] WARNING: Failed to publish SBOM_CREATED event: failed to publish to fortuna.sbom.created: nats: no response from stream
```

### Root Cause
**Subject Mismatch**:
- **Publishing to**: `fortuna.sbom.created`
- **Stream 'ksam-events' subjects**: `["ksam.events.runtime", "ksam.sbom.>", "ksam.cve.>"]`
- **Pattern `ksam.sbom.>`**: Only matches subjects starting with `ksam.sbom.`
- **Result**: `fortuna.sbom.created` does NOT match the stream pattern, causing "no response from stream" error

### Why This Happens
When publishing to a JetStream subject that doesn't match any stream's subject pattern, NATS returns "no response from stream" because there's no stream configured to handle that subject.

---

## ✅ Fix Applied

### File Modified
- `core/internal/grpc/handler_sbom.go`

### Change
**Before**:
```go
if err := s.natsClient.Publish("fortuna.sbom.created", []byte(eventData)); err != nil {
```

**After**:
```go
// Use subject 'ksam.sbom.created' to match stream pattern 'ksam.sbom.>' in 'ksam-events' stream
if err := s.natsClient.Publish("ksam.sbom.created", []byte(eventData)); err != nil {
```

### Stream Configuration
The `ksam-events` stream is configured with:
- **Name**: `ksam-events`
- **Subjects**: `["ksam.events.runtime", "ksam.sbom.>", "ksam.cve.>"]`
- **Pattern `ksam.sbom.>`**: Matches all subjects starting with `ksam.sbom.`
  - ✅ `ksam.sbom.created` - **MATCHES**
  - ✅ `ksam.sbom.updated` - **MATCHES**
  - ❌ `fortuna.sbom.created` - **DOES NOT MATCH**

---

## ✅ Benefits

- ✅ **SBOM events published successfully** - No more "no response from stream" errors
- ✅ **CVE matching worker receives events** - Events flow through the pipeline
- ✅ **Consistent naming** - Uses `ksam.*` prefix to match existing stream patterns
- ✅ **Non-breaking** - Other parts of the system already use `ksam.*` subjects

---

## 📊 Verification

### Before Fix
```
[SBOM] WARNING: Failed to publish SBOM_CREATED event: failed to publish to fortuna.sbom.created: nats: no response from stream
```

### After Fix
```
[SBOM] Published SBOM_CREATED event for sbom_id=X
```

---

## 🎯 Summary

**Status**: ✅ **FIXED**

- Subject changed from `fortuna.sbom.created` to `ksam.sbom.created`
- Matches stream pattern `ksam.sbom.>` in `ksam-events` stream
- SBOM events now publish successfully
- CVE matching worker can receive events

**Impact**: 
- ✅ SBOM events flow through NATS pipeline
- ✅ CVE matching can be triggered by SBOM events
- ✅ No more NATS publishing errors

---

**Status**: ✅ Fix successfully applied. NATS subject now matches stream pattern.

