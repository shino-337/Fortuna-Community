# Performance Verification Checklist

Checklist for verifying performance targets and optimizations.

## Performance Targets

| Metric | Target | Measurement |
|--------|--------|-------------|
| SBOM Processing | < 30s | Time from pod creation to SBOM storage |
| CVE Matching (200 packages) | < 5s | Time from SBOM storage to CVE match storage |
| Insight Generation (100 CVEs) | < 2s | Time from CVE match to insight storage |
| Total E2E Time | < 60s | Time from pod creation to API availability |
| API Response Time | < 100ms | Time for API to return insights |
| Database Query Time | < 200ms | Time for complex queries |

## Database Performance

### Query Performance
- [ ] SBOM queries use indexes (EXPLAIN shows index usage)
- [ ] CVE match queries use indexes
- [ ] Insight queries use indexes
- [ ] No full table scans in slow queries
- [ ] Query times meet targets

### Connection Pool
- [ ] Connection pool metrics are exported
- [ ] Pool size is appropriate (not exhausted)
- [ ] Connection wait time is minimal
- [ ] Idle connections are managed correctly

### Batch Operations
- [ ] Batch UPSERT is used for insights
- [ ] Bulk queries are used (no N+1)
- [ ] Transaction batching is implemented
- [ ] Batch sizes are optimal

## Processing Performance

### SBOM Processing
- [ ] Extraction time is acceptable
- [ ] Component count is accurate
- [ ] PURL generation is efficient
- [ ] Storage is optimized

### CVE Matching
- [ ] Bulk CVE lookup is used
- [ ] Ecosystem grouping is efficient
- [ ] Version range matching is fast
- [ ] No N+1 queries

### Insight Generation
- [ ] Batch UPSERT is used
- [ ] Deduplication is efficient
- [ ] Risk scoring is fast
- [ ] No sequential processing

## NATS Performance

### Message Processing
- [ ] Messages are processed promptly
- [ ] No message backlog
- [ ] Acknowledgment is timely
- [ ] Retry logic works correctly

### Stream Configuration
- [ ] Retention policies are appropriate
- [ ] Stream limits are set correctly
- [ ] WorkQueuePolicy is used where needed
- [ ] Message limits prevent overflow

## API Performance

### Response Time
- [ ] API responses are fast (< 100ms)
- [ ] Pagination works efficiently
- [ ] Filtering is optimized
- [ ] No N+1 queries in API handlers

### Caching
- [ ] Caching is used where appropriate
- [ ] Cache invalidation works correctly
- [ ] Cache hit rates are acceptable

## Monitoring

### Metrics
- [ ] Prometheus metrics are exported
- [ ] Metrics are accurate
- [ ] Performance metrics are tracked
- [ ] Alerts are configured (if applicable)

### Logging
- [ ] Performance logs are generated
- [ ] Slow queries are logged
- [ ] Timing information is captured
- [ ] Logs are structured and searchable

## Performance Test Results

### SBOM Processing
- [ ] Average time: `___ seconds`
- [ ] Min time: `___ seconds`
- [ ] Max time: `___ seconds`
- [ ] Meets target: ✅ / ❌

### CVE Matching
- [ ] Average time: `___ seconds`
- [ ] Min time: `___ seconds`
- [ ] Max time: `___ seconds`
- [ ] Meets target: ✅ / ❌

### Insight Generation
- [ ] Average time: `___ seconds`
- [ ] Min time: `___ seconds`
- [ ] Max time: `___ seconds`
- [ ] Meets target: ✅ / ❌

### Total E2E
- [ ] Average time: `___ seconds`
- [ ] Min time: `___ seconds`
- [ ] Max time: `___ seconds`
- [ ] Meets target: ✅ / ❌

## Optimization Verification

### N+1 Query Elimination
- [ ] No N+1 queries detected
- [ ] Bulk queries are used
- [ ] Query counts are verified
- [ ] Performance improvement confirmed

### Batch Operations
- [ ] Batch UPSERT is implemented
- [ ] Batch sizes are optimal
- [ ] Performance improvement confirmed
- [ ] No data loss in batches

### Index Usage
- [ ] All critical indexes exist
- [ ] Indexes are used in queries
- [ ] Query performance improved
- [ ] Index maintenance is acceptable

## Recommendations

- [ ] List any performance improvements needed
- [ ] Document any bottlenecks found
- [ ] Suggest optimizations
- [ ] Track performance over time

