# mTLS Traffic Generation Test

**Date**: 2025-12-01  
**Purpose**: Generate actual traffic between Agent and Core for mTLS encryption testing

---

## Overview

This test creates test Kubernetes resources to trigger Agent to stream data to Core, ensuring there is actual traffic during packet capture tests.

---

## Test Script: `generate_test_traffic.sh`

### Purpose
Creates test resources that trigger Agent's watchers, causing it to stream inventory data to Core via mTLS.

### Resources Created

1. **Test Pod**
   - Name: `ksam-mtls-test-<timestamp>-pod`
   - Image: `busybox:latest`
   - Purpose: Triggers PodWatcher

2. **Test ServiceAccount**
   - Name: `ksam-mtls-test-<timestamp>-sa`
   - Purpose: Triggers ServiceAccountWatcher

3. **Test Role**
   - Name: `ksam-mtls-test-<timestamp>-role`
   - Purpose: Triggers RoleWatcher

4. **Test RoleBinding**
   - Name: `ksam-mtls-test-<timestamp>-binding`
   - Purpose: Triggers RoleBindingWatcher

---

## Usage

### Standalone Usage

Generate traffic before running packet capture:

```bash
# Step 1: Generate test traffic
./scripts/generate_test_traffic.sh

# Step 2: Run packet capture test (while traffic is active)
./scripts/test_mtls_traffic_with_debug_pod.sh
```

### Integrated Usage

The `test_mtls_traffic_with_debug_pod.sh` script now automatically creates test resources before capturing traffic.

---

## Expected Behavior

### Agent Response

After creating test resources, Agent should:
1. Detect new resources via Kubernetes Watch API
2. Collect resource data
3. Stream to Core via gRPC with mTLS
4. Log: "Successfully streamed 1 inventory items to core"

### Traffic Flow

```
Test Resource Created
    ↓
Agent Watcher Detects
    ↓
Agent Collects Data
    ↓
Agent Streams to Core (mTLS)
    ↓
Core Receives & Processes
    ↓
Traffic Visible in Packet Capture
```

---

## Verification

### Check Agent Streaming

```bash
kubectl logs -n ksam -l app=ksam-agent --tail=20 --since=10s | grep "Successfully streamed"
```

### Check Core Processing

```bash
kubectl logs -n ksam -l app=ksam-core --tail=20 | grep -E "received|Ingest|ksam.raw"
```

### Check Database

```bash
kubectl exec -n ksam <postgres-pod> -- psql -U ksam -d ksam -c "
  SELECT name, namespace FROM pods WHERE name LIKE 'ksam-mtls-test%';
"
```

---

## Test Results

### Successful Traffic Generation

**Indicators**:
- ✅ Test resources created
- ✅ Agent logs show "Successfully streamed"
- ✅ Packet capture shows traffic
- ✅ Database contains test resources

**Example Output**:
```
✅ Test pod created: ksam-mtls-test-1234567890-pod
✅ Agent is streaming (detected 4 messages in last 10s)
✅ Traffic generated: 4 messages in last 15s
```

---

## Cleanup

Test resources are automatically cleaned up:
- When test script exits (trap cleanup)
- Manually: `kubectl delete pod -n ksam -l app=ksam-mtls-test`

---

## Troubleshooting

### Issue: Agent not streaming

**Possible Causes**:
- Agent not running
- RBAC permissions missing
- Network issues

**Solution**:
1. Check Agent pod status
2. Check Agent logs for errors
3. Verify RBAC permissions
4. Check network connectivity

### Issue: No traffic in capture

**Possible Causes**:
- Traffic generated before capture started
- Traffic generated after capture ended
- Network policy blocking

**Solution**:
1. Run `generate_test_traffic.sh` first
2. Start capture, then generate traffic
3. Increase capture duration
4. Check network policies

---

## Best Practices

1. **Generate traffic before capture**: Run traffic generation script first
2. **Monitor Agent logs**: Verify streaming is happening
3. **Use appropriate timing**: Allow time for Agent to detect and stream
4. **Clean up resources**: Ensure test resources are removed

---

## Integration with Packet Capture

### Recommended Workflow

1. **Start packet capture** (in background)
2. **Generate test traffic** (while capture is running)
3. **Wait for traffic to flow**
4. **Stop capture**
5. **Analyze packets**

### Example

```bash
# Terminal 1: Start capture
kubectl exec -n ksam <debug-pod> -- tcpdump -i any -w /tmp/capture.pcap port 9090

# Terminal 2: Generate traffic
./scripts/generate_test_traffic.sh

# Wait 10-15 seconds

# Terminal 1: Stop capture (Ctrl+C)
# Analyze: tcpdump -r /tmp/capture.pcap -A
```

---

**Status**: Ready for use


