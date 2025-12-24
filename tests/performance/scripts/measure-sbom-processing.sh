#!/bin/bash

# ============================================================================
# Measure SBOM Processing Performance
# ============================================================================
# Measures time taken for SBOM extraction and storage
# ============================================================================

set -euo pipefail

NAMESPACE="${TEST_NAMESPACE:-ksam-test}"
IMAGE="${TEST_IMAGE:-nginx:latest}"
ITERATIONS="${ITERATIONS:-5}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-ksam}"
DB_USER="${DB_USER:-ksam}"
RESULTS_DIR="${RESULTS_DIR:-$(dirname $0)/../results}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

mkdir -p "${RESULTS_DIR}"

db_query() {
    local query=$1
    PGPASSWORD="${DB_PASSWORD:-ksam}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -A -c "${query}" 2>/dev/null || echo ""
}

wait_for_sbom() {
    local pod_uid=$1
    local max_wait=${2:-300}
    local elapsed=0
    
    while [ $elapsed -lt $max_wait ]; do
        local sbom_count=$(db_query "SELECT COUNT(*) FROM sboms WHERE pod_uid = '${pod_uid}' AND deleted_at IS NULL;")
        
        if [ "${sbom_count}" -gt 0 ]; then
            return 0
        fi
        
        sleep 1
        elapsed=$((elapsed + 1))
    done
    
    return 1
}

echo "SBOM Processing Performance Test"
echo "=================================="
echo "Image: ${IMAGE}"
echo "Iterations: ${ITERATIONS}"
echo ""

TIMES=()

for i in $(seq 1 ${ITERATIONS}); do
    POD_NAME="perf-test-${i}-$(date +%s)"
    
    echo "Iteration ${i}/${ITERATIONS}: ${POD_NAME}"
    
    # Create pod
    kubectl run "${POD_NAME}" \
        --image="${IMAGE}" \
        --namespace="${NAMESPACE}" \
        --restart=Never \
        --labels="test=ksam-perf" \
        >/dev/null 2>&1
    
    # Wait for pod ready
    kubectl wait --for=condition=ready pod/"${POD_NAME}" -n "${NAMESPACE}" --timeout=120s >/dev/null 2>&1
    
    # Get pod UID
    POD_UID=$(kubectl get pod "${POD_NAME}" -n "${NAMESPACE}" -o jsonpath='{.metadata.uid}')
    
    # Measure SBOM extraction time
    START_TIME=$(date +%s.%N)
    wait_for_sbom "${POD_UID}" 300
    END_TIME=$(date +%s.%N)
    
    DURATION=$(echo "$END_TIME - $START_TIME" | bc)
    TIMES+=(${DURATION})
    
    echo "  SBOM extraction time: ${DURATION} seconds"
    
    # Get component count
    SBOM_ID=$(db_query "SELECT id FROM sboms WHERE pod_uid = '${POD_UID}' AND deleted_at IS NULL LIMIT 1;")
    COMPONENT_COUNT=$(db_query "SELECT COUNT(*) FROM sbom_components WHERE sbom_id = ${SBOM_ID} AND deleted_at IS NULL;")
    echo "  Components extracted: ${COMPONENT_COUNT}"
    
    # Cleanup
    kubectl delete pod "${POD_NAME}" -n "${NAMESPACE}" --ignore-not-found=true >/dev/null 2>&1
    
    sleep 2
done

# Calculate statistics
TOTAL=0
for time in "${TIMES[@]}"; do
    TOTAL=$(echo "$TOTAL + $time" | bc)
done

AVG=$(echo "scale=2; $TOTAL / ${ITERATIONS}" | bc)
MIN=$(printf '%s\n' "${TIMES[@]}" | sort -n | head -1)
MAX=$(printf '%s\n' "${TIMES[@]}" | sort -n | tail -1)

echo ""
echo "Performance Summary"
echo "==================="
echo "Average: ${AVG} seconds"
echo "Min: ${MIN} seconds"
echo "Max: ${MAX} seconds"
echo "Iterations: ${ITERATIONS}"

# Save results
cat > "${RESULTS_DIR}/sbom_performance_${TIMESTAMP}.txt" <<EOF
SBOM Processing Performance Test
================================
Date: $(date)
Image: ${IMAGE}
Iterations: ${ITERATIONS}

Results:
- Average: ${AVG} seconds
- Min: ${MIN} seconds
- Max: ${MAX} seconds

Individual Times:
$(printf '%s\n' "${TIMES[@]}")
EOF

echo ""
echo "Results saved to: ${RESULTS_DIR}/sbom_performance_${TIMESTAMP}.txt"

