#!/usr/bin/env bash
# Enable the optional webhook using the CA already installed for Core.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
if [[ "$NAMESPACE" != fortuna ]]; then
  echo "The bundled webhook manifests currently support namespace fortuna only." >&2
  exit 1
fi
for tool in kubectl openssl base64; do
  command -v "$tool" >/dev/null || { echo "$tool is required" >&2; exit 1; }
done
CERT_TMP="$(mktemp -d)"
trap 'rm -rf "$CERT_TMP"' EXIT
CA_BUNDLE="$(kubectl -n "$NAMESPACE" get secret fortuna-ca-cert -o 'jsonpath={.data.ca\.crt}')"
printf '%s' "$CA_BUNDLE" | base64 -d > "$CERT_TMP/ca.crt"
kubectl -n "$NAMESPACE" get secret fortuna-webhook-tls -o 'jsonpath={.data.tls\.crt}' | base64 -d > "$CERT_TMP/server.crt"
if ! openssl verify -CAfile "$CERT_TMP/ca.crt" -verify_hostname "fortuna-webhook.${NAMESPACE}.svc" "$CERT_TMP/server.crt"; then
  echo "Webhook certificate does not match its CA/service DNS or is expired. See docs/05-operations/WEBHOOK.md for certificate rotation before retrying." >&2
  exit 1
fi
kubectl apply -f "$ROOT_DIR/deploy/webhook-service.yaml"
kubectl patch --local -f "$ROOT_DIR/deploy/webhook-config.yaml" --type=json \
  -p "[{\"op\":\"add\",\"path\":\"/webhooks/0/clientConfig/caBundle\",\"value\":\"${CA_BUNDLE}\"}]" -o yaml | kubectl apply -f -
echo "Webhook configuration applied with the installed CA. Verify endpoints and an admission request before enabling enforcement; see docs/05-operations/WEBHOOK.md."
