# Phân tích và Giải pháp Xử lý

**Date**: 2025-12-01  
**Status**: Đang xử lý

---

## 1. Phân tích Vấn đề Hiện tại

### 1.1. JSON Database Error (Đã fix)

**Vấn đề**: 
```
ERROR: invalid input syntax for type json (SQLSTATE 22P02)
```

**Nguyên nhân**:
- GORM tự động escape JSON string khi insert vào jsonb field
- PostgreSQL yêu cầu JSON hợp lệ, nhưng GORM có thể double-encode
- Containers JSON có thể chứa special characters (newlines, quotes)

**Giải pháp đã áp dụng**:
- Sử dụng raw SQL với `::jsonb` cast để tránh GORM encoding
- Validate JSON trước khi insert
- Đảm bảo cluster tồn tại trước khi insert pod

**Code changes**:
```go
// Sử dụng raw SQL thay vì GORM model
insertSQL := `INSERT INTO pods (uid, cluster_id, name, namespace, service_account, containers, created_at, updated_at)
    VALUES (?, ?, ?, ?, ?, ?::jsonb, ?, ?)`
w.db.Exec(insertSQL, ..., containersJSON, ...)
```

---

### 1.2. Core Pod OOMKilled (Đã fix)

**Vấn đề**: 
- Core pod bị kill do memory limit 512Mi quá thấp
- Pod restart liên tục, Agent không thể kết nối

**Giải pháp đã áp dụng**:
- Tăng memory limit từ 512Mi → 2Gi
- Tăng CPU limit từ 500m → 1000m
- Tăng memory request từ 256Mi → 512Mi

**Deployment changes**:
```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "100m"
  limits:
    memory: "2Gi"
    cpu: "1000m"
```

---

### 1.3. mTLS Connection (Đã xác nhận working)

**Status**: ✅ Core đang chạy với mTLS
- Configuration verified: `TLSEnabled=true`
- Certificates loaded successfully
- gRPC server configured with mTLS

**Pending**: Verify Agent connection sau khi Core ổn định

---

## 2. Các Vấn đề Còn lại từ Architecture Review

### 2.1. Error Handling & Retry Strategy (Priority: High)

**Vấn đề**: Workers không có retry mechanism khi xử lý messages

**Giải pháp đề xuất**:
1. **Implement exponential backoff retry**:
   ```go
   type RetryConfig struct {
       MaxRetries    int
       InitialDelay  time.Duration
       MaxDelay      time.Duration
       BackoffFactor float64
   }
   ```

2. **Add dead letter queue** cho messages failed sau max retries

3. **Implement circuit breaker** để tránh cascade failures

**Files cần sửa**:
- `KSAM/core/pkg/worker/worker_pool.go`
- `KSAM/core/pkg/worker/normalizer_worker.go`
- `KSAM/core/pkg/worker/correlator_worker.go`
- `KSAM/core/pkg/worker/risk_worker.go`

---

### 2.2. Rate Limiting cho Ingest API (Priority: High)

**Vấn đề**: Ingest API không có rate limiting, có thể bị overload

**Giải pháp đề xuất**:
1. **Implement token bucket rate limiter**:
   ```go
   type RateLimiter struct {
       tokens    int
       capacity  int
       refillRate time.Duration
   }
   ```

2. **Add rate limiting middleware** cho gRPC server

3. **Configurable limits** qua environment variables

**Files cần sửa**:
- `KSAM/core/internal/grpc/server.go`
- `KSAM/core/internal/ingest/ingest.go`

---

### 2.3. Database Connection Pooling (Priority: Medium)

**Vấn đề**: Có thể có connection leaks hoặc pool không đủ

**Giải pháp đề xuất**:
1. **Configure GORM connection pool**:
   ```go
   sqlDB.SetMaxIdleConns(10)
   sqlDB.SetMaxOpenConns(100)
   sqlDB.SetConnMaxLifetime(time.Hour)
   ```

2. **Add connection monitoring** và alerts

**Files cần sửa**:
- `KSAM/core/internal/storage/storage.go`

---

### 2.4. NATS Stream Configuration (Priority: Medium)

**Vấn đề**: Streams có thể không được config đúng retention policy

**Giải pháp đề xuất**:
1. **Configure retention policy**:
   - Max age: 7 days
   - Max bytes: 10GB per stream
   - Storage type: File

2. **Add stream monitoring** và alerts

**Files cần sửa**:
- `KSAM/core/pkg/messaging/nats_client.go`

---

### 2.5. Logging & Observability (Priority: Medium)

**Vấn đề**: Logs chưa structured, thiếu metrics

**Giải pháp đề xuất**:
1. **Structured logging** với JSON format
2. **Add Prometheus metrics**:
   - Message processing rate
   - Error rates
   - Processing latency
   - Database query duration

3. **Add distributed tracing** (OpenTelemetry)

**Files cần sửa**:
- Tất cả worker files
- API handlers
- gRPC server

---

## 3. Kế hoạch Thực hiện

### Phase 1: Stability Fixes (Immediate)
- [x] Fix JSON database error
- [x] Fix Core pod OOMKilled
- [x] Verify mTLS configuration
- [ ] Verify Agent connection
- [ ] Test end-to-end event flow

### Phase 2: Error Handling (High Priority)
- [ ] Implement retry strategy cho workers
- [ ] Add dead letter queue
- [ ] Implement circuit breaker
- [ ] Add error monitoring

### Phase 3: Performance & Scalability (Medium Priority)
- [ ] Add rate limiting cho Ingest API
- [ ] Optimize database connection pool
- [ ] Configure NATS retention policies
- [ ] Add caching layer (Redis)

### Phase 4: Observability (Medium Priority)
- [ ] Structured logging
- [ ] Prometheus metrics
- [ ] Distributed tracing
- [ ] Dashboard & alerts

---

## 4. Testing Strategy

### 4.1. Unit Tests
- Test JSON serialization/deserialization
- Test retry logic
- Test rate limiting

### 4.2. Integration Tests
- Test event flow end-to-end
- Test database operations
- Test NATS message flow

### 4.3. Load Tests
- Test rate limiting under load
- Test worker pool under high message rate
- Test database connection pool limits

---

## 5. Monitoring & Alerts

### 5.1. Key Metrics
- Message processing rate
- Error rate
- Processing latency
- Database connection pool usage
- Memory usage
- CPU usage

### 5.2. Alerts
- High error rate (> 5%)
- High processing latency (> 1s)
- Database connection pool exhaustion
- OOMKilled events
- Agent disconnection

---

## 6. Next Steps

1. **Immediate** (Today):
   - Verify JSON fix works
   - Verify Agent connection
   - Test end-to-end flow

2. **Short-term** (This week):
   - Implement retry strategy
   - Add rate limiting
   - Configure connection pool

3. **Medium-term** (Next week):
   - Add observability
   - Performance optimization
   - Load testing

---

## 7. Risk Assessment

### High Risk
- **Database JSON errors**: Fixed ✅
- **Core pod stability**: Fixed ✅
- **Agent connection**: Pending verification

### Medium Risk
- **Message loss**: Cần dead letter queue
- **Performance degradation**: Cần rate limiting
- **Connection leaks**: Cần connection pool config

### Low Risk
- **Observability gaps**: Có thể add sau
- **Caching**: Nice to have

---

**Status**: Đang thực hiện Phase 1 - Stability Fixes


