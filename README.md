# K8s Workload Management Platform

> Formerly known as KSAM (K8s Service Account Management)

A comprehensive Kubernetes workload management platform for monitoring ServiceAccounts, RBAC, audit logs, and cluster resources.

## ✨ Features

### 🔐 Authentication & Authorization
- JWT-based authentication
- Role-Based Access Control (RBAC)
- Two roles: Admin & User
- Granular permissions system

### 📊 Dashboard & Monitoring
- **Dashboard**: Overview of cluster resources
- **Graph View**: Visual representation of ServiceAccounts, Namespaces, and Clusters
- **ServiceAccounts**: Manage and monitor all ServiceAccounts
- **Audit Logs**: Complete audit trail of all changes

### 🌓 Modern UI/UX
- Dark mode support with system preference detection
- Responsive design
- Real-time updates
- Custom branding and icons

### 📝 Audit Logging
- Track all CREATE, UPDATE, DELETE operations
- No throttling - every event is logged
- 90-day retention with automatic cleanup
- Detailed event information

### 🔄 Real-time Sync
- Agent-based architecture
- Delta sync for efficiency
- Full sync every ~10th cycle
- Watcher for real-time events

## 🏗️ Architecture

```
┌─────────────────┐
│   Dashboard     │  (React + TypeScript + Tailwind)
│   (Frontend)    │
└────────┬────────┘
         │ HTTP/REST
         ▼
┌─────────────────┐
│   Core API      │  (Go + Gin + GORM)
│   Controller    │
└────────┬────────┘
         │
         ├─────► PostgreSQL (Data storage)
         │
         ▲ gRPC/HTTP
         │
┌────────┴────────┐
│   Agent         │  (Go + K8s Client)
│   (DaemonSet)   │
└─────────────────┘
         │
         ▼
    Kubernetes API
```

## 🚀 Quick Start

### Prerequisites
- Minikube or Kubernetes cluster
- kubectl configured
- Docker

### Installation

1. **Clone repository**
```bash
git clone <repository-url>
cd KSAM
```

2. **Build and deploy**
```bash
# Build all components
./scripts/rebuild.sh all

# Or build individually
./scripts/rebuild.sh core
./scripts/rebuild.sh agent
./scripts/rebuild.sh dashboard
```

3. **Access Dashboard**
```bash
# Get dashboard URL
minikube service ksam-dashboard -n ksam --url

# Or port-forward
kubectl port-forward -n ksam svc/ksam-dashboard 30080:80
```

4. **Default Login**
- **Username**: `admin`
- **Password**: `admin123`
- **Role**: admin

## 📚 Documentation

- [UI Improvements & RBAC](./docs/UI_IMPROVEMENTS.md)
- [Architecture Document](./docs/k8s-event-sync-architecture.md)
- [Graph Dashboard Enhancement](./docs/k8s-graph-dashboard-enhancement.md)
- [Migration Guide](./core/migrations/README.md)

## 🔐 RBAC Permissions

| Feature | Admin | User |
|---------|-------|------|
| View Dashboard | ✅ | ✅ |
| View ServiceAccounts | ✅ | ✅ |
| Create/Edit ServiceAccounts | ✅ | ❌ |
| Delete ServiceAccounts | ✅ | ❌ |
| View Audit Logs | ✅ | ✅ |
| View Graph | ✅ | ✅ |
| User Management | ✅ | ❌ |

## 🛠️ Development

### Project Structure
```
KSAM/
├── agent/              # Kubernetes agent (collects data)
│   ├── cmd/           # Main application
│   ├── internal/      # Internal packages
│   └── deploy/        # Kubernetes manifests
├── core/              # Core API controller
│   ├── cmd/           # Main application
│   ├── internal/      # Internal packages
│   ├── migrations/    # Database migrations
│   └── pkg/           # Public packages
├── dashboard/         # Frontend dashboard
│   ├── src/           # React source code
│   ├── public/        # Static assets
│   └── dist/          # Build output
├── docs/              # Documentation
├── scripts/           # Utility scripts
└── helm/              # Helm charts
```

### Technologies

**Backend:**
- Go 1.20
- Gin (HTTP framework)
- GORM (ORM)
- PostgreSQL
- gRPC

**Frontend:**
- React 18
- TypeScript
- Tailwind CSS
- React Query
- React Router
- Axios

**Infrastructure:**
- Kubernetes
- Docker
- Minikube (dev)

## 🧪 Testing

### Create Test ServiceAccounts
```bash
kubectl create serviceaccount test-sa -n default
kubectl label serviceaccount test-sa env=test -n default
kubectl delete serviceaccount test-sa -n default
```

### Check Audit Logs
```bash
# Via API
curl http://localhost:8080/api/v1/audit -H "Authorization: Bearer <token>"

# Via Database
kubectl exec -n ksam postgres-xxx -- psql -U postgres -d ksam -c "SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 10;"
```

### View Core Logs
```bash
kubectl logs -n ksam -l app=ksam-core -f
```

### View Agent Logs
```bash
kubectl logs -n kube-system -l app=ksam-agent -f
```

## 📊 Monitoring

### Check System Status
```bash
kubectl get pods -n ksam
kubectl get pods -n kube-system -l app=ksam-agent
```

### View Metrics
```bash
# Core metrics
curl http://localhost:8080/metrics

# Health checks
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

## 🔧 Configuration

### Environment Variables

**Core Controller:**
- `DATABASE_URL`: PostgreSQL connection string
- `JWT_SECRET`: Secret for JWT token generation
- `HTTP_PORT`: HTTP server port (default: 8080)
- `GRPC_PORT`: gRPC server port (default: 9090)

**Agent:**
- `CORE_CONTROLLER_URL`: Core API endpoint
- `CLUSTER_ID`: Kubernetes cluster identifier
- `SYNC_INTERVAL`: Data sync interval (default: 30s)

## 🤝 Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## 📝 License

MIT License - see LICENSE file for details

## 🙏 Acknowledgments

- Kubernetes community
- Go community
- React community

## 📧 Contact

For questions or support, please open an issue on GitHub.

---

**K8s Workload Management Platform** - Simplifying Kubernetes workload management
