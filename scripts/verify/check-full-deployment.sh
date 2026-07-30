#!/bin/bash
# ============================================================================
# Full Deployment Check - Fortuna
# ============================================================================
# Verifies all components: Core, Dashboard, Agent, PostgreSQL, NATS, RBAC
# Usage: ./scripts/verify/check-full-deployment.sh
# ============================================================================

set -uo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

NAMESPACE="${NAMESPACE:-fortuna}"
ERRORS=0
WARNINGS=0

ok()  { echo -e "${GREEN}✅${NC} $*"; }
fail() { echo -e "${RED}❌${NC} $*"; ((ERRORS++)); }
warn() { echo -e "${YELLOW}⚠️${NC}  $*"; ((WARNINGS++)); }

echo "=========================================="
echo "Full Deployment Check - Fortuna"
echo "=========================================="
echo ""

# 1. Cluster & namespace
echo "=== 1. Cluster & Namespace ==="
if kubectl cluster-info &>/dev/null; then
  ok "Cluster accessible"
else
  fail "Cannot access cluster"
  exit 1
fi
if kubectl get namespace "$NAMESPACE" &>/dev/null; then
  ok "Namespace $NAMESPACE exists"
else
  fail "Namespace $NAMESPACE not found"
fi
echo ""

# 2. Workloads
echo "=== 2. Workloads (fortuna) ==="
for kind in deployment daemonset statefulset; do
  count=$(kubectl get "$kind" -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l)
  if [ "$count" -gt 0 ]; then
    ok "$kind: $count"
    kubectl get "$kind" -n "$NAMESPACE" --no-headers 2>/dev/null | while read -r line; do echo "   $line"; done
  else
    [ "$kind" = "deployment" ] && fail "No deployments in $NAMESPACE"
  fi
done
echo ""

# 3. Pods
echo "=== 3. Pods ==="
POD_LINES=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null || true)
TOTAL=$(echo "$POD_LINES" | wc -l)
NOT_READY=$(echo "$POD_LINES" | grep -v "Running" | grep -v "Completed" | wc -l)
if [ "${NOT_READY:-0}" -gt 0 ]; then
  warn "Some pods not Running: $NOT_READY / $TOTAL"
  echo "$POD_LINES" | grep -v Running || true
else
  ok "All $TOTAL pod(s) Running (or Completed)"
fi
kubectl get pods -n "$NAMESPACE" 2>/dev/null | sed 's/^/   /'
echo ""

# 4. Services & Endpoints
echo "=== 4. Services & Endpoints ==="
for svc in fortuna-core fortuna-dashboard postgres nats-client; do
  if kubectl get svc -n "$NAMESPACE" "$svc" &>/dev/null; then
    ep=$(kubectl get endpoints -n "$NAMESPACE" "$svc" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null | tr ' ' '\n' | grep -c . 2>/dev/null || echo "0")
    if [ "${ep:-0}" -gt 0 ]; then
      ok "Service $svc has endpoints"
    else
      warn "Service $svc has no endpoints"
    fi
  else
    [ "$svc" != "nats-client" ] && warn "Service $svc not found"
  fi
done
echo ""

# 5. Secrets & RBAC
echo "=== 5. Secrets & RBAC ==="
for secret in fortuna-secrets fortuna-core-tls fortuna-agent-tls; do
  kubectl get secret -n "$NAMESPACE" "$secret" &>/dev/null && ok "Secret $secret" || warn "Secret $secret missing"
done
for sa in fortuna-core fortuna-agent; do
  kubectl get sa -n "$NAMESPACE" "$sa" &>/dev/null && ok "ServiceAccount $sa" || warn "ServiceAccount $sa missing"
done
kubectl get clusterrole fortuna-core fortuna-agent &>/dev/null && ok "ClusterRoles (fortuna-core, fortuna-agent)" || { warn "ClusterRoles missing"; true; }
echo ""

# 6. Core health (port-forward + curl)
echo "=== 6. Core API Health ==="
PF_PID=""
if command -v curl &>/dev/null; then
  kubectl port-forward -n "$NAMESPACE" svc/fortuna-core 18080:8080 &>/dev/null & PF_PID=$!
  sleep 2
  HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:18080/health 2>/dev/null || echo "000")
  [ -n "$PF_PID" ] && kill $PF_PID 2>/dev/null; true
  if [ "$HEALTH" = "200" ]; then
    ok "Core /health returned 200"
  else
    fail "Core /health returned $HEALTH (expected 200)"
  fi
else
  warn "curl not found, skipping Core health check"
fi
echo ""

# 7. Dashboard (serve HTML via port-forward + curl)
echo "=== 7. Dashboard ==="
if ! kubectl get deployment -n "$NAMESPACE" fortuna-dashboard &>/dev/null; then
  fail "Deployment fortuna-dashboard not found"
else
  DASH_READY=$(kubectl get deployment -n "$NAMESPACE" fortuna-dashboard -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
  if [ "${DASH_READY:-0}" -ge 1 ]; then
    ok "Dashboard deployment: $DASH_READY replica(s) ready"
    if command -v curl &>/dev/null; then
      DASH_PF_PID=""
      kubectl port-forward -n "$NAMESPACE" svc/fortuna-dashboard 28081:80 &>/dev/null & DASH_PF_PID=$!
      sleep 3
      DASH_HTML=$(curl -s -m 10 http://127.0.0.1:28081/ 2>/dev/null | head -1 || echo "")
      [ -n "$DASH_PF_PID" ] && kill $DASH_PF_PID 2>/dev/null; true
      if echo "$DASH_HTML" | grep -qi "html"; then
        ok "Dashboard serves HTML at :80"
      else
        warn "Dashboard port 80 did not return HTML (curl: ${DASH_HTML:-empty})"
      fi
    fi
  else
    fail "Dashboard has no ready replicas"
  fi
fi
echo ""

# 8. Agent (DaemonSet)
echo "=== 8. Agent (DaemonSet) ==="
AGENT_DS=$(kubectl get daemonset -n "$NAMESPACE" -l app.kubernetes.io/name=fortuna -o name 2>/dev/null | head -1)
if [ -n "$AGENT_DS" ]; then
  DESIRED=$(kubectl get daemonset -n "$NAMESPACE" fortuna-agent -o jsonpath='{.status.desiredNumberScheduled}' 2>/dev/null || echo "0")
  READY=$(kubectl get daemonset -n "$NAMESPACE" fortuna-agent -o jsonpath='{.status.numberReady}' 2>/dev/null || echo "0")
  if [ "${DESIRED:-0}" -eq "${READY:-0}" ] && [ "${READY:-0}" -gt 0 ]; then
    ok "Agent DaemonSet: $READY/$DESIRED ready"
  else
    warn "Agent DaemonSet: $READY/$DESIRED ready"
  fi
else
  warn "fortuna-agent DaemonSet not found"
fi
echo ""

# Summary
echo "=========================================="
echo "Summary"
echo "=========================================="
echo -e "Errors:   ${RED}${ERRORS}${NC}"
echo -e "Warnings: ${YELLOW}${WARNINGS}${NC}"
echo ""

if [ "$ERRORS" -eq 0 ]; then
  echo -e "${GREEN}✅ Deployment check passed (no critical errors)${NC}"
  echo ""
  echo "Quick access:"
  echo "  Core API:     kubectl port-forward -n $NAMESPACE svc/fortuna-core 8080:8080  # then http://localhost:8080"
  echo "  Dashboard:    kubectl port-forward -n $NAMESPACE svc/fortuna-dashboard 8081:80  # then http://localhost:8081"
  echo "  Or NodePort:  kubectl get svc -n $NAMESPACE fortuna-dashboard  # port 30124 (or similar)"
  exit 0
else
  echo -e "${RED}❌ Deployment check found $ERRORS error(s)${NC}"
  exit 1
fi
