# DiskPressure, Evicted pods & dọn dẹp

## Vì sao pod bị Evicted?

Kubelet trên node gán điều kiện **`DiskPressure`** khi **ephemeral-storage** / đĩa gần đầy. Pod có thể bị **Evicted** với message:

`The node had condition: [DiskPressure]`

Thường gặp sau **`build-and-load-containerd.sh`** trên **cùng máy với node** (image/layer/cache chiếm nhiều dung lượng trong `/var/lib/containerd`).

## 1. Kiểm tra node

```bash
kubectl describe node <node-name> | grep -E 'Pressure|Allocatable|ephemeral'
df -h
```

## 2. Dọn đĩa trên node (SSH vào master/worker)

Script trong repo:

```bash
# Chỉ xem df + du (tương đương --check-only)
sudo bash scripts/utils/cleanup-node-disk.sh
sudo bash scripts/utils/cleanup-node-disk.sh --check-only

# Journal + prune nerdctl (namespace k8s.io), an toàn hơn: không dùng prune -a
sudo bash scripts/utils/cleanup-node-disk.sh --all

# Chỉ prune mạnh (xóa mọi image không dùng — cẩn thận)
sudo PRUNE_ALL=1 bash scripts/utils/cleanup-node-disk.sh --prune-nerdctl
```

Thủ công: `sudo journalctl --vacuum-time=7d`, xem `du -sh /var/lib/containerd`.

## 3. Xóa pod Evicted trong Kubernetes

Pod Evicted chỉ là “xác”; xóa để UI/API sạch (không giải phóng đĩa node):

```bash
bash scripts/utils/cleanup-evicted-pods.sh
# hoặc chỉ fortuna:
NAMESPACE=fortuna bash scripts/utils/cleanup-evicted-pods.sh
DRY_RUN=1 bash scripts/utils/cleanup-evicted-pods.sh   # xem trước
```

## 4. Sau khi hết DiskPressure

```bash
kubectl rollout restart deployment/fortuna-core -n fortuna
# các workload khác tùy môi trường
```

## 5. Phòng ngừa

- Build image trên **máy khác** hoặc CI; hoặc build xong **`nerdctl system prune -f`** (có chừng mực).
- Tăng dung lượng đĩa VM / tách volume cho containerd.
- Tránh `NO_CACHE=true` mọi lần rebuild.

Xem thêm: [STORAGE_CLASS_USAGE.md](STORAGE_CLASS_USAGE.md) (PVC vs đĩa node).

**Worker (multi-node), agent không pull/import được:** [WORKER_IMAGE_CLEAN_AND_AGENT_PUSH.md](WORKER_IMAGE_CLEAN_AND_AGENT_PUSH.md) — dọn image trên worker01, `push-images-to-workers.sh --clean-remote`, rollout DaemonSet.
