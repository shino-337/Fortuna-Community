# KSAM Core Controller

KSAM Core Controller là central API Server và Scheduler quản lý dữ liệu từ tất cả các clusters.

## Chức năng

- Nhận và lưu trữ dữ liệu từ các KSAM Agents
- Cung cấp REST/gRPC API cho Dashboard và external tools
- Quản lý kết nối với nhiều clusters
- Xử lý audit logs và compliance reports
- Graph data processing cho visualization

## Cấu trúc

```
core/
├── cmd/
│   └── main.go           # Entry point
├── internal/
│   ├── api/              # REST API handlers
│   ├── grpc/             # gRPC server
│   ├── service/          # Business logic
│   ├── graph/            # Graph processing
│   ├── storage/          # Database layer
│   └── config/           # Configuration
├── pkg/
│   └── models/           # Data models
├── proto/                # gRPC proto definitions
├── Dockerfile
├── go.mod
└── go.sum
```

## Build

```bash
go build -o bin/ksam-core ./cmd
```

## Run

```bash
./bin/ksam-core --config config.yaml
```

## Configuration

- `DATABASE_URL`: PostgreSQL connection string
- `REDIS_URL`: Redis connection string (optional)
- `GRPC_PORT`: gRPC server port (default: 9090)
- `HTTP_PORT`: HTTP server port (default: 8080)
- `LOG_LEVEL`: Log level (default: info)

## API Endpoints

### REST API

- `GET /health` - Health check
- `GET /api/v1/clusters` - List all clusters
- `GET /api/v1/clusters/:id` - Get cluster by ID
- `GET /api/v1/serviceaccounts` - List service accounts (with filters)
- `GET /api/v1/serviceaccounts/:id` - Get service account by ID
- `PUT /api/v1/serviceaccounts/:id` - Update service account
- `DELETE /api/v1/serviceaccounts/:id` - Delete service account
- `GET /api/v1/graph` - Get graph data (with filters)
- `GET /api/v1/audit` - Get audit logs (with filters)
- `GET /api/v1/audit/reports` - Get audit reports

### gRPC API

- `SyncData` - Sync collected data from agent
- `HealthCheck` - Health check

## Database Models

- **Cluster**: Kubernetes cluster information
- **ServiceAccount**: ServiceAccount data
- **RoleBinding**: RoleBinding data
- **ClusterRoleBinding**: ClusterRoleBinding data
- **Role**: Role data
- **ClusterRole**: ClusterRole data
- **Pod**: Pod data (to track SA usage)
- **AuditLog**: Audit log entries

## Graph Processing

Graph service builds relationships between:
- ServiceAccounts → Namespaces → Clusters
- ServiceAccounts → Roles (via RoleBindings)
- ServiceAccounts → ClusterRoles (via ClusterRoleBindings)

## Local Development

### Prerequisites

- Go 1.20+
- PostgreSQL
- Redis (optional)

### Setup

1. Start PostgreSQL:
```bash
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:15-alpine
```

2. Set environment variables:
```bash
export DATABASE_URL=postgres://postgres:postgres@localhost:5432/ksam?sslmode=disable
export HTTP_PORT=8080
export GRPC_PORT=9090
```

3. Run:
```bash
go run cmd/main.go
```

## Generate Proto Files

```bash
# Install protoc and plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate Go code from proto
protoc --go_out=. --go-grpc_out=. proto/ksam.proto
```

## Testing

```bash
go test ./...
```
