# Admission Metrics Setup Guide

**Date**: 2025-12-09  
**Status**: ✅ **COMPLETE**

---

## Overview

Hướng dẫn setup và sử dụng Admission Webhook metrics cho monitoring và alerting.

---

## 1. Test Admission Metrics Exposure

### Quick Test

```bash
# Run test script
./KSAM/scripts/test_admission_metrics.sh
```

### Manual Test

```bash
# Port-forward to core service
kubectl port-forward -n ksam deployment/ksam-core 8080:8080

# Check metrics endpoint
curl http://localhost:8080/metrics | grep admission

# Expected output:
# admission_latency_ms_bucket{operation="validate",le="10"} 0
# admission_latency_ms_bucket{operation="validate",le="25"} 0
# admission_latency_ms_bucket{operation="validate",le="50"} 0
# admission_latency_ms_bucket{operation="validate",le="100"} 0
# admission_denied_count{reason="policy_violation"} 0
# admission_allowed_count{resource_type="Pod"} 0
# cel_eval_ms_bucket{template_id="unknown",le="1"} 0
# event_publish_fail_count{event_type="violation_detected"} 0
# event_publish_success_count{event_type="violation_detected"} 0
# admission_errors_total{error_type="parse_error"} 0
```

### Verify Metrics

```bash
# Check if all metrics are present
curl -s http://localhost:8080/metrics | grep -E "^admission_|^cel_eval_|^event_publish_" | wc -l
# Should return: 7+ (depending on buckets)
```

---

## 2. Setup Grafana Dashboards

### Option 1: Import Dashboard JSON

1. **Access Grafana**:
   ```bash
   kubectl port-forward -n ksam deployment/ksam-dashboard 3000:3000
   # Open http://localhost:3000
   ```

2. **Import Dashboard**:
   - Go to Dashboards → Import
   - Upload `KSAM/deploy/grafana/dashboards/admission-webhook-dashboard.json`
   - Select Prometheus data source
   - Click Import

### Option 2: Manual Setup

1. **Create New Dashboard**:
   - Name: "Admission Webhook - Performance & Health"
   - Tags: `ksam`, `admission`, `webhook`

2. **Add Panels**:

   **Panel 1: Admission Latency (P95)**
   ```promql
   histogram_quantile(0.95, rate(admission_latency_ms_bucket[5m]))
   ```
   - Type: Graph
   - Y-axis: ms
   - Thresholds: Green <50ms, Yellow <100ms, Red >=100ms

   **Panel 2: Admission Requests Rate**
   ```promql
   rate(admission_allowed_count[5m])
   rate(admission_denied_count[5m])
   ```
   - Type: Graph
   - Y-axis: req/s

   **Panel 3: CEL Evaluation Time**
   ```promql
   histogram_quantile(0.95, rate(cel_eval_ms_bucket[5m]))
   ```
   - Type: Graph
   - Y-axis: ms

   **Panel 4: Event Publish Status**
   ```promql
   rate(event_publish_success_count[5m])
   rate(event_publish_fail_count[5m])
   ```
   - Type: Graph
   - Y-axis: events/s

   **Panel 5: Admission Errors**
   ```promql
   rate(admission_errors_total[5m])
   ```
   - Type: Graph
   - Y-axis: errors/s

   **Panel 6: Decision Breakdown**
   ```promql
   sum(rate(admission_allowed_count[5m]))
   sum(rate(admission_denied_count[5m]))
   ```
   - Type: Pie Chart

   **Panel 7: Latency Distribution**
   ```promql
   rate(admission_latency_ms_bucket[5m])
   ```
   - Type: Heatmap

### Dashboard Features

- **Real-time Updates**: 30s refresh interval
- **Time Range**: Default 1 hour, adjustable
- **Alerts**: Built-in alerts for high latency
- **Thresholds**: Visual indicators for performance

---

## 3. Configure Alert Rules

### Option 1: Prometheus Operator (Recommended)

```bash
# Apply alert rules
kubectl apply -f KSAM/deploy/prometheus/alerts/admission-webhook-alerts.yaml
```

### Option 2: Prometheus Config

Add to `prometheus.yml`:

```yaml
rule_files:
  - "admission-webhook-alerts.yml"
```

### Alert Rules Included

1. **AdmissionWebhookHighLatency**
   - Condition: P95 latency > 100ms for 5m
   - Severity: Warning

2. **AdmissionWebhookVeryHighLatency**
   - Condition: P95 latency > 200ms for 2m
   - Severity: Critical

3. **AdmissionWebhookHighErrorRate**
   - Condition: Error rate > 0.1/s for 5m
   - Severity: Warning

4. **AdmissionWebhookEventPublishFailures**
   - Condition: Publish failure rate > 0.05/s for 5m
   - Severity: Warning

5. **AdmissionWebhookHighDenialRate**
   - Condition: Denial rate > 10/s for 5m
   - Severity: Info

6. **CELEvaluationHighLatency**
   - Condition: P95 CEL eval time > 50ms for 5m
   - Severity: Warning

7. **AdmissionWebhookNoRequests**
   - Condition: No requests for 10m
   - Severity: Warning

### Test Alerts

```bash
# Check if alerts are loaded
kubectl get prometheusrules -n ksam

# View alert status
# Access Prometheus UI and check Alerts tab
```

---

## 4. Monitor Production Metrics

### Option 1: Continuous Monitoring Script

```bash
# Run monitoring script
./KSAM/scripts/monitor_admission_metrics.sh
```

### Option 2: Prometheus Queries

**Key Metrics to Monitor**:

1. **Latency**:
   ```promql
   # Average latency
   rate(admission_latency_ms_sum[5m]) / rate(admission_latency_ms_count[5m])
   
   # P95 latency
   histogram_quantile(0.95, rate(admission_latency_ms_bucket[5m]))
   
   # P99 latency
   histogram_quantile(0.99, rate(admission_latency_ms_bucket[5m]))
   ```

2. **Throughput**:
   ```promql
   # Total requests per second
   rate(admission_allowed_count[5m]) + rate(admission_denied_count[5m])
   
   # Denial rate
   rate(admission_denied_count[5m])
   ```

3. **Error Rate**:
   ```promql
   # Error rate
   rate(admission_errors_total[5m])
   
   # Errors by type
   sum by (error_type) (rate(admission_errors_total[5m]))
   ```

4. **Event Publish**:
   ```promql
   # Success rate
   rate(event_publish_success_count[5m]) / 
   (rate(event_publish_success_count[5m]) + rate(event_publish_fail_count[5m]))
   
   # Failure rate
   rate(event_publish_fail_count[5m])
   ```

5. **CEL Performance**:
   ```promql
   # Average CEL eval time
   rate(cel_eval_ms_sum[5m]) / rate(cel_eval_ms_count[5m])
   
   # P95 CEL eval time
   histogram_quantile(0.95, rate(cel_eval_ms_bucket[5m]))
   ```

### Option 3: Grafana Dashboard

Use the imported dashboard for visual monitoring:
- Real-time graphs
- Historical trends
- Alert notifications
- Performance thresholds

---

## 5. Troubleshooting

### Metrics Not Appearing

1. **Check if webhook is initialized**:
   ```bash
   kubectl logs -n ksam deployment/ksam-core | grep "Admission webhook"
   ```

2. **Check if metrics are registered**:
   ```bash
   curl http://localhost:8080/metrics | grep admission
   ```

3. **Check if webhook is being called**:
   ```bash
   kubectl logs -n ksam deployment/ksam-core | grep "Webhook"
   ```

### High Latency

1. **Check CEL evaluation time**:
   ```promql
   histogram_quantile(0.95, rate(cel_eval_ms_bucket[5m]))
   ```

2. **Check resource parsing time**:
   - Review webhook logs
   - Check for large resources

3. **Check event publish time**:
   - Verify NATS connectivity
   - Check worker queue depth

### Event Publish Failures

1. **Check NATS connectivity**:
   ```bash
   kubectl logs -n ksam deployment/ksam-core | grep NATS
   ```

2. **Check worker status**:
   ```bash
   kubectl logs -n ksam deployment/ksam-core | grep PolicyWorker
   ```

3. **Check queue depth**:
   ```promql
   ksam_worker_queue_depth{worker_type="policy"}
   ```

---

## 6. Best Practices

### Monitoring

1. **Set up alerts** for critical metrics
2. **Review dashboards** regularly
3. **Track trends** over time
4. **Document** any anomalies

### Performance

1. **Target latency**: <100ms (P95)
2. **Target CEL eval**: <50ms (P95)
3. **Monitor error rate**: <0.1/s
4. **Monitor event publish**: >99% success rate

### Alerting

1. **Use appropriate severity** levels
2. **Set reasonable thresholds**
3. **Test alerts** regularly
4. **Document** alert responses

---

## 7. Example Queries

### Performance Analysis

```promql
# Average latency by operation
avg(rate(admission_latency_ms_sum[5m]) / rate(admission_latency_ms_count[5m]))

# Denial rate by reason
sum by (reason) (rate(admission_denied_count[5m]))

# Error rate by type
sum by (error_type) (rate(admission_errors_total[5m]))
```

### Capacity Planning

```promql
# Requests per second
sum(rate(admission_allowed_count[5m]) + rate(admission_denied_count[5m]))

# Peak load
max_over_time(
  sum(rate(admission_allowed_count[5m]) + rate(admission_denied_count[5m]))[1h:]
)
```

---

## Status

✅ **SETUP COMPLETE**

**Files Created**:
- `KSAM/scripts/test_admission_metrics.sh` - Test script
- `KSAM/scripts/monitor_admission_metrics.sh` - Monitoring script
- `KSAM/deploy/prometheus/alerts/admission-webhook-alerts.yaml` - Alert rules
- `KSAM/deploy/grafana/dashboards/admission-webhook-dashboard.json` - Dashboard
- `KSAM/docs/ADMISSION_METRICS_SETUP_GUIDE.md` - This guide

**Next Steps**:
1. Run test script to verify metrics
2. Import Grafana dashboard
3. Apply alert rules
4. Start monitoring

---

**Last Updated**: 2025-12-09

