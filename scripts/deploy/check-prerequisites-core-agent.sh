#!/usr/bin/env bash
# ============================================================================
# Prerequisites check before deploying Core / Agent
# ============================================================================
# Verifies: (0) Pod network (Flannel via ensure-flannel.sh unless SKIP_FLANNEL_INSTALL=1);
#           (1) mTLS secrets exist; (1b) fortuna-secrets; (2) PostgreSQL reachable (service + endpoints);
#           (3) NATS reachable (service + endpoints).
# Exit 1 on first failure with clear instructions. Run after infra + mTLS are in place.
#
# Usage:
#   NAMESPACE=fortuna ./scripts/deploy/check-prerequisites-core-agent.sh
#   Called by deploy-fortuna-robust.sh before Step 8 (Deploy Core).
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"
NAMESPACE="${NAMESPACE:-fortuna}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

fail() {
  echo -e "${RED}❌ $*${NC}" >&2
  exit 1
}
ok() {
  echo -e "${GREEN}✅${NC} $*"
}

echo "=========================================="
echo "Prerequisites check (Core / Agent)"
echo "=========================================="
echo "Namespace: $NAMESPACE"
echo ""

# 0. Pod network: Flannel (or skip if another CNI — ensure-flannel exits 0)
echo ""
echo "=== Pod network (Flannel / CNI) ==="
if [ -x "$SCRIPTS/deploy/ensure-flannel.sh" ] && [ "${SKIP_FLANNEL_INSTALL:-0}" != "1" ]; then
  if bash "$SCRIPTS/deploy/ensure-flannel.sh"; then
    ok "Pod network (ensure-flannel) OK"
  else
    fail "Flannel / pod network not ready (subnet.env). Run: ./scripts/deploy/ensure-flannel.sh"
  fi
elif [ "${SKIP_FLANNEL_INSTALL:-0}" = "1" ]; then
  echo -e "${YELLOW}⚠️${NC} SKIP_FLANNEL_INSTALL=1 — skipping ensure-flannel (ensure CNI manually)"
else
  fail "ensure-flannel.sh not found at $SCRIPTS/deploy/ensure-flannel.sh"
fi


# 1. mTLS secrets (Core and Agent need these to mount certs)
for secret in fortuna-core-tls fortuna-agent-tls; do
  if kubectl get secret "$secret" -n "$NAMESPACE" &>/dev/null; then
    ok "Secret $secret exists"
  else
    fail "Secret $secret not found in namespace $NAMESPACE. Create mTLS first: NAMESPACE=$NAMESPACE $SCRIPTS/utils/create_mtls_secret.sh"
  fi
done

# 1b. Application secrets (Core envFrom: database-url, jwt-secret, admin-password)
if kubectl get secret fortuna-secrets -n "$NAMESPACE" &>/dev/null; then
  ok "Secret fortuna-secrets exists"
else
  fail "Secret fortuna-secrets not found in $NAMESPACE. Run: $SCRIPTS/utils/ensure-fortuna-secrets.sh $NAMESPACE"
fi

# 2. PostgreSQL: service exists and has endpoints (so Core can connect)
if ! kubectl get service postgres -n "$NAMESPACE" &>/dev/null; then
  fail "Service postgres not found in $NAMESPACE. Deploy infrastructure first: kubectl apply -f $PROJECT_ROOT/deploy/infrastructure/postgresql-with-age.yaml"
fi
EP_COUNT=$(kubectl get endpoints postgres -n "$NAMESPACE" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null | tr ' ' '\n' | grep -c . 2>/dev/null || echo "0")
if [ "${EP_COUNT:-0}" -lt 1 ]; then
  fail "Service postgres has no endpoints (PostgreSQL pod not ready). Wait for: kubectl wait --for=condition=ready pod -n $NAMESPACE -l app=postgres --timeout=300s"
fi
ok "PostgreSQL service has endpoints"

# Optional: one ready postgres pod
PG_READY=$(kubectl get pods -n "$NAMESPACE" -l app=postgres --no-headers 2>/dev/null | grep -c "Running" || echo "0")
if [ "${PG_READY:-0}" -lt 1 ]; then
  fail "No PostgreSQL pod in Running state. Check: kubectl get pods -n $NAMESPACE -l app=postgres"
fi
ok "PostgreSQL pod(s) Running"

# 3. NATS: service exists and has endpoints (Core uses NATS)
NATS_SVC="nats"
if kubectl get service nats-client -n "$NAMESPACE" &>/dev/null; then
  NATS_SVC="nats-client"
fi
if ! kubectl get service "$NATS_SVC" -n "$NAMESPACE" &>/dev/null; then
  fail "Service nats/nats-client not found in $NAMESPACE. Deploy: kubectl apply -f $PROJECT_ROOT/deploy/infrastructure/nats.yaml"
fi
NATS_EP=$(kubectl get endpoints "$NATS_SVC" -n "$NAMESPACE" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null | tr ' ' '\n' | grep -c . 2>/dev/null || echo "0")
if [ "${NATS_EP:-0}" -lt 1 ]; then
  fail "Service $NATS_SVC has no endpoints (NATS not ready). Wait for: kubectl wait --for=condition=ready pod -n $NAMESPACE -l app=nats --timeout=300s"
fi
ok "NATS service $NATS_SVC has endpoints"

echo ""
echo -e "${GREEN}✅ All prerequisites OK. Safe to deploy Core and Agent.${NC}"
exit 0
