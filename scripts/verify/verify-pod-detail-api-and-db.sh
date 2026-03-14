#!/usr/bin/env bash
# =============================================================================
# Verify Pod Detail (POD_DETAIL_SPEC): DB schema (migration 068), sample data,
# and Core API response (podIP, startTime, restartCount, ownerKind, qosClass).
# Usage: NAMESPACE=fortuna ./scripts/verify/verify-pod-detail-api-and-db.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
DB_NAME="${DB_NAME:-fortuna}"
DB_USER="${DB_USER:-postgres}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
info() { echo -e "${BLUE}[INFO]${NC} $1"; }
ok() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }

echo ""
echo "=========================================="
echo "Pod Detail: DB schema + data + API"
echo "=========================================="
echo ""

PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

if [ -z "$PG_POD" ]; then
  fail "Postgres pod not found in namespace $NAMESPACE"
  exit 1
fi
if [ -z "$CORE_POD" ]; then
  warn "Core pod not found; will skip API check"
fi

info "Postgres pod: $PG_POD"
info "Core pod: ${CORE_POD:-none}"
echo ""

# --- 1. DB: pods table columns (migration 068) ---
section_db_columns() {
  echo "=== 1. DB schema: pods table (migration 068) ==="
  for col in pod_ip start_time restart_count owner_kind owner_name replica_set_name qos_class; do
    EXISTS=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c \
      "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='pods' AND column_name='$col');" 2>/dev/null | tr -d ' \r')
    if [ "$EXISTS" = "t" ]; then
      ok "pods.$col exists"
    else
      fail "pods.$col MISSING (migration 068 may not have run)"
    fi
  done
  echo ""
}

# --- 2. DB: sample rows ---
section_db_sample() {
  echo "=== 2. DB sample: pod detail columns (first 3 pods) ==="
  kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -c \
    "SELECT id, name, namespace, pod_ip, start_time, restart_count, owner_kind, owner_name, qos_class FROM pods WHERE deleted_at IS NULL ORDER BY id DESC LIMIT 3;" 2>/dev/null || true
  echo ""
  # Count how many have non-empty pod_ip
  WITH_IP=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A -c \
    "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL AND pod_ip IS NOT NULL AND pod_ip != '';" 2>/dev/null | tr -d ' \r' || echo "0")
  if [ -n "$WITH_IP" ] && [ "${WITH_IP:-0}" -gt 0 ]; then
    ok "Pods with pod_ip set: $WITH_IP"
  else
    warn "No pods with pod_ip set. Agent may not be sending pod detail (old agent or sync not run)."
  fi
  echo ""
}

# --- 3. Core API: GET /inventory/pods and GET /inventory/pods/:uid ---
section_api() {
  if [ -z "$CORE_POD" ]; then
    warn "Skipping API check (no Core pod)"
    return
  fi
  echo "=== 3. Core API: pod detail fields ==="
  TOKEN=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
    -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('token','') or '')" 2>/dev/null || true)
  if [ -z "$TOKEN" ]; then
    warn "Could not get JWT from Core"
    return
  fi
  ok "JWT obtained"

  # GET /pods?pageSize=1
  RESP=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/pods?pageSize=1" 2>/dev/null || true)
  if echo "$RESP" | grep -q '"pods"'; then
    FIRST_POD=$(echo "$RESP" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    pods = d.get('pods') or []
    if pods:
        p = pods[0]
        print('podIP:', p.get('podIP'))
        print('startTime:', p.get('startTime'))
        print('restartCount:', p.get('restartCount'))
        print('ownerKind:', p.get('ownerKind'))
        print('ownerName:', p.get('ownerName'))
        print('qosClass:', p.get('qosClass'))
    else:
        print('(no pods in response)')
except Exception as e:
    print('parse error:', e)
" 2>/dev/null || true)
    if [ -n "$FIRST_POD" ]; then
      echo "$FIRST_POD"
      if echo "$FIRST_POD" | grep -q 'podIP: None\|podIP: ""'; then
        warn "API returned pod but podIP is empty (DB has no data or Core model not selecting columns)"
      else
        ok "API returns pod detail fields"
      fi
    fi
  else
    warn "GET /pods response missing or error: $(echo "$RESP" | head -c 120)"
  fi
  echo ""
}

section_db_columns
section_db_sample
section_api

echo "=========================================="
echo "Done. If schema OK but data/API empty: rebuild and redeploy Core + Agent (migration 068 + agent sync payload)."
echo "=========================================="
