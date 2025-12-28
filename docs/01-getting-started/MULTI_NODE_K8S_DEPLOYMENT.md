# Fortuna - Multi-Node Kubernetes Deployment Guide

**Target**: Production-ready Kubernetes cluster with master and worker nodes using containerd  
**Architecture**: Master-Control-Plane + Worker Nodes  
**Container Runtime**: containerd

---

## 📋 Table of Contents

1. [Environment Requirements](#environment-requirements)
2. [Cluster Setup](#cluster-setup)
3. [Build Docker Images](#build-docker-images)
4. [Deploy Infrastructure](#deploy-infrastructure)
5. [Deploy Fortuna](#deploy-fortuna)
6. [Verification](#verification)
7. [Troubleshooting](#troubleshooting)

---

## 1. Environment Requirements

### 1.1 Hardware Requirements

#### Master Node (Control Plane)
| Component | Minimum | Recommended |
|-----------|---------|-------------|
| **CPU** | 2 cores | 4+ cores |
| **RAM** | 4GB | 8GB+ |
| **Disk** | 20GB | 50GB+ SSD |
| **Network** | 1 Gbps | 10 Gbps |

#### Worker Nodes
| Component | Minimum | Recommended |
|-----------|---------|-------------|
| **CPU** | 2 cores | 4+ cores |
| **RAM** | 4GB | 8GB+ |
| **Disk** | 20GB | 50GB+ SSD |
| **Network** | 1 Gbps | 10 Gbps |

**Total Cluster Resources**:
- **Minimum**: 1 Master + 1 Worker = 4 cores, 8GB RAM, 40GB disk
- **Recommended**: 1 Master + 2-3 Workers = 8-12 cores, 16-24GB RAM, 100GB+ disk

### 1.2 Software Requirements

#### All Nodes
- **OS**: Ubuntu 20.04 LTS or 22.04 LTS
- **Kernel**: 5.4+ (for containerd support)
- **Container Runtime**: containerd 1.7+
- **Kubernetes**: 1.28+ (kubeadm, kubelet, kubectl)
- **CNI Plugin**: Flannel, Calico, or Cilium
- **Network**: All nodes must be able to communicate

#### Build Machine (Optional - can be separate)
- **Go**: 1.24+
- **Docker**: 20.10+ (for building images)
- **Git**: Latest
- **Make**: Latest

### 1.3 Network Requirements

- **Pod Network CIDR**: 10.244.0.0/16 (default for Flannel)
- **Service CIDR**: 10.96.0.0/12 (default)
- **Master API Server**: Accessible from all nodes
- **Ports**:
  - **Master**: 6443 (API), 2379-2380 (etcd), 10250 (kubelet), 10259 (kube-scheduler), 10257 (kube-controller-manager)
  - **Worker**: 10250 (kubelet), 30000-32767 (NodePort services)

### 1.4 Storage Requirements

- **PostgreSQL**: 20GB+ persistent volume
- **NATS JetStream**: 10GB+ per replica (3 replicas = 30GB+)
- **Container Images**: ~2GB for Fortuna images
- **Logs**: 5GB+ for application logs

---

## 2. Cluster Setup

### 2.1 Prepare All Nodes

**Run on ALL nodes (master + workers):**

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
    lsb-release \
    net-tools \
    iproute2 \
    iptables \
    conntrack

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

# 5. Configure sysctl
cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

sudo sysctl --system
```

### 2.2 Install containerd on All Nodes

**Run on ALL nodes:**

```bash
# 1. Install containerd
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt-get update
sudo apt-get install -y containerd.io

# 2. Configure containerd
sudo mkdir -p /etc/containerd
sudo containerd config default | sudo tee /etc/containerd/config.toml

# 3. Enable systemd cgroup driver
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml

# 4. Restart containerd
sudo systemctl restart containerd
sudo systemctl enable containerd

# 5. Verify
sudo systemctl status containerd
```

### 2.3 Install Kubernetes on All Nodes

**Run on ALL nodes:**

```bash
# 1. Add Kubernetes repository
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list

# 2. Install Kubernetes components
sudo apt-get update
sudo apt-get install -y kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl

# 3. Verify
kubeadm version
kubectl version --client
```

### 2.4 Initialize Master Node

**Run ONLY on master node:**

```bash
# 1. Get master node IP
MASTER_IP=$(hostname -I | awk '{print $1}')
echo "Master IP: $MASTER_IP"

# 2. Initialize cluster
sudo kubeadm init \
    --pod-network-cidr=10.244.0.0/16 \
    --apiserver-advertise-address=$MASTER_IP \
    --cri-socket=unix:///var/run/containerd/containerd.sock \
    --control-plane-endpoint=$MASTER_IP

# 3. Setup kubeconfig for root
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config

# 4. Save join command (you'll need this for workers)
kubeadm token create --print-join-command > /tmp/join-command.sh
cat /tmp/join-command.sh
# Example output:
# kubeadm join 192.168.1.100:6443 --token abc123... --discovery-token-ca-cert-hash sha256:...
```

### 2.5 Install CNI Plugin (Flannel)

**Run ONLY on master node:**

```bash
# Install Flannel
kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml

# Wait for CNI to be ready
kubectl wait --for=condition=ready pod --all -n kube-flannel --timeout=300s

# Verify
kubectl get nodes
# Should show master as Ready
```

### 2.6 Join Worker Nodes

**Run on EACH worker node:**

```bash
# Use the join command from master node
# Replace with your actual join command
sudo kubeadm join 192.168.1.100:6443 \
    --token <token> \
    --discovery-token-ca-cert-hash sha256:<hash> \
    --cri-socket=unix:///var/run/containerd/containerd.sock

# If token expired, generate new one on master:
# kubeadm token create --print-join-command
```

**Verify on master:**

```bash
kubectl get nodes
# Should show all nodes as Ready
```

---

## 3. Build Docker Images

### 3.1 Setup Build Environment

**On build machine (can be master or separate):**

```bash
# 1. Install Docker (for building images)
sudo apt-get update
sudo apt-get install -y docker.io
sudo systemctl start docker
sudo systemctl enable docker
sudo usermod -aG docker $USER
# Logout and login again

# 2. Install Go (if building from source)
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 3. Clone repository
git clone <repository-url> fortuna
cd fortuna
```

### 3.2 Build Core Image

```bash
cd fortuna

# Build Core image (from repository root)
# Note: Base image is Debian Bookworm Slim (not Alpine)
docker build \
    --build-arg FORTUNA_BUILD_VERSION=v1.0.0 \
    --build-arg FORTUNA_BUILD_COMMIT=$(git rev-parse --short HEAD) \
    --build-arg FORTUNA_BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
    -t fortuna-core:latest \
    -f core/Dockerfile .

# Verify
docker images | grep fortuna-core
```

### 3.3 Build Agent Image

```bash
# Build Agent image (from repository root)
# Note: Base image is Debian Bookworm Slim (not Alpine)
docker build \
    --build-arg FORTUNA_BUILD_VERSION=v1.0.0 \
    --build-arg FORTUNA_BUILD_COMMIT=$(git rev-parse --short HEAD) \
    --build-arg FORTUNA_BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
    -t fortuna-agent:latest \
    -f agent/Dockerfile .
```

# Verify
docker images | grep fortuna-agent
```

### 3.4 Push Images to Registry (Production)

**Option A: Docker Hub**

```bash
# Tag images
docker tag fortuna-core:latest <your-dockerhub-username>/fortuna-core:latest
docker tag fortuna-agent:latest <your-dockerhub-username>/fortuna-agent:latest

# Login
docker login

# Push
docker push <your-dockerhub-username>/fortuna-core:latest
docker push <your-dockerhub-username>/fortuna-agent:latest
```

**Option B: Private Registry**

```bash
# Tag images
docker tag fortuna-core:latest <registry-url>/fortuna-core:latest
docker tag fortuna-agent:latest <registry-url>/fortuna-agent:latest

# Push
docker push <registry-url>/fortuna-core:latest
docker push <registry-url>/fortuna-agent:latest
```

**Option C: Load to Nodes (Development)**

```bash
# Save images
docker save fortuna-core:latest -o fortuna-core.tar
docker save fortuna-agent:latest -o fortuna-agent.tar

# Copy to each node and load
# On each node:
docker load -i fortuna-core.tar
docker load -i fortuna-agent.tar
```

---

## 4. Deploy Infrastructure

### 4.1 Create Namespace

**On master node:**

```bash
kubectl create namespace fortuna
```

### 4.2 Deploy PostgreSQL

```bash
# Apply PostgreSQL deployment
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml

# Wait for PostgreSQL to be ready
kubectl wait --for=condition=ready pod -l app=postgres -n fortuna --timeout=300s

# Verify
kubectl get pods -n fortuna -l app=postgres
kubectl get pvc -n fortuna
```

### 4.3 Deploy NATS JetStream

```bash
# Apply NATS deployment
kubectl apply -f deploy/infrastructure/nats.yaml

# Wait for NATS to be ready
kubectl wait --for=condition=ready pod -l app=nats -n fortuna --timeout=300s

# Verify
kubectl get pods -n fortuna -l app=nats
kubectl get svc -n fortuna -l app=nats
```

### 4.4 Initialize Database

```bash
# Get PostgreSQL pod
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# Wait for PostgreSQL to be ready
kubectl exec -n fortuna $POSTGRES_POD -- pg_isready

# Create database (if needed)
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -c "CREATE DATABASE ksam;" || echo "Database may already exist"

# Run migrations (if you have migration files)
# kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -f /path/to/migrations.sql
```

---

## 5. Deploy Fortuna

### 5.1 Generate mTLS Certificates

**On master node:**

```bash
cd fortuna
mkdir -p .certs

# Generate CA
openssl genrsa -out .certs/ca.key 4096
openssl req -new -x509 -days 365 -key .certs/ca.key -out .certs/ca.crt \
    -subj "/CN=Fortuna CA"

# Generate server certificate
openssl genrsa -out .certs/server.key 4096
openssl req -new -key .certs/server.key -out .certs/server.csr \
    -subj "/CN=fortuna-core.fortuna.svc.cluster.local"
openssl x509 -req -days 365 -in .certs/server.csr -CA .certs/ca.crt \
    -CAkey .certs/ca.key -CAcreateserial -out .certs/server.crt

# Generate client certificate
openssl genrsa -out .certs/client.key 4096
openssl req -new -key .certs/client.key -out .certs/client.csr \
    -subj "/CN=fortuna-agent"
openssl x509 -req -days 365 -in .certs/client.csr -CA .certs/ca.crt \
    -CAkey .certs/ca.key -CAcreateserial -out .certs/client.crt

# Create Kubernetes secrets
kubectl create secret generic fortuna-core-server-tls \
    --from-file=tls.crt=.certs/server.crt \
    --from-file=tls.key=.certs/server.key \
    --from-file=ca.crt=.certs/ca.crt \
    -n fortuna

kubectl create secret generic fortuna-agent-client-tls \
    --from-file=tls.crt=.certs/client.crt \
    --from-file=tls.key=.certs/client.key \
    --from-file=ca.crt=.certs/ca.crt \
    -n fortuna
```

### 5.2 Update Deployment Files

**Update image references in deployment files:**

```bash
# Edit fortuna-core-deployment.yaml
# Change: image: fortuna-core:latest
# To: image: <your-registry>/fortuna-core:latest
# Or: imagePullPolicy: IfNotPresent (if loaded locally)

# Edit fortuna-agent-daemonset.yaml
# Change: image: fortuna-agent:latest
# To: image: <your-registry>/fortuna-agent:latest
# Or: imagePullPolicy: IfNotPresent (if loaded locally)
```

**For containerd socket access, ensure Agent DaemonSet has:**

```yaml
volumes:
  - name: containerd-socket
    hostPath:
      path: /run/containerd/containerd.sock
      type: Socket
```

### 5.3 Deploy RBAC

```bash
kubectl apply -f deploy/fortuna-rbac.yaml

# Verify
kubectl get clusterrole,clusterrolebinding | grep fortuna
```

### 5.4 Deploy Core

```bash
# Apply Core deployment
kubectl apply -f deploy/fortuna-core-deployment.yaml

# Wait for Core to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=core -n fortuna --timeout=300s

# Verify
kubectl get pods -n fortuna -l app.kubernetes.io/component=core
kubectl get svc -n fortuna -l app.kubernetes.io/component=core
```

### 5.5 Deploy Agent

```bash
# Apply Agent DaemonSet
kubectl apply -f deploy/fortuna-agent-daemonset.yaml

# Wait for Agent pods (one per worker node)
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=agent -n fortuna --timeout=300s

# Verify
kubectl get pods -n fortuna -l app.kubernetes.io/component=agent -o wide
# Should show one pod per worker node
```

---

## 6. Verification

### 6.1 Check Cluster Status

```bash
# Check all nodes
kubectl get nodes -o wide

# Check all pods
kubectl get pods --all-namespaces

# Check Fortuna pods
kubectl get pods -n fortuna
```

### 6.2 Check Core Logs

```bash
# Get Core pod
CORE_POD=$(kubectl get pods -n fortuna -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}')

# View logs
kubectl logs -n fortuna $CORE_POD --tail=50

# Expected: No errors, gRPC server started, workers initialized
```

### 6.3 Check Agent Logs

```bash
# Get Agent pods
kubectl get pods -n fortuna -l app.kubernetes.io/component=agent

# View logs for each agent
kubectl logs -n fortuna <agent-pod-name> --tail=50

# Expected: Connected to Core, registered successfully
```

### 6.4 Test API

```bash
# Port forward to Core service
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &

# Test health endpoint
curl http://localhost:8080/health

# Expected: {"status":"ok"} or similar

# Stop port forward
kill %1
```

### 6.5 Test Database Connection

```bash
# Get PostgreSQL pod
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# Connect and check
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c "\dt"

# Should show tables if migrations ran
```

### 6.6 Test SBOM Flow

```bash
# Create test pod
kubectl run test-nginx --image=nginx:1.21 -n default

# Wait for pod
kubectl wait --for=condition=ready pod test-nginx -n default --timeout=60s

# Check Agent logs (should see SBOM processing)
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=20 | grep -i sbom

# Check Core logs (should see SBOM received)
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=20 | grep -i sbom
```

---

## 7. Troubleshooting

### 7.1 Pods Not Starting

```bash
# Describe pod
kubectl describe pod <pod-name> -n fortuna

# Check events
kubectl get events -n fortuna --sort-by='.lastTimestamp'

# Check logs
kubectl logs <pod-name> -n fortuna
```

### 7.2 Image Pull Errors

```bash
# Check image pull policy
kubectl get deployment fortuna-core -n fortuna -o yaml | grep imagePullPolicy

# For local images, ensure imagePullPolicy: IfNotPresent or Never
# For registry images, ensure imagePullPolicy: Always or IfNotPresent

# Verify image exists on node
# SSH to node and run:
docker images | grep fortuna
# Or for containerd:
crictl images | grep fortuna
```

### 7.3 Agent Cannot Access containerd Socket

```bash
# Check if containerd socket exists on worker nodes
# SSH to worker node:
ls -la /run/containerd/containerd.sock

# Check Agent pod volume mounts
kubectl describe pod <agent-pod> -n fortuna | grep -A 5 "Mounts:"

# Ensure containerd socket is mounted correctly
```

### 7.4 Database Connection Failed

```bash
# Check PostgreSQL pod
kubectl get pods -n fortuna -l app=postgres

# Check PostgreSQL logs
kubectl logs -n fortuna -l app=postgres

# Test connection from Core pod
CORE_POD=$(kubectl get pods -n fortuna -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n fortuna $CORE_POD -- nc -zv postgres.fortuna.svc.cluster.local 5432
```

### 7.5 NATS Connection Failed

```bash
# Check NATS pods
kubectl get pods -n fortuna -l app=nats

# Check NATS logs
kubectl logs -n fortuna -l app=nats

# Test connection
NATS_POD=$(kubectl get pods -n fortuna -l app=nats -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n fortuna $NATS_POD -- nc -zv nats.fortuna.svc.cluster.local 4222
```

---

## 📊 Deployment Checklist

### Pre-Deployment
- [ ] All nodes prepared (swap disabled, kernel modules loaded)
- [ ] containerd installed and configured on all nodes
- [ ] Kubernetes installed on all nodes
- [ ] Master node initialized
- [ ] CNI plugin installed
- [ ] Worker nodes joined

### Build
- [ ] Core image built
- [ ] Agent image built
- [ ] Images pushed to registry or loaded to nodes

### Infrastructure
- [ ] Namespace created
- [ ] PostgreSQL deployed and running
- [ ] NATS deployed and running
- [ ] Database initialized

### Fortuna
- [ ] Certificates generated
- [ ] Secrets created
- [ ] RBAC deployed
- [ ] Core deployed and running
- [ ] Agent deployed on all worker nodes

### Verification
- [ ] All pods Running
- [ ] Core logs show no errors
- [ ] Agent logs show connection to Core
- [ ] API accessible
- [ ] Database connected
- [ ] NATS connected
- [ ] SBOM flow working

---

## 🎯 Next Steps

1. **Load CVE Database**: Import CVE data from OSV.dev
2. **Configure Policies**: Create and enable policy templates
3. **Access Dashboard**: Port forward dashboard service
4. **Monitor**: Set up monitoring and alerting
5. **Scale**: Add more worker nodes as needed

---

**Deployment Complete!** 🎉

---

*Fortuna K8s Management Platform - Multi-Node Deployment Guide v1.0*


