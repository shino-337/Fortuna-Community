# DNS Resolution Fix for Core Pod

**Date**: 2026-01-07  
**Issue**: Core pod cannot resolve DNS names, causing database connection failures

---

## Problem

Core pod on master node was unable to resolve `postgres.fortuna.svc.cluster.local`, causing:
- DNS query timeout: `read udp 10.244.0.155 -> 10.96.0.10:53: i/o timeout`
- Database connection failures
- Core pod in CrashLoopBackOff

---

## Root Cause Analysis

### 1. Network Location Mismatch
- **Core Pod**: Master node (10.244.0.155)
- **CoreDNS Pods**: Worker node (10.244.1.20, 10.244.1.21)
- DNS queries from master pod network experienced timeouts

### 2. CoreDNS Forward Configuration Issue
- CoreDNS was forwarding ALL queries (including internal) to `/etc/resolv.conf`
- External DNS queries (8.8.8.8) were timing out
- Internal service queries were also affected by forward timeout

### 3. DNS Query Timeout
- Core pod queries: `read udp 10.244.0.155 -> 10.96.0.10:53: i/o timeout`
- NATS pod (worker) could resolve because it's on the same node as CoreDNS

---

## Solutions Implemented

### Solution 1: Core Deployment DNS Config

Added explicit `dnsConfig` to Core deployment:

```yaml
dnsPolicy: ClusterFirst
dnsConfig:
  nameservers:
  - "10.96.0.10"
  searches:
  - "fortuna.svc.cluster.local"
  - "svc.cluster.local"
  - "cluster.local"
  options:
  - name: "ndots"
    value: "5"
  - name: "timeout"
    value: "2"
  - name: "attempts"
    value: "3"
```

**File**: `deploy/core-deployment.yaml`

### Solution 2: CoreDNS Forward Configuration

Modified CoreDNS ConfigMap to skip forwarding for internal domains:

```yaml
forward . /etc/resolv.conf {
   max_concurrent 1000
   except cluster.local svc.cluster.local fortuna.svc.cluster.local
}
```

**Effect**: Internal service queries are handled directly by Kubernetes plugin, not forwarded to external DNS.

**Apply**: `kubectl apply -f <coredns-configmap.yaml>`

---

## Verification

### Before Fix
```bash
$ kubectl exec -n fortuna fortuna-core-xxx -- getent hosts postgres.fortuna.svc.cluster.local
command terminated with exit code 2
```

### After Fix
```bash
$ kubectl exec -n fortuna fortuna-core-xxx -- getent hosts postgres.fortuna.svc.cluster.local
10.102.67.249   postgres.fortuna.svc.cluster.local
```

✅ DNS resolution successful!

---

## Expected Results

- ✅ Core pod can resolve internal service names
- ✅ Database connection succeeds
- ✅ Core pod becomes Ready
- ✅ No more DNS timeout errors

---

## Monitoring

After applying fixes:
1. CoreDNS pods restart automatically
2. Core pod restarts with new DNS config
3. Monitor Core logs for database connection success
4. Verify Core pod becomes Ready

---

## Files Modified

1. `deploy/core-deployment.yaml` - Added dnsConfig
2. CoreDNS ConfigMap (kube-system/coredns) - Modified forward config

---

**Status**: ✅ Fixed  
**DNS Resolution**: ✅ Working  
**Database Connection**: ⏳ In Progress
