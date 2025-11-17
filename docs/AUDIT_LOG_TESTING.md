# Audit Log Testing Guide

## Tổng quan

Tài liệu này mô tả cách test tính năng audit log tự động sync khi tạo/xóa ServiceAccount trên Kubernetes.

## Test Case

### Script Test Tự Động

Sử dụng script `scripts/test-audit-sync.sh` để test tự động:

```bash
./scripts/test-audit-sync.sh
```

Script này sẽ:
1. Tạo ServiceAccount mới
2. Đợi agent sync (90 giây)
3. Kiểm tra audit log CREATE
4. Xóa ServiceAccount
5. Đợi agent sync và full sync (90 giây)
6. Kiểm tra audit log DELETE
7. Hiển thị kết quả test

### Test Thủ Công

#### 1. Tạo ServiceAccount

```bash
./scripts/test-sa-operations.sh -a create -n test-sa-1 -s default
```

#### 2. Kiểm tra Audit Log CREATE

```bash
kubectl exec -n ksam $(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d ksam -c \
  "SELECT id, action, resource, \"user\", ip, details::text, created_at FROM audit_logs WHERE details::text LIKE '%test-sa-1%' AND action = 'create' ORDER BY created_at DESC;"
```

#### 3. Xóa ServiceAccount

```bash
./scripts/test-sa-operations.sh -a delete -n test-sa-1 -s default
```

#### 4. Kiểm tra Audit Log DELETE

```bash
# Đợi 90 giây cho full sync
sleep 90

kubectl exec -n ksam $(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d ksam -c \
  "SELECT id, action, resource, \"user\", ip, details::text, created_at FROM audit_logs WHERE details::text LIKE '%test-sa-1%' AND action = 'delete' ORDER BY created_at DESC;"
```

## Kết Quả Mong Đợi

### CREATE Audit Log
- **Action**: `create`
- **User**: `system`
- **IP**: `agent-sync`
- **Details**: `{"source":"agent-sync","namespace":"default","name":"test-sa-1"}`

### DELETE Audit Log
- **Action**: `delete`
- **User**: `system`
- **IP**: `agent-sync`
- **Details**: `{"source":"agent-sync","namespace":"default","name":"test-sa-1"}`

## Timing

- **Agent sync**: Mỗi khi có event (create/update/delete) - ngay lập tức
- **Full sync**: Mỗi 30 giây (collector chạy định kỳ)
- **Audit log CREATE**: Được tạo khi agent sync detect ServiceAccount mới
- **Audit log DELETE**: Được tạo khi full sync phát hiện ServiceAccount không còn trong K8s

## Troubleshooting

### Audit log không được tạo

1. **Kiểm tra agent logs**:
   ```bash
   kubectl logs -n kube-system -l app=ksam-agent --tail=50 | grep -i "test-sa-1"
   ```

2. **Kiểm tra core logs**:
   ```bash
   kubectl logs -n ksam -l app=ksam-core --tail=50 | grep -i "test-sa-1\|audit\|Full sync"
   ```

3. **Kiểm tra ServiceAccount trong DB**:
   ```bash
   kubectl exec -n ksam $(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
     psql -U postgres -d ksam -c \
     "SELECT id, name, namespace, uid, deleted_at FROM service_accounts WHERE name = 'test-sa-1';"
   ```

4. **Kiểm tra tất cả audit logs**:
   ```bash
   kubectl exec -n ksam $(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
     psql -U postgres -d ksam -c \
     "SELECT id, action, resource, \"user\", ip, created_at FROM audit_logs ORDER BY created_at DESC LIMIT 10;"
   ```

## Notes

- Audit logs được tạo bởi agent sync (user="system", ip="agent-sync")
- CREATE audit log được tạo khi ServiceAccount mới được sync vào DB
- DELETE audit log được tạo khi full sync phát hiện ServiceAccount không còn trong K8s
- Có thể có delay giữa khi xóa ServiceAccount và khi audit log được tạo (đợi full sync)

