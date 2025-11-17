# Performance Guide

## Tối ưu hiệu năng đã implement

### 1. Database Connection Pooling

**Location**: `core/internal/storage/storage.go`

```go
sqlDB.SetMaxIdleConns(10)                  // Maximum idle connections
sqlDB.SetMaxOpenConns(100)                 // Maximum open connections
sqlDB.SetConnMaxLifetime(time.Hour)        // Connection max lifetime
sqlDB.SetConnMaxIdleTime(10 * time.Minute) // Idle connection timeout
```

**Lợi ích**:
- Giảm overhead của connection creation
- Tái sử dụng connections
- Tự động cleanup idle connections

### 2. Shared Informer Cache

**Location**: `agent/internal/k8s/informer.go`, `agent/internal/watcher/watcher.go`

**Lợi ích**:
- Reuse informers across multiple watchers
- Giảm API server load
- Better memory efficiency
- Automatic resync management

**Cấu hình**:
- Resync period: 0 (chỉ watch changes, không resync)
- Shared cache cho tất cả informers

### 3. Retry Logic với Exponential Backoff

**Location**: `agent/internal/retry/retry.go`

**Cấu hình mặc định**:
- Max retries: 5
- Initial wait: 1 second
- Max wait: 30 seconds
- Multiplier: 2.0

**Lợi ích**:
- Tự động retry khi network errors
- Exponential backoff giảm load
- Context-aware cancellation

### 4. Pagination

**Location**: `core/internal/api/handlers.go`

**Cấu hình**:
- Default page size: 50
- Configurable via query parameter
- Efficient database queries với LIMIT/OFFSET

**Lợi ích**:
- Giảm memory usage
- Faster response times
- Better user experience

### 5. Redis Caching (Optional)

**Location**: `core/internal/cache/cache.go`

**Lợi ích**:
- Cache frequently accessed data
- Giảm database load
- Faster response times

**Enable**:
```bash
export REDIS_URL=redis://localhost:6379
```

### 6. gRPC Keepalive

**Location**: `agent/internal/client/grpc_client.go`

**Cấu hình**:
- Keepalive time: 10 seconds
- Keepalive timeout: 3 seconds
- Permit without stream: true

**Lợi ích**:
- Maintain long-lived connections
- Detect dead connections early
- Better resource utilization

## Performance Metrics

### Prometheus Metrics

Available tại `/metrics` endpoint:

- `ksam_http_requests_total` - Total HTTP requests
- `ksam_http_request_duration_seconds` - HTTP request duration
- `ksam_database_queries_total` - Total database queries
- `ksam_database_query_duration_seconds` - Database query duration
- `ksam_agent_syncs_total` - Total agent syncs
- `ksam_agent_sync_duration_seconds` - Agent sync duration
- `ksam_serviceaccounts_total` - Total service accounts
- `ksam_clusters_total` - Total clusters
- `ksam_cache_hits_total` - Cache hits
- `ksam_cache_misses_total` - Cache misses

## Best Practices

### 1. Database Queries
- Sử dụng indexes cho frequently queried columns
- Avoid N+1 queries (sử dụng Preload)
- Use pagination cho large datasets
- Connection pooling đã được configure

### 2. Agent Performance
- Shared informer cache giảm API server load
- Retry logic với exponential backoff
- Batch data collection
- Configurable sync interval

### 3. API Performance
- Pagination cho large datasets
- Caching cho frequently accessed data
- Metrics middleware cho monitoring
- Efficient database queries

### 4. Memory Management
- Connection pooling giảm memory usage
- Shared informer cache
- Pagination giảm memory footprint
- Proper cleanup với defer

## Monitoring

### Health Endpoints
- `/health` - Overall health check
- `/ready` - Readiness check (database connectivity)
- `/live` - Liveness check
- `/metrics` - Prometheus metrics

### Grafana Dashboards (TODO)
- HTTP request rate và latency
- Database query performance
- Agent sync status
- Resource usage

## Scaling

### Horizontal Scaling
- Core Controller: Stateless, có thể scale horizontally
- Dashboard: Stateless, có thể scale horizontally
- Agent: DaemonSet, tự động scale với nodes

### Vertical Scaling
- Database: Tăng connection pool size nếu cần
- Core: Tăng resource limits
- Agent: Resource limits đã được optimize

### Async Queue (Optional)
- Chỉ cần khi >50 clusters
- Sử dụng Kafka hoặc NATS
- Giảm load trên Core Controller

## Troubleshooting

### Slow Queries
1. Check database connection pool
2. Review query execution plans
3. Check indexes
4. Enable query logging

### High Memory Usage
1. Check connection pool size
2. Review pagination settings
3. Check informer cache size
4. Monitor resource limits

### Network Issues
1. Check retry logic configuration
2. Review keepalive settings
3. Check network latency
4. Monitor connection errors

