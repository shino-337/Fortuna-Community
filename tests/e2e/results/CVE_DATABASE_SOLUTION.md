# CVE Database Population Solution

**Date**: $(date)

---

## Problem

CVE database is empty, causing:
- No CVE matches during SBOM processing
- No insights generated
- API returns empty results

---

## Root Cause

✅ **Database Tables**: Exist and correctly structured
❌ **Database Data**: Empty (0 CVEs, 0 package_vulnerabilities)
❌ **CVE Loader**: Not running (code exists but not deployed)

---

## Solution: Populate CVE Database

### Option 1: Run CVE Loader Locally (Quick Test)

```bash
cd KSAM/core

# Set database connection
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/ksam?sslmode=disable"

# Run CVE loader
go run cmd/cve-loader/main.go \
  --source ../cve-data/all \
  --workers 20 \
  --batch-size 100
```

### Option 2: Run CVE Loader in Core Pod

```bash
# Get Core pod
CORE_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')

# Copy CVE data to pod (if needed)
kubectl cp KSAM/cve-data/all fortuna/$CORE_POD:/cve-data/all

# Run CVE loader in pod
kubectl exec -n fortuna $CORE_POD -- /app/cve-loader \
  --source /cve-data/all \
  --workers 20 \
  --batch-size 100
```

### Option 3: Create Kubernetes Job (Recommended)

Create a Job manifest:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: cve-loader
  namespace: fortuna
spec:
  template:
    spec:
      containers:
      - name: cve-loader
        image: fortuna-core:latest
        command: ["/app/cve-loader"]
        args: [
          "--source", "/cve-data/all",
          "--workers", "20",
          "--batch-size", "100"
        ]
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: fortuna-database
              key: url
        volumeMounts:
        - name: cve-data
          mountPath: /cve-data
      volumes:
      - name: cve-data
        hostPath:
          path: /path/to/cve-data/all
          type: Directory
      restartPolicy: Never
  backoffLimit: 3
```

Then apply:
```bash
kubectl apply -f cve-loader-job.yaml
```

---

## Verification

After running CVE loader:

```bash
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# Check CVE count
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) FROM cves;"

# Check package_vulnerabilities count
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) FROM package_vulnerabilities;"

# Sample data
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT cve_id, package_name, ecosystem FROM package_vulnerabilities LIMIT 10;"
```

---

## Expected Results

After population:
- CVEs: Thousands of records
- package_vulnerabilities: Hundreds of thousands of records
- CVE matching will find matches
- Insights will be generated
- API will return results

---

## Notes

1. **CVE Data Source**: OSV.dev JSON files in `KSAM/cve-data/all/`
2. **Processing Time**: Depends on number of files (may take 10-30 minutes)
3. **Database Size**: Will increase significantly after loading
4. **Regular Updates**: Consider setting up CronJob for periodic updates

---

**Report Generated**: $(date)

