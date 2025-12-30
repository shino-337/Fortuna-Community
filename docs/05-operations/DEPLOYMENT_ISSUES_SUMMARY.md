# Deployment Issues Summary

**Quick Reference Guide**  
**Date:** December 29, 2025

---

## 🎯 Quick Overview

**Primary Issue:** DNS resolution failures causing cascading failures across Core and Agent.

**Workaround Applied:** IP-based connections (temporary, not production-ready).

**Status:** ✅ Functional with workarounds | ⚠️ Requires DNS fix for production

---

## 📊 Issues at a Glance

| Component | Issue | Impact | Workaround | Status |
|-----------|-------|--------|------------|--------|
| Core | Database DNS timeout | Cannot connect to PostgreSQL | Use IP in DATABASE_URL | ✅ Fixed |
| Core | HTTP server not starting | Readiness probe fails | Fix DB connection | ✅ Fixed |
| Core | Multiple deployments | Resource conflicts | Cleanup script | ✅ Fixed |
| Agent | Core DNS timeout | Cannot connect to Core | Use IP in CORE_GRPC_ENDPOINT | ✅ Fixed |
| Agent | TLS ServerName validation | TLS handshake fails | Disable TLS temporarily | ✅ Fixed |
| NATS | Storage/placement issues | Streams cannot be created | Single-node mode | ✅ Fixed |
| NATS | Cluster DNS issues | Cluster cannot form | Single-node mode | ✅ Fixed |

---

## 🔧 Quick Fixes

### Fix Core Database Connection

```bash
POSTGRES_IP=$(kubectl get svc -n fortuna postgres -o jsonpath='{.spec.clusterIP}')
kubectl set env deployment/fortuna-core -n fortuna \
  DATABASE_URL="postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam?sslmode=disable"
kubectl rollout restart deployment -n fortuna fortuna-core
```

### Fix Agent Core Connection

```bash
CORE_IP=$(kubectl get svc -n fortuna fortuna-core -o jsonpath='{.spec.clusterIP}')
kubectl set env daemonset/fortuna-agent -n fortuna \
  CORE_GRPC_ENDPOINT="${CORE_IP}:9090"
kubectl set env daemonset/fortuna-agent -n fortuna TLS_ENABLED="false"
kubectl rollout restart daemonset -n fortuna fortuna-agent
```

### Complete Deployment (Recommended)

```bash
./scripts/deploy-fortuna-complete.sh
```

---

## 📋 Root Causes

1. **DNS Resolution Failure** (Primary)
   - CoreDNS pods unstable (high restart count)
   - Network connectivity issues between nodes
   - DNS queries timing out

2. **Multiple Deployments**
   - Old deployments not cleaned up
   - Manual and script deployments mixed

3. **Dependency Chain**
   - Readiness probe → Health endpoint → Database → DNS → Fails

---

## ✅ Solutions Applied

### Temporary Workarounds (Current)

- ✅ IP-based DATABASE_URL for Core
- ✅ IP-based CORE_GRPC_ENDPOINT for Agent
- ✅ TLS disabled for Agent
- ✅ Single-node NATS
- ✅ Master-only Core deployment
- ✅ Comprehensive cleanup script

### Permanent Fixes Required

- [ ] Fix CoreDNS/DNS infrastructure
- [ ] Migrate to DNS-based configuration
- [ ] Re-enable TLS with proper validation
- [ ] Scale NATS to production (3 replicas)
- [ ] Update mTLS ServerName validation code

---

## 🚀 Deployment Script

**Use this script for complete deployment:**

```bash
./scripts/deploy-fortuna-complete.sh
```

**What it does:**
1. Checks prerequisites
2. Cleans up old resources
3. Deploys Core with IP-based config
4. Waits for Core to be Ready
5. Deploys Agent with IP-based config
6. Verifies everything

---

## 📚 Detailed Documentation

- **[Full Analysis](./DEPLOYMENT_ISSUES_ANALYSIS.md)** - Comprehensive analysis
- **[DNS Troubleshooting](./DNS_TROUBLESHOOTING.md)** - DNS-specific issues
- **[Core Connection](./CORE_CONNECTION_TROUBLESHOOTING.md)** - Core connection issues
- **[Agent Deployment](./AGENT_DEPLOYMENT_TROUBLESHOOTING.md)** - Agent issues

---

## ⚠️ Important Notes

1. **Workarounds are temporary** - Not suitable for production
2. **DNS must be fixed** - For production deployment
3. **TLS is disabled** - Security risk, re-enable after DNS fix
4. **Single-node NATS** - No HA, suitable for testing only

---

**Last Updated:** December 29, 2025

