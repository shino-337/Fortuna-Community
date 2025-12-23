# Deployment Scripts

Scripts for deploying, rebuilding, and managing Fortuna components.

---

## 📝 Scripts (7 total)

| Script | Purpose | Usage |
|--------|---------|-------|
| **deploy.sh** | Standard deployment | `./deploy.sh` |
| **deploy_full.sh** | Full deployment with all components | `./deploy_full.sh` |
| **rebuild_and_deploy.sh** | Rebuild images and redeploy | `./rebuild_and_deploy.sh` |
| **clean-rebuild-deploy.sh** | Clean, rebuild, and deploy | `./clean-rebuild-deploy.sh` |
| **deploy-and-test-webhook.sh** | Deploy admission webhook | `./deploy-and-test-webhook.sh` |
| **rebuild-deploy-v2-scorer.sh** | Rebuild V2 risk scorer | `./rebuild-deploy-v2-scorer.sh` |
| **fix-deployment-old-image.sh** | Fix deployment using old images | `./fix-deployment-old-image.sh` |

---

## 🚀 Quick Start

### Initial Deployment
```bash
./deploy.sh
```

### Full Deployment
```bash
./deploy_full.sh
```

### After Code Changes
```bash
./rebuild_and_deploy.sh
```

### Complete Clean Rebuild
```bash
./clean-rebuild-deploy.sh
```

---

## 📚 Detailed Usage

### deploy.sh
Standard deployment to Kubernetes:
```bash
# Deploy to default namespace (fortuna)
./deploy.sh

# Deploy to custom namespace
NAMESPACE=my-namespace ./deploy.sh
```

### deploy_full.sh
Deploys all Fortuna components including:
- Core controller
- PostgreSQL database
- NATS JetStream
- Dashboard (optional)
- Admission webhook (optional)

```bash
./deploy_full.sh
```

### rebuild_and_deploy.sh
Rebuilds Docker images and redeploys:
```bash
# Rebuild and deploy
./rebuild_and_deploy.sh

# Rebuild specific component
COMPONENT=core ./rebuild_and_deploy.sh
```

### clean-rebuild-deploy.sh
Complete clean rebuild:
1. Stops running pods
2. Cleans old images
3. Rebuilds from scratch
4. Redeploys

```bash
./clean-rebuild-deploy.sh
```

---

## 🔄 Typical Workflows

### First Time Setup
```bash
# 1. Setup minikube
../setup/start_minikube.sh

# 2. Deploy Fortuna
./deploy.sh

# 3. Setup database
../database/setup_database.sh

# 4. Verify
../setup/verify_deployment.sh
```

### Development Cycle
```bash
# 1. Make code changes
# 2. Rebuild and deploy
./rebuild_and_deploy.sh

# 3. Test
../testing/run_validation_tests.sh

# 4. Monitor
../monitoring/monitor_fortuna.sh
```

### Troubleshooting Deployment
```bash
# Clean everything and start fresh
./clean-rebuild-deploy.sh

# Verify deployment
../setup/verify_deployment.sh

# Check logs
kubectl logs -n fortuna -l app=fortuna-core --tail=50
```

---

## ⚙️ Environment Variables

### Common Variables
```bash
# Namespace
NAMESPACE=fortuna ./deploy.sh

# Image tag
IMAGE_TAG=v1.0.0 ./deploy.sh

# Registry
REGISTRY=myregistry.io ./deploy.sh
```

### Example
```bash
NAMESPACE=production \
IMAGE_TAG=v2.0.0 \
REGISTRY=prod.registry.io \
./deploy.sh
```

---

## 🚨 Troubleshooting

### Deployment Fails
```bash
# Check pod status
kubectl get pods -n fortuna

# Check events
kubectl get events -n fortuna --sort-by='.lastTimestamp'

# Check specific pod
kubectl describe pod <pod-name> -n fortuna
```

### Image Pull Errors
```bash
# Check image exists
docker images | grep fortuna

# Rebuild images
./rebuild_and_deploy.sh

# Or clean rebuild
./clean-rebuild-deploy.sh
```

### Pod Not Starting
```bash
# Check logs
kubectl logs -n fortuna <pod-name>

# Check resource limits
kubectl describe pod <pod-name> -n fortuna

# Verify dependencies (database, NATS)
kubectl get pods -n fortuna
```

---

## 📖 Related Documentation

- [Setup Scripts](../setup/README.md) - Initial setup
- [Database Scripts](../database/README.md) - Database management
- [Monitoring Scripts](../monitoring/README.md) - Monitor deployment

---

*Back to [Scripts README](../README.md)*

