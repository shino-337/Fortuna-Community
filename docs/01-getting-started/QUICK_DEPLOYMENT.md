# Fortuna - Quick Deployment Reference

**Quick reference for deploying Fortuna on a fresh Ubuntu VM**

---

## 🚀 Quick Commands

### 1. Setup Kubernetes (One-time)

```bash
# Use standalone script
sudo bash scripts/setup-k8s-standalone.sh

# Or manual setup (see COMPLETE_SETUP_GUIDE.md)
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
kubectl get pods --all-namespaces

# Check Core logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50

# Test API
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
curl http://localhost:8080/health
```

---

## 📋 Deployment Order

```
1. Kubernetes Setup
   └─> kubeadm init
   
2. Infrastructure
   ├─> PostgreSQL
   └─> NATS
   
3. Fortuna Components
   ├─> RBAC
   ├─> Core
   └─> Agent (optional)
   
4. Verification
   ├─> Pods Running
   ├─> Logs OK
   └─> API Accessible
```

---

## 🔧 Common Commands

### Check Status
```bash
kubectl get pods --all-namespaces
kubectl get svc -n fortuna
kubectl get nodes
```

### View Logs
```bash
# Core
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=100 -f

# Agent
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=100 -f

# PostgreSQL
kubectl logs -n fortuna -l app=postgres --tail=50

# NATS
kubectl logs -n fortuna -l app=nats --tail=50
```

### Restart Components
```bash
# Restart Core
kubectl rollout restart deployment/fortuna-core -n fortuna

# Restart Agent
kubectl rollout restart daemonset/fortuna-agent -n fortuna
```

### Access Database
```bash
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna
```

---

## 🚨 Quick Troubleshooting

### Pods not starting?
```bash
kubectl describe pod <pod-name> -n fortuna
kubectl logs <pod-name> -n fortuna
```

### Database connection failed?
```bash
kubectl get pods -n fortuna -l app=postgres
kubectl logs -n fortuna -l app=postgres
```

### API not accessible?
```bash
kubectl get svc -n fortuna
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080
curl http://localhost:8080/health
```

---

**For detailed steps, see**: [COMPLETE_SETUP_GUIDE.md](./COMPLETE_SETUP_GUIDE.md)




