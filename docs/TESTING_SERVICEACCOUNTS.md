# Testing ServiceAccount Operations

Hướng dẫn test các thao tác với ServiceAccount trên Kubernetes và kiểm tra xem KSAM có detect và sync các thay đổi không.

## Scripts Available

### 1. `test-sa-operations.sh` - Main Test Script

Script chính để thực hiện các thao tác với ServiceAccount.

**Các actions:**
- `create` - Tạo ServiceAccount mới
- `delete` - Xóa ServiceAccount
- `edit` - Edit ServiceAccount (thêm annotations/labels)
- `list` - List tất cả ServiceAccounts
- `create-multiple` - Tạo nhiều ServiceAccounts để test
- `cleanup` - Xóa tất cả test ServiceAccounts

### 2. `demo-sa-test.sh` - Demo Script

Script demo tự động để test toàn bộ flow từ create → edit → verify.

## Quick Start

### Test Create Operation

```bash
# Tạo ServiceAccount mới
./scripts/test-sa-operations.sh -a create -n test-sa-1 -s default

# Đợi vài giây (30s) để agent sync
# Sau đó kiểm tra trên dashboard: http://localhost:3000/serviceaccounts
```

### Test Delete Operation

```bash
# Xóa ServiceAccount
./scripts/test-sa-operations.sh -a delete -n test-sa-1 -s default

# Kiểm tra trên dashboard - ServiceAccount không còn trong list
```

### Test Edit Operation

```bash
# Edit ServiceAccount (thêm annotations/labels)
./scripts/test-sa-operations.sh -a edit -n test-sa-1 -s default

# Kiểm tra trên dashboard - ServiceAccount có updated timestamp
```

### Run Full Demo

```bash
# Chạy demo tự động
./scripts/demo-sa-test.sh
```

## Test Scenarios

### Scenario 1: Basic CRUD Operations

1. **Create**
   ```bash
   ./scripts/test-sa-operations.sh -a create -n scenario1-sa -s default
   ```
   - Verify: ServiceAccount xuất hiện trên dashboard
   - Verify: Audit log có record "create"

2. **Read/List**
   ```bash
   ./scripts/test-sa-operations.sh -a list -s default
   ```
   - Verify: List hiển thị đúng

3. **Update/Edit**
   ```bash
   ./scripts/test-sa-operations.sh -a edit -n scenario1-sa -s default
   ```
   - Verify: Annotations/labels được update
   - Verify: Audit log có record "update"

4. **Delete**
   ```bash
   ./scripts/test-sa-operations.sh -a delete -n scenario1-sa -s default
   ```
   - Verify: ServiceAccount biến mất khỏi dashboard
   - Verify: Audit log có record "delete"

### Scenario 2: Bulk Operations

1. **Create Multiple**
   ```bash
   ./scripts/test-sa-operations.sh -a create-multiple -s default
   ```
   - Verify: Tất cả 5 ServiceAccounts xuất hiện trên dashboard
   - Verify: Graph view hiển thị các nodes mới

2. **Cleanup**
   ```bash
   ./scripts/test-sa-operations.sh -a cleanup -s default
   ```
   - Verify: Tất cả test ServiceAccounts bị xóa

### Scenario 3: Cross-Namespace Testing

1. **Create in different namespaces**
   ```bash
   ./scripts/test-sa-operations.sh -a create -n test-sa -s kube-system
   ./scripts/test-sa-operations.sh -a create -n test-sa -s default
   ./scripts/test-sa-operations.sh -a create -n test-sa -s kube-public
   ```
   - Verify: Tất cả ServiceAccounts xuất hiện trên dashboard với đúng namespace

## Verification Steps

### 1. Check Dashboard UI

1. Open http://localhost:3000/serviceaccounts
2. Filter by namespace/cluster
3. Search for the test ServiceAccount
4. Verify details are correct

### 2. Check Audit Logs

1. Open http://localhost:3000/audit
2. Filter by resource = "serviceaccount"
3. Filter by action (create/update/delete)
4. Verify timestamps match

### 3. Check Graph View

1. Open http://localhost:3000/graph
2. Filter by namespace
3. Verify ServiceAccount nodes appear
4. Check connections to Roles/RoleBindings

### 4. Check Agent Logs

```bash
# View recent agent logs
kubectl logs -n ksam -l app=ksam-agent --tail=50 | grep -i serviceaccount

# Follow logs in real-time
kubectl logs -n ksam -l app=ksam-agent -f | grep -i serviceaccount
```

### 5. Check Database

```bash
# Port forward postgres
kubectl port-forward -n ksam svc/postgres 5432:5432

# Query ServiceAccounts
psql -h localhost -U ksam -d ksam -c \
  "SELECT name, namespace, cluster_id, created_at, updated_at 
   FROM service_accounts 
   WHERE name LIKE 'test%' 
   ORDER BY created_at DESC 
   LIMIT 10;"

# Query Audit Logs
psql -h localhost -U ksam -d ksam -c \
  "SELECT action, resource, resource_id, user, created_at 
   FROM audit_logs 
   WHERE resource = 'serviceaccount' 
   ORDER BY created_at DESC 
   LIMIT 10;"
```

## Expected Behavior

### Sync Timing

- **Agent sync interval**: 30 seconds (default)
- **Expected delay**: 30-60 seconds after operation
- **Real-time updates**: Agent uses informers for real-time detection

### What Gets Synced

- ✅ ServiceAccount name
- ✅ Namespace
- ✅ Cluster ID
- ✅ UID
- ✅ Creation timestamp
- ✅ Update timestamp
- ✅ Annotations (metadata)
- ✅ Labels (metadata)

### What Creates Audit Logs

- ✅ Create ServiceAccount → `action: create`
- ✅ Update ServiceAccount → `action: update`
- ✅ Delete ServiceAccount → `action: delete`

## Troubleshooting

### ServiceAccount không xuất hiện

1. **Check agent status**
   ```bash
   kubectl get pods -n ksam -l app=ksam-agent
   kubectl logs -n ksam -l app=ksam-agent --tail=100
   ```

2. **Check cluster ID**
   ```bash
   kubectl get configmap -n ksam -o yaml | grep CLUSTER_ID
   ```

3. **Check network connectivity**
   ```bash
   kubectl exec -n ksam -l app=ksam-agent -- curl http://ksam-core:8080/health
   ```

4. **Wait longer**
   - Sync interval là 30s, có thể cần đợi thêm

### Audit logs không hiển thị

1. **Check core service**
   ```bash
   kubectl get pods -n ksam -l app=ksam-core
   kubectl logs -n ksam -l app=ksam-core --tail=100
   ```

2. **Check database**
   ```bash
   kubectl exec -n ksam -it postgres-xxx -- psql -U ksam -d ksam -c \
     "SELECT COUNT(*) FROM audit_logs WHERE resource = 'serviceaccount';"
   ```

### Sync không hoạt động

1. **Check agent config**
   ```bash
   kubectl get daemonset -n ksam ksam-agent -o yaml | grep -A 10 env
   ```

2. **Check RBAC permissions**
   ```bash
   kubectl get clusterrolebinding | grep ksam-agent
   kubectl get rolebinding -n ksam | grep ksam-agent
   ```

3. **Restart agent**
   ```bash
   kubectl rollout restart daemonset/ksam-agent -n ksam
   ```

## Best Practices

1. **Always wait 30-60 seconds** after operations before checking dashboard
2. **Use unique names** for test ServiceAccounts to avoid conflicts
3. **Clean up** test resources after testing
4. **Check logs** if something doesn't work as expected
5. **Test in different namespaces** to verify cross-namespace functionality

## Example Test Session

```bash
# 1. Create test ServiceAccount
./scripts/test-sa-operations.sh -a create -n my-test-sa -s default

# 2. Wait 30 seconds
sleep 30

# 3. Check dashboard
open http://localhost:3000/serviceaccounts

# 4. Edit ServiceAccount
./scripts/test-sa-operations.sh -a edit -n my-test-sa -s default

# 5. Wait 30 seconds
sleep 30

# 6. Check audit logs
open http://localhost:3000/audit

# 7. Delete ServiceAccount
./scripts/test-sa-operations.sh -a delete -n my-test-sa -s default

# 8. Wait 30 seconds
sleep 30

# 9. Verify deletion on dashboard
open http://localhost:3000/serviceaccounts
```

