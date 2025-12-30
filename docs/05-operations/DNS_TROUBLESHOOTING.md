# DNS Troubleshooting Guide

**Date:** December 29, 2025  
**Status:** Active  
**Related Issues:** Core database connection, Agent Core connection

---

## 🔴 Problem Statement

DNS resolution failures affecting:
- Core → PostgreSQL connection
- Agent → Core gRPC connection
- NATS cluster formation

**Symptoms:**
```
hostname resolving error (lookup <service>.fortuna.svc.cluster.local on 10.96.0.10:53: 
read udp <pod-ip>:<port>->10.96.0.10:53: i/o timeout)
```

---

## 🔍 Diagnosis Steps

### Step 1: Check CoreDNS Pods

```bash
# Check CoreDNS pods status
kubectl get pods -n kube-system -l k8s-app=kube-dns -o wide

# Check CoreDNS logs
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=50

# Check CoreDNS restart count
kubectl get pods -n kube-system -l k8s-app=kube-dns | grep -v "RESTARTS" | awk '{print $4}'
```

**Expected:**
- All CoreDNS pods should be `Running`
- Restart count should be low (< 10)
- No errors in logs

**Issues Found:**
- High restart count (228, 229) → CoreDNS unstable
- Pods on master node only → May not be reachable from workers

---

### Step 2: Check CoreDNS Service

```bash
# Check CoreDNS service
kubectl get svc -n kube-system kube-dns

# Check CoreDNS endpoints
kubectl get endpoints -n kube-system kube-dns
```

**Expected:**
- Service ClusterIP: `10.96.0.10` (or similar)
- Endpoints should have CoreDNS pod IPs

---

### Step 3: Test DNS Resolution

```bash
# From a test pod
kubectl run dns-test --image=busybox:1.36 --rm -i --restart=Never -- \
  nslookup postgres.fortuna.svc.cluster.local

# Test with CoreDNS directly
kubectl run dns-test --image=busybox:1.36 --rm -i --restart=Never -- \
  nslookup postgres.fortuna.svc.cluster.local 10.96.0.10
```

**Expected:**
- DNS resolution should return IP address
- No timeout errors

---

### Step 4: Check Network Connectivity

```bash
# From application pod to CoreDNS
kubectl exec -n fortuna <pod-name> -- ping -c 3 10.96.0.10

# Test UDP port 53
kubectl exec -n fortuna <pod-name> -- nc -u -v -w 2 10.96.0.10 53
```

**Expected:**
- Ping should succeed
- UDP port 53 should be reachable

---

### Step 5: Check DNS Config in Pods

```bash
# Check /etc/resolv.conf
kubectl exec -n fortuna <pod-name> -- cat /etc/resolv.conf
```

**Expected:**
```
nameserver 10.96.0.10
search fortuna.svc.cluster.local svc.cluster.local cluster.local
```

---

## 🔧 Solutions

### Solution 1: Restart CoreDNS

```bash
# Restart CoreDNS deployment
kubectl rollout restart deployment -n kube-system coredns

# Wait for CoreDNS to be ready
kubectl wait --for=condition=ready pod -n kube-system -l k8s-app=kube-dns --timeout=120s
```

---

### Solution 2: Check Network Policies

```bash
# Check for NetworkPolicies blocking DNS
kubectl get networkpolicies -n fortuna
kubectl get networkpolicies -A | grep -i dns

# If found, temporarily disable to test
kubectl delete networkpolicy -n fortuna --all
```

---

### Step 3: Verify Node Network

```bash
# Check node network connectivity
# From worker node
ping -c 3 <master-ip>
ping -c 3 10.96.0.10

# Check flannel/CNI plugin
kubectl get pods -n kube-system | grep -E "flannel|calico|weave"
```

---

### Solution 4: Workaround - Use IP Addresses

**For Core:**
```bash
POSTGRES_IP=$(kubectl get svc -n fortuna postgres -o jsonpath='{.spec.clusterIP}')
kubectl set env deployment/fortuna-core -n fortuna \
  DATABASE_URL="postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam?sslmode=disable"
```

**For Agent:**
```bash
CORE_IP=$(kubectl get svc -n fortuna fortuna-core -o jsonpath='{.spec.clusterIP}')
kubectl set env daemonset/fortuna-agent -n fortuna \
  CORE_GRPC_ENDPOINT="${CORE_IP}:9090"
```

**⚠️ Warning:** This is a temporary workaround. Fix DNS for production.

---

## 🎯 Root Cause Analysis

### Possible Causes

1. **CoreDNS Pod Issues:**
   - High restart count indicates instability
   - Pods may be crashing and restarting
   - Resource constraints (CPU/memory)

2. **Network Connectivity:**
   - Worker nodes cannot reach CoreDNS on master
   - Firewall rules blocking DNS traffic
   - CNI plugin issues (flannel/calico)

3. **CoreDNS Configuration:**
   - Incorrect upstream DNS servers
   - Forwarding issues
   - Cache problems

4. **Node Network:**
   - Network plugin not properly configured
   - Routing issues between nodes
   - MTU mismatches

---

## 📋 Checklist

### Pre-Deployment DNS Check

- [ ] CoreDNS pods are Running
- [ ] CoreDNS restart count < 10
- [ ] CoreDNS service has endpoints
- [ ] DNS resolution works from test pod
- [ ] Network connectivity to CoreDNS IP
- [ ] No NetworkPolicies blocking DNS
- [ ] Node network connectivity verified

### Post-Deployment DNS Check

- [ ] Application pods can resolve service names
- [ ] No DNS timeout errors in logs
- [ ] Service discovery working
- [ ] Cross-namespace resolution working

---

## 🔄 Permanent Fix Strategy

### Phase 1: Diagnose
1. Check CoreDNS logs for errors
2. Verify network plugin configuration
3. Test connectivity between nodes
4. Check firewall rules

### Phase 2: Fix
1. Fix CoreDNS configuration if needed
2. Resolve network connectivity issues
3. Update NetworkPolicies if blocking
4. Restart CoreDNS

### Phase 3: Verify
1. Test DNS resolution from all nodes
2. Verify service discovery
3. Monitor CoreDNS metrics
4. Update documentation

---

## 📚 Related Issues

- [Deployment Issues Analysis](./DEPLOYMENT_ISSUES_ANALYSIS.md)
- [Core Connection Troubleshooting](./CORE_CONNECTION_TROUBLESHOOTING.md)
- [Agent Deployment Troubleshooting](./AGENT_DEPLOYMENT_TROUBLESHOOTING.md)

---

**Last Updated:** December 29, 2025

