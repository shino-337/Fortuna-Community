#!/bin/bash

# ============================================================================
# Create mTLS Secrets for Fortuna
# ============================================================================
# Generates mTLS certificates and creates Kubernetes secrets
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
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CERT_DIR="$PROJECT_ROOT/.certs"

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

# Generate certificates
log_info "Generating mTLS certificates..."

if [ -f "$CERT_DIR/ca.crt" ] && [ -f "$CERT_DIR/server.crt" ] && [ -f "$CERT_DIR/client.crt" ]; then
    log_info "Certificates already exist, regenerating..."
    rm -f "$CERT_DIR"/*.crt "$CERT_DIR"/*.key "$CERT_DIR"/*.csr "$CERT_DIR"/*.srl 2>/dev/null || true
fi

# Generate CA
log_info "Generating CA certificate..."
openssl genrsa -out "$CERT_DIR/ca.key" 4096
openssl req -new -x509 -days 365 -key "$CERT_DIR/ca.key" -out "$CERT_DIR/ca.crt" \
    -subj "/CN=Fortuna CA/O=Fortuna/C=US" 2>/dev/null

# Generate server certificate
log_info "Generating server certificate..."
openssl genrsa -out "$CERT_DIR/server.key" 4096
openssl req -new -key "$CERT_DIR/server.key" -out "$CERT_DIR/server.csr" \
    -subj "/CN=fortuna-core.${NAMESPACE}.svc.cluster.local/O=Fortuna/C=US" 2>/dev/null
openssl x509 -req -days 365 -in "$CERT_DIR/server.csr" -CA "$CERT_DIR/ca.crt" \
    -CAkey "$CERT_DIR/ca.key" -CAcreateserial -out "$CERT_DIR/server.crt" 2>/dev/null

# Generate client certificate
log_info "Generating client certificate..."
openssl genrsa -out "$CERT_DIR/client.key" 4096
openssl req -new -key "$CERT_DIR/client.key" -out "$CERT_DIR/client.csr" \
    -subj "/CN=fortuna-agent/O=Fortuna/C=US" 2>/dev/null
openssl x509 -req -days 365 -in "$CERT_DIR/client.csr" -CA "$CERT_DIR/ca.crt" \
    -CAkey "$CERT_DIR/ca.key" -CAcreateserial -out "$CERT_DIR/client.crt" 2>/dev/null

log_success "Certificates generated in $CERT_DIR"

# Create Kubernetes secrets
log_info "Creating Kubernetes secrets in namespace '$NAMESPACE'..."

# Core server TLS secret
kubectl create secret generic fortuna-core-server-tls \
    --from-file=tls.crt="$CERT_DIR/server.crt" \
    --from-file=tls.key="$CERT_DIR/server.key" \
    --from-file=ca.crt="$CERT_DIR/ca.crt" \
    -n "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

log_success "Core server TLS secret created/updated"

# Agent client TLS secret
kubectl create secret generic fortuna-agent-client-tls \
    --from-file=tls.crt="$CERT_DIR/client.crt" \
    --from-file=tls.key="$CERT_DIR/client.key" \
    --from-file=ca.crt="$CERT_DIR/ca.crt" \
    -n "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

log_success "Agent client TLS secret created/updated"

# Verify secrets
echo ""
log_info "Verifying secrets:"
kubectl get secrets -n "$NAMESPACE" | grep fortuna || log_warning "No fortuna secrets found"

echo ""
log_success "mTLS secrets created successfully!"
log_info "Secrets:"
echo "  - fortuna-core-server-tls (for Core gRPC server)"
echo "  - fortuna-agent-client-tls (for Agent gRPC client)"
echo ""
log_info "Certificates location: $CERT_DIR"

