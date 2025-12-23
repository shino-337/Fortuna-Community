# Setup Scripts

**8 scripts** for initial setup, minikube, and environment configuration.

---

## 📝 Scripts

| Script | Purpose |
|--------|---------|
| **start_minikube.sh** | Start minikube with Fortuna config |
| **quick_start_minikube.sh** | Quick minikube setup |
| **setup_test_environment.sh** | Setup test environment |
| **start_portforwards.sh** | Start all port forwards |
| **start_dashboard_portforward.sh** | Port forward dashboard only |
| **check_minikube_resources.sh** | Check minikube resources |
| **verify_deployment.sh** | Verify Fortuna deployment |
| **verify-dependencies.sh** | Verify dependencies installed |

---

## 🚀 Quick Start

### Complete Setup
```bash
# 1. Verify dependencies
./verify-dependencies.sh

# 2. Start minikube
./start_minikube.sh

# 3. Deploy Fortuna
../deployment/deploy.sh

# 4. Verify
./verify_deployment.sh

# 5. Start port forwards
./start_portforwards.sh
```

---

## 📚 Detailed Usage

### start_minikube.sh
Start minikube with optimal configuration:
```bash
# Start with defaults
./start_minikube.sh

# Custom resources
CPUS=4 MEMORY=8192 ./start_minikube.sh
```

Configuration:
- CPUs: 2-4 (default: 2)
- Memory: 4096-8192 MB (default: 4096)
- Driver: docker/virtualbox
- Kubernetes version: 1.28+

---

### quick_start_minikube.sh
Quick minikube setup with minimal resources:
```bash
./quick_start_minikube.sh
```

Starts minikube with:
- 2 CPUs
- 4GB RAM
- Minimal addons

---

### setup_test_environment.sh
Setup complete test environment:
```bash
./setup_test_environment.sh
```

Actions:
1. Verifies dependencies
2. Starts minikube
3. Deploys Fortuna
4. Sets up database
5. Loads test data

---

### start_portforwards.sh
Start all port forwards:
```bash
./start_portforwards.sh
```

Port forwards:
- API: localhost:8080 → fortuna-core:8080
- Dashboard: localhost:3000 → fortuna-dashboard:80
- PostgreSQL: localhost:5432 → postgres:5432
- NATS: localhost:4222 → nats:4222

---

### start_dashboard_portforward.sh
Port forward dashboard only:
```bash
./start_dashboard_portforward.sh
```

Access dashboard at: http://localhost:3000

---

### check_minikube_resources.sh
Check minikube resource usage:
```bash
./check_minikube_resources.sh
```

Shows:
- CPU usage
- Memory usage
- Disk usage
- Pod count

---

### verify_deployment.sh
Verify Fortuna deployment:
```bash
./verify_deployment.sh
```

Checks:
- All pods running
- Services available
- Database accessible
- NATS connected

---

### verify-dependencies.sh
Verify required dependencies:
```bash
./verify-dependencies.sh
```

Checks for:
- kubectl
- docker
- minikube
- helm (optional)
- jq (optional)

---

## 🔄 Setup Workflows

### First Time Setup
```bash
# 1. Check dependencies
./verify-dependencies.sh

# 2. Start minikube
./start_minikube.sh

# 3. Deploy
../deployment/deploy_full.sh

# 4. Setup database
../database/setup_database.sh

# 5. Verify
./verify_deployment.sh

# 6. Port forwards
./start_portforwards.sh

# 7. Access dashboard
open http://localhost:3000
```

### Daily Development
```bash
# Start minikube (if stopped)
./start_minikube.sh

# Start port forwards
./start_portforwards.sh

# Verify everything is running
./verify_deployment.sh
```

### After Restart
```bash
# Start minikube
minikube start

# Verify deployment
./verify_deployment.sh

# Start port forwards
./start_portforwards.sh
```

---

## ⚙️ Configuration

### Minikube Resources
```bash
# Minimum (testing)
CPUS=2 MEMORY=4096 ./start_minikube.sh

# Recommended (development)
CPUS=4 MEMORY=8192 ./start_minikube.sh

# High performance
CPUS=6 MEMORY=16384 ./start_minikube.sh
```

### Port Forwards
Edit `start_portforwards.sh` to customize ports:
```bash
# API port
API_PORT=8080

# Dashboard port
DASHBOARD_PORT=3000

# Database port
DB_PORT=5432
```

---

## 🚨 Troubleshooting

### Minikube Won't Start
```bash
# Delete and recreate
minikube delete
./start_minikube.sh

# Check Docker
docker info

# Check available resources
docker system df
```

### Port Forward Fails
```bash
# Kill existing port forwards
pkill -f "kubectl port-forward"

# Restart
./start_portforwards.sh

# Check pod status
kubectl get pods -n fortuna
```

### Deployment Verification Fails
```bash
# Check pod status
kubectl get pods -n fortuna

# Check events
kubectl get events -n fortuna --sort-by='.lastTimestamp'

# Check logs
kubectl logs -n fortuna -l app=fortuna-core --tail=50

# Redeploy if needed
../deployment/clean-rebuild-deploy.sh
```

---

## 📖 Related Documentation

- [Quick Start Guide](../../docs/01-getting-started/QUICKSTART.md)
- [Minikube Setup](../../docs/01-getting-started/MINIKUBE_SETUP.md)
- [Deployment Scripts](../deployment/README.md)

---

*Back to [Scripts README](../README.md)*

