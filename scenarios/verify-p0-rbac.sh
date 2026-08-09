#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
S6="$ROOT_DIR/scenarios/s6-clusterrole-excessive-permission.yaml"
S7="$ROOT_DIR/scenarios/s7-wildcard-rbac-permission.yaml"
NS="fortuna-test"

command -v kubectl >/dev/null 2>&1 || {
  echo "kubectl is required"
  exit 1
}

for manifest in "$S6" "$S7"; do
  kubectl apply --dry-run=client -f "$manifest" >/dev/null
  echo "PASS: manifest validation: $(basename "$manifest")"
done

if [[ "${RUNTIME:-0}" != "1" ]]; then
  echo "Runtime checks skipped. Set RUNTIME=1 with a disposable test cluster to exercise RBAC effective permissions."
  exit 0
fi

cleanup() {
  kubectl delete -f "$S6" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete -f "$S7" --ignore-not-found >/dev/null 2>&1 || true
}
trap cleanup EXIT

kubectl apply -f "$S6" >/dev/null
kubectl apply -f "$S7" >/dev/null
kubectl wait --for=condition=Ready pod/cross-namespace-reader -n "$NS" --timeout=90s >/dev/null
kubectl wait --for=condition=Ready pod/wildcard-rbac -n "$NS" --timeout=90s >/dev/null

# S6: ClusterRoleBinding must grant the identity cluster-wide read access.
if kubectl auth can-i get secrets --all-namespaces --as="system:serviceaccount:${NS}:sa-cross-namespace-reader" | grep -qx 'yes'; then
  echo "PASS: S6 service account has cluster-wide Secret read access"
else
  echo "FAIL: S6 service account does not have expected cluster-wide Secret read access"
  exit 1
fi

# S7: wildcard Role must remain namespace-scoped.
if kubectl auth can-i get secrets -n "$NS" --as="system:serviceaccount:${NS}:sa-wildcard" | grep -qx 'yes'; then
  echo "PASS: S7 service account has expected namespace-scoped access"
else
  echo "FAIL: S7 service account lacks expected namespace-scoped access"
  exit 1
fi

if kubectl auth can-i get secrets -n default --as="system:serviceaccount:${NS}:sa-wildcard" | grep -qx 'yes'; then
  echo "FAIL: S7 wildcard Role unexpectedly grants access outside its namespace"
  exit 1
else
  echo "PASS: S7 wildcard Role remains namespace-scoped"
fi
