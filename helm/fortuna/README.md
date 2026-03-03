# Fortuna Helm Chart

Helm chart for Fortuna – Kubernetes security and risk management (SBOM, CVE, PCE, Runtime Signals).

## Requirements

- Kubernetes 1.24+
- Helm 3
- **mTLS secrets** must exist before install: `fortuna-core-tls`, `fortuna-agent-tls`, `fortuna-ca-cert`, `fortuna-webhook-tls` (e.g. `./scripts/utils/create_mtls_secret.sh`)
- **PostgreSQL** and **NATS** running in the same namespace (or configure `core.env.databaseUrl` / `core.env.natsEndpoint` to point to your endpoints)

## Install

```bash
# From repo root
helm upgrade --install fortuna ./helm/fortuna -n fortuna --create-namespace

# Override image tag / registry
helm upgrade --install fortuna ./helm/fortuna -n fortuna --create-namespace \
  --set image.tag=v1.0.0 \
  --set image.registry=registry.company.com/fortuna

# Custom values file
helm upgrade --install fortuna ./helm/fortuna -n fortuna -f my-values.yaml
```

## Main values (values.yaml)

| Group | Key | Description |
|-------|-----|-------------|
| image | image.registry, image.tag, image.pullPolicy | Registry, tag, pull policy |
| core | core.replicaCount, core.env.*, core.resources | Core deployment |
| agent | agent.env.*, agent.resources | Agent DaemonSet |
| dashboard | dashboard.replicaCount, dashboard.service.type | Dashboard |
| tls | tls.coreSecretName, tls.agentSecretName, tls.caSecretName, tls.webhookSecretName | mTLS secret names |

## Uninstall

```bash
helm uninstall fortuna -n fortuna
```

Note: PVCs (PostgreSQL, NATS) and the namespace are not removed automatically.

## See also

- [deploy/README.md](../../deploy/README.md) – kubectl deploy
- [docs-prod/](../../docs-prod/) – Production docs
