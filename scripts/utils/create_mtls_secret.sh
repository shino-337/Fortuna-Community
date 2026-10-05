#!/bin/bash

# ============================================================================
# Create mTLS Secrets for Fortuna
# ============================================================================
# Generates mTLS certificates and creates Kubernetes secrets.
# By default, if all TLS secrets already exist in the namespace, exits without
# regenerating (avoids Agent disconnect on every deploy). Force new certs:
#   MTLS_REGEN=1 NAMESPACE=fortuna ./scripts/utils/create_mtls_secret.sh
#     New CA and certificates. Every Agent, including remote clusters, must get
#     the new client certificate and CA.
#   MTLS_RENEW=1 NAMESPACE=fortuna ./scripts/utils/create_mtls_secret.sh
#     New Core, webhook and Agent certificates from the existing CA in CERT_DIR
#     (used by rotate_mtls_secret.sh before the certificates expire).
# Certificates and the CA key are written to CERT_DIR (default: <repo>/.certs).
# Validity: MTLS_CA_DAYS (default 3650) for the CA, MTLS_CERT_DAYS (default 365)
# for the certificates it signs.
# ============================================================================

set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
NAMESPACE="${NAMESPACE:-fortuna}"

# Logging
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} ✅ $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} ❌ $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} ⚠️  $1"
}

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CERT_DIR="${CERT_DIR:-$PROJECT_ROOT/.certs}"

# Check prerequisites
if ! command -v openssl >/dev/null 2>&1; then
    log_error "openssl is not installed (required for certificate generation)"
    exit 1
fi

if ! command -v kubectl >/dev/null 2>&1; then
    log_error "kubectl is not installed"
    exit 1
fi

# Check namespace
if ! kubectl get namespace "$NAMESPACE" &>/dev/null 2>&1; then
    log_warning "Namespace '$NAMESPACE' does not exist. Creating..."
    kubectl create namespace "$NAMESPACE"
    log_success "Namespace '$NAMESPACE' created"
fi

# Create cert directory
mkdir -p "$CERT_DIR"

MTLS_CA_DAYS="${MTLS_CA_DAYS:-3650}"
MTLS_CERT_DAYS="${MTLS_CERT_DAYS:-365}"
RENEW=false
case "${MTLS_RENEW:-0}" in 1|true) RENEW=true ;; esac
REGEN=false
case "${MTLS_REGEN:-0}" in 1|true) REGEN=true ;; esac
if [ "$RENEW" = true ] && [ "$REGEN" = true ]; then
    log_error "Set MTLS_RENEW or MTLS_REGEN, not both."
    exit 1
fi
if [ "$RENEW" = true ] && { [ ! -f "$CERT_DIR/ca.crt" ] || [ ! -f "$CERT_DIR/ca.key" ]; }; then
    log_error "MTLS_RENEW=1 needs the existing CA ($CERT_DIR/ca.crt and ca.key). Without it, only MTLS_REGEN=1 (new CA) is possible."
    exit 1
fi

# Skip if cluster already has TLS material (avoids Agent disconnect on every deploy)
if [ "$REGEN" = false ] && [ "$RENEW" = false ]; then
    if kubectl get secret fortuna-core-tls -n "$NAMESPACE" &>/dev/null && \
       kubectl get secret fortuna-agent-tls -n "$NAMESPACE" &>/dev/null && \
       kubectl get secret fortuna-ca-cert -n "$NAMESPACE" &>/dev/null && \
       kubectl get secret fortuna-webhook-tls -n "$NAMESPACE" &>/dev/null; then
        log_success "mTLS secrets already present in namespace $NAMESPACE — skipping (set MTLS_REGEN=1 to regenerate)."
        exit 0
    fi
fi

if [ "$REGEN" = true ]; then
    log_info "MTLS_REGEN=1 — clearing local cert material before regeneration..."
    rm -f "$CERT_DIR"/*.crt "$CERT_DIR"/*.key "$CERT_DIR"/*.csr "$CERT_DIR"/*.srl "$CERT_DIR"/*.conf 2>/dev/null || true
fi
if [ "$RENEW" = true ]; then
    log_info "MTLS_RENEW=1 — issuing new certificates from the existing CA..."
    rm -f "$CERT_DIR"/server.* "$CERT_DIR"/client.* 2>/dev/null || true
fi

GENERATE=true
if [ -f "$CERT_DIR/ca.crt" ] && [ -f "$CERT_DIR/ca.key" ] && [ -f "$CERT_DIR/server.crt" ] && [ -f "$CERT_DIR/server.key" ] && \
   [ -f "$CERT_DIR/client.crt" ] && [ -f "$CERT_DIR/client.key" ]; then
    GENERATE=false
    log_info "Reusing existing certificate files in $CERT_DIR (set MTLS_RENEW=1 or MTLS_REGEN=1 to replace them)."
fi

# Generate certificates (first run, missing files, or MTLS_REGEN/MTLS_RENEW cleared them)
if [ "$GENERATE" = true ]; then
log_info "Generating mTLS certificates..."

if [ ! -f "$CERT_DIR/ca.crt" ] || [ ! -f "$CERT_DIR/ca.key" ]; then
    log_info "Generating CA certificate (valid ${MTLS_CA_DAYS} days)..."
    openssl genrsa -out "$CERT_DIR/ca.key" 4096
    openssl req -new -x509 -days "$MTLS_CA_DAYS" -key "$CERT_DIR/ca.key" -out "$CERT_DIR/ca.crt" \
        -subj "/CN=Fortuna CA/O=Fortuna/C=US" 2>/dev/null
    chmod 600 "$CERT_DIR/ca.key"
fi

# Generate server certificate with SANs
log_info "Generating server certificate with SANs..."
openssl genrsa -out "$CERT_DIR/server.key" 4096

# Create server certificate config with SANs
cat > "$CERT_DIR/server.conf" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = fortuna-core.${NAMESPACE}.svc.cluster.local
O = Fortuna
C = US

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = fortuna-core
DNS.2 = fortuna-core.${NAMESPACE}
DNS.3 = fortuna-core.${NAMESPACE}.svc.cluster.local
DNS.4 = *.${NAMESPACE}.svc.cluster.local
DNS.5 = fortuna-webhook.${NAMESPACE}.svc
DNS.6 = fortuna-webhook.${NAMESPACE}.svc.cluster.local
IP.1 = 127.0.0.1
EOF

openssl req -new -key "$CERT_DIR/server.key" -out "$CERT_DIR/server.csr" \
    -config "$CERT_DIR/server.conf" 2>/dev/null
openssl x509 -req -days "$MTLS_CERT_DAYS" -in "$CERT_DIR/server.csr" -CA "$CERT_DIR/ca.crt" \
    -CAkey "$CERT_DIR/ca.key" -CAcreateserial -out "$CERT_DIR/server.crt" \
    -extensions v3_req -extfile "$CERT_DIR/server.conf" 2>/dev/null

# Generate client certificate with SANs
log_info "Generating client certificate with SANs..."
openssl genrsa -out "$CERT_DIR/client.key" 4096

# Create client certificate config with SANs
cat > "$CERT_DIR/client.conf" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = fortuna-agent
O = Fortuna
C = US

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = clientAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = fortuna-agent
DNS.2 = *.fortuna-agent
EOF

openssl req -new -key "$CERT_DIR/client.key" -out "$CERT_DIR/client.csr" \
    -config "$CERT_DIR/client.conf" 2>/dev/null
openssl x509 -req -days "$MTLS_CERT_DAYS" -in "$CERT_DIR/client.csr" -CA "$CERT_DIR/ca.crt" \
    -CAkey "$CERT_DIR/ca.key" -CAcreateserial -out "$CERT_DIR/client.crt" \
    -extensions v3_req -extfile "$CERT_DIR/client.conf" 2>/dev/null

log_success "Certificates generated in $CERT_DIR"

fi

# Create Kubernetes secrets
log_info "Creating Kubernetes secrets in namespace '$NAMESPACE'..."

# CA Certificate secret
kubectl create secret generic fortuna-ca-cert \
    --from-file=ca.crt="$CERT_DIR/ca.crt" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
log_success "CA certificate secret created/updated"

# Core server TLS secret (tls type for proper key names)
kubectl create secret tls fortuna-core-tls \
    --cert="$CERT_DIR/server.crt" \
    --key="$CERT_DIR/server.key" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
log_success "Core server TLS secret created/updated"

# Agent client TLS secret (tls type for proper key names)
kubectl create secret tls fortuna-agent-tls \
    --cert="$CERT_DIR/client.crt" \
    --key="$CERT_DIR/client.key" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
log_success "Agent client TLS secret created/updated"

# Webhook TLS secret (reuse server cert for webhook)
kubectl create secret tls fortuna-webhook-tls \
    --cert="$CERT_DIR/server.crt" \
    --key="$CERT_DIR/server.key" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
log_success "Webhook TLS secret created/updated"

# Verify secrets
echo ""
log_info "Verifying secrets:"
kubectl get secrets -n "$NAMESPACE" | grep fortuna || log_warning "No fortuna secrets found"

echo ""
log_success "mTLS secrets created successfully!"
log_info "Secrets:"
echo "  - fortuna-ca-cert (CA certificate)"
echo "  - fortuna-core-tls (for Core gRPC server)"
echo "  - fortuna-agent-tls (for Agent gRPC client)"
echo "  - fortuna-webhook-tls (for Webhook server)"
echo ""
log_info "Certificates location: $CERT_DIR"
log_info "Expiry: CA $(openssl x509 -enddate -noout -in "$CERT_DIR/ca.crt" | cut -d= -f2); certificates $(openssl x509 -enddate -noout -in "$CERT_DIR/server.crt" | cut -d= -f2)"

