# Getting Started with Fortuna

Welcome to the Fortuna Kubernetes Security Platform.

The project name is Fortuna. The public repository is `shino-337/Fortuna-Community`, and published GHCR images use `ghcr.io/shino-337/fortuna-community`.

## Documentation

| Document | Purpose |
|----------|---------|
| [QUICKSTART.md](./QUICKSTART.md) | Short path to a running stack |
| [INSTALLATION.md](./INSTALLATION.md) | Full install, runtime coverage, verification, and reset workflow |
| [ENVIRONMENT_REQUIREMENTS.md](./ENVIRONMENT_REQUIREMENTS.md) | Hardware, software, network requirements |
| [Production deployment](../05-operations/PRODUCTION_DEPLOYMENT.md) | Full production checklist and procedures |
| [Operations deployment](../05-operations/DEPLOYMENT.md) | Quick links, migrations, containerd pipeline |

## Quick Start

```bash
# 1. Ensure Kubernetes is ready
kubectl get nodes

# 2. Choose the image tag to deploy, then follow QUICKSTART.md
export FORTUNA_REGISTRY="ghcr.io/shino-337/fortuna-community"
export FORTUNA_VERSION="latest"

# 3. Verify after deploy
kubectl get pods -n fortuna
```

The full public image workflow is in [QUICKSTART.md](./QUICKSTART.md). Local build/deploy scripts are documented there as a developer path, not the default user path.

## Prerequisites

- Ubuntu 20.04+ or 22.04 LTS
- 4GB+ RAM (8GB recommended)
- 20GB+ free disk space
- Kubernetes 1.28+
- Go 1.24+ only when building images locally
- kubectl configured

## Verification Steps

```bash
# All pods running
kubectl get pods -n fortuna

# Core healthy
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50

# API accessible
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
curl http://localhost:8080/healthz

# Dashboard
kubectl port-forward --address 0.0.0.0 -n fortuna svc/fortuna-dashboard 8081:80
# Open http://localhost:8081
```

## Next Steps

1. **Load CVE Database:** `./scripts/utils/load-cve-data.sh`
2. **Access Dashboard:** Port-forward and open in browser
3. **Use the Dashboard:** [User guide](../04-user-guide/README.md)
4. **Review Architecture:** [Architecture docs](../02-architecture/ARCHITECTURE.md)
5. **Operations:** [Deployment guide](../05-operations/DEPLOYMENT.md)
