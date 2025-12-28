# Database và API Query Commands - Reference Guide

Tài liệu này cung cấp tất cả các câu lệnh để query database và API để verify logic xử lý.

---

## Cấu hình

```bash
export NAMESPACE="fortuna"
export TEST_POD_NAME="test-pod-e2e-1766897554"
export POD_UID="9caa6290-5471-46de-9a0b-a43ba7937d9d"

# Get pod names
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
CORE_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')
```

---

## Database Queries

### 1. Kiểm tra Pod trong Database

```bash
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT uid, name, namespace, created_at FROM pods WHERE uid = '$POD_UID' OR name = '$TEST_POD_NAME' LIMIT 1;"
```

### 2. Query SBOM

```bash
# Query SBOM cho test pod
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT id, pod_uid, pod_name, namespace, image_name, image_digest, created_at 
   FROM sboms 
   WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' 
   ORDER BY created_at DESC LIMIT 5;"

# Lấy SBOM ID
SBOM_ID=$(kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c \
  "SELECT id FROM sboms WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' ORDER BY created_at DESC LIMIT 1;" | tr -d ' ')
```

### 3. Query SBOM Components

```bash
# Tổng số components
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) as total_components, COUNT(DISTINCT name) as unique_packages 
   FROM sbom_components 
   WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;"

# Sample components (10 đầu tiên)
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT name, version, package_type 
   FROM sbom_components 
   WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL 
   LIMIT 10;"
```

### 4. Query CVE Matches

```bash
# Tổng số CVE matches
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) as total_matches, COUNT(DISTINCT cve_id) as unique_cves 
   FROM cve_matches 
   WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;"

# Sample CVE matches (10 đầu tiên)
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT cve_id, package_name, package_version, cvss_score, matched_by 
   FROM cve_matches 
   WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL 
   LIMIT 10;"
```

### 5. Query Insights

```bash
# Tổng số insights
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) as total_insights, COUNT(DISTINCT cve_id) as unique_cves, 
          string_agg(DISTINCT insight_type, ', ') as types 
   FROM insights 
   WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL;"

# Sample insights (10 đầu tiên)
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT id, resource_uid, resource_type, resource_name, insight_type, severity, cvss, created_at 
   FROM insights 
   WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL 
   ORDER BY created_at DESC 
   LIMIT 10;"
```

### 6. Processing Timeline

```bash
# SBOM creation time
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT created_at as sbom_created FROM sboms WHERE id = $SBOM_ID;"

# First CVE match time
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT MIN(created_at) as first_cve_match FROM cve_matches WHERE sbom_id = $SBOM_ID;"

# First insight time
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT MIN(created_at) as first_insight FROM insights WHERE resource_uid = '$POD_UID';"
```

---

## API Queries

### 1. Setup Port-Forward

```bash
# Start port-forward
kubectl port-forward -n fortuna $CORE_POD 8080:8080 > /tmp/port-forward.log 2>&1 &
PORT_FORWARD_PID=$!
sleep 3

# Stop port-forward (khi xong)
kill $PORT_FORWARD_PID 2>/dev/null || true
```

### 2. Health Check

```bash
curl -s http://localhost:8080/health | python3 -m json.tool
```

### 3. Get Insights by Resource UID

```bash
curl -s "http://localhost:8080/api/v1/insights?resource_uid=$POD_UID" | python3 -m json.tool
```

### 4. Get Insights with Filters

```bash
# By severity
curl -s "http://localhost:8080/api/v1/insights?resource_uid=$POD_UID&severity=high" | python3 -m json.tool

# By insight type
curl -s "http://localhost:8080/api/v1/insights?resource_uid=$POD_UID&insight_type=vulnerability" | python3 -m json.tool

# Pagination
curl -s "http://localhost:8080/api/v1/insights?resource_uid=$POD_UID&page=1&pageSize=10" | python3 -m json.tool
```

### 5. Get All Insights

```bash
curl -s "http://localhost:8080/api/v1/insights?page=1&pageSize=50" | python3 -m json.tool
```

---

## Complete Verification Script

### Chạy tất cả queries

```bash
cd KSAM/tests/e2e/scripts

# Set variables
export TEST_POD_NAME="test-pod-e2e-1766897554"
export POD_UID="9caa6290-5471-46de-9a0b-a43ba7937d9d"

# Run complete verification
./verify-e2e-flow.sh
```

---

## Quick Reference

### Database - One-liners

```bash
# SBOM count
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT COUNT(*) FROM sboms WHERE pod_uid = '$POD_UID';"

# Components count
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT COUNT(*) FROM sbom_components WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;"

# CVE matches count
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT COUNT(*) FROM cve_matches WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;"

# Insights count
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT COUNT(*) FROM insights WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL;"
```

### API - One-liners

```bash
# Quick API check
kubectl port-forward -n fortuna $CORE_POD 8080:8080 > /dev/null 2>&1 & sleep 2 && \
curl -s "http://localhost:8080/api/v1/insights?resource_uid=$POD_UID" | grep -o '"total":[0-9]*' && \
pkill -f "kubectl port-forward"
```

---

## Expected Results

### Khi SBOM đang processing
- Pod: Có thể chưa có trong database
- SBOM: Chưa có
- Components: N/A
- CVE Matches: N/A
- Insights: Chưa có
- API: Trả về `{"insights":[],"total":0}`

### Khi SBOM đã complete
- Pod: Có trong database
- SBOM: Có với components
- Components: Count > 0
- CVE Matches: Có thể 0 hoặc > 0 (tùy vào CVEs)
- Insights: Được tạo nếu có CVEs
- API: Trả về insights array

---

**Last Updated**: $(date)

