# Getting Started with Fortuna

**Welcome to Fortuna K8s Management Platform!**

This section provides comprehensive guides for setting up and deploying Fortuna on a fresh environment.

---

## 📚 Documentation Index

### 🚀 Quick Start Guides

1. **[Deployment Guide](./DEPLOYMENT.md)**
   - Complete deployment guide
   - Quick deploy, Minikube, Production
   - Troubleshooting and verification
   - **Time**: 15-20 minutes to read

2. **[Complete Setup Guide](./COMPLETE_SETUP_GUIDE.md)**
   - Step-by-step instructions
   - Detailed explanations
   - Full deployment process
   - **Time**: 30-45 minutes to complete

3. **[Multi-Node K8s Deployment Guide](./MULTI_NODE_K8S_DEPLOYMENT.md)** ⭐ **NEW**
   - Production-ready multi-node cluster
   - Master-worker architecture
   - containerd runtime
   - **Time**: 45-60 minutes to complete

4. **[Environment Requirements](./ENVIRONMENT_REQUIREMENTS.md)** ⭐ **NEW**
   - Complete hardware/software specs
   - Network requirements
   - Resource limits
   - Deployment scenarios

---

## 🎯 Choose Your Path

### Path 1: Automated Deployment (Recommended)

**For**: Quick setup with automation

```bash
# 1. Ensure Kubernetes is ready
kubectl get nodes

# 2. Run automated script
./scripts/build-and-deploy.sh

# 3. Verify
kubectl get pods -n fortuna
```

**See**: [Complete Setup Guide - Automated](./COMPLETE_SETUP_GUIDE.md#automated-deployment)

---

### Path 2: Manual Step-by-Step

**For**: Learning, customization, troubleshooting

1. **[Environment Setup](./COMPLETE_SETUP_GUIDE.md#2-environment-setup)**
   - Install Kubernetes
   - Install Go
   - Install prerequisites

2. **[Build Components](./COMPLETE_SETUP_GUIDE.md#3-build-components)**
   - Build Core
   - Build Agent
   - Build Docker images (optional)

3. **[Deploy Infrastructure](./COMPLETE_SETUP_GUIDE.md#4-deploy-infrastructure)**
   - PostgreSQL with Apache AGE
   - NATS JetStream

4. **[Deploy Fortuna](./COMPLETE_SETUP_GUIDE.md#5-deploy-fortuna)**
   - RBAC
   - Core component
   - Agent component

5. **[Verification](./COMPLETE_SETUP_GUIDE.md#6-verification)**
   - Check pods
   - Check logs
   - Test API

**See**: [Complete Setup Guide](./COMPLETE_SETUP_GUIDE.md)

---

## 📋 Prerequisites Checklist

Before starting, ensure you have:

- [ ] **Ubuntu 20.04+** or **22.04 LTS**
- [ ] **4GB+ RAM** (8GB recommended)
- [ ] **20GB+ free disk space**
- [ ] **Internet access** (for downloads)
- [ ] **Root/sudo access**
- [ ] **Kubernetes 1.28+** installed
- [ ] **Go 1.24+** installed (for building)
- [ ] **kubectl** configured

---

## 🛠️ Available Scripts

### Build and Deploy

**For Multi-Node Production Cluster:**
```bash
# Build and deploy to multi-node K8s cluster
./scripts/build-and-deploy-multinode.sh

# With custom registry
REGISTRY=docker.io/yourusername ./scripts/build-and-deploy-multinode.sh

# With custom image tag
IMAGE_TAG=v1.0.0 ./scripts/build-and-deploy-multinode.sh
```

**For Local/Development:**
```bash
# Full automated deployment (minikube/local)
./scripts/build-and-deploy.sh

# Skip build (use existing binaries)
./scripts/build-and-deploy.sh --skip-build

# Skip infrastructure (already deployed)
./scripts/build-and-deploy.sh --skip-infra

# Skip deployment (only build)
./scripts/build-and-deploy.sh --skip-deploy
```

### Kubernetes Setup

```bash
# Setup Kubernetes on Ubuntu VM (single node)
sudo bash scripts/setup-k8s-standalone.sh
```

---

## 📖 Documentation Structure

```
docs/01-getting-started/
├── README.md                    # This file
├── DEPLOYMENT.md                # Complete deployment guide
├── QUICKSTART.md                # Quick start tutorial
└── COMPLETE_SETUP_GUIDE.md      # Detailed setup guide

scripts/
├── build-and-deploy.sh          # Automated deployment
└── setup-k8s-standalone.sh      # K8s setup script
```

---

## 🚀 Quick Start

### For Multi-Node Production Cluster

**1. Setup Kubernetes Cluster** (see [Multi-Node Guide](./MULTI_NODE_K8S_DEPLOYMENT.md#2-cluster-setup))

**2. Build and Deploy**
```bash
# With registry
REGISTRY=docker.io/yourusername ./scripts/build-and-deploy-multinode.sh

# Or without registry (load images manually to nodes)
./scripts/build-and-deploy-multinode.sh
```

**3. Verify**
```bash
kubectl get pods -n fortuna
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50
```

### For Local/Development (Single Node)

**1. Setup Kubernetes**
```bash
sudo bash scripts/setup-k8s-standalone.sh
```

**2. Build and Deploy**
```bash
./scripts/build-and-deploy.sh
```

**3. Verify**
```bash
kubectl get pods -n fortuna
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50
```

---

## 🔍 Verification Steps

After deployment, verify:

1. **All pods are Running**:
   ```bash
   kubectl get pods -n fortuna
   ```

2. **Core is healthy**:
   ```bash
   kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50
   ```

3. **API is accessible**:
   ```bash
   kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
   curl http://localhost:8080/health
   ```

4. **Database is connected**:
   ```bash
   POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
   kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna -c "\dt"
   ```

---

## 🚨 Common Issues

### Pods not starting?

```bash
# Check events
kubectl get events -n fortuna --sort-by='.lastTimestamp'

# Describe pod
kubectl describe pod <pod-name> -n fortuna

# Check logs
kubectl logs <pod-name> -n fortuna
```

### Database connection failed?

```bash
# Check PostgreSQL
kubectl get pods -n fortuna -l app=postgres
kubectl logs -n fortuna -l app=postgres
```

### API not accessible?

```bash
# Check service
kubectl get svc -n fortuna

# Port forward
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080
```

**See**: [Troubleshooting Section](./COMPLETE_SETUP_GUIDE.md#8-troubleshooting)

---

## 📊 Deployment Time Estimates

| Step | Time | Notes |
|------|------|-------|
| **Kubernetes Setup** | 10-15 min | One-time setup |
| **Build Components** | 2-5 min | Depends on machine |
| **Deploy Infrastructure** | 5-10 min | PostgreSQL + NATS |
| **Deploy Fortuna** | 2-5 min | Core + Agent |
| **Verification** | 2-3 min | Testing |
| **Total** | **20-40 min** | First-time setup |

---

## 🎯 Next Steps

After successful deployment:

1. **Load CVE Database**:
   - Use CVE loader tool
   - Import CVE data from OSV.dev

2. **Configure Policies**:
   - Create policy templates
   - Enable policy instances

3. **Access Dashboard**:
   - Port forward dashboard service
   - Open in browser

4. **Run Tests**:
   - Create test pods
   - Verify SBOM generation
   - Check insights

**See**: [Architecture Documentation](../02-architecture/README.md)

---

## 📞 Getting Help

- **Documentation**: Check [Complete Setup Guide](./COMPLETE_SETUP_GUIDE.md)
- **Troubleshooting**: See [Troubleshooting Section](./COMPLETE_SETUP_GUIDE.md#8-troubleshooting)
- **Architecture**: Review [Architecture Docs](../02-architecture/README.md)

---

## ✅ Deployment Checklist

### Pre-Deployment
- [ ] Kubernetes cluster ready
- [ ] kubectl configured
- [ ] Go installed (for building)
- [ ] Internet access available

### Deployment
- [ ] Components built
- [ ] Infrastructure deployed
- [ ] Certificates generated
- [ ] Fortuna components deployed

### Post-Deployment
- [ ] All pods Running
- [ ] Core logs show no errors
- [ ] API accessible
- [ ] Database connected
- [ ] NATS connected

---

**Ready to get started?** Choose your path above! 🚀

---

*Fortuna K8s Management Platform - Getting Started Guide v1.0*
