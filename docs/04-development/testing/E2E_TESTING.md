# End-to-End Testing Guide

**Last Validated Against Version:** v2.0.0  
**Last Updated:** 2025-12-27  
**Status:** Current

---

## Overview

This guide covers end-to-end (E2E) testing of Fortuna K8s Management Platform, including the complete flow from Pod creation to Insight generation.

---

## E2E Flow Architecture

### Data Plane / Control Plane Architecture

- **Agent (Data Plane)**: Watches pods, extracts SBOMs, sends raw data to Core
- **Core (Control Plane)**: Receives SBOMs, matches CVEs, generates insights, exposes API

### Complete E2E Flow

```
1. Pod Created in Kubernetes
   │
   ▼
2. Agent Pod Watcher Detects Pod
   │
   ▼
3. Agent Extracts SBOM from Container Image
   │
   ▼
4. Agent Sends SBOM to Core via gRPC (mTLS)
   │
   ▼
5. Core Stores SBOM in Database
   │
   ▼
6. Core Publishes SBOM_CREATED Event to NATS
   │
   ▼
7. CVEMatcherWorker Receives Event
   │
   ▼
8. Core Matches CVEs Against SBOM Packages
   │
   ▼
9. Core Persists CVE Matches to Database
   │
   ▼
10. Core Creates Vulnerability Insights
    │
    ▼
11. Insights Available via API
```

---

## Running E2E Tests

### Prerequisites

- Kubernetes cluster running (Minikube or full cluster)
- Fortuna deployed and running
- Test namespace available

### Quick Test

```bash
cd tests/e2e/scenarios
./pod-to-insight-flow.sh
```

This script:
1. Creates a test pod with known vulnerable image
2. Waits for Agent to detect and process
3. Verifies SBOM extraction
4. Verifies CVE matching
5. Verifies insight generation
6. Verifies API response

### Detailed Test Steps

#### 1. Create Test Pod

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-vulnerable-pod
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx:1.19.0  # Known vulnerable (CVE-2021-23017)
    ports:
    - containerPort: 80
EOF
```

#### 2. Wait for Processing

```bash
# Wait for pod to be ready
kubectl wait --for=condition=ready pod/test-vulnerable-pod --timeout=60s

# Wait for Agent to process (30-60 seconds)
sleep 60
```

#### 3. Verify SBOM Extraction

```bash
# Check SBOM in database
kubectl exec -it -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -c \
  "SELECT id, image_digest, created_at FROM sboms WHERE image_digest LIKE '%nginx%' LIMIT 5;"
```

#### 4. Verify CVE Matching

```bash
# Check CVE matches
kubectl exec -it -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -c \
  "SELECT cm.id, c.id as cve_id, cm.package_name, cm.severity 
   FROM cve_matches cm 
   JOIN cves c ON cm.cve_id = c.id 
   LIMIT 10;"
```

#### 5. Verify Insights

```bash
# Check insights via API
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
curl http://localhost:8080/api/v1/insights?severity=HIGH

# Or check database
kubectl exec -it -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -c \
  "SELECT id, title, severity, insight_type FROM insights WHERE severity IN ('HIGH', 'CRITICAL') LIMIT 10;"
```

---

## Test Scenarios

### Scenario 1: Pod to Insight Flow

**Objective**: Verify complete flow from pod creation to insight generation

**Steps**:
1. Create pod with vulnerable image
2. Verify Agent detects pod
3. Verify SBOM extraction
4. Verify CVE matching
5. Verify insight creation
6. Verify API response

**Expected Results**:
- SBOM created in database
- CVE matches found
- Insights created with HIGH/CRITICAL severity
- API returns insights

### Scenario 2: CVE Insights Test

**Objective**: Verify CVE-based insights are created correctly

**Steps**:
1. Create pod with image containing known CVE
2. Wait for processing
3. Query insights API
4. Verify insight metadata (CVE ID, severity, affected component)

**Expected Results**:
- Insights contain CVE ID
- Severity matches CVE severity
- Affected component identified
- Recommendation provided

### Scenario 3: SBOM Extraction Verification

**Objective**: Verify SBOM extraction from container images

**Steps**:
1. Create pod with various image types (Alpine, Debian, Ubuntu)
2. Verify SBOM extraction
3. Verify component listing
4. Verify PURL format

**Expected Results**:
- SBOM extracted for all image types
- Components listed with PURLs
- Package versions identified

---

## Test Scripts

### Available Scripts

Located in `tests/e2e/scripts/`:

- `create-test-pod.sh` - Create test pod with specified image
- `verify-sbom.sh` - Verify SBOM extraction
- `verify-cve-matches.sh` - Verify CVE matching
- `verify-insights.sh` - Verify insight generation
- `compare-api-db.sh` - Compare API response with database

### Usage

```bash
# Create test pod
./tests/e2e/scripts/create-test-pod.sh nginx:1.19.0

# Verify SBOM
./tests/e2e/scripts/verify-sbom.sh nginx:1.19.0

# Verify CVE matches
./tests/e2e/scripts/verify-cve-matches.sh

# Verify insights
./tests/e2e/scripts/verify-insights.sh
```

---

## Verification Checklists

### SBOM Verification

- [ ] SBOM created in database
- [ ] Image digest matches
- [ ] Components extracted
- [ ] PURLs formatted correctly
- [ ] Package versions identified

### CVE Matching Verification

- [ ] CVE matches found
- [ ] Version ranges correct
- [ ] Severity levels accurate
- [ ] Package names match
- [ ] Ecosystem identified

### Insight Verification

- [ ] Insights created
- [ ] Severity levels correct
- [ ] CVE IDs linked
- [ ] Resource UIDs correct
- [ ] Recommendations provided

### API Verification

- [ ] API endpoints accessible
- [ ] Insights returned via API
- [ ] Filtering works (severity, type)
- [ ] Pagination works
- [ ] Response format correct

---

## Performance Testing

### Metrics to Measure

- **SBOM Extraction Time**: Time to extract SBOM from image
- **CVE Matching Time**: Time to match CVEs against SBOM
- **Insight Generation Time**: Time to create insights
- **API Response Time**: Time for API to return results

### Performance Scripts

Located in `tests/performance/scripts/`:

- `measure-sbom-processing.sh` - Measure SBOM processing time
- `measure-cve-matching.sh` - Measure CVE matching time
- `measure-insight-generation.sh` - Measure insight generation time

### Expected Performance

- **SBOM Extraction**: 2-5 seconds per image
- **CVE Matching**: <1 second per SBOM
- **Insight Generation**: <100ms per resource
- **API Response**: <50ms (p99)

---

## Troubleshooting

### Agent Not Detecting Pods

**Problem**: Agent not detecting new pods

**Solutions**:
```bash
# Check Agent logs
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=100

# Verify Agent RBAC
kubectl get clusterrole fortuna-agent -o yaml

# Check pod watcher status
kubectl exec -it -n fortuna <agent-pod> -- ps aux | grep watcher
```

### SBOM Not Extracted

**Problem**: SBOM not created in database

**Solutions**:
```bash
# Check Core logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=100 | grep SBOM

# Verify image access
kubectl exec -it -n fortuna <core-pod> -- ls /var/run/docker.sock

# Check SBOM worker status
kubectl logs -n fortuna -l app.kubernetes.io/component=core | grep SBOMWorker
```

### CVEs Not Matching

**Problem**: No CVE matches found

**Solutions**:
```bash
# Verify CVE data loaded
kubectl exec -it -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -c \
  "SELECT COUNT(*) FROM cves;"

# Check CVE matcher logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core | grep CVEMatcher

# Verify SBOM components have PURLs
kubectl exec -it -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -c \
  "SELECT purl FROM sbom_components LIMIT 10;"
```

### Insights Not Created

**Problem**: No insights generated

**Solutions**:
```bash
# Check insight manager logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core | grep InsightManager

# Verify CVE matches exist
kubectl exec -it -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -c \
  "SELECT COUNT(*) FROM cve_matches WHERE severity IN ('HIGH', 'CRITICAL');"

# Check risk worker status
kubectl logs -n fortuna -l app.kubernetes.io/component=core | grep RiskWorker
```

---

## Best Practices

### Test Data

- Use known vulnerable images for predictable results
- Test with various image types (Alpine, Debian, Ubuntu)
- Test with different package ecosystems (npm, PyPI, Go)

### Test Isolation

- Use separate namespace for tests
- Clean up test resources after tests
- Use unique pod names to avoid conflicts

### Verification

- Always verify at multiple levels (database, API, logs)
- Check both positive and negative cases
- Verify error handling and edge cases

---

## References

- [Test Suite README](../../../tests/README.md)
- [E2E Test Scenarios](../../../tests/e2e/scenarios/TEST_SCENARIOS.md)
- [Performance Testing](../performance/README.md)
- [API Reference](../../08-api-reference/README.md)

---

**For test execution guide, see:** `tests/TEST_EXECUTION_GUIDE.md`

