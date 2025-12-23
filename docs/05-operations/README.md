# Operations Documentation

Deployment, monitoring, and operational guides for Fortuna.

## 📚 Documents

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
