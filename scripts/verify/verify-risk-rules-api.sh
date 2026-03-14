#!/usr/bin/env bash
# Verify Risk Rules API: list source, count, and optionally create one rule.
# Usage:
#   ./scripts/verify/verify-risk-rules-api.sh [BASE_URL]
# Example: BASE_URL=http://localhost:8080 ./scripts/verify/verify-risk-rules-api.sh

set -e
BASE_URL="${1:-http://localhost:8080}"
API="${BASE_URL}/api/v1"

echo "=========================================="
echo "Risk Rules API verification"
echo "BASE_URL=$BASE_URL"
echo "=========================================="

# 1. GET /risk/rules — list (this is what Settings → Risk Rules uses)
echo ""
echo "1. GET $API/risk/rules (list used by Settings → Risk Rules)"
RESP=$(curl -s -w "\n%{http_code}" "$API/risk/rules")
BODY=$(echo "$RESP" | head -n -1)
CODE=$(echo "$RESP" | tail -n 1)
echo "   HTTP $CODE"
if [ "$CODE" = "200" ]; then
  SOURCE=$(echo "$BODY" | jq -r '.source // "n/a"')
  TOTAL=$(echo "$BODY" | jq -r '.total // 0')
  echo "   source: $SOURCE"
  echo "   total:  $TOTAL"
  echo "   Rule IDs: $(echo "$BODY" | jq -r '.rules[].id' 2>/dev/null | tr '\n' ' ')"
  if [ "$SOURCE" = "files" ]; then
    echo ""
    echo "   ⚠️  List is from FILES (read-only). Risk rules you add go to DB."
    echo "   If you added a rule in Settings but see source=files, the DB table"
    echo "   risk_rules may be missing or empty. Check migration 075 and DB."
  fi
else
  echo "   body: $BODY"
fi

# 2. Compare with Policy Rules (different API — Rules page uses this)
echo ""
echo "2. GET $API/policy/rules (list used by Rules page — Policy rules, not Risk rules)"
POLICY_RESP=$(curl -s -w "\n%{http_code}" "$API/policy/rules")
POLICY_BODY=$(echo "$POLICY_RESP" | head -n -1)
POLICY_CODE=$(echo "$POLICY_RESP" | tail -n 1)
echo "   HTTP $POLICY_CODE"
if [ "$POLICY_CODE" = "200" ]; then
  POLICY_COUNT=$(echo "$POLICY_BODY" | jq -r '.rules | length' 2>/dev/null || echo "?")
  echo "   rules count: $POLICY_COUNT"
  echo "   (Reload Rules Engine on Rules page only affects this policy list, not Risk rules.)"
fi

echo ""
echo "=========================================="
echo "Summary:"
echo "  - Settings → Risk Rules  => GET /risk/rules   (source: db or files)"
echo "  - Rules page (Policy)    => GET /policy/rules (YAML engine)"
echo "  - New risk rules added in Settings are stored in DB and appear in Settings only."
echo "  - Reload Rules Engine on the Rules page does NOT refresh Risk rules."
echo "=========================================="
