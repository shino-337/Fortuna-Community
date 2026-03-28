#!/usr/bin/env bash
# R10: print hybrid admission gate env from fortuna-core and webhook objects (requires kubectl + fortuna namespace).
set -euo pipefail
NS="${FORTUNA_NAMESPACE:-fortuna}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl not found; skip cluster checks."
  exit 0
fi

echo "=== Core pod: ADMISSION_RISK_* (first ready pod) ==="
POD=$(kubectl -n "$NS" get pods -l app.kubernetes.io/component=core -o jsonpath='{.items[?(@.status.phase=="Running")].metadata.name}' 2>/dev/null | awk '{print $1}')
if [[ -z "${POD:-}" ]]; then
  echo "No Running fortuna-core pod in $NS (optional: apply deploy/fortuna-core-deployment.yaml)."
else
  kubectl -n "$NS" exec "$POD" -- env 2>/dev/null | grep -E '^ADMISSION_RISK_' || echo "(no ADMISSION_RISK_* env)"
fi

echo ""
echo "=== ValidatingWebhookConfiguration fortuna-policy-webhook ==="
kubectl get validatingwebhookconfiguration fortuna-policy-webhook -o yaml 2>/dev/null | head -n 40 || echo "Not installed (see deploy/webhook-config.yaml)."

echo ""
echo "=== Service fortuna-webhook (namespace $NS) ==="
kubectl -n "$NS" get svc fortuna-webhook -o wide 2>/dev/null || echo "Not found."
