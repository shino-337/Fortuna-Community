# Worker (vd. worker01): dọn image cũ & đưa agent image vào node

Khi **đĩa worker đầy** hoặc **quá nhiều layer/image cũ**, `ctr import` / pull local có thể **thất bại** → pod agent trên worker báo **ErrImagePull / ImagePullBackOff** hoặc không có chỗ ghi image mới.

Luồng khắc phục: **giải phóng dung lượng trên worker01** → **import lại** `fortuna-agent` (và thường kèm `fortuna-core`) từ máy đã build.

**Tên image trong deploy:** `fortuna-agent:latest` (xem `deploy/fortuna-agent-daemonset.yaml`).

---

## 1. Kiểm tra nhanh

```bash
kubectl describe node k8s-worker01 | grep -E 'Pressure|Allocatable|ephemeral'
kubectl get pods -n fortuna -o wide | grep agent
# Pod agent trên worker01:
kubectl describe pod -n fortuna -l app.kubernetes.io/component=agent --field-selector spec.nodeName=k8s-worker01
```

---

## 2. Dọn trên worker01 (SSH trực tiếp)

### 2a. Kiểm tra + journal + prune chung (an toàn tương đối)

Trên worker01 (có clone repo hoặc copy script):

```bash
sudo bash scripts/utils/cleanup-node-disk.sh --all
# hoặc không có repo: xem docs/05-operations/DISK_PRESSURE_AND_CLEANUP.md
```

### 2b. Chỉ gỡ image Fortuna/KSAM cũ + prune (không cần repo)

```bash
sudo ctr -n k8s.io images ls -q 2>/dev/null | grep -E 'fortuna|ksam' | xargs -r -I {} sudo ctr -n k8s.io images rm {} 2>/dev/null || true
sudo nerdctl --namespace k8s.io system prune -f
```

Nếu vẫn thiếu chỗ (cẩn thận — xóa image không dùng toàn cluster trên node):

```bash
sudo nerdctl --namespace k8s.io system prune -a -f
```

---

## 3. Dọn từ máy admin qua SSH (một lệnh)

Dùng script (từ repo, máy có `ssh` tới worker):

```bash
WORKER_SSH=root@192.168.56.101   # hoặc k8s@IP
bash scripts/utils/cleanup-remote-worker-images.sh "$WORKER_SSH"
```

Script gọi trên remote: xóa image `fortuna*`|`ksam*` trong `k8s.io`, rồi `nerdctl system prune -f`.

---

## 4. Build local + đẩy lại image lên worker01

Trên **máy đã build** (có `fortuna-core:latest` và `fortuna-agent:latest` trong containerd):

```bash
cd /path/to/KSAM
bash scripts/build/build-and-load-containerd.sh   # nếu chưa build
```

Chỉ đẩy tới **worker01** (đặt IP/hostname SSH đúng môi trường):

```bash
export WORKER_NODES="k8s-worker01"   # hoặc IP, ví dụ 10.0.0.12
export SSH_USER=k8s
export SSH_PASS=...                  # nếu dùng sshpass; hoặc dùng SSH key → không cần
bash scripts/utils/push-images-to-workers.sh --clean-remote
```

- `--clean-remote`: trước khi import, xóa image fortuna/ksam cũ **trên từng node** trong `WORKER_NODES` (tránh chồng layer / hết đĩa khi import).

**Bỏ qua `push-images.config`** (để không merge master + danh sách worker dài) và **chỉ đẩy worker01**:

```bash
cd /path/to/KSAM
PUSH_CONFIG_FILE= WORKER_NODES=k8s-worker01 SSH_USER=k8s \
  bash scripts/utils/push-images-to-workers.sh --clean-remote
```

(`PUSH_CONFIG_FILE=` rỗng → script không `source` file config; `WORKER_NODES` dùng đúng giá trị bạn export.)

Nếu vẫn dùng file config và cần chỉ worker01: sửa tạm `WORKER_NODES=` trong config **hoặc** chạy dọn trực tiếp trên worker (mục 2 / mục 3).

Script gốc: `push-images-to-workers.sh` còn hỗ trợ:

```bash
bash scripts/utils/push-images-to-workers.sh --clean-only
```

(chỉ dọn fortuna/ksam trên các node trong `WORKER_NODES`, không export/import — dùng trước khi rebuild/push.)

---

## 5. Rollout lại DaemonSet agent

```bash
kubectl rollout restart daemonset/fortuna-agent -n fortuna
kubectl rollout status daemonset/fortuna-agent -n fortuna --timeout=120s
kubectl get pods -n fortuna -l app.kubernetes.io/component=agent -o wide
```

Xác nhận trên worker01:

```bash
sudo ctr -n k8s.io images ls | grep fortuna-agent
```

---

## 6. Liên quan

- [DISK_PRESSURE_AND_CLEANUP.md](DISK_PRESSURE_AND_CLEANUP.md) — DiskPressure, Evicted, prune chi tiết.
- [DEPLOYMENT_CHECKLIST.md](DEPLOYMENT_CHECKLIST.md) — multi-node, `PUSH_TO_WORKERS`, biến môi trường.
