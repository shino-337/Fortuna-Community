# Infrastructure Check Report

## Date: 2025-11-28

## Summary

**Status**: ⚠️ **ISSUES FOUND**

**Critical Issue**: PostgreSQL failing due to disk space

**Root Cause**: Minikube disk space exhausted

---

## 1. Disk Space Analysis

### Minikube Disk
- **Status**: ⚠️ Full or near full
- **Issue**: PostgreSQL cannot write lock file

### Storage Usage
- **Total PVCs**: 8 volumes
- **Total Requested**: ~85Gi
  - NATS: 60Gi (6 x 10Gi - includes duplicates)
  - PostgreSQL: 20Gi
  - Redis: 5Gi

### Duplicate PVCs Found
- **NATS**: Has both `data-nats-*` and `nats-pvc-*` volumes
  - This is likely causing unnecessary disk usage
  - Should clean up duplicate volumes

---

## 2. Component Status

### ✅ Healthy Components
- **NATS**: Running (3 replicas)
- **Redis**: Running (1 replica)
- **Services**: All services defined correctly

### ❌ Failed Components
- **PostgreSQL**: CrashLoopBackOff
  - Error: `No space left on device`
  - Cannot write lock file
  
- **Core**: CrashLoopBackOff
  - Cannot connect to PostgreSQL
  - Blocked by database issue
  
- **Agent**: CrashLoopBackOff
  - Cannot connect to Core
  - Blocked by Core issue

---

## 3. Issues Identified

### Critical Issues (P0)
1. **Disk Space Exhausted**
   - Minikube has no free space
   - PostgreSQL cannot start
   - Blocks entire system

2. **Duplicate NATS PVCs**
   - Both `data-nats-*` and `nats-pvc-*` exist
   - Wasting ~30Gi of space
   - Should clean up duplicates

### Non-Critical Issues
1. Metrics server may not be available (for resource monitoring)

---

## 4. Recommendations

### Immediate Actions (P0)

1. **Free Up Disk Space**
   ```bash
   # Clean up unused Docker images in Minikube
   minikube ssh -- docker system prune -a -f
   
   # Remove duplicate NATS PVCs (after verifying which are in use)
   # Keep: data-nats-* (used by StatefulSet)
   # Remove: nats-pvc-* (duplicates)
   ```

2. **Increase Minikube Disk Size** (if possible)
   ```bash
   minikube stop
   minikube start --disk-size=50g  # or larger
   ```

3. **Clean Up Unused Resources**
   - Remove old/unused PVCs
   - Clean up old Docker images
   - Remove unused volumes

### Short-term Actions (P1)

4. **Fix PostgreSQL**
   - After freeing space, restart PostgreSQL
   - Verify database is accessible
   - Check data persistence

5. **Verify All Components**
   - After PostgreSQL fix, verify Core starts
   - Check Agent connectivity
   - Verify Risk Engine functionality

### Long-term Actions (P2)

6. **Optimize Storage**
   - Review PVC sizes (may be too large)
   - Consider using smaller volumes for dev
   - Implement storage cleanup policies

---

## 5. Storage Breakdown

| Component | PVCs | Total Size | Status |
|-----------|------|------------|--------|
| NATS | 6 (3 duplicates) | 60Gi | ⚠️ Duplicates |
| PostgreSQL | 1 | 20Gi | ❌ Failed |
| Redis | 1 | 5Gi | ✅ OK |
| **Total** | **8** | **~85Gi** | ⚠️ |

---

## 6. Action Plan

### Step 1: Free Disk Space
- [ ] Clean up Docker images in Minikube
- [ ] Remove duplicate NATS PVCs
- [ ] Clean up unused volumes

### Step 2: Fix PostgreSQL
- [ ] Verify disk space available
- [ ] Restart PostgreSQL pod
- [ ] Verify database accessible

### Step 3: Verify System
- [ ] Check Core pod starts
- [ ] Verify Agent connectivity
- [ ] Test Risk Engine

### Step 4: Continue Phase 2
- [ ] After system verified
- [ ] Continue with Phase 2.2

---

## 7. Commands to Fix

### Clean Up Docker Images
```bash
eval $(minikube docker-env)
docker system prune -a -f
```

### Remove Duplicate NATS PVCs
```bash
# First, verify which PVCs are actually in use
kubectl get pvc -n ksam

# Remove duplicates (nats-pvc-* if data-nats-* are in use)
kubectl delete pvc nats-pvc-0 nats-pvc-1 nats-pvc-2 -n ksam
```

### Restart PostgreSQL
```bash
kubectl delete pod -n ksam -l app=postgres
# Wait for new pod to start
kubectl get pods -n ksam -l app=postgres
```

---

## Status

**Infrastructure**: ⚠️ **NEEDS ATTENTION**

**Blocking Issue**: Disk space

**Action Required**: Free up disk space before continuing

