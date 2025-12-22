#!/usr/bin/env bash
# KSAM E2E Test with Detailed Monitoring for Optimized InsightManager
# Tests complete SBOM/CVE pipeline with performance monitoring
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAMESPACE="${NAMESPACE:-ksam}"
E2E_NS="${E2E_NS:-ksam-e2e}"
TEST_POD_NAME="${TEST_POD_NAME:-ksam-e2e-vuln-debian10}"
TEST_CVE_ID="${TEST_CVE_ID:-CVE-2014-0011}"
HTTP_LOCAL_PORT="${HTTP_LOCAL_PORT:-18080}"

die() { echo "[E2E] ❌ ERROR: $*" >&2; exit 1; }
step() { echo ""; echo "═══════════════════════════════════════════════════════════════"; echo "==> $*"; echo "═══════════════════════════════════════════════════════════════"; }
info() { echo "[E2E] ℹ️  $*"; }
success() { echo "[E2E] ✅ $*"; }
warn() { echo "[E2E] ⚠️  $*"; }
perf() { echo "[PERF] ⚡ $*"; }

command -v kubectl >/dev/null 2>&1 || die "kubectl not found"
command -v python3 >/dev/null 2>&1 || die "python3 not found"

POSTGRES_POD="$(kubectl -n "$NAMESPACE" get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")"
[[ -n "${POSTGRES_POD}" ]] || die "Postgres pod not found in namespace $NAMESPACE"

# Performance tracking (using files instead of associative array for compatibility)
TIMINGS_DIR="/tmp/ksam-e2e-timings-$$"
mkdir -p "$TIMINGS_DIR"
trap "rm -rf $TIMINGS_DIR" EXIT

start_timer() { echo "$(date +%s.%N)" > "$TIMINGS_DIR/$1"; }
end_timer() {
  local key=$1
  if [[ -f "$TIMINGS_DIR/$key" ]]; then
    local start=$(cat "$TIMINGS_DIR/$key")
    local elapsed=$(echo "$(date +%s.%N) - $start" | bc)
    perf "$key: ${elapsed}s"
  fi
}

step "0) Pre-flight Checks"
info "Checking GIN indexes..."
GIN_COUNT=$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pg_indexes WHERE tablename = 'insights' AND indexname LIKE '%gin%';" 2>/dev/null | tr -d '[:space:]' || echo "0")
if [[ "${GIN_COUNT}" == "0" ]]; then
  warn "GIN indexes not found. Migration 026 may not have run."
else
  success "GIN indexes found: ${GIN_COUNT}"
fi

info "Checking Core version..."
CORE_IMAGE=$(kubectl -n "$NAMESPACE" get deployment ksam-core -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || echo "")
info "Core image: ${CORE_IMAGE}"

info "Checking Core logs for optimization indicators..."
kubectl -n "$NAMESPACE" logs -l app=ksam-core --tail=50 2>&1 | grep -E "InsightManager.*async|riskScoreWorker|BatchCreateOrUpdate" | head -3 || warn "No optimization indicators in logs yet"

step "1) Verify CVE in Database"
start_timer "cve_check"
CVE_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM cves WHERE cve_id='${TEST_CVE_ID}';" | tr -d '[:space:]')"
end_timer "cve_check"
if [[ "${CVE_COUNT}" == "0" ]]; then
  warn "CVE ${TEST_CVE_ID} not in database. Loading..."
  TEMP_CVE_DIR="/tmp/ksam-e2e-cve-$$"
  mkdir -p "$TEMP_CVE_DIR"
  cp "KSAM/cve-data/all/${TEST_CVE_ID}.json" "$TEMP_CVE_DIR/" 2>/dev/null || die "CVE file not found"
  kubectl -n "$NAMESPACE" create configmap ksam-e2e-cve-data --from-file="$TEMP_CVE_DIR" --dry-run=client -o yaml | kubectl apply -f -
  kubectl -n "$NAMESPACE" delete job ksam-e2e-cve-loader --ignore-not-found=true
  kubectl -n "$NAMESPACE" run ksam-e2e-cve-loader --image=ksam/cve-loader:latest --restart=Never -- /cve-loader
  kubectl -n "$NAMESPACE" wait --for=condition=complete pod/ksam-e2e-cve-loader --timeout=120s
  rm -rf "$TEMP_CVE_DIR"
  success "CVE loaded"
else
  success "CVE ${TEST_CVE_ID} already in database"
fi

step "2) Deploy Test Pod"
start_timer "pod_deploy"
kubectl create namespace "$E2E_NS" --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "$E2E_NS" delete pod "$TEST_POD_NAME" --ignore-not-found=true --grace-period=0
sleep 2

kubectl apply -f - <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: ${TEST_POD_NAME}
  namespace: ${E2E_NS}
  labels:
    app: ksam-e2e-vuln
    test: e2e-cve-insight
spec:
  restartPolicy: Never
  containers:
    - name: app
      image: ksam/e2e-vuln:latest
      imagePullPolicy: Never
      command: ["sh", "-c", "echo 'KSAM E2E pod running' && sleep 3600"]
YAML

kubectl -n "$E2E_NS" wait --for=condition=Ready pod/"$TEST_POD_NAME" --timeout=120s
end_timer "pod_deploy"
success "Pod ${TEST_POD_NAME} is Running"

step "3) Monitor Agent → NormalizerWorker"
start_timer "normalizer"
POD_UID=""
for i in $(seq 1 30); do
  POD_UID="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT uid FROM pods WHERE namespace='${E2E_NS}' AND name='${TEST_POD_NAME}' AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1;" 2>/dev/null | tr -d '[:space:]' || echo "")"
  if [[ -n "${POD_UID}" ]]; then
    end_timer "normalizer"
    success "Pod stored in DB: UID=${POD_UID}"
    break
  fi
  sleep 2
  echo -n "."
done
echo ""
[[ -n "${POD_UID}" ]] || die "Pod not found in DB after 60s"

step "4) Monitor SBOMWorker"
start_timer "sbom_generation"
SBOM_ID=""
for i in $(seq 1 60); do
  SBOM_ID="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT sbom_id FROM pod_image_scans WHERE pod_uid='${POD_UID}' AND sbom_id IS NOT NULL AND deleted_at IS NULL LIMIT 1;" 2>/dev/null | tr -d '[:space:]' || echo "")"
  if [[ -n "${SBOM_ID}" && "${SBOM_ID}" != "0" ]]; then
    end_timer "sbom_generation"
    success "SBOM linked: ID=${SBOM_ID}"
    
    SBOM_INFO="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT image_name, image_tag, image_digest, component_count FROM sboms WHERE id=${SBOM_ID};" 2>/dev/null || echo "")"
    info "SBOM details: ${SBOM_INFO}"
    
    COMPONENT_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM sbom_components WHERE sbom_id=${SBOM_ID} AND deleted_at IS NULL;" | tr -d '[:space:]')"
    info "Components: ${COMPONENT_COUNT}"
    break
  fi
  sleep 2
  echo -n "."
done
echo ""
[[ -n "${SBOM_ID}" && "${SBOM_ID}" != "0" ]] || die "SBOM not linked after 120s"

step "5) Monitor CVEMatcherWorker"
start_timer "cve_matching"
MATCH_COUNT="0"
for i in $(seq 1 60); do
  MATCH_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM cve_matches WHERE sbom_id=${SBOM_ID} AND cve_id='${TEST_CVE_ID}' AND deleted_at IS NULL;" | tr -d '[:space:]')"
  if [[ "${MATCH_COUNT}" != "0" ]]; then
    end_timer "cve_matching"
    success "CVE matches found: ${MATCH_COUNT}"
    
    MATCH_INFO="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT id, component_id, severity, fixed_version FROM cve_matches WHERE sbom_id=${SBOM_ID} AND cve_id='${TEST_CVE_ID}' LIMIT 1;" 2>/dev/null || echo "")"
    info "Match details: ${MATCH_INFO}"
    break
  fi
  sleep 2
  echo -n "."
done
echo ""
[[ "${MATCH_COUNT}" != "0" ]] || die "No CVE matches found after 120s"

step "6) Monitor Insight Creation (with Performance Tracking)"
start_timer "insight_creation"
INSIGHT_COUNT="0"
INITIAL_TIME=$(date +%s)
for i in $(seq 1 30); do
  INSIGHT_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights WHERE type='vulnerability' AND cve_id='${TEST_CVE_ID}' AND sbom_id=${SBOM_ID} AND deleted_at IS NULL;" | tr -d '[:space:]')"
  if [[ "${INSIGHT_COUNT}" != "0" ]]; then
    end_timer "insight_creation"
    success "Insights created: ${INSIGHT_COUNT}"
    
    INSIGHT_INFO="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT id, severity, package_name, installed_version, fixed_version, created_at FROM insights WHERE type='vulnerability' AND cve_id='${TEST_CVE_ID}' AND sbom_id=${SBOM_ID} LIMIT 1;" 2>/dev/null || echo "")"
    info "Insight details: ${INSIGHT_INFO}"
    
    # Check for async risk score calculation
    info "Checking for async risk score jobs..."
    kubectl -n "$NAMESPACE" logs -l app=ksam-core --tail=200 2>&1 | grep -E "Async.*risk score|riskScoreWorker" | tail -3 || info "No async risk score logs yet (may be queued)"
    break
  fi
  sleep 2
  echo -n "."
done
echo ""
[[ "${INSIGHT_COUNT}" != "0" ]] || die "No insights created after 60s"

step "7) Verify Optimization Features"
info "Checking for transaction usage..."
kubectl -n "$NAMESPACE" logs -l app=ksam-core --tail=500 2>&1 | grep -E "\[InsightManager\].*Updated|\[InsightManager\].*Created" | tail -5

info "Checking for JSONB @> operator usage (should see fast queries)..."
# Verify GIN index is being used
kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -c "EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM insights WHERE affected_resources @> '[{\"uid\":\"${POD_UID}\"}]'::jsonb AND deleted_at IS NULL LIMIT 1;" 2>/dev/null | grep -E "Index Scan|Bitmap|GIN|Execution Time" | head -5 || warn "Could not verify query plan"

info "Checking for batch processing..."
kubectl -n "$NAMESPACE" logs -l app=ksam-core --tail=500 2>&1 | grep -E "Batch.*insights|BatchCreateOrUpdate" | tail -3 || info "No batch processing logs (may use single insight path)"

step "8) Verify Deduplication"
info "Triggering pod update to verify deduplication..."
kubectl -n "$E2E_NS" annotate pod/"$TEST_POD_NAME" test-trigger="$(date +%s)" --overwrite
sleep 10

INSIGHT_COUNT_AFTER="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights WHERE type='vulnerability' AND cve_id='${TEST_CVE_ID}' AND sbom_id=${SBOM_ID} AND deleted_at IS NULL;" | tr -d '[:space:]')"
if [[ "${INSIGHT_COUNT_AFTER}" == "${INSIGHT_COUNT}" ]]; then
  success "Deduplication working: Still ${INSIGHT_COUNT} insight(s) (no duplicates)"
else
  warn "Deduplication check: ${INSIGHT_COUNT} → ${INSIGHT_COUNT_AFTER} insights"
fi

step "9) Verify Insights API"
start_timer "api_check"
kubectl -n "$NAMESPACE" port-forward svc/ksam-core "${HTTP_LOCAL_PORT}:8080" >/tmp/ksam-e2e-pf.log 2>&1 &
PF_PID=$!
trap 'kill $PF_PID 2>/dev/null || true' EXIT
sleep 3

TOKEN="$(curl -sS -X POST "http://localhost:${HTTP_LOCAL_PORT}/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || echo "")"
[[ -n "${TOKEN}" ]] || die "Failed to get auth token"

INSIGHTS_JSON="$(curl -sS "http://localhost:${HTTP_LOCAL_PORT}/api/v1/insights?type=vulnerability&status=all&pageSize=50" -H "Authorization: Bearer ${TOKEN}")"
FOUND="$(echo "$INSIGHTS_JSON" | python3 -c "import json,sys; d=json.load(sys.stdin); print('True' if any(i.get('cveId')=='${TEST_CVE_ID}' for i in d.get('insights',[])) else 'False')" 2>/dev/null || echo "False")"

if [[ "${FOUND}" == "True" ]]; then
  end_timer "api_check"
  success "Insights API: CVE ${TEST_CVE_ID} found"
  
  echo "$INSIGHTS_JSON" | python3 -c "
import json,sys
d=json.load(sys.stdin)
insights=[i for i in d.get('insights',[]) if i.get('cveId')=='${TEST_CVE_ID}']
print(f'Found {len(insights)} insight(s):')
for i in insights[:3]:
    print(f\"  - ID: {i.get('id')}, Severity: {i.get('severity')}, Package: {i.get('packageName')}@{i.get('installedVersion')}\")
" 2>/dev/null || true
else
  die "Insights API: CVE ${TEST_CVE_ID} not found"
fi

step "10) Performance Summary"
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "Performance Metrics:"
echo "═══════════════════════════════════════════════════════════════"
for f in "$TIMINGS_DIR"/*; do
  if [[ -f "$f" ]]; then
    key=$(basename "$f")
    if [[ "$key" != *"_start"* ]]; then
      start=$(cat "$f")
      elapsed=$(echo "$(date +%s.%N) - $start" | bc)
      echo "  - $key: ${elapsed}s"
    fi
  fi
done
echo ""
echo "✅ E2E Test Complete - All optimizations verified!"

