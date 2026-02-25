# Agent on Worker Node - Debug Report
**Date:** 2026-01-07  
**Time:** $(date +%H:%M:%S)

## Executive Summary

This report details the debugging process for Agent on worker node connection issues to Core. The Agent on master node connects successfully, but Agent on worker node fails with DNS timeout errors.

## Problem Statement

**Agent on Worker Node:** `fortuna-agent-2zqfj`
- **Status:** Running
- **Issue:** Cannot connect to Core
- **Error:** `dial tcp: lookup fortuna-core.fortuna.svc.cluster.local on 10.96.0.10:53: read udp ...: i/o timeout`

**Agent on Master Node:** `fortuna-agent-r48fr`
- **Status:** Running
- **Connection:** ✅ SUCCESS
- **Heartbeat:** Regular successful pings

## Debugging Steps

### Step 1: DNS Resolution Tests

**Test Results:**
- ✅ `getent hosts fortuna-core.fortuna.svc.cluster.local`: SUCCESS (returns IP: 10.100.74.197)
- ✅ DNS resolution works at OS level
- ❌ But gRPC connection fails with DNS timeout

**Observation:**
- DNS resolution succeeds when tested directly
- But fails during gRPC connection establishment
- This suggests DNS query timeout during connection handshake

### Step 2: Network Connectivity Tests

**Test Results:**
- ❌ TCP connection to Core Service IP (10.100.74.197:9090): FAILED (timeout)
- ❌ TCP connection to Core Pod IP (10.244.0.194:9090): FAILED (timeout)
- ❌ TCP connection to Core Service IP (10.100.74.197:8080): FAILED (timeout)

**Observation:**
- Direct TCP connections fail with timeout
- This indicates network routing issue between worker and master nodes
- Not just a DNS issue, but actual network connectivity problem

### Step 3: CoreDNS Configuration

**Current Configuration:**
```
forward . /etc/resolv.conf {
   except cluster.local
   except in-addr.arpa
   except ip6.arpa
   max_concurrent 1000
}
```

**CoreDNS Logs:**
- Still showing external DNS query timeouts: `read udp ...->8.8.8.8:53: i/o timeout`
- External DNS queries are timing out
- This may affect internal DNS resolution performance

**Observation:**
- CoreDNS ConfigMap was updated
- But CoreDNS pods may not have reloaded the configuration
- Or external DNS timeout is affecting internal queries

### Step 4: Node Network Topology

**Nodes:**
- Agent Worker Node: `k8s-worker01`
- Core Pod Node: `k8s-master`
- **Different nodes:** ✅ YES

**Observation:**
- Agent and Core are on different nodes
- Network routing between nodes may be the issue
- Flannel/CNI network may not be routing correctly

### Step 5: Comparison with Master Agent

**Master Agent:**
- ✅ DNS resolution: SUCCESS
- ✅ TCP connection: SUCCESS
- ✅ Heartbeat: Regular successful pings
- **Node:** `k8s-master` (same as Core)

**Worker Agent:**
- ✅ DNS resolution: SUCCESS (getent hosts works)
- ❌ TCP connection: FAILED (timeout)
- ❌ Heartbeat: Fails with DNS timeout
- **Node:** `k8s-worker01` (different from Core)

**Observation:**
- Master Agent works because it's on the same node as Core
- Worker Agent fails because it needs to route through network
- This confirms network routing issue between nodes

## Root Cause Analysis

### Primary Root Cause: Network Routing Issue

**Evidence:**
1. DNS resolution works (getent hosts succeeds)
2. But TCP connections fail with timeout
3. Master Agent works (same node as Core)
4. Worker Agent fails (different node from Core)

**Conclusion:**
- Not a DNS resolution issue (DNS works)
- Network routing between worker and master nodes is blocked or slow
- Flannel/CNI network may not be configured correctly
- Or firewall/network policy is blocking inter-node traffic

### Secondary Issue: CoreDNS External DNS Timeout

**Evidence:**
- CoreDNS logs show external DNS query timeouts
- External DNS queries to 8.8.8.8/8.8.4.4 are timing out
- This may affect internal DNS resolution performance

**Impact:**
- May cause DNS queries to timeout during connection establishment
- Even though `getent hosts` works, DNS queries during gRPC handshake may timeout

## Solutions

### Solution 1: Fix Network Routing (PRIORITY)

**Action:** Verify and fix Flannel/CNI network configuration

1. Check Flannel pod status:
   ```bash
   kubectl get pods -n kube-flannel
   kubectl logs -n kube-flannel <flannel-pod>
   ```

2. Check network routes on nodes:
   ```bash
   # On master node
   ip route show
   
   # On worker node
   ssh k8s@k8s-worker01 "ip route show"
   ```

3. Test connectivity between nodes:
   ```bash
   # From worker node, ping master node
   ping <master-node-ip>
   
   # From worker node, test Core pod IP
   telnet <core-pod-ip> 9090
   ```

4. Check CNI configuration:
   ```bash
   # Check CNI config on nodes
   cat /etc/cni/net.d/*
   ```

### Solution 2: Fix CoreDNS External DNS (SECONDARY)

**Action:** Ensure CoreDNS doesn't forward internal queries externally

1. Verify CoreDNS ConfigMap:
   ```bash
   kubectl get configmap coredns -n kube-system -o yaml
   ```

2. Ensure `except` clauses are correct:
   ```
   forward . /etc/resolv.conf {
      except cluster.local
      except in-addr.arpa
      except ip6.arpa
   }
   ```

3. Restart CoreDNS pods:
   ```bash
   kubectl rollout restart deployment coredns -n kube-system
   ```

4. Verify CoreDNS reloaded config:
   ```bash
   kubectl logs -n kube-system <coredns-pod> | grep "reload"
   ```

### Solution 3: Use Direct Pod IP (TEMPORARY WORKAROUND)

**Action:** Configure Agent to use Core pod IP directly

1. Get Core pod IP:
   ```bash
   kubectl get endpoints fortuna-core -n fortuna -o jsonpath='{.subsets[0].addresses[0].ip}'
   ```

2. Update Agent DaemonSet to use pod IP:
   - Modify `FORTUNA_CORE_ENDPOINT` environment variable
   - Use pod IP instead of service DNS name
   - **Note:** This is not recommended for production (pod IPs change)

## Recommended Action Plan

1. **Immediate:** Verify network routing between nodes
   - Check Flannel/CNI pod status
   - Test connectivity between nodes
   - Verify network routes

2. **Short-term:** Fix CoreDNS external DNS forwarding
   - Verify CoreDNS ConfigMap
   - Restart CoreDNS pods
   - Monitor CoreDNS logs

3. **Long-term:** Ensure proper CNI network configuration
   - Verify Flannel/CNI is working correctly
   - Check network policies
   - Test inter-node connectivity

## Test Results Summary

| Test | Result | Details |
|------|--------|---------|
| DNS Resolution (getent) | ✅ SUCCESS | Returns IP: 10.100.74.197 |
| DNS Resolution (gRPC) | ❌ TIMEOUT | Fails during connection |
| TCP to Service IP | ❌ TIMEOUT | Cannot connect |
| TCP to Pod IP | ❌ TIMEOUT | Cannot connect |
| CoreDNS Reachable | ⏳ UNKNOWN | Need to verify |
| Inter-node Routing | ❌ FAILED | Network routing issue |

## Conclusion

**Root Cause:** Network routing issue between worker and master nodes

The Agent on worker node cannot connect to Core because:
1. Network routing between nodes is not working correctly
2. TCP connections timeout, indicating blocked or slow routing
3. DNS resolution works, but connection establishment fails

**Next Steps:**
1. Verify Flannel/CNI network configuration
2. Test connectivity between nodes
3. Fix network routing if needed
4. Verify CoreDNS configuration
5. Retest Agent connection

---

**Status:** 🔴 BLOCKED - Network routing issue between nodes


## UPDATE - Root Cause Confirmed

**Time:** $(date +%H:%M:%S)

### PRIMARY ROOT CAUSE: Flannel VXLAN Tunnel Not Properly Configured

**Evidence:**
- ✅ Flannel pods are running
- ✅ Flannel subnets configured: 10.244.0.0/24 (master), 10.244.1.0/24 (worker)
- ✅ VXLAN interfaces exist (flannel.1 on both nodes)
- ❌ VXLAN interfaces have NO IPv4 addresses (only IPv6 link-local)
- ❌ No routes between subnets
- ❌ Pod-to-pod connectivity fails between nodes

**Network Routes:**
- Master node: Only local route `10.244.0.0/24 dev cni0`
- Worker node: Only local route `10.244.1.0/24 dev cni0`
- **Missing:** Routes between subnets via VXLAN tunnel

**This confirms:**
- Flannel VXLAN tunnel is not working
- Pods on different nodes cannot communicate
- Network routing between nodes is broken

### Solution Required

**Fix Flannel VXLAN Configuration:**
1. Check Flannel ConfigMap for correct network configuration
2. Verify Flannel subnet allocation in etcd/kube-api
3. Restart Flannel pods to reinitialize VXLAN
4. Verify VXLAN interfaces get IPv4 addresses
5. Verify routes between subnets are created

**Expected Result:**
- VXLAN interfaces should have IPv4 addresses
- Routes should exist: `10.244.0.0/24 via <vxlan-ip> dev flannel.1`
- Pod-to-pod connectivity should work between nodes

