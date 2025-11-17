# ServiceAccount Operations Test Script

Script để test các thao tác với ServiceAccount trên Kubernetes và kiểm tra xem KSAM có detect và sync các thay đổi không.

## Cách sử dụng

### 1. Tạo ServiceAccount mới

```bash
./scripts/test-sa-operations.sh -a create -n test-sa-1 -s default
```

### 2. Xóa ServiceAccount

```bash
./scripts/test-sa-operations.sh -a delete -n test-sa-1 -s default
```

### 3. Edit ServiceAccount (thêm annotations/labels)

```bash
./scripts/test-sa-operations.sh -a edit -n test-sa-1 -s default
```

### 4. List tất cả ServiceAccounts

```bash
./scripts/test-sa-operations.sh -a list -s default
```

### 5. Tạo nhiều ServiceAccounts để test

```bash
./scripts/test-sa-operations.sh -a create-multiple -s default
```

### 6. Cleanup tất cả test ServiceAccounts

```bash
./scripts/test-sa-operations.sh -a cleanup -s default
```

## Test Scenarios

### Scenario 1: Test Create Operation
1. Tạo ServiceAccount mới:
   ```bash
   ./scripts/test-sa-operations.sh -a create -n test-create-sa -s default
   ```
2. Đợi vài giây (để agent sync)
3. Kiểm tra trên dashboard:
   - Vào http://localhost:3000/serviceaccounts
   - Tìm ServiceAccount `test-create-sa`
   - Kiểm tra audit logs

### Scenario 2: Test Delete Operation
1. Tạo ServiceAccount trước:
   ```bash
   ./scripts/test-sa-operations.sh -a create -n test-delete-sa -s default
   ```
2. Đợi sync
3. Xóa ServiceAccount:
   ```bash
   ./scripts/test-sa-operations.sh -a delete -n test-delete-sa -s default
   ```
4. Kiểm tra trên dashboard:
   - ServiceAccount không còn trong list
   - Audit log có record về delete action

### Scenario 3: Test Edit Operation
1. Tạo ServiceAccount trước:
   ```bash
   ./scripts/test-sa-operations.sh -a create -n test-edit-sa -s default
   ```
2. Đợi sync
3. Edit ServiceAccount:
   ```bash
   ./scripts/test-sa-operations.sh -a edit -n test-edit-sa -s default
   ```
4. Kiểm tra trên dashboard:
   - ServiceAccount có updated timestamp
   - Audit log có record về update action

### Scenario 4: Test Multiple Operations
1. Tạo nhiều ServiceAccounts:
   ```bash
   ./scripts/test-sa-operations.sh -a create-multiple -s default
   ```
2. Kiểm tra trên dashboard:
   - Tất cả ServiceAccounts xuất hiện trong list
   - Graph view hiển thị các nodes mới
3. Cleanup:
   ```bash
   ./scripts/test-sa-operations.sh -a cleanup -s default
   ```

## Kiểm tra Sync Status

### 1. Kiểm tra Agent Logs
```bash
kubectl logs -n ksam -l app=ksam-agent --tail=50
```

### 2. Kiểm tra Core Logs
```bash
kubectl logs -n ksam -l app=ksam-core --tail=50
```

### 3. Kiểm tra Database
```bash
# Port forward postgres
kubectl port-forward -n ksam svc/postgres 5432:5432

# Connect và query
psql -h localhost -U ksam -d ksam -c "SELECT name, namespace, cluster_id, created_at FROM service_accounts ORDER BY created_at DESC LIMIT 10;"
```

## Troubleshooting

### ServiceAccount không xuất hiện trên dashboard
1. Kiểm tra agent có đang chạy:
   ```bash
   kubectl get pods -n ksam -l app=ksam-agent
   ```
2. Kiểm tra agent logs xem có lỗi không
3. Kiểm tra cluster ID có đúng không
4. Đợi thêm vài giây (sync interval thường là 30s)

### Audit logs không hiển thị
1. Kiểm tra core service có đang chạy
2. Kiểm tra database connection
3. Kiểm tra core logs

### Sync không hoạt động
1. Kiểm tra agent config:
   ```bash
   kubectl get configmap -n ksam
   ```
2. Kiểm tra agent có kết nối được với core không
3. Kiểm tra network policies

