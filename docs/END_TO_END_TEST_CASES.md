# End-to-End Event Flow Test Cases

**Date**: 2025-12-01  
**Purpose**: Verify complete event flow from Agent → Core → NATS → Workers → Database

---

## Test Overview

### Event Flow Architecture
```
Agent (mTLS) → Core Ingest → NATS (ksam.raw.*) 
  → Normalizer Worker → NATS (ksam.normalized.*)
  → Correlator Worker → Database
  → Risk Worker → NATS (ksam.insights.*)
```

---

## Test Case 1: Agent to Core mTLS Connection

### Objective
Verify Agent can establish mTLS connection with Core

### Prerequisites
- Core pod running with mTLS enabled
- Agent pod running with TLS enabled
- Certificates mounted correctly

### Test Steps
1. Check Core pod status
2. Check Agent pod status
3. Verify Core TLS configuration
4. Verify Agent TLS configuration
5. Check Agent connection logs
6. Verify data streaming

### Expected Results
- ✅ Core pod: Running, TLS enabled
- ✅ Agent pod: Running, TLS enabled
- ✅ Agent logs: "Successfully streamed inventory items"
- ✅ No connection errors
- ✅ No TLS handshake errors

### Validation Commands
```bash
# Check Core status
kubectl get pods -n ksam -l app=ksam-core

# Check Agent status
kubectl get pods -n ksam -l app=ksam-agent

# Check Agent connection
kubectl logs -n ksam -l app=ksam-agent | grep -E "connected|streamed|error"

# Check Core TLS
kubectl logs -n ksam -l app=ksam-core | grep -E "WITH mTLS|TLS_ENABLED"
```

---

## Test Case 2: Agent Inventory Collection

### Objective
Verify Agent collects Kubernetes resources correctly

### Prerequisites
- Agent connected to Core
- Kubernetes cluster accessible
- RBAC permissions configured

### Test Steps
1. Trigger Agent inventory collection
2. Check Agent logs for collection events
3. Verify resources being collected (Pods, ServiceAccounts, Roles, etc.)

### Expected Results
- ✅ Agent collecting Pods
- ✅ Agent collecting ServiceAccounts
- ✅ Agent collecting Roles/RoleBindings
- ✅ Agent collecting ClusterRoles/ClusterRoleBindings
- ✅ Collection happening at configured interval

### Validation Commands
```bash
# Check Agent collection logs
kubectl logs -n ksam -l app=ksam-agent | grep -E "Collecting|inventory|Pod|ServiceAccount"

# Check collection frequency
kubectl logs -n ksam -l app=ksam-agent | grep -E "Sync|interval"
```

---

## Test Case 3: Core Ingest API Processing

### Objective
Verify Core receives and processes inventory from Agent

### Prerequisites
- Agent streaming to Core
- Core Ingest API running
- mTLS connection established

### Test Steps
1. Monitor Core Ingest logs
2. Verify messages received from Agent
3. Check message parsing
4. Verify publishing to NATS

### Expected Results
- ✅ Core receiving inventory items
- ✅ Messages parsed correctly
- ✅ Publishing to `ksam.raw.*` subjects
- ✅ No parsing errors

### Validation Commands
```bash
# Check Core Ingest logs
kubectl logs -n ksam -l app=ksam-core | grep -E "Ingest|received|publish|ksam.raw"

# Check NATS messages
kubectl exec -n ksam <nats-pod> -- nats stream info ksam-raw
```

---

## Test Case 4: NATS Raw Event Publishing

### Objective
Verify raw events published to NATS correctly

### Prerequisites
- Core publishing to NATS
- NATS stream configured
- NATS subjects: `ksam.raw.*`

### Test Steps
1. Check NATS stream status
2. Verify messages in `ksam-raw` stream
3. Check message subjects
4. Verify message content

### Expected Results
- ✅ Stream `ksam-raw` exists
- ✅ Messages in stream
- ✅ Subjects: `ksam.raw.pods`, `ksam.raw.serviceaccounts`, etc.
- ✅ Message payload valid JSON

### Validation Commands
```bash
# Check NATS streams
kubectl exec -n ksam <nats-pod> -- nats stream ls

# Check stream info
kubectl exec -n ksam <nats-pod> -- nats stream info ksam-raw

# Check messages
kubectl exec -n ksam <nats-pod> -- nats stream view ksam-raw --last 10
```

---

## Test Case 5: Normalizer Worker Processing

### Objective
Verify Normalizer Worker processes raw events and publishes normalized events

### Prerequisites
- Raw events in NATS
- Normalizer Worker running
- Subscribing to `ksam.raw.>`

### Test Steps
1. Check Normalizer Worker logs
2. Verify subscription to raw events
3. Check normalization processing
4. Verify publishing to normalized subjects

### Expected Results
- ✅ Worker subscribed to `ksam.raw.>`
- ✅ Processing raw events
- ✅ Publishing to `ksam.normalized.*`
- ✅ Normalized data structure correct

### Validation Commands
```bash
# Check Normalizer Worker logs
kubectl logs -n ksam -l app=ksam-core | grep -E "NormalizerWorker|normalized|ksam.normalized"

# Check normalized stream
kubectl exec -n ksam <nats-pod> -- nats stream info ksam-normalized
```

---

## Test Case 6: Correlator Worker Database Storage

### Objective
Verify Correlator Worker stores normalized data in database

### Prerequisites
- Normalized events in NATS
- Correlator Worker running
- Database accessible
- Subscribing to `ksam.normalized.>`

### Test Steps
1. Check Correlator Worker logs
2. Verify database inserts
3. Check stored data
4. Verify relationships created

### Expected Results
- ✅ Worker subscribed to `ksam.normalized.>`
- ✅ Processing normalized events
- ✅ Storing Pods in database
- ✅ Storing ServiceAccounts in database
- ✅ Storing Roles/RoleBindings in database
- ✅ No JSON errors
- ✅ Relationships created correctly

### Validation Commands
```bash
# Check Correlator Worker logs
kubectl logs -n ksam -l app=ksam-core | grep -E "CorrelatorWorker|Stored|database|ERROR.*json"

# Check database
kubectl exec -n ksam <postgres-pod> -- psql -U ksam -d ksam -c "SELECT COUNT(*) FROM pods;"
kubectl exec -n ksam <postgres-pod> -- psql -U ksam -d ksam -c "SELECT COUNT(*) FROM service_accounts;"
```

---

## Test Case 7: Risk Worker Risk Evaluation

### Objective
Verify Risk Worker evaluates risks and generates insights

### Prerequisites
- Normalized events in NATS
- Risk Worker running
- Risk rules loaded
- Subscribing to `ksam.normalized.>`

### Test Steps
1. Check Risk Worker logs
2. Verify risk evaluation
3. Check insight generation
4. Verify publishing to insights stream

### Expected Results
- ✅ Worker subscribed to `ksam.normalized.>`
- ✅ Evaluating resources for risks
- ✅ Generating insights
- ✅ Publishing to `ksam.insights.*`
- ✅ Insights stored in database

### Validation Commands
```bash
# Check Risk Worker logs
kubectl logs -n ksam -l app=ksam-core | grep -E "RiskWorker|risk|insight|evaluating"

# Check insights stream
kubectl exec -n ksam <nats-pod> -- nats stream info ksam-insights

# Check insights in database
kubectl exec -n ksam <postgres-pod> -- psql -U ksam -d ksam -c "SELECT COUNT(*) FROM insights;"
```

---

## Test Case 8: Complete Event Flow Integration

### Objective
Verify complete end-to-end flow from Agent to Database

### Prerequisites
- All components running
- All previous test cases passed

### Test Steps
1. Trigger Agent collection
2. Monitor complete flow:
   - Agent → Core (mTLS)
   - Core → NATS (raw)
   - Normalizer → NATS (normalized)
   - Correlator → Database
   - Risk → Insights
3. Verify data consistency
4. Check for errors

### Expected Results
- ✅ Complete flow working
- ✅ Data in database
- ✅ Insights generated
- ✅ No errors in any component
- ✅ Data consistency maintained

### Validation Commands
```bash
# Complete flow check
./scripts/test_end_to_end_flow.sh

# Check all components
kubectl get pods -n ksam

# Check database state
kubectl exec -n ksam <postgres-pod> -- psql -U ksam -d ksam -c "
  SELECT 
    (SELECT COUNT(*) FROM pods) as pods,
    (SELECT COUNT(*) FROM service_accounts) as service_accounts,
    (SELECT COUNT(*) FROM roles) as roles,
    (SELECT COUNT(*) FROM insights) as insights;
"
```

---

## Test Case 9: Error Handling and Recovery

### Objective
Verify system handles errors gracefully

### Prerequisites
- System running normally

### Test Steps
1. Simulate network issues
2. Simulate database errors
3. Simulate NATS connection loss
4. Verify recovery mechanisms

### Expected Results
- ✅ Errors logged appropriately
- ✅ Retry mechanisms working
- ✅ System recovers automatically
- ✅ No data loss

---

## Test Case 10: Performance and Load

### Objective
Verify system handles load correctly

### Prerequisites
- System running normally

### Test Steps
1. Monitor message processing rate
2. Check worker pool utilization
3. Monitor database connection pool
4. Check memory/CPU usage

### Expected Results
- ✅ Processing rate acceptable
- ✅ No worker pool exhaustion
- ✅ Database connections managed
- ✅ Resource usage within limits

---

## Test Execution Script

See `KSAM/scripts/test_end_to_end_flow.sh` for automated test execution.

---

## Success Criteria

All test cases must pass:
- ✅ Test Case 1: mTLS Connection
- ✅ Test Case 2: Inventory Collection
- ✅ Test Case 3: Ingest Processing
- ✅ Test Case 4: NATS Raw Publishing
- ✅ Test Case 5: Normalizer Processing
- ✅ Test Case 6: Database Storage
- ✅ Test Case 7: Risk Evaluation
- ✅ Test Case 8: Complete Flow
- ✅ Test Case 9: Error Handling
- ✅ Test Case 10: Performance

---

**Status**: Ready for execution


