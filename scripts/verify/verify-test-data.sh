#!/bin/bash

set -euo pipefail

echo "=========================================="
echo "Test Data Verification Script"
echo "=========================================="

# Get Core service info
CORE_IP=$(kubectl get svc fortuna-core -n fortuna -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "localhost")
CORE_PORT=$(kubectl get svc fortuna-core -n fortuna -o jsonpath='{.spec.ports[?(@.name=="http")].port}' 2>/dev/null || echo "8080")
CORE_URL="http://${CORE_IP}:${CORE_PORT}/api/v1"

echo "Core URL: ${CORE_URL}"
echo ""

# 1. Check Promotion Rules API
echo "=== 1. Promotion Rules API ==="
echo "GET ${CORE_URL}/promotion-rules"
RESPONSE=$(curl -s "${CORE_URL}/promotion-rules" || echo '{"error":"API not available"}')
echo "${RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${RESPONSE}"
echo ""

# 2. Check Runtime Signals API (domain: /runtime/signals)
echo "=== 2. Runtime Signals API ==="
echo "GET ${CORE_URL}/runtime/signals?limit=5"
RESPONSE=$(curl -s "${CORE_URL}/runtime/signals?limit=5" || echo '{"error":"API not available"}')
echo "${RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${RESPONSE}"
echo ""

# 3. Check Pod Capabilities (domain: /inventory/pods/:uid/capabilities)
echo "=== 3. Pod Capabilities API ==="
POD_UID=$(curl -s "${CORE_URL}/inventory/pods?limit=1" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('pods', [{}])[0].get('uid', ''))" 2>/dev/null || echo "")
if [ -n "$POD_UID" ] && [ "$POD_UID" != "None" ]; then
  echo "GET ${CORE_URL}/inventory/pods/${POD_UID}/capabilities"
  RESPONSE=$(curl -s "${CORE_URL}/inventory/pods/${POD_UID}/capabilities" || echo '{"error":"API not available"}')
  echo "${RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${RESPONSE}"
else
  echo "No pods found to query capabilities"
fi
echo ""

# 4. Database Schema Info (if psql available)
if command -v psql &> /dev/null; then
  echo "=== 4. Database Schema ==="
  echo "Table: pod_capabilities"
  psql -h localhost -U fortuna -d fortuna -c "\d pod_capabilities" 2>/dev/null || echo "Cannot connect to database"
  echo ""
  
  echo "Table: promotion_rules"
  psql -h localhost -U fortuna -d fortuna -c "\d promotion_rules" 2>/dev/null || echo "Cannot connect to database"
  echo ""
  
  echo "Table: runtime_signals"
  psql -h localhost -U fortuna -d fortuna -c "\d runtime_signals" 2>/dev/null || echo "Cannot connect to database"
  echo ""
  
  echo "=== 5. Database Data Counts ==="
  psql -h localhost -U fortuna -d fortuna -c "
    SELECT 'pod_capabilities' as table_name, COUNT(*) as count FROM pod_capabilities
    UNION ALL
    SELECT 'promotion_rules', COUNT(*) FROM promotion_rules
    UNION ALL
    SELECT 'runtime_signals', COUNT(*) FROM runtime_signals
    UNION ALL
    SELECT 'pod_attack_steps', COUNT(*) FROM pod_attack_steps;
  " 2>/dev/null || echo "Cannot connect to database"
  echo ""
  
  echo "=== 6. Sample Capability States ==="
  psql -h localhost -U fortuna -d fortuna -c "
    SELECT capability_id, state, COUNT(*) as count 
    FROM pod_capabilities 
    GROUP BY capability_id, state 
    ORDER BY capability_id, state 
    LIMIT 10;
  " 2>/dev/null || echo "Cannot connect to database"
  echo ""
else
  echo "=== 4. Database Schema ==="
  echo "psql not available. To check database schema, run:"
  echo "  kubectl exec -it <postgres-pod> -n fortuna -- psql -U fortuna -d fortuna -c '\\d pod_capabilities'"
  echo ""
fi

echo "=========================================="
echo "Verification Complete"
echo "=========================================="
