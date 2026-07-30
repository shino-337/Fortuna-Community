#!/usr/bin/env bash
# ============================================================================
# Verify Agent–Core connectivity preconditions
# ============================================================================
# Checks: Core pod Ready, Service endpoints, optional DNS from agent pod.
# Usage: ./scripts/verify/verify-agent-core-connectivity.sh
# ============================================================================

set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "Agent – Core connectivity check"
echo "=========================================="
echo ""

# 1. Core deployment and pod
echo -e "${BLUE}[1] Core deployment & pod${NC}"
if ! kubectl get deployment -n "$NAMESPACE" fortuna-core -o name &>/dev/null; then
  echo -e "  ${RED}Deployment fortuna-core not found in namespace $NAMESPACE${NC}"
  exit 1
fi
kubectl get deployment -n "$NAMESPACE" fortuna-core
CORE_READY=$(kubectl get deployment -n "$NAMESPACE" fortuna-core -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
CORE_DESIRED=$(kubectl get deployment -n "$NAMESPACE" fortuna-core -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "1")
if [ "${CORE_READY:-0}" -eq 0 ] || [ "${CORE_READY:-0}" != "${CORE_DESIRED:-1}" ]; then
  echo -e "  ${YELLOW}Core pod not Ready (readyReplicas=$CORE_READY, desired=$CORE_DESIRED). Agents will see connection refused until Core is Ready.${NC}"
else
  echo -e "  ${GREEN}Core deployment Ready${NC}"
fi
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o wide
echo ""

# 2. Service and Endpoints
echo -e "${BLUE}[2] Service fortuna-core & Endpoints (must have addresses for Agents to connect)${NC}"
if ! kubectl get svc -n "$NAMESPACE" fortuna-core -o name &>/dev/null; then
  echo -e "  ${RED}Service fortuna-core not found${NC}"
  exit 1
fi
kubectl get svc -n "$NAMESPACE" fortuna-core
EP_COUNT=$(kubectl get endpoints -n "$NAMESPACE" fortuna-core -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null | wc -w)
if [ "${EP_COUNT:-0}" -eq 0 ]; then
  echo -e "  ${YELLOW}No endpoints (Core pod not Ready or selector mismatch). Agent connections will fail.${NC}"
else
  echo -e "  ${GREEN}Endpoints: $EP_COUNT address(es)${NC}"
fi
kubectl get endpoints -n "$NAMESPACE" fortuna-core
echo ""

# 3. DNS resolution (fortuna-core in cluster)
echo -e "${BLUE}[3] DNS: fortuna-core.$NAMESPACE.svc.cluster.local${NC}"
CORE_FQDN="fortuna-core.${NAMESPACE}.svc.cluster.local"
# Agent image (debian-slim) often has no nslookup; use a temporary busybox pod for reliable DNS check
DNS_POD="dns-check-$(date +%s)"
if kubectl run "$DNS_POD" --image=busybox:1.36 --restart=Never -n "$NAMESPACE" -- nslookup "$CORE_FQDN" &>/dev/null; then
  echo "  Waiting for DNS check pod to complete (max 15s)..."
  for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
    PHASE=$(kubectl get pod -n "$NAMESPACE" "$DNS_POD" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    [ "$PHASE" = "Succeeded" ] || [ "$PHASE" = "Running" ] && break
    [ "$PHASE" = "Failed" ] && break
    sleep 1
  done
  DNS_LOGS=$(kubectl logs -n "$NAMESPACE" "$DNS_POD" 2>/dev/null || true)
  if echo "$DNS_LOGS" | grep -q "Address"; then
    echo -e "  ${GREEN}DNS OK (resolves from cluster)${NC}"
  elif echo "$DNS_LOGS" | grep -qi "can't find\|NXDOMAIN"; then
    echo -e "  ${YELLOW}DNS lookup failed (can't find $CORE_FQDN)${NC}"
  else
    echo -e "  ${YELLOW}DNS check inconclusive (pod may not have run)${NC}"
  fi
  kubectl delete pod -n "$NAMESPACE" "$DNS_POD" --ignore-not-found --wait=false &>/dev/null || true
else
  # Fallback: try from agent pod if it has nslookup/getent
  AGENT_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  if [ -n "$AGENT_POD" ]; then
    if kubectl exec -n "$NAMESPACE" "$AGENT_POD" -- nslookup "$CORE_FQDN" &>/dev/null; then
      echo -e "  ${GREEN}DNS OK (from agent pod)${NC}"
    elif kubectl exec -n "$NAMESPACE" "$AGENT_POD" -- getent hosts "$CORE_FQDN" &>/dev/null; then
      echo -e "  ${GREEN}DNS OK (getent from agent pod)${NC}"
    else
      echo -e "  ${YELLOW}DNS check skipped (agent image has no nslookup/getent; or resolution failed)${NC}"
    fi
  else
    echo "  No Running agent pod; skip DNS check."
  fi
fi
echo ""

echo "=========================================="
echo "If Core is Ready and Endpoints exist, Agent should connect (or retry until then)."
echo "If not, check Core logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=core --tail=100"
echo "=========================================="
