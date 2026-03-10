# Monitor lỗi Agent và Core

Script và lệnh để theo dõi lỗi/warning của Agent và Core, cùng cách phân tích và xử lý. **Vận hành đầy đủ:** `deploy/README.md` (Operations & troubleshooting), `scripts/README.md` (Vận hành và xử lý lỗi).

## Lệnh monitor nhanh

```bash
# Chỉ xem các dòng lỗi/warning (Core + tất cả Agent), 200 dòng gần nhất mỗi pod
./scripts/monitor/monitor-agent-core-errors.sh

# Theo dõi liên tục, chỉ in ra dòng có lỗi/warning (Ctrl+C thoát)
./scripts/monitor/monitor-agent-core-errors.sh --follow

# Lấy 500 dòng gần nhất rồi lọc lỗi
./scripts/monitor/monitor-agent-core-errors.sh --logs 500

# Ghi thêm vào file
./scripts/monitor/monitor-agent-core-errors.sh --save /tmp/fortuna-errors.log
```

**Xem toàn bộ log (không lọc):**

```bash
./scripts/monitor/monitor-agent-core.sh --follow --logs 100
# Hoặc trực tiếp:
kubectl logs -n fortuna -l app.kubernetes.io/component=core -f --tail=200
kubectl logs -n fortuna -l app.kubernetes.io/component=agent -f --tail=200
```

---

## Phân tích một số lỗi thường gặp

### 1. `Containerd fetch failed ... content digest ... not found`

**Ví dụ:**

```
[SBOMExtractor] Containerd fetch failed (image/layer may be missing on this node): containerd export error: content digest sha256:18fe...: not found
[SBOMExtractor] Falling back to remote registry: nats:2.10-alpine
```

**Ý nghĩa:** Agent lấy SBOM theo cơ chế local-first: thử export image từ containerd trên node trước. Trên node đó image có thể có theo tag (ví dụ `nats:2.10-alpine`) nhưng một vài layer/content đã bị thiếu hoặc chưa có (GC, image chưa pull đủ, v.v.) nên export thất bại. Đây là **warning**, không phải lỗi chết: agent sẽ **fallback sang pull từ remote registry** và tiếp tục extract SBOM.

**Cần làm gì:**

- **Nếu sau đó có dòng "Fetched image from remote registry" / "Extracted N packages":** Không cần xử lý; SBOM vẫn chạy qua remote.
- **Nếu sau đó lỗi "failed to fetch from remote registry":** Trên node không kéo được image (mạng, registry, auth). Cần kiểm tra mạng/registry từ node đó hoặc đảm bảo image có sẵn trên node (ví dụ `ctr -n k8s.io images pull docker.io/library/nats:2.10-alpine` trên từng node).
- **Giảm log:** Có thể bật log chi tiết bằng biến môi trường (xem phần “Giảm log containerd fallback” bên dưới).

#### Chi tiết lý do lỗi "content digest ... not found"

Containerd trên mỗi node lưu hai thứ:

1. **Image metadata (ImageService)** — Chứa tên image (vd. `docker.io/library/nats:2.10-alpine`) và tham chiếu tới manifest. Lệnh `ctr -n k8s.io images ls` đọc từ đây nên image "có trong list".
2. **Content store (ContentStore)** — Chứa dữ liệu thật của từng blob (config, từng layer) theo digest `sha256:xxx`. Export image = đọc manifest rồi đọc **từng blob** từ ContentStore.

**Lỗi xảy ra khi nào:** Agent gọi `archive.Export(..., client.ContentStore(), ...)`. Export lấy manifest từ ImageService (thành công vì image có trên node), rồi với từng digest mà manifest tham chiếu (config + từng layer), gọi ContentStore để đọc blob. Thông báo **`content digest sha256:...: not found`** nghĩa là: **manifest bảo image có layer/config với digest này, nhưng trong ContentStore không có blob tương ứng** — export dừng và trả lỗi.

**Vì sao metadata có mà content lại thiếu:**

- **Pull chưa xong / bị gián đoạn:** Một vài layer chưa tải xong hoặc chưa ghi vào ContentStore; trên node vẫn thấy image trong `ctr images ls` nhưng export thiếu blob.
- **Garbage collection (GC):** Containerd/CRI dọn blob "không còn tham chiếu"; blob có thể bị xóa dù image vẫn còn trong list.
- **Image từng có rồi bị xóa/dọn:** Pod chuyển node hoặc đã xong; kubelet/containerd xóa image giải phóng dung lượng; có thể mất blob trước khi cập nhật metadata.
- **Nhiều node:** Agent chỉ đọc containerd **local** node đó. Pod (vd. nats) chạy trên node A; node B có thể "thấy" image trong metadata nhưng content đầy đủ chỉ ở node A — trên node B export dễ gặp "digest not found".

**Kết luận:** Trên node đó containerd có **metadata** của image nhưng thiếu ít nhất **một blob (digest)** trong ContentStore. Agent gặp lỗi này thì bỏ qua export local và pull image từ registry (fallback) để tiếp tục SBOM.

### 2. Core: `duplicate key` / `already stored`

Agent gửi SBOM lên Core; Core báo duplicate (component đã tồn tại). Agent coi đây là **non-fatal**: log warning và bỏ qua, không crash. Không cần xử lý trừ khi muốn tránh gửi trùng (ví dụ logic lần quét).

### 3. Agent: `Sync failed: status=500` / `column "kubeconfig" does not exist`

Core trả 500 thường do schema DB thiếu cột (ví dụ `clusters.kubeconfig`). Cần chạy migration hoặc reset DB (ví dụ `--db-reset` rồi deploy lại) để Core dùng đúng schema.

### 4. Agent: `no such host` / `connection refused` tới Core

Agent không resolve hoặc kết nối được tới Core (DNS hoặc Core chưa listen). Kiểm tra:

- Core đã Running và có endpoint: `kubectl get endpoints -n fortuna fortuna-core`
- DNS trong cluster: `nslookup fortuna-core.fortuna.svc.cluster.local` từ pod trong cluster
- Có thể dùng script: `./scripts/verify/verify-agent-core-connectivity.sh`

### 5. Agent: CrashLoopBackOff — OOMKilled (Exit 137)

Pod agent bị kill do **vượt memory limit** (Last State: Terminated, Reason: OOMKilled, Exit Code: 137). SBOM extraction tải image/layers vào memory (đặc biệt khi fallback pull từ registry), nhiều pod xử lý song song (2 workers) dễ vượt limit 1Gi.

**Cách xử lý:**

- **Tăng memory limit** (đã cập nhật trong `deploy/fortuna-agent-daemonset.yaml`): `limits.memory: 2Gi`, `requests.memory: 512Mi`. Apply lại và restart agent:
  ```bash
  kubectl apply -f deploy/fortuna-agent-daemonset.yaml
  kubectl rollout restart daemonset/fortuna-agent -n fortuna
  ```
- **Giảm peak memory (tùy chọn):** Set env `SBOM_WORKERS=1` trong daemonset để chỉ 1 pod SBOM xử lý tại một thời điểm (ít memory hơn, chậm hơn).

### 6. Core/Agent: Secret not found (ContainerCreating)

Pod kẹt ContainerCreating do thiếu secret mTLS. Deploy script đã có bước tạo mTLS (Step 7c). Nếu vẫn thiếu, chạy tay:

```bash
./scripts/utils/create_mtls_secret.sh
```

---

## Giảm log “containerd digest not found”

Từ bản agent hiện tại: khi export từ containerd thất bại, agent chỉ log ngắn **“Using remote registry for image: &lt;image&gt;”** rồi pull từ registry. Dòng dài **“Containerd fetch failed ... digest ... not found”** chỉ xuất hiện khi bật debug: set env **`SBOM_DEBUG=1`** (hoặc `true`) trên deployment/daemonset Agent nếu cần xem chi tiết. Trên node: đảm bảo image đã có đủ trên containerd (pull đủ layers) thì agent ít khi fallback.

---

## Tóm tắt lệnh hữu ích

| Mục đích | Lệnh |
|----------|------|
| Chỉ lỗi/warning Core + Agent | `./scripts/monitor/monitor-agent-core-errors.sh` |
| Theo dõi lỗi liên tục | `./scripts/monitor/monitor-agent-core-errors.sh --follow` |
| Trạng thái + log đầy đủ | `./scripts/monitor/monitor-agent-core.sh --follow` |
| Log Core | `kubectl logs -n fortuna -l app.kubernetes.io/component=core -f --tail=300` |
| Log Agent (một pod) | `kubectl logs -n fortuna <agent-pod> -f --tail=300` |

Namespace mặc định: `fortuna`. Ghi đè bằng `NAMESPACE=...` nếu cần.
