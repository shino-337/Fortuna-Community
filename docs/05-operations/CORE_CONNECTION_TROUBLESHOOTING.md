# Core gRPC Connection Troubleshooting

## Problem

Agent cannot connect to Core gRPC endpoint:
```
rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing: dial tcp 10.108.158.11:9090: connect: connection refused"
```

## Root Causes

### 1. Core Pod Not Ready
- **Symptom**: Service has no endpoints (`kubectl get endpoints fortuna-core -n fortuna` shows no addresses)
- **Cause**: Readiness probe failing (HTTP port 8080 not responding)
- **Impact**: Kubernetes does not add pod IP to service endpoints, so Agent cannot resolve service

### 2. Core gRPC Server Not Started
- **Symptom**: Port 9090 not listening in Core pod
- **Cause**: 
  - gRPC server failed to start (check logs)
  - TLS certificate issues
  - Database connection failed
  - NATS connection failed

### 3. Network Issues
- **Symptom**: DNS resolution works but port 9090 not reachable
- **Cause**: Network policies, firewall, or service mesh blocking traffic

## Diagnosis Steps

### Step 1: Check Core Pod Status
```bash
kubectl get pods -n fortuna -l app.kubernetes.io/component=core
kubectl describe pod -n fortuna <core-pod-name>
```

Look for:
- **Phase**: Should be `Running`
- **Ready**: Should be `True` (not just Running)
- **Container State**: Should not be `Waiting` or `Terminated`

### Step 2: Check Service Endpoints
```bash
kubectl get endpoints -n fortuna fortuna-core
kubectl get endpoints -n fortuna fortuna-core -o yaml
```

**Expected**: Should have at least one IP address in `subsets[0].addresses`

**If empty**: Core pod is not Ready (readiness probe failing)

### Step 3: Check Core Logs
```bash
kubectl logs -n fortuna <core-pod-name> | grep -i "gRPC\|grpc\|error\|fatal"
kubectl logs -n fortuna <core-pod-name> --tail=100
```

Look for:
- `✅ gRPC server listening on 0.0.0.0:9090` - Server started successfully
- `ERROR: Failed to listen on` - Port binding failed
- `Failed to create gRPC server` - Server creation failed
- Database connection errors
- NATS connection errors

### Step 4: Check if gRPC Port is Listening
```bash
kubectl exec -n fortuna <core-pod-name> -- netstat -tlnp | grep 9090
# Or
kubectl exec -n fortuna <core-pod-name> -- ss -tlnp | grep 9090
```

**Expected**: Should show port 9090 listening

### Step 5: Check Readiness Probe
```bash
kubectl describe pod -n fortuna <core-pod-name> | grep -A 10 "Readiness"
```

Check if:
- Readiness probe is configured (should check HTTP port 8080)
- Probe is passing (no "Readiness probe failed" messages)

### Step 6: Test HTTP Health Endpoint
```bash
kubectl exec -n fortuna <core-pod-name> -- wget -qO- http://localhost:8080/health
```

**Expected**: Should return HTTP 200 OK

If this fails, readiness probe will fail, and pod will not become Ready.

### Step 7: Test Connectivity from Agent
```bash
AGENT_POD=$(kubectl get pods -n fortuna -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}')

# Test DNS resolution
kubectl exec -n fortuna $AGENT_POD -- nslookup fortuna-core.fortuna.svc.cluster.local

# Test port connectivity
kubectl exec -n fortuna $AGENT_POD -- nc -zv fortuna-core.fortuna.svc.cluster.local 9090
```

## Common Fixes

### Fix 1: Core Pod Not Ready (Readiness Probe Failing)

**Symptoms**:
- Pod is Running but not Ready
- Service has no endpoints
- Readiness probe failing

**Causes**:
1. HTTP server (port 8080) not started
2. Database connection failed
3. NATS connection failed
4. Application startup errors

**Solution**:
```bash
# Check Core logs for startup errors
kubectl logs -n fortuna <core-pod-name> --tail=100

# Check if HTTP port is responding
kubectl exec -n fortuna <core-pod-name> -- wget -qO- http://localhost:8080/health

# If HTTP not responding, check:
# 1. Database connection (DATABASE_URL env var)
# 2. NATS connection (NATS_ENDPOINT env var)
# 3. Application errors in logs
```

### Fix 2: gRPC Server Not Started

**Symptoms**:
- Port 9090 not listening
- Logs show gRPC server errors

**Causes**:
1. TLS certificate issues
2. Port already in use (unlikely in container)
3. Server creation failed

**Solution**:
```bash
# Check Core logs for gRPC errors
kubectl logs -n fortuna <core-pOD-name> | grep -i "gRPC\|grpc"

# Check TLS certificates
kubectl exec -n fortuna <core-pod-name> -- ls -la /etc/fortuna/tls/server/

# Verify TLS certificates are valid
kubectl exec -n fortuna <core-pod-name> -- cat /etc/fortuna/tls/server/tls.crt | openssl x509 -text -noout
```

### Fix 3: TLS Certificate Issues

**Symptoms**:
- gRPC server fails to start
- TLS-related errors in logs

**Solution**:
```bash
# Recreate mTLS secrets
./scripts/create-mtls-secrets.sh

# Restart Core deployment
kubectl rollout restart deployment -n fortuna fortuna-core
```

### Fix 4: Database Connection Issues

**Symptoms**:
- Core logs show database connection errors
- Pod not becoming Ready

**Solution**:
```bash
# Check PostgreSQL is running
kubectl get pods -n fortuna -l app=postgres

# Check DATABASE_URL env var
kubectl get deployment -n fortuna fortuna-core -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="DATABASE_URL")].value}'

# Test database connection from Core pod
kubectl exec -n fortuna <core-pod-name> -- sh -c 'echo $DATABASE_URL'
```

### Fix 5: NATS Connection Issues

**Symptoms**:
- Core logs show NATS connection errors
- Pod not becoming Ready

**Solution**:
```bash
# Check NATS is running
kubectl get pods -n fortuna -l app=nats

# Check NATS_ENDPOINT env var
kubectl get deployment -n fortuna fortuna-core -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="NATS_ENDPOINT")].value}'
```

## Automated Diagnosis

Use the provided scripts:

```bash
# Comprehensive diagnosis
./scripts/check-core-connection.sh

# Detailed troubleshooting
./scripts/fix-core-connection.sh
```

## Verification

After fixing, verify:

1. **Core pod is Ready**:
   ```bash
   kubectl get pods -n fortuna -l app.kubernetes.io/component=core
   ```
   Should show `1/1 Running`

2. **Service has endpoints**:
   ```bash
   kubectl get endpoints -n fortuna fortuna-core
   ```
   Should show at least one IP address

3. **gRPC port is listening**:
   ```bash
   kubectl exec -n fortuna <core-pod-name> -- ss -tlnp | grep 9090
   ```

4. **Agent can connect**:
   ```bash
   kubectl logs -n fortuna <agent-pod-name> | grep -i "connected\|connection"
   ```
   Should show `✅ Connected to Core` or successful heartbeat

## Prevention

1. **Ensure dependencies are ready**:
   - PostgreSQL must be running and accessible
   - NATS must be running and accessible
   - mTLS secrets must be created

2. **Monitor Core startup**:
   - Check logs during deployment
   - Verify all services start successfully
   - Ensure readiness probe passes

3. **Health checks**:
   - HTTP health endpoint should respond quickly
   - gRPC server should start within readiness probe initial delay

## Related Documentation

- [Agent Deployment Troubleshooting](./AGENT_DEPLOYMENT_TROUBLESHOOTING.md)
- [Complete Setup Guide](../01-getting-started/COMPLETE_SETUP_GUIDE.md)
- [Multi-Node Deployment](../01-getting-started/MULTI_NODE_K8S_DEPLOYMENT.md)

