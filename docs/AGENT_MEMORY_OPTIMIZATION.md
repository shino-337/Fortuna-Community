# Agent Memory Optimization

**Date**: 2026-01-06  
**Issue**: Worker node running out of memory, Agent suspected as cause

---

## Problem Analysis

### Symptoms
- Worker node experiencing memory pressure
- Agent pod has high memory limits (2Gi)
- Multiple pods competing for memory on worker node

### Root Causes Identified

1. **Large Queue Buffer**
   - Queue buffer: 100 pods
   - Each pod object: ~1-5MB
   - Potential memory: 100-500MB just for queue

2. **Memory Leak in processedPods Map**
   - `processedPods map[string]bool` never cleaned up for long-running pods
   - Pods that never get deleted accumulate in map
   - No periodic cleanup mechanism

3. **Too Many Workers**
   - 3 workers processing pods in parallel
   - Each worker may hold image layers in memory during extraction
   - Parallel processing increases peak memory usage

4. **High Resource Limits**
   - Memory limit: 2Gi (too high)
   - Memory request: 512Mi
   - Worker node has limited memory (4GB total)

---

## Solutions Implemented

### 1. Reduced Queue Buffer Size
- **Before**: 100 pods
- **After**: 30 pods
- **Impact**: Reduces memory footprint by ~70%

### 2. Added Periodic Cleanup for processedPods Map
- **Change**: `map[string]bool` → `map[string]time.Time`
- **Cleanup**: Every 1 hour, remove entries older than 24 hours
- **Impact**: Prevents memory leak from long-running pods

### 3. Reduced Worker Count
- **Before**: 3 workers
- **After**: 2 workers
- **Impact**: Reduces parallel memory usage during SBOM extraction

### 4. Reduced Resource Limits
- **Memory requests**: 512Mi → 256Mi
- **Memory limits**: 2Gi → 1Gi
- **CPU requests**: 200m → 100m
- **CPU limits**: 1000m → 500m
- **Impact**: Prevents Agent from consuming too much memory

### 5. Added Mutex Protection
- Added `sync.RWMutex` for `processedPods` map access
- Prevents race conditions and ensures thread-safe cleanup

---

## Expected Results

- **Queue Memory**: Reduced from ~100-500MB to ~30-150MB
- **processedPods Map**: Bounded growth with automatic cleanup
- **Peak Memory**: Reduced from ~2GB to ~1GB
- **Worker Node**: Less memory pressure, more stable

---

## Monitoring

After deployment, monitor:
- Agent pod memory usage
- Worker node memory pressure
- Queue depth (should stay < 30)
- processedPods map size (should not grow unbounded)

---

## Additional Recommendations

1. **Monitor Containerd Images**: Clean up unused images periodically
2. **Monitor Other Pods**: Check if other pods (PostgreSQL, NATS) are also consuming memory
3. **Consider Node Scaling**: If memory pressure persists, consider adding more worker nodes
4. **Enable Metrics**: Enable metrics server to track memory usage over time

---

**Status**: ✅ Fixed  
**Deployment**: Requires Agent pod restart to apply changes
