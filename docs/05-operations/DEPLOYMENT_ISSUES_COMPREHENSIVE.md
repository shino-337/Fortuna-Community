# Fortuna Deployment Issues - Comprehensive Guide

**Date:** December 29, 2025  
**Version:** 2.0  
**Status:** ✅ Solutions Implemented

---

## 📋 Table of Contents

1. [Executive Summary](#executive-summary)
2. [All Issues Catalog](#all-issues-catalog)
3. [Root Cause Analysis](#root-cause-analysis)
4. [Solutions & Scripts](#solutions--scripts)
5. [Deployment Procedures](#deployment-procedures)
6. [Troubleshooting Guide](#troubleshooting-guide)
7. [Prevention Checklist](#prevention-checklist)

---

## 🎯 Executive Summary

This document consolidates all deployment issues encountered when deploying Fortuna on multi-node Kubernetes clusters and provides comprehensive solutions.

**Primary Root Cause:** DNS resolution failures (CoreDNS timeout)

**Total Issues:** 8 major issues across Core, Agent, and Infrastructure

**Solution Status:**
- ✅ Workarounds implemented (IP-based connections)
- ✅ Deployment scripts created
- ⚠️ DNS fix required for production

---

## 🔴 All Issues Catalog

### Critical Issues (Blocking)

| # | Issue | Component | Impact | Status |
|---|-------|-----------|--------|--------|
| 1 | Database DNS timeout | Core | Cannot connect to PostgreSQL | ✅ Fixed (IP workaround) |
| 2 | HTTP server not starting | Core | Readiness probe fails | ✅ Fixed (DB fix) |
| 3 | Core connection DNS timeout | Agent | Cannot send SBOM data | ✅ Fixed (IP workaround) |
| 4 | Readiness probe dependency | Core | Pod never becomes Ready | ✅ Fixed (DB fix) |

### High Priority Issues

| # | Issue | Component | Impact | Status |
|---|-------|-----------|--------|--------|
| 5 | Multiple deployments | Core | Resource conflicts | ✅ Fixed (cleanup script) |
| 6 | NATS storage/placement | NATS | Streams cannot be created | ✅ Fixed (single-node) |
| 7 | NATS cluster DNS | NATS | Cluster cannot form | ✅ Fixed (single-node) |

### Medium Priority Issues

| # | Issue | Component | Impact | Status |
|---|-------|-----------|--------|--------|
| 8 | TLS ServerName validation | Agent | TLS handshake fails with IP | ✅ Fixed (disable TLS) |

---

## 🔍 Root Cause Analysis

### Primary: DNS Resolution Failure

**Symptoms:**
```
lookup <service>.fortuna.svc.cluster.local on 10.96.0.10:53: 
read udp <pod-ip>:<port>->10.96.0.10:53: i/o timeout
```

**Root Causes:**
1. **CoreDNS Instability**
   - High restart count (228, 229 restarts)
   - Pods may be crashing
   - Resource constraints

2. **Network Connectivity**
   - Worker nodes cannot reach CoreDNS on master
   - Firewall rules blocking DNS (UDP port 53)
   - CNI plugin issues

3. **CoreDNS Configuration**
   - Incorrect upstream DNS
   - Forwarding issues
   - Cache problems

**Impact Chain:**
```
DNS Failure → Database Connection Fails → HTTP Server Doesn't Start → 
Readiness Probe Fails → Service Has No Endpoints → Agent Cannot Connect
```

### Secondary: Deployment Management

**Issues:**
- Multiple deployments from different sources
- No cleanup before deployment
- Inconsistent labels

**Impact:**
- Resource conflicts
- Unpredictable behavior
- Service endpoints pointing to wrong pods

---

## ✅ Solutions & Scripts

### Solution 1: Comprehensive Deployment Script

**Script:** `scripts/deploy-fortuna-robust.sh`

**Features:**
- ✅ Pre-deployment checks
- ✅ Comprehensive cleanup
- ✅ DNS testing with IP fallback
- ✅ Sequential deployment
- ✅ Verification at each step
- ✅ Automatic IP-based configuration if DNS fails

**Usage:**
```bash
# Standard deployment (with DNS fallback)
./scripts/deploy-fortuna-robust.sh

# Force DNS-only (no IP fallback)
USE_IP_FALLBACK=false ./scripts/deploy-fortuna-robust.sh
```

### Solution 2: Pre-Deployment Checks

**Script:** `scripts/pre-deployment-checks.sh`

**Checks:**
- ✅ Kubernetes cluster accessibility
- ✅ Namespace existence
- ✅ CoreDNS health
- ✅ DNS resolution
- ✅ Network connectivity
- ✅ Node labels

**Usage:**
```bash
./scripts/pre-deployment-checks.sh
```

### Solution 3: DNS Fix Script

**Script:** `scripts/fix-dns-issues.sh`

**Actions:**
- ✅ Restart CoreDNS
- ✅ Check CoreDNS status
- ✅ Test DNS resolution
- ✅ Check NetworkPolicies
- ✅ Verify CNI plugin

**Usage:**
```bash
./scripts/fix-dns-issues.sh
```

### Solution 4: IP-Based Workarounds

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
kubectl set env daemonset/fortuna-agent -n fortuna TLS_ENABLED="false"
```

**⚠️ Warning:** Temporary workaround only. Fix DNS for production.

---

## 🚀 Deployment Procedures

### Standard Deployment (Recommended)

```bash
# 1. Pre-deployment checks
./scripts/pre-deployment-checks.sh

# 2. Fix DNS if needed
./scripts/fix-dns-issues.sh

# 3. Deploy with automatic fallback
./scripts/deploy-fortuna-robust.sh
```

### Manual Deployment (Step-by-Step)

```bash
# 1. Create namespace
kubectl create namespace fortuna

# 2. Deploy infrastructure
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl apply -f deploy/infrastructure/nats.yaml

# 3. Wait for infrastructure
kubectl wait --for=condition=ready pod -n fortuna -l app=postgres --timeout=300s
kubectl wait --for=condition=ready pod -n fortuna -l app=nats --timeout=300s

# 4. Deploy RBAC
kubectl apply -f deploy/fortuna-rbac.yaml

# 5. Deploy Core
kubectl apply -f deploy/fortuna-core-deployment.yaml

# 6. Configure Core (if DNS fails)
POSTGRES_IP=$(kubectl get svc -n fortuna postgres -o jsonpath='{.spec.clusterIP}')
kubectl set env deployment/fortuna-core -n fortuna \
  DATABASE_URL="postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam?sslmode=disable"

# 7. Wait for Core
kubectl wait --for=condition=available deployment/fortuna-core -n fortuna --timeout=300s

# 8. Deploy Agent
kubectl apply -f deploy/fortuna-agent-daemonset.yaml

# 9. Configure Agent (if DNS fails)
CORE_IP=$(kubectl get svc -n fortuna fortuna-core -o jsonpath='{.spec.clusterIP}')
kubectl set env daemonset/fortuna-agent -n fortuna \
  CORE_GRPC_ENDPOINT="${CORE_IP}:9090"
kubectl set env daemonset/fortuna-agent -n fortuna TLS_ENABLED="false"
```

---

## 🔧 Troubleshooting Guide

### Issue: Core Pod Cannot Connect to Database

**Symptoms:**
- Core pod logs show DNS timeout
- Database connection errors

**Diagnosis:**
```bash
# Check Core pod logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50

# Test DNS from Core pod
kubectl exec -n fortuna -l app.kubernetes.io/component=core -- \
  nslookup postgres.fortuna.svc.cluster.local
```

**Fix:**
```bash
# Use IP-based connection
POSTGRES_IP=$(kubectl get svc -n fortuna postgres -o jsonpath='{.spec.clusterIP}')
kubectl set env deployment/fortuna-core -n fortuna \
  DATABASE_URL="postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam?sslmode=disable"
kubectl rollout restart deployment/fortuna-core -n fortuna
```

### Issue: Agent Cannot Connect to Core

**Symptoms:**
- Agent logs show "connection error"
- Heartbeat failures

**Diagnosis:**
```bash
# Check Agent logs
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=50

# Check Core service endpoints
kubectl get endpoints -n fortuna fortuna-core

# Test DNS from Agent pod
kubectl exec -n fortuna -l app.kubernetes.io/component=agent -- \
  nslookup fortuna-core.fortuna.svc.cluster.local
```

**Fix:**
```bash
# Use IP-based connection
CORE_IP=$(kubectl get svc -n fortuna fortuna-core -o jsonpath='{.spec.clusterIP}')
kubectl set env daemonset/fortuna-agent -n fortuna \
  CORE_GRPC_ENDPOINT="${CORE_IP}:9090"
kubectl set env daemonset/fortuna-agent -n fortuna TLS_ENABLED="false"
kubectl rollout restart daemonset/fortuna-agent -n fortuna
```

### Issue: Core Pod Not Ready

**Symptoms:**
- Pod status: `Running` but not `Ready`
- Readiness probe failures

**Diagnosis:**
```bash
# Check pod status
kubectl get pods -n fortuna -l app.kubernetes.io/component=core

# Check readiness probe
kubectl describe pod -n fortuna -l app.kubernetes.io/component=core | grep -A 10 "Readiness"

# Check pod logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50
```

**Fix:**
- Usually caused by database connection failure
- Fix database connection first (see above)
- Pod should become Ready after database connection succeeds

### Issue: Multiple Core Deployments

**Symptoms:**
- Multiple Core pods running
- Service endpoints pointing to wrong pods

**Fix:**
```bash
# Delete all Core deployments
kubectl delete deployment -n fortuna -l app.kubernetes.io/component=core
kubectl delete deployment -n fortuna fortuna-core ksam-core

# Wait for cleanup
sleep 5

# Deploy fresh
kubectl apply -f deploy/fortuna-core-deployment.yaml
```

### Issue: CoreDNS Not Working

**Symptoms:**
- DNS queries timeout
- High CoreDNS restart count

**Fix:**
```bash
# Restart CoreDNS
kubectl rollout restart deployment -n kube-system coredns

# Wait for CoreDNS
kubectl wait --for=condition=ready pod -n kube-system -l k8s-app=kube-dns --timeout=120s

# Check CoreDNS logs
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=50
```

---

## ✅ Prevention Checklist

### Pre-Deployment

- [ ] Run pre-deployment checks: `./scripts/pre-deployment-checks.sh`
- [ ] Verify CoreDNS is healthy
- [ ] Test DNS resolution from test pod
- [ ] Check network connectivity between nodes
- [ ] Verify node labels (worker/master)
- [ ] Check for existing deployments (cleanup if needed)

### During Deployment

- [ ] Use robust deployment script: `./scripts/deploy-fortuna-robust.sh`
- [ ] Monitor deployment progress
- [ ] Verify each step completes
- [ ] Check pod status after each deployment

### Post-Deployment

- [ ] Verify Core pod is Ready
- [ ] Verify Core service has endpoints
- [ ] Verify Agent pods are running
- [ ] Test Core API: `kubectl port-forward -n fortuna svc/fortuna-core 8080:8080`
- [ ] Check logs for errors
- [ ] Test Agent → Core connection

---

## 📊 Issue Resolution Matrix

| Issue | Root Cause | Workaround | Permanent Fix | Status |
|-------|-----------|------------|---------------|--------|
| Core DB DNS | DNS timeout | IP-based URL | Fix DNS | ✅ Workaround |
| Core HTTP | DB connection | IP-based URL | Fix DNS | ✅ Workaround |
| Multiple deploys | Cleanup missing | Script cleanup | Better scripts | ✅ Fixed |
| Agent Core DNS | DNS timeout | IP-based endpoint | Fix DNS | ✅ Workaround |
| Agent TLS | ServerName mismatch | Disable TLS | Update code | ✅ Workaround |
| NATS storage | Cluster config | Single node | Fix cluster | ✅ Workaround |
| NATS DNS | DNS timeout | Single node | Fix DNS | ✅ Workaround |
| Readiness probe | Dependency chain | IP-based URL | Fix DNS | ✅ Workaround |

---

## 🔄 Migration to Production

### Phase 1: Current (Workarounds)
- ✅ IP-based connections
- ✅ TLS disabled
- ✅ Single-node NATS
- ✅ Master-only Core (optional)

### Phase 2: DNS Fix
- [ ] Diagnose CoreDNS issues
- [ ] Fix network connectivity
- [ ] Test DNS resolution
- [ ] Update CoreDNS config if needed

### Phase 3: Configuration Migration
- [ ] Change DATABASE_URL to DNS name
- [ ] Change CORE_GRPC_ENDPOINT to DNS name
- [ ] Re-enable TLS
- [ ] Update mTLS ServerName validation

### Phase 4: High Availability
- [ ] Scale NATS to 3 replicas
- [ ] Configure NATS cluster
- [ ] Test failover
- [ ] Update documentation

---

## 📚 Related Documentation

- [Deployment Issues Analysis](./DEPLOYMENT_ISSUES_ANALYSIS.md) - Detailed analysis
- [Deployment Issues Summary](./DEPLOYMENT_ISSUES_SUMMARY.md) - Quick reference
- [DNS Troubleshooting](./DNS_TROUBLESHOOTING.md) - DNS-specific guide
- [Node Placement Strategy](../DEPLOYMENT_NODE_PLACEMENT.md) - Core placement
- [Complete Setup Guide](../01-getting-started/COMPLETE_SETUP_GUIDE.md) - Full setup

---

## 🎯 Quick Reference

### Deployment Commands

```bash
# Full deployment (recommended)
./scripts/deploy-fortuna-robust.sh

# Pre-deployment checks only
./scripts/pre-deployment-checks.sh

# Fix DNS issues
./scripts/fix-dns-issues.sh

# Quick fixes
POSTGRES_IP=$(kubectl get svc -n fortuna postgres -o jsonpath='{.spec.clusterIP}')
kubectl set env deployment/fortuna-core -n fortuna \
  DATABASE_URL="postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam?sslmode=disable"

CORE_IP=$(kubectl get svc -n fortuna fortuna-core -o jsonpath='{.spec.clusterIP}')
kubectl set env daemonset/fortuna-agent -n fortuna \
  CORE_GRPC_ENDPOINT="${CORE_IP}:9090"
kubectl set env daemonset/fortuna-agent -n fortuna TLS_ENABLED="false"
```

### Verification Commands

```bash
# Check Core
kubectl get pods -n fortuna -l app.kubernetes.io/component=core
kubectl get endpoints -n fortuna fortuna-core
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50

# Check Agent
kubectl get pods -n fortuna -l app.kubernetes.io/component=agent
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=50

# Test DNS
kubectl run dns-test --image=busybox:1.36 --rm -i --restart=Never -- \
  nslookup fortuna-core.fortuna.svc.cluster.local
```

---

**Last Updated:** December 29, 2025  
**Maintained By:** DevOps Team

