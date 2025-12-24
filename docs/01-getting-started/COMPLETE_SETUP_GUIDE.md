# Fortuna - Complete Setup Guide (Fresh Environment)

**Purpose**: Step-by-step guide to build and deploy Fortuna on a completely new environment  
**Target**: Ubuntu VM with fresh installation  
**Time**: ~30-45 minutes

---

## 📋 Table of Contents

1. [Prerequisites](#prerequisites)
2. [Environment Setup](#environment-setup)
3. [Build Components](#build-components)
4. [Deploy Infrastructure](#deploy-infrastructure)
5. [Deploy Fortuna](#deploy-fortuna)
6. [Verification](#verification)
7. [Testing](#testing)
8. [Troubleshooting](#troubleshooting)

---

## 1. Prerequisites

### System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| **OS** | Ubuntu 20.04+ | Ubuntu 22.04 LTS |
| **RAM** | 4GB | 8GB+ |
| **CPU** | 2 cores | 4+ cores |
| **Disk** | 20GB free | 50GB+ free |
| **Network** | Internet access | Stable connection |

### Software Requirements

- **Kubernetes**: 1.28+ (kubeadm, kubelet, kubectl)
- **Container Runtime**: containerd or Docker
- **Go**: 1.24+ (for building)
- **Git**: Latest
- **Make**: Latest

---

## 2. Environment Setup

### Step 2.1: Install Kubernetes

**On Ubuntu VM:**

```bash
# 1. Update system
sudo apt-get update
sudo apt-get upgrade -y

# 2. Install prerequisites
sudo apt-get install -y \
    apt-transport-https \
    ca-certificates \
    curl \
    gpg \
    lsb-release

# 3. Disable swap
sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab

# 4. Configure kernel modules
cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

sudo modprobe overlay
sudo modprobe br_netfilter

cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

sudo sysctl --system

# 5. Install containerd
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt-get update
sudo apt-get install -y containerd.io

# Configure containerd
sudo mkdir -p /etc/containerd
sudo containerd config default | sudo tee /etc/containerd/config.toml
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
sudo systemctl restart containerd
sudo systemctl enable containerd

# 6. Install Kubernetes
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list

sudo apt-get update
sudo apt-get install -y kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl

# 7. Initialize cluster
sudo kubeadm init \
    --pod-network-cidr=10.244.0.0/16 \
    --apiserver-advertise-address=$(hostname -I | awk '{print $1}') \
    --ignore-preflight-errors=Swap \
    --cri-socket=unix:///var/run/containerd/containerd.sock

# 8. Setup kubeconfig
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config

# 9. Install CNI (Flannel)
kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml

# 10. Wait for cluster to be ready
kubectl wait --for=condition=ready node --all --timeout=300s

# 11. Verify
kubectl get nodes
# Should show: Ready
```

**Expected Output:**
```
NAME     STATUS   ROLES           AGE   VERSION
ubuntu   Ready    control-plane   2m    v1.28.x
```

---

### Step 2.2: Install Go (for building)

```bash
# Install Go 1.24
cd /tmp
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify
go version
# Should show: go version go1.24.0 linux/amd64
```

---

### Step 2.3: Install Git and Make

```bash
sudo apt-get install -y git make
```

---

## 3. Build Components

### Step 3.1: Clone Repository

```bash
# Clone Fortuna repository
cd ~
git clone <repository-url> fortuna
# Or if you have the code locally, copy it to VM
cd fortuna
```

### Step 3.2: Build Core Component

```bash
cd ~/fortuna/core

# Download dependencies
go mod download

# Build
go build -o ../bin/fortuna-core \
    -ldflags="-w -s -X main.BuildVersion=v1.0.0 -X main.BuildCommit=$(git rev-parse --short HEAD 2>/dev/null || echo 'dev')" \
    ./cmd/main.go

# Verify
ls -lh ../bin/fortuna-core
# Should show: ~20-30MB binary
```

### Step 3.3: Build Agent Component

```bash
cd ~/fortuna/agent

# Download dependencies
go mod download

# Build
go build -o ../bin/fortuna-agent \
    -ldflags="-w -s -X main.BuildVersion=v1.0.0 -X main.BuildCommit=$(git rev-parse --short HEAD 2>/dev/null || echo 'dev')" \
    ./cmd/main.go

# Verify
ls -lh ../bin/fortuna-agent
# Should show: ~15-25MB binary
```

### Step 3.4: Build Docker Images (Optional)

If you want to use Docker images instead of local binaries:

```bash
cd ~/fortuna

# Build Core image
cd core
docker build -t fortuna-core:latest .

# Build Agent image
cd ../agent
docker build -t fortuna-agent:latest .

# Verify
docker images | grep fortuna
```

---

## 4. Deploy Infrastructure

### Step 4.1: Create Namespace

```bash
kubectl create namespace fortuna
```

### Step 4.2: Deploy PostgreSQL with Apache AGE

```bash
cd ~/fortuna

# Apply PostgreSQL deployment
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml

# Wait for PostgreSQL to be ready
kubectl wait --for=condition=ready pod -l app=postgres -n fortuna --timeout=300s

# Verify
kubectl get pods -n fortuna -l app=postgres
# Should show: Running
```

### Step 4.3: Deploy NATS JetStream

```bash
# Apply NATS deployment
kubectl apply -f deploy/infrastructure/nats.yaml

# Wait for NATS to be ready
kubectl wait --for=condition=ready pod -l app=nats -n fortuna --timeout=300s

# Verify
kubectl get pods -n fortuna -l app=nats
# Should show: Running
```

### Step 4.4: Setup Database Schema

```bash
# Get PostgreSQL pod name
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# Wait for PostgreSQL to be ready
kubectl exec -n fortuna $POSTGRES_POD -- pg_isready

# Create database (if not exists)
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -c "CREATE DATABASE fortuna;" || echo "Database may already exist"

# Run migrations (if you have migration files)
# kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna -f /path/to/migrations.sql
```

---

## 5. Deploy Fortuna

### Step 5.1: Deploy RBAC

```bash
cd ~/fortuna
kubectl apply -f deploy/fortuna-rbac.yaml

# Verify
kubectl get clusterrole,clusterrolebinding | grep fortuna
```

### Step 5.2: Generate Certificates (for mTLS)

```bash
# Create certs directory
mkdir -p ~/fortuna/.certs

# Generate CA
openssl genrsa -out ~/fortuna/.certs/ca.key 4096
openssl req -new -x509 -days 365 -key ~/fortuna/.certs/ca.key -out ~/fortuna/.certs/ca.crt \
    -subj "/CN=Fortuna CA"

# Generate server certificate
openssl genrsa -out ~/fortuna/.certs/server.key 4096
openssl req -new -key ~/fortuna/.certs/server.key -out ~/fortuna/.certs/server.csr \
    -subj "/CN=fortuna-core.fortuna.svc.cluster.local"
openssl x509 -req -days 365 -in ~/fortuna/.certs/server.csr -CA ~/fortuna/.certs/ca.crt \
    -CAkey ~/fortuna/.certs/ca.key -CAcreateserial -out ~/fortuna/.certs/server.crt

# Generate client certificate
openssl genrsa -out ~/fortuna/.certs/client.key 4096
openssl req -new -key ~/fortuna/.certs/client.key -out ~/fortuna/.certs/client.csr \
    -subj "/CN=fortuna-agent"
openssl x509 -req -days 365 -in ~/fortuna/.certs/client.csr -CA ~/fortuna/.certs/ca.crt \
    -CAkey ~/fortuna/.certs/ca.key -CAcreateserial -out ~/fortuna/.certs/client.crt

# Create Kubernetes secrets
kubectl create secret generic fortuna-core-server-tls \
    --from-file=tls.crt=~/fortuna/.certs/server.crt \
    --from-file=tls.key=~/fortuna/.certs/server.key \
    --from-file=ca.crt=~/fortuna/.certs/ca.crt \
    -n fortuna

kubectl create secret generic fortuna-agent-client-tls \
    --from-file=tls.crt=~/fortuna/.certs/client.crt \
    --from-file=tls.key=~/fortuna/.certs/client.key \
    --from-file=ca.crt=~/fortuna/.certs/ca.crt \
    -n fortuna
```

### Step 5.3: Deploy Core Component

**Option A: Using Local Binary (Development)**

```bash
# Create ConfigMap with binary
kubectl create configmap fortuna-core-binary \
    --from-file=fortuna-core=~/fortuna/bin/fortuna-core \
    -n fortuna

# Create deployment YAML (customize as needed)
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fortuna-core
  namespace: fortuna
spec:
  replicas: 1
  selector:
    matchLabels:
      app: fortuna-core
  template:
    metadata:
      labels:
        app: fortuna-core
    spec:
      containers:
      - name: fortuna-core
        image: busybox:latest
        command: ["/fortuna-core"]
        args: []
        volumeMounts:
        - name: binary
          mountPath: /fortuna-core
          subPath: fortuna-core
        env:
        - name: DATABASE_URL
          value: "postgres://postgres:postgres@postgres:5432/fortuna?sslmode=disable"
        - name: NATS_ENDPOINT
          value: "nats://nats:4222"
        - name: GRPC_PORT
          value: "9090"
        - name: HTTP_PORT
          value: "8080"
        - name: TLS_ENABLED
          value: "true"
        - name: TLS_CERT_PATH
          value: "/etc/fortuna/tls/server/tls.crt"
        - name: TLS_KEY_PATH
          value: "/etc/fortuna/tls/server/tls.key"
        - name: TLS_CA_CERT_PATH
          value: "/etc/fortuna/tls/server/ca.crt"
        volumeMounts:
        - name: server-tls
          mountPath: /etc/fortuna/tls/server
          readOnly: true
        volumes:
        - name: binary
          configMap:
            name: fortuna-core-binary
        - name: server-tls
          secret:
            secretName: fortuna-core-server-tls
EOF
```

**Option B: Using Docker Image (Recommended)**

```bash
# If you built Docker images, load them into cluster
# For minikube:
# minikube image load fortuna-core:latest

# Or use existing deployment file
kubectl apply -f deploy/fortuna-core-deployment.yaml

# Wait for Core to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=core -n fortuna --timeout=300s
```

### Step 5.4: Deploy Agent Component (if using Agent-Based)

```bash
# Apply Agent DaemonSet
kubectl apply -f deploy/fortuna-agent-daemonset.yaml

# Wait for Agent pods
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=agent -n fortuna --timeout=300s

# Verify
kubectl get pods -n fortuna -l app.kubernetes.io/component=agent
```

### Step 5.5: Deploy Core Service

```bash
kubectl apply -f deploy/core-service.yaml

# Verify
kubectl get svc -n fortuna
```

---

## 6. Verification

### Step 6.1: Check All Pods

```bash
kubectl get pods --all-namespaces

# Expected output:
# NAMESPACE     NAME                          READY   STATUS    RESTARTS   AGE
# fortuna       postgres-xxx                   1/1     Running   0          5m
# fortuna       nats-xxx                       1/1     Running   0          5m
# fortuna       fortuna-core-xxx                1/1     Running   0          3m
# fortuna       fortuna-agent-xxx (if used)    1/1     Running   0          2m
```

### Step 6.2: Check Core Logs

```bash
# Get Core pod name
CORE_POD=$(kubectl get pods -n fortuna -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}')

# View logs
kubectl logs -n fortuna $CORE_POD --tail=50

# Expected: Should show startup messages, no errors
```

### Step 6.3: Check Database Connection

```bash
# Get PostgreSQL pod
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# Connect and check
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna -c "\dt"

# Should show tables if migrations ran
```

### Step 6.4: Check NATS Connection

```bash
# Get NATS pod
NATS_POD=$(kubectl get pods -n fortuna -l app=nats -o jsonpath='{.items[0].metadata.name}')

# Check NATS status
kubectl exec -n fortuna $NATS_POD -- nats server info
```

### Step 6.5: Test API Endpoint

```bash
# Port forward to Core API
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &

# Test health endpoint
curl http://localhost:8080/health

# Expected: {"status":"ok"} or similar

# Test API (if auth disabled)
curl http://localhost:8080/api/v1/clusters

# Stop port forward
kill %1
```

---

## 7. Testing

### Step 7.1: Create Test Pod

```bash
# Create a test pod
kubectl run test-nginx --image=nginx:1.21 -n default

# Wait for pod to be ready
kubectl wait --for=condition=ready pod test-nginx -n default --timeout=60s
```

### Step 7.2: Verify SBOM Generation (if Agent-Based)

```bash
# Check Agent logs
AGENT_POD=$(kubectl get pods -n fortuna -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}')
kubectl logs -n fortuna $AGENT_POD --tail=50 | grep -i sbom

# Check Core logs for SBOM received
CORE_POD=$(kubectl get pods -n fortuna -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}')
kubectl logs -n fortuna $CORE_POD --tail=50 | grep -i sbom
```

### Step 7.3: Verify Database

```bash
# Check if SBOM was stored
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna -c "SELECT COUNT(*) FROM sboms;"

# Check insights
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna -c "SELECT COUNT(*) FROM insights WHERE status='active';"
```

---

## 8. Troubleshooting

### Issue: Pods not starting

**Check:**
```bash
# Describe pod
kubectl describe pod <pod-name> -n fortuna

# Check events
kubectl get events -n fortuna --sort-by='.lastTimestamp'

# Check logs
kubectl logs <pod-name> -n fortuna
```

### Issue: Database connection failed

**Check:**
```bash
# Verify PostgreSQL is running
kubectl get pods -n fortuna -l app=postgres

# Check PostgreSQL logs
kubectl logs -n fortuna -l app=postgres

# Test connection
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n fortuna $POSTGRES_POD -- pg_isready
```

### Issue: NATS connection failed

**Check:**
```bash
# Verify NATS is running
kubectl get pods -n fortuna -l app=nats

# Check NATS logs
kubectl logs -n fortuna -l app=nats

# Test connection
NATS_POD=$(kubectl get pods -n fortuna -l app=nats -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n fortuna $NATS_POD -- nats server ping
```

### Issue: Core not receiving SBOMs

**Check:**
```bash
# Check Agent logs
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=100

# Check Core gRPC logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=100 | grep -i grpc

# Check mTLS certificates
kubectl get secrets -n fortuna | grep tls
```

---

## 📊 Complete Deployment Checklist

### Infrastructure
- [ ] Kubernetes cluster initialized
- [ ] PostgreSQL deployed and running
- [ ] NATS deployed and running
- [ ] Database schema created
- [ ] Certificates generated

### Fortuna Components
- [ ] RBAC deployed
- [ ] Core component built
- [ ] Core deployed and running
- [ ] Agent deployed (if using Agent-Based)
- [ ] Services created

### Verification
- [ ] All pods Running
- [ ] Core logs show no errors
- [ ] Database connection works
- [ ] NATS connection works
- [ ] API endpoint accessible
- [ ] Test pod created
- [ ] SBOM generation works (if applicable)

---

## 🎯 Next Steps

After successful deployment:

1. **Load CVE Database**:
   ```bash
   # Use CVE loader tool
   kubectl exec -n fortuna <core-pod> -- /app/cve-loader
   ```

2. **Access Dashboard** (if deployed):
   ```bash
   kubectl port-forward -n fortuna svc/fortuna-dashboard 3000:80
   # Open http://localhost:3000
   ```

3. **Run E2E Tests**:
   ```bash
   cd ~/fortuna
   ./scripts/testing/e2e/test_e2e_comprehensive.sh
   ```

---

## 📖 Related Documentation

- [Quick Start Guide](./QUICKSTART.md)
- [Architecture Overview](../02-architecture/README.md)
- [Component Documentation](../03-components/)
- [Development Guide](../04-development/README.md)

---

**Setup Complete!** 🎉

---

*Fortuna K8s Management Platform - Complete Setup Guide v1.0*

