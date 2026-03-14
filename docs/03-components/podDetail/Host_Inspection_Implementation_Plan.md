# Kế hoạch triển khai Host Inspection (Runtime Monitoring không dùng Exec)

**Mục tiêu:** Thay thế hoàn toàn (hoặc fallback) cơ chế thu thập process/network bằng **exec vào container** bằng thu thập từ **node (host)**: `/proc`, cgroup, CRI. Không cần `ps`/`ss`/`netstat`/`sh` trong container; hoạt động với distroless và khi policy chặn exec.

**Tham chiếu kiến trúc:** `docs/02-architecture/Runtime_Monitoring_Architecture.md`

---

## 1. Phân tích hiện trạng

### 1.1 Luồng hiện tại (exec)

| Bước | Thành phần | Hành động |
|------|------------|-----------|
| 1 | Reporter | `listPodsOnNode()` → danh sách pod trên node |
| 2 | Reporter | Với mỗi pod: `CollectProcessesFromPod()` / `CollectNetworkFromPod()` |
| 3 | Process/Network collector | **Exec** vào từng container: `ps -eo ...` hoặc `ss -tunap` / `netstat -tunap` |
| 4 | Reporter | Gửi POST `/api/v1/agent/pod-processes`, `pod-network-connections` (podUid, clusterId, namespace, processes[], connections[]) |
| 5 | Core | Lưu vào `pod_processes`, `pod_network_connections`; broadcast WebSocket |

### 1.2 Payload và model Core (giữ nguyên)

- **Process:** `PodProcess`: podUid, clusterId, namespace, **containerName**, pid, ppid, userName, cpuPercent, memoryPercent, command, binaryPath, observedAt.
- **Network:** `PodNetworkConnection`: podUid, clusterId, namespace, **containerName**, sourceIp, sourcePort, destIp, destPort, protocol, state, observedAt.

API ingest **không đổi**: POST body giữ nguyên; agent chỉ thay **nguồn** dữ liệu (host thay vì exec).

### 1.3 Ràng buộc cần đáp ứng

- Map **PID (host)** → **container** → **pod** để gán đúng `containerName` và `podUid`.
- Process: từ host cần ít nhất pid, ppid, comm (command); userName/cpuPercent/memoryPercent có thể từ `/proc` (một phần) hoặc để 0 nếu không lấy được.
- Network: từ `/proc/net/tcp`, `/proc/net/udp` hoặc `/proc/<pid>/net/*` cần map **inode** hoặc **pid** → container để gán connection vào đúng pod/container.

---

## 2. Kiến trúc Host Inspection

### 2.1 Nguồn dữ liệu trên host

| Dữ liệu | Nguồn | Ghi chú |
|---------|--------|--------|
| Danh sách PID | `/proc` (dir) | Chỉ thư mục số, bỏ qua kernel thread nếu cần |
| PID → cgroup | `/proc/<pid>/cgroup` | Cgroup v1 hoặc v2 format khác nhau |
| Cgroup → containerID | Parse cgroup path | containerd: `.../cri-containerd-<id>.scope` hoặc `.../containerd-<id>.scope` |
| containerID → (pod, containerName) | K8s API: pod.Status.ContainerStatuses | ContainerID dạng `containerd://<shortID>`; so khớp với cgroup id (short) |
| Process: comm, ppid | `/proc/<pid>/comm`, `/proc/<pid>/stat` | comm (tên), ppid (parent PID) |
| Process: user (optional) | `/proc/<pid>/status` (Uid) hoặc `/proc/<pid>/loginuid` | Có thể bỏ qua hoặc map uid → username từ /etc/passwd (host) |
| Process: CPU/Mem % (optional) | `/proc/<pid>/stat` + sampling hoặc cgroup `cpuacct`/`memory` | Phase 2; có thể để 0 ban đầu |
| Network: connections | `/proc/net/tcp`, `/proc/net/udp` | Format hex; cần map (uid, inode) → pid → container |
| Network: per-process | `/proc/<pid>/net/tcp`, `/proc/<pid>/net/udp` | Mỗi PID (container process) có namespace riêng; đọc theo PID đã map container |

### 2.2 Map containerID ↔ pod + containerName

- Agent đã có: `listPodsOnNode(ctx)` → `[]corev1.Pod`.
- Mỗi `pod.Status.ContainerStatuses` có: `Name` (containerName), `ContainerID` (ví dụ `containerd://a1b2c3d4e5...`).
- Chuẩn hóa ID: cgroup thường chỉ có **short** id (12 ký tự); ContainerID đôi khi full. So khớp: dùng suffix của ContainerID so với id trích từ cgroup (containerd dùng id ngắn trong cgroup path).
- Build một lần mỗi vòng report: `map[containerIDShort](podUID, namespace, containerName)` từ các pod trên node.

### 2.3 Cgroup format (containerd)

- **Cgroup v2** (default từ Kubernetes 1.24+):  
  `0::/kubepods.slice/kubepods-pod<uid>.slice/cri-containerd-<containerID>.scope`  
  → trích `containerID` = phần trước `.scope`.
- **Cgroup v1:**  
  Nhiều dòng, tìm dòng chứa `containerd` hoặc `cri-containerd` và id.

Cần hỗ trợ **cả hai** (detect bằng đọc dòng đầu `/proc/self/cgroup` hoặc kiểm tra `/sys/fs/cgroup/cgroup.controllers`).

### 2.4 Luồng mới (host-only)

```
Reporter.reportOnce()
  → listPodsOnNode()
  → buildContainerIDToPodMap(pods)   // containerID (short) → (podUID, namespace, containerName)
  → CollectProcessesFromHost(procRoot, containerMap)  // scan /proc, cgroup, map → []processPayload
  → CollectNetworkFromHost(procRoot, containerMap)   // /proc/net/* hoặc per-PID net, map → []connectionPayload per pod
  → Gộp process/connection theo podUid, gửi POST như hiện tại (per-pod)
```

---

## 3. Kế hoạch triển khai theo phase

### Phase 0: Điều kiện tiên quyết (DaemonSet + RBAC)

| # | Hạng mục | Chi tiết | Trạng thái |
|---|----------|----------|------------|
| 0.1 | hostPID | DaemonSet: `hostPID: true` để đọc /proc của các process trên node | ✅ Đã làm |
| 0.2 | Mount /proc | Volume hostPath `/proc` → mount trong agent (readOnly) tại `/host/proc` | ✅ Đã làm |
| 0.3 | Mount /sys (optional) | Có thể cần cho cgroup v2 hoặc cpu/memory (phase sau) | Tùy chọn |
| 0.4 | Bỏ exec permission | Sau khi host pipeline ổn định: có thể thu hồi `pods/exec` khỏi ClusterRole (tùy chọn) | Sau |

**Lưu ý:** `privileged: true` không bắt buộc nếu chỉ đọc `/proc` và cgroup; nhiều cluster cho phép hostPID mà không privileged. Nếu đọc bị denied, mới cân nhắc privileged hoặc capability (ví dụ CAP_SYS_PTRACE không cần cho đọc /proc cơ bản).

**File:** `deploy/fortuna-agent-daemonset.yaml`

---

### Phase 1: Cơ sở Host Inspection (cgroup, map, proc đọc cơ bản)

| # | Hạng mục | Chi tiết | File / gói dự kiến |
|---|----------|----------|--------------------|
| 1.1 | ContainerID → Pod map | Hàm build từ `[]corev1.Pod` (Status.ContainerStatuses) → map containerID short → (podUID, namespace, containerName). Xử lý prefix `containerd://`, `cri-o://`. | ✅ `host_map.go` |
| 1.2 | Đọc cgroup từ /proc | Đọc `/proc/<pid>/cgroup`, parse cgroup v1 và v2, trích containerID (containerd/cri-o). | ✅ `cgroup.go` |
| 1.3 | List PID từ /proc | Duyệt thư mục /proc, lấy các tên là số (PID); bỏ qua lỗi đọc. | ✅ `proc_scan.go` |
| 1.4 | Process từ /proc | Với mỗi PID: cgroup → containerID; map → đọc comm/stat; tạo processPayload. | ✅ `host_process.go` |
| 1.5 | Tích hợp Reporter | Env `POD_DETAIL_RUNTIME_SOURCE=host` → CollectProcessesFromHost, group theo podUid, sendProcessSnapshotsForPod. | ✅ `reporter.go` |

**Deliverable Phase 1:** Process list lấy từ host; không exec; Dashboard vẫn hiển thị process theo pod/container. Payload và API Core không đổi.

---

### Phase 2: Network từ host

| # | Hạng mục | Chi tiết | File / gói dự kiến |
|---|----------|----------|--------------------|
| 2.1 | Đọc /proc/net/tcp, udp | Parse format kernel (hex): local_addr, local_port, rem_addr, rem_port, state, inode. | `agent/internal/poddetail/proc_net.go` (mới) |
| 2.2 | Map connection → PID | Cách 1: Đọc `/proc/<pid>/fd/*` inode so với inode trong /proc/net (chậm). Cách 2 (khuyến nghị): Đọc **per-process** `/proc/<pid>/net/tcp`, `/proc/<pid>/net/udp` — mỗi container process có net namespace riêng, nên với mỗi PID đã map container ta đọc connection của chính PID đó → gán luôn containerName/pod. | `agent/internal/poddetail/host_network.go` (mới) |
| 2.3 | Tích hợp Reporter | Khi mode host: CollectNetworkFromHost() thay CollectNetworkFromPod(); gộp connection theo pod, POST như hiện tại. | `reporter.go`, `host_network.go` |

**Deliverable Phase 2:** Network connections lấy từ host; không exec. API Core không đổi.

---

### Phase 3: Chọn nguồn (exec vs host) và fallback

| # | Hạng mục | Chi tiết |
|---|----------|----------|
| 3.1 | Env / config | `POD_DETAIL_RUNTIME_SOURCE=host` → luôn host. `auto` (default): thử host trước (nếu hostPID và /proc có), fallback exec khi cần (hoặc ngược lại: exec trước, fallback host khi exec fail tool not found). |
| 3.2 | Fallback spec | Khi `POD_DETAIL_RUNTIME_SOURCE=auto`: nếu exec fail (tool not found) cho một pod/container → đánh dấu và dùng host cho pod đó (khi đã có host pipeline). Hoặc đơn giản: khi set host thì toàn bộ node dùng host. |
| 3.3 | Runtime Source trong payload (optional) | Thêm field optional `runtimeSource: "host" | "exec"` vào POST; Core lưu vào bảng (cần migration nhỏ) để Dashboard hiển thị "Runtime collected from host kernel". Có thể làm sau. |

---

### Phase 4: UI và tùy chọn

| # | Hạng mục | Chi tiết |
|---|----------|----------|
| 4.1 | Pod Detail UI | Indicator "Runtime Source: Host Inspection" hoặc "Container Exec" (nếu Core lưu runtimeSource; hoặc mặc định hiển thị "Host" khi toàn cluster dùng host). |
| 4.2 | CPU/Memory % (process) | Có thể bổ sung từ cgroup v2 (`cpu.stat`, `memory.current`) hoặc /proc sampling; effort trung bình. |
| 4.3 | Bỏ quyền exec | Sau khi chắc chắn dùng host: xóa `pods/exec` khỏi ClusterRole agent (giảm attack surface). |

---

## 4. Chi tiết kỹ thuật (để implement)

### 4.1 Chuẩn hóa containerID (K8s vs cgroup)

- K8s `ContainerID`: `containerd://<fullOrShort>` — có thể 64 ký tự hoặc 12.
- Cgroup path containerd: thường **short** (12 ký tự). So khớp: `strings.HasSuffix(containerIDFromK8s, idFromCgroup)` hoặc normalize cả hai về 12 ký tự (trim prefix, lấy 12 ký tự cuối).

### 4.2 Cgroup v1 vs v2

- **v2:** Một dòng `0::/path/to/scope`. Tìm segment chứa `cri-containerd-` hoặc `containerd-`, cắt ra id.
- **v1:** Nhiều dòng, controller:path. Tìm line có path chứa `containerd` và id (thường dạng `.../pod<uid>/<id>`).

### 4.3 /proc/<pid>/stat

- Format: `pid (comm) state ppid ...`. Comm nằm trong ngoặc, có thể chứa dấu ngoặc/spaces → parse cẩn thận (đọc từ "(" đến ")" cuối trước khi state).

### 4.4 /proc/<pid>/net/tcp (và udp)

- Dòng đầu là header; các dòng sau: sl, local_address, rem_address, st, tx_queue, rx_queue, tr, tm->when, retrnsmt, uid, timeout, inode, ...  
- local_address = addr:port (hex). Parse để có sourceIp, sourcePort, destIp, destPort, state.

### 4.5 Gộp theo pod

- Host trả về: process list (có containerName, và từ map có podUID, namespace). Nhóm theo (podUID, namespace), mỗi nhóm gửi một POST `pod-processes` với processes của pod đó. Tương tự connections.

---

## 5. Luồng dữ liệu (Core / Agent / DB / Dashboard)

| Bước | Thành phần | Mô tả |
|------|------------|--------|
| 1 | Agent | `reportOnce()`: listPodsOnNode → (khi host) BuildContainerIDToPodMap, CollectProcessesFromHost, CollectNetworkFromHost; group theo podUID. |
| 2 | Agent | POST `/api/v1/agent/pod-processes` (podUid, clusterId, namespace, **runtimeSource**, processes[]); POST `/api/v1/agent/pod-network-connections` (tương tự + connections[]). |
| 3 | Core | IngestPodProcessesPayload / IngestPodNetworkConnectionsPayload: bind JSON, set RuntimeSource trên từng row, db.CreateInBatches; BroadcastPodDetailUpdate(podUid, "processes"|"network"). |
| 4 | DB | `pod_processes`, `pod_network_connections` (cột `runtime_source` VARCHAR(32) DEFAULT 'exec'). |
| 5 | Core | GET `/api/v1/pods/:id/processes`, `by-uid/:uid/processes` → getPodProcessesByUID; GET network-connections tương tự; decrypt process list; JSON { podUid, items: [...] } (mỗi item có runtimeSource). |
| 6 | Dashboard | getPodProcesses / getPodNetworkConnections → hiển thị bảng; badge "Runtime: Host Inspection" khi items[0].runtimeSource === 'host', ngược lại "Container Exec". |

---

## 6. Thứ tự thực hiện đề xuất

| Thứ tự | Phase | Nội dung |
|--------|--------|----------|
| 1 | 0 | Cập nhật DaemonSet: hostPID, mount /proc |
| 2 | 1 | host_map.go, cgroup.go, proc_scan.go, host_process.go; Reporter gọi host khi POD_DETAIL_RUNTIME_SOURCE=host |
| 3 | 2 | proc_net.go, host_network.go; Reporter gọi host network khi mode host |
| 4 | 3 | Env auto + fallback (exec ↔ host) |
| 5 | 4 | UI Runtime Source (và optional Core field); tùy chọn bỏ exec RBAC |

---

## 7. Rủi ro và giảm thiểu

| Rủi ro | Giảm thiểu |
|--------|------------|
| hostPID = true mở rộng quyền | Chỉ mount /proc read-only; không thêm privileged nếu không cần; document security trade-off. |
| Cgroup v1 vs v2 khác nhau giữa distro | Implement parse cho cả hai; test trên Ubuntu 22.04 (v2) và nếu có máy cũ (v1). |
| Container runtime khác (CRI-O, Docker) | Parse cgroup path cho cri-o, docker; map containerID tương tự từ ContainerStatuses (format khác prefix). |
| Performance: scan toàn bộ /proc mỗi kỳ | Giới hạn PID (ví dụ bỏ qua pid > 1 triệu); cache container map trong một vòng; interval report giữ 2 phút. |

---

## 8. Checklist triển khai

- [x] Phase 0: DaemonSet hostPID + mount /proc — **Đã làm:** `deploy/fortuna-agent-daemonset.yaml` (hostPID: true, volume host-proc /proc → /host/proc, env POD_DETAIL_RUNTIME_SOURCE commented)
- [x] Phase 1.1–1.4: host_map, cgroup, proc_scan, host_process — **Đã làm:** `agent/internal/poddetail/host_map.go`, `cgroup.go`, `proc_scan.go`, `host_process.go`
- [x] Phase 1.5: Reporter tích hợp host process — **Đã làm:** `reporter.go` (useHostRuntime(), hostProcRoot(), CollectProcessesFromHost, sendProcessSnapshotsForPod; env POD_DETAIL_RUNTIME_SOURCE=host, POD_DETAIL_PROC_ROOT)
- [x] Phase 2.1–2.3: proc_net, host_network, Reporter network host — **Đã làm:** `proc_net.go`, `host_network.go`, Reporter gọi CollectNetworkFromHost và sendNetworkConnectionsForPod
- [x] Phase 3: Env POD_DETAIL_RUNTIME_SOURCE=auto + fallback — **Đã làm:** `useHostRuntime()` hỗ trợ host|exec|auto; `canUseHostProc(procRoot)` khi auto
- [x] Phase 4: UI Runtime Source — **Đã làm:** Migration 074 (runtime_source), Core models + ingest; Agent gửi runtimeSource: "host"; Dashboard badge "Runtime: Host Inspection | Container Exec"
- [ ] Test E2E: distroless pod có process/network từ host; so sánh với exec (khi có) về số process/connection
- [x] Unit test: host_map_test, cgroup_test, proc_scan_test, host_process_test, proc_net_test
- [ ] Doc: cập nhật Runtime_Monitoring_Architecture.md khi cần
