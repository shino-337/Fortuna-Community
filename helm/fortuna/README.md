# Fortuna Helm Chart

Helm chart để deploy Fortuna - Kubernetes Security and Compliance Platform.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.0+
- Containerd runtime (for Agent)
- PostgreSQL database (can be deployed with chart or external)
- NATS (can be deployed with chart or external)
- mTLS certificates (see `scripts/create-mtls-secrets.sh`)

## Installation

### Quick Install

```bash
# Install with default values
helm install fortuna ./helm/fortuna --namespace fortuna --create-namespace

# Install with custom values
helm install fortuna ./helm/fortuna \
  --namespace fortuna \
  --create-namespace \
  --set core.image.tag=v1.0.0 \
  --set agent.image.tag=v1.0.0
```

### Install with Custom Values File

```bash
# Create custom values file
cat > my-values.yaml <<EOF
core:
  image:
    tag: v1.0.0
  database:
    url: "postgres://user:pass@external-db:5432/fortuna"

agent:
  image:
    tag: v1.0.0
EOF

# Install
helm install fortuna ./helm/fortuna \
  --namespace fortuna \
  --create-namespace \
  -f my-values.yaml
```

## Configuration

### Core Component

```yaml
core:
  enabled: true
  image:
    repository: fortuna-core
    tag: latest
    pullPolicy: IfNotPresent
  
  database:
    url: "postgres://postgres:postgres@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable"
  
  nats:
    endpoint: "nats://nats-client.fortuna.svc.cluster.local:4222"
  
  tls:
    enabled: true
    certSecret: fortuna-core-server-tls
```

### Agent Component

```yaml
agent:
  enabled: true
  image:
    repository: fortuna-agent
    tag: latest
    pullPolicy: IfNotPresent
  
  coreEndpoint: "fortuna-core.fortuna.svc.cluster.local:9090"
  
  tls:
    enabled: true
    certSecret: fortuna-agent-client-tls
  
  containerdSocket:
    enabled: true
    path: /run/containerd/containerd.sock
```

## mTLS Certificates

Before installing, create mTLS certificates:

```bash
# Generate certificates
./scripts/create-mtls-secrets.sh

# Or manually create secrets
kubectl create secret tls fortuna-core-server-tls \
  --cert=certs/core-server.crt \
  --key=certs/core-server.key \
  -n fortuna

kubectl create secret tls fortuna-agent-client-tls \
  --cert=certs/agent-client.crt \
  --key=certs/agent-client.key \
  -n fortuna
```

## Upgrading

```bash
# Upgrade with new values
helm upgrade fortuna ./helm/fortuna \
  --namespace fortuna \
  --set core.image.tag=v1.1.0

# Upgrade with values file
helm upgrade fortuna ./helm/fortuna \
  --namespace fortuna \
  -f my-values.yaml
```

## Uninstalling

```bash
helm uninstall fortuna --namespace fortuna
```

## Components

- **Core**: Fortuna Core Controller (Deployment)
- **Agent**: Fortuna Agent (DaemonSet)
- **RBAC**: ServiceAccounts, ClusterRoles, ClusterRoleBindings

## Values Reference

See `values.yaml` for all available configuration options.

### Key Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `core.enabled` | Enable Core component | `true` |
| `core.image.repository` | Core image repository | `fortuna-core` |
| `core.image.tag` | Core image tag | `latest` |
| `core.database.url` | Database connection URL | (see values.yaml) |
| `agent.enabled` | Enable Agent component | `true` |
| `agent.image.repository` | Agent image repository | `fortuna-agent` |
| `agent.image.tag` | Agent image tag | `latest` |
| `agent.coreEndpoint` | Core gRPC endpoint | (see values.yaml) |
| `rbac.create` | Create RBAC resources | `true` |

## Troubleshooting

### Check Pod Status

```bash
kubectl get pods -n fortuna -l app.kubernetes.io/name=fortuna
```

### Check Logs

```bash
# Core logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core

# Agent logs
kubectl logs -n fortuna -l app.kubernetes.io/component=agent
```

### Verify Certificates

```bash
kubectl get secrets -n fortuna | grep fortuna
```

## Production Recommendations

1. **Use versioned images**: Set `core.image.tag` and `agent.image.tag` to specific versions
2. **Use imagePullPolicy: IfNotPresent**: For production registries
3. **Configure resource limits**: Adjust based on your cluster capacity
4. **Enable TLS**: Set `core.tls.enabled: true` and `agent.tls.enabled: true`
5. **Use external database**: Set `postgresql.enabled: false` and configure `core.database.url`
6. **Use external NATS**: Set `nats.enabled: false` and configure `core.nats.endpoint`

## Support

For issues and questions, see:
- `docs/05-operations/DEPLOYMENT_ISSUES_COMPREHENSIVE.md`
- `docs/01-getting-started/DEPLOYMENT_QUICK_START.md`

