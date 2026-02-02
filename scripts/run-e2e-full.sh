#!/bin/bash
# E2E full run with detailed report: step-by-step, API responses, DB state, dashboard.
# Output: docs/test-results/E2E-FULL-<timestamp>.md
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT_DIR="$REPO_ROOT/docs/test-results"
REPORT="$REPORT_DIR/E2E-FULL-$TIMESTAMP.md"
mkdir -p "$REPORT_DIR"

exec 3>&1
exec 1>"$REPORT"
exec 2>&1

echo "# E2E Full Run – Chi tiết từng bước"
echo ""
echo "**Thời gian**: $(date -Iseconds)"
echo "**Namespace**: $NAMESPACE"
echo ""

section() { echo ""; echo "---"; echo "## $1"; echo ""; }
step() { echo "### $1"; echo ""; }

# 1. Cluster & pods
section "1. Cluster và Pods"
step "1.1 Nodes"
kubectl get nodes -o wide 2>/dev/null || echo "kubectl not available"
step "1.2 Pods (fortuna namespace)"
kubectl get pods -n "$NAMESPACE" -o wide 2>/dev/null || true
step "1.3 Services"
kubectl get svc -n "$NAMESPACE" 2>/dev/null || true

# 2. Core API
section "2. Core API"
CORE_IP=$(kubectl get svc fortuna-core -n "$NAMESPACE" -o jsonpath='{.spec.clusterIP}' 2>/dev/null)
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
echo "Core IP: ${CORE_IP}"
echo "Core Pod: ${CORE_POD}"
echo ""

# JWT: login and get Bearer token (env FORTUNA_E2E_USER / FORTUNA_E2E_PASSWORD, default admin/admin123)
E2E_USER="${FORTUNA_E2E_USER:-admin}"
E2E_PASS="${FORTUNA_E2E_PASSWORD:-admin123}"
LOGIN_RESP=$(curl -s -X POST "http://${CORE_IP}:8080/api/v1/auth/login" -H "Content-Type: application/json" -d "{\"username\":\"$E2E_USER\",\"password\":\"$E2E_PASS\"}" 2>/dev/null || echo "{}")
TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token',''))" 2>/dev/null || echo "")
if [ -n "$TOKEN" ]; then
  echo "JWT obtained for $E2E_USER (Authorization: Bearer ...)"
else
  echo "Login failed or Core unreachable from host; API calls may return 401."
fi
echo ""

step "2.1 Health"
echo "GET http://${CORE_IP}:8080/health"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://${CORE_IP}:8080/health" 2>/dev/null || echo "000")
echo "HTTP $HTTP_CODE"
echo ""

step "2.2 GET /api/v1/capability-metadata"
echo "Response:"
if [ -n "$TOKEN" ]; then
  curl -s -H "Authorization: Bearer $TOKEN" "http://${CORE_IP}:8080/api/v1/capability-metadata" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -60 || curl -s -H "Authorization: Bearer $TOKEN" "http://${CORE_IP}:8080/api/v1/capability-metadata" 2>/dev/null | head -c 800
else
  curl -s "http://${CORE_IP}:8080/api/v1/capability-metadata" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -60 || curl -s "http://${CORE_IP}:8080/api/v1/capability-metadata" 2>/dev/null | head -c 800
fi
echo ""
echo ""

step "2.3 GET /api/v1/promotion-rules"
echo "Response:"
if [ -n "$TOKEN" ]; then
  curl -s -H "Authorization: Bearer $TOKEN" "http://${CORE_IP}:8080/api/v1/promotion-rules" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -80 || curl -s -H "Authorization: Bearer $TOKEN" "http://${CORE_IP}:8080/api/v1/promotion-rules" 2>/dev/null | head -c 1000
else
  curl -s "http://${CORE_IP}:8080/api/v1/promotion-rules" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -80 || curl -s "http://${CORE_IP}:8080/api/v1/promotion-rules" 2>/dev/null | head -c 1000
fi
echo ""
echo ""

step "2.4 GET /api/v1/promotion-rules/capability/ESC_HOSTPATH_NODE"
if [ -n "$TOKEN" ]; then
  curl -s -H "Authorization: Bearer $TOKEN" "http://${CORE_IP}:8080/api/v1/promotion-rules/capability/ESC_HOSTPATH_NODE" 2>/dev/null | python3 -m json.tool 2>/dev/null || curl -s -H "Authorization: Bearer $TOKEN" "http://${CORE_IP}:8080/api/v1/promotion-rules/capability/ESC_HOSTPATH_NODE" 2>/dev/null | head -c 600
else
  curl -s "http://${CORE_IP}:8080/api/v1/promotion-rules/capability/ESC_HOSTPATH_NODE" 2>/dev/null | python3 -m json.tool 2>/dev/null || curl -s "http://${CORE_IP}:8080/api/v1/promotion-rules/capability/ESC_HOSTPATH_NODE" 2>/dev/null | head -c 600
fi
echo ""
echo ""

step "2.5 GET /api/v1/runtime-signals?limit=5"
if [ -n "$TOKEN" ]; then
  curl -s -H "Authorization: Bearer $TOKEN" "http://${CORE_IP}:8080/api/v1/runtime-signals?limit=5" 2>/dev/null | python3 -m json.tool 2>/dev/null || curl -s -H "Authorization: Bearer $TOKEN" "http://${CORE_IP}:8080/api/v1/runtime-signals?limit=5" 2>/dev/null | head -c 600
else
  curl -s "http://${CORE_IP}:8080/api/v1/runtime-signals?limit=5" 2>/dev/null | python3 -m json.tool 2>/dev/null || curl -s "http://${CORE_IP}:8080/api/v1/runtime-signals?limit=5" 2>/dev/null | head -c 600
fi
echo ""
echo ""

step "2.6 GET /api/v1/attack-steps/summary"
if [ -n "$TOKEN" ]; then
  curl -s -H "Authorization: Bearer $TOKEN" "http://${CORE_IP}:8080/api/v1/attack-steps/summary" 2>/dev/null | python3 -m json.tool 2>/dev/null || curl -s -H "Authorization: Bearer $TOKEN" "http://${CORE_IP}:8080/api/v1/attack-steps/summary" 2>/dev/null
else
  curl -s "http://${CORE_IP}:8080/api/v1/attack-steps/summary" 2>/dev/null | python3 -m json.tool 2>/dev/null || curl -s "http://${CORE_IP}:8080/api/v1/attack-steps/summary" 2>/dev/null
fi
echo ""
echo ""

# Get a pod UID for pod capabilities test
POD_UID=$(kubectl exec -n "$NAMESPACE" deployment/postgres -- psql -U postgres -d fortuna -t -c "SELECT pod_uid FROM pod_capabilities LIMIT 1;" 2>/dev/null | tr -d ' ' | head -1)
if [ -n "$POD_UID" ] && [ "$POD_UID" != "" ]; then
  step "2.7 GET /api/v1/pods/${POD_UID}/capabilities"
  echo "Pod UID: $POD_UID"
  if [ -n "$TOKEN" ]; then
    curl -s -H "Authorization: Bearer $TOKEN" "http://${CORE_IP}:8080/api/v1/pods/${POD_UID}/capabilities" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -50 || curl -s -H "Authorization: Bearer $TOKEN" "http://${CORE_IP}:8080/api/v1/pods/${POD_UID}/capabilities" 2>/dev/null | head -c 800
  else
    curl -s "http://${CORE_IP}:8080/api/v1/pods/${POD_UID}/capabilities" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -50 || curl -s "http://${CORE_IP}:8080/api/v1/pods/${POD_UID}/capabilities" 2>/dev/null | head -c 800
  fi
  echo ""
  echo ""
fi

# 3. Database
section "3. Database (Postgres)"
PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$PG_POD" ]; then
  step "3.1 Row counts"
  kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "
    SELECT 'promotion_rules' AS tbl, COUNT(*) FROM promotion_rules
    UNION ALL SELECT 'capability_metadata', COUNT(*) FROM capability_metadata
    UNION ALL SELECT 'pod_capabilities', COUNT(*) FROM pod_capabilities
    UNION ALL SELECT 'runtime_signals', COUNT(*) FROM runtime_signals
    UNION ALL SELECT 'pod_attack_steps', COUNT(*) FROM pod_attack_steps;
  " 2>/dev/null || echo "Query failed"
  step "3.2 Capability states distribution"
  kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT state, COUNT(*) FROM pod_capabilities GROUP BY state ORDER BY state;" 2>/dev/null || true
  step "3.3 Sample promotion_rules (first 3)"
  kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT capability_id, signal_type, promote_to, min_occurrences, required_capabilities FROM promotion_rules LIMIT 3;" 2>/dev/null || true
  step "3.4 Sample pod_capabilities (first 3)"
  kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT pod_uid, capability_id, state, severity, confidence FROM pod_capabilities LIMIT 3;" 2>/dev/null || true
else
  echo "Postgres pod not found."
fi

# 4. Dashboard
section "4. Dashboard"
DASH_SVC=$(kubectl get svc -n "$NAMESPACE" -l app.kubernetes.io/component=dashboard -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
DASH_PORT=$(kubectl get svc -n "$NAMESPACE" "$DASH_SVC" -o jsonpath='{.spec.ports[0].port}' 2>/dev/null 2>/dev/null || echo "80")
NODE_PORT=$(kubectl get svc -n "$NAMESPACE" "$DASH_SVC" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "")
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "")
echo "Dashboard service: $DASH_SVC"
echo "Port: $DASH_PORT"
if [ -n "$NODE_PORT" ]; then
  echo "NodePort: $NODE_PORT"
  if [ -n "$NODE_IP" ]; then
    echo "URL: http://${NODE_IP}:${NODE_PORT}"
  fi
fi
echo ""
echo "Port-forward (nếu cần):"
echo "  kubectl port-forward -n $NAMESPACE svc/$DASH_SVC 8081:${DASH_PORT}"
echo "  Sau đó truy cập: http://localhost:8081"
echo ""

# 5. Test scripts
section "5. Test Scripts"
step "5.1 PCE API test"
if [ -x "$SCRIPT_DIR/test-pce-api.sh" ]; then
  "$SCRIPT_DIR/test-pce-api.sh" 2>&1 || true
else
  echo "test-pce-api.sh not found"
fi
step "5.2 Priority 1 APIs test"
if [ -x "$SCRIPT_DIR/test-priority1-apis.sh" ]; then
  "$SCRIPT_DIR/test-priority1-apis.sh" 2>&1 || true
else
  echo "test-priority1-apis.sh not found"
fi

# 6. Unit tests
section "6. Core unit tests"
(cd "$REPO_ROOT/core" && go test ./pkg/capability/... ./internal/api/risk/... -count=1 -short 2>&1) || true

echo ""
echo "---"
echo "**Kết thúc báo cáo E2E.** File: $REPORT"

exec 1>&3
exec 3>&-
echo "Report written to: $REPORT"
cat "$REPORT" | head -100
