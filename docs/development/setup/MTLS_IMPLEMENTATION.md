# mTLS Implementation for Agent-Core Communication

## Overview

Mutual TLS (mTLS) has been implemented to secure gRPC communication between the Agent and Core components. This ensures that both parties authenticate each other using X.509 certificates.

## Implementation Details

### 1. Certificate Generation

Certificates are generated using a script located at `scripts/generate_certs.sh`:

- **CA Certificate**: Root certificate authority (valid for 10 years)
- **Core Server Certificate**: Server certificate for Core gRPC server
  - CN: `ksam-core.ksam.svc.cluster.local`
  - Valid for 365 days
  - Includes SANs for service DNS names
- **Agent Client Certificate**: Client certificate for Agent gRPC client
  - CN: `ksam-agent`
  - Valid for 365 days

### 2. Core gRPC Server (mTLS)

**File**: `core/internal/grpc/server.go`

- Loads CA certificate for client verification
- Loads server certificate and key
- Configures TLS with `RequireAndVerifyClientCert` (mutual authentication)
- Uses TLS 1.3 minimum version
- Creates gRPC server with TLS credentials

**Configuration**:
- `TLS_ENABLED=true` (environment variable)
- `TLS_CA_CERT_PATH=/etc/ksam/ca-cert/ca.crt`
- `TLS_CERT_PATH=/etc/ksam/certs/tls.crt`
- `TLS_KEY_PATH=/etc/ksam/certs/tls.key`

### 3. Agent gRPC Client (mTLS)

**File**: `agent/internal/client/grpc_client_new.go`

- Loads CA certificate for server verification
- Loads client certificate and key
- Configures TLS with client certificate
- Uses TLS 1.3 minimum version
- Creates gRPC client connection with TLS credentials

**Configuration**:
- `TLS_ENABLED=true` (environment variable)
- `TLS_CA_CERT_PATH=/etc/ksam/ca-cert/ca.crt`
- `TLS_CERT_PATH=/etc/ksam/certs/tls.crt`
- `TLS_KEY_PATH=/etc/ksam/certs/tls.key`

### 4. Kubernetes Deployment

**Core Deployment** (`deploy/core-deployment.yaml`):
- Mounts `ksam-core-tls` secret to `/etc/ksam/certs`
- Mounts `ksam-ca-cert` secret to `/etc/ksam/ca-cert`
- Sets `TLS_ENABLED=true`

**Agent DaemonSet** (`deploy/agent-daemonset.yaml`):
- Mounts `ksam-agent-tls` secret to `/etc/ksam/certs`
- Mounts `ksam-ca-cert` secret to `/etc/ksam/ca-cert`
- Sets `TLS_ENABLED=true`

### 5. Kubernetes Secrets

Secrets are created by the certificate generation script:

- `ksam-core-tls`: Contains Core server certificate and key
- `ksam-agent-tls`: Contains Agent client certificate and key
- `ksam-ca-cert`: Contains CA certificate

## Usage

### Generate Certificates

```bash
./scripts/generate_certs.sh ./certs
```

This will:
1. Generate CA, Core, and Agent certificates
2. Create Kubernetes secrets in the `ksam` namespace
3. Store certificates in the specified directory

### Deploy with mTLS

1. Generate certificates (if not already done)
2. Build Docker images:
   ```bash
   eval $(minikube docker-env)
   docker build -t ksam-core:latest ./core
   docker build -t ksam-agent:latest ./agent
   ```
3. Deploy services:
   ```bash
   kubectl apply -f deploy/core-deployment.yaml
   kubectl apply -f deploy/agent-daemonset.yaml
   ```

### Verify mTLS

Check Agent logs:
```bash
kubectl logs -n ksam -l app=ksam-agent | grep -i "mTLS"
```

Expected output: `gRPC client configured with mTLS`

Check Core logs:
```bash
kubectl logs -n ksam -l app=ksam-core | grep -i "mTLS\|gRPC server"
```

Expected output: `gRPC server configured with mTLS`

## Security Features

1. **Mutual Authentication**: Both client and server verify each other's certificates
2. **TLS 1.3**: Uses the latest TLS version for enhanced security
3. **Certificate Validation**: Full certificate chain validation
4. **Secure Storage**: Certificates stored in Kubernetes secrets
5. **Read-only Mounts**: Certificates mounted as read-only in pods

## Future Enhancements

1. **Certificate Rotation**: Implement automatic certificate rotation using cert-manager
2. **Certificate Monitoring**: Add metrics for certificate expiration
3. **Certificate Revocation**: Implement CRL or OCSP support
4. **Production CA**: Use a proper CA (e.g., cert-manager with Let's Encrypt or internal CA)

## Troubleshooting

### Agent cannot connect to Core

1. Check if certificates are mounted:
   ```bash
   kubectl exec -n ksam <agent-pod> -- ls -la /etc/ksam/certs/
   kubectl exec -n ksam <agent-pod> -- ls -la /etc/ksam/ca-cert/
   ```

2. Check Agent logs for TLS errors:
   ```bash
   kubectl logs -n ksam -l app=ksam-agent | grep -i "tls\|certificate\|error"
   ```

3. Verify Core is accepting mTLS connections:
   ```bash
   kubectl logs -n ksam -l app=ksam-core | grep -i "gRPC server"
   ```

### Core fails to start

1. Check if certificates exist in secrets:
   ```bash
   kubectl get secrets -n ksam | grep tls
   ```

2. Verify certificate paths in deployment:
   ```bash
   kubectl describe deployment -n ksam ksam-core | grep -A 5 "TLS_"
   ```

3. Check Core logs for certificate loading errors:
   ```bash
   kubectl logs -n ksam -l app=ksam-core | grep -i "certificate\|tls\|error"
   ```

## Status

✅ **Phase 1.2: Agent Implementation - gRPC Client với mTLS** - **COMPLETED**

- Certificate generation script created
- Core gRPC server updated with mTLS support
- Agent gRPC client updated with mTLS support
- Configurations updated for TLS paths
- Deployments updated to mount certificates
- Certificates generated and secrets created
- Agent successfully connecting with mTLS

