# KSAM E2E Testing Scripts

Scripts for testing and verifying the KSAM CVE detection pipeline.

## Quick Start

### 1. Re-trigger E2E Pod Processing

If your E2E pod was created before SBOMWorker subscribed, re-trigger it:

```bash
./scripts/retrigger-e2e-pod.sh
```

This will:
- Delete the existing E2E pod
- Recreate it with fresh events
- Wait for it to become Ready

### 2. Verify Pipeline

Check if the E2E pod has been processed through the full pipeline:

```bash
./scripts/verify-e2e-pipeline.sh
```

This checks:
- ✅ SBOM generation
- ✅ Component extraction
- ✅ CVE matching
- ✅ Insight creation

### 3. Monitor Pipeline in Real-Time

Watch the pipeline process events as they happen:

```bash
# Monitor all events
./scripts/monitor-pipeline.sh

# Monitor specific pod
./scripts/monitor-pipeline.sh ksam ksam-e2e-vuln-debian10
```

### 4. Full E2E Test with Timeouts

Run a complete end-to-end test with proper timeouts:

```bash
./scripts/test-e2e-cve-pipeline.sh
```

This will:
1. Delete old pod
2. Create new pod
3. Wait for SBOM generation (3 min timeout)
4. Wait for CVE matching (2 min timeout)
5. Wait for insight creation (1 min timeout)
6. Display detailed results

## Timeouts

The test script uses these timeouts:

| Stage | Timeout | Notes |
|-------|---------|-------|
| Pod Ready | 60s | Usually completes in 5-10s |
| SBOM Generation | 180s (3 min) | Includes image pull + extraction |
| CVE Matching | 120s (2 min) | Depends on component count |
| Insight Creation | 60s (1 min) | May be filtered by severity |
| Total | 420s (7 min) | Maximum end-to-end time |

## Troubleshooting

### SBOM Not Generated

If SBOM is not generated after 3 minutes:

```bash
# Check if SBOMWorker is running
kubectl get pods -n ksam -l app=ksam-core

# Check SBOMWorker logs
kubectl logs -n ksam -l app=ksam-core --tail=50 | grep SBOMWorker

# Check if pod is in Running phase (not Completed)
kubectl get pod -n ksam-e2e ksam-e2e-vuln-debian10
```

**Common issues:**
- Pod completed before SBOMWorker could process it (use `restartPolicy: Never` and long sleep)
- SBOMWorker filtering out non-Running pods
- NATS slow consumer dropping messages

### CVE Matching Failed

If CVE matching returns 0 results:

```bash
# Check if CVE database is populated
kubectl exec -n ksam postgres-xxx -- psql -U postgres -d ksam -c "SELECT COUNT(*) FROM cves;"

# Expected: at least 1 (CVE-2014-0011)
# For full testing: ~74,000 CVEs

# Check CVEMatcherWorker logs
kubectl logs -n ksam -l app=ksam-core --tail=100 | grep CVEMatcher

# Verify sbom.created events are being published
kubectl logs -n ksam -l app=ksam-core --tail=100 | grep "sbom.created"
```

**Common issues:**
- CVE database empty (run cve-loader job)
- Package name mismatch (check PURL format)
- CVEMatcherWorker not subscribed to ksam.sbom.created

### Insights Not Created

If insights are not created:

```bash
# Check insight severity filter
kubectl logs -n ksam -l app=ksam-core --tail=50 | grep -i insight

# Check database directly
kubectl exec -n ksam postgres-xxx -- psql -U postgres -d ksam \
  -c "SELECT COUNT(*) FROM insights WHERE status='active';"
```

**Common issues:**
- CVEs are filtered by severity (only CRITICAL/HIGH create insights by default)
- Check `CVEMatcherWorker.onlySeverities` in core/pkg/worker/cve_matcher_worker.go

## Environment Variables

Override defaults in test script:

```bash
# Custom namespace
TEST_NAMESPACE=my-test ./scripts/test-e2e-cve-pipeline.sh

# Custom pod name
TEST_POD_NAME=my-test-pod ./scripts/test-e2e-cve-pipeline.sh

# Custom expected CVE
EXPECTED_CVE=CVE-2099-9999 ./scripts/test-e2e-cve-pipeline.sh
```

## Pipeline Flow

```
Agent
  ↓ publishes to
NATS (ksam.raw.pods)
  ↓ consumed by
NormalizerWorker
  ↓ publishes to
NATS (ksam.normalized.pods)
  ↓ consumed by
SBOMWorker (DeliverAll, MaxAckPending=10)
  ↓ generates SBOM + publishes to
NATS (ksam.sbom.created)
  ↓ consumed by
CVEMatcherWorker (DeliverAll, MaxAckPending=50)
  ↓ matches CVEs + creates insights
Insights API
```

## Known Issues

### Issue #1: Messages Dropped Before Subscription

**Problem:** If a pod is created before SBOMWorker subscribes, events are lost (with `DeliverNew()`)

**Solution:** Use `DeliverAll()` mode (applied in latest version)

### Issue #2: NATS Slow Consumer

**Problem:** Messages dropped if workers can't keep up

**Solution:** Increase `MaxAckPending` (1 → 10 for SBOM, 1 → 50 for CVE)

### Issue #3: Pod Phase Filtering

**Problem:** SBOMWorker skips non-Running pods

**Solution:** Ensure test pods use `restartPolicy: Never` and long sleep duration

## Performance Benchmarks

Typical times on minikube (single core):

| Stage | Time | Notes |
|-------|------|-------|
| Pod startup | 5-10s | Includes image pull from local |
| SBOM generation | 10-30s | Depends on image size |
| CVE matching | 5-15s | Depends on component count |
| Insight creation | 1-3s | Almost instant |
| **Total** | **30-60s** | End-to-end for E2E test |

## Further Reading

- [KSAM Architecture](../docs/architecture.md)
- [CVE Database Setup](../docs/cve-database.md)
- [Worker Configuration](../docs/workers.md)
