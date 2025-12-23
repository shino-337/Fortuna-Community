#!/bin/bash
# Generate self-signed certificates for mTLS between Agent and Core
# For development/testing purposes only

set -e

CERT_DIR="${1:-./certs}"
CA_CERT="${CERT_DIR}/ca.crt"
CA_KEY="${CERT_DIR}/ca.key"
CORE_CERT="${CERT_DIR}/core.crt"
CORE_KEY="${CERT_DIR}/core.key"
CORE_CSR="${CERT_DIR}/core.csr"
AGENT_CERT="${CERT_DIR}/agent.crt"
AGENT_KEY="${CERT_DIR}/agent.key"
AGENT_CSR="${CERT_DIR}/agent.csr"

# Create cert directory
mkdir -p "${CERT_DIR}"

echo "=== Generating CA Certificate ==="
openssl genrsa -out "${CA_KEY}" 4096
openssl req -new -x509 -days 3650 -key "${CA_KEY}" -out "${CA_CERT}" \
    -subj "/C=US/ST=CA/L=San Francisco/O=KSAM/CN=KSAM CA"

echo ""
echo "=== Generating Core Server Certificate ==="
openssl genrsa -out "${CORE_KEY}" 2048
openssl req -new -key "${CORE_KEY}" -out "${CORE_CSR}" \
    -subj "/C=US/ST=CA/L=San Francisco/O=KSAM/CN=ksam-core.ksam.svc.cluster.local"

# Create extensions file for server cert
cat > "${CERT_DIR}/core-extensions.conf" <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = ksam-core
DNS.2 = ksam-core.ksam
DNS.3 = ksam-core.ksam.svc
DNS.4 = ksam-core.ksam.svc.cluster.local
DNS.5 = localhost
IP.1 = 127.0.0.1
EOF

openssl x509 -req -days 365 -in "${CORE_CSR}" -CA "${CA_CERT}" -CAkey "${CA_KEY}" \
    -CAcreateserial -out "${CORE_CERT}" \
    -extfile "${CERT_DIR}/core-extensions.conf"

echo ""
echo "=== Generating Agent Client Certificate ==="
openssl genrsa -out "${AGENT_KEY}" 2048
openssl req -new -key "${AGENT_KEY}" -out "${AGENT_CSR}" \
    -subj "/C=US/ST=CA/L=San Francisco/O=KSAM/CN=ksam-agent"

# Create extensions file for client cert
cat > "${CERT_DIR}/agent-extensions.conf" <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
EOF

openssl x509 -req -days 365 -in "${AGENT_CSR}" -CA "${CA_CERT}" -CAkey "${CA_KEY}" \
    -CAcreateserial -out "${AGENT_CERT}" \
    -extfile "${CERT_DIR}/agent-extensions.conf"

# Clean up CSR and extension files
rm -f "${CORE_CSR}" "${AGENT_CSR}" "${CERT_DIR}/core-extensions.conf" "${CERT_DIR}/agent-extensions.conf"

echo ""
echo "=== Certificates Generated Successfully ==="
echo "CA Certificate:     ${CA_CERT}"
echo "Core Certificate:   ${CORE_CERT}"
echo "Core Key:           ${CORE_KEY}"
echo "Agent Certificate:  ${AGENT_CERT}"
echo "Agent Key:          ${AGENT_KEY}"
echo ""
echo "=== Creating Kubernetes Secrets ==="

# Create namespace if it doesn't exist
kubectl create namespace ksam --dry-run=client -o yaml | kubectl apply -f -

# Create Core TLS secret
kubectl create secret tls ksam-core-tls \
    --cert="${CORE_CERT}" \
    --key="${CORE_KEY}" \
    --namespace=ksam \
    --dry-run=client -o yaml | kubectl apply -f -

# Create Agent TLS secret
kubectl create secret tls ksam-agent-tls \
    --cert="${AGENT_CERT}" \
    --key="${AGENT_KEY}" \
    --namespace=ksam \
    --dry-run=client -o yaml | kubectl apply -f -

# Create CA secret (for client verification)
kubectl create secret generic ksam-ca-cert \
    --from-file=ca.crt="${CA_CERT}" \
    --namespace=ksam \
    --dry-run=client -o yaml | kubectl apply -f -

echo ""
echo "✅ Certificates and Kubernetes secrets created successfully!"
echo ""
echo "Next steps:"
echo "1. Update Core deployment to mount ksam-core-tls and ksam-ca-cert"
echo "2. Update Agent deployment to mount ksam-agent-tls and ksam-ca-cert"
echo "3. Update Core and Agent configs to use TLS paths"

