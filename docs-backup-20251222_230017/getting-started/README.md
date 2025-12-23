# Getting Started with KSAM

Welcome to KSAM! This section contains everything you need to get started with installation, configuration, and basic usage.

---

## Quick Links

- **[Prerequisites](#prerequisites)** - System requirements and dependencies
- **[Installation Guide](#installation)** - Step-by-step installation
- **[Quick Start](#quick-start)** - Get KSAM running in 15 minutes
- **[First Steps](#first-steps)** - What to do after installation

---

## Prerequisites

### Required

- **Kubernetes Cluster**: v1.24+ (Minikube, EKS, GKE, AKS, or any CNCF-certified distribution)
- **kubectl**: Configured to access your cluster
- **PostgreSQL**: 14+ with Apache AGE extension support
- **NATS**: For message queuing (optional but recommended)

### Recommended

- **Helm**: v3.0+ for easier installation
- **Docker**: For building custom images
- **4 CPU cores**: Minimum for production deployment
- **8 GB RAM**: Minimum for production deployment

### Optional

- **Prometheus + Grafana**: For monitoring
- **Redis**: For caching (improves performance)

---

## Installation

### Option 1: Helm Chart (Recommended)

```bash
# Add KSAM Helm repository
helm repo add ksam https://charts.ksam.io
helm repo update

# Install KSAM
helm install ksam ksam/ksam \
  --namespace ksam \
  --create-namespace \
  --set postgresql.enabled=true \
  --set nats.enabled=true

# Wait for all pods to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=ksam -n ksam --timeout=300s
```

### Option 2: Kubernetes Manifests

```bash
# Clone repository
git clone https://github.com/ksam-io/ksam.git
cd ksam

# Apply manifests
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl apply -f deploy/infrastructure/nats.yaml
kubectl apply -f deploy/core-deployment.yaml
kubectl apply -f deploy/agent-daemonset.yaml
kubectl apply -f deploy/dashboard-deployment.yaml

# Check status
kubectl get pods -n ksam
```

### Option 3: Minikube (Development)

```bash
# Start Minikube with sufficient resources
minikube start --cpus 4 --memory 8192

# Run setup script
./scripts/start_minikube.sh

# This script will:
# - Set up PostgreSQL with AGE extension
# - Deploy NATS
# - Deploy KSAM Core
# - Deploy KSAM Agent
# - Deploy Dashboard
# - Generate TLS certificates for mTLS
```

---

## Quick Start

### Step 1: Verify Installation

```bash
# Check all pods are running
kubectl get pods -n ksam

# Expected output:
# NAME                           READY   STATUS
# ksam-core-*                    1/1     Running
# ksam-agent-*                   1/1     Running (one per node)
# ksam-dashboard-*               1/1     Running
# ksam-postgresql-*              1/1     Running
# ksam-nats-*                    1/1     Running
```

### Step 2: Access Dashboard

```bash
# Port-forward dashboard
kubectl port-forward -n ksam svc/ksam-dashboard 8080:80

# Open in browser
open http://localhost:8080
```

**Default Credentials** (change immediately!):
- Username: `admin`
- Password: `changeme`

### Step 3: Verify Data Collection

```bash
# Check ServiceAccounts collected
kubectl exec -n ksam ksam-postgresql-* -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) FROM serviceaccounts WHERE deleted_at IS NULL;"

# Check Pods collected
kubectl exec -n ksam ksam-postgresql-* -- psql -U postgres -d ksam -c \
  "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL;"

# Check Insights generated
kubectl exec -n ksam ksam-postgresql-* -- psql -U postgres -d ksam -c \
  "SELECT severity, COUNT(*) FROM insights WHERE deleted_at IS NULL GROUP BY severity;"
```

### Step 4: Enable CVE Scanning (Optional)

```bash
# Load CVE database from OSV.dev
kubectl exec -n ksam ksam-core-* -- /app/cve-loader --source /cve-data/all

# Wait ~30 minutes for 74,561 CVEs to load
# Check progress
kubectl logs -n ksam ksam-core-* -f | grep "CVE"

# Trigger SBOM generation for existing pods
kubectl exec -n ksam ksam-core-* -- /app/trigger-sbom-scan
```

See [CVE Scanner Quick Start](../03-components/cve-scanner/quick-start-guide.md) for details.

---

## First Steps

### 1. Review Dashboard Overview

Navigate to the dashboard and review:
- **Total ServiceAccounts**: Should match `kubectl get sa --all-namespaces | wc -l`
- **Total Pods**: Should match `kubectl get pods --all-namespaces | wc -l`
- **Active Insights**: Security findings requiring attention

### 2. Check Risk Center

Go to **Risk Center** → **Insights**:
- Review critical and high severity insights
- Understand what each insight means
- Plan remediation for top risks

### 3. Explore ServiceAccounts

Go to **ServiceAccounts** view:
- Sort by risk score (highest first)
- Review cluster-admin bindings
- Check which ServiceAccounts have excessive permissions

### 4. Review Attack Paths

Go to **Attack Paths** visualization:
- See graphical representation of relationships
- Identify potential privilege escalation paths
- Understand lateral movement possibilities

### 5. Configure Policies

Go to **Rules Management**:
- Review built-in policies (20+ rules)
- Enable/disable policies as needed
- Create custom policies for your environment

---

## Common Next Steps

### Enable Policy Enforcement

```bash
# Enable admission webhook (blocks violating resources)
kubectl apply -f deploy/webhook-config.yaml

# Set enforcement mode
kubectl set env deployment/ksam-core -n ksam \
  POLICY_ENFORCEMENT_ENABLED=true \
  POLICY_ENFORCEMENT_MODE=deny  # or 'warn'
```

### Configure Monitoring

```bash
# Deploy Prometheus ServiceMonitor
kubectl apply -f deploy/prometheus/servicemonitor.yaml

# Import Grafana dashboards
kubectl apply -f deploy/grafana/
```

### Set Up Alerts

```bash
# Configure Prometheus alerts
kubectl apply -f deploy/prometheus/alerts.yaml
```

### Customize Risk Scoring

Edit ConfigMap:
```bash
kubectl edit configmap -n ksam ksam-core-config

# Adjust:
# RISK_NAMESPACE_MULTIPLIERS='{"kube-system": 1.5, "production": 1.4}'
```

---

## Verification Checklist

Use this checklist to verify your installation:

- [ ] All pods in `ksam` namespace are Running
- [ ] Dashboard accessible and login works
- [ ] ServiceAccounts data visible in dashboard
- [ ] Pods data visible in dashboard
- [ ] At least one insight generated
- [ ] Risk scores calculated for ServiceAccounts
- [ ] Graph visualization shows relationships
- [ ] Metrics available at `/metrics` endpoint
- [ ] Logs show no recurring errors

---

## Troubleshooting

### Pods Not Starting

```bash
# Check events
kubectl get events -n ksam --sort-by='.lastTimestamp'

# Check specific pod
kubectl describe pod -n ksam <pod-name>

# Check logs
kubectl logs -n ksam <pod-name>
```

### No Data Collected

**Check Agent**:
```bash
# Agent pods running?
kubectl get pods -n ksam -l app=ksam-agent

# Agent logs
kubectl logs -n ksam -l app=ksam-agent | grep -i "collected\\|sync"

# Agent connectivity to Core
kubectl logs -n ksam -l app=ksam-agent | grep -i "grpc\\|connected"
```

**Check RBAC**:
```bash
# Agent has permissions?
kubectl describe clusterrolebinding ksam-agent
```

### Dashboard Not Accessible

```bash
# Check service
kubectl get svc -n ksam ksam-dashboard

# Check pod logs
kubectl logs -n ksam -l app=ksam-dashboard

# Port-forward directly to pod
kubectl port-forward -n ksam <dashboard-pod> 8080:80
```

---

## Resource Requirements

### Minimum (Development)

```yaml
Core:
  CPU: 500m
  Memory: 512Mi

Agent (per node):
  CPU: 100m
  Memory: 128Mi

Dashboard:
  CPU: 100m
  Memory: 128Mi

PostgreSQL:
  CPU: 1000m
  Memory: 2Gi
```

### Recommended (Production)

```yaml
Core:
  CPU: 2000m
  Memory: 4Gi
  Replicas: 3

Agent (per node):
  CPU: 500m
  Memory: 512Mi

Dashboard:
  CPU: 500m
  Memory: 512Mi
  Replicas: 2

PostgreSQL:
  CPU: 4000m
  Memory: 8Gi
  Storage: 100Gi
```

---

## Next Steps

After completing this guide:

1. **Read [Architecture Overview](../02-architecture/)** to understand how KSAM works
2. **Explore [Components](../03-components/)** to learn about each component
3. **Review [Deployment Guide](../04-deployment/)** for production setup
4. **Check [Operations Guide](../05-operations/)** for monitoring and maintenance

---

## Getting Help

- **Documentation**: Start with [Main README](../README.md)
- **Common Issues**: See [Troubleshooting](../04-deployment/troubleshooting/)
- **GitHub Issues**: https://github.com/ksam-io/ksam/issues
- **Community**: Join our Slack/Discord (links in main README)

---

**Last Updated**: December 16, 2025
