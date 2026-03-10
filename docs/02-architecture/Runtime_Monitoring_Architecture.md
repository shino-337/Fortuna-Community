# Runtime Monitoring Architecture – Hiện tại và Đề xuất (Production-grade)

Tài liệu mô tả kiến trúc thu thập runtime (process, network) cho Pod Detail: vấn đề hiện tại (exec), giải pháp đề xuất (node-level / host inspection), và thông tin cần có để triển khai chính xác.

---

## 1. Vấn đề kiến trúc hiện tại

Luồng hiện tại:

```
Agent (DaemonSet)
   ↓
kubectl exec vào từng container
   ↓
chạy command Linux: ps, ss, netstat
```

**Vì sao không scalable / không reliable:**

| Vấn đề | Mô tả |
|--------|--------|
| **Distroless images** | Nhiều image (etcd, kube-apiserver, flannel, …) không có `ps`, `ss`, `netstat`, `sh` → exec fail. |
| **Security policies** | PodSecurityPolicy / PSS có thể chặn exec; policy “no exec” làm runtime collection không chạy được. |
| **Exec chậm** | Mỗi exec = fork process trong container, latency cao khi nhiều pod/container. |
| **Exec storm** | Nhiều agent cùng lúc exec nhiều container → kubelet quá tải. |

Các vendor lớn (Datadog, Sysdig, Aqua Security) **không dùng exec** để collect runtime data; họ dùng node-level (host) hoặc eBPF.

---

## 2. Giải pháp đúng (production-grade)

Thu thập từ **node level**, không exec vào container. Agent đã chạy DaemonSet → mỗi node đã có một agent → có lợi thế để đọc từ host.

---

## 3. Thu thập process runtime – phương án chuẩn

**Option A (khuyến nghị): Đọc /proc từ host**

- Mount host: `/proc`, `/sys`, `/var/lib/containerd`, `/var/run/containerd` (hoặc tương đương CRI-O/Docker).
- Đọc `/proc/<pid>/...` trên host.
- **Mapping PID → container:** đọc `/proc/<pid>/cgroup`:
  - Ví dụ: `kubepods.slice/.../containerd://<containerID>` → map được containerID.
- Cách này tương đương nhiều runtime monitor trong production.

**Yêu cầu agent:** `hostPID: true` (và thường `privileged: true` hoặc capability tối thiểu để đọc /proc của container).

---

## 4. Thu thập network runtime

Không cần `netstat`/`ss` trong container.

- Đọc từ host:
  - `/proc/net/tcp`, `/proc/net/udp`, hoặc
  - `/proc/<pid>/net/*` (per-process).
- Hoặc dùng **ss API via netlink** trên host.

---

## 5. Thu thập container metadata

- **Pod/container metadata:** từ Kubernetes API (pod, containerID, namespace, node).
- **Container runtime:** qua CRI.
  - Containerd: socket `/run/containerd/containerd.sock` → CRI API → map **containerID ↔ pid**.

---

## 6. Kiến trúc chuẩn cho Runtime Monitoring

Agent chạy với:

- `privileged: true` (hoặc capability tối thiểu)
- `hostPID: true`
- Mount: `/proc`, `/sys`, `/var/lib/containerd`, `/run/containerd`

Luồng:

```
Agent
  ↓
scan /proc (host)
  ↓
detect pid
  ↓
read /proc/<pid>/cgroup
  ↓
map containerID
  ↓
map pod (K8s API + CRI)
```

**Không cần exec container.**

---

## 7. Phương án advanced: eBPF

- Hook: `execve`, `connect`, `accept`, `fork`, …
- Framework: Falco, Cilium, hoặc custom eBPF program.
- Event flow: kernel → eBPF → agent → core.
- Ưu điểm: realtime, không poll, không exec.

---

## 8. Đề xuất thay đổi cho Fortuna

**Bỏ hoàn toàn (khi chuyển sang host inspection):**

- `kubectl exec` cho process/network
- Phụ thuộc vào `ps`, `netstat`, `ss`, `sh` trong container

**Runtime collection pipeline mới:**

- Pod metadata (K8s API)
- Container metadata (CRI: containerd socket)
- Process scan (/proc trên host)
- Network scan (/proc/net hoặc netlink)
- Tùy chọn: eBPF events

---

## 9. Xử lý lỗi hiện tại (ngắn hạn)

Khi vẫn dùng exec:

- Nếu exec lỗi **“executable file not found”** (hoặc tool not found):
  - **Không** log mức error.
  - Chỉ log **DEBUG**: “container does not provide shell utilities (distroless/minimal), skipping exec”.
  - Fallback: (sau này) chuyển sang node-level collection khi đã triển khai host inspection.

---

## 10. Spec fallback (recommend)

```
if exec_command_fail (tool not found / distroless):
    mark_container_distroless = true
    switch_runtime_collect_mode = host   # khi đã có host pipeline
```

Khi chưa có host pipeline: chỉ skip container, không báo lỗi; khi đã có host pipeline: dùng host runtime inspection cho pod đó.

---

## 11. UI/UX đề xuất

Trong Pod Detail UI:

- Thêm indicator **Runtime Source:**  
  `[Container Exec]` | `[Host Inspection]`
- Nếu distroless hoặc đang dùng host: hiển thị  
  **“Runtime collected from host kernel”** (hoặc tương đương).

---

## 12. Thông tin cần có để đề xuất chính xác

Dưới đây là câu trả lời **từ codebase** và chỗ cần **bổ sung lúc deploy / môi trường**.

### 12.1 Container runtime

| Nguồn | Giá trị |
|-------|--------|
| Codebase | **containerd** (DaemonSet mount `/run/containerd/containerd.sock`, env `CONTAINERD_NAMESPACE=k8s.io`, SBOM extractor dùng containerd client). |
| CRI-O / Docker | Không thấy cấu hình; mặc định coi là **containerd**. |

→ **Trả lời: containerd.** Nếu cluster dùng CRI-O hoặc Docker, cần cấu hình socket và API tương ứng.

### 12.2 Agent DaemonSet: hostPID, privileged

| Nguồn | Giá trị |
|-------|--------|
| `deploy/fortuna-agent-daemonset.yaml` | `hostPID: false` |
| | Không có `privileged: true`; `securityContext`: `runAsUser: 0`, `allowPrivilegeEscalation: false`, `capabilities: drop: [ALL]`. |

→ **Trả lời:** Hiện tại **hostPID: false**, **privileged: false**. Để làm host-level collection (/proc, CRI), cần bật **hostPID: true** và cân nhắc **privileged: true** (hoặc capability tối thiểu) và mount `/proc`, `/sys`, `/var/lib/containerd`, `/run/containerd`.

### 12.3 Kernel version của node

Không thể suy ra từ repo. Cần lấy tại node:

```bash
uname -r
```

→ **Bổ sung khi triển khai:** Ghi lại kernel version (quan trọng nếu dùng eBPF – yêu cầu kernel đủ mới).

### 12.4 Cluster version

Không thể suy ra từ repo. Cần lấy tại cluster:

```bash
kubectl version
```

→ **Bổ sung khi triển khai:** Ghi lại server (cluster) version để đảm bảo tương thích CRI và K8s API.

---

## 13. Liên hệ với Pod Detail Implementation Plan

- **Phase 4.1** trong `Pod_Detail_Implementation_Plan.md`: “Process procfs (nsenter + /proc) thay/fallback exec ps” đang ở **Backlog** (cần CAP_SYS_PTRACE, thiết kế host access).
- Tài liệu này mở rộng thành **kiến trúc đầy đủ**: bỏ exec, dùng host /proc + CRI + (tùy chọn) eBPF, và trả lời 4 câu hỏi để triển khai đúng runtime (containerd, hostPID/privileged, kernel, cluster version).
