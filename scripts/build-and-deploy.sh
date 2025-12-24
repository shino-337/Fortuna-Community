#!/bin/bash
# Build and Deploy Fortuna on Fresh Environment
# Usage: ./scripts/build-and-deploy.sh [--skip-build] [--skip-infra] [--skip-deploy]

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Parse arguments
SKIP_BUILD=false
SKIP_INFRA=false
SKIP_DEPLOY=false

for arg in "$@"; do
    case $arg in
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --skip-infra)
            SKIP_INFRA=true
            shift
            ;;
        --skip-deploy)
            SKIP_DEPLOY=true
            shift
            ;;
        *)
            ;;
    esac
done

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}  Fortuna Build & Deploy Script${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Check prerequisites
echo -e "${CYAN}[1/6] Checking prerequisites...${NC}"

# Check Go
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed${NC}"
    echo "Install Go: https://go.dev/dl/"
    exit 1
fi
GO_VERSION=$(go version | awk '{print $3}')
echo "  ✓ Go: $GO_VERSION"

# Check kubectl
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}Error: kubectl is not installed${NC}"
    exit 1
fi
KUBECTL_VERSION=$(kubectl version --client --short 2>/dev/null | awk '{print $3}')
echo "  ✓ kubectl: $KUBECTL_VERSION"

# Check Kubernetes cluster
if ! kubectl cluster-info &>/dev/null; then
    echo -e "${RED}Error: Kubernetes cluster is not accessible${NC}"
    echo "Make sure kubectl is configured correctly"
    exit 1
fi
echo "  ✓ Kubernetes cluster: accessible"

# Check namespace
if ! kubectl get namespace fortuna &>/dev/null; then
    echo -e "${YELLOW}  Creating namespace 'fortuna'...${NC}"
    kubectl create namespace fortuna
fi
echo "  ✓ Namespace 'fortuna': exists"

echo ""

# Build components
if [ "$SKIP_BUILD" = false ]; then
    echo -e "${CYAN}[2/6] Building components...${NC}"
    
    # Create bin directory
    mkdir -p "$PROJECT_ROOT/bin"
    
    # Build Core
    echo "  Building Core..."
    cd "$PROJECT_ROOT/core"
    if [ ! -f go.mod ]; then
        echo -e "${RED}Error: go.mod not found in core/${NC}"
        exit 1
    fi
    
    go mod download
    go build -o "$PROJECT_ROOT/bin/fortuna-core" \
        -ldflags="-w -s -X main.BuildVersion=v1.0.0 -X main.BuildCommit=$(git rev-parse --short HEAD 2>/dev/null || echo 'dev')" \
        ./cmd/main.go
    
    if [ -f "$PROJECT_ROOT/bin/fortuna-core" ]; then
        SIZE=$(du -h "$PROJECT_ROOT/bin/fortuna-core" | cut -f1)
        echo -e "${GREEN}  ✓ Core built: $SIZE${NC}"
    else
        echo -e "${RED}  ✗ Core build failed${NC}"
        exit 1
    fi
    
    # Build Agent
    echo "  Building Agent..."
    cd "$PROJECT_ROOT/agent"
    if [ ! -f go.mod ]; then
        echo -e "${RED}Error: go.mod not found in agent/${NC}"
        exit 1
    fi
    
    go mod download
    go build -o "$PROJECT_ROOT/bin/fortuna-agent" \
        -ldflags="-w -s -X main.BuildVersion=v1.0.0 -X main.BuildCommit=$(git rev-parse --short HEAD 2>/dev/null || echo 'dev')" \
        ./cmd/main.go
    
    if [ -f "$PROJECT_ROOT/bin/fortuna-agent" ]; then
        SIZE=$(du -h "$PROJECT_ROOT/bin/fortuna-agent" | cut -f1)
        echo -e "${GREEN}  ✓ Agent built: $SIZE${NC}"
    else
        echo -e "${RED}  ✗ Agent build failed${NC}"
        exit 1
    fi
    
    echo ""
else
    echo -e "${YELLOW}[2/6] Skipping build (--skip-build)${NC}"
    echo ""
fi

# Deploy infrastructure
if [ "$SKIP_INFRA" = false ]; then
    echo -e "${CYAN}[3/6] Deploying infrastructure...${NC}"
    
    # Deploy PostgreSQL
    if [ -f "$PROJECT_ROOT/deploy/infrastructure/postgresql-with-age.yaml" ]; then
        echo "  Deploying PostgreSQL with Apache AGE..."
        kubectl apply -f "$PROJECT_ROOT/deploy/infrastructure/postgresql-with-age.yaml"
        
        echo "  Waiting for PostgreSQL to be ready..."
        if kubectl wait --for=condition=ready pod -l app=postgres -n fortuna --timeout=300s 2>/dev/null; then
            echo -e "${GREEN}  ✓ PostgreSQL: Ready${NC}"
        else
            echo -e "${YELLOW}  ⚠ PostgreSQL: Not ready yet (may take a few minutes)${NC}"
        fi
    else
        echo -e "${YELLOW}  ⚠ PostgreSQL deployment file not found${NC}"
    fi
    
    # Deploy NATS
    if [ -f "$PROJECT_ROOT/deploy/infrastructure/nats.yaml" ]; then
        echo "  Deploying NATS JetStream..."
        kubectl apply -f "$PROJECT_ROOT/deploy/infrastructure/nats.yaml"
        
        echo "  Waiting for NATS to be ready..."
        if kubectl wait --for=condition=ready pod -l app=nats -n fortuna --timeout=300s 2>/dev/null; then
            echo -e "${GREEN}  ✓ NATS: Ready${NC}"
        else
            echo -e "${YELLOW}  ⚠ NATS: Not ready yet (may take a few minutes)${NC}"
        fi
    else
        echo -e "${YELLOW}  ⚠ NATS deployment file not found${NC}"
    fi
    
    echo ""
else
    echo -e "${YELLOW}[3/6] Skipping infrastructure (--skip-infra)${NC}"
    echo ""
fi

# Generate certificates (if needed)
echo -e "${CYAN}[4/6] Setting up certificates...${NC}"

CERT_DIR="$PROJECT_ROOT/.certs"
mkdir -p "$CERT_DIR"

# Check if certificates exist
if [ ! -f "$CERT_DIR/ca.crt" ] || [ ! -f "$CERT_DIR/server.crt" ] || [ ! -f "$CERT_DIR/client.crt" ]; then
    echo "  Generating certificates..."
    
    # Generate CA
    openssl genrsa -out "$CERT_DIR/ca.key" 4096 2>/dev/null
    openssl req -new -x509 -days 365 -key "$CERT_DIR/ca.key" -out "$CERT_DIR/ca.crt" \
        -subj "/CN=Fortuna CA" 2>/dev/null
    
    # Generate server certificate
    openssl genrsa -out "$CERT_DIR/server.key" 4096 2>/dev/null
    openssl req -new -key "$CERT_DIR/server.key" -out "$CERT_DIR/server.csr" \
        -subj "/CN=fortuna-core.fortuna.svc.cluster.local" 2>/dev/null
    openssl x509 -req -days 365 -in "$CERT_DIR/server.csr" -CA "$CERT_DIR/ca.crt" \
        -CAkey "$CERT_DIR/ca.key" -CAcreateserial -out "$CERT_DIR/server.crt" 2>/dev/null
    
    # Generate client certificate
    openssl genrsa -out "$CERT_DIR/client.key" 4096 2>/dev/null
    openssl req -new -key "$CERT_DIR/client.key" -out "$CERT_DIR/client.csr" \
        -subj "/CN=fortuna-agent" 2>/dev/null
    openssl x509 -req -days 365 -in "$CERT_DIR/client.csr" -CA "$CERT_DIR/ca.crt" \
        -CAkey "$CERT_DIR/ca.key" -CAcreateserial -out "$CERT_DIR/client.crt" 2>/dev/null
    
    echo -e "${GREEN}  ✓ Certificates generated${NC}"
else
    echo -e "${GREEN}  ✓ Certificates already exist${NC}"
fi

# Create Kubernetes secrets
echo "  Creating Kubernetes secrets..."
kubectl create secret generic fortuna-core-server-tls \
    --from-file=tls.crt="$CERT_DIR/server.crt" \
    --from-file=tls.key="$CERT_DIR/server.key" \
    --from-file=ca.crt="$CERT_DIR/ca.crt" \
    -n fortuna --dry-run=client -o yaml | kubectl apply -f - 2>/dev/null || true

kubectl create secret generic fortuna-agent-client-tls \
    --from-file=tls.crt="$CERT_DIR/client.crt" \
    --from-file=tls.key="$CERT_DIR/client.key" \
    --from-file=ca.crt="$CERT_DIR/ca.crt" \
    -n fortuna --dry-run=client -o yaml | kubectl apply -f - 2>/dev/null || true

echo -e "${GREEN}  ✓ Secrets created${NC}"
echo ""

# Deploy Fortuna components
if [ "$SKIP_DEPLOY" = false ]; then
    echo -e "${CYAN}[5/6] Deploying Fortuna components...${NC}"
    
    # Deploy RBAC
    if [ -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml" ]; then
        echo "  Deploying RBAC..."
        kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml"
        echo -e "${GREEN}  ✓ RBAC deployed${NC}"
    else
        echo -e "${YELLOW}  ⚠ RBAC file not found${NC}"
    fi
    
    # Deploy Core
    if [ -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" ]; then
        echo "  Deploying Core..."
        kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml"
        
        echo "  Waiting for Core to be ready..."
        if kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=core -n fortuna --timeout=300s 2>/dev/null; then
            echo -e "${GREEN}  ✓ Core: Ready${NC}"
        else
            echo -e "${YELLOW}  ⚠ Core: Not ready yet (check logs)${NC}"
        fi
    else
        echo -e "${YELLOW}  ⚠ Core deployment file not found${NC}"
    fi
    
    # Deploy Agent
    if [ -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" ]; then
        echo "  Deploying Agent..."
        kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml"
        
        echo "  Waiting for Agent to be ready..."
        sleep 5
        AGENT_PODS=$(kubectl get pods -n fortuna -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | wc -l)
        if [ "$AGENT_PODS" -gt 0 ]; then
            echo -e "${GREEN}  ✓ Agent: $AGENT_PODS pod(s) deployed${NC}"
        else
            echo -e "${YELLOW}  ⚠ Agent: No pods found${NC}"
        fi
    else
        echo -e "${YELLOW}  ⚠ Agent deployment file not found${NC}"
    fi
    
    echo ""
else
    echo -e "${YELLOW}[5/6] Skipping deployment (--skip-deploy)${NC}"
    echo ""
fi

# Verification
echo -e "${CYAN}[6/6] Verification...${NC}"
echo ""

# Check pods
echo "Pods status:"
kubectl get pods -n fortuna

echo ""
echo "Services:"
kubectl get svc -n fortuna

echo ""
echo -e "${CYAN}========================================${NC}"
echo -e "${GREEN}  Deployment Complete!${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""

# Show next steps
echo -e "${CYAN}Next steps:${NC}"
echo "  1. Check logs:"
echo "     kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50"
echo "     kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=50"
echo ""
echo "  2. Test API:"
echo "     kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &"
echo "     curl http://localhost:8080/health"
echo ""
echo "  3. Check database:"
echo "     POSTGRES_POD=\$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')"
echo "     kubectl exec -n fortuna \$POSTGRES_POD -- psql -U postgres -d fortuna -c '\\dt'"
echo ""

