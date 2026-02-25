# Remote vào worker01 và kiểm tra disk sau khi tăng lên 40GB

**Lưu ý:** Từ môi trường build/CI không có SSH key vào worker01 nên không thể SSH trực tiếp. Bạn cần **SSH từ máy có quyền** (laptop/máy có key hoặc mật khẩu) vào worker01 và chạy lệnh/script bên dưới. Worker01 đang DiskPressure nên mọi pod schedule lên đó đều bị evict — không thể dùng pod trong cluster để chạy lệnh trên node.

## 1. Kết nối SSH vào worker01

Từ máy có SSH key hoặc mật khẩu của user trên worker01:

```bash
# Nếu dùng hostname (cần resolve trong /etc/hosts hoặc DNS)
ssh k8s-worker01

# Hoặc dùng IP (InternalIP của worker01)
ssh 192.168.56.101

# Nếu dùng user khác root
ssh your_user@192.168.56.101
```

Nếu bị "Permission denied (publickey,password)": cần copy SSH key sang worker01 hoặc dùng mật khẩu đúng user.

---

## 2. Cách nhanh: chạy script kiểm tra + extend disk (trên worker01)

Sau khi SSH vào worker01:

**Cách A – Copy script từ máy có repo (master hoặc laptop):**

```bash
# Trên máy có repo (ví dụ master), copy script sang worker01 (cần SSH key từ máy đó sang worker01)
scp scripts/verify/expand-disk-worker01-remote.sh root@192.168.56.101:/tmp/
# Trên worker01:
ssh root@192.168.56.101 "sudo bash /tmp/expand-disk-worker01-remote.sh"
# Sau đó restart kubelet trên worker01:
ssh root@192.168.56.101 "sudo systemctl restart kubelet"
```

**Cách B – Chạy thủ công từng bước (xem mục 2–4 bên dưới):** kiểm tra `lsblk`, `df -h`, rồi chạy `growpart` + `resize2fs` (hoặc LVM) theo hướng dẫn.

---

## 3. Kiểm tra disk trên worker01

Sau khi đã SSH vào worker01, chạy:

```bash
# Block devices và kích thước
lsblk

# Dung lượng filesystem
df -h

# Partition/volume của root (/)
findmnt -n -o SOURCE /
```

- Nếu **df -h** vẫn chỉ ~10GB cho partition chứa `/` thì **OS chưa dùng hết disk 40GB** (partition hoặc filesystem chưa mở rộng).
- Nếu **lsblk** cho thấy disk (vd. `sda` hoặc `vda`) đã ~40GB nhưng partition bên trong vẫn ~10GB → cần **mở rộng partition** rồi **resize filesystem**.

---

## 4. Mở rộng partition và filesystem (khi VM đã tăng disk)

**Lưu ý:** Chỉ làm khi bạn chắc disk ở tầng VM/cloud đã được tăng lên 40GB (vd. đã resize disk trong VMware/VirtualBox/cloud console).

### 3.1 Disk dạng partition (vd. /dev/sda1, /dev/vda1)

```bash
# Cài nếu chưa có
sudo apt-get update && sudo apt-get install -y cloud-guest-utils

# Mở rộng partition (sửa sda 1 cho đúng device và số thứ tự partition của bạn, xem lsblk)
sudo growpart /dev/sda 1
# hoặc
sudo growpart /dev/vda 1

# Mở rộng filesystem trên partition đó (sửa /dev/sda1 cho đúng)
sudo resize2fs /dev/sda1
# hoặc nếu dùng xfs:
# sudo xfs_growfs /
```

### 3.2 Disk dạng LVM (Logical Volume)

```bash
# Xem VG/PV
sudo pvdisplay
sudo vgdisplay
sudo lvdisplay

# Mở rộng PV (physical volume) nếu disk đã tăng
sudo pvresize /dev/sda1   # hoặc /dev/vda1 tùy lsblk

# Mở rộng LV (logical volume) chứa root
sudo lvextend -l +100%FREE /dev/ubuntu-vg/ubuntu-lv   # tên VG/LV có thể khác, xem lvdisplay

# Resize filesystem
sudo resize2fs /dev/ubuntu-vg/ubuntu-lv
# hoặc xfs: sudo xfs_growfs /
```

Sau bước này chạy lại `df -h`: partition chứa `/` nên tăng lên ~40GB (hoặc gần đó).

---

## 5. Cho Kubernetes nhận lại dung lượng

Kubelet đọc ephemeral-storage từ node (thường là rootfs). Sau khi đã mở rộng FS:

```bash
sudo systemctl restart kubelet
```

Đợi vài chục giây rồi từ **master** kiểm tra:

```bash
kubectl describe node k8s-worker01 | grep -A5 Capacity
kubectl describe node k8s-worker01 | grep -A2 Conditions
```

- **Capacity / Allocatable** `ephemeral-storage` nên tăng (vd. ~40Gi hoặc hơn).
- **DiskPressure** chuyển sang **False** khi đủ dung lượng.

---

## 6. Script kiểm tra + extend (chạy trên worker01)

Trong repo: **`scripts/verify/expand-disk-worker01-remote.sh`**. Script tự động: in `lsblk`/`df -h`, nhận diện LVM hoặc partition, chạy `pvresize`/`lvextend`/`resize2fs` hoặc `growpart`/`resize2fs`. Chạy trên worker01 (sau khi SSH): `sudo bash /tmp/expand-disk-worker01-remote.sh` (sau khi copy script sang /tmp).

---

## 7. Tóm tắt

| Bước | Làm gì |
|------|--------|
| 1 | SSH vào worker01 (192.168.56.101 hoặc k8s-worker01). |
| 2 | Chạy `lsblk`, `df -h` để xem disk/partition/FS hiện tại. |
| 3 | Nếu partition/FS chưa 40GB: dùng `growpart` + `resize2fs` (hoặc LVM: `pvresize`/`lvextend` + `resize2fs`). |
| 4 | Restart kubelet: `sudo systemctl restart kubelet`. |
| 5 | Từ master: `kubectl describe node k8s-worker01` kiểm tra Capacity và DiskPressure. |

---

**Nếu bạn đang ở máy có SSH vào worker01:** chạy `sudo bash /tmp/expand-disk-worker01-remote.sh` (sau khi copy script từ repo sang worker01) rồi `sudo systemctl restart kubelet`.

Sau khi node báo đủ ephemeral-storage và hết DiskPressure, pod (Postgres, NATS, Agent) có thể schedule lại trên worker01 nếu bạn không dùng nodeSelector cố định master.
