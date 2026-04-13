# Getting Started with Fortuna

Welcome to the Fortuna Kubernetes Security Platform!

## Documentation

| Document | Purpose |
|----------|---------|
| [QUICKSTART.md](./QUICKSTART.md) | Quick 10-minute deployment guide |
| [DEPLOYMENT.md](./DEPLOYMENT.md) | Complete deployment guide (Quick/Minikube/Production) |
| [ENVIRONMENT_REQUIREMENTS.md](./ENVIRONMENT_REQUIREMENTS.md) | Hardware, software, network requirements |

## Quick Start

```bash
# 1. Ensure Kubernetes is ready
kubectl get nodes

# 2. Build and deploy
./scripts/build-and-deploy.sh

# 3. Verify
kubectl get pods -n fortuna
```

## Prerequisites

- Ubuntu 20.04+ or 22.04 LTS
- 4GB+ RAM (8GB recommended)
- 20GB+ free disk space
- Kubernetes 1.28+
- Go 1.24+ (for building)
- kubectl configured

## Verification Steps

```bash
# All pods running
kubectl get pods -n fortuna

# Core healthy
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50

# API accessible
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
curl http://localhost:8080/health

# Dashboard
kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80
# Open http://localhost:8081
```

## Next Steps

1. **Load CVE Database:** `go build -o cve-loader core/cmd/cve-loader/main.go && ./cve-loader --source /cve-data/all`
2. **Access Dashboard:** Port-forward and open in browser
3. **Review Architecture:** [Architecture docs](../02-architecture/ARCHITECTURE.md)
4. **Operations:** [Deployment guide](../05-operations/DEPLOYMENT.md)
