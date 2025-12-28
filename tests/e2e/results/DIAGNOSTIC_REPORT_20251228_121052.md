# Diagnostic Report - SBOM Processing Issue

**Date**: Sun Dec 28 12:10:52 +07 2025  
**Test Pod**: test-pod-e2e-1766897554  
**Pod UID**: 9caa6290-5471-46de-9a0b-a43ba7937d9d

---

## Issue Summary

Database không có thông tin SBOM và API insights không có kết quả cho test pod.

---

## Current Status

### Test Pod
    NAME                      READY   STATUS    RESTARTS   AGE
    test-pod-e2e-1766897554   1/1     Running   0          18m

### Database
    SBOM count:      1
    SBOM count: 

### Agent Logs
    - [SBOMQueue] 2025/12/28 05:07:03 [Worker 2] Processing pod fortuna/test-pod-e2e-1766897554
    - [SBOMProcessor] 2025/12/28 05:07:03 Processing pod fortuna/test-pod-e2e-1766897554 on node minikube
    - [SBOMProcessor] 2025/12/28 05:07:03 🔍 Extracting SBOM: pod=fortuna/test-pod-e2e-1766897554 container=test-container image=nginx:1.25-alpine
    - [gRPCClient] 2025/12/28 05:09:33 Sending SBOM: pod=fortuna/test-pod-e2e-1766897554 image=sha256:e0bdce2b6eda0c42428bdf653481a3853086197221e1d374923d75677335f9e9
    - [SBOMQueue] 2025/12/28 05:09:33 [Worker 2] ✅ Completed pod fortuna/test-pod-e2e-1766897554 in 2m30.143300819s

### Queue Status
    - [SBOMQueue] 2025/12/28 05:07:03 [Worker 2] ✅ Completed pod kube-system/storage-provisioner in 1m29.242014248s
    - [SBOMQueue] 2025/12/28 05:07:03 [Worker 2] Processing pod fortuna/test-pod-e2e-1766897554
    - [SBOMQueue] 2025/12/28 05:09:18 [Worker 0] ✅ Completed pod kube-system/kube-scheduler-minikube in 3m59.372286567s
    - [SBOMQueue] 2025/12/28 05:09:26 [Worker 1] ✅ Completed pod fortuna/postgres-747fc6cdfb-dh64f in 17m7.620439802s
    - [SBOMQueue] 2025/12/28 05:09:33 [Worker 2] ✅ Completed pod fortuna/test-pod-e2e-1766897554 in 2m30.143300819s

---

## Analysis

### Possible Causes

1. **Queue Backlog**: Test pod may be queued behind other pods
2. **SBOM Extraction Delay**: Extraction may take 1-5 minutes per pod
3. **Queue Processing Issue**: Worker may not be processing test pod
4. **Pod Not Detected**: Agent may not have detected the pod correctly

---

## Recommendations

1. Check if test pod is still in queue
2. Monitor Agent logs for SBOM extraction activity
3. Verify queue workers are active
4. Check for any errors in Agent/Core logs

---

**Report Generated**: Sun Dec 28 12:10:52 +07 2025
