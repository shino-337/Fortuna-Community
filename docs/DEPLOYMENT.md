# Deployment Guide

## Prerequisites

- Kubernetes cluster (1.21+)
- kubectl configured
- Helm 3.x
- Docker (for building images)

## Quick Start

### 1. Build Images

```bash
# Build Agent
cd agent
docker build -t ksam/agent:latest .

# Build Core
cd ../core
docker build -t ksam/core:latest .

# Build Dashboard
cd ../dashboard
docker build -t ksam/dashboard:latest .
```

### 2. Deploy with Helm

```bash
helm install ksam ./helm/ksam
```

### 3. Deploy Agent to Clusters

```bash
kubectl apply -f agent/deploy/rbac.yaml
kubectl apply -f agent/deploy/daemonset.yaml
```

## Configuration

Xem `helm/ksam/values.yaml` để cấu hình:
- Database connection
- Resource limits
- Service endpoints
- Authentication

## Verification

```bash
# Check Core Controller
kubectl get pods -l app=ksam-core

# Check Agent
kubectl get pods -l app=ksam-agent -n kube-system

# Check Dashboard
kubectl get pods -l app=ksam-dashboard
```

## Troubleshooting

- Check logs: `kubectl logs -l app=ksam-core`
- Check Agent connectivity: `kubectl logs -l app=ksam-agent -n kube-system`
- Verify database connection

