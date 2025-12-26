# Pod Watcher Fixes Summary - 2025-12-26

## Issues Identified and Fixed

### 1. Missing Namespace Configuration ✅ FIXED
**Problem**: Informer factory wasn't configured to watch all namespaces
- Originally: No namespace specified → only watched default namespace
- Fix: Added `informers.WithNamespace(metav1.NamespaceAll)`

**File**: `agent/internal/watcher/pod_watcher_local.go`
```go
factory := informers.NewSharedInformerFactoryWithOptions(
    w.clientset,
    5*time.Second,
    informers.WithNamespace(metav1.NamespaceAll), // Watch all namespaces
    informers.WithTweakListOptions(func(options *metav1.ListOptions) {
        options.FieldSelector = fieldSelector
    }),
)
```

### 2. Insufficient Memory Limits ✅ FIXED
**Problem**: Agent OOMKilled during startup when processing all existing pods
- Original limit: 1Gi
- Fix: Increased to 2Gi

**Command**:
```bash
kubectl patch daemonset fortuna-agent -n fortuna --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/resources/limits/memory", "value": "2Gi"}]'
```

**Evidence**: Agent had 3 restarts with `OOMKilled` exitCode before fix

### 3. Pod Watcher Performance Improvements ✅ FIXED (from earlier work)
From the `AGENT_POD_WATCHER_FIX.md`:
- Reduced resync period: 30s → 5s
- Enhanced AddFunc with comprehensive logging
- Enhanced UpdateFunc to catch all Running pods
- Added post-sync check for pods added during startup
- Added duplicate prevention with processedPods map
- Added memory leak prevention in DeleteFunc

## Remaining Issue: SBOM Extraction Blocks Pod Detection ⚠️ CRITICAL

### Problem Description
The agent successfully detects pods in all namespaces, BUT SBOM extraction is synchronous and extremely slow (2-3 minutes per pod), which blocks the informer's event processing queue.

### Impact
- Agent startup processes 15+ existing pods
- Each pod takes 2-3 minutes for SBOM extraction
- Total startup time: 30-45 minutes of blocked event processing
- **New pods created during this time are NOT detected until the backlog clears**
- E2E tests fail because they create pods while agent is processing startup backlog

### Evidence
```
# SBOM extraction times from logs:
[SBOMExtractor] 2025/12/26 04:11:10 ✅ Extracted 18 unique packages in 2m47.598977451s
[SBOMExtractor] 2025/12/26 04:08:00 ✅ Extracted 187 unique packages in 2m14.115880146s

# Informer queue blocking:
I1226 04:29:16.839789 Trace[1642499826]: "DeltaFIFO Pop Process"
ID:fortuna/nats-1,Depth:12,Reason:slow event handlers blocking the queue
```

### Root Cause Analysis

**Current Flow (BLOCKING)**:
```
1. Pod event arrives at informer
2. AddFunc/UpdateFunc called synchronously
3. w.handler(ctx, pod) called directly
4. SBOM extraction runs (2-3 minutes)
5. Event handler returns
6. Next event can be processed
```

**Code Location**: `agent/internal/watcher/pod_watcher_local.go:83-87`
```go
AddFunc: func(obj interface{}) {
    pod := obj.(*corev1.Pod)
    // ...
    if err := w.handler(ctx, pod); err != nil { // BLOCKS HERE
        w.logger.Printf("⚠️  Handler error...")
    } else {
        w.processedPods[podUID] = true
    }
}
```

### Recommended Solution

Implement asynchronous processing with a work queue:

**New Flow (NON-BLOCKING)**:
```
1. Pod event arrives at informer
2. AddFunc/UpdateFunc called synchronously
3. Pod added to work queue (fast, non-blocking)
4. Event handler returns immediately
5. Next event can be processed
6. Background workers process queue asynchronously
```

**Implementation Approach**:
1. Add workqueue.RateLimitingInterface to LocalPodWatcher
2. Event handlers enqueue pod UIDs instead of processing directly
3. Start N worker goroutines to process queue
4. Workers call w.handler() asynchronously
5. Maintain processedPods map for duplicate prevention

**Example Skeleton**:
```go
import "k8s.io/client-go/util/workqueue"

type LocalPodWatcher struct {
    ...
    workqueue     workqueue.RateLimitingInterface
    processedPods map[string]bool
}

// AddFunc just enqueues, doesn't process
AddFunc: func(obj interface{}) {
    pod := obj.(*corev1.Pod)
    w.logger.Printf("🆕 Pod added: %s/%s", pod.Namespace, pod.Name)

    if pod.Status.Phase == corev1.PodRunning {
        podUID := string(pod.UID)
        if !w.processedPods[podUID] {
            w.workqueue.Add(podUID)
        }
    }
}

// Worker processes queue asynchronously
func (w *LocalPodWatcher) worker() {
    for w.processNextItem() {
    }
}

func (w *LocalPodWatcher) processNextItem() bool {
    obj, shutdown := w.workqueue.Get()
    if shutdown {
        return false
    }
    defer w.workqueue.Done(obj)

    podUID := obj.(string)
    // Get pod from informer cache and process
    // ...
    w.handler(ctx, pod)
    w.processedPods[podUID] = true
}
```

### Alternative Quick Fix (Temporary)

Increase the number of concurrent SBOM extractions or skip processing existing pods at startup:

**Option A**: Skip startup check
```go
// Comment out the post-sync check that processes existing pods
// w.logger.Printf("📋 Checking for pods that may have been missed during startup...")
// ...
```

**Option B**: Add startup delay to E2E tests
```bash
# Wait for agent to finish processing startup pods
echo "Waiting for agent startup to complete..."
sleep 60
```

## Test Results

### Before All Fixes
- ✅ Pods created successfully
- ✅ Pods scheduled on correct node
- ✅ Pods reached Running state
- ❌ Agent not detecting pods (namespace issue)
- ❌ Agent OOMKilled crashes
- ❌ E2E tests: 0/5 passed

### After Namespace + Memory Fixes
- ✅ Pods created successfully
- ✅ Pods scheduled on correct node
- ✅ Pods reached Running state
- ✅ Agent detecting pods in all namespaces
- ✅ Agent stable (no crashes)
- ❌ Agent blocked by slow SBOM extraction
- ❌ E2E tests: 0/5 passed (timeout waiting for SBOM)

### After Async Processing (NOT YET IMPLEMENTED)
- Expected: All E2E tests pass

## Files Modified

1. **`agent/internal/watcher/pod_watcher_local.go`**
   - Added `informers.WithNamespace(metav1.NamespaceAll)`
   - (Earlier fixes: resync period, event handlers, duplicate prevention)

2. **Kubernetes DaemonSet**
   - Increased memory limit: 1Gi → 2Gi

## Next Steps

### Priority 1: Make SBOM Extraction Async
- [ ] Add workqueue to LocalPodWatcher
- [ ] Modify event handlers to enqueue instead of process
- [ ] Add worker goroutines
- [ ] Test with E2E suite

### Priority 2: Optimize SBOM Extraction Performance
- [ ] Investigate why extraction takes 2-3 minutes
- [ ] Add caching for already-processed images
- [ ] Parallelize parser execution
- [ ] Skip redundant extractions for same image

### Priority 3: Improve Agent Startup
- [ ] Process only newly created pods, not all existing
- [ ] Or defer existing pod processing until after informer is fully synced
- [ ] Add health check endpoint to signal "ready to process new pods"

## Conclusion

**Good News**:
- ✅ Pod watcher CAN detect pods in all namespaces (ksam-test, fortuna, kube-system, etc.)
- ✅ All logging improvements working perfectly
- ✅ Duplicate prevention working
- ✅ Agent no longer crashing due to OOM

**Bad News**:
- ❌ Synchronous SBOM extraction blocks pod detection
- ❌ E2E tests still failing due to processing backlog

**Recommendation**: Implement async work queue pattern before running E2E tests again.

---
**Date**: 2025-12-26
**Status**: Namespace and memory issues resolved, async processing needed
**E2E Test Pass Rate**: 0/5 (blocked by sync SBOM extraction)
