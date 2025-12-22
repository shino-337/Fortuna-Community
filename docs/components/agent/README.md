# Agent Component

The KSAM Agent runs as a DaemonSet on each Kubernetes node, collecting security data and reporting to Core.

---

## Overview

**Status**: ✅ Production Ready (MVP1)
**Deployment**: DaemonSet (one pod per node)
**Communication**: gRPC with mTLS
**Collection Interval**: Configurable (default: 30s)

---

## Architecture

```
┌─────────────────────────────────┐
│     Kubernetes API Server       │
└────────────┬────────────────────┘
             │ Watch
             ▼
┌─────────────────────────────────┐
│      KSAM Agent (DaemonSet)     │
│  ┌──────────┐   ┌───────────┐  │
│  │Collector │   │ Converter │  │
│  └────┬─────┘   └─────┬─────┘  │
│       │               │         │
│       ▼               ▼         │
│  ┌────────────────────────┐    │
│  │    gRPC Client (mTLS)  │    │
│  └──────────┬─────────────┘    │
└─────────────┼──────────────────┘
              │
              ▼
┌─────────────────────────────────┐
│        KSAM Core Service        │
└─────────────────────────────────┘
```

---

## Key Features

✅ **Resource Collection**
- ServiceAccounts
- Pods
- RoleBindings
- ClusterRoleBindings

✅ **Security Features**
- mTLS authentication
- Certificate-based identity
- Secure gRPC communication

✅ **Performance**
- Efficient Kubernetes API watching
- Incremental updates only
- Configurable collection intervals

---

## Configuration

### Environment Variables

```yaml
# Core connectivity
CORE_GRPC_ENDPOINT: "ksam-core:50051"
CORE_GRPC_TIMEOUT: "30s"

# TLS/mTLS
TLS_ENABLED: "true"
MTLS_ENABLED: "true"
CERT_PATH: "/etc/ksam/certs/agent.crt"
KEY_PATH: "/etc/ksam/certs/agent.key"
CA_CERT_PATH: "/etc/ksam/certs/ca.crt"

# Collection
COLLECTION_INTERVAL: "30s"
CLUSTER_NAME: "production"
```

### DaemonSet Deployment

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ksam-agent
  namespace: ksam
spec:
  template:
    spec:
      containers:
      - name: agent
        image: ksam/agent:latest
        envFrom:
        - configMapRef:
            name: ksam-agent-config
        volumeMounts:
        - name: certs
          mountPath: /etc/ksam/certs
          readOnly: true
```

---

## Monitoring

### Prometheus Metrics

```
ksam_agent_collection_duration_seconds
ksam_agent_resources_collected_total{type="serviceaccount"}
ksam_agent_grpc_errors_total
ksam_agent_last_sync_timestamp
```

### Health Checks

```bash
# Check agent pods
kubectl get pods -n ksam -l app=ksam-agent

# View agent logs
kubectl logs -n ksam -l app=ksam-agent --tail=100

# Check connectivity to core
kubectl logs -n ksam -l app=ksam-agent | grep -i "connected\\|grpc"
```

---

## Troubleshooting

### Agent Not Connecting to Core

**Symptoms**: Logs show connection errors

**Debug Steps**:
```bash
# Check certificates
kubectl exec -n ksam ksam-agent-* -- ls -la /etc/ksam/certs/

# Test gRPC connectivity
kubectl exec -n ksam ksam-agent-* -- nc -zv ksam-core 50051

# Check mTLS handshake
kubectl logs -n ksam ksam-agent-* | grep -i "tls\\|certificate"
```

**Common Fixes**:
- Regenerate certificates if expired
- Verify CA cert matches between agent and core
- Check network policies allow gRPC traffic

### No Resources Being Collected

**Symptoms**: Database shows no ServiceAccounts from cluster

**Debug Steps**:
```bash
# Check RBAC permissions
kubectl describe clusterrolebinding ksam-agent

# View collection logs
kubectl logs -n ksam ksam-agent-* | grep -i "collect\\|watch"

# Test Kubernetes API access
kubectl exec -n ksam ksam-agent-* -- kubectl get serviceaccounts
```

### High Memory Usage

**Symptoms**: Agent pods consuming excessive memory

**Debug Steps**:
```bash
# Check resource usage
kubectl top pod -n ksam -l app=ksam-agent

# Review collection interval
kubectl get configmap -n ksam ksam-agent-config -o yaml

# Check for resource leaks
kubectl logs -n ksam ksam-agent-* | grep -i "memory\\|leak"
```

**Solutions**:
- Increase collection interval
- Set memory limits in DaemonSet
- Check for goroutine leaks

---

## Development

### Local Testing

```bash
# Build agent binary
cd agent
go build -o ksam-agent cmd/main.go

# Run locally (requires kubeconfig)
export KUBECONFIG=~/.kube/config
export CORE_GRPC_ENDPOINT=localhost:50051
./ksam-agent
```

### Unit Tests

```bash
cd agent
go test ./... -v
```

### Integration Tests

See [06-development/testing/](../../06-development/testing/) for integration test guides.

---

## Related Components

- [Core](../core/) - Receives data from agents
- [Risk Engine](../risk-engine/) - Analyzes collected data
- [Graph Engine](../graph-engine/) - Builds relationship graphs

---

## Common Operations

### Update Agent Configuration

```bash
# Edit ConfigMap
kubectl edit configmap -n ksam ksam-agent-config

# Restart agents
kubectl rollout restart daemonset -n ksam ksam-agent
```

### Rotate Certificates

```bash
# Generate new certificates
./scripts/generate_certs.sh

# Update Secret
kubectl create secret generic ksam-agent-certs \
  --from-file=agent.crt=certs/agent.crt \
  --from-file=agent.key=certs/agent.key \
  --from-file=ca.crt=certs/ca.crt \
  -n ksam \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart agents
kubectl rollout restart daemonset -n ksam ksam-agent
```

### Scale Collection Interval

```bash
# Update interval in ConfigMap
kubectl patch configmap ksam-agent-config -n ksam \
  -p '{"data":{"COLLECTION_INTERVAL":"60s"}}'

# Restart to apply
kubectl rollout restart daemonset -n ksam ksam-agent
```

---

**Last Updated**: December 16, 2025
**Status**: ✅ Production Ready
