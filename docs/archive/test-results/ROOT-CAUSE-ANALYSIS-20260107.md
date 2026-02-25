# Root Cause Analysis - Core Pod CrashLoopBackOff
**Date:** 2026-01-07  
**Status:** 🔴 CRITICAL

## Executive Summary

Core pods are in `CrashLoopBackOff` state, preventing the system from starting. The root cause is **DNS resolution timeout** when Core tries to connect to PostgreSQL, causing the database connection to block HTTP server startup.

## Issues Identified

### 1. 🔴 CRITICAL: Core Pod CrashLoopBackOff

**Status:** Core pods crash before HTTP server starts

**Evidence:**
- Core pod logs stop at: `[MAIN] Initializing database connection...`
- No logs after database connection attempt
- Pod restarts 7 times
- Status: `CrashLoopBackOff`

**Database Connection Details:**
- Database URL: `postgres://postgres:postgres@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable`
- Connection retries: 20 attempts with exponential backoff
- Connection timeout: 10 seconds per attempt
- Total timeout: Can take several minutes

**Impact:**
- Core never reaches HTTP server startup
- `/ready` endpoint never becomes accessible
- Service has no ready endpoints
- Agent cannot connect to Core

### 2. 🔴 DNS Resolution Timeout

**Status:** CoreDNS forwarding external queries times out

**Evidence:**
- CoreDNS logs show: `read udp ...->8.8.8.8:53: i/o timeout`
- CoreDNS is trying to forward queries to external DNS (8.8.8.8, 8.8.4.4)
- External DNS queries fail (network/firewall issue)
- This may affect internal DNS resolution performance

**CoreDNS Configuration:**
- Has `except` clauses for internal domains ✅
- But still forwarding some queries externally ❌

**Impact:**
- DNS resolution for `postgres.fortuna.svc.cluster.local` may timeout
- Core cannot resolve database service name
- Database connection fails

### 3. 🔴 Core Service Has No Endpoints

**Status:** Service exists but has no ready addresses

**Evidence:**
```bash
$ kubectl get endpoints fortuna-core -n fortuna
NAME           ENDPOINTS   AGE
fortuna-core               29h
```

**Impact:**
- Agent cannot connect to Core
- Agent logs show: `connection refused` or `DNS timeout`
- System cannot process SBOMs or CVEs

## Network Topology

- **Core Pod:** `k8s-master` node
- **PostgreSQL Pod:** `k8s-master` node (same node ✅)
- **PostgreSQL Service IP:** `10.102.67.249`
- **PostgreSQL Pod IP:** `10.244.0.176`
- **TCP Connection Test:** ✅ SUCCESS (from node to pod IP)

## Root Cause

**PRIMARY ROOT CAUSE:** DNS resolution timeout for `postgres.fortuna.svc.cluster.local`

Even though:
- PostgreSQL is running and healthy ✅
- PostgreSQL service has endpoint ✅
- Core and PostgreSQL are on same node ✅
- TCP connection works from node level ✅

Core cannot resolve the DNS name `postgres.fortuna.svc.cluster.local` due to:
1. CoreDNS forwarding queries to external DNS (8.8.8.8/8.8.4.4)
2. External DNS queries timing out
3. This may cause internal DNS resolution to timeout or be delayed
4. Core's database connection retries 20 times, taking several minutes
5. Eventually Core exits with error before HTTP server starts

**SECONDARY ISSUE:** Database connection blocks HTTP server startup

The readiness probe fix is correct (`/ready` doesn't check database), but:
- Core never reaches HTTP server startup
- Database connection is blocking the main thread
- After 20 retries, Core exits with `log.Fatalf`
- This prevents `/ready` endpoint from ever being accessible

## Solutions

### Option 1: Fix DNS Resolution (PREFERRED)

**Action:** Ensure CoreDNS can resolve internal services without external DNS

1. Verify CoreDNS `except` clauses are working
2. Check if external DNS is necessary (may be blocked by firewall)
3. Consider using `forward . /etc/resolv.conf` only for external queries
4. Test DNS resolution from Core pod

**Expected Result:**
- Core can resolve `postgres.fortuna.svc.cluster.local`
- Database connection succeeds
- HTTP server starts
- `/ready` endpoint becomes accessible

### Option 2: Use Direct Pod IP (QUICK FIX)

**Action:** Use PostgreSQL pod IP directly instead of service DNS

1. Get PostgreSQL pod IP: `10.244.0.176`
2. Update `DATABASE_URL` secret to use pod IP
3. Note: This is not recommended for production (pod IPs change)

**Expected Result:**
- Bypasses DNS resolution
- Database connection succeeds immediately
- HTTP server starts

### Option 3: Make Database Connection Non-Blocking (COMPLEX)

**Action:** Refactor Core startup to start HTTP server before database connection

1. Start HTTP server in goroutine immediately
2. Connect database in background
3. `/ready` endpoint works even if database is not ready
4. Requires significant code changes

**Expected Result:**
- HTTP server starts immediately
- `/ready` endpoint becomes accessible
- Database connects in background
- System can accept requests even if database is down

### Option 4: Reduce Database Retry Timeout (QUICK FIX)

**Action:** Reduce database connection retries and timeout

1. Reduce retries from 20 to 5
2. Reduce timeout from 10s to 5s
3. Core fails faster, allowing faster restart cycle

**Expected Result:**
- Core fails faster if database is unreachable
- Faster restart cycle
- But still won't solve DNS issue

## Recommended Action Plan

1. **Immediate:** Test DNS resolution from Core pod
   ```bash
   kubectl exec -n fortuna <core-pod> -- getent hosts postgres.fortuna.svc.cluster.local
   ```

2. **Short-term:** Fix CoreDNS external DNS forwarding
   - Check if external DNS is necessary
   - Update CoreDNS config if needed
   - Restart CoreDNS pods

3. **Medium-term:** Verify database connection works
   - Test connection from Core pod
   - Check database logs for connection attempts
   - Verify network policies

4. **Long-term:** Consider making database connection non-blocking
   - This allows system to start even if database is temporarily unavailable
   - Improves resilience

## Files Modified

- ✅ `core/internal/health/health.go` - Readiness probe fixed
- ✅ `core/cmd/main.go` - Endpoints added
- ✅ `deploy/core-deployment.yaml` - Probes updated

## Next Steps

1. Fix DNS resolution issue
2. Verify Core can connect to database
3. Confirm HTTP server starts
4. Verify `/ready` endpoint works
5. Test Agent connection to Core

---

**Status:** 🔴 BLOCKED - Waiting for DNS resolution fix

