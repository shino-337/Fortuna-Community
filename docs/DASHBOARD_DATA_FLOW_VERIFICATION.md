# Dashboard Data Flow Verification (No Mock/Seed)

Kiểm tra lần cuối: mọi dữ liệu trên dashboard đều từ API → Core DB → Agent sync. Không dùng seed hay mock.

## Resources (Pod tab)

| UI Field   | API (dashboard)     | Core endpoint        | Core source              | Agent sync                    |
|------------|---------------------|----------------------|---------------------------|-------------------------------|
| Name       | `getPods()`         | GET /api/v1/pods     | `models.Pod` from DB      | `PodPayload.Name`             |
| Namespace  | ✓                   | ✓                    | `pod.Namespace`           | `PodPayload.Namespace`        |
| Node       | ✓                   | ✓                    | `pod.NodeName`            | `PodPayload.NodeName`         |
| **Status** | `pod.status` ← `p.phase` | Pod embedded in JSON | `pod.Phase` (column `phase`) | `PodPayload.Phase` = `p.Status.Phase` |
| Risk       | `pod.riskCount`     | riskCount from insights | COUNT(insights) WHERE resource_uid=pod.UID | N/A (core computes)   |
| Created    | `pod.createdAt`     | Pod.CreatedAt        | DB created_at             | N/A (DB on create)            |

- **Agent:** `agent/internal/syncer/syncer.go`: `PodPayload.Phase`, set `phase := string(p.Status.Phase)`.
- **Core:** `core/internal/service/agent_service.go`: read `phase` from payload, set `pod.Phase`, persist in create/update/restore; `core/pkg/models/models.go`: `Pod.Phase` json `"phase"`; `core/internal/api/handlers.go`: GetPods/GetPod/GetPodByUID return embedded `models.Pod` (includes phase, createdAt) + riskCount.
- **Migration:** `core/migrations/066_add_pods_phase.go` adds `pods.phase`; registered in `migrations.go`.

## Resources (ServiceAccount / Role / RoleBinding tabs)

| Source | API (dashboard)   | Core endpoint        | Core source        | Agent sync     |
|--------|-------------------|----------------------|--------------------|----------------|
| Data   | `getResources(kind)` | GET /api/v1/resources?kind= | GetResources() from DB | processSynced* |

- `core/internal/api/resources_handlers.go`: GetResources reads Pod, ServiceAccount, Role, ClusterRole, RoleBinding, ClusterRoleBinding from DB (all from agent sync).

## Pod Detail page

| UI Field   | API                | Core / Agent same as above |
|------------|--------------------|----------------------------|
| Status     | `getPod` / `getPodByUid` → `status` from `phase` | ✓ |
| Risk Count | riskCount          | ✓ |
| Service Account | pod.serviceAccount | ✓ |
| Created    | pod.createdAt      | ✓ |
| SBOM       | getPodSbom(pod.uid) | GET /api/v1/sbom/... (DB from agent scan) |
| Risks      | getPodRiskReport(pod.uid) | GET /api/v1/risks/pods/:uid/report (insights from DB) |

## Mock/Seed check

- **Dashboard:** No mock data. `dashboard/lib/api.ts` only has comments "REMOVED: MOCK_* - replaced with real API".
- **Core:** Pod/SA/Role data from DB; insights from DB. Seed only for reference data (e.g. capability metadata) when `FORTUNA_ENABLE_SEED_DATA` is set; not used for pod/list/detail.

## Clean / Rebuild / Redeploy

Script đầy đủ (clean images + rebuild core/agent/dashboard + deploy):

```bash
# Chạy đầy đủ (có thể mất 15–20+ phút)
./scripts/pipeline/full-clean-database-rebuild-deploy.sh

# Chạy nền, log vào file
RUN_ASYNC=1 ./scripts/pipeline/full-clean-database-rebuild-deploy.sh
# Theo dõi: tail -f /tmp/clean-rebuild-deploy.log
```

Chỉ rebuild + redeploy dashboard:

```bash
./scripts/clean/clean-rebuild-dashboard.sh
```

Sau khi deploy, Core chạy migration 066 (nếu chưa). Agent sync sẽ gửi `phase`; Core lưu vào `pods.phase`; Dashboard hiển thị Status từ API.
