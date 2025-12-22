# CVE Database Status

**Last Updated**: 2024-12-20

---

## Current Status

### CVE Data Files

- **Available in `cve-data/all/`**: ~74,561 CVEs (OSV.dev format)
- **Available in `cve-data/e2e/`**: 1 CVE (CVE-2014-0011 for testing)

### Database Status

- **CVEs Loaded**: 1 (CVE-2014-0011)
- **Package Vulnerabilities**: 1 entry
- **Loading Status**: Only E2E test CVE loaded (0.001% of available)

---

## Detection Statistics

- **Unique CVEs Detected**: 1
- **Total CVE Matches**: 1
- **Vulnerability Insights Created**: 3

---

## Currently Loaded CVE

| CVE ID | Severity | CVSS Score | Published Date | Ecosystem |
|--------|----------|------------|----------------|-----------|
| CVE-2014-0011 | CRITICAL | 10.0 | 2020-01-02 | debian |

---

## Loading All CVEs

To load all 74,561 CVEs from `cve-data/all/`:

### Option 1: Kubernetes Job

```bash
# Create ConfigMap with CVE data
kubectl create configmap ksam-cve-data-all \
  --from-file=KSAM/cve-data/all \
  -n ksam

# Create Job to load all CVEs
kubectl create job -n ksam cve-full-loader \
  --image=ksam/cve-loader:latest \
  --from=configmap/ksam-cve-data-all \
  -- /cve-loader
```

### Option 2: Direct Mount

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: cve-full-loader
  namespace: ksam
spec:
  template:
    spec:
      containers:
      - name: cve-loader
        image: ksam/cve-loader:latest
        command: ["/cve-loader"]
        env:
        - name: DATABASE_URL
          value: "postgres://postgres:postgres@postgres.ksam.svc.cluster.local:5432/ksam?sslmode=disable"
        volumeMounts:
        - name: cve-data
          mountPath: /cve
      volumes:
      - name: cve-data
        hostPath:
          path: /path/to/KSAM/cve-data/all
          type: Directory
      restartPolicy: OnFailure
```

### Expected Time

- **Loading 74,561 CVEs**: ~30-60 minutes
- **Package vulnerabilities**: ~100,000+ entries (many CVEs affect multiple packages)

---

## Verification

After loading, verify:

```bash
# Check CVE count
kubectl exec -n ksam postgres-xxx -- psql -U postgres -d ksam \
  -c "SELECT COUNT(*) FROM cves WHERE deleted_at IS NULL;"
# Expected: ~74,561

# Check package vulnerabilities
kubectl exec -n ksam postgres-xxx -- psql -U postgres -d ksam \
  -c "SELECT COUNT(*) FROM package_vulnerabilities WHERE deleted_at IS NULL;"
# Expected: ~100,000+
```

---

## Current Configuration

The system is currently configured for **E2E testing** with minimal CVE data:
- Only CVE-2014-0011 loaded
- Used for testing the complete pipeline
- Sufficient for development and testing

For **production**, load all CVEs from `cve-data/all/` to enable detection of all known vulnerabilities.

---

**Note**: The CVE loader processes OSV.dev JSON files and extracts:
- CVE metadata (ID, severity, CVSS score, published date)
- Package vulnerabilities (ecosystem, package name, affected versions)
- References and CWE IDs

