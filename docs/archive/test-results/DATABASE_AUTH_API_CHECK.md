# Kiểm tra Database, Migrations, Auth và API (curl)

**Ngày**: 2026-02-22

---

## 1. Trạng thái Database

### 1.1 PostgreSQL

- **Pod**: `postgres-5484f7745d-gsr4f` (Running, namespace fortuna)
- **Service**: `postgres.fortuna.svc.cluster.local:5432`
- **Database**: `fortuna`, user `postgres`

### 1.2 Bảng trong DB

```sql
\dt  -- 37 bảng
```

Các bảng chính: `agents`, `audit_logs`, `capability_metadata`, `cluster_role_bindings`, `cluster_roles`, `clusters`, `cve_file_metadata`, `cve_matches`, `cves`, `deployments`, `error_logs`, `events_index`, `image_scan_results`, `insights`, `namespaces`, `nodes`, `notifications`, `package_vulnerabilities`, `pod_attack_steps`, `pod_capabilities`, `pod_image_scans`, `pod_instances`, `pod_risk_profiles`, `pods`, `policies`, `policy_templates`, `promotion_rules`, `replicasets`, `risk_scores`, `role_bindings`, `roles`, `runtime_events`, `runtime_signals`, `sbom_components`, `sboms`, `service_accounts`, `users`.

### 1.3 Schema `clusters` (migration 062)

Bảng `clusters` có đủ cột: `id`, `name`, **`region`**, **`endpoint`**, **`kubeconfig`**, `status`, `last_sync`, `created_at`, `updated_at`, `deleted_at`, `source`, `k8s_version`, `distribution` → **migration 062 đã chạy**.

### 1.4 Bảng `users` và admin

- Ban đầu: bảng `users` thiếu cột `deleted_at` → Login trả về lỗi `column users.deleted_at does not exist`.
- **Khắc phục**: `ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE; CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);`
- Sau khi restart Core: **CreateDefaultAdmin** (env `FORTUNA_ADMIN_USERNAME=admin`, `FORTUNA_ADMIN_PASSWORD=admin123`) đã tạo user admin.
- **Kết quả**: `SELECT id, username, role, active FROM users;` → 1 row: `admin`, role `admin`, active `true`.

---

## 2. Migrations

- Core chạy migrations khi khởi động (theo `core/migrations/migrations.go`).
- Sự tồn tại của 37 bảng và cột `region`/`endpoint`/`kubeconfig` trong `clusters` cho thấy **các migration đã chạy** (gồm 062).
- Migration 037 không thêm `deleted_at` cho bảng `users`; model GORM `User` có `DeletedAt` nên đã bổ sung cột thủ công (xem trên).

---

## 3. Authentication

### 3.1 Login

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

**Kết quả**: HTTP 200, trả về JWT và thông tin user:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": 1,
    "username": "admin",
    "email": "admin@fortuna.local",
    "role": "admin",
    "active": true,
    "lastLogin": "2026-02-22T13:04:33.341217918Z",
    ...
  },
  "expiresAt": "2026-02-23T13:04:33.346333671Z"
}
```

### 3.2 Health (không cần auth)

```bash
curl -s http://localhost:8080/health
```

**Kết quả**: `{"status":"healthy","timestamp":"...","checks":{"database":"ok"}}`

---

## 4. Test curl các API (với Bearer token)

Token lấy từ response login, gửi header: `Authorization: Bearer <token>`.

| API | Method | Kết quả |
|-----|--------|--------|
| `/health` | GET | `{"status":"healthy","checks":{"database":"ok"}}` |
| `/api/v1/auth/login` | POST | JWT + user (admin) |
| `/api/v1/dashboard/stats` | GET | `{"totalClusters":0,"activeAgents":0,"runningPods":0,"totalRisks":0,"criticalRisks":0,"resolved24h":0,"affectedPodCount":0}` |
| `/api/v1/agents/status` | GET | `{"agents":[],"disconnected":0,"healthy":0,"slow":0,"total":0}` |
| `/api/v1/clusters` | GET | `{"clusters":[]}` |
| `/api/v1/insights/summary` | GET | `{"total":0,"critical":0,"high":0,"medium":0,"low":0,"byType":{}}` |

Các API đều trả về HTTP 200 và JSON hợp lệ. Giá trị 0/rỗng là bình thường khi chưa có cluster/agent/insight trong DB (agent sync sẽ điền dần).

---

## 5. Lệnh kiểm tra nhanh (trong cluster)

```bash
# Login và lưu token
TOKEN=$(kubectl exec -n fortuna deploy/fortuna-core -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r .token)

# Gọi API (từ trong pod Core)
kubectl exec -n fortuna deploy/fortuna-core -- curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/dashboard/stats
kubectl exec -n fortuna deploy/fortuna-core -- curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/agents/status
kubectl exec -n fortuna deploy/fortuna-core -- curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/clusters
```

Từ máy trạm (sau khi port-forward):

```bash
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'
curl -s -H "Authorization: Bearer <token>" http://localhost:8080/api/v1/dashboard/stats
```

---

## 6. Tóm tắt

| Hạng mục | Trạng thái |
|----------|------------|
| Database (Postgres) | ✅ Running, 37 bảng |
| Migrations | ✅ Đã chạy (gồm 062: clusters.region/endpoint/kubeconfig) |
| Bảng `users` | ✅ Có cột `deleted_at` (đã bổ sung); có user `admin` |
| Auth (login) | ✅ admin / admin123 → JWT |
| API dashboard/stats | ✅ 200, JSON hợp lệ |
| API agents/status | ✅ 200, JSON hợp lệ |
| API clusters | ✅ 200, JSON hợp lệ |
| API insights/summary | ✅ 200, JSON hợp lệ |
