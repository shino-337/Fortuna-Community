# Quick Start Guide - Fortuna in 10 Minutes

Get Fortuna running and see it in action in just 10 minutes!

---

## 📋 What You'll Learn

By the end of this guide, you'll have:
- ✅ Fortuna deployed in your Kubernetes cluster
- ✅ CVE data loaded (sample set)
- ✅ A vulnerable test application deployed
- ✅ Vulnerability insights generated
- ✅ Risk scores calculated

**Time required**: ~10 minutes

---

## 🚀 Step 1: Deploy Fortuna (3 minutes)

```bash
# 1. Create namespace
kubectl create namespace fortuna

# 2. Deploy PostgreSQL
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-config
  namespace: fortuna
data:
  POSTGRES_DB: fortuna
  POSTGRES_USER: postgres
  POSTGRES_PASSWORD: postgres
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  namespace: fortuna
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:15
        envFrom:
        - configMapRef:
            name: postgres-config
        ports:
        - containerPort: 5432
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: fortuna
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
EOF

# 3. Deploy NATS
kubectl apply -f https://raw.githubusercontent.com/nats-io/k8s/master/nats-server/simple-nats.yaml -n fortuna

# 4. Wait for infrastructure
kubectl wait --for=condition=ready pod -l app=postgres -n fortuna --timeout=120s
kubectl wait --for=condition=ready pod -l app=nats -n fortuna --timeout=120s

# 5. Deploy Fortuna Core
kubectl apply -f https://raw.githubusercontent.com/your-org/fortuna/main/deploy/core-deployment.yaml
kubectl apply -f https://raw.githubusercontent.com/your-org/fortuna/main/deploy/core-service.yaml

# 6. Wait for Core to be ready
kubectl wait --for=condition=ready pod -l app=fortuna-core -n fortuna --timeout=180s
```

✅ **Checkpoint**: Verify all pods are running:
```bash
kubectl get pods -n fortuna
```

---

## 📦 Step 2: Load Sample CVE Data (2 minutes)

```bash
# Load a small sample of CVEs for testing
kubectl exec -it $(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -n fortuna -- bash -c "
psql -U postgres -d fortuna <<SQL
-- Insert sample CVEs
INSERT INTO cves (cve_id, severity, description, cvss_score, published_date)
VALUES 
  ('CVE-2021-44228', 'CRITICAL', 'Log4Shell vulnerability', 10.0, '2021-12-10'),
  ('CVE-2022-1292', 'HIGH', 'OpenSSL command injection', 9.8, '2022-05-03'),
  ('CVE-2023-4911', 'HIGH', 'Looney Tunables glibc vulnerability', 7.8, '2023-10-03');

-- Insert package vulnerabilities
INSERT INTO package_vulnerabilities (cve_id, package_name, ecosystem, version_end_excluding)
VALUES
  ('CVE-2021-44228', 'log4j-core', 'maven', '2.15.0'),
  ('CVE-2022-1292', 'openssl', 'debian', '1.1.1n-0+deb11u2'),
  ('CVE-2023-4911', 'glibc', 'debian', '2.31-13+deb11u7');
SQL
"
```

✅ **Checkpoint**: Verify CVEs loaded:
```bash
kubectl exec -it $(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -n fortuna -- \
  psql -U postgres -d fortuna -c "SELECT COUNT(*) FROM cves;"
```

Expected output: `3` CVEs

---

## 🐛 Step 3: Deploy Vulnerable Application (1 minute)

```bash
# Deploy a pod with Log4Shell vulnerability
kubectl run log4shell-demo \
  --image=docker.io/vulfocus/log4j2-rce-2021-12-09:latest \
  --port=8080 \
  -n default

# Wait for pod to be running
kubectl wait --for=condition=ready pod/log4shell-demo -n default --timeout=60s
```

✅ **Checkpoint**: Verify pod is running:
```bash
kubectl get pod log4shell-demo -n default
```

---

## 🔍 Step 4: Trigger SBOM Generation (2 minutes)

Fortuna automatically detects new pods and generates SBOMs. Wait ~60 seconds for processing.

```bash
# Monitor Fortuna Core logs
kubectl logs -f -n fortuna -l app=fortuna-core --tail=50
```

Look for log messages like:
```
[SBOMWorker] Processing pod: default/log4shell-demo
[SBOMExtractor] Extracting SBOM from image...
[CVEMatcher] Matching CVEs against SBOM...
[InsightManager] Created vulnerability insight
```

---

## 📊 Step 5: View Insights (2 minutes)

### Via API

```bash
# Port forward Fortuna API
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &

# Get all insights
curl -s http://localhost:8080/api/v1/insights | jq '.'

# Get vulnerability insights only
curl -s http://localhost:8080/api/v1/insights | \
  jq '.[] | select(.type=="vulnerability")'

# Get critical insights
curl -s http://localhost:8080/api/v1/insights | \
  jq '.[] | select(.severity=="critical")'
```

### Expected Output

```json
{
  "id": 1,
  "type": "vulnerability",
  "severity": "critical",
  "description": "CVE-2021-44228 detected in log4j-core",
  "affected_resources": [
    {
      "name": "log4shell-demo",
      "namespace": "default",
      "type": "Pod"
    }
  ],
  "cve_id": "CVE-2021-44228",
  "cvss_score": 10.0,
  "package_name": "log4j-core",
  "recommended_action": "Upgrade to version 2.15.0 or later",
  "status": "active"
}
```

✅ **Success!** You've detected a critical vulnerability!

---

## 🎯 Bonus: Risk Score

Check the risk score calculated for the vulnerable pod:

```bash
curl -s http://localhost:8080/api/v1/risk/pods/default/log4shell-demo | jq '.'
```

Expected output:
```json
{
  "resource_uid": "...",
  "resource_type": "Pod",
  "total_score": 95.5,
  "vulnerability_score": 80.0,
  "rbac_score": 10.0,
  "network_score": 5.5,
  "calculated_at": "2024-12-22T10:00:00Z"
}
```

---

## 🎉 Congratulations!

You've successfully:
- ✅ Deployed Fortuna
- ✅ Loaded CVE data
- ✅ Detected a critical vulnerability (Log4Shell!)
- ✅ Generated risk scores

---

## 🔧 Clean Up

```bash
# Delete test pod
kubectl delete pod log4shell-demo -n default

# (Optional) Delete Fortuna
kubectl delete namespace fortuna
```

---

## 🚀 Next Steps

### Learn More
- **[Full Installation Guide](./README.md)** - Production deployment
- **[Architecture Overview](../architecture/README.md)** - How it works
- **[Load Full CVE Database](../development/CVE_LOADING_GUIDE.md)** - 74,000+ CVEs

### Explore Features
- **[SBOM Generation](../components/sbom/README.md)** - Custom extraction
- **[Policy Engine](../components/policy-engine/README.md)** - Write policies
- **[Risk Engine](../components/risk-engine/README.md)** - Risk scoring

### Operations
- **[Monitoring](../operations/MONITORING.md)** - Set up observability
- **[Backup & Restore](../operations/BACKUP_RESTORE.md)** - Protect data

---

## 💡 Tips

**Performance**: This quickstart uses minimal resources. For production:
- Use persistent volumes for PostgreSQL
- Deploy NATS cluster (3 replicas)
- Load full CVE database (74k+ CVEs)
- Enable TLS/mTLS

**Troubleshooting**: If insights don't appear:
```bash
# Check Core logs
kubectl logs -n fortuna -l app=fortuna-core --tail=100

# Check NATS connectivity
kubectl exec -it -n fortuna nats-0 -- nats server check jetstream

# Verify CVE data
kubectl exec -it $(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -n fortuna -- \
  psql -U postgres -d fortuna -c "SELECT COUNT(*) FROM cves;"
```

---

**Questions?** Check the [FAQ](./FAQ.md) or [open an issue](https://github.com/your-org/fortuna/issues)!

