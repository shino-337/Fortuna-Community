# Operations Documentation

Deployment, monitoring, and operational guides for Fortuna.

## 📚 Documents

### Troubleshooting
- **[DEPLOYMENT_ISSUES_ANALYSIS.md](./DEPLOYMENT_ISSUES_ANALYSIS.md)** - Comprehensive analysis of deployment issues and solutions
- **[DNS_TROUBLESHOOTING.md](./DNS_TROUBLESHOOTING.md)** - DNS resolution issues and fixes
- **[CORE_CONNECTION_TROUBLESHOOTING.md](./CORE_CONNECTION_TROUBLESHOOTING.md)** - Core gRPC connection issues
- **[AGENT_DEPLOYMENT_TROUBLESHOOTING.md](./AGENT_DEPLOYMENT_TROUBLESHOOTING.md)** - Agent deployment issues
- **[PVC_TROUBLESHOOTING.md](./PVC_TROUBLESHOOTING.md)** - PersistentVolumeClaim issues
- **[CONTAINERD_IMAGE_LOADING.md](./CONTAINERD_IMAGE_LOADING.md)** - Loading images into containerd
- **[MULTI_NODE_IMAGE_DEPLOYMENT.md](./MULTI_NODE_IMAGE_DEPLOYMENT.md)** - Deploying images across multiple nodes

### Performance
- **[performance/BENCHMARKS.md](./performance/BENCHMARKS.md)** - Performance benchmarks

### Deployment
- **[deployment/](./deployment/)** - Deployment guides (if exists)

### Monitoring
- **[monitoring/](./monitoring/)** - Monitoring & alerting setup

---

## 🚀 Quick Operations Guide

### Check System Health
```bash
kubectl get pods -n fortuna
kubectl logs -n fortuna -l app=fortuna-core --tail=50
```

### Monitor CVE Database
```bash
# Connect to PostgreSQL
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it $POSTGRES_POD -n fortuna -- psql -U postgres -d fortuna

# Check database status
SELECT COUNT(*) FROM cves;
SELECT COUNT(*) FROM insights WHERE status = 'active';
SELECT COUNT(*) FROM sboms;
```

### Performance Monitoring
- SBOM generation: ~2-5 seconds per image
- CVE matching: <1 second per SBOM
- API response: <50ms (p99)

---

**Next**: [Reference](../06-reference/)
