#!/bin/bash

# Monitor Admission Webhook Metrics
# This script continuously monitors admission webhook metrics and alerts on issues

set -e

CORE_SERVICE="${CORE_SERVICE:-ksam-core.ksam.svc.cluster.local}"
CORE_PORT="${CORE_PORT:-8080}"
METRICS_URL="http://${CORE_SERVICE}:${CORE_PORT}/metrics"
CHECK_INTERVAL="${CHECK_INTERVAL:-30}"

echo "=========================================="
echo "Admission Webhook Metrics Monitor"
echo "=========================================="
echo "Metrics URL: ${METRICS_URL}"
echo "Check Interval: ${CHECK_INTERVAL}s"
echo ""

# Function to get metric value
get_metric() {
    local metric_name=$1
    kubectl exec -n ksam deployment/ksam-core -- wget -q -O- "${METRICS_URL}" 2>/dev/null | \
        grep "^${metric_name}" | awk '{print $2}' | head -1 || echo "0"
}

# Function to check latency
check_latency() {
    echo "📊 Checking Admission Latency..."
    
    # Get P95 latency (approximate from buckets)
    local latency_p95=$(get_metric "admission_latency_ms_bucket{operation=\"validate\",le=\"100\"}")
    
    if [ -z "$latency_p95" ] || [ "$latency_p95" = "0" ]; then
        echo "   ⚠️  No latency data available (webhook may not have received requests)"
        return
    fi
    
    echo "   P95 Latency: ${latency_p95}ms"
    
    if (( $(echo "$latency_p95 > 100" | bc -l) )); then
        echo "   ⚠️  WARNING: Latency exceeds 100ms threshold!"
        return 1
    else
        echo "   ✅ Latency within acceptable range"
        return 0
    fi
}

# Function to check error rate
check_errors() {
    echo "📊 Checking Error Rate..."
    
    local error_count=$(get_metric "admission_errors_total")
    
    if [ -z "$error_count" ] || [ "$error_count" = "0" ]; then
        echo "   ✅ No errors detected"
        return 0
    fi
    
    echo "   Total Errors: ${error_count}"
    
    # Check error rate (would need time series data for accurate rate)
    echo "   ⚠️  Errors detected - check logs for details"
    return 1
}

# Function to check event publish failures
check_event_publish() {
    echo "📊 Checking Event Publish Status..."
    
    local fail_count=$(get_metric "event_publish_fail_count")
    local success_count=$(get_metric "event_publish_success_count")
    
    if [ -z "$fail_count" ]; then
        fail_count=0
    fi
    if [ -z "$success_count" ]; then
        success_count=0
    fi
    
    echo "   Success: ${success_count}"
    echo "   Failures: ${fail_count}"
    
    if [ "$fail_count" -gt 0 ]; then
        echo "   ⚠️  WARNING: Event publish failures detected!"
        return 1
    else
        echo "   ✅ All events published successfully"
        return 0
    fi
}

# Function to check admission decisions
check_decisions() {
    echo "📊 Checking Admission Decisions..."
    
    local allowed=$(get_metric "admission_allowed_count")
    local denied=$(get_metric "admission_denied_count")
    
    if [ -z "$allowed" ]; then
        allowed=0
    fi
    if [ -z "$denied" ]; then
        denied=0
    fi
    
    local total=$((allowed + denied))
    
    if [ "$total" -eq 0 ]; then
        echo "   ⚠️  No admission requests received"
        return 1
    fi
    
    local deny_rate=$(echo "scale=2; $denied * 100 / $total" | bc)
    
    echo "   Allowed: ${allowed}"
    echo "   Denied: ${denied}"
    echo "   Denial Rate: ${deny_rate}%"
    
    if (( $(echo "$deny_rate > 50" | bc -l) )); then
        echo "   ⚠️  WARNING: High denial rate (>50%)"
        return 1
    else
        echo "   ✅ Denial rate within normal range"
        return 0
    fi
}

# Main monitoring loop
while true; do
    echo ""
    echo "=========================================="
    echo "$(date): Metrics Check"
    echo "=========================================="
    
    ISSUES=0
    
    check_latency || ((ISSUES++))
    check_errors || ((ISSUES++))
    check_event_publish || ((ISSUES++))
    check_decisions || ((ISSUES++))
    
    echo ""
    if [ $ISSUES -eq 0 ]; then
        echo "✅ All checks passed"
    else
        echo "⚠️  ${ISSUES} issue(s) detected"
    fi
    
    echo ""
    echo "Sleeping for ${CHECK_INTERVAL}s..."
    sleep ${CHECK_INTERVAL}
done

