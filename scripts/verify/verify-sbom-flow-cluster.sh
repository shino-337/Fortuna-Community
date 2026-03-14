#!/usr/bin/env bash
# =============================================================================
# Verify SBOM flow on live cluster: DB (pods vs sboms), Agent logs, Core logs.
# Usage: ./scripts/verify/verify-sbom-flow-cluster.sh
# =============================================================================

set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
section() { echo -e "\n${BLUE}=== $1 ===${NC}"; }
ok() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

section "1. DB: pods vs sboms"
PG_POD=$(kubectl -n "$NAMESPACE" get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -z "$PG_POD" ]; then
  warn "Postgres pod not found in $NAMESPACE"
else
  kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -c \
    "SELECT 'pods' AS tbl, COUNT(*) FROM pods WHERE deleted_at IS NULL UNION ALL SELECT 'sboms', COUNT(*) FROM sboms WHERE deleted_at IS NULL;" 2>/dev/null || true
  echo ""
  echo "Pods that have SBOM:"
  kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -c \
    "SELECT pod_uid, pod_name, namespace FROM sboms WHERE deleted_at IS NULL;" 2>/dev/null || true
fi

section "2. Agent logs (SBOM / Send / fail / Queue) last 200 lines"
kubectl logs -n "$NAMESPACE" -l app=fortuna-agent --tail=200 2>/dev/null | grep -iE 'SBOM|SendSBOM|Queued|Failed to process|client not connected|Extracted.*packages' | tail -25

section "3. Core logs ([SBOM] Received / Created) last 100 lines"
kubectl logs -n "$NAMESPACE" deployment/fortuna-core --tail=100 2>/dev/null | grep '\[SBOM\]' || echo "(no [SBOM] lines)"

section "4. Summary"
PODS=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL;" 2>/dev/null || echo "?")
SBOMS=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c "SELECT COUNT(*) FROM sboms WHERE deleted_at IS NULL;" 2>/dev/null || echo "?")
echo "Pods in DB: $PODS | SBOMs in DB: $SBOMS"
if [ "${SBOMS:-0}" -lt "${PODS:-1}" ] 2>/dev/null; then
  warn "Dashboard will show 'no SBOM' for pods without a row in sboms. See docs/SBOM_FLOW_AND_CLUSTER_CHECK.md and agent logs (client not connected?)."
fi
