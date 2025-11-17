# KSAM Agent

KSAM Agent là một lightweight DaemonSet được deploy trong mỗi Kubernetes cluster để thu thập và theo dõi ServiceAccounts, RoleBindings, và ClusterRoleBindings.

## Chức năng

- Thu thập ServiceAccounts, RoleBindings, ClusterRoleBindings từ cluster
- Theo dõi real-time changes qua Kubernetes Watch API
- Gửi dữ liệu về KSAM Core Controller qua gRPC
- Hoạt động với least privilege - chỉ cần read-only access

## Cấu trúc

```
agent/
├── cmd/
│   └── main.go           # Entry point
├── internal/
│   ├── collector/        # Thu thập dữ liệu từ K8s API
│   ├── watcher/          # Watch API cho real-time updates
│   ├── k8s/              # Kubernetes client setup
│   └── config/           # Configuration management
├── pkg/
│   └── types/            # Shared types
├── deploy/
│   ├── rbac.yaml         # RBAC permissions cho Agent
│   └── daemonset.yaml    # DaemonSet manifest
├── Dockerfile
├── go.mod
└── go.sum
```

## Build

```bash
go build -o bin/ksam-agent ./cmd
```

## Deploy

### 1. Deploy RBAC

```bash
kubectl apply -f deploy/rbac.yaml
```

### 2. Deploy DaemonSet

```bash
kubectl apply -f deploy/daemonset.yaml
```

## Configuration

Agent được cấu hình qua environment variables hoặc ConfigMap:

- `KSAM_CORE_ENDPOINT`: Endpoint của KSAM Core Controller (default: `http://ksam-core:8080`)
- `KSAM_CLUSTER_ID`: Unique identifier cho cluster (default: node name)
- `KSAM_AUTH_TOKEN`: Authentication token (optional)
- `KSAM_SYNC_INTERVAL`: Interval để sync dữ liệu (default: `30s`)
- `KSAM_KUBECONFIG`: Path to kubeconfig file (optional, uses in-cluster config if empty)
- `KSAM_WATCH_NAMESPACE`: Namespace to watch (empty = all namespaces)

## Local Development

### Prerequisites

- Go 1.21+
- kubectl configured
- Access to a Kubernetes cluster

### Run locally

```bash
# Set environment variables
export KSAM_CORE_ENDPOINT=http://localhost:8080
export KSAM_CLUSTER_ID=local-cluster
export KSAM_SYNC_INTERVAL=30s

# Run agent
go run cmd/main.go
```

### Test with local cluster

```bash
# Use minikube or kind
minikube start

# Set kubeconfig
export KSAM_KUBECONFIG=~/.kube/config

# Run agent
go run cmd/main.go
```

## Data Collection

Agent thu thập các loại dữ liệu sau:

1. **ServiceAccounts**: Tất cả ServiceAccounts trong cluster
2. **RoleBindings**: Tất cả RoleBindings (namespace-scoped)
3. **ClusterRoleBindings**: Tất cả ClusterRoleBindings (cluster-scoped)
4. **Roles**: Tất cả Roles (namespace-scoped)
5. **ClusterRoles**: Tất cả ClusterRoles (cluster-scoped)
6. **Pods**: Pods để xác định ServiceAccounts đang được sử dụng

## Real-time Watching

Agent sử dụng Kubernetes Informers để theo dõi real-time changes:

- ServiceAccount add/update/delete events
- RoleBinding add/update/delete events
- ClusterRoleBinding add/update/delete events
- Role add/update/delete events
- ClusterRole add/update/delete events

## Security

Agent chỉ cần read-only permissions:

- `get`, `list`, `watch` trên `serviceaccounts`, `pods`
- `get`, `list`, `watch` trên `roles`, `rolebindings`, `clusterroles`, `clusterrolebindings`

Xem `deploy/rbac.yaml` để biết chi tiết.

## Troubleshooting

### Check Agent logs

```bash
kubectl logs -l app=ksam-agent -n kube-system
```

### Verify RBAC permissions

```bash
kubectl auth can-i list serviceaccounts --as=system:serviceaccount:kube-system:ksam-agent
```

### Check connectivity to Core

```bash
kubectl exec -it <agent-pod> -n kube-system -- wget -O- http://ksam-core:8080/health
```
