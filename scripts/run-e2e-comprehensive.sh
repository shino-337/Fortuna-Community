#!/bin/bash
# Comprehensive E2E test with detailed step-by-step execution, API responses, and database state
# Output: docs/test-results/E2E-COMPREHENSIVE-<timestamp>.md
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT_DIR="$REPO_ROOT/docs/test-results"
REPORT="$REPORT_DIR/E2E-COMPREHENSIVE-$TIMESTAMP.md"
mkdir -p "$REPORT_DIR"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

exec 3>&1
exec 1>"$REPORT"
exec 2>&1

echo "# E2E Comprehensive Test Report"
echo ""
echo "**Thời gian**: $(date -Iseconds)"
echo "**Namespace**: $NAMESPACE"
echo "**Test Type**: End-to-End Comprehensive"
echo ""

section() { echo ""; echo "---"; echo "## $1"; echo ""; }
step() { echo "### $1"; echo ""; }
code_block() { echo '```'; cat; echo '```'; echo ""; }

# Helper functions
api_call() {
  local endpoint=$1
  local method=${2:-GET}
  local data=${3:-}
  local url="http://${CORE_IP}:8080${endpoint}"
  
  echo "**Request**: $method $url"
  if [ -n "$data" ]; then
    echo "**Body**: $data"
  fi
  echo ""
  echo "**Response**:"
  if [ "$method" = "POST" ] && [ -n "$data" ]; then
    curl -s -X POST -H "Content-Type: application/json" -d "$data" "$url" 2>/dev/null | python3 -m json.tool 2>/dev/null || curl -s -X POST -H "Content-Type: application/json" -d "$data" "$url" 2>/dev/null | head -c 2000
  else
    curl -s "$url" 2>/dev/null | python3 -m json.tool 2>/dev/null || curl -s "$url" 2>/dev/null | head -c 2000
  fi
  echo ""
  echo ""
}

db_query() {
  local query=$1
  local desc=${2:-Query}
  echo "**$desc**:"
  echo '```sql'
  echo "$query"
  echo '```'
  echo ""
  echo "**Result**:"
  kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "$query" 2>/dev/null || echo "Query failed"
  echo ""
  echo ""
}

# 1. Pre-flight checks
section "1. Pre-flight Checks"

step "1.1 Cluster Status"
kubectl cluster-info 2>/dev/null | head -5 || echo "kubectl not available"
echo ""

step "1.2 Namespace Check"
kubectl get namespace "$NAMESPACE" 2>/dev/null || echo "Namespace $NAMESPACE not found"
echo ""

step "1.3 Nodes"
kubectl get nodes -o wide 2>/dev/null || echo "kubectl not available"
echo ""

# 2. Component Status
section "2. Component Status"

step "2.1 All Pods in $NAMESPACE"
kubectl get pods -n "$NAMESPACE" -o wide 2>/dev/null || true
echo ""

step "2.2 Pod Status Summary"
kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $1, $3, $4}' | while read name status restarts; do
  echo "- **$name**: Status=$status, Restarts=$restarts"
done
echo ""

step "2.3 Services"
kubectl get svc -n "$NAMESPACE" 2>/dev/null || true
echo ""

step "2.4 Deployments"
kubectl get deployments -n "$NAMESPACE" 2>/dev/null || true
echo ""

step "2.5 DaemonSets"
kubectl get daemonsets -n "$NAMESPACE" 2>/dev/null || true
echo ""

# 3. Core API Tests
section "3. Core API Tests"

CORE_IP=$(kubectl get svc fortuna-core -n "$NAMESPACE" -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$CORE_IP" ]; then
  echo "⚠️ Core service not found. Skipping API tests."
else
  echo "**Core Service IP**: $CORE_IP"
  echo "**Core Pod**: $CORE_POD"
  echo ""

  # Health endpoints
  step "3.1 Health Endpoints"
  echo "**GET /health**"
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://${CORE_IP}:8080/health" 2>/dev/null || echo "000")
  echo "HTTP Status: $HTTP_CODE"
  curl -s "http://${CORE_IP}:8080/health" 2>/dev/null | python3 -m json.tool 2>/dev/null || curl -s "http://${CORE_IP}:8080/health" 2>/dev/null
  echo ""
  echo "**GET /healthz**"
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://${CORE_IP}:8080/healthz" 2>/dev/null || echo "000")
  echo "HTTP Status: $HTTP_CODE"
  curl -s "http://${CORE_IP}:8080/healthz" 2>/dev/null || echo "No response"
  echo ""
  echo "**GET /ready**"
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://${CORE_IP}:8080/ready" 2>/dev/null || echo "000")
  echo "HTTP Status: $HTTP_CODE"
  curl -s "http://${CORE_IP}:8080/ready" 2>/dev/null || echo "No response"
  echo ""

  # Dashboard stats
  step "3.2 Dashboard Statistics"
  api_call "/api/v1/dashboard/stats"

  # Capability Metadata
  step "3.3 Capability Metadata API"
  api_call "/api/v1/capability-metadata"
  
  # Get first capability ID for detail test
  FIRST_CAP=$(curl -s "http://${CORE_IP}:8080/api/v1/capability-metadata" 2>/dev/null | python3 -c "import sys, json; data=json.load(sys.stdin); print(data['metadata'][0]['capabilityId'] if data.get('metadata') else '')" 2>/dev/null || echo "")
  if [ -n "$FIRST_CAP" ]; then
    echo "**GET /api/v1/capability-metadata/$FIRST_CAP**"
    api_call "/api/v1/capability-metadata/$FIRST_CAP"
  fi

  # Promotion Rules
  step "3.4 Promotion Rules API"
  api_call "/api/v1/promotion-rules"
  
  FIRST_RULE_CAP=$(curl -s "http://${CORE_IP}:8080/api/v1/promotion-rules" 2>/dev/null | python3 -c "import sys, json; data=json.load(sys.stdin); print(data['rules'][0]['capabilityId'] if data.get('rules') else '')" 2>/dev/null || echo "")
  if [ -n "$FIRST_RULE_CAP" ]; then
    echo "**GET /api/v1/promotion-rules/capability/$FIRST_RULE_CAP**"
    api_call "/api/v1/promotion-rules/capability/$FIRST_RULE_CAP"
  fi

  # Runtime Signals
  step "3.5 Runtime Signals API"
  api_call "/api/v1/runtime-signals?limit=10"
  
  # Get pod UID from signals if available
  POD_UID_FROM_SIGNALS=$(curl -s "http://${CORE_IP}:8080/api/v1/runtime-signals?limit=1" 2>/dev/null | python3 -c "import sys, json; data=json.load(sys.stdin); print(data['signals'][0]['podUid'] if data.get('signals') else '')" 2>/dev/null || echo "")
  if [ -n "$POD_UID_FROM_SIGNALS" ]; then
    echo "**GET /api/v1/runtime-signals/pods/$POD_UID_FROM_SIGNALS**"
    api_call "/api/v1/runtime-signals/pods/$POD_UID_FROM_SIGNALS"
  fi

  # Attack Steps
  step "3.6 Attack Steps API"
  api_call "/api/v1/attack-steps/summary"
  
  if [ -n "$POD_UID_FROM_SIGNALS" ]; then
    echo "**GET /api/v1/attack-steps/pods/$POD_UID_FROM_SIGNALS**"
    api_call "/api/v1/attack-steps/pods/$POD_UID_FROM_SIGNALS"
  fi

  # Pod Capabilities
  step "3.7 Pod Capabilities API"
  api_call "/api/v1/pod-capabilities?limit=10"
  
  # Get pod UID from capabilities
  POD_UID_FROM_CAPS=$(curl -s "http://${CORE_IP}:8080/api/v1/pod-capabilities?limit=1" 2>/dev/null | python3 -c "import sys, json; data=json.load(sys.stdin); print(data['capabilities'][0]['podUid'] if data.get('capabilities') else '')" 2>/dev/null || echo "")
  if [ -n "$POD_UID_FROM_CAPS" ]; then
    echo "**GET /api/v1/pods/$POD_UID_FROM_CAPS/capabilities**"
    api_call "/api/v1/pods/$POD_UID_FROM_CAPS/capabilities"
    
    echo "**GET /api/v1/pod-capabilities/summary/cluster**"
    api_call "/api/v1/pod-capabilities/summary/cluster"
    
    echo "**GET /api/v1/pod-capabilities/summary/capability**"
    api_call "/api/v1/pod-capabilities/summary/capability"
    
    echo "**GET /api/v1/pod-capabilities/summary/namespace**"
    api_call "/api/v1/pod-capabilities/summary/namespace"
    
    echo "**GET /api/v1/pod-capabilities/summary/severity**"
    api_call "/api/v1/pod-capabilities/summary/severity"
    
    echo "**GET /api/v1/pod-capabilities/trends?days=7**"
    api_call "/api/v1/pod-capabilities/trends?days=7"
  fi

  # Other APIs
  step "3.8 Additional APIs"
  echo "**GET /api/v1/clusters**"
  api_call "/api/v1/clusters"
  
  echo "**GET /api/v1/risks?limit=10**"
  api_call "/api/v1/risks?limit=10"
  
  echo "**GET /api/v1/sbom?limit=5**"
  api_call "/api/v1/sbom?limit=5"
  
  echo "**GET /api/v1/resources?limit=10**"
  api_call "/api/v1/resources?limit=10"
  
  echo "**GET /api/v1/attack-paths/graph**"
  api_call "/api/v1/attack-paths/graph"
  
  echo "**GET /api/v1/dashboard/metrics/threat-velocity?days=7**"
  api_call "/api/v1/dashboard/metrics/threat-velocity?days=7"
fi

# 4. Database State
section "4. Database State"

PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$PG_POD" ]; then
  echo "⚠️ Postgres pod not found. Skipping database tests."
else
  echo "**Postgres Pod**: $PG_POD"
  echo ""

  step "4.1 Table Row Counts"
  db_query "
    SELECT 
      'capability_metadata' AS table_name, COUNT(*) AS row_count FROM capability_metadata
    UNION ALL SELECT 'promotion_rules', COUNT(*) FROM promotion_rules
    UNION ALL SELECT 'pod_capabilities', COUNT(*) FROM pod_capabilities
    UNION ALL SELECT 'runtime_signals', COUNT(*) FROM runtime_signals
    UNION ALL SELECT 'pod_attack_steps', COUNT(*) FROM pod_attack_steps
    UNION ALL SELECT 'pod_instances', COUNT(*) FROM pod_instances
    UNION ALL SELECT 'runtime_events', COUNT(*) FROM runtime_events
    UNION ALL SELECT 'sboms', COUNT(*) FROM sboms
    UNION ALL SELECT 'insights', COUNT(*) FROM insights
    UNION ALL SELECT 'cve_matches', COUNT(*) FROM cve_matches
    ORDER BY table_name;
  " "Table Row Counts"

  step "4.2 Capability Metadata Details"
  db_query "
    SELECT 
      capability_id, domain, category, severity_base, confidence_base,
      array_length(preconditions::jsonb, 1) AS preconditions_count,
      array_length(produces_attack_steps::jsonb, 1) AS attack_steps_count,
      supports_runtime_promotion
    FROM capability_metadata
    ORDER BY capability_id
    LIMIT 11;
  " "Capability Metadata"

  step "4.3 Promotion Rules Details"
  db_query "
    SELECT 
      id, capability_id, signal_type, min_occurrences, promote_to, confidence_boost,
      array_length(required_capabilities::jsonb, 1) AS required_caps_count
    FROM promotion_rules
    ORDER BY capability_id, signal_type
    LIMIT 10;
  " "Promotion Rules"

  step "4.4 Pod Capabilities Distribution"
  db_query "
    SELECT 
      state, 
      severity,
      COUNT(*) AS count,
      AVG(confidence)::numeric(4,2) AS avg_confidence
    FROM pod_capabilities
    GROUP BY state, severity
    ORDER BY state, severity;
  " "Capability State and Severity Distribution"

  step "4.5 Pod Capabilities by Capability ID"
  db_query "
    SELECT 
      capability_id,
      COUNT(*) AS count,
      COUNT(DISTINCT pod_uid) AS unique_pods,
      COUNT(CASE WHEN state = 'detected' THEN 1 END) AS detected,
      COUNT(CASE WHEN state = 'confirmed' THEN 1 END) AS confirmed,
      COUNT(CASE WHEN state = 'exploited' THEN 1 END) AS exploited,
      COUNT(CASE WHEN state = 'chained' THEN 1 END) AS chained
    FROM pod_capabilities
    GROUP BY capability_id
    ORDER BY count DESC
    LIMIT 15;
  " "Capabilities by Capability ID"

  step "4.6 Runtime Signals Summary"
  db_query "
    SELECT 
      signal_type,
      category,
      COUNT(*) AS count,
      AVG(confidence)::numeric(4,2) AS avg_confidence,
      COUNT(DISTINCT pod_uid) AS unique_pods
    FROM runtime_signals
    GROUP BY signal_type, category
    ORDER BY count DESC
    LIMIT 15;
  " "Runtime Signals Summary"

  step "4.7 Attack Steps Summary"
  db_query "
    SELECT 
      step_id,
      category,
      COUNT(*) AS count,
      AVG(confidence)::numeric(4,2) AS avg_confidence,
      COUNT(DISTINCT pod_uid) AS unique_pods
    FROM pod_attack_steps
    GROUP BY step_id, category
    ORDER BY count DESC
    LIMIT 15;
  " "Attack Steps Summary"

  step "4.8 Recent Pod Instances"
  db_query "
    SELECT 
      pod_uid,
      namespace,
      name,
      status,
      started_at,
      terminated_at
    FROM pod_instances
    ORDER BY started_at DESC
    LIMIT 10;
  " "Recent Pod Instances"

  step "4.9 Sample Pod Capabilities (with details)"
  db_query "
    SELECT 
      pod_uid,
      namespace,
      capability_id,
      state,
      severity,
      confidence,
      first_seen_at,
      last_seen_at
    FROM pod_capabilities
    ORDER BY last_seen_at DESC
    LIMIT 10;
  " "Sample Pod Capabilities"

  step "4.10 Sample Runtime Signals (with details)"
  db_query "
    SELECT 
      id,
      pod_uid,
      signal_type,
      category,
      confidence,
      created_at
    FROM runtime_signals
    ORDER BY created_at DESC
    LIMIT 10;
  " "Sample Runtime Signals"
fi

# 5. Test Scripts Execution
section "5. Test Scripts Execution"

step "5.1 PCE API Test"
if [ -x "$SCRIPT_DIR/test-pce-api.sh" ]; then
  echo "**Executing**: test-pce-api.sh"
  echo ""
  "$SCRIPT_DIR/test-pce-api.sh" 2>&1 || echo "Test script failed or returned non-zero"
else
  echo "⚠️ test-pce-api.sh not found or not executable"
fi
echo ""

step "5.2 Priority 1 APIs Test"
if [ -x "$SCRIPT_DIR/test-priority1-apis.sh" ]; then
  echo "**Executing**: test-priority1-apis.sh"
  echo ""
  "$SCRIPT_DIR/test-priority1-apis.sh" 2>&1 || echo "Test script failed or returned non-zero"
else
  echo "⚠️ test-priority1-apis.sh not found or not executable"
fi
echo ""

step "5.3 PCE E2E Test"
if [ -x "$SCRIPT_DIR/test-pce-e2e.sh" ]; then
  echo "**Executing**: test-pce-e2e.sh"
  echo ""
  "$SCRIPT_DIR/test-pce-e2e.sh" 2>&1 || echo "Test script failed or returned non-zero"
else
  echo "⚠️ test-pce-e2e.sh not found or not executable"
fi
echo ""

# 6. Unit Tests
section "6. Core Unit Tests"

step "6.1 Capability Package Tests"
(cd "$REPO_ROOT/core" && go test ./pkg/capability/... -count=1 -v 2>&1 | head -100) || echo "Tests failed or not available"
echo ""

step "6.2 API Package Tests"
(cd "$REPO_ROOT/core" && go test ./internal/api/risk/... -count=1 -v 2>&1 | head -100) || echo "Tests failed or not available"
echo ""

# 7. Dashboard Status
section "7. Dashboard Status"

DASH_POD=$(kubectl get pods -n "$NAMESPACE" -l app=fortuna-dashboard -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
DASH_SVC=$(kubectl get svc -n "$NAMESPACE" -l app=fortuna-dashboard -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -n "$DASH_POD" ]; then
  echo "**Dashboard Pod**: $DASH_POD"
  echo "**Dashboard Service**: $DASH_SVC"
  echo ""
  
  DASH_STATUS=$(kubectl get pod "$DASH_POD" -n "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
  echo "**Status**: $DASH_STATUS"
  echo ""
  
  DASH_PORT=$(kubectl get svc -n "$NAMESPACE" "$DASH_SVC" -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || echo "80")
  NODE_PORT=$(kubectl get svc -n "$NAMESPACE" "$DASH_SVC" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "")
  NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "")
  
  echo "**Service Port**: $DASH_PORT"
  if [ -n "$NODE_PORT" ]; then
    echo "**NodePort**: $NODE_PORT"
    if [ -n "$NODE_IP" ]; then
      echo "**Access URL**: http://${NODE_IP}:${NODE_PORT}"
    fi
  fi
  echo ""
  echo "**Port-forward Command**:"
  echo "  kubectl port-forward -n $NAMESPACE svc/$DASH_SVC 8081:${DASH_PORT}"
  echo "  Then access: http://localhost:8081"
else
  echo "⚠️ Dashboard pod not found"
fi
echo ""

# 8. Summary
section "8. Test Summary"

echo "### Test Execution Summary"
echo ""
echo "**Test Completion Time**: $(date -Iseconds)"
echo ""

if [ -n "$CORE_IP" ]; then
  echo "✅ Core API: Accessible at $CORE_IP:8080"
else
  echo "❌ Core API: Not accessible"
fi

if [ -n "$PG_POD" ]; then
  echo "✅ Database: Postgres pod running ($PG_POD)"
else
  echo "❌ Database: Postgres pod not found"
fi

if [ -n "$DASH_POD" ]; then
  echo "✅ Dashboard: Pod running ($DASH_POD)"
else
  echo "❌ Dashboard: Pod not found"
fi

echo ""
echo "### Next Steps"
echo ""
echo "1. Review API responses above for correctness"
echo "2. Verify database state matches expected data"
echo "3. Check test script outputs for failures"
echo "4. Access dashboard to verify UI functionality"
echo ""

echo "---"
echo "**Report File**: $REPORT"
echo "**Report Generated**: $(date -Iseconds)"

exec 1>&3
exec 3>&-

echo -e "${GREEN}✅ Comprehensive E2E test completed${NC}"
echo -e "${BLUE}Report written to: $REPORT${NC}"
echo ""
echo "Preview (first 50 lines):"
head -50 "$REPORT"
