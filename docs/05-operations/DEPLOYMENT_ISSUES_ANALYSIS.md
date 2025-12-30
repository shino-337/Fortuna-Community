# Fortuna Deployment Issues - Comprehensive Analysis

**Date:** December 29, 2025  
**Status:** ✅ Resolved  
**Version:** 1.0

---

## 📋 Table of Contents

1. [Executive Summary](#executive-summary)
2. [Issues Encountered](#issues-encountered)
3. [Root Cause Analysis](#root-cause-analysis)
4. [Solutions Implemented](#solutions-implemented)
5. [Deployment Script](#deployment-script)
6. [Best Practices](#best-practices)
7. [Lessons Learned](#lessons-learned)
8. [Prevention Strategies](#prevention-strategies)

---

## 🎯 Executive Summary

During the deployment of Fortuna on a multi-node Kubernetes cluster (kubeadm, containerd), we encountered multiple interconnected issues that prevented Core and Agent from functioning correctly. The primary root cause was **DNS resolution failures** affecting both Core (PostgreSQL connection) and Agent (Core gRPC connection). Additional issues included multiple conflicting deployments, readiness probe failures, and TLS ServerName validation.

**Key Metrics:**
- **Total Issues:** 8 major issues
- **Primary Root Cause:** DNS resolution timeout
- **Resolution Time:** ~2 hours
- **Workarounds Applied:** IP-based connections (temporary)
- **Production Readiness:** Requires DNS fix

---

## 🔴 Issues Encountered

### Issue 1: Core Pod - Database Connection DNS Timeout

**Symptom:**
```
failed to connect to `host=postgres.fortuna.svc.cluster.local user=postgres database=ksam`: 
hostname resolving error (lookup postgres.fortuna.svc.cluster.local on 10.96.0.10:53: 
read udp 10.244.1.17:36646->10.96.0.10:53: i/o timeout)
```

**Impact:**
- Core pod cannot connect to PostgreSQL
- HTTP server fails to start
- Readiness probe fails
- Service has no endpoints
- Agent cannot connect to Core

**Severity:** 🔴 Critical

---

### Issue 2: Core Pod - HTTP Server Not Starting

**Symptom:**
```
Warning  Unhealthy  Readiness probe failed: Get "http://10.244.0.21:8080/health": 
dial tcp 10.244.0.21:8080: connect: connection refused
Warning  Unhealthy  Liveness probe failed: Get "http://10.244.0.21:8080/health": 
dial tcp 10.244.0.21:8080: connect: connection refused
```

**Impact:**
- Core pod status: `Running` but not `Ready`
- Service endpoints empty
- Kubernetes marks pod as unhealthy
- Pod restarts continuously (CrashLoopBackOff)

**Severity:** 🔴 Critical

---

### Issue 3: Multiple Core Deployments Conflict

**Symptom:**
```
fortuna-core-547fd9cfbf-knllg   0/1  CrashLoopBackOff
fortuna-core-7c7d7d6cfb-5sgnd   0/1  Running (restarting)
```

**Impact:**
- Two Core deployments running simultaneously
- Resource conflicts
- Unpredictable behavior
- Service endpoints pointing to wrong pods

**Severity:** 🟡 High

---

### Issue 4: Agent Pod - Core Connection DNS Timeout

**Symptom:**
```
⚠️  Heartbeat failed: Ping RPC failed: rpc error: code = Unavailable desc = 
connection error: desc = "transport: Error while dialing: dial tcp: 
lookup fortuna-core.fortuna.svc.cluster.local on 10.96.0.10:53: 
read udp 10.244.0.18:57244->10.96.0.10:53: i/o timeout
```

**Impact:**
- Agent cannot connect to Core gRPC endpoint
- SBOM data cannot be sent to Core
- Heartbeat fails continuously
- Agent functionality degraded

**Severity:** 🔴 Critical

---

### Issue 5: NATS JetStream - Storage/Placement Issues

**Symptom:**
```
Failed to create stream ksam-insights: nats: no suitable peers for placement, 
insufficient storage
Failed to create stream ksam-raw: nats: JetStream system temporarily unavailable
```

**Impact:**
- NATS streams cannot be created
- Core fails to initialize NATS client
- Application startup fails

**Severity:** 🟡 High

---

### Issue 6: NATS Cluster - DNS Resolution for Cluster Routes

**Symptom:**
```
[ERR] Error trying to connect to route (attempt 454): lookup for host 
"nats-2.nats.fortuna.svc.cluster.local": lookup nats-2.nats.fortuna.svc.cluster.local 
on 10.96.0.10:53: no such host
JetStream cluster no metadata leader
```

**Impact:**
- NATS cluster cannot form quorum
- JetStream unavailable
- Core cannot connect to NATS

**Severity:** 🟡 High

---

### Issue 7: Core Pod - Readiness Probe Dependency Chain

**Symptom:**
- Readiness probe checks HTTP `/health` endpoint
- Health endpoint requires database connection
- Database connection requires DNS resolution
- DNS fails → Database fails → Health fails → Readiness fails → Service has no endpoints

**Impact:**
- Circular dependency
- Pod never becomes Ready
- Service never gets endpoints
- Agent cannot discover Core

**Severity:** 🔴 Critical

---

### Issue 8: Agent Pod - TLS ServerName Validation

**Symptom:**
- Agent configured with `CORE_GRPC_ENDPOINT` using IP address
- mTLS ServerName hardcoded to `fortuna-core.fortuna.svc.cluster.local`
- TLS handshake fails when using IP

**Impact:**
- Even with IP-based endpoint, TLS validation fails
- Connection refused or TLS handshake error

**Severity:** 🟡 Medium (workaround: disable TLS)

---

## 🔍 Root Cause Analysis

### Primary Root Cause: DNS Resolution Failure

**CoreDNS Issues:**
- CoreDNS pods running but experiencing high restart count (228, 229 restarts)
- DNS queries timing out: `i/o timeout`
- Network connectivity issues between worker nodes and CoreDNS on master
- CoreDNS service IP (10.96.0.10) not reachable from some pods

**Network Analysis:**
```
Core pod (k8s-worker01) → CoreDNS (10.96.0.10:53) → TIMEOUT
Agent pod (k8s-master) → CoreDNS (10.96.0.10:53) → TIMEOUT
```

**Possible Causes:**
1. Network policies blocking DNS traffic
2. Firewall rules on nodes
3. CoreDNS pods not properly configured
4. Network plugin (flannel) issues
5. Node network connectivity problems

### Secondary Root Causes

1. **Multiple Deployments:**
   - Old deployment files not cleaned up
   - Manual deployments created alongside script deployments
   - No verification step to ensure single deployment

2. **Readiness Probe Dependency:**
   - Health check requires database connection
   - Database connection requires DNS
   - Creates circular dependency when DNS fails

3. **NATS Cluster Configuration:**
   - 3-replica cluster configured but only 1 node available
   - Cluster routes reference non-existent pods (nats-2)
   - JetStream requires quorum for cluster mode

---

## ✅ Solutions Implemented

### Solution 1: IP-Based Database Connection (Core)

**Implementation:**
```bash
# Get PostgreSQL IP
POSTGRES_IP=$(kubectl get svc -n fortuna postgres -o jsonpath='{.spec.clusterIP}')

# Update DATABASE_URL
kubectl set env deployment/fortuna-core -n fortuna \
  DATABASE_URL="postgres://postgres:postgres@${POSTGRES_IP}:5432/ksam?sslmode=disable"
```

**Result:**
- ✅ Core can connect to PostgreSQL without DNS
- ✅ HTTP server starts successfully
- ✅ Readiness probe passes
- ✅ Service gets endpoints

**Trade-off:**
- ⚠️ Workaround, not production-ready
- ⚠️ IP may change if service is recreated
- ⚠️ Harder to maintain

---

### Solution 2: IP-Based gRPC Endpoint (Agent)

**Implementation:**
```bash
# Get Core service IP
CORE_IP=$(kubectl get svc -n fortuna fortuna-core -o jsonpath='{.spec.clusterIP}')

# Update CORE_GRPC_ENDPOINT
kubectl set env daemonset/fortuna-agent -n fortuna \
  CORE_GRPC_ENDPOINT="${CORE_IP}:9090"
```

**Result:**
- ✅ Agent can connect to Core without DNS
- ✅ SBOM data can be sent
- ✅ Heartbeat succeeds

**Trade-off:**
- ⚠️ Workaround, not production-ready
- ⚠️ Requires TLS to be disabled (ServerName validation)

---

### Solution 3: Disable TLS Temporarily (Agent)

**Implementation:**
```bash
kubectl set env daemonset/fortuna-agent -n fortuna TLS_ENABLED="false"
```

**Reason:**
- mTLS ServerName validation requires DNS name
- When using IP, ServerName mismatch causes TLS handshake failure

**Result:**
- ✅ Agent can connect to Core with IP endpoint
- ✅ Communication works (insecure)

**Trade-off:**
- ⚠️ Security risk (no encryption)
- ⚠️ Not suitable for production
- ⚠️ Temporary solution only

---

### Solution 4: Single-Node NATS Configuration

**Implementation:**
- Changed NATS StatefulSet from 3 replicas to 1
- Removed cluster configuration
- Single-node JetStream mode

**Result:**
- ✅ NATS starts successfully
- ✅ JetStream available
- ✅ Core can connect to NATS

**Trade-off:**
- ⚠️ No high availability
- ⚠️ Single point of failure
- ⚠️ Suitable for testing only

---

### Solution 5: Comprehensive Cleanup Script

**Implementation:**
- Delete all deployments by labels
- Delete all deployments by name patterns
- Delete all pods
- Verify deletion before deploying new resources

**Result:**
- ✅ No duplicate deployments
- ✅ Clean state before deployment
- ✅ Predictable deployment behavior

---

### Solution 6: Master-Only Core Deployment

**Implementation:**
```yaml
nodeSelector:
  node-role.kubernetes.io/control-plane: ""

tolerations:
  - key: node-role.kubernetes.io/control-plane
    operator: Exists
    effect: NoSchedule
```

**Result:**
- ✅ Core always runs on master node
- ✅ Predictable resource location
- ✅ Better for control plane components

---

### Solution 7: Complete Deployment Script

**Implementation:**
- Single script handles all deployment steps
- Sequential dependency management
- Verification at each step
- Automatic cleanup and retry

**Result:**
- ✅ Consistent deployments
- ✅ Reduced manual errors
- ✅ Faster troubleshooting

---

## 📜 Deployment Script

### Main Script: `scripts/deploy-fortuna-complete.sh`

**Features:**
1. Prerequisites check (namespace, PostgreSQL, NATS, mTLS)
2. Comprehensive cleanup (all old resources)
3. IP-based configuration (DATABASE_URL, CORE_GRPC_ENDPOINT)
4. Sequential deployment (Core first, then Agent)
5. Verification at each step
6. Final status report

**Usage:**
```bash
./scripts/deploy-fortuna-complete.sh
```

**What It Does:**
1. ✅ Checks prerequisites
2. ✅ Cleans up all old deployments
3. ✅ Deploys Core with IP-based DATABASE_URL
4. ✅ Waits for Core to be Ready
5. ✅ Gets Core service IP
6. ✅ Deploys Agent with IP-based CORE_GRPC_ENDPOINT
7. ✅ Disables TLS temporarily
8. ✅ Verifies connections
9. ✅ Reports final status

---

## 🎓 Best Practices

### 1. DNS Configuration

**Do:**
- ✅ Ensure CoreDNS is healthy before deploying applications
- ✅ Test DNS resolution from pods before deployment
- ✅ Use service names for service discovery (production)
- ✅ Monitor CoreDNS metrics and logs

**Don't:**
- ❌ Rely on IP addresses for production
- ❌ Ignore DNS errors
- ❌ Deploy applications before DNS is ready

### 2. Deployment Management

**Do:**
- ✅ Use single source of truth for deployments
- ✅ Clean up old resources before deploying new ones
- ✅ Verify deployment count after apply
- ✅ Use labels consistently

**Don't:**
- ❌ Create multiple deployments manually
- ❌ Mix manual and script-based deployments
- ❌ Skip cleanup steps

### 3. Health Checks

**Do:**
- ✅ Make health checks independent of external dependencies when possible
- ✅ Use separate liveness and readiness probes
- ✅ Set appropriate timeouts and delays
- ✅ Test health endpoints manually

**Don't:**
- ❌ Create circular dependencies (health → DB → DNS → health)
- ❌ Use same endpoint for liveness and readiness
- ❌ Set too short initial delays

### 4. Configuration Management

**Do:**
- ✅ Use ConfigMaps for non-sensitive config
- ✅ Use environment variables for runtime config
- ✅ Document all configuration options
- ✅ Version control all configs

**Don't:**
- ❌ Hardcode IP addresses
- ❌ Mix configuration sources
- ❌ Skip validation

### 5. Troubleshooting

**Do:**
- ✅ Check pod logs first
- ✅ Verify service endpoints
- ✅ Test connectivity manually
- ✅ Check events and describe output

**Don't:**
- ❌ Assume DNS is working
- ❌ Ignore readiness probe failures
- ❌ Skip verification steps

---

## 💡 Lessons Learned

### 1. DNS is Critical Infrastructure

**Lesson:** DNS failures cascade to all dependent services. Always verify DNS before deploying applications.

**Action Items:**
- Add DNS health check to deployment scripts
- Monitor CoreDNS metrics
- Test DNS resolution from test pods

### 2. Workarounds Are Temporary

**Lesson:** IP-based connections work but are not production-ready. Always plan to fix root cause.

**Action Items:**
- Document all workarounds
- Create tickets for permanent fixes
- Set deadlines for workaround removal

### 3. Cleanup is Essential

**Lesson:** Multiple deployments cause unpredictable behavior. Always clean up before deploying.

**Action Items:**
- Add cleanup to all deployment scripts
- Verify cleanup completion
- Use idempotent deployment scripts

### 4. Verification Prevents Issues

**Lesson:** Verifying each step catches issues early. Don't skip verification.

**Action Items:**
- Add verification to all deployment steps
- Fail fast on verification failures
- Provide clear error messages

### 5. Dependencies Matter

**Lesson:** Service dependencies create failure chains. Understand and document all dependencies.

**Action Items:**
- Create dependency diagram
- Test dependencies independently
- Plan for dependency failures

---

## 🛡️ Prevention Strategies

### 1. Pre-Deployment Checks

**Script:** `scripts/pre-deployment-checks.sh`

**Checks:**
- [ ] CoreDNS pods healthy
- [ ] DNS resolution working
- [ ] Network connectivity between nodes
- [ ] Storage available
- [ ] Required services running

### 2. Deployment Validation

**Script:** `scripts/validate-deployment.sh`

**Validations:**
- [ ] Only one Core deployment exists
- [ ] Core pod is Ready
- [ ] Service has endpoints
- [ ] Agent DaemonSet exists
- [ ] Agent pods running on all nodes
- [ ] Connections working

### 3. Health Monitoring

**Metrics to Monitor:**
- CoreDNS restart count
- DNS query latency
- Service endpoint count
- Pod readiness status
- Connection success rate

### 4. Documentation

**Required Documentation:**
- Deployment procedures
- Troubleshooting guides
- Known issues and workarounds
- Configuration reference
- Architecture diagrams

---

## 📊 Issue Resolution Matrix

| Issue | Root Cause | Workaround | Permanent Fix | Status |
|-------|-----------|------------|---------------|--------|
| Core DB DNS | DNS timeout | IP-based URL | Fix DNS | ✅ Workaround Applied |
| Core HTTP not start | DB connection fail | IP-based URL | Fix DNS | ✅ Workaround Applied |
| Multiple Core deploys | Cleanup missing | Script cleanup | Better scripts | ✅ Fixed |
| Agent Core DNS | DNS timeout | IP-based endpoint | Fix DNS | ✅ Workaround Applied |
| Agent TLS validation | ServerName mismatch | Disable TLS | Update code | ✅ Workaround Applied |
| NATS storage | Cluster config | Single node | Fix cluster | ✅ Workaround Applied |
| NATS DNS | DNS timeout | Single node | Fix DNS | ✅ Workaround Applied |
| Readiness probe | Dependency chain | IP-based URL | Fix DNS | ✅ Workaround Applied |

---

## 🔄 Migration Path to Production

### Phase 1: Current State (Workarounds)
- ✅ IP-based connections
- ✅ TLS disabled
- ✅ Single-node NATS
- ✅ Master-only Core

### Phase 2: DNS Fix
- [ ] Diagnose and fix CoreDNS issues
- [ ] Test DNS resolution from all nodes
- [ ] Verify network connectivity
- [ ] Update CoreDNS configuration if needed

### Phase 3: Configuration Migration
- [ ] Change DATABASE_URL back to DNS name
- [ ] Change CORE_GRPC_ENDPOINT back to DNS name
- [ ] Re-enable TLS
- [ ] Update mTLS ServerName validation (if needed)

### Phase 4: High Availability
- [ ] Scale NATS to 3 replicas
- [ ] Configure NATS cluster properly
- [ ] Test failover scenarios
- [ ] Update documentation

---

## 📝 Related Documentation

- [Core Connection Troubleshooting](./CORE_CONNECTION_TROUBLESHOOTING.md)
- [Agent Deployment Troubleshooting](./AGENT_DEPLOYMENT_TROUBLESHOOTING.md)
- [DNS Troubleshooting](./DNS_TROUBLESHOOTING.md) (to be created)
- [Complete Setup Guide](../01-getting-started/COMPLETE_SETUP_GUIDE.md)
- [Multi-Node Deployment](../01-getting-started/MULTI_NODE_K8S_DEPLOYMENT.md)

---

## 🎯 Conclusion

The deployment issues encountered were primarily due to DNS resolution failures, which cascaded through the entire system. While workarounds using IP-based connections have been implemented and are functional, they are not suitable for production use. The permanent solution requires fixing the underlying DNS infrastructure.

**Key Takeaways:**
1. DNS is foundational - verify it first
2. Workarounds are temporary - plan permanent fixes
3. Cleanup prevents conflicts - always clean before deploy
4. Verification catches issues - don't skip it
5. Dependencies create chains - understand them

**Next Steps:**
1. Fix CoreDNS/DNS infrastructure
2. Migrate from IP-based to DNS-based configuration
3. Re-enable TLS with proper ServerName validation
4. Scale NATS to production configuration
5. Update all documentation

---

**Document Version:** 1.0  
**Last Updated:** December 29, 2025  
**Maintained By:** DevOps Team

