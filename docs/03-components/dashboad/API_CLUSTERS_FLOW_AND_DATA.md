# API /api/v1/clusters – Luồng xử lý và dữ liệu thực tế từ K8s

**Cập nhật**: 2026-01-31

---

## 1. Điểm vào: `http://localhost:8081/api/v1/clusters`

- **8081**: Port của Dashboard (khi port-forward `kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80`).
- Dashboard dùng Nginx proxy mọi request `/api/` tới Core:

```nginx
# dashboard/nginx.conf
location /api/ {
  set $upstream_core fortuna-core.fortuna.svc.cluster.local;
  proxy_pass http://$upstream_core:8080;
  ...
}
```

- Request từ browser: `GET http://localhost:8081/api/v1/clusters` → Nginx chuyển thành `GET http://fortuna-core.fortuna.svc.cluster.local:8080/api/v1/clusters` (giữ nguyên path).
- **Lưu ý**: Core bật auth (AUTH_ENABLED=true); request từ Dashboard đã gửi kèm header `Authorization: Bearer <JWT>` (sau khi user đăng nhập). Nếu gọi trực tiếp bằng curl từ ngoài mà không có JWT sẽ nhận 401.

---

## 2. Core: handler và nguồn dữ liệu

### 2.1 Route và handler

- **Route**: `v1.GET("/clusters", GetClusters(db))` (core/internal/api/routes.go). Nhóm `/api/v1` dùng `AuthMiddleware` khi AUTH_ENABLED=true.
- **Handler**: `GetClusters(db)` trong `core/internal/api/handlers.go`.

### 2.2 Logic trong `GetClusters`

- Đọc bảng **`clusters`** (model `models.Cluster`).
- **Lọc “cluster đang hoạt động”**: mặc định chỉ lấy cluster có `last_sync >= (now - 7 ngày)` (hằng số `ActiveClusterCutoff = 7 * 24 * time.Hour`). Query thêm điều kiện `WHERE last_sync >= ?` trừ khi gọi với `?includeStale=true` (dành cho admin).
- Sắp xếp: `ORDER BY last_sync DESC`.
- Trả về JSON: `{"clusters": [ ... ]}` (mảng các cluster).

### 2.3 Model `Cluster` (bảng `clusters`)

- `id` (string, PK): định danh cluster, trùng với `cluster_id` do Agent gửi.
- `name` (string): tên hiển thị; Core dùng `clusterName` từ Agent, nếu rỗng thì dùng `clusterID`.
- `region`, `endpoint`: có trong model, thường rỗng nếu Agent không gửi.
- `status`: ví dụ `"active"`.
- `last_sync`: thời điểm sync gần nhất từ Agent (cập nhật mỗi lần sync).
- `created_at`, `updated_at`, `deleted_at` (soft delete).

Dữ liệu **không** lấy trực tiếp từ Kubernetes API tại thời điểm gọi `/clusters`. Toàn bộ đến từ bảng `clusters` do Core ghi khi xử lý sync từ Agent.

---

## 3. Dữ liệu từ K8s: Agent → Core → DB

### 3.1 Ai ghi vào bảng `clusters`?

- **Agent** (chạy trên từng node trong cluster) định kỳ gửi sync (HTTP/gRPC) lên **Core** với payload chứa `clusterId` và `clusterName`.
- Core nhận sync trong `AgentService.SyncData(clusterID, clusterName, data)` (core/internal/service/agent_service.go):
  - Tạo hoặc cập nhật một dòng trong `clusters`: `ID = clusterID`, `Name = clusterName` (hoặc fallback `clusterID`), `Status = "active"`, `LastSync = time.Now()`.
  - Các bảng khác (pods, service_accounts, roles, …) cũng được cập nhật theo payload và gắn với `cluster_id` này.

### 3.2 Agent lấy `clusterId` / `clusterName` từ đâu?

- **Cấu hình Agent** (agent/internal/config, agent/cmd/main.go):
  - `CLUSTER_ID`, `CLUSTER_NAME`: từ biến môi trường (nếu set).
  - Nếu có `KUBECONFIG`: đọc **tên cluster trong kubeconfig** bằng `k8s.GetClusterName(kubeconfigPath)`.
- **GetClusterName** (agent/internal/k8s/client.go):
  - Đọc file kubeconfig, lấy `CurrentContext` → `Contexts[ctx].Cluster`.
  - Giá trị này chính là tên cluster mà `kubectl config get-clusters` hiển thị (ví dụ `minikube`, `kubernetes`).
- Nếu không set env và không đọc được kubeconfig, Agent dùng `"unknown"` cho cả id và name.

Kết luận: **dữ liệu “cluster” mà API `/api/v1/clusters` trả về thực chất là “cluster đã từng được Agent sync”, với id/name xuất phát từ cấu hình và kubeconfig của cluster đó (K8s context), không phải từ một lời gọi Kubernetes API tại thời điểm request.

---

## 4. Kiểm tra thực tế (ví dụ)

### 4.1 Response API (Core, có JWT)

```json
{
  "clusters": [
    {
      "id": "minikube",
      "name": "minikube",
      "region": "",
      "endpoint": "",
      "status": "active",
      "lastSync": "2026-01-25T08:57:11.465528Z",
      "createdAt": "2026-01-25T08:57:11.465528Z",
      "updatedAt": "2026-01-25T08:57:11.465528Z"
    }
  ]
}
```

### 4.2 Bảng `clusters` trong Postgres

```sql
SELECT id, name, status, last_sync, created_at FROM clusters ORDER BY last_sync DESC;
```

Ví dụ kết quả (khi Agent có `CLUSTER_ID=kubernetes`, `CLUSTER_NAME=kubernetes`):

| id        | name      | status | last_sync              |
|-----------|-----------|--------|------------------------|
| kubernetes| kubernetes| active | 2026-01-31 13:56:06... |

- Một cluster trong DB tương ứng một cluster mà ít nhất một Agent (với cluster id/name trùng) đã sync lên Core.
- `id`/`name` lấy từ env Agent (`CLUSTER_ID`, `CLUSTER_NAME`) hoặc từ kubeconfig (`context.cluster`) khi env không set. Deploy mặc định dùng `kubernetes` để khớp cluster thực tế (kubeadm/standard).

### 4.3 Dashboard (frontend)

- Dashboard gọi `getClusters()` (dashboard/lib/api.ts): `request('/clusters')` → `GET /api/v1/clusters` (qua proxy tới Core), gửi kèm JWT lưu trong auth store.
- Map response: `data.clusters` → kiểu `Cluster[]` (id, name, status, lastSync, …). Trang Clusters và Dashboard hiển thị từ mảng này.

---

## 5. Tóm tắt luồng dữ liệu

1. **K8s / kubeconfig**: Tên cluster (ví dụ `minikube`) nằm trong kubeconfig mà Agent dùng (`context.cluster`).
2. **Agent**: Đọc cluster id/name từ env hoặc kubeconfig, định kỳ gửi sync lên Core kèm `clusterId`, `clusterName` và dữ liệu pods/RBAC/…
3. **Core**: Trong `SyncData` cập nhật (hoặc tạo) bản ghi trong bảng `clusters` với `id`, `name`, `last_sync`, `status`.
4. **API GET /api/v1/clusters**: Chỉ đọc từ bảng `clusters` (lọc theo `last_sync`, có hỗ trợ `includeStale=true`), trả về JSON.
5. **Dashboard**: Proxy request từ `http://localhost:8081/api/v1/clusters` tới Core, gửi JWT; frontend dùng response để hiển thị danh sách cluster.

Không có bước nào gọi trực tiếp Kubernetes API (list clusters, v.v.) khi xử lý `GET /api/v1/clusters`; toàn bộ dữ liệu cluster hiển thị đều xuất phát từ lịch sử sync của Agent và bảng `clusters` trong DB.
