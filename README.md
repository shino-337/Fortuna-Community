# Kubernetes Service Account Manager (KSAM)

Hệ thống quản lý ServiceAccount tập trung cho Kubernetes clusters.

## Tổng quan

KSAM là một công cụ quản lý tập trung để quản lý, audit và visualize ServiceAccounts trên nhiều Kubernetes clusters, đảm bảo visibility, compliance và least-privilege principles.

## Cấu trúc dự án

```
.
├── agent/              # KSAM Agent - DaemonSet chạy trong mỗi cluster
├── core/               # KSAM Core Controller - API Server + Scheduler
├── dashboard/          # KSAM Dashboard - Web UI
├── helm/               # Helm charts cho deployment
├── docs/               # Documentation
├── scripts/            # Utility scripts
└── docker-compose.yml  # Local development setup
```

## Components

### 1. KSAM Agent
- Lightweight DaemonSet deployed trong mỗi cluster
- Thu thập và theo dõi ServiceAccounts, RoleBindings, ClusterRoleBindings
- Gửi dữ liệu về Core Controller qua gRPC với retry logic
- Sử dụng shared informer cache cho performance

### 2. KSAM Core Controller
- Central API Server và Scheduler
- REST/gRPC API
- PostgreSQL database với connection pooling
- Prometheus metrics
- Health endpoints (/health, /ready, /live)
- Authentication và authorization

### 3. KSAM Dashboard
- Web UI với graph visualization (Cytoscape.js)
- Hiển thị relationships giữa SAs, Roles, Namespaces
- Filtering và pagination
- Real-time updates

## Quick Start

### Prerequisites
- Go 1.20+
- Node.js 18+
- Docker & Docker Compose
- kubectl
- Helm 3.x
- PostgreSQL

### Development

1. Clone repository
2. Start local development environment:
```bash
docker-compose up -d
```

3. Deploy Agent to cluster:
```bash
cd helm/ksam
helm install ksam ./helm/ksam
```

4. Start Core Controller:
```bash
cd core
export DATABASE_URL=postgres://postgres:postgres@localhost:5432/ksam?sslmode=disable
export JWT_SECRET=your-secret-key
go run cmd/main.go
```

5. Start Dashboard:
```bash
cd dashboard
npm install
npm run dev
```

## Features

### ✅ Đã Implement
- ✅ Discovery: Enumerate và monitor ServiceAccounts, RoleBindings
- ✅ Centralized Management: View, edit, delete, disable ServiceAccounts
- ✅ Bulk Operations: Disable/delete multiple SAs, disable inactive SAs
- ✅ Visualization: Interactive graph view với filtering
- ✅ Audit & Compliance: Audit logs, reports
- ✅ Authentication: JWT-based authentication với roles
- ✅ Security: Security headers, CORS, password hashing
- ✅ Performance: Connection pooling, shared informer cache, retry logic
- ✅ Observability: Prometheus metrics, health endpoints

### ⚠️ Optional/Enhancement
- ⚠️ Token usage tracking
- ⚠️ Role revocation API
- ⚠️ Token rotation API
- ⚠️ TLS/mTLS configuration
- ⚠️ Redis caching (code ready, cần enable)
- ⚠️ OPA/Kyverno integration

## API Endpoints

### Public
- `GET /health` - Health check
- `GET /ready` - Readiness check
- `GET /live` - Liveness check
- `GET /metrics` - Prometheus metrics
- `POST /api/v1/auth/login` - Login

### Protected (require authentication)
- `GET /api/v1/clusters` - List clusters
- `GET /api/v1/serviceaccounts` - List service accounts
- `GET /api/v1/graph` - Get graph data
- `GET /api/v1/audit` - Get audit logs
- `POST /api/v1/serviceaccounts/bulk/disable` - Bulk disable (admin only)
- `POST /api/v1/serviceaccounts/disable-inactive` - Disable inactive (admin only)

## Configuration

### Core Controller
- `DATABASE_URL` - PostgreSQL connection string
- `JWT_SECRET` - JWT signing secret (required)
- `AUTH_ENABLED` - Enable authentication (default: true)
- `HTTP_PORT` - HTTP server port (default: 8080)
- `GRPC_PORT` - gRPC server port (default: 9090)
- `REDIS_URL` - Redis connection string (optional)

### Agent
- `KSAM_CORE_ENDPOINT` - Core Controller endpoint
- `KSAM_CLUSTER_ID` - Cluster identifier
- `KSAM_SYNC_INTERVAL` - Sync interval (default: 30s)

## Deployment

### Helm Chart
```bash
helm install ksam ./helm/ksam
```

### Manual Deployment
```bash
# Deploy Agent
kubectl apply -f agent/deploy/rbac.yaml
kubectl apply -f agent/deploy/daemonset.yaml

# Deploy Core
kubectl apply -f core/deploy/
```

## Performance Optimizations

1. **Database Connection Pooling**: Max 100 connections, idle timeout 10min
2. **Shared Informer Cache**: Reuse informers, reduce API server load
3. **Retry Logic**: Exponential backoff cho Agent sync
4. **Pagination**: Default page size 50, configurable
5. **Caching**: Redis support (optional)

## Security

- JWT authentication với token expiration
- Password hashing (bcrypt, cost 12)
- Security headers (X-Frame-Options, CSP, HSTS, etc.)
- Role-based access control (admin, user, viewer)
- Audit logging với user tracking

## Documentation

Xem [docs/](docs/) để biết thêm chi tiết:
- [Architecture](docs/ARCHITECTURE.md)
- [Deployment Guide](docs/DEPLOYMENT.md)
- [Security Guide](core/docs/SECURITY.md)
- [Implementation Status](docs/IMPLEMENTATION_STATUS.md)

## License

MIT
