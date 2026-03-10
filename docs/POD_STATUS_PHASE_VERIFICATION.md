# Pod Status (phase) – Kiểm tra và xử lý

## Kết quả kiểm tra

### 1. Database (Postgres)
- **Cột `phase`:** Có (migration 066 đã chạy): `phase VARCHAR(32) DEFAULT ''`
- **Dữ liệu:** Toàn bộ bản ghi đang có `phase` **rỗng** (trừ 1 bản ghi test đã UPDATE tay)

```sql
-- Kiểm tra
SELECT name, namespace, phase FROM pods LIMIT 10;
```

### 2. Core API
- Handlers (`GetPods`, `GetPod`, `GetPodByUID`) trả về struct nhúng `models.Pod` → JSON có field `"phase"`.
- Nếu DB có `phase` thì API sẽ trả đúng; khi `phase` rỗng thì frontend nhận `""` hoặc không hiển thị.

### 3. Agent
- Code: `PodPayload.Phase` được set `phase := string(p.Status.Phase)` và gửi trong sync.
- Log Agent có dòng `[Syncer] ✅ Full sync completed` → sync đã chạy.
- **Nguyên nhân khả dĩ:** Image **Core** (hoặc Agent) đang chạy là bản **cũ**, không có logic đọc/ghi `phase` (Core) hoặc không gửi `phase` (Agent).

### 4. Luồng code (đã có trong repo)
| Bước | Vị trí | Nội dung |
|------|--------|----------|
| Agent gửi | `agent/internal/syncer/syncer.go` | `Phase: phase` trong `PodPayload`, `phase := string(p.Status.Phase)` |
| Core nhận | `core/internal/service/agent_service.go` | `phase, _ := podMap["phase"].(string)` |
| Core lưu (create) |同上 | `pod := models.Pod{ ..., Phase: phase, ... }` → `s.db.Create(&pod)` |
| Core lưu (update) | 同上 | `Updates(map[string]interface{}{ "phase": pod.Phase, ... })` |
| API trả về | `core/internal/api/handlers.go` | Trả về struct nhúng `models.Pod` → JSON có `phase` |
| Dashboard | `dashboard/lib/api.ts` | `status: p.phase != null ? String(p.phase) : undefined` |
| UI | `Resources.tsx`, `PodDetail.tsx` | Hiển thị `pod.status ?? '—'` |

## Cách xử lý (khuyến nghị)

**Rebuild Core và Agent, sau đó redeploy** để đảm bảo process đang chạy đúng bản code có xử lý `phase`:

```bash
# Rebuild cả Core và Agent (Dashboard đã có map phase → status)
./scripts/build/build-and-load-containerd.sh

# Push image tới các node (nếu dùng imagePullPolicy: Never)
./scripts/utils/push-images-to-workers.sh

# Chỉ deploy (không xóa DB, không clean images)
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --skip-clean --skip-rebuild
```

Sau khi Core + Agent chạy bản mới:
1. Đợi 1–2 phút cho agent full sync.
2. Kiểm tra DB: `phase` phải có giá trị (Running, Pending, …):

```bash
POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n fortuna $POD -- psql -U postgres -d fortuna -t -c "SELECT name, namespace, phase FROM pods WHERE phase != '' LIMIT 10;"
```

3. Kiểm tra log Core khi sync (nếu thấy log “Pod phase from agent” thì Core đã nhận và xử lý phase):

```bash
kubectl logs -n fortuna deployment/fortuna-core --tail=200 | grep -E "phase|Processing.*pods"
```

4. Mở Dashboard → Resources (tab Pod) và Pod Detail → cột/card Status sẽ hiển thị (Running, Pending, …).

## Log debug đã thêm

Trong `core/internal/service/agent_service.go` đã thêm log khi nhận được `phase` từ agent:

```go
if phase != "" && name != "" {
    s.logger.Printf("📦 Pod phase from agent: %s/%s phase=%s", namespace, name, phase)
}
```

Sau khi xác nhận mọi thứ hoạt động, có thể xóa đoạn log này.
