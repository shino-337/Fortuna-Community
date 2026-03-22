# Build và Deploy Fortuna với Containerd / nerdctl

*Cập nhật: 2026-02-02*

## 1. Cơ chế build và deploy

- **Runtime**: Kubernetes dùng **containerd** (CRI), namespace `k8s.io`.
- **Build**: **nerdctl** build image và load vào containerd (không cần Docker daemon).
- **Deploy**: **kubectl apply** các file trong `deploy/` (postgres, nats, RBAC, core, agent, dashboard).

## 2. Yêu cầu

- `nerdctl`, `ctr` (containerd), `go`, `kubectl`
- Containerd socket: `/run/containerd/containerd.sock` hoặc `/var/run/containerd/containerd.sock`
- Cluster Kubernetes (minikube, kubeadm, …) dùng containerd làm CRI
- **Không cần npm/Node.js trên host**: Dashboard build trong Dockerfile (trong container).

## 3. Build (nerdctl → containerd)

Từ thư mục gốc repo:

```bash
# Build core, agent, dashboard và load vào containerd (namespace k8s.io)
./scripts/build/build-and-load-containerd.sh

# Chỉ build core + agent (bỏ qua dashboard)
SKIP_DASHBOARD=true ./scripts/build/build-and-load-containerd.sh

# Build không dùng cache
NO_CACHE=true ./scripts/build/build-and-load-containerd.sh
```

Image sau build: `fortuna-core:latest`, `fortuna-agent:latest`, `fortuna-dashboard:latest` (và tag `VERSION` nếu set).  
Deployment YAML dùng `imagePullPolicy: Never` để dùng image local.

## 4. Deploy

```bash
# Deploy đầy đủ (namespace, postgres, nats, RBAC, core, agent, dashboard)
./scripts/deploy/deploy-fortuna-robust.sh
```

Hoặc apply thủ công:

```bash
kubectl create namespace fortuna --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml   # hoặc postgresql.yaml
kubectl apply -f deploy/infrastructure/nats.yaml
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/fortuna-core-deployment.yaml   # Service + Deployment
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
kubectl apply -f deploy/dashboard-nginx-configmap.yaml
kubectl apply -f deploy/dashboard-deployment.yaml
```

## 5. Clean toàn bộ rồi rebuild và deploy lại

Script tổng hợp: clean image cũ (nerdctl), tùy chọn clean DB, rebuild (nerdctl), deploy:

```bash
# Clean images + rebuild + deploy (không đụng DB)
./scripts/pipeline/full-clean-database-rebuild-deploy.sh

# Thêm: xóa dữ liệu DB (DELETE, giữ schema) rồi rebuild + deploy
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db

# Thêm: full reset DB (DROP tables; Core chạy lại migrations khi start) – dùng cẩn thận
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset

# Chỉ clean + deploy (dùng lại image hiện có)
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --skip-rebuild

# Chỉ clean + rebuild (không deploy)
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --skip-deploy
```

## 6. File deployment chính

| File | Mô tả |
|------|--------|
| `deploy/fortuna-core-deployment.yaml` | Service + Deployment Core (HTTP 8080, gRPC 9090) |
| `deploy/fortuna-agent-daemonset.yaml` | DaemonSet Agent (NODE_NAME, CORE_GRPC_ENDPOINT, mTLS, containerd socket) |
| `deploy/dashboard-deployment.yaml` | Deployment + Service Dashboard |
| `deploy/dashboard-nginx-configmap.yaml` | ConfigMap Nginx cho dashboard |
| `deploy/fortuna-rbac.yaml` | RBAC (ServiceAccount, Role, RoleBinding) cho core/agent |
| `deploy/infrastructure/postgresql.yaml` hoặc `postgresql-with-age.yaml` | Postgres |
| `deploy/infrastructure/nats.yaml` | NATS |

## 7. Dashboard không hiển thị Agent

Xem phân tích chi tiết: **`docs/DASHBOARD_AGENT_DISPLAY_ANALYSIS.md`**.

Tóm tắt: API `/api/v1/agents/status` chỉ trả về agent có `last_seen_at` trong 10 phút. Core (code mới) cập nhật `last_seen_at` khi nhận Ping có `agent_id`; Agent (code mới) gửi `AgentId` trong Ping. Cần **build lại image Core và Agent** (script trên) và **deploy lại**; sau 1–2 phút Dashboard sẽ thấy agent.

## 8. Kiểm tra nhanh

```bash
kubectl get pods -n fortuna
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=30
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=30
# API agents (sau khi login lấy JWT)
curl -s -H "Authorization: Bearer <JWT>" http://localhost:8080/api/v1/agents/status
```

Port-forward để test từ máy local:

```bash
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080
kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80
```
