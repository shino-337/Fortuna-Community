# Fortuna Helm Charts

This directory contains Helm charts for deploying Fortuna on Kubernetes.

## Charts

### `fortuna/` (Recommended)

Production-ready Helm chart with:
- Core deployment with mTLS, webhook TLS, PCE scheduler
- Agent DaemonSet with containerd support
- RBAC configuration
- Flexible configuration via values.yaml

**Features**:
- ✅ mTLS for secure communication
- ✅ Webhook TLS certificates
- ✅ PCE scheduler enabled
- ✅ Auth enabled by default
- ✅ Health probes configured
- ✅ Resource limits optimized
- ✅ Containerd socket support

### `ksam/` (Legacy)

Legacy chart for backward compatibility. Use `fortuna/` chart for new deployments.

## Quick Start

### Install Fortuna Chart

```bash
# 1. Create namespace
kubectl create namespace fortuna

# 2. Create secrets (mTLS certificates)
# See deploy/certs/ for certificate generation

# 3. Install chart
helm install fortuna ./helm/fortuna \
  --namespace fortuna \
  --set core.image.pullPolicy=Never \
  --set agent.image.pullPolicy=Never
```

### Configuration

Edit `values.yaml` or override via `--set`:

```bash
helm install fortuna ./helm/fortuna \
  --namespace fortuna \
  --set core.database.url="postgres://user:pass@host:5432/fortuna" \
  --set core.nats.endpoint="nats://nats-client.fortuna.svc.cluster.local:4222" \
  --set core.image.tag="v1.0.0"
```

## Key Configuration

### Core

- **Database**: PostgreSQL connection string
- **NATS**: NATS endpoint (default: `nats://nats-client.fortuna.svc.cluster.local:4222`)
- **TLS**: mTLS enabled by default
- **Webhook TLS**: Separate certificates for admission webhook
- **PCE Scheduler**: Enabled by default (interval: 6h)
- **Auth**: JWT authentication enabled

### Agent

- **Core Endpoint**: gRPC endpoint for Core service
- **Containerd**: Socket path and namespace configured
- **TLS**: mTLS client certificates
- **Resources**: Optimized limits (1Gi memory, 500m CPU)

## Values Reference

See `helm/fortuna/values.yaml` for complete configuration options.

### Important Settings

```yaml
core:
  image:
    pullPolicy: Never  # For local development
    # pullPolicy: IfNotPresent  # For production
  
  nats:
    endpoint: "nats://nats-client.fortuna.svc.cluster.local:4222"
  
  tls:
    enabled: true
    certSecret: fortuna-core-tls
  
  webhookTls:
    enabled: true
    certSecret: fortuna-webhook-tls
  
  env:
    AUTH_ENABLED: "true"
    PCE_SCHEDULER_ENABLED: "true"
    PCE_SCHEDULER_INTERVAL: "6h"

agent:
  image:
    pullPolicy: Never  # For local development
  
  env:
    CLUSTER_ID: "minikube"
    SYNC_INTERVAL: "5m"
    CONTAINERD_SOCKET: "/run/containerd/containerd.sock"
    CONTAINERD_NAMESPACE: "k8s.io"
```

## Prerequisites

1. **Kubernetes Cluster**: 1.24+ with containerd
2. **PostgreSQL**: Database instance (can use included postgresql chart)
3. **NATS**: NATS JetStream cluster (can use included nats chart)
4. **Certificates**: mTLS certificates (see `deploy/certs/`)

## Notes

- Core deployment requires control-plane node (nodeSelector + tolerations)
- Agent DaemonSet runs on all nodes
- All components use mTLS for secure communication
- Metrics endpoint available at `/metrics` (Prometheus format)
- No Prometheus/Grafana deployment included (optional)

## Troubleshooting

### Image Pull Errors

For local development, set `imagePullPolicy: Never`:
```yaml
core:
  image:
    pullPolicy: Never
```

### TLS Errors

Ensure certificates are created and secrets exist:
```bash
kubectl get secrets -n fortuna | grep fortuna
```

### Health Check Failures

Check Core logs:
```bash
kubectl logs -n fortuna -l app.kubernetes.io/component=core
```
