# Database và API Query Commands

Tài liệu này cung cấp các câu lệnh để query database và API để verify logic xử lý.

---

## Cấu hình

```bash
export NAMESPACE="fortuna"
export TEST_POD_NAME="test-pod-e2e-1766897554"
export POD_UID="9caa6290-5471-46de-9a0b-a43ba7937d9d"
```

---

## Database Queries

### 1. Kiểm tra Pod trong Database

```bash
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')

kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT uid, name, namespace, created_at FROM pods WHERE uid = '$POD_UID' OR name = '$TEST_POD_NAME' LIMIT 1;"
```

### 2. Query SBOM

```bash
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT id, pod_uid, pod_name, namespace, image_name, image_digest, created_at 
   FROM sboms 
   WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' 
   ORDER BY created_at DESC LIMIT 5;"
```

### 3. Query SBOM Components

```bash
# Lấy SBOM ID trước
SBOM_ID=$(kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c \
  "SELECT id FROM sboms WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' ORDER BY created_at DESC LIMIT 1;" | tr -d ' ')

# Query components
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) as total_components, COUNT(DISTINCT name) as unique_packages 
   FROM sbom_components 
   WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;"

# Sample components
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT name, version, package_type 
   FROM sbom_components 
   WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL 
   LIMIT 10;"
```

### 4. Query CVE Matches

```bash
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) as total_matches, COUNT(DISTINCT cve_id) as unique_cves 
   FROM cve_matches 
   WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;"

# Sample CVE matches
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT cve_id, package_name, package_version, cvss_score, matched_by 
   FROM cve_matches 
   WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL 
   LIMIT 10;"
```

### 5. Query Insights

```bash
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) as total_insights, COUNT(DISTINCT cve_id) as unique_cves, 
          string_agg(DISTINCT insight_type, ', ') as types 
   FROM insights 
   WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL;"

# Sample insights
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
CORE_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')

kubectl port-forward -n fortuna $CORE_POD 8080:8080 &
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

## Quick Verification Scripts

### Run All Database Queries

```bash
cd KSAM/tests/e2e/scripts
./database-queries.sh
```

### Run All API Queries

```bash
cd KSAM/tests/e2e/scripts
./api-queries.sh
```

### Run Complete E2E Verification

```bash
cd KSAM/tests/e2e/scripts
./verify-e2e-flow.sh
```

---

## Expected Results

### When SBOM is Processing
- Pod: Found in database (may take time to sync)
- SBOM: Not found yet
- Components: N/A
- CVE Matches: N/A
- Insights: Not found yet
- API: Returns empty array

### When SBOM is Complete
- Pod: Found
- SBOM: Found with components
- Components: Count > 0
- CVE Matches: May be 0 if no CVEs found, or > 0 if CVEs exist
- Insights: Created if CVEs found
- API: Returns insights array

---

**Last Updated**: $(date)

