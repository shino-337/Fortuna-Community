# KSAM Security Guide

**Document Version**: 1.0  
**Last Updated**: 2025-11-28  
**Audience**: Security Engineers, DevOps, Platform Teams

---

## Table of Contents

- [Security Philosophy](#security-philosophy)
- [Threat Model](#threat-model)
- [Authentication & Authorization](#authentication--authorization)
- [Network Security](#network-security)
- [Data Security](#data-security)
- [Secrets Management](#secrets-management)
- [RBAC Configuration](#rbac-configuration)
- [Agent Security](#agent-security)
- [Supply Chain Security](#supply-chain-security)
- [Audit & Compliance](#audit--compliance)
- [Incident Response](#incident-response)
- [Security Hardening Checklist](#security-hardening-checklist)
- [Appendix](#appendix)

---

## Security Philosophy

KSAM is designed with **security-first principles**:

1. **Principle of Least Privilege**: Components have minimal permissions required
2. **Defense in Depth**: Multiple layers of security controls
3. **Zero Trust**: All communication authenticated and encrypted
4. **Fail Secure**: Default deny, explicit allow
5. **Auditability**: All actions logged and traceable
6. **Isolation**: Components isolated with network policies

**Security Posture**:
- ✅ mTLS for all inter-component communication
- ✅ Role-based access control (RBAC)
- ✅ Encrypted data at rest and in transit
- ✅ Audit logging for all operations
- ✅ Minimal agent privileges (no secret read access)
- ✅ Regular security scanning (Trivy, Falco)

---

## Threat Model

### Assets to Protect

| Asset | Criticality | Threats |
|-------|-------------|---------|
| Kubernetes API credentials | Critical | Theft, misuse, privilege escalation |
| PostgreSQL data | Critical | Data breach, tampering, deletion |
| Agent-Core communication | High | MITM, eavesdropping, tampering |
| Dashboard access | High | Unauthorized access, data exfiltration |
| Risk insights | Medium | Information disclosure |
| eBPF programs | Medium | Malicious injection, kernel exploits |

### Threat Actors

**Internal Threats**:
- Malicious insider with cluster access
- Compromised ServiceAccount
- Misconfigured workload

**External Threats**:
- External attacker who compromised a pod
- Supply chain attack (compromised container image)
- Network-based attacker (if exposed externally)

### Attack Scenarios

#### Scenario 1: Compromised KSAM Agent
**Attack**: Attacker compromises node, gains access to agent pod

**Impact**:
- Agent can forward fake inventory data
- Agent can attempt to exploit Core controller
- Agent has read access to node resources

**Mitigations**:
- ✅ mTLS prevents agent impersonation
- ✅ Agent has no write access to cluster
- ✅ Agent cannot read secrets
- ✅ Core validates and sanitizes all agent data
- ✅ Network policies limit agent egress

#### Scenario 2: Database Compromise
**Attack**: Attacker gains access to PostgreSQL

**Impact**:
- Read all cluster inventory and insights
- Modify risk scores and insights
- Delete data

**Mitigations**:
- ✅ Database credentials in external secret manager (Vault)
- ✅ TLS for database connections
- ✅ Database network policy (only Core can access)
- ✅ Encryption at rest (PostgreSQL TDE)
- ✅ Regular backups with encryption
- ✅ Database audit logging

#### Scenario 3: Dashboard Credential Theft
**Attack**: Attacker steals dashboard credentials

**Impact**:
- View all cluster data
- Trigger attack simulations
- Modify policies (if admin)

**Mitigations**:
- ✅ Strong authentication (JWT + password hashing)
- ✅ Role-based access control
- ✅ Session timeout (30 minutes)
- ✅ Audit logging of all actions
- ✅ Rate limiting on login attempts
- ✅ Optional: SSO/SAML integration

#### Scenario 4: Supply Chain Attack
**Attack**: Malicious code injected into KSAM images

**Impact**:
- Complete cluster compromise
- Data exfiltration
- Backdoor installation

**Mitigations**:
- ✅ Container image signing (cosign)
- ✅ SBOM (Software Bill of Materials)
- ✅ Vulnerability scanning (Trivy)
- ✅ Minimal base images (distroless)
- ✅ Reproducible builds
- ✅ Admission controller (verify signatures)

#### Scenario 5: MITM on Agent-Core Communication
**Attack**: Attacker intercepts agent-core traffic

**Impact**:
- Eavesdrop on inventory data
- Tamper with events
- Inject malicious data

**Mitigations**:
- ✅ mTLS (mutual TLS) required
- ✅ Certificate validation
- ✅ Certificate rotation (30 days)
- ✅ Network policies (only agents can reach Core gRPC)

---

## Authentication & Authorization

### Component Authentication Matrix

| Component | Authentication Method | Authorization |
|-----------|----------------------|---------------|
| Agent → Core | mTLS (client certificates) | Agent RBAC role |
| Dashboard → Core API | JWT (Bearer token) | User RBAC role |
| Core → PostgreSQL | Username/Password (Vault) | Database user permissions |
| Core → NATS | Username/Password | NATS ACL |
| User → Dashboard | Username/Password + JWT | User role (admin/user/viewer) |

---

### User Authentication

#### JWT Configuration

```yaml
# ConfigMap: ksam-core-config
auth:
  jwt:
    secret: ${JWT_SECRET}  # From secret
    issuer: "ksam.io"
    expiration: 1800  # 30 minutes
    refresh_expiration: 86400  # 24 hours
  
  password:
    min_length: 12
    require_uppercase: true
    require_lowercase: true
    require_number: true
    require_special: true
    bcrypt_cost: 12
  
  session:
    timeout: 1800  # 30 minutes
    max_concurrent: 3
  
  rate_limiting:
    login_attempts: 5
    lockout_duration: 900  # 15 minutes
```

#### User Roles

**Admin**:
- Full access to all resources
- Can manage users and roles
- Can apply policies
- Can modify system configuration

**User**:
- Read/write insights
- View resources
- Run simulations
- Generate policies (preview only)

**Viewer**:
- Read-only access
- View resources, insights, graphs
- Cannot modify anything

**Role Definition**:
```go
type Role string

const (
    RoleAdmin  Role = "admin"
    RoleUser   Role = "user"
    RoleViewer Role = "viewer"
)

var RolePermissions = map[Role][]Permission{
    RoleAdmin: {
        PermReadAll,
        PermWriteAll,
        PermManageUsers,
        PermApplyPolicies,
        PermManageConfig,
    },
    RoleUser: {
        PermReadAll,
        PermWriteInsights,
        PermRunSimulations,
        PermPreviewPolicies,
    },
    RoleViewer: {
        PermReadAll,
    },
}
```

#### API Authentication Flow

```
1. User Login:
   POST /api/v1/auth/login
   {
     "username": "user@example.com",
     "password": "SecurePassword123!"
   }
   
   Response:
   {
     "access_token": "eyJhbGc...",
     "refresh_token": "eyJhbGc...",
     "expires_in": 1800,
     "user": {
       "id": "user-123",
       "username": "user@example.com",
       "role": "user"
     }
   }

2. Authenticated Request:
   GET /api/v1/insights
   Authorization: Bearer eyJhbGc...

3. Token Refresh:
   POST /api/v1/auth/refresh
   {
     "refresh_token": "eyJhbGc..."
   }
   
   Response:
   {
     "access_token": "eyJhbGc...",
     "expires_in": 1800
   }
```

#### Middleware Chain

```go
// API request flow
Request → RateLimitMiddleware → AuthMiddleware → RBACMiddleware → Handler

func AuthMiddleware(c *gin.Context) {
    // Extract JWT from Authorization header
    token := extractToken(c.Request)
    
    // Validate JWT
    claims, err := validateJWT(token)
    if err != nil {
        c.JSON(401, gin.H{"error": "Unauthorized"})
        c.Abort()
        return
    }
    
    // Load user from database
    user, err := db.GetUser(claims.UserID)
    if err != nil {
        c.JSON(401, gin.H{"error": "User not found"})
        c.Abort()
        return
    }
    
    // Check if session is still valid
    if !user.SessionValid(claims.SessionID) {
        c.JSON(401, gin.H{"error": "Session expired"})
        c.Abort()
        return
    }
    
    // Set user in context
    c.Set("user", user)
    c.Next()
}

func RBACMiddleware(requiredPermission Permission) gin.HandlerFunc {
    return func(c *gin.Context) {
        user := c.MustGet("user").(*User)
        
        if !user.HasPermission(requiredPermission) {
            c.JSON(403, gin.H{"error": "Forbidden"})
            c.Abort()
            return
        }
        
        c.Next()
    }
}
```

---

### Agent Authentication (mTLS)

#### Certificate Architecture

```
CA Certificate (Root)
├── Server Certificate (ksam-core)
│   ├── CN: ksam-core.ksam.svc.cluster.local
│   ├── Valid: 90 days
│   └── Auto-renewal: 60 days
│
└── Client Certificates (ksam-agent-*)
    ├── CN: ksam-agent-node-1
    ├── Valid: 30 days
    └── Auto-renewal: 20 days
```

#### Certificate Generation (cert-manager)

```yaml
# CA Issuer
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: ksam-ca-issuer
  namespace: ksam
spec:
  ca:
    secretName: ksam-ca-secret

---
# Server Certificate (Core)
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ksam-core-server
  namespace: ksam
spec:
  secretName: ksam-core-tls
  duration: 2160h  # 90 days
  renewBefore: 720h  # 30 days before expiry
  issuerRef:
    name: ksam-ca-issuer
    kind: Issuer
  commonName: ksam-core.ksam.svc.cluster.local
  dnsNames:
  - ksam-core
  - ksam-core.ksam
  - ksam-core.ksam.svc
  - ksam-core.ksam.svc.cluster.local
  usages:
  - server auth
  - key encipherment
  - digital signature

---
# Client Certificate Template (Agent)
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ksam-agent-client
  namespace: ksam
spec:
  secretName: ksam-agent-tls
  duration: 720h  # 30 days
  renewBefore: 240h  # 10 days before expiry
  issuerRef:
    name: ksam-ca-issuer
    kind: Issuer
  commonName: ksam-agent
  usages:
  - client auth
  - key encipherment
  - digital signature
```

#### mTLS Configuration (Server)

```go
// pkg/grpc/server.go

func NewSecureGRPCServer(tlsConfig *TLSConfig) (*grpc.Server, error) {
    // Load CA cert
    caCert, err := os.ReadFile(tlsConfig.CACertPath)
    if err != nil {
        return nil, err
    }
    
    certPool := x509.NewCertPool()
    if !certPool.AppendCertsFromPEM(caCert) {
        return nil, errors.New("failed to add CA cert")
    }
    
    // Load server cert and key
    serverCert, err := tls.LoadX509KeyPair(
        tlsConfig.ServerCertPath,
        tlsConfig.ServerKeyPath,
    )
    if err != nil {
        return nil, err
    }
    
    // TLS config with client cert verification
    tlsConf := &tls.Config{
        ClientAuth:   tls.RequireAndVerifyClientCert,
        ClientCAs:    certPool,
        Certificates: []tls.Certificate{serverCert},
        MinVersion:   tls.VersionTLS13,
        CipherSuites: []uint16{
            tls.TLS_AES_256_GCM_SHA384,
            tls.TLS_AES_128_GCM_SHA256,
            tls.TLS_CHACHA20_POLY1305_SHA256,
        },
    }
    
    // Create gRPC server with TLS
    creds := credentials.NewTLS(tlsConf)
    return grpc.NewServer(
        grpc.Creds(creds),
        grpc.MaxRecvMsgSize(10 * 1024 * 1024),  // 10MB
        grpc.KeepaliveParams(keepalive.ServerParameters{
            Time:    30 * time.Second,
            Timeout: 10 * time.Second,
        }),
    ), nil
}
```

#### mTLS Configuration (Client/Agent)

```go
// pkg/agent/grpc_client.go

func NewSecureGRPCClient(tlsConfig *TLSConfig, serverAddr string) (*grpc.ClientConn, error) {
    // Load CA cert
    caCert, err := os.ReadFile(tlsConfig.CACertPath)
    if err != nil {
        return nil, err
    }
    
    certPool := x509.NewCertPool()
    if !certPool.AppendCertsFromPEM(caCert) {
        return nil, errors.New("failed to add CA cert")
    }
    
    // Load client cert and key
    clientCert, err := tls.LoadX509KeyPair(
        tlsConfig.ClientCertPath,
        tlsConfig.ClientKeyPath,
    )
    if err != nil {
        return nil, err
    }
    
    // TLS config
    tlsConf := &tls.Config{
        ServerName:   "ksam-core.ksam.svc.cluster.local",
        RootCAs:      certPool,
        Certificates: []tls.Certificate{clientCert},
        MinVersion:   tls.VersionTLS13,
    }
    
    // Create gRPC client connection
    creds := credentials.NewTLS(tlsConf)
    return grpc.Dial(
        serverAddr,
        grpc.WithTransportCredentials(creds),
        grpc.WithBlock(),
        grpc.WithTimeout(10*time.Second),
    )
}
```

#### Certificate Rotation

**Automatic Rotation** (cert-manager handles this):
- Server certs renewed 30 days before expiry
- Client certs renewed 10 days before expiry
- Zero-downtime rotation (new cert loaded, old connections drain)

**Manual Rotation** (emergency):
```bash
# Revoke and reissue certificate
kubectl delete certificate ksam-core-server -n ksam
kubectl apply -f certificates/ksam-core-server.yaml

# Restart pods to pick up new cert
kubectl rollout restart deployment ksam-core -n ksam
kubectl rollout restart daemonset ksam-agent -n ksam
```

**Monitoring**:
```yaml
# Prometheus alert
- alert: CertificateExpiringSoon
  expr: |
    certmanager_certificate_expiration_timestamp_seconds
    - time() < 86400 * 7  # 7 days
  labels:
    severity: warning
  annotations:
    summary: "Certificate {{ $labels.name }} expiring soon"
```

---

## Network Security

### Network Policies

#### Principle: Default Deny + Explicit Allow

**Default Deny All**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: ksam
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
```

#### Agent Network Policy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ksam-agent-netpol
  namespace: ksam
spec:
  podSelector:
    matchLabels:
      app: ksam-agent
  policyTypes:
  - Egress
  egress:
  # Allow DNS
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
  
  # Allow Kubernetes API
  - to:
    - namespaceSelector: {}
      podSelector:
        matchLabels:
          component: kube-apiserver
    ports:
    - protocol: TCP
      port: 6443
  
  # Allow Core gRPC
  - to:
    - podSelector:
        matchLabels:
          app: ksam-core
    ports:
    - protocol: TCP
      port: 9090
  
  # Allow containerd/CRI-O (for container context resolution)
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 10010  # containerd
```

#### Core Network Policy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ksam-core-netpol
  namespace: ksam
spec:
  podSelector:
    matchLabels:
      app: ksam-core
  policyTypes:
  - Ingress
  - Egress
  
  ingress:
  # Allow agents (gRPC)
  - from:
    - podSelector:
        matchLabels:
          app: ksam-agent
    ports:
    - protocol: TCP
      port: 9090
  
  # Allow dashboard (HTTP API)
  - from:
    - podSelector:
        matchLabels:
          app: ksam-dashboard
    ports:
    - protocol: TCP
      port: 8080
  
  # Allow Prometheus scraping
  - from:
    - namespaceSelector:
        matchLabels:
          name: monitoring
      podSelector:
        matchLabels:
          app: prometheus
    ports:
    - protocol: TCP
      port: 8080
  
  egress:
  # Allow DNS
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
  
  # Allow PostgreSQL
  - to:
    - podSelector:
        matchLabels:
          app: postgres
    ports:
    - protocol: TCP
      port: 5432
  
  # Allow NATS
  - to:
    - podSelector:
        matchLabels:
          app: nats
    ports:
    - protocol: TCP
      port: 4222
  
  # Allow Kubernetes API
  - to:
    - namespaceSelector: {}
      podSelector:
        matchLabels:
          component: kube-apiserver
    ports:
    - protocol: TCP
      port: 6443
```

#### Dashboard Network Policy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ksam-dashboard-netpol
  namespace: ksam
spec:
  podSelector:
    matchLabels:
      app: ksam-dashboard
  policyTypes:
  - Ingress
  - Egress
  
  ingress:
  # Allow from ingress controller or LoadBalancer
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 80
  
  egress:
  # Allow DNS
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
  
  # Allow Core API
  - to:
    - podSelector:
        matchLabels:
          app: ksam-core
    ports:
    - protocol: TCP
      port: 8080
```

#### PostgreSQL Network Policy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: postgres-netpol
  namespace: ksam
spec:
  podSelector:
    matchLabels:
      app: postgres
  policyTypes:
  - Ingress
  
  ingress:
  # Only allow Core
  - from:
    - podSelector:
        matchLabels:
          app: ksam-core
    ports:
    - protocol: TCP
      port: 5432
```

---

### Service Mesh (Optional)

For advanced security, consider using **Istio** or **Linkerd**:

**Benefits**:
- Automatic mTLS for all service-to-service communication
- Fine-grained traffic policies
- Observability (distributed tracing)
- Circuit breakers and retries

**Trade-offs**:
- Additional complexity
- Resource overhead (~50MB per sidecar)
- Learning curve

**Recommendation**: Not required for MVP, consider for enterprise deployments

---

## Data Security

### Encryption at Rest

#### PostgreSQL Transparent Data Encryption (TDE)

**Option 1: PostgreSQL Native Encryption** (pg_crypto)
```sql
-- Enable pgcrypto extension
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Encrypt sensitive columns
CREATE TABLE insights (
    id UUID PRIMARY KEY,
    title TEXT,
    description TEXT,
    -- Encrypt sensitive data
    remediation_encrypted BYTEA,
    metadata_encrypted BYTEA
);

-- Encrypt function
CREATE OR REPLACE FUNCTION encrypt_data(data TEXT, key TEXT)
RETURNS BYTEA AS $$
BEGIN
    RETURN pgp_sym_encrypt(data, key);
END;
$$ LANGUAGE plpgsql;

-- Decrypt function
CREATE OR REPLACE FUNCTION decrypt_data(data BYTEA, key TEXT)
RETURNS TEXT AS $$
BEGIN
    RETURN pgp_sym_decrypt(data, key);
END;
$$ LANGUAGE plpgsql;
```

**Option 2: Storage-Level Encryption** (Recommended)
- Use encrypted persistent volumes
- Cloud provider managed encryption:
  - AWS: EBS encryption with KMS
  - GCP: PD encryption with Cloud KMS
  - Azure: Disk encryption with Key Vault

```yaml
# Example: AWS EBS with encryption
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-pvc
  namespace: ksam
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
  storageClassName: gp3-encrypted

---
# StorageClass with encryption
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: gp3-encrypted
provisioner: ebs.csi.aws.com
parameters:
  type: gp3
  encrypted: "true"
  kmsKeyId: "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"
```

#### Backup Encryption

```bash
# Encrypt PostgreSQL backup
pg_dump ksam | \
  gpg --symmetric --cipher-algo AES256 \
  > backup-$(date +%Y%m%d).sql.gpg

# Upload to S3 with server-side encryption
aws s3 cp backup-$(date +%Y%m%d).sql.gpg \
  s3://ksam-backups/ \
  --server-side-encryption aws:kms \
  --ssekms-key-id arn:aws:kms:...
```

---

### Encryption in Transit

**All network communication must be encrypted**:

| Connection | Protocol | Encryption |
|------------|----------|------------|
| Agent → Core | gRPC | mTLS (TLS 1.3) |
| Dashboard → Core | HTTPS | TLS 1.3 |
| Core → PostgreSQL | PostgreSQL | TLS 1.2+ |
| Core → NATS | NATS | TLS 1.2+ |
| User → Dashboard | HTTPS | TLS 1.3 |

#### PostgreSQL TLS Configuration

```yaml
# PostgreSQL ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-config
  namespace: ksam
data:
  postgresql.conf: |
    ssl = on
    ssl_cert_file = '/etc/ssl/certs/server.crt'
    ssl_key_file = '/etc/ssl/private/server.key'
    ssl_ca_file = '/etc/ssl/certs/ca.crt'
    ssl_min_protocol_version = 'TLSv1.2'
    ssl_ciphers = 'HIGH:!aNULL:!MD5'
```

**Connection String**:
```
postgresql://user:pass@postgres.ksam.svc:5432/ksam?sslmode=require&sslrootcert=/path/to/ca.crt
```

#### NATS TLS Configuration

```yaml
# NATS ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: nats-config
  namespace: ksam
data:
  nats.conf: |
    port: 4222
    
    tls {
      cert_file: "/etc/nats/certs/server.crt"
      key_file:  "/etc/nats/certs/server.key"
      ca_file:   "/etc/nats/certs/ca.crt"
      verify: true
    }
    
    jetstream {
      store_dir: "/data"
      max_memory_store: 1GB
      max_file_store: 10GB
    }
```

---

## Secrets Management

### External Secrets Operator

**Recommended Setup**: Use External Secrets Operator with Vault/AWS Secrets Manager

#### Architecture

```
┌──────────────────────────────────────────────┐
│        Secret Provider (Vault/AWS)           │
│  ┌────────────────────────────────────────┐  │
│  │  - PostgreSQL credentials              │  │
│  │  - JWT secret                          │  │
│  │  - API keys                            │  │
│  │  - TLS certificates (optional)         │  │
│  └────────────────────────────────────────┘  │
└────────────────┬─────────────────────────────┘
                 │
                 │ External Secrets Operator
                 │ polls every 1 hour
                 │
┌────────────────▼─────────────────────────────┐
│           Kubernetes Cluster                 │
│  ┌────────────────────────────────────────┐  │
│  │  ExternalSecret CRDs                   │  │
│  └──────────────┬─────────────────────────┘  │
│                 │                             │
│                 │ creates/updates             │
│                 │                             │
│  ┌──────────────▼─────────────────────────┐  │
│  │  Kubernetes Secrets                    │  │
│  │  - ksam-db-secret                      │  │
│  │  - ksam-jwt-secret                     │  │
│  │  - ksam-api-keys                       │  │
│  └────────────────────────────────────────┘  │
└──────────────────────────────────────────────┘
```

#### Installation

```bash
# Install External Secrets Operator
helm repo add external-secrets https://charts.external-secrets.io
helm install external-secrets \
  external-secrets/external-secrets \
  -n external-secrets-system \
  --create-namespace
```

#### Vault SecretStore

```yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: vault-backend
  namespace: ksam
spec:
  provider:
    vault:
      server: "https://vault.example.com"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "ksam"
          serviceAccountRef:
            name: ksam-core
```

#### ExternalSecret for PostgreSQL Credentials

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: ksam-db-credentials
  namespace: ksam
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: ksam-db-secret
    creationPolicy: Owner
  data:
  - secretKey: username
    remoteRef:
      key: database/ksam/credentials
      property: username
  - secretKey: password
    remoteRef:
      key: database/ksam/credentials
      property: password
  - secretKey: connection_string
    remoteRef:
      key: database/ksam/credentials
      property: connection_string
```

#### ExternalSecret for JWT Secret

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: ksam-jwt-secret
  namespace: ksam
spec:
  refreshInterval: 24h
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: ksam-jwt-secret
    creationPolicy: Owner
  data:
  - secretKey: jwt_secret
    remoteRef:
      key: ksam/jwt
      property: secret
```

#### Vault Policy

```hcl
# Vault policy for KSAM
path "secret/data/database/ksam/*" {
  capabilities = ["read"]
}

path "secret/data/ksam/*" {
  capabilities = ["read"]
}
```

#### Secret Rotation

**Automatic Rotation** (External Secrets Operator):
- Polls Vault every 1 hour
- Updates Kubernetes Secret if changed
- Applications reload secrets (requires implementation)

**Application Secret Reload**:
```go
// pkg/config/secret_reloader.go

type SecretReloader struct {
    secretPath string
    lastModTime time.Time
    onReload func([]byte) error
}

func (sr *SecretReloader) Watch(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            sr.checkAndReload()
        }
    }
}

func (sr *SecretReloader) checkAndReload() {
    info, err := os.Stat(sr.secretPath)
    if err != nil {
        log.Error().Err(err).Msg("Failed to stat secret file")
        return
    }
    
    if info.ModTime().After(sr.lastModTime) {
        data, err := os.ReadFile(sr.secretPath)
        if err != nil {
            log.Error().Err(err).Msg("Failed to read secret file")
            return
        }
        
        if err := sr.onReload(data); err != nil {
            log.Error().Err(err).Msg("Failed to reload secret")
            return
        }
        
        sr.lastModTime = info.ModTime()
        log.Info().Msg("Secret reloaded successfully")
    }
}
```

---

## RBAC Configuration

### Kubernetes RBAC for KSAM Components

#### Agent ClusterRole (Minimal Permissions)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ksam-agent
rules:
# Read-only access to resources
- apiGroups: [""]
  resources: ["pods", "nodes", "namespaces", "serviceaccounts", "services", "endpoints"]
  verbs: ["get", "list", "watch"]

- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch"]
  # Note: NO access to secrets

- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["roles", "rolebindings", "clusterroles", "clusterrolebindings"]
  verbs: ["get", "list", "watch"]

- apiGroups: ["apps"]
  resources: ["deployments", "daemonsets", "statefulsets", "replicasets"]
  verbs: ["get", "list", "watch"]

- apiGroups: ["batch"]
  resources: ["jobs", "cronjobs"]
  verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ksam-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ksam-agent
subjects:
- kind: ServiceAccount
  name: ksam-agent
  namespace: ksam
```

**Security Notes**:
- ✅ **NO** secret read access
- ✅ **NO** write permissions
- ✅ **NO** delete permissions
- ✅ **NO** exec/attach permissions
- ✅ Read-only for inventory collection

#### Core ClusterRole (Controller Permissions)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ksam-core
rules:
# Read access for inventory
- apiGroups: [""]
  resources: ["pods", "nodes", "namespaces", "serviceaccounts", "services"]
  verbs: ["get", "list", "watch"]

# Write access for policy application
- apiGroups: ["security.kubearmor.com"]
  resources: ["kubearmorpolicies"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

- apiGroups: ["networking.k8s.io"]
  resources: ["networkpolicies"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# Label pods for quarantine
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "patch"]

# Read RoleBindings for RBAC analysis
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["roles", "rolebindings", "clusterroles", "clusterrolebindings"]
  verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ksam-core
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ksam-core
subjects:
- kind: ServiceAccount
  name: ksam-core
  namespace: ksam
```

**Security Notes**:
- ✅ Write access only for policies (KubeArmor, NetworkPolicy)
- ✅ Can label pods (for quarantine feature)
- ✅ Cannot delete pods or other resources
- ✅ Cannot exec into pods

#### ServiceAccount Configuration

```yaml
# Agent ServiceAccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ksam-agent
  namespace: ksam
automountServiceAccountToken: true

---
# Core ServiceAccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ksam-core
  namespace: ksam
automountServiceAccountToken: true

---
# Dashboard ServiceAccount (no special permissions needed)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ksam-dashboard
  namespace: ksam
automountServiceAccountToken: false  # Doesn't need to talk to K8s API
```

---

## Agent Security

### Pod Security Standards

**Apply PodSecurityStandard to KSAM namespace**:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ksam
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

**Exception for Agent (requires eBPF)**:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ksam
  labels:
    pod-security.kubernetes.io/enforce: baseline
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

### Agent SecurityContext

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ksam-agent
  namespace: ksam
spec:
  selector:
    matchLabels:
      app: ksam-agent
  template:
    metadata:
      labels:
        app: ksam-agent
    spec:
      serviceAccountName: ksam-agent
      hostPID: true  # Required for eBPF PID resolution
      hostNetwork: false  # NOT needed
      
      securityContext:
        seccompProfile:
          type: RuntimeDefault
        runAsNonRoot: false  # eBPF requires root privileges
      
      containers:
      - name: agent
        image: ksam/agent:latest
        
        securityContext:
          privileged: false  # NOT privileged
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          runAsNonRoot: false
          runAsUser: 0  # Required for eBPF
          capabilities:
            drop:
            - ALL
            add:
            - BPF          # Load eBPF programs
            - PERFMON      # Performance monitoring
            - SYS_RESOURCE # Resource limits
        
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "200m"
        
        volumeMounts:
        - name: sys
          mountPath: /sys
          readOnly: true
        - name: proc
          mountPath: /proc
          readOnly: true
        - name: tmp
          mountPath: /tmp
        - name: agent-config
          mountPath: /etc/ksam
          readOnly: true
        - name: tls-certs
          mountPath: /etc/ksam/certs
          readOnly: true
      
      volumes:
      - name: sys
        hostPath:
          path: /sys
          type: Directory
      - name: proc
        hostPath:
          path: /proc
          type: Directory
      - name: tmp
        emptyDir: {}
      - name: agent-config
        configMap:
          name: ksam-agent-config
      - name: tls-certs
        secret:
          secretName: ksam-agent-tls
```

**Key Security Features**:
- ✅ NOT privileged (common misconception that eBPF requires privileged)
- ✅ readOnlyRootFilesystem (immutable container)
- ✅ Minimal capabilities (BPF, PERFMON, SYS_RESOURCE only)
- ✅ Resource limits (prevent resource exhaustion)
- ✅ seccompProfile RuntimeDefault

### Core SecurityContext

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ksam-core
  namespace: ksam
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ksam-core
  template:
    metadata:
      labels:
        app: ksam-core
    spec:
      serviceAccountName: ksam-core
      
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
        seccompProfile:
          type: RuntimeDefault
      
      containers:
      - name: core
        image: ksam/core:latest
        
        securityContext:
          privileged: false
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 1000
          capabilities:
            drop:
            - ALL
        
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        
        volumeMounts:
        - name: tmp
          mountPath: /tmp
        - name: core-config
          mountPath: /etc/ksam
          readOnly: true
        - name: tls-certs
          mountPath: /etc/ksam/certs
          readOnly: true
      
      volumes:
      - name: tmp
        emptyDir: {}
      - name: core-config
        configMap:
          name: ksam-core-config
      - name: tls-certs
        secret:
          secretName: ksam-core-tls
```

**Key Security Features**:
- ✅ Runs as non-root user (UID 1000)
- ✅ readOnlyRootFilesystem
- ✅ Drop all capabilities
- ✅ No privilege escalation

---

## Supply Chain Security

### Container Image Security

#### Image Scanning (Trivy)

```bash
# Scan image for vulnerabilities
trivy image ksam/agent:latest \
  --severity HIGH,CRITICAL \
  --exit-code 1  # Fail if vulnerabilities found

# Scan for misconfigurations
trivy config deploy/

# Scan IaC (Helm charts)
trivy config helm/ksam/
```

**CI Integration** (GitHub Actions):
```yaml
name: Container Security Scan

on:
  push:
    branches: [main]
  pull_request:

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Build image
      run: docker build -t ksam/agent:${{ github.sha }} -f agent/Dockerfile .
    
    - name: Run Trivy scan
      uses: aquasecurity/trivy-action@master
      with:
        image-ref: ksam/agent:${{ github.sha }}
        format: 'sarif'
        output: 'trivy-results.sarif'
        severity: 'CRITICAL,HIGH'
    
    - name: Upload results to GitHub Security
      uses: github/codeql-action/upload-sarif@v2
      with:
        sarif_file: 'trivy-results.sarif'
```

#### Image Signing (cosign)

```bash
# Generate key pair
cosign generate-key-pair

# Sign image
cosign sign --key cosign.key ksam/agent:v1.0.0

# Verify signature
cosign verify --key cosign.pub ksam/agent:v1.0.0
```

**Admission Controller** (Kyverno):
```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: verify-ksam-images
spec:
  validationFailureAction: enforce
  rules:
  - name: verify-signature
    match:
      any:
      - resources:
          kinds:
          - Pod
          namespaces:
          - ksam
    verifyImages:
    - imageReferences:
      - "ksam/*"
      attestors:
      - count: 1
        entries:
        - keys:
            publicKeys: |-
              -----BEGIN PUBLIC KEY-----
              MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...
              -----END PUBLIC KEY-----
```

#### SBOM (Software Bill of Materials)

```bash
# Generate SBOM with Syft
syft ksam/agent:latest -o spdx-json > sbom-agent.json

# Attach SBOM to image
cosign attach sbom --sbom sbom-agent.json ksam/agent:latest

# Verify SBOM
cosign verify-attestation --key cosign.pub \
  --type spdxjson \
  ksam/agent:latest
```

#### Minimal Base Images

**Dockerfile Best Practices**:
```dockerfile
# Use distroless base image (no shell, minimal attack surface)
FROM gcr.io/distroless/static:nonroot

# Copy only necessary binaries
COPY --from=builder /app/agent /agent

# Run as non-root
USER nonroot:nonroot

# Set read-only filesystem
VOLUME /tmp

ENTRYPOINT ["/agent"]
```

**Multi-stage Build**:
```dockerfile
# Stage 1: Build
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags '-extldflags "-static" -s -w' \
    -o agent ./cmd/agent

# Stage 2: Runtime (distroless)
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/agent /agent

USER nonroot:nonroot

ENTRYPOINT ["/agent"]
```

---

## Audit & Compliance

### Audit Logging

#### Application Audit Logs

**What to Log**:
- Authentication attempts (success/failure)
- Authorization decisions (allowed/denied)
- Resource modifications (create/update/delete policies)
- Sensitive data access (viewing insights, running simulations)
- Configuration changes
- Admin actions

**Log Format** (structured JSON):
```json
{
  "timestamp": "2025-11-28T10:30:00Z",
  "level": "info",
  "event_type": "policy_applied",
  "user_id": "user-123",
  "user_email": "admin@example.com",
  "user_role": "admin",
  "action": "apply_policy",
  "resource_type": "kubearmor_policy",
  "resource_id": "pol-abc123",
  "resource_name": "nginx-security-policy",
  "cluster_id": "cluster-prod-1",
  "namespace": "default",
  "result": "success",
  "client_ip": "10.0.1.50",
  "user_agent": "Mozilla/5.0...",
  "trace_id": "trace-xyz789"
}
```

**Audit Middleware**:
```go
func AuditMiddleware(c *gin.Context) {
    start := time.Now()
    
    // Get user from context
    user, _ := c.Get("user")
    
    // Process request
    c.Next()
    
    // Log audit event
    logAuditEvent(AuditEvent{
        Timestamp:    time.Now(),
        EventType:    deriveEventType(c.Request.Method, c.Request.URL.Path),
        UserID:       user.(*User).ID,
        UserEmail:    user.(*User).Email,
        UserRole:     string(user.(*User).Role),
        Action:       c.Request.Method,
        ResourceType: extractResourceType(c.Request.URL.Path),
        ResourceID:   c.Param("id"),
        Result:       deriveResult(c.Writer.Status()),
        ClientIP:     c.ClientIP(),
        UserAgent:    c.Request.UserAgent(),
        Duration:     time.Since(start),
        TraceID:      c.GetHeader("X-Trace-ID"),
    })
}
```

#### Kubernetes Audit Logging

**Enable Audit Logging** (kube-apiserver):
```yaml
# audit-policy.yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
# Log all requests from KSAM ServiceAccounts
- level: RequestResponse
  users:
  - "system:serviceaccount:ksam:ksam-agent"
  - "system:serviceaccount:ksam:ksam-core"
  
# Log policy changes
- level: RequestResponse
  verbs: ["create", "update", "patch", "delete"]
  resources:
  - group: "security.kubearmor.com"
    resources: ["kubearmorpolicies"]
  - group: "networking.k8s.io"
    resources: ["networkpolicies"]

# Don't log read-only requests
- level: None
  verbs: ["get", "list", "watch"]
```

**kube-apiserver flags**:
```
--audit-policy-file=/etc/kubernetes/audit-policy.yaml
--audit-log-path=/var/log/kubernetes/audit.log
--audit-log-maxage=30
--audit-log-maxbackup=10
--audit-log-maxsize=100
```

#### Log Retention

**Retention Policy**:
- Application audit logs: 1 year
- Kubernetes audit logs: 90 days
- Access logs: 30 days

**Storage**:
- Local: `/var/log/ksam/audit/`
- Remote: Ship to SIEM (Splunk, Elastic, etc.)
- Backup: S3 with lifecycle policies

### Compliance Frameworks

#### CIS Kubernetes Benchmark v1.8

KSAM helps achieve CIS compliance:

**Section 5: Policies** (KSAM Focus):
- ✅ 5.1.1: ServiceAccount token mounting
- ✅ 5.1.2: Default ServiceAccounts
- ✅ 5.1.3: Cluster-admin bindings
- ✅ 5.2.1: Privileged containers
- ✅ 5.2.2-5.2.9: Pod Security Standards
- ✅ 5.3.1: NetworkPolicies
- ✅ 5.4.1: Secrets management

**Compliance Report Generation**:
```bash
# Run CIS compliance check
ksam compliance check --framework cis-1.8 --cluster prod-1

# Generate report
ksam compliance report --format pdf --output cis-report.pdf
```

#### SOC 2 Controls

KSAM supports SOC 2 compliance:

**CC6.1** (Logical Access Controls):
- ✅ RBAC implementation
- ✅ Audit logging
- ✅ Session management

**CC6.6** (Change Management):
- ✅ Policy change tracking
- ✅ Approval workflows (future)
- ✅ Rollback capability

**CC7.2** (System Monitoring):
- ✅ Real-time risk detection
- ✅ Anomaly detection
- ✅ Alert generation

---

## Incident Response

### Incident Response Playbooks

#### Scenario 1: Compromised ServiceAccount Detected

**Detection**:
- KSAM insight: "Unusual API access from ServiceAccount"
- Alert severity: High

**Response Steps**:

1. **Contain** (Immediate):
```bash
# Revoke ServiceAccount permissions
kubectl delete rolebinding <binding-name> -n <namespace>

# Quarantine pods using the SA
kubectl label pod -n <namespace> -l serviceaccount=<sa-name> \
  security.ksam.io/quarantine=true

# Apply deny-all NetworkPolicy
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: quarantine-<sa-name>
  namespace: <namespace>
spec:
  podSelector:
    matchLabels:
      security.ksam.io/quarantine: "true"
  policyTypes:
  - Ingress
  - Egress
EOF
```

2. **Investigate** (Within 1 hour):
```bash
# Check KSAM blast radius
curl -H "Authorization: Bearer $TOKEN" \
  http://ksam-core/api/v1/graph/blast-radius/<sa-id>

# Review audit logs
kubectl logs -n ksam -l app=ksam-core --since=1h | \
  grep <sa-name>

# Check Kubernetes audit logs
grep <sa-name> /var/log/kubernetes/audit.log
```

3. **Eradicate** (Within 4 hours):
```bash
# Delete compromised pods
kubectl delete pod -n <namespace> -l serviceaccount=<sa-name>

# Rotate secrets
kubectl delete secret -n <namespace> <secret-name>

# Create new ServiceAccount with least privilege
kubectl create sa <new-sa-name> -n <namespace>
kubectl apply -f new-role-binding.yaml
```

4. **Recover** (Within 24 hours):
```bash
# Deploy with new ServiceAccount
kubectl set serviceaccount deployment <deployment-name> <new-sa-name> -n <namespace>

# Apply KSAM-generated policies
kubectl apply -f ksam-policies/

# Monitor for 24 hours
watch kubectl get pods -n <namespace>
```

5. **Post-Incident**:
- Document in incident report
- Update detection rules if needed
- Review and improve policies

#### Scenario 2: Policy Violation Detected

**Detection**:
- KubeArmor alert: "Blocked unexpected process execution"
- KSAM insight: "Runtime behavior deviation"

**Response Steps**:

1. **Assess**:
```bash
# Check policy details
kubectl get kubearmor policy <policy-name> -o yaml

# Review violation logs
kubectl logs -n kube-system -l app=kubearmor | grep <pod-name>

# Check KSAM baseline
curl -H "Authorization: Bearer $TOKEN" \
  http://ksam-core/api/v1/baseline/<pod-id>
```

2. **Decide**:
- **Legitimate**: Update baseline, modify policy
- **Attack**: Follow incident response playbook

3. **Tune** (if false positive):
```bash
# Update baseline manually
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://ksam-core/api/v1/baseline/<pod-id>/add-process \
  -d '{"process": "/usr/bin/new-tool"}'

# Regenerate policy
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://ksam-core/api/v1/policies/generate/<pod-id>
```

### Security Contacts

**Internal**:
- Security Team: security@example.com
- On-Call: PagerDuty integration

**External**:
- KSAM Security: security@ksam.io
- Vulnerability Disclosure: security-reports@ksam.io

---

## Security Hardening Checklist

### Pre-Deployment Checklist

**Infrastructure**:
- [ ] Kubernetes version ≥ 1.24
- [ ] RBAC enabled
- [ ] Network policies supported
- [ ] PodSecurityStandards enabled
- [ ] Audit logging enabled
- [ ] etcd encryption enabled

**KSAM Configuration**:
- [ ] mTLS certificates generated (cert-manager)
- [ ] External Secrets Operator installed
- [ ] Secrets stored in Vault/AWS Secrets Manager
- [ ] Strong JWT secret (≥32 random bytes)
- [ ] Database credentials rotated
- [ ] Network policies applied
- [ ] Resource limits configured
- [ ] Backup strategy in place

**Images**:
- [ ] Images scanned for vulnerabilities
- [ ] Images signed (cosign)
- [ ] SBOM attached
- [ ] Using distroless base images
- [ ] Latest stable versions

### Post-Deployment Checklist

**Week 1**:
- [ ] Verify mTLS working (check logs)
- [ ] Verify authentication working (test login)
- [ ] Verify agent registration (check dashboard)
- [ ] Verify inventory collection (check data)
- [ ] Test backup/restore
- [ ] Configure monitoring alerts
- [ ] Review audit logs

**Month 1**:
- [ ] Security scan (trivy, kube-bench)
- [ ] Penetration test
- [ ] Load test
- [ ] Review RBAC permissions
- [ ] Review network policies
- [ ] Certificate rotation test
- [ ] Incident response drill

**Quarterly**:
- [ ] Security audit
- [ ] Compliance review (CIS, SOC2)
- [ ] Update dependencies
- [ ] Rotate secrets
- [ ] Review and update policies
- [ ] Disaster recovery test

### Security Monitoring

**Critical Alerts**:
- Certificate expiring < 7 days
- Failed authentication attempts > 10/min
- Database connection failures
- Agent disconnects > 50% nodes
- Policy violations (critical severity)
- Anomaly detection (high confidence)

**Prometheus Alerts**:
```yaml
groups:
- name: ksam_security
  rules:
  - alert: CertificateExpiringSoon
    expr: |
      certmanager_certificate_expiration_timestamp_seconds
      - time() < 604800  # 7 days
    labels:
      severity: warning
    annotations:
      summary: "Certificate {{ $labels.name }} expiring in < 7 days"
  
  - alert: HighFailedAuthAttempts
    expr: |
      rate(ksam_auth_failed_attempts_total[5m]) > 2
    labels:
      severity: critical
    annotations:
      summary: "High failed authentication rate"
  
  - alert: PolicyViolationCritical
    expr: |
      rate(ksam_policy_violations_total{severity="critical"}[5m]) > 0
    labels:
      severity: critical
    annotations:
      summary: "Critical policy violation detected"
```

---

## Appendix

### A. Security Hardening Scripts

#### A.1 Generate Strong Secrets

```bash
#!/bin/bash
# generate-secrets.sh

# Generate JWT secret
JWT_SECRET=$(openssl rand -base64 32)
echo "JWT_SECRET: $JWT_SECRET"

# Generate database password
DB_PASSWORD=$(openssl rand -base64 24)
echo "DB_PASSWORD: $DB_PASSWORD"

# Store in Vault
vault kv put secret/ksam/jwt secret="$JWT_SECRET"
vault kv put secret/database/ksam/credentials \
  username=ksam \
  password="$DB_PASSWORD"
```

#### A.2 Rotate Certificates

```bash
#!/bin/bash
# rotate-certs.sh

# Delete old certificates
kubectl delete certificate ksam-core-server -n ksam
kubectl delete certificate ksam-agent-client -n ksam

# Recreate (cert-manager will issue new certs)
kubectl apply -f certificates/

# Wait for issuance
kubectl wait --for=condition=Ready certificate/ksam-core-server -n ksam --timeout=300s

# Restart pods to pick up new certs
kubectl rollout restart deployment ksam-core -n ksam
kubectl rollout restart daemonset ksam-agent -n ksam
```

#### A.3 Backup Encryption Keys

```bash
#!/bin/bash
# backup-keys.sh

# Backup Vault keys
vault operator raft snapshot save vault-snapshot-$(date +%Y%m%d).snap

# Encrypt snapshot
gpg --symmetric --cipher-algo AES256 \
  vault-snapshot-$(date +%Y%m%d).snap

# Upload to S3
aws s3 cp vault-snapshot-$(date +%Y%m%d).snap.gpg \
  s3://ksam-backups/vault/ \
  --server-side-encryption aws:kms
```

### B. Security Best Practices

1. **Principle of Least Privilege**: Always grant minimum required permissions
2. **Defense in Depth**: Multiple layers of security (network, RBAC, encryption)
3. **Zero Trust**: Verify every request, encrypt all communication
4. **Regular Updates**: Keep all components up to date
5. **Monitor Everything**: Comprehensive logging and alerting
6. **Test Regularly**: Security scans, penetration tests, DR drills
7. **Incident Response**: Have playbooks ready, practice them
8. **Supply Chain**: Verify all dependencies, sign images

### C. Common Security Mistakes to Avoid

❌ **Don't**:
- Run containers as root (unless absolutely necessary)
- Use privileged containers
- Mount host filesystem without read-only
- Store secrets in ConfigMaps or environment variables
- Use default passwords
- Expose dashboard publicly without authentication
- Disable RBAC or network policies
- Ignore security scan results

✅ **Do**:
- Use non-root users
- Apply SecurityContext
- Use mTLS for internal communication
- Store secrets in Vault/external secret manager
- Rotate credentials regularly
- Implement authentication and RBAC
- Apply network policies
- Scan images and fix vulnerabilities

### D. Useful Resources

**KSAM**:
- Documentation: https://docs.ksam.io/security
- Security Advisories: https://github.com/ksam/security-advisories
- Vulnerability Reports: security@ksam.io

**Kubernetes Security**:
- CIS Kubernetes Benchmark: https://www.cisecurity.org/benchmark/kubernetes
- Kubernetes Security Best Practices: https://kubernetes.io/docs/concepts/security/
- Pod Security Standards: https://kubernetes.io/docs/concepts/security/pod-security-standards/

**Tools**:
- Trivy: https://github.com/aquasecurity/trivy
- Falco: https://falco.org/
- KubeArmor: https://kubearmor.io/
- cert-manager: https://cert-manager.io/
- External Secrets Operator: https://external-secrets.io/

---

**Document End**

For questions or security concerns, contact: security@ksam.io
