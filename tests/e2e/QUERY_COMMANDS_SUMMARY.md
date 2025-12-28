# Database và API Query Commands - Tóm tắt

## Các Scripts Đã Tạo

1. **`database-queries.sh`** - Query database chi tiết
2. **`api-queries.sh`** - Query API chi tiết  
3. **`verify-e2e-flow.sh`** - Verification script đầy đủ
4. **`QUICK_VERIFY.sh`** - Quick check script

## Cách Sử Dụng

### Quick Verification
```bash
cd KSAM/tests/e2e/scripts
export TEST_POD_NAME="test-pod-e2e-1766897554"
export POD_UID="9caa6290-5471-46de-9a0b-a43ba7937d9d"
./QUICK_VERIFY.sh
```

### Database Queries
```bash
cd KSAM/tests/e2e/scripts
export TEST_POD_NAME="test-pod-e2e-1766897554"
export POD_UID="9caa6290-5471-46de-9a0b-a43ba7937d9d"
./database-queries.sh
```

### API Queries
```bash
cd KSAM/tests/e2e/scripts
export POD_UID="9caa6290-5471-46de-9a0b-a43ba7937d9d"
./api-queries.sh
```

### Complete Verification
```bash
cd KSAM/tests/e2e/scripts
export TEST_POD_NAME="test-pod-e2e-1766897554"
export POD_UID="9caa6290-5471-46de-9a0b-a43ba7937d9d"
./verify-e2e-flow.sh
```

## Manual Commands

### Database

```bash
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# SBOM
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT id, pod_uid, pod_name, image_name, created_at FROM sboms WHERE pod_uid = '$POD_UID' ORDER BY created_at DESC LIMIT 5;"

# Components
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) FROM sbom_components WHERE sbom_id = <SBOM_ID> AND deleted_at IS NULL;"

# CVE Matches
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) FROM cve_matches WHERE sbom_id = <SBOM_ID> AND deleted_at IS NULL;"

# Insights
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) FROM insights WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL;"
```

### API

```bash
CORE_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')

kubectl port-forward -n fortuna $CORE_POD 8080:8080 &

curl -s "http://localhost:8080/api/v1/insights?resource_uid=$POD_UID" | python3 -m json.tool
```

