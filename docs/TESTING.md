# Testing Guide

Hướng dẫn test các tính năng của KSAM.

## Test Scenarios

### 1. Discovery Test

**Mục tiêu**: Verify Agent thu thập dữ liệu từ Kubernetes cluster.

**Steps**:
1. Deploy Agent vào cluster
2. Tạo test ServiceAccounts, RoleBindings
3. Chờ Agent sync (30s)
4. Verify data trong database

**Commands**:
```bash
# Create test data
./scripts/create-test-data.sh

# Check Agent logs
kubectl logs -l app=ksam-agent -n kube-system

# Verify in database
kubectl port-forward -n ksam svc/postgres 5432:5432
psql -h localhost -U postgres -d ksam -c "SELECT COUNT(*) FROM service_accounts;"
```

**Expected Results**:
- Agent logs show collection complete
- Database có ServiceAccounts, RoleBindings
- Data matches với cluster resources

### 2. Real-time Watching Test

**Mục tiêu**: Verify Agent detect real-time changes.

**Steps**:
1. Monitor Agent logs
2. Create new ServiceAccount
3. Verify change detected immediately
4. Delete ServiceAccount
5. Verify deletion detected

**Commands**:
```bash
# Watch Agent logs
kubectl logs -l app=ksam-agent -n kube-system -f

# In another terminal, create SA
kubectl create serviceaccount test-watch -n default

# Should see log: "ServiceAccount added: default/test-watch"

# Delete SA
kubectl delete serviceaccount test-watch -n default

# Should see log: "ServiceAccount deleted: default/test-watch"
```

**Expected Results**:
- Changes detected within seconds
- Logs show add/update/delete events

### 3. API Authentication Test

**Mục tiêu**: Verify authentication và authorization.

**Steps**:
1. Test login endpoint
2. Test protected endpoints với token
3. Test protected endpoints không có token
4. Test admin-only endpoints

**Commands**:
```bash
# Login
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' \
  | jq -r '.token')

# Test protected endpoint với token
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/clusters

# Test protected endpoint không có token (should fail)
curl http://localhost:8080/api/v1/clusters

# Test admin-only endpoint
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[1,2,3]}' \
  http://localhost:8080/api/v1/serviceaccounts/bulk/disable
```

**Expected Results**:
- Login returns token
- Protected endpoints require token
- Admin-only endpoints require admin role

### 4. Graph Visualization Test

**Mục tiêu**: Verify graph data generation và visualization.

**Steps**:
1. Create test data với relationships
2. Get graph data từ API
3. Verify graph structure
4. Test filtering

**Commands**:
```bash
# Get graph data
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/graph | jq .

# Get graph với filter
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/graph?cluster=minikube&namespace=test-ksam" | jq .

# Verify nodes và edges
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/graph | \
  jq '{nodes: .nodes | length, edges: .edges | length}'
```

**Expected Results**:
- Graph có nodes cho ServiceAccounts, Roles, Namespaces
- Edges show relationships
- Filtering works correctly

### 5. Bulk Operations Test

**Mục tiêu**: Verify bulk operations.

**Steps**:
1. Get ServiceAccount IDs
2. Test bulk disable
3. Verify audit logs
4. Test bulk delete

**Commands**:
```bash
# Get SA IDs
SAs=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/serviceaccounts | \
  jq -r '.serviceAccounts[0:3] | map(.id) | @json')

# Bulk disable
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"ids\":$SAs}" \
  http://localhost:8080/api/v1/serviceaccounts/bulk/disable

# Verify audit logs
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit?action=disable" | jq .
```

**Expected Results**:
- Bulk operations succeed
- Audit logs created
- ServiceAccounts disabled/deleted

### 6. Performance Test

**Mục tiêu**: Verify performance với large datasets.

**Steps**:
1. Create many ServiceAccounts
2. Monitor sync time
3. Test API response times
4. Check metrics

**Commands**:
```bash
# Create many SAs
for i in {1..100}; do
  kubectl create serviceaccount perf-test-$i -n test-ksam
done

# Monitor sync
time kubectl logs -l app=ksam-agent -n kube-system --tail=1

# Test API performance
time curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/serviceaccounts

# Check metrics
curl http://localhost:8080/metrics | grep ksam_http_request_duration
```

**Expected Results**:
- Sync completes within 10s for 10k SAs
- API responses < 100ms
- Metrics show good performance

### 7. Error Handling Test

**Mục tiêu**: Verify error handling và retry logic.

**Steps**:
1. Stop Core Controller
2. Verify Agent retries
3. Restart Core Controller
4. Verify reconnection

**Commands**:
```bash
# Stop Core
kubectl scale deployment ksam-core -n ksam --replicas=0

# Watch Agent logs (should see retries)
kubectl logs -l app=ksam-agent -n kube-system -f

# Restart Core
kubectl scale deployment ksam-core -n ksam --replicas=1

# Verify reconnection
kubectl logs -l app=ksam-agent -n kube-system --tail=20
```

**Expected Results**:
- Agent retries với exponential backoff
- Reconnects when Core available
- No data loss

## Automated Testing

### Run All Tests

```bash
# Setup
./scripts/setup-minikube.sh

# Create test data
./scripts/create-test-data.sh

# Test API
./scripts/test-api.sh

# Manual testing
# - Access Dashboard
# - Test graph visualization
# - Test bulk operations
```

## Test Checklist

- [ ] Agent collects data successfully
- [ ] Real-time watching works
- [ ] API authentication works
- [ ] Graph visualization works
- [ ] Bulk operations work
- [ ] Performance is acceptable
- [ ] Error handling works
- [ ] Audit logging works
- [ ] Metrics are collected
- [ ] Health endpoints work

## Performance Benchmarks

### Expected Performance

- **Data Sync**: < 10s cho 10k ServiceAccounts
- **API Response**: < 100ms cho paginated queries
- **Graph Generation**: < 500ms cho 1k nodes
- **Database Queries**: < 50ms với indexes

### Monitoring

```bash
# Watch metrics
watch -n 5 'curl -s http://localhost:8080/metrics | grep ksam_http_request_duration'

# Check resource usage
kubectl top pods -n ksam
kubectl top pods -n kube-system | grep ksam
```

## Troubleshooting Tests

### Test Failures

1. **Check logs**: `kubectl logs -l app=ksam-core -n ksam`
2. **Check database**: Verify data in PostgreSQL
3. **Check network**: Test connectivity between components
4. **Check resources**: Verify pods have enough resources

### Common Issues

1. **Agent không sync**: Check Core endpoint, network connectivity
2. **API errors**: Check authentication, database connection
3. **Graph empty**: Check data collection, verify relationships
4. **Performance slow**: Check database indexes, connection pool

