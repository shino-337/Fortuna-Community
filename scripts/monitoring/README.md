# Monitoring Scripts

**6 scripts** for monitoring Fortuna components and pipelines.

---

## 📝 Scripts

| Script | Purpose |
|--------|---------|
| **monitor_fortuna.sh** | Monitor entire Fortuna system |
| **monitor_admission_metrics.sh** | Monitor admission webhook metrics |
| **monitor-pipeline.sh** | Monitor SBOM/CVE pipeline |
| **monitor-sbom-performance.sh** | Monitor SBOM performance |
| **list_insights.sh** | List all insights |
| **query_insights.sh** | Query insights with filters |

---

## 🚀 Quick Start

### Monitor Everything
```bash
./monitor_fortuna.sh
```

### Monitor Pipeline
```bash
./monitor-pipeline.sh
```

### List Insights
```bash
./list_insights.sh
```

---

## 📚 Detailed Usage

### monitor_fortuna.sh
Monitor entire Fortuna system:
```bash
# Monitor all components
./monitor_fortuna.sh

# Monitor with interval
INTERVAL=5 ./monitor_fortuna.sh
```

Shows:
- Pod status
- Resource usage
- Event counts
- Error rates

---

### monitor-pipeline.sh
Monitor SBOM/CVE pipeline:
```bash
# Monitor all pods
./monitor-pipeline.sh

# Monitor specific pod
./monitor-pipeline.sh fortuna pod-name
```

Tracks:
- Pod events
- SBOM generation
- CVE matching
- Insight creation

---

### monitor-sbom-performance.sh
Monitor SBOM generation performance:
```bash
./monitor-sbom-performance.sh
```

Metrics:
- Generation time
- Success rate
- Error rate
- Throughput

---

### monitor_admission_metrics.sh
Monitor admission webhook metrics:
```bash
./monitor_admission_metrics.sh
```

Shows:
- Admission requests
- Policy evaluations
- Denials/approvals
- Latency

---

### list_insights.sh
List all insights:
```bash
# List all
./list_insights.sh

# List recent
./list_insights.sh --recent 10

# List by severity
./list_insights.sh --severity critical
```

---

### query_insights.sh
Query insights with filters:
```bash
# Query by severity
./query_insights.sh --severity high

# Query by namespace
./query_insights.sh --namespace production

# Query by type
./query_insights.sh --type vulnerability
```

---

## 📊 Monitoring Workflows

### Real-time Monitoring
```bash
# Terminal 1: Monitor system
./monitor_fortuna.sh

# Terminal 2: Monitor pipeline
./monitor-pipeline.sh

# Terminal 3: Watch insights
watch -n 5 ./list_insights.sh
```

### Performance Check
```bash
# Check SBOM performance
./monitor-sbom-performance.sh

# Check admission webhook
./monitor_admission_metrics.sh
```

### Debug Issues
```bash
# Monitor pipeline for specific pod
./monitor-pipeline.sh fortuna problem-pod

# Check logs
kubectl logs -n fortuna -l app=fortuna-core --tail=100

# List recent insights
./list_insights.sh --recent 20
```

---

## 🔍 What to Monitor

### System Health
- Pod status (Running/Pending/Failed)
- Resource usage (CPU/Memory)
- Event counts
- Error rates

### Pipeline Performance
- SBOM generation time
- CVE matching time
- Insight creation rate
- Queue depths

### Insights Quality
- Insight count by severity
- False positive rate
- Coverage (% pods with insights)
- Time to insight (pod created → insight created)

---

## 🚨 Troubleshooting

### No Insights Generated
```bash
# Monitor pipeline
./monitor-pipeline.sh

# Check if SBOM generated
kubectl exec -n fortuna postgres-xxx -- \
  psql -U postgres -d fortuna -c "SELECT COUNT(*) FROM sboms;"

# Check CVE database
kubectl exec -n fortuna postgres-xxx -- \
  psql -U postgres -d fortuna -c "SELECT COUNT(*) FROM cves;"
```

### Slow SBOM Generation
```bash
# Monitor SBOM performance
./monitor-sbom-performance.sh

# Check worker logs
kubectl logs -n fortuna -l app=fortuna-core | grep SBOMWorker

# Check resource limits
kubectl describe pod -n fortuna -l app=fortuna-core
```

### Admission Webhook Issues
```bash
# Monitor webhook metrics
./monitor_admission_metrics.sh

# Check webhook logs
kubectl logs -n fortuna -l app=fortuna-admission-webhook

# Test webhook
../testing/test_admission_webhook.sh
```

---

## 📖 Related Documentation

- [Operations Guide](../../docs/05-operations/README.md)
- [Performance Benchmarks](../../docs/05-operations/performance/BENCHMARKS.md)
- [Troubleshooting](../../docs/01-getting-started/TROUBLESHOOTING.md)

---

*Back to [Scripts README](../README.md)*

