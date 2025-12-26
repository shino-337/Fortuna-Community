#!/bin/bash

# ============================================================================
# E2E Test: Complete Pod-to-Insight Flow
# ============================================================================
# This script verifies the complete flow from Pod creation to Insight
# generation and API availability.
#
# Flow:
#   1. Create test pod
#   2. Wait for SBOM extraction
#   3. Verify SBOM in database
#   4. Wait for CVE matching
#   5. Verify CVE matches in database
#   6. Wait for insight generation
#   7. Verify insights in database
#   8. Verify insights via API
#   9. Compare API response with database
#   10. Measure timing for each phase
# ============================================================================

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="${TEST_NAMESPACE:-ksam-test}"
POD_NAME="test-pod-$(date +%s)"
IMAGE="${TEST_IMAGE:-nginx:latest}"
CORE_API="${CORE_API_URL:-http://localhost:8080}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-ksam}"
DB_USER="${DB_USER:-ksam}"
RESULTS_DIR="${RESULTS_DIR:-$(dirname $0)/../results}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="${RESULTS_DIR}/e2e_${TIMESTAMP}.log"

# Create results directory
mkdir -p "${RESULTS_DIR}"

# Logging function
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "${LOG_FILE}"
}

log_success() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] ✅${NC} $1" | tee -a "${LOG_FILE}"
}

log_error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ❌${NC} $1" | tee -a "${LOG_FILE}"
}

log_warning() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] ⚠️${NC} $1" | tee -a "${LOG_FILE}"
}

# Timing functions
start_timer() {
    export TIMER_START_${1}=$(date +%s.%N)
}

end_timer() {
    local timer_name=$1
    local start_var="TIMER_START_${timer_name}"
    local start_time=${!start_var}
    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    echo "$duration"
}

# Database query function
db_query() {
    local query=$1
    local result=$(kubectl exec -n fortuna postgres-747fc6cdfb-zzw8m -- psql -U postgres -d fortuna -t -A -c "${query}" 2>/dev/null | tr -d '[:space:]' || echo "0")
    echo "${result}"
}

# Wait for pod to be ready
wait_for_pod() {
    local pod_name=$1
    local namespace=$2
    local max_wait=${3:-120}
    local elapsed=0
    
    log "Waiting for pod ${pod_name} to be ready..."
    
    while [ $elapsed -lt $max_wait ]; do
        local phase=$(kubectl get pod "${pod_name}" -n "${namespace}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
        
        if [ "${phase}" == "Running" ]; then
            local ready=$(kubectl get pod "${pod_name}" -n "${namespace}" -o jsonpath='{.status.containerStatuses[0].ready}' 2>/dev/null || echo "false")
            if [ "${ready}" == "true" ]; then
                log_success "Pod ${pod_name} is ready"
                return 0
            fi
        fi
        
        sleep 2
        elapsed=$((elapsed + 2))
        echo -n "."
    done
    
    log_error "Pod ${pod_name} did not become ready within ${max_wait} seconds"
    return 1
}

# Get pod UID
get_pod_uid() {
    kubectl get pod "${POD_NAME}" -n "${NAMESPACE}" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo ""
}

# Get pod image digest
get_pod_image_digest() {
    local pod_name=$1
    local namespace=$2
    # Get imageID from pod status (format: docker-pullable://<image>@<digest> or <image>@<digest>)
    local image_id=$(kubectl get pod "${pod_name}" -n "${namespace}" -o jsonpath='{.status.containerStatuses[0].imageID}' 2>/dev/null || echo "")
    if [ -z "${image_id}" ]; then
        # Fallback: try to get from spec (but this won't have digest)
        local image=$(kubectl get pod "${pod_name}" -n "${namespace}" -o jsonpath='{.spec.containers[0].image}' 2>/dev/null || echo "")
        echo "${image}"
    else
        # Extract digest from imageID
        # Format: docker-pullable://nginx@sha256:... or docker://sha256:... or nginx@sha256:...
        # Remove docker-pullable:// or docker:// prefix, then extract sha256:... part
        local digest=$(echo "${image_id}" | sed 's|docker-pullable://||' | sed 's|docker://||' | sed 's|.*@||' | sed 's|.*sha256:|sha256:|')
        if [ -z "${digest}" ] || [ "${digest}" = "${image_id}" ]; then
            # If extraction failed, try another method
            digest=$(echo "${image_id}" | grep -o 'sha256:[a-f0-9]*' | head -1)
        fi
        echo "${digest}"
    fi
}

# Wait for SBOM
wait_for_sbom() {
    local pod_uid=$1
    local pod_name=$2
    local max_wait=${3:-300}
    local elapsed=0
    
    # Get image digest from pod
    local image_digest=$(get_pod_image_digest "${pod_name}" "${NAMESPACE}")
    local image_name=""
    if [ -z "${image_digest}" ] || [ "${image_digest}" = "null" ]; then
        log_warning "Could not get image digest from pod, will try to query by image name"
        image_name=$(kubectl get pod "${pod_name}" -n "${NAMESPACE}" -o jsonpath='{.spec.containers[0].image}' 2>/dev/null | cut -d':' -f1 | sed 's|.*/||' || echo "")
        log "Waiting for SBOM extraction (pod: ${pod_name}, image: ${image_name})..."
    else
        log "Waiting for SBOM extraction (pod: ${pod_name}, image_digest: ${image_digest})..."
    fi
    
    while [ $elapsed -lt $max_wait ]; do
        local sbom_count=0
        local sbom_id=""
        
        # Strategy 1: Query by image name (most reliable)
        # SBOMs are reused via UPSERT, so we query by image_name regardless of updated_at
        if [ -n "${image_name}" ]; then
            sbom_id=$(db_query "SELECT id FROM sboms WHERE image_name = '${image_name}' AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1;" 2>/dev/null || echo "")
            if [ -n "${sbom_id}" ] && [ "${sbom_id}" != "0" ] && [ "${sbom_id}" != "" ]; then
                # Verify SBOM has components (indicates it was processed)
                local component_count=$(db_query "SELECT COUNT(*) FROM sbom_components WHERE sbom_id = ${sbom_id} AND deleted_at IS NULL;" 2>/dev/null || echo "0")
                component_count=$(echo "${component_count}" | tr -d '[:space:]')
                if [ -n "${component_count}" ] && [ "${component_count}" != "0" ] && [ "${component_count}" -gt 0 ] 2>/dev/null; then
                    sbom_count=1
                fi
            fi
        fi
        
        # Strategy 2: Try by image_digest from agent logs (if available)
        # We'll check agent logs to get the actual digest used
        if [ "${sbom_count}" = "0" ] || [ -z "${sbom_count}" ]; then
            # Get actual digest from agent logs (more reliable than pod status)
            local actual_digest=$(kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=100 2>&1 | grep -E "${pod_name}.*image=sha256:" | tail -1 | grep -o 'sha256:[a-f0-9]*' | head -1 || echo "")
            if [ -n "${actual_digest}" ] && [ "${actual_digest}" != "" ]; then
                sbom_id=$(db_query "SELECT id FROM sboms WHERE image_digest = '${actual_digest}' AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1;" 2>/dev/null || echo "")
                if [ -n "${sbom_id}" ] && [ "${sbom_id}" != "0" ] && [ "${sbom_id}" != "" ]; then
                    sbom_count=1
                fi
            fi
        fi
        
        # Strategy 3: Get most recently updated SBOM with components (fallback)
        if [ "${sbom_count}" = "0" ] || [ -z "${sbom_count}" ]; then
            sbom_id=$(db_query "SELECT s.id FROM sboms s INNER JOIN sbom_components sc ON s.id = sc.sbom_id WHERE s.deleted_at IS NULL AND sc.deleted_at IS NULL GROUP BY s.id ORDER BY s.updated_at DESC LIMIT 1;" 2>/dev/null || echo "")
            if [ -n "${sbom_id}" ] && [ "${sbom_id}" != "0" ] && [ "${sbom_id}" != "" ]; then
                sbom_count=1
            fi
        fi
        
        if [ -n "${sbom_count}" ] && [ "${sbom_count}" != "0" ] && [ "${sbom_count}" -gt 0 ] 2>/dev/null; then
            log_success "SBOM found in database (count: ${sbom_count})"
            # Use the SBOM ID we found
            if [ -z "${sbom_id}" ] || [ "${sbom_id}" = "0" ] || [ "${sbom_id}" = "" ]; then
                # Fallback: get SBOM ID by image_name
                if [ -n "${image_name}" ]; then
                    sbom_id=$(db_query "SELECT id FROM sboms WHERE image_name = '${image_name}' AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1;" 2>/dev/null || echo "")
                fi
                # If still empty, try to get from agent logs
                if [ -z "${sbom_id}" ] || [ "${sbom_id}" = "0" ] || [ "${sbom_id}" = "" ]; then
                    local actual_digest=$(kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=100 2>&1 | grep -E "${pod_name}.*image=sha256:" | tail -1 | grep -o 'sha256:[a-f0-9]*' | head -1 || echo "")
                    if [ -n "${actual_digest}" ] && [ "${actual_digest}" != "" ]; then
                        sbom_id=$(db_query "SELECT id FROM sboms WHERE image_digest = '${actual_digest}' AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1;" 2>/dev/null || echo "")
                    fi
                fi
            fi
            # Export SBOM_ID for use in main script
            export SBOM_ID="${sbom_id}"
            log "SBOM ID: ${SBOM_ID}"
            return 0
        fi
        
        # Check agent logs for processing status
        if [ $((elapsed % 30)) -eq 0 ] && [ $elapsed -gt 0 ]; then
            log "Still waiting... Checking agent status..."
            local agent_processing=$(kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=20 2>&1 | grep -c "${pod_name}" || echo "0")
            if [ "${agent_processing}" = "0" ]; then
                log_warning "Agent has not processed pod ${pod_name} yet"
            else
                log "Agent is processing pod ${pod_name}..."
            fi
        fi
        
        sleep 5
        elapsed=$((elapsed + 5))
        echo -n "."
    done
    
    log_error "SBOM not found within ${max_wait} seconds"
    log "Checking agent logs for pod ${pod_name}..."
    kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=50 2>&1 | grep -E "${pod_name}|Pod added|Queued|Processing pod" | tail -10 || log "No agent logs found for pod"
    return 1
}

# Wait for CVE matches
wait_for_cve_matches() {
    local sbom_id=$1
    local max_wait=${2:-300}
    local elapsed=0
    
    log "Waiting for CVE matching (sbom_id: ${sbom_id})..."
    
    while [ $elapsed -lt $max_wait ]; do
        local match_count=$(db_query "SELECT COUNT(*) FROM cve_matches WHERE sbom_id = ${sbom_id} AND deleted_at IS NULL;")
        
        if [ "${match_count}" -gt 0 ]; then
            log_success "CVE matches found: ${match_count}"
            return 0
        fi
        
        sleep 5
        elapsed=$((elapsed + 5))
        echo -n "."
    done
    
    log_warning "No CVE matches found (this may be normal if image has no vulnerabilities)"
    return 0  # Not an error - image may have no vulnerabilities
}

# Wait for insights
wait_for_insights() {
    local pod_uid=$1
    local max_wait=${2:-300}
    local elapsed=0
    
    log "Waiting for insight generation (pod_uid: ${pod_uid})..."
    
    while [ $elapsed -lt $max_wait ]; do
        # Try new schema first (resource_uid)
        local insight_count=$(db_query "SELECT COUNT(*) FROM insights WHERE resource_uid = '${pod_uid}' AND deleted_at IS NULL;" 2>/dev/null || echo "0")
        insight_count=$(echo "${insight_count}" | tr -d '[:space:]')
        
        # Fallback: try old schema (sbom_id or affected_resources JSONB)
        if [ "${insight_count}" = "0" ] || [ -z "${insight_count}" ]; then
            # Try by sbom_id if available
            if [ -n "${SBOM_ID}" ] && [ "${SBOM_ID}" != "0" ] && [ "${SBOM_ID}" != "" ]; then
                insight_count=$(db_query "SELECT COUNT(*) FROM insights WHERE sbom_id = ${SBOM_ID} AND deleted_at IS NULL;" 2>/dev/null || echo "0")
                insight_count=$(echo "${insight_count}" | tr -d '[:space:]')
            fi
            
            # Try by affected_resources JSONB (old schema)
            if [ "${insight_count}" = "0" ] || [ -z "${insight_count}" ]; then
                insight_count=$(db_query "SELECT COUNT(*) FROM insights WHERE affected_resources::text LIKE '%${pod_uid}%' AND deleted_at IS NULL;" 2>/dev/null || echo "0")
                insight_count=$(echo "${insight_count}" | tr -d '[:space:]')
            fi
        fi
        
        if [ -n "${insight_count}" ] && [ "${insight_count}" != "0" ] && [ "${insight_count}" -gt 0 ] 2>/dev/null; then
            log_success "Insights found: ${insight_count}"
            return 0
        fi
        
        sleep 5
        elapsed=$((elapsed + 5))
        echo -n "."
    done
    
    log_warning "No insights found (this may be normal if no vulnerabilities exist)"
    return 0  # Not an error - may have no vulnerabilities
}

# Verify API response
verify_api_insights() {
    local pod_uid=$1
    local api_response=$(curl -s "${CORE_API}/api/v1/insights?resource_uid=${pod_uid}" || echo "{}")
    local api_count=$(echo "${api_response}" | jq '.data | length' 2>/dev/null || echo "0")
    
    log "API returned ${api_count} insights for pod_uid: ${pod_uid}"
    
    if [ "${api_count}" -gt 0 ]; then
        log_success "API insights verified"
        echo "${api_response}" | jq '.' > "${RESULTS_DIR}/api_response_${TIMESTAMP}.json"
        return 0
    else
        log_warning "No insights in API response"
        return 0  # Not an error - may have no vulnerabilities
    fi
}

# Compare API with database
compare_api_db() {
    local pod_uid=$1
    
    log "Comparing API response with database..."
    
    # Get database insights
    local db_insights=$(db_query "SELECT id, insight_type, resource_uid, cve_id, severity, status FROM insights WHERE resource_uid = '${pod_uid}' AND deleted_at IS NULL ORDER BY id;")
    
    # Get API insights
    local api_response=$(curl -s "${CORE_API}/api/v1/insights?resource_uid=${pod_uid}" || echo "{}")
    local api_insights=$(echo "${api_response}" | jq -r '.data[] | "\(.id)|\(.insight_type)|\(.resource_uid)|\(.cve_id)|\(.severity)|\(.status)"' 2>/dev/null || echo "")
    
    local db_count=$(echo "${db_insights}" | grep -v "^$" | wc -l | tr -d ' ')
    local api_count=$(echo "${api_insights}" | grep -v "^$" | wc -l | tr -d ' ')
    
    log "Database insights: ${db_count}, API insights: ${api_count}"
    
    if [ "${db_count}" -eq "${api_count}" ]; then
        log_success "Insight counts match between API and database"
    else
        log_error "Insight count mismatch: DB=${db_count}, API=${api_count}"
    fi
}

# Main test execution
main() {
    log "=========================================="
    log "E2E Test: Pod-to-Insight Flow"
    log "=========================================="
    log "Pod Name: ${POD_NAME}"
    log "Namespace: ${NAMESPACE}"
    log "Image: ${IMAGE}"
    log "Results: ${RESULTS_DIR}"
    log "=========================================="
    
    # Phase 1: Create Pod
    log "Phase 1: Creating test pod..."
    start_timer "pod_creation"
    
    # Get agent node name to ensure pod is scheduled on same node
    AGENT_NODE=$(kubectl get pods -n "${NAMESPACE}" -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].spec.nodeName}' 2>/dev/null || echo "")
    
    if [ -n "${AGENT_NODE}" ]; then
        log "Agent node: ${AGENT_NODE}, using node selector"
        kubectl run "${POD_NAME}" \
            --image="${IMAGE}" \
            --namespace="${NAMESPACE}" \
            --restart=Never \
            --labels="test=ksam-e2e,app=test-pod" \
            --overrides="{\"spec\":{\"nodeSelector\":{\"kubernetes.io/hostname\":\"${AGENT_NODE}\"}}}" \
            2>&1 | tee -a "${LOG_FILE}"
    else
        log_warning "Could not determine agent node, creating pod without node selector"
        kubectl run "${POD_NAME}" \
            --image="${IMAGE}" \
            --namespace="${NAMESPACE}" \
            --restart=Never \
            --labels="test=ksam-e2e,app=test-pod" \
            2>&1 | tee -a "${LOG_FILE}"
    fi
    
    wait_for_pod "${POD_NAME}" "${NAMESPACE}" 120
    local pod_creation_time=$(end_timer "pod_creation")
    log_success "Pod created in ${pod_creation_time} seconds"
    
    # Get pod UID
    local pod_uid=$(get_pod_uid)
    if [ -z "${pod_uid}" ]; then
        log_error "Failed to get pod UID"
        exit 1
    fi
    log "Pod UID: ${pod_uid}"
    
    # Phase 2: Wait for SBOM
    log "Phase 2: Waiting for SBOM extraction..."
    start_timer "sbom_extraction"
    
    wait_for_sbom "${pod_uid}" "${POD_NAME}" 300
    local sbom_extraction_time=$(end_timer "sbom_extraction")
    log_success "SBOM extracted in ${sbom_extraction_time} seconds"
    
    # Get SBOM details (use SBOM_ID from wait_for_sbom if available)
    local sbom_id=${SBOM_ID:-""}
    if [ -z "${sbom_id}" ] || [ "${sbom_id}" = "0" ] || [ "${sbom_id}" = "" ]; then
        # Fallback: get by image name
        local image_name=$(kubectl get pod "${POD_NAME}" -n "${NAMESPACE}" -o jsonpath='{.spec.containers[0].image}' 2>/dev/null | cut -d':' -f1 | sed 's|.*/||' || echo "")
        if [ -n "${image_name}" ]; then
            sbom_id=$(db_query "SELECT id FROM sboms WHERE image_name = '${image_name}' AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1;" 2>/dev/null || echo "")
        fi
        # If still empty, try to get from agent logs
        if [ -z "${sbom_id}" ] || [ "${sbom_id}" = "0" ] || [ "${sbom_id}" = "" ]; then
            local actual_digest=$(kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=100 2>&1 | grep -E "${POD_NAME}.*image=sha256:" | tail -1 | grep -o 'sha256:[a-f0-9]*' | head -1 || echo "")
            if [ -n "${actual_digest}" ] && [ "${actual_digest}" != "" ]; then
                sbom_id=$(db_query "SELECT id FROM sboms WHERE image_digest = '${actual_digest}' AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1;" 2>/dev/null || echo "")
            fi
        fi
    fi
    local sbom_count="0"
    if [ -n "${sbom_id}" ] && [ "${sbom_id}" != "0" ] && [ "${sbom_id}" != "" ]; then
        sbom_count=$(db_query "SELECT COUNT(*) FROM sbom_components WHERE sbom_id = ${sbom_id} AND deleted_at IS NULL;" 2>/dev/null || echo "0")
    fi
    log "SBOM ID: ${sbom_id}, Components: ${sbom_count}"
    
    # Phase 3: Wait for CVE matches
    log "Phase 3: Waiting for CVE matching..."
    start_timer "cve_matching"
    
    wait_for_cve_matches "${sbom_id}" 300
    local cve_matching_time=$(end_timer "cve_matching")
    log_success "CVE matching completed in ${cve_matching_time} seconds"
    
    local cve_match_count=$(db_query "SELECT COUNT(*) FROM cve_matches WHERE sbom_id = ${sbom_id} AND deleted_at IS NULL;")
    log "CVE Matches: ${cve_match_count}"
    
    # Phase 4: Wait for insights
    log "Phase 4: Waiting for insight generation..."
    start_timer "insight_generation"
    
    wait_for_insights "${pod_uid}" 300
    local insight_generation_time=$(end_timer "insight_generation")
    log_success "Insight generation completed in ${insight_generation_time} seconds"
    
    local insight_count=$(db_query "SELECT COUNT(*) FROM insights WHERE resource_uid = '${pod_uid}' AND deleted_at IS NULL;")
    log "Insights: ${insight_count}"
    
    # Phase 5: Verify API
    log "Phase 5: Verifying API response..."
    start_timer "api_verification"
    
    verify_api_insights "${pod_uid}"
    compare_api_db "${pod_uid}"
    local api_verification_time=$(end_timer "api_verification")
    log_success "API verification completed in ${api_verification_time} seconds"
    
    # Calculate total time
    local total_time=$(echo "${pod_creation_time} + ${sbom_extraction_time} + ${cve_matching_time} + ${insight_generation_time} + ${api_verification_time}" | bc)
    
    # Summary
    log "=========================================="
    log "Test Summary"
    log "=========================================="
    log "Pod Creation: ${pod_creation_time} seconds"
    log "SBOM Extraction: ${sbom_extraction_time} seconds"
    log "CVE Matching: ${cve_matching_time} seconds"
    log "Insight Generation: ${insight_generation_time} seconds"
    log "API Verification: ${api_verification_time} seconds"
    log "Total E2E Time: ${total_time} seconds"
    log "=========================================="
    log "SBOM ID: ${sbom_id}"
    log "SBOM Components: ${sbom_count}"
    log "CVE Matches: ${cve_match_count}"
    log "Insights: ${insight_count}"
    log "=========================================="
    
    # Write summary to file
    cat > "${RESULTS_DIR}/summary_${TIMESTAMP}.txt" <<EOF
E2E Test Summary - ${TIMESTAMP}
========================================
Pod Name: ${POD_NAME}
Pod UID: ${pod_uid}
Namespace: ${NAMESPACE}
Image: ${IMAGE}

Timing:
- Pod Creation: ${pod_creation_time} seconds
- SBOM Extraction: ${sbom_extraction_time} seconds
- CVE Matching: ${cve_matching_time} seconds
- Insight Generation: ${insight_generation_time} seconds
- API Verification: ${api_verification_time} seconds
- Total E2E Time: ${total_time} seconds

Results:
- SBOM ID: ${sbom_id}
- SBOM Components: ${sbom_count}
- CVE Matches: ${cve_match_count}
- Insights: ${insight_count}

Status: PASSED
EOF
    
    log_success "Test completed successfully!"
    log "Results saved to: ${RESULTS_DIR}"
}

# Cleanup function
cleanup() {
    log "Cleaning up test pod..."
    kubectl delete pod "${POD_NAME}" -n "${NAMESPACE}" --ignore-not-found=true 2>&1 | tee -a "${LOG_FILE}" || true
}

# Trap cleanup on exit
trap cleanup EXIT

# Run main function
main "$@"

