# Build and Deploy Fortuna with Containerd

*Updated: 2026-04-13*

## 1. Build and Deploy Architecture

- **Runtime**: Kubernetes uses **containerd** (CRI), namespace `k8s.io`.
- **Build**: Auto-detects **nerdctl**, **docker**, or **buildctl**. Override: `BUILD_TOOL=docker|nerdctl|buildctl`.
- **Deploy**: **kubectl apply** manifests in `deploy/` (postgres, nats, RBAC, core, agent, dashboard).

## 2. Requirements

- One of: `nerdctl` + buildkitd, `docker`, or `buildctl` + buildkitd
- `ctr` (containerd CLI) — for importing Docker-built images into containerd
- `kubectl`
- Containerd socket: `/run/containerd/containerd.sock` or `/var/run/containerd/containerd.sock`
- Kubernetes cluster (minikube, kubeadm, …) using containerd as CRI
- **No npm/Node.js needed on host**: Dashboard builds inside Dockerfile (container).

## 3. Build (auto-detect: nerdctl / docker / buildctl)

From repo root:

```bash
# Build core, agent, dashboard and load into containerd (namespace k8s.io)
./scripts/build/build-and-load-containerd.sh

# Force Docker backend (when buildkitd is not running)
BUILD_TOOL=docker ./scripts/build/build-and-load-containerd.sh

# Build core + agent only (skip dashboard)
SKIP_DASHBOARD=true ./scripts/build/build-and-load-containerd.sh

# Build without cache
NO_CACHE=true ./scripts/build/build-and-load-containerd.sh
```

Images after build: `fortuna-core:latest`, `fortuna-agent:latest`, `fortuna-dashboard:latest` (and `VERSION` tag if set).
Deployment YAMLs use `imagePullPolicy: Never` for local images.

### Image Digest Consistency (master + worker)

- `scripts/build/build-and-load-containerd.sh` syncs short tag `fortuna-*:<tag>` with canonical ref `docker.io/library/fortuna-*:<tag>`.
- `scripts/utils/push-images-to-workers.sh` defaults `VERIFY_REMOTE_DIGEST=true` and fails fast when remote digest doesn't match local.

Recommended workflow:

```bash
# 1) Build locally
./scripts/build/build-and-load-containerd.sh

# 2) Push to all nodes with digest verification
./scripts/utils/push-images-to-workers.sh

# 3) Rollout workloads
kubectl -n fortuna rollout restart deployment/fortuna-core
kubectl -n fortuna rollout restart daemonset/fortuna-agent
```

Kiểm tra nhanh imageID sau rollout:

```bash
kubectl get pods -n fortuna -l app.kubernetes.io/component=agent -o jsonpath='{range .items[*]}{.metadata.name}{"|"}{.spec.nodeName}{"|"}{.status.containerStatuses[0].imageID}{"\n"}{end}'
kubectl get pods -n fortuna -l app.kubernetes.io/component=core -o jsonpath='{range .items[*]}{.metadata.name}{"|"}{.spec.nodeName}{"|"}{.status.containerStatuses[0].imageID}{"\n"}{end}'
```

Nếu cần bỏ verify digest tạm thời (không khuyến nghị):

```bash
VERIFY_REMOTE_DIGEST=false ./scripts/utils/push-images-to-workers.sh
```

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
