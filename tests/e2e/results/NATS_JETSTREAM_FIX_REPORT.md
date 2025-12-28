# NATS JetStream Fix Report

**Date**: 2025-12-27  
**Time**: 21:00 UTC

---

## Problem

Core pod was failing to start with error:
```
Failed to connect to NATS: failed to setup streams: failed to create stream ksam-raw: nats: JetStream system temporarily unavailable
```

---

## Root Cause Analysis

1. **JetStream Availability**: NATS JetStream may not be immediately available when Core starts
2. **Replication Issues**: Streams configured with `Replicas: 3` require quorum, which may fail during cluster startup
3. **No Retry Logic**: Stream creation failed immediately without retries

---

## Fixes Applied

### 1. Added JetStream Retry Logic
**File**: `core/pkg/messaging/nats_client.go`

- Added retry logic when getting JetStream context (10 retries, 2s delay)
- Prevents immediate failure if JetStream is temporarily unavailable

```go
// Wait for JetStream to be available (with retries)
maxRetries := 10
retryDelay := 2 * time.Second
for i := 0; i < maxRetries; i++ {
    js, err = conn.JetStream()
    if err == nil {
        break
    }
    if i < maxRetries-1 {
        log.Printf("[NATS] JetStream not available yet (attempt %d/%d), retrying in %v...", i+1, maxRetries, retryDelay)
        time.Sleep(retryDelay)
    }
}
```

### 2. Reduced Replication Factor
**File**: `core/pkg/messaging/nats_client.go`

- Changed `Replicas: 3` to `Replicas: 1`
- Avoids quorum issues during cluster startup
- Can be increased later when cluster is stable

```go
Replicas: 1, // Use 1 replica for now to avoid quorum issues during startup
```

### 3. Added Stream Creation Retry Logic
**File**: `core/pkg/messaging/nats_client.go`

- Added exponential backoff retry for stream creation (5 retries)
- Handles temporary JetStream unavailability gracefully

```go
// Retry stream creation with exponential backoff
maxRetries := 5
retryDelay := 2 * time.Second
for i := 0; i < maxRetries; i++ {
    _, err := c.js.AddStream(cfg)
    if err == nil {
        break
    }
    // ... retry logic with exponential backoff
}
```

---

## Benefits

1. **Resilience**: Core can now handle temporary JetStream unavailability
2. **Startup Reliability**: Streams are created with retries, reducing startup failures
3. **Quorum Avoidance**: Single replica avoids quorum issues during cluster startup

---

## Testing

After applying fixes:
1. Rebuild Core image
2. Restart Core pod
3. Monitor logs for successful NATS connection and stream creation

---

## Status

- ✅ Fixes applied to code
- ⏳ Core image rebuilt
- ⏳ Core pod restarting
- ⏳ Monitoring for successful connection

---

**Report Generated**: 2025-12-27 21:00 UTC

