#!/usr/bin/env bash
# Install Falco (Helm) into namespace fortuna with JSONL on host /var/log/falco/events.jsonl
# for fortuna-agent FalcoReader. After install, enable the agent:
#   kubectl -n fortuna set env ds/fortuna-agent FALCO_EVENTS_ENABLED=true
#   kubectl -n fortuna rollout status ds/fortuna-agent --timeout=120s
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NS="${FORTUNA_NAMESPACE:-fortuna}"
RELEASE="${FALCO_RELEASE_NAME:-falco}"

if ! command -v helm >/dev/null 2>&1; then
  echo "helm is required. Install: https://helm.sh/docs/intro/install/"
  exit 1
fi

helm repo add falcosecurity https://falcosecurity.github.io/charts 2>/dev/null || true
helm repo update falcosecurity

kubectl create namespace "$NS" 2>/dev/null || true

helm upgrade --install "$RELEASE" falcosecurity/falco \
  --namespace "$NS" \
  --values "$ROOT/deploy/falco/helm-values-fortuna.yaml"

echo ""
echo "Falco installed. Check pods: kubectl -n $NS get pods -l app.kubernetes.io/name=falco"
echo "Enable Fortuna agent ingest:"
echo "  kubectl -n $NS set env ds/fortuna-agent FALCO_EVENTS_ENABLED=true"
echo "  kubectl -n $NS rollout restart ds/fortuna-agent"
