# Dashboard Access Guide

**Date:** 2025-12-03

---

## Port-Forward Commands

### Quick Start

```bash
# Start port-forward (default port 3000)
kubectl port-forward -n ksam svc/ksam-dashboard 3000:80

# Or use the script
bash KSAM/scripts/start_dashboard_portforward.sh 3000
```

### Full Command

```bash
kubectl port-forward \
  --namespace ksam \
  service/ksam-dashboard \
  3000:80
```

### Custom Port

```bash
# Use port 8080 instead
kubectl port-forward -n ksam svc/ksam-dashboard 8080:80

# Or
bash KSAM/scripts/start_dashboard_portforward.sh 8080
```

### Background Mode

```bash
# Run in background
kubectl port-forward -n ksam svc/ksam-dashboard 3000:80 &

# Or use nohup
nohup kubectl port-forward -n ksam svc/ksam-dashboard 3000:80 > /tmp/dashboard-pf.log 2>&1 &
```

---

## Access Dashboard

### Browser Access

Once port-forward is running:

```
http://localhost:3000
```

### Test Connection

```bash
# Test if dashboard is accessible
curl http://localhost:3000

# Check HTTP status
curl -I http://localhost:3000
```

---

## Stop Port-Forward

### Find and Kill Process

```bash
# Find port-forward process
ps aux | grep "kubectl port-forward.*ksam-dashboard"

# Kill by PID
kill <PID>

# Or kill all port-forwards for dashboard
pkill -f "kubectl port-forward.*ksam-dashboard"
```

---

## Troubleshooting

### Port Already in Use

```bash
# Check what's using port 3000
lsof -i :3000

# Kill the process
kill <PID>

# Or use different port
kubectl port-forward -n ksam svc/ksam-dashboard 3001:80
```

### Service Not Found

```bash
# Check if service exists
kubectl get svc -n ksam ksam-dashboard

# Check namespace
kubectl get namespace ksam
```

### Pods Not Ready

```bash
# Check pod status
kubectl get pods -n ksam -l app=ksam-dashboard

# Check pod logs
kubectl logs -n ksam -l app=ksam-dashboard --tail=50
```

### Connection Refused

```bash
# Verify service is running
kubectl get svc -n ksam ksam-dashboard

# Check service endpoints
kubectl get endpoints -n ksam ksam-dashboard

# Test from inside cluster
kubectl run -it --rm test --image=curlimages/curl --restart=Never -- curl http://ksam-dashboard.ksam.svc.cluster.local
```

---

## Alternative Access Methods

### NodePort (if configured)

```bash
# Get NodePort
kubectl get svc -n ksam ksam-dashboard -o jsonpath='{.spec.ports[0].nodePort}'

# Access via minikube IP
minikube ip
# Then: http://<minikube-ip>:<nodePort>
```

### LoadBalancer (if available)

```bash
# Get LoadBalancer IP
kubectl get svc -n ksam ksam-dashboard -o jsonpath='{.status.loadBalancer.ingress[0].ip}'

# Access via LoadBalancer IP
http://<loadbalancer-ip>
```

### Ingress (if configured)

```bash
# Get Ingress
kubectl get ingress -n ksam

# Access via Ingress hostname
http://<ingress-hostname>
```

---

## Complete Example

```bash
# 1. Check service exists
kubectl get svc -n ksam ksam-dashboard

# 2. Check pods are running
kubectl get pods -n ksam -l app=ksam-dashboard

# 3. Start port-forward
kubectl port-forward -n ksam svc/ksam-dashboard 3000:80

# 4. In another terminal, test
curl http://localhost:3000

# 5. Open in browser
open http://localhost:3000
# or
xdg-open http://localhost:3000  # Linux
```

---

## Script Usage

### Using the Helper Script

```bash
# Start on default port 3000
bash KSAM/scripts/start_dashboard_portforward.sh

# Start on custom port
bash KSAM/scripts/start_dashboard_portforward.sh 8080

# Script will:
# - Clean up existing port-forwards
# - Check service exists
# - Wait for pods to be ready
# - Start port-forward
# - Test connection
```

---

## API Access

### Through Dashboard Proxy

Dashboard proxies API requests to Core:

```
http://localhost:3000/api/v1/insights/summary
http://localhost:3000/api/v1/clusters
http://localhost:3000/api/v1/insights
```

### Direct API Access

```bash
# Port-forward Core service
kubectl port-forward -n ksam svc/ksam-core 8080:8080

# Access API
curl http://localhost:8080/api/v1/insights/summary
```

---

## Summary

**Quick Command:**
```bash
kubectl port-forward -n ksam svc/ksam-dashboard 3000:80
```

**Access:**
```
http://localhost:3000
```

**Stop:**
```bash
pkill -f "kubectl port-forward.*ksam-dashboard"
```

---

**Last Updated:** 2025-12-03

