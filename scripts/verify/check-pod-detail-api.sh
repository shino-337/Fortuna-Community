#!/usr/bin/env bash
# Check Pod Detail API: login (admin/admin123), list pods, get pod by id/uid, and show podIP, startTime, phase.
# Usage: CORE_URL=http://localhost:8080 ./scripts/verify/check-pod-detail-api.sh
#        Or with port-forward: kubectl port-forward -n fortuna svc/fortuna-core 8080:8080
#        Then: CORE_URL=http://localhost:8080 ./scripts/verify/check-pod-detail-api.sh

set -euo pipefail

CORE_URL="${CORE_URL:-http://localhost:8080}"
BASE="${CORE_URL%/}/api/v1"
USER="${POD_DETAIL_CHECK_USER:-admin}"
PASS="${POD_DETAIL_CHECK_PASS:-admin123}"

echo "=== Pod Detail API check ==="
echo "  CORE_URL=$CORE_URL"
echo "  Login: $USER / ****"
echo ""

# Login
LOGIN=$(curl -s -X POST "$BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}")
TOKEN=$(echo "$LOGIN" | jq -r '.token')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "Login failed. Response: $LOGIN"
  exit 1
fi
echo "[OK] Login successful"

# List pods (first page) — domain API: /inventory/pods
PODS=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE/inventory/pods?pageSize=5")
FIRST_UID=$(echo "$PODS" | jq -r '.pods[0].uid // empty')
TOTAL=$(echo "$PODS" | jq -r '.total // 0')

echo "[OK] GET /inventory/pods (total=$TOTAL)"
if [ -z "$FIRST_UID" ]; then
  echo "  No pods in cluster; skip pod detail fetch."
  exit 0
fi

echo "  First pod: uid=$FIRST_UID"
echo "  Fields from list response (first pod):"
echo "$PODS" | jq -r '.pods[0] | "    podIP: \(.podIP // "null"), startTime: \(.startTime // "null"), phase: \(.phase // "null"), restartCount: \(.restartCount // "null")"'

# Get pod by UID (domain API: by-id removed, only UID)
BY_UID=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE/inventory/pods/$(echo "$FIRST_UID" | jq -sRr @uri)")
echo ""
echo "[OK] GET /inventory/pods/$FIRST_UID"
echo "$BY_UID" | jq -r '"    podIP: \(.podIP // "null"), startTime: \(.startTime // "null"), phase: \(.phase // "null"), restartCount: \(.restartCount // "null"), qosClass: \(.qosClass // "null")"'

echo ""
echo "=== DB check (pod_ip, start_time) ==="
echo "  If podIP/startTime are null above, DB columns pod_ip and start_time are empty."
echo "  They are filled by the agent sync (payload has podIP, startTime from pod status)."
echo "  Trigger a sync: kubectl rollout restart daemonset/fortuna-agent -n fortuna && sleep 90"
echo "  Then re-run this script."
echo ""
echo "  Query DB: kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -t -c \"SELECT uid, name, pod_ip, start_time, phase FROM pods WHERE deleted_at IS NULL LIMIT 5;\""
