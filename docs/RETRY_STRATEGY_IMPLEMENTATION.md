# Retry Strategy Implementation

**Date**: 2025-12-01  
**Status**: ✅ **IMPLEMENTED**

---

## Overview

Implemented comprehensive retry strategy and error handling for workers, addressing Architecture Review Issue #3.

---

## Components Implemented

### 1. Error Classification (`retry.go`)

**Error Types**:
- **Retryable**: Transient errors that should be retried (network, database connection, timeouts)
- **Non-Retryable**: Permanent errors that should not be retried (invalid data, schema validation)
- **Fatal**: Errors that should crash the worker (out of memory, panic)

**Classification Logic**:
- Database connection errors → Retryable
- Network/timeout errors → Retryable
- Data validation errors → Non-Retryable
- NATS connection errors → Retryable
- Unknown errors → Retryable (safer default)

### 2. Retry Configuration

**Default Configuration**:
```go
MaxAttempts:  5
InitialDelay: 1 second
MaxDelay:     60 seconds
Multiplier:   2.0 (exponential)
Jitter:       true
```

**Retry Behavior**:
- Uses exponential backoff: `delay = initialDelay * (multiplier ^ attempt)`
- Adds jitter to prevent thundering herd
- Respects context cancellation
- Logs retry attempts

### 3. Dead Letter Queue (`dlq.go`)

**Features**:
- Automatic DLQ stream creation
- Stores failed messages with metadata:
  - Original subject and data
  - Error message and type
  - Attempt count
  - Timestamp
  - Worker name
- Alert threshold monitoring (default: 100 messages)
- 7-day retention

**DLQ Message Structure**:
```json
{
  "original_subject": "ksam.raw.pods",
  "original_data": {...},
  "error": "database connection failed",
  "error_type": "retryable",
  "attempts": 5,
  "timestamp": "2025-12-01T08:00:00Z",
  "worker_name": "correlator",
  "metadata": {...}
}
```

### 4. Worker Pool Integration (`pool.go`)

**Processing Flow**:
1. Worker receives message from NATS
2. Get delivery count from NATS metadata
3. Process message
4. If error:
   - Classify error type
   - If fatal/non-retryable → Send to DLQ, Ack message
   - If retryable and max attempts reached → Send to DLQ, Ack message
   - If retryable and attempts < max → Don't ack, let NATS redeliver
5. If success → Ack message

**Key Points**:
- Leverages NATS JetStream built-in redelivery mechanism
- No manual retry loop in worker (NATS handles it)
- DLQ captures all permanently failed messages
- Proper error classification prevents infinite retries

---

## Configuration

### Environment Variables

```bash
# Retry configuration (via code, can be made configurable)
WORKER_MAX_RETRY_ATTEMPTS=5
WORKER_INITIAL_RETRY_DELAY=1s
WORKER_MAX_RETRY_DELAY=60s

# DLQ configuration
DLQ_ENABLED=true
DLQ_STREAM_NAME=ksam-dlq
DLQ_ALERT_THRESHOLD=100
```

---

## Usage

### Default Usage

```go
// Create worker pool (automatically sets up DLQ)
workerPool, err := worker.NewPool(js, 5)
if err != nil {
    log.Fatalf("Failed to create worker pool: %v", err)
}

// Add workers
workerPool.AddWorker(worker.NewNormalizerWorker(js, db))
workerPool.AddWorker(worker.NewCorrelatorWorker(js, db))
workerPool.AddWorker(worker.NewRiskWorker(js, db))

// Start pool
workerPool.Start()
```

### Custom Retry Configuration

```go
// Create pool
workerPool, _ := worker.NewPool(js, 5)

// Custom retry config
customConfig := worker.RetryConfig{
    MaxAttempts:  10,
    InitialDelay: 2 * time.Second,
    MaxDelay:     120 * time.Second,
    Multiplier:   1.5,
    Jitter:       true,
}
workerPool.SetRetryConfig(customConfig)
```

---

## Error Handling Examples

### Retryable Error (Database Connection)

```
[WorkerPool] Worker correlator-0 retryable error (attempt 1/5): database connection failed. Will retry via NATS redelivery.
[NATS] Redelivers message after backoff
[WorkerPool] Worker correlator-0 processed message in 2.5s (after 2 attempts)
```

### Non-Retryable Error (Invalid Data)

```
[WorkerPool] Worker normalizer-0 non-retryable error: invalid json format
[DLQ] Sent message to DLQ: subject=ksam.raw.pods, worker=normalizer, attempts=1, error=invalid json format
[WorkerPool] Worker normalizer-0 processed message (acknowledged, sent to DLQ)
```

### Max Attempts Reached

```
[WorkerPool] Worker risk-0 max attempts (5) reached: database connection timeout
[DLQ] Sent message to DLQ: subject=ksam.normalized.pods, worker=risk, attempts=5, error=database connection timeout
[WorkerPool] Worker risk-0 processed message (acknowledged, sent to DLQ)
```

---

## Monitoring

### DLQ Statistics

```go
dlqManager := worker.NewDLQManager(js, config)
stats, err := dlqManager.GetDLQStats()
// Returns: message_count, byte_count, consumer_count, etc.
```

### Logs

All retry attempts and DLQ operations are logged:
- `[Retry]` prefix for retry operations
- `[DLQ]` prefix for DLQ operations
- `[WorkerPool]` prefix for worker pool operations

---

## Testing

### Test Retryable Errors

1. Simulate database connection failure
2. Verify message is not acked
3. Verify NATS redelivers message
4. Verify message eventually succeeds or goes to DLQ

### Test Non-Retryable Errors

1. Send invalid JSON data
2. Verify error is classified as non-retryable
3. Verify message is sent to DLQ
4. Verify message is acked (not redelivered)

### Test DLQ

1. Send message that will fail permanently
2. Wait for max attempts
3. Verify message appears in DLQ stream
4. Query DLQ statistics

---

## Benefits

1. **Reliability**: Messages are not lost on transient failures
2. **Efficiency**: Non-retryable errors don't waste resources
3. **Observability**: DLQ provides visibility into failed messages
4. **Flexibility**: Configurable retry behavior per use case
5. **Safety**: Prevents infinite retry loops

---

## Next Steps

1. ✅ Retry strategy implemented
2. ✅ Error classification implemented
3. ✅ DLQ implemented
4. ⏳ Circuit breaker (optional enhancement)
5. ⏳ Metrics collection for retry/DLQ
6. ⏳ DLQ replay mechanism

---

**Status**: ✅ **COMPLETED** (Core functionality)


