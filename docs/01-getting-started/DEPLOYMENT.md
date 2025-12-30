# Fortuna Deployment Guide

**Last Validated Against Version:** v2.0.0  
**Last Updated:** 2025-12-27  
**Status:** Current

---

## Overview

This guide covers deployment of Fortuna K8s Management Platform on Kubernetes. Choose the deployment method that fits your environment:

- **Quick Deploy**: Fast deployment on fresh Ubuntu VM
- **Minikube**: Local development/testing environment
- **Production**: Full production deployment with best practices

---

## Prerequisites

### Required
- Kubernetes cluster (1.24+)
- `kubectl` configured
- Docker or container runtime
- PostgreSQL 15+ (or use provided deployment)
- NATS JetStream (or use provided deployment)

### Optional
- Minikube (for local development)
- Helm (for production deployments)

---

## Quick Deploy (Ubuntu VM)

Fast deployment reference for fresh Ubuntu VM.

### 1. Setup Kubernetes

```bash
# Use standalone script
sudo bash scripts/setup-k8s-standalone.sh

# Or follow COMPLETE_SETUP_GUIDE.md for manual setup
```

### 2. Build Components

```bash
# Build Core
cd core && go build -o ../bin/fortuna-core ./cmd/main.go

# Build Agent
cd ../agent && go build -o ../bin/fortuna-agent ./cmd/main.go
```

### 3. Deploy Infrastructure

```bash
# Create namespace
kubectl create namespace fortuna

# Deploy PostgreSQL
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl wait --for=condition=ready pod -l app=postgres -n fortuna --timeout=300s

# Deploy NATS
kubectl apply -f deploy/infrastructure/nats.yaml
kubectl wait --for=condition=ready pod -l app=nats -n fortuna --timeout=300s
```

### 4. Deploy Fortuna

```bash
# Deploy RBAC
kubectl apply -f deploy/fortuna-rbac.yaml

# Deploy Core
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=core -n fortuna --timeout=300s

# Deploy Agent (if using Agent-Based)
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
```

### 5. Verify

```bash
# Check all pods
kubectl get pods -n fortuna

# Check Core logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50

# Test API
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
curl http://localhost:8080/health
```

---

## Minikube Deployment

For local development and testing.

### Setup Minikube

```bash
# Start Minikube
minikube start

# Configure Docker for Minikube
eval $(minikube docker-env)
```

**Note:** Run `eval $(minikube docker-env)` in each new terminal session.

### Build Docker Images

```bash
# Build Core image
cd core
docker build -t fortuna-core:latest .
cd ..

# Build Agent image
cd agent
docker build -t fortuna-agent:latest .
cd ..

# Verify images
docker images | grep fortuna
```

### Deploy to Minikube

```bash
# Create namespace
kubectl create namespace fortuna

# Deploy infrastructure
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl apply -f deploy/infrastructure/nats.yaml

# Wait for infrastructure
kubectl wait --for=condition=ready pod -l app=postgres -n fortuna --timeout=300s
kubectl wait --for=condition=ready pod -l app=nats -n fortuna --timeout=300s

# Deploy Fortuna
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
```

### Verify Minikube Deployment

```bash
# Check pods
kubectl get pods -n fortuna

# Check deployments
kubectl get deployments -n fortuna

# Check services
kubectl get svc -n fortuna

# View logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=50
```

---

## Production Deployment

### 1. Prepare Images

Build and push images to your container registry:

```bash
# Build and tag
docker build -t your-registry/fortuna-core:v2.0.0 -f core/Dockerfile .
docker build -t your-registry/fortuna-agent:v2.0.0 -f agent/Dockerfile .

# Push to registry
docker push your-registry/fortuna-core:v2.0.0
docker push your-registry/fortuna-agent:v2.0.0
```

### 2. Configure Secrets

Create secrets for database, NATS, and certificates:

```bash
# Database credentials
kubectl create secret generic fortuna-db \
  --from-literal=username=postgres \
  --from-literal=password=your-secure-password \
  -n fortuna

# NATS credentials
kubectl create secret generic fortuna-nats \
  --from-literal=username=nats \
  --from-literal=password=your-nats-password \
  -n fortuna
```

### 3. Deploy Infrastructure

```bash
# PostgreSQL with high availability
kubectl apply -f deploy/infrastructure/postgresql-ha.yaml

# NATS JetStream cluster
kubectl apply -f deploy/infrastructure/nats-cluster.yaml
```

### 4. Deploy Fortuna

```bash
# Update deployment files with your image registry
# Then deploy
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
```

### 5. Configure Ingress

```bash
# Apply ingress configuration
kubectl apply -f deploy/ingress/fortuna-ingress.yaml
```

---

## Deployment Order

```
1. Kubernetes Setup
   └─> Cluster ready, kubectl configured
   
2. Infrastructure
   ├─> PostgreSQL (with Apache AGE)
   └─> NATS JetStream
   
3. Fortuna Components
   ├─> RBAC (ServiceAccounts, Roles, RoleBindings)
   ├─> Core (Deployment)
   └─> Agent (DaemonSet, optional)
   
4. Verification
   ├─> Pods Running
   ├─> Logs OK
   ├─> API Accessible
   └─> Database Connected
```

---

## Common Commands

### Check Status

```bash
# All resources
kubectl get all -n fortuna

# Pods
kubectl get pods -n fortuna

# Services
kubectl get svc -n fortuna

# Deployments
kubectl get deployments -n fortuna
```

### View Logs

```bash
# Core logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=100 -f

# Agent logs
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=100 -f

# PostgreSQL logs
kubectl logs -n fortuna -l app=postgres --tail=50

# NATS logs
kubectl logs -n fortuna -l app=nats --tail=50
```

### Restart Components

```bash
# Restart Core
kubectl rollout restart deployment/fortuna-core -n fortuna

# Restart Agent
kubectl rollout restart daemonset/fortuna-agent -n fortuna

# Check rollout status
kubectl rollout status deployment/fortuna-core -n fortuna
```

### Access Database

```bash
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna
```

---

## Troubleshooting

### Pods Not Starting

```bash
# Describe pod for events
kubectl describe pod <pod-name> -n fortuna

# Check logs
kubectl logs <pod-name> -n fortuna

# Check events
kubectl get events -n fortuna --sort-by='.lastTimestamp'
```

### Database Connection Failed

```bash
# Check PostgreSQL pod
kubectl get pods -n fortuna -l app=postgres

# Check PostgreSQL logs
kubectl logs -n fortuna -l app=postgres

# Test connection
kubectl exec -it -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -c "SELECT 1;"
```

### API Not Accessible

```bash
# Check service
kubectl get svc -n fortuna fortuna-core

# Port forward for testing
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080

# Test health endpoint
curl http://localhost:8080/health
```

### ImagePullBackOff Error

**Problem:** Kubernetes cannot find the image.

**Solutions:**
- For Minikube: Ensure `imagePullPolicy: Never` and build images in minikube's Docker daemon
- For Production: Ensure images are pushed to registry and credentials are configured

### Minikube: Images Not Found

```bash
# Set docker environment for minikube
eval $(minikube docker-env)

# Rebuild images
cd core && docker build -t fortuna-core:latest .
cd ../agent && docker build -t fortuna-agent:latest .
```

---

## Configuration

### Environment Variables

Core and Agent support configuration via environment variables. See `docs/01-getting-started/CONFIGURATION.md` for full list.

Key variables:
- `FORTUNA_DB_HOST` - Database host
- `FORTUNA_DB_NAME` - Database name
- `FORTUNA_NATS_URL` - NATS server URL
- `FORTUNA_LOG_LEVEL` - Logging level

### Update Deployment

After code changes:

```bash
# Rebuild images
docker build -t fortuna-core:latest -f core/Dockerfile .
docker build -t fortuna-agent:latest -f agent/Dockerfile .

# Restart deployments
kubectl rollout restart deployment/fortuna-core -n fortuna
kubectl rollout restart daemonset/fortuna-agent -n fortuna

# Monitor rollout
kubectl rollout status deployment/fortuna-core -n fortuna
```

---

## Success Indicators

Deployment is successful when:

1. ✅ All pods are `Running`
2. ✅ Deployments show `AVAILABLE`
3. ✅ Logs show normal startup (no errors)
4. ✅ API health endpoint returns `200 OK`
5. ✅ Database connection established
6. ✅ NATS connection established

---

## Next Steps

After successful deployment:

1. **Verify Services**: Check all pods and services are running
2. **Test API**: Access API endpoints and verify responses
3. **Run Tests**: Execute test suite: `cd tests && ./run-all-tests.sh`
4. **Configure Monitoring**: Set up monitoring and alerting (see operations guide)
5. **Review Security**: Ensure RBAC and network policies are configured

---

## Additional Resources

- [Complete Setup Guide](./COMPLETE_SETUP_GUIDE.md) - Detailed setup instructions
- [Configuration Guide](./CONFIGURATION.md) - All configuration options
- [Troubleshooting Guide](./TROUBLESHOOTING.md) - Common issues and solutions
- [Operations Guide](../05-operations/) - Production operations

---

**For detailed setup instructions, see:** [COMPLETE_SETUP_GUIDE.md](./COMPLETE_SETUP_GUIDE.md)


