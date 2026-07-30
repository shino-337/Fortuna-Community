#!/usr/bin/env bash
# ============================================================================
# DNS pre-flight: resolve Core dependencies inside the target namespace.
# Run before applying fortuna-core (or from deploy-fortuna-robust.sh Step 7f).
#
# Usage:
#   NAMESPACE=fortuna ./scripts/verify/check-dns-prereq.sh
# ============================================================================

set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

need_ns() {
  if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
    echo -e "${RED}Namespace $NAMESPACE does not exist.${NC}" >&2
    exit 1
  fi
}

nslookup_in_cluster() {
  local label="$1"
  local host="$2"
  local pod="dns-prereq-${label}-$(date +%s)-$$"
  echo "Checking nslookup $host ..."
  if kubectl run "$pod" --image=busybox:1.36 --rm --attach --restart=Never \
    --namespace="$NAMESPACE" --request-timeout=120s \
    --command -- nslookup "$host" &>/dev/null; then
    echo -e "${GREEN}OK${NC} $host"
    return 0
  fi
  echo -e "${RED}FAIL${NC} nslookup $host (pod in namespace $NAMESPACE)" >&2
  return 1
}

echo "=========================================="
echo "DNS pre-flight (namespace=$NAMESPACE)"
echo "=========================================="
need_ns

ok=true
nslookup_in_cluster pg "postgres.${NAMESPACE}.svc.cluster.local" || ok=false
# NATS: manifest uses nats-client; some clusters only expose "nats"
if kubectl get svc nats-client -n "$NAMESPACE" &>/dev/null; then
  nslookup_in_cluster nats "nats-client.${NAMESPACE}.svc.cluster.local" || ok=false
elif kubectl get svc nats -n "$NAMESPACE" &>/dev/null; then
  nslookup_in_cluster nats "nats.${NAMESPACE}.svc.cluster.local" || ok=false
else
  echo -e "${YELLOW}WARN${NC} No Service nats-client or nats in $NAMESPACE — skipping NATS DNS check"
fi
nslookup_in_cluster core "fortuna-core.${NAMESPACE}.svc.cluster.local" || true

if [ "$ok" != true ]; then
  echo -e "${RED}DNS pre-flight failed.${NC} Fix CoreDNS / Flannel (e.g. ./scripts/deploy/ensure-flannel.sh) then retry." >&2
  exit 1
fi
echo -e "${GREEN}DNS pre-flight passed.${NC}"
exit 0
