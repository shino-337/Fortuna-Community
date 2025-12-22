#!/bin/bash
# generate-webhook-certs.sh
# Generate self-signed certificates for admission webhook

set -e

NAMESPACE="ksam"
SERVICE="ksam-webhook"

echo "=========================================="
echo "Generating certificates for admission webhook"
echo "Service: ${SERVICE}.${NAMESPACE}.svc"
echo "=========================================="

# Create temporary directory
TMPDIR=$(mktemp -d)
cd "$TMPDIR"

# Generate CA
echo "Generating CA..."
openssl genrsa -out ca.key 2048
openssl req -new -x509 -days 365 -key ca.key -subj "/CN=${SERVICE}.${NAMESPACE}.svc" -out ca.crt

# Generate server cert
echo "Generating server certificate..."
openssl genrsa -out server.key 2048
openssl req -new -key server.key -subj "/CN=${SERVICE}.${NAMESPACE}.svc" -out server.csr

# Create extension file for SAN
cat > extfile.cnf <<EOF
subjectAltName = DNS:${SERVICE},DNS:${SERVICE}.${NAMESPACE},DNS:${SERVICE}.${NAMESPACE}.svc,DNS:${SERVICE}.${NAMESPACE}.svc.cluster.local
EOF

# Sign server cert with CA
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -extfile extfile.cnf

# Create Kubernetes secret
echo "Creating Kubernetes secret..."
kubectl create secret tls ksam-webhook-tls \
  --cert=server.crt \
  --key=server.key \
  -n ${NAMESPACE} \
  --dry-run=client -o yaml > /tmp/ksam-webhook-tls-secret.yaml

echo ""
echo "=========================================="
echo "✅ Certificates generated successfully!"
echo "=========================================="
echo ""
echo "Files created:"
echo "  - ca.crt (CA certificate)"
echo "  - ca.key (CA private key)"
echo "  - server.crt (Server certificate)"
echo "  - server.key (Server private key)"
echo ""
echo "Kubernetes secret YAML:"
echo "  /tmp/ksam-webhook-tls-secret.yaml"
echo ""
echo "CA Bundle for ValidatingWebhookConfiguration:"
echo "=========================================="
cat ca.crt | base64 | tr -d '\n'
echo ""
echo "=========================================="
echo ""
echo "To apply the secret:"
echo "  kubectl apply -f /tmp/ksam-webhook-tls-secret.yaml"
echo ""
echo "To copy certificates to local directory:"
echo "  cp ca.crt server.crt server.key /path/to/destination/"
echo ""

# Cleanup
cd -
rm -rf "$TMPDIR"

