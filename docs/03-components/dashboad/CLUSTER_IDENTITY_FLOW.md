# Cluster Identity Flow – Auto-discovery & SSOT

**Cập nhật**: 2026-01-31

---

## Luồng chuẩn (happy path)

```
Kubernetes API
   ↓
Agent (auto-discovery)
   ↓
Core (normalize + deduplicate, SSOT)
   ↓
Database (cluster table)
   ↓
API
   ↓
Dashboard
```

---

## 1. AGENT: Tự động discover cluster metadata

### 1.1 Nguồn cluster identity (không hardcode)

**Ưu tiên:**

1. **Env override (optional)**  
   Nếu `CLUSTER_ID` được set (ConfigMap/env) → dùng giá trị env, log `[cluster] source=env_override`.  
   `CLUSTER_NAME` tùy chọn; nếu không set thì dùng `CLUSTER_ID`.

2. **Auto-discovery (primary)**  
   Nếu không set env:
   - **cluster_id (stable):** `sha256(kube-system namespace UID)` → prefix `sha256-` + 16 ký tự hex.
   - **cluster_name:** từ kubeconfig `context.cluster` (nếu có), nếu không thì `inferred-k8s-cluster`.
   - **k8s_version:** từ K8s API `GET /version` (ServerVersion).
   - **distribution:** suy từ version string: `eks` | `gke` | `aks` | `k3s` | `kubeadm` | `unknown`.

**Không dùng:** node name, pod UID, giá trị cố định (ví dụ `"kubernetes"`) làm cluster identity.

### 1.2 Log bắt buộc trong agent

- `[cluster] id=xxx source=auto name=... k8s_version=... distribution=...`
- Hoặc `[cluster] id=xxx source=env_override name=... (CLUSTER_ID set)`

### 1.3 Contract agent → core (sync payload)

```json
{
  "cluster": {
    "id": "sha256-xxxx",
    "name": "prod-cluster",
    "source": "auto|env",
    "k8s_version": "v1.29.1",
    "distribution": "eks|gke|aks|kubeadm|unknown"
  },
  "clusterId": "sha256-xxxx",
  "clusterName": "prod-cluster",
  "data": { ... }
}
```

- Core chấp nhận cả object `cluster` (ưu tiên) hoặc chỉ `clusterId`/`clusterName` (backward compat).

---

## 2. CORE: Single source of truth (SSOT)

### 2.1 Logic khi nhận cluster info

- **cluster_id chưa có trong DB** → **tạo mới** cluster (id, name, source, k8s_version, distribution, status, last_sync).
- **cluster_id đã tồn tại** → **chỉ cập nhật** các trường mutable: name, source, k8s_version, distribution, status, last_sync. **Không** đổi id, **không** tạo cluster mới.

**Không** tạo cluster mới chỉ vì: name khác, agent restart, namespace khác.

### 2.2 Quy tắc dữ liệu

| Trường           | Immutable | Ghi chú                                |
|------------------|-----------|----------------------------------------|
| id               | ✅        | Primary key, không đổi                 |
| name             | ❌        | Cập nhật mỗi sync                      |
| source           | ❌        | `auto` \| `env`                        |
| k8s_version      | ❌        | Từ agent                               |
| distribution     | ❌        | Từ agent                               |
| last_sync        | ❌        | Cập nhật mỗi sync/heartbeat            |

- N agent → 1 cluster (cùng cluster_id).

---

## 3. Database model (minimal)

Bảng `clusters` có ít nhất:

- `id` (VARCHAR, PK)
- `name` (VARCHAR)
- `source` (VARCHAR, nullable)
- `k8s_version` (VARCHAR, nullable)
- `distribution` (VARCHAR, nullable)
- `status`, `last_sync`, `created_at`, `updated_at`, `deleted_at`

Migration 054 thêm `source`, `k8s_version`, `distribution`.

---

## 4. API & Database (không trùng lặp)

- **Nguồn dữ liệu:** Chỉ DB. Không merge từ agent runtime.
- **Endpoints:**
  - `GET /api/v1/clusters` – danh sách cluster (active theo `last_sync >= cutoff`).
  - `GET /api/v1/clusters/stats` – cùng danh sách + thống kê (dùng chung helper `getClustersForAPI`).
  - `GET /api/v1/clusters/:id` – chi tiết theo cluster_id.
- **Định nghĩa "active cluster":** `ActiveClusterCutoff = 7 * 24 * time.Hour` (một hằng số duy nhất).
  - Dashboard stats, metrics (`resources.clusters`), GetClusters, GetClustersStats đều dùng cutoff này → số cluster active thống nhất.
- **Tạo/cập nhật cluster:** Chỉ tại `AgentService.SyncData` (SSOT); không có path tạo cluster trùng lặp.

---

## 5. Dashboard (đã cập nhật theo logic SSOT)

- **Nguồn:** Chỉ từ API (`GET /clusters`, `GET /clusters/stats`, `GET /dashboard/stats`), không fallback cứng. Trang Clusters gọi `/clusters/stats`; home dùng `/dashboard/stats` + `getClusters()` (map k8sVersion, source, distribution). Type Cluster có connectionStatus, podCount, deploymentCount; hiển thị `name || id`.
- Hiển thị đúng cluster từ API (id + name từ DB).
- Nếu 1 cluster: hiển thị rõ tên + ID.
- Nếu >1 cluster: bắt buộc selector.
- Không dùng default cluster cố định, không fallback name “đỡ trống”.

---

## 6. Verification checklist (agent)

Sau khi implement, tự verify:

1. **Restart agent** → cluster_id không đổi (vẫn cùng hash từ kube-system UID).
2. **Scale agent replicas** → vẫn 1 cluster duy nhất (cùng cluster_id).
3. **Override env** (CLUSTER_ID/CLUSTER_NAME) → cluster_name thay đổi theo env, cluster_id theo env; log `source=env_override`.
4. **Xóa ConfigMap/env override** → agent fallback auto-discovery, log `source=auto`.
5. **Dashboard** → hiển thị đúng cluster đang chạy (id/name từ DB).

---

## 7. Deploy: optional override

Không set cứng CLUSTER_ID/CLUSTER_NAME trong manifest. Mặc định agent auto-discover.

**Override tùy chọn** (ví dụ ConfigMap):

```yaml
# Optional: create ConfigMap fortuna-cluster-config with cluster_id, cluster_name
env:
  - name: CLUSTER_ID
    valueFrom:
      configMapKeyRef:
        name: fortuna-cluster-config
        key: cluster_id
        optional: true
  - name: CLUSTER_NAME
    valueFrom:
      configMapKeyRef:
        name: fortuna-cluster-config
        key: cluster_name
        optional: true
```

Nếu ConfigMap không tồn tại hoặc key trống, agent dùng auto-discovery.

---

## 8. Deployments & migrations (kiểm tra với luồng dữ liệu)

### 8.1 Deployments

| File | Cluster-related | Khớp luồng |
|------|-----------------|------------|
| **deploy/fortuna-agent-daemonset.yaml** | Không hardcode CLUSTER_ID/CLUSTER_NAME; optional ConfigMap commented | ✅ Auto-discovery mặc định |
| **deploy/agent-daemonset.yaml** | Không set cluster env | ✅ Auto-discovery |
| **deploy/fortuna-core-deployment.yaml** | Không cần cluster env (Core nhận từ Agent) | ✅ |
| **deploy/dashboard-deployment.yaml** | Không cluster env; Dashboard đọc từ API | ✅ |
| **helm/fortuna/templates/agent-daemonset.yaml** | Dùng `.Values.agent.env`; có thể set CLUSTER_ID/CLUSTER_NAME qua values | ✅ Optional override |
| **helm/fortuna/values.yaml** | `agent.env`: LOG_LEVEL, SYNC_INTERVAL; comment CLUSTER_ID/CLUSTER_NAME | ✅ |
| **helm/ksam/templates/agent-daemonset.yaml** | CORE_GRPC_ENDPOINT, CORE_HTTP_ENDPOINT, SYNC_INTERVAL, NODE_NAME; **không** set CLUSTER_ID=nodeName | ✅ Đã sửa (trước đây FORTUNA_CLUSTER_ID=nodeName sai luồng) |

**Lưu ý:** Agent cần env `CORE_GRPC_ENDPOINT`, `CORE_HTTP_ENDPOINT`, `SYNC_INTERVAL`. Helm KSAM đã dùng đúng tên; không set cluster identity theo node name.

### 8.2 Migrations (thứ tự và ý nghĩa với luồng cluster)

| Migration | Mục đích | Liên quan cluster |
|-----------|----------|-------------------|
| **001** | Bảng `clusters` (id PK, name NOT NULL UNIQUE, …) | Schema gốc; **055** bỏ UNIQUE(name) |
| **039** | Thêm `deleted_at` cho clusters | Soft delete |
| **053** | Đổi tên hiển thị cluster (optional): env `CLUSTER_ID_TO_UPDATE`, `CLUSTER_DISPLAY_NAME` | Một lần; Core không set mặc định → no-op nếu không set env |
| **054** | Thêm `source`, `k8s_version`, `distribution` | SSOT metadata từ agent |
| **055** | Bỏ UNIQUE trên `clusters(name)` | Cho phép nhiều cluster cùng display name (id là identity) |

**Luồng sau khi deploy:** Agent (auto-discovery hoặc env override) → sync lên Core → Core tạo/cập nhật cluster (SSOT) → API/Dashboard đọc từ DB. Migrations 054 + 055 đảm bảo schema hỗ trợ đúng luồng này.
