# Database và API Query Commands - Complete Reference

## Quick Start

```bash
# Set variables
export NAMESPACE="fortuna"
export TEST_POD_NAME="test-pod-e2e-1766897554"
export POD_UID="9caa6290-5471-46de-9a0b-a43ba7937d9d"

# Get pod names
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
CORE_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')
```

---

## Database Queries

### 1. Check Pod
```bash
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT uid, name, namespace, created_at FROM pods WHERE uid = '$POD_UID' OR name = '$TEST_POD_NAME' LIMIT 1;"
```

### 2. Query SBOM
```bash
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT id, pod_uid, pod_name, namespace, image_name, image_digest, created_at 
   FROM sboms WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' ORDER BY created_at DESC LIMIT 5;"
```

### 3. Get SBOM ID
```bash
SBOM_ID=$(kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c \
  "SELECT id FROM sboms WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' ORDER BY created_at DESC LIMIT 1;" | tr -d ' ')
echo "SBOM ID: $SBOM_ID"
```

### 4. Query Components
```bash
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) as total FROM sbom_components WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;"
```

### 5. Query CVE Matches
```bash
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) as total FROM cve_matches WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;"
```

### 6. Query Insights
```bash
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) as total FROM insights WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL;"
```

---

## API Queries

### 1. Setup Port-Forward
```bash
kubectl port-forward -n fortuna $CORE_POD 8080:8080 > /tmp/port-forward.log 2>&1 &
sleep 3
```

### 2. Query API
```bash
curl -s "http://localhost:8080/api/v1/insights?resource_uid=$POD_UID" | python3 -m json.tool
```

### 3. Cleanup
```bash
pkill -f "kubectl port-forward"
```

---

## All-in-One Script

Xem file: `KSAM/tests/e2e/scripts/verify-e2e-flow.sh`

