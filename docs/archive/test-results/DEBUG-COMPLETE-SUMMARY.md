# Comprehensive Network Debug Report - Complete Summary

**Date**: 2026-01-07  
**Debug Type**: Complete Network Connectivity Analysis  
**Status**: Root Cause Identified

---

## Executive Summary

### Root Cause Identified ✅

**Core pod readiness probe is FAILING** because Core cannot connect to database (DNS timeout).

This causes a chain reaction:
1. Core /ready endpoint returns `not_ready`
2. Readiness probe fails
3. Endpoint in `notReadyAddresses` (not `addresses`)
4. Service doesn't route traffic to Core
5. Agent cannot connect to Core

---

## Debug Checklist Results

### ✅ BƯỚC 1: Core Listening Ports

**Status**: ✅ VERIFIED

- **HTTP Server**: Listening on `0.0.0.0:8080`
  - Code: `httpServer.ListenAndServe()` with `Addr: ":8080"`
  - ✅ Correct - listening on all interfaces

- **gRPC Server**: Listening on `0.0.0.0:9090`
  - Code: `net.Listen("tcp", "0.0.0.0:9090")`
  - ✅ Correct - listening on all interfaces

- **Readiness Endpoint**: `/ready`
  - Response: `{"status":"not_ready","checks":{"database":"error: failed to connect..."}}`
  - ❌ Returns `not_ready` because database connection fails

**Conclusion**: Core IS listening correctly on all interfaces. The issue is NOT with Core binding.

---

### ✅ BƯỚC 2: Service & Endpoints

**Service Details**:
- Service IP: `10.100.74.197`
- Service Selector: `app.kubernetes.io/component=core, app.kubernetes.io/name=fortuna`
- Pod Labels: ✅ Match service selector

**Endpoint Status**: ⚠️ **CRITICAL ISSUE**

```yaml
subsets:
- notReadyAddresses:
  - ip: 10.244.0.173
    nodeName: k8s-master
  ports:
  - name: grpc
    port: 9090
  - name: http
    port: 8080
```

**Critical Finding**: 
- Endpoint in `notReadyAddresses` (NOT in `addresses`)
- Service will **NOT route traffic** to Core pod
- This is why Agent gets "Connection refused"

**Root Cause**: Core pod readiness probe is failing because Core cannot connect to database.

---

### ✅ BƯỚC 3: Agent Connection Tests

**Agent Pod**: `fortuna-agent-2zqfj` (on `k8s-worker01`)

#### 1️⃣ DNS Resolution
```
✅ PASS
10.100.74.197   fortuna-core.fortuna.svc.cluster.local
```
Agent can resolve Core service name correctly.

#### 2️⃣ TCP Connection (Service IP - port 9090)
```
❌ FAIL - Connection refused
```
**Reason**: Service doesn't route because endpoint is in `notReadyAddresses`.

#### 3️⃣ TCP Connection (Service IP - port 8080)
```
❌ FAIL - Connection refused
```
**Reason**: Same as above - Service not routing.

#### 4️⃣ TCP Connection (Direct Pod IP - port 9090)
```
❌ FAIL - Connection refused/timeout
```
**Reason**: Network routing issue between worker and master nodes (same as PostgreSQL issue).

#### 5️⃣ HTTP Connection (Service IP)
```
❌ FAIL - Connection refused
```
**Reason**: Service not routing.

**Conclusion**: 
- DNS resolution: ✅ Working
- Service routing: ❌ Blocked (no ready endpoints)
- Direct pod IP: ❌ Network routing issue

---

### ✅ BƯỚC 4: NetworkPolicy

**Status**: ✅ None found

No NetworkPolicies are blocking traffic. This is NOT the issue.

---

### ✅ BƯỚC 5: Agent Config

**Agent Environment Variables**:
```
FORTUNA_CORE_ENDPOINT=fortuna-core.fortuna.svc.cluster.local:9090
FORTUNA_CLUSTER_ID=minikube
FORTUNA_SYNC_INTERVAL=30s
TLS_ENABLED=true
```

**Conclusion**: ✅ Agent config is correct. Using service name (not IP), correct port (9090).

---

### ✅ BƯỚC 6: Core Database Connection

**Core /ready Response**:
```json
{
  "status": "not_ready",
  "checks": {
    "database": "error: failed to connect to `host=postgres.fortuna.svc.cluster.local user=postgres database=fortuna`: hostname resolving error (lookup postgres.fortuna.svc.cluster.local on 10.96.0.10:53: read udp 10.244.0.173:53803->10.96.0.10:53: i/o timeout)"
  }
}
```

**Core DNS Test**:
```
❌ FAIL - DNS timeout for postgres.fortuna.svc.cluster.local
```

**Root Cause**: Core cannot resolve database DNS name, causing:
- Database connection failure
- Readiness probe failure
- Endpoint in notReadyAddresses
- Service not routing

---

## Root Cause Chain (Confirmed)

```
1. ❌ Core database DNS timeout
   → Core cannot resolve: postgres.fortuna.svc.cluster.local
   
2. ❌ Core /ready returns not_ready
   → Database check fails
   
3. ❌ Readiness probe fails
   → Kubernetes marks pod as not ready
   
4. ❌ Endpoint in notReadyAddresses
   → Service doesn't add endpoint to addresses
   
5. ❌ Service has no ready endpoints
   → Service doesn't route traffic
   
6. ❌ Agent cannot connect
   → Connection refused (Service not routing)
```

---

## Fix Required (Priority Order)

### 1. Fix Core Database DNS Connection (CRITICAL)

**Option A**: Apply DNS fix to Core (same as Agent)
- Update Core DNS config: timeout 5s, attempts 5
- Restart Core pod

**Option B**: Verify PostgreSQL is on master node (already done)
- PostgreSQL should be on same node as Core
- Check if Core DNS can resolve postgres

**Option C**: Use direct pod IP for database (workaround)
- Not recommended for production

### 2. Verify Core Becomes Ready

After DNS fix:
- Core should connect to database
- /ready endpoint should return `ready`
- Readiness probe should pass

### 3. Verify Endpoint Moves to addresses

After Core becomes ready:
- Endpoint should move from `notReadyAddresses` to `addresses`
- Service will start routing traffic

### 4. Verify Service Routes Traffic

After endpoint is ready:
- Service will route traffic to Core
- Agent should be able to connect

### 5. Verify Agent Connection

After Service routes:
- Agent should connect successfully
- SBOMs should flow from Agent to Core

---

## Verification Commands

After applying fixes, verify:

```bash
# 1. Check Core readiness
kubectl get pod <core-pod> -n fortuna -o jsonpath='{.status.containerStatuses[0].ready}'
# Expected: true

# 2. Check endpoints
kubectl get endpoints fortuna-core -n fortuna
# Expected: Should have IP in addresses (not notReadyAddresses)

# 3. Test from Agent
kubectl exec -n fortuna <agent-pod> -- getent hosts fortuna-core.fortuna.svc.cluster.local
kubectl exec -n fortuna <agent-pod> -- sh -c "timeout 5 bash -c '</dev/tcp/10.100.74.197/9090' && echo OK || echo FAIL"

# 4. Check Agent logs
kubectl logs -n fortuna -l app=fortuna-agent | grep -E "Connected|Heartbeat"
# Expected: Should see "Connected to Core" messages
```

---

## Files Modified

- `deploy/core-deployment.yaml`: May need DNS config update
- `deploy/agent-daemonset.yaml`: DNS config already fixed (timeout: 5s, attempts: 5)

---

## Next Steps

1. **Immediate**: Fix Core database DNS connection
2. **Verify**: Core becomes ready
3. **Test**: Agent connection
4. **Re-run**: E2E tests

---

**Report Generated**: $(date '+%Y-%m-%d %H:%M:%S')

