# Quick Start Guide

Quick guide to running KSAM test suite.

## Prerequisites

1. **Kubernetes Cluster** - Accessible via `kubectl`
2. **Core Service** - Running and accessible
3. **Agent Service** - Running and connected
4. **Database** - PostgreSQL accessible
5. **NATS** - Running and accessible
6. **Tools**:
   - `kubectl`
   - `curl`
   - `jq`
   - `psql` (PostgreSQL client)
   - `bc` (calculator)

## Environment Variables

Set these before running tests:

```bash
export TEST_NAMESPACE="ksam-test"
export CORE_API_URL="http://localhost:8080"
export DB_HOST="localhost"
export DB_PORT="5432"
export DB_NAME="ksam"
export DB_USER="ksam"
export DB_PASSWORD="your-password"
```

## Quick Test

### 1. Run Complete E2E Test

```bash
cd tests/e2e/scenarios
./pod-to-insight-flow.sh
```

This will:
- Create a test pod
- Wait for SBOM extraction
- Wait for CVE matching
- Wait for insight generation
- Verify API response
- Compare API with database
- Generate timing report

### 2. Run Individual Verification Scripts

```bash
# Create test pod
cd tests/e2e/scripts
./create-test-pod.sh

# Get pod UID from output, then:
POD_UID="<pod-uid-from-output>"

# Verify SBOM
./verify-sbom.sh "${POD_UID}"

# Get SBOM ID from output, then:
SBOM_ID="<sbom-id-from-output>"

# Verify CVE matches
./verify-cve-matches.sh "${SBOM_ID}"

# Verify insights
./verify-insights.sh "${POD_UID}"

# Compare API with database
./compare-api-db.sh "${POD_UID}"
```

### 3. Run Performance Tests

```bash
# Measure SBOM processing
cd tests/performance/scripts
./measure-sbom-processing.sh

# Measure CVE matching (requires SBOM ID)
./measure-cve-matching.sh "<sbom-id>"

# Measure insight generation (requires pod UID)
./measure-insight-generation.sh "<pod-uid>"
```

## Test Results

Results are saved in:
- `tests/e2e/results/` - E2E test results
- `tests/performance/results/` - Performance test results

Each test generates:
- Log file with complete execution log
- Summary file with key metrics
- API response snapshots (for E2E tests)

## Verification Checklist

Use the checklist to manually verify:
```bash
cat tests/verification/checklists/e2e-verification-checklist.md
```

## Troubleshooting

### Pod Not Created
- Check Kubernetes cluster access
- Verify namespace exists: `kubectl get namespace ${TEST_NAMESPACE}`
- Check pod creation logs

### SBOM Not Extracted
- Check Agent is running
- Verify Agent can access Kubernetes API
- Check Agent logs for errors
- Verify image is accessible

### CVE Matches Not Found
- This may be normal if image has no vulnerabilities
- Check CVE database is populated
- Verify CVE matching worker is running
- Check Core logs for CVE matching activity

### Insights Not Generated
- This may be normal if no CVEs found
- Check RiskWorker is running
- Verify insight generation logic
- Check Core logs for insight creation

### API Not Responding
- Check Core service is running
- Verify API endpoint is accessible
- Check Core logs for errors
- Verify database connection

## Next Steps

1. Review test results
2. Compare with performance targets
3. Verify data consistency
4. Check for errors or warnings
5. Review logs for issues

## Advanced Usage

### Custom Test Image

```bash
export TEST_IMAGE="your-image:tag"
./pod-to-insight-flow.sh
```

### Multiple Iterations

```bash
for i in {1..5}; do
    echo "Iteration $i"
    ./pod-to-insight-flow.sh
    sleep 10
done
```

### Parallel Testing

```bash
# Create multiple pods in parallel
for i in {1..10}; do
    POD_NAME="test-pod-${i}"
    kubectl run "${POD_NAME}" --image=nginx:latest -n ksam-test &
done
wait

# Then verify each
```

