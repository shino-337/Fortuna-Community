#!/usr/bin/env bash
# Install Falco (Helm) with JSONL on host /var/log/falco/events.jsonl and
# enable fortuna-agent FalcoReader to ingest those events.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NS="${FORTUNA_NAMESPACE:-fortuna}"
RELEASE="${FALCO_RELEASE_NAME:-falco}"
AGENT_DS="${FORTUNA_AGENT_DAEMONSET:-fortuna-agent}"
ENABLE_AGENT_FALCO="${ENABLE_AGENT_FALCO:-true}"
FALCO_EVENTS_PATH="${FALCO_EVENTS_PATH:-/var/log/falco/events.jsonl}"
FALCO_EVENTS_POLL="${FALCO_EVENTS_POLL:-5s}"
WAIT_TIMEOUT="${FALCO_WAIT_TIMEOUT:-180s}"
TRUNCATE_FALCO_EVENTS="${TRUNCATE_FALCO_EVENTS:-true}"

if ! command -v helm >/dev/null 2>&1; then
  echo "[INFO] Helm not found — installing..."
  curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
  if ! command -v helm >/dev/null 2>&1; then
    echo "[ERR] Helm installation failed."
    exit 1
  fi
  echo "[OK] Helm installed: $(helm version --short)"
fi

helm repo add falcosecurity https://falcosecurity.github.io/charts 2>/dev/null || true
helm repo update falcosecurity

kubectl create namespace "$NS" 2>/dev/null || true

helm upgrade --install "$RELEASE" falcosecurity/falco \
  --namespace "$NS" \
  --values "$ROOT/deploy/falco/helm-values-fortuna.yaml" \
  --wait --timeout "$WAIT_TIMEOUT"

DISABLED_SELECTOR="$(kubectl get daemonset "$RELEASE" -n "$NS" -o jsonpath='{.spec.template.spec.nodeSelector.fortuna\.dev/disabled}' 2>/dev/null || true)"
if [ -n "$DISABLED_SELECTOR" ]; then
  echo "[WARN] Removing stale Falco nodeSelector fortuna.dev/disabled=$DISABLED_SELECTOR so DaemonSet can schedule"
  kubectl patch daemonset/"$RELEASE" -n "$NS" --type=json -p='[{"op":"remove","path":"/spec/template/spec/nodeSelector"}]' >/dev/null 2>&1 || \
    echo "[WARN] Could not remove stale Falco nodeSelector; inspect: kubectl describe ds $RELEASE -n $NS"
fi

echo ""
echo "[INFO] Waiting for Falco pods..."
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=falco \
  -n "$NS" \
  --timeout="$WAIT_TIMEOUT" 2>/dev/null || \
  echo "[WARN] Falco pods are not all Ready yet; continuing with agent configuration"

if [ "$TRUNCATE_FALCO_EVENTS" = "true" ]; then
  echo "[INFO] Truncating Falco JSONL backlog before enabling agent reader..."
  FALCO_PODS="$(kubectl get pods -n "$NS" -l app.kubernetes.io/name=falco -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)"
  if [ -n "$FALCO_PODS" ]; then
    while IFS= read -r pod; do
      [ -z "$pod" ] && continue
      kubectl exec -n "$NS" "$pod" -c falco -- sh -c ": > '$FALCO_EVENTS_PATH'" 2>/dev/null || \
        echo "[WARN] Could not truncate $FALCO_EVENTS_PATH from pod $pod"
    done <<< "$FALCO_PODS"
  else
    echo "[WARN] No Falco pods found for JSONL truncation"
  fi
fi

if [ "$ENABLE_AGENT_FALCO" = "true" ]; then
  if kubectl get daemonset "$AGENT_DS" -n "$NS" >/dev/null 2>&1; then
    echo "[INFO] Enabling Falco event ingest on $AGENT_DS..."
    kubectl set env daemonset/"$AGENT_DS" -n "$NS" \
      FALCO_EVENTS_ENABLED=true \
      FALCO_EVENTS_PATH="$FALCO_EVENTS_PATH" \
      FALCO_EVENTS_POLL="$FALCO_EVENTS_POLL"
    kubectl rollout restart daemonset/"$AGENT_DS" -n "$NS" >/dev/null 2>&1 || true
    kubectl rollout status daemonset/"$AGENT_DS" -n "$NS" --timeout=120s 2>/dev/null || \
      echo "[WARN] Agent rollout did not complete within 120s"
  else
    echo "[WARN] Agent DaemonSet not found in namespace $NS; Falco is installed but agent ingest was not enabled"
  fi
fi

echo ""
echo "Falco installed. Check pods: kubectl -n $NS get pods -l app.kubernetes.io/name=falco"
echo "Agent ingest: FALCO_EVENTS_ENABLED=true FALCO_EVENTS_PATH=$FALCO_EVENTS_PATH FALCO_EVENTS_POLL=$FALCO_EVENTS_POLL"
