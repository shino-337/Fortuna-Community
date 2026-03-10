# Runtime Escape Probe Detection (REP) – Technical Specification

## 1. Mục tiêu

Thiết kế cơ chế **Runtime Escape Probe Detection (REP)** nhằm phát hiện sớm các pod/container **bị ảnh hưởng bởi CVE runtime (ví dụ runc/containerd)** dù:

* Không có CVE ở SBOM
* Chưa khai thác thành công
* Chỉ mới ở giai đoạn *probe / capability discovery*

REP bổ sung vào hệ thống hiện tại (đã có PCE – Pod Capability Engine), tập trung **runtime behavior + syscall + namespace interaction**.

---

## 2. Phạm vi & Giả định

* Kubernetes cluster, CRI: containerd / CRI-O
* Runtime CVE dạng: container escape, namespace break, runc overwrite, procfs abuse
* Không sửa workload image
* Không dùng eBPF phức tạp giai đoạn 1 (có roadmap)

---

## 3. Kiến trúc tổng thể

```
[ Kernel ]
   │
   │ (syscall, audit, seccomp notify)
   ▼
[ Runtime Sensor ]  <-- Falco / auditd / custom agent
   │
   ▼
[ REP Engine ]
   │   ├─ Rule Matcher
   │   ├─ Context Enricher (PodSpec, Capability, Runtime)
   │   ├─ Scoring Engine
   │   └─ Response Orchestrator
   ▼
[ PCE Core ]
   │
   ▼
[ SOC / SIEM / Admission / K8s API ]
```

---

## 4. Capability Model mở rộng

### 4.1 Capability mới

| Capability         | Level    | Ý nghĩa                                            |
| ------------------ | -------- | -------------------------------------------------- |
| ESC_RUNTIME_PROBE  | HIGH     | Pod có điều kiện thuận lợi + hành vi probe runtime |
| ESC_RUNTIME_ACTIVE | CRITICAL | Có dấu hiệu exploit runtime rõ ràng                |

---

## 5. Điều kiện kích hoạt ESC_RUNTIME_PROBE

### 5.1 Static Risk (từ PodSpec)

Gắn **risk baseline** nếu:

* `hostPID = true`
* `hostIPC = true`
* `hostPath` mount trỏ tới:

  * `/proc`
  * `/sys`
  * `/run`, `/var/run`
  * `/dev`

→ **StaticRiskScore = 30–50**

**Phase 1 (hiện tại):** PCE sẽ tạo **ESC_RUNTIME_PROBE** ngay khi pod được tạo/sync nếu có các dấu hiệu tĩnh ở trên.  
Runtime signal sẽ được bổ sung ở Phase 2 để giảm false-positive và phân biệt PROBE/ACTIVE.

---

### 5.2 Runtime Signal (bắt buộc)

Khi pod **đồng thời** có static risk **và** runtime signal sau:

#### Syscall bất thường

```
syscall ∈ { mount, pivot_root, setns, unshare }
```

#### Target nhạy cảm

```
/path ∈ { /proc/self/exe, /proc/1/exe, /proc/*/ns/* }
```

#### Capability mismatch

```
capability_used ∉ expected_capability_from_podspec
```

→ Gắn **ESC_RUNTIME_PROBE**

---

## 6. Rule Logic (Formal)

```pseudo
IF pod.runtime IN [containerd, crio]
AND pod.staticRisk >= MEDIUM
AND syscall IN [mount, pivot_root, setns, unshare]
AND target_path MATCHES sensitive_runtime_path
AND capability_used NOT IN pod.allowedCapabilities
THEN
  emit REP_EVENT(type=PROBE)
```

---

## 7. Scoring Engine

### 7.1 Công thức

```
TotalScore = StaticRisk
           + SyscallWeight
           + TargetWeight
           + FrequencyWeight
```

| Thành phần      | Ví dụ                          |
| --------------- | ------------------------------ |
| StaticRisk      | hostPID + hostPath(/proc) = 40 |
| SyscallWeight   | setns = 20                     |
| TargetWeight    | /proc/1/exe = 25               |
| FrequencyWeight | >3 lần / 60s = 15              |

### 7.2 Ngưỡng

| Score | Kết quả            |
| ----- | ------------------ |
| ≥60   | ESC_RUNTIME_PROBE  |
| ≥90   | ESC_RUNTIME_ACTIVE |

---

## 8. Response Strategy

### 8.1 PROBE

* Annotate pod
* Emit event → SIEM
* Increase pod risk profile
* Enable deep monitoring (tight seccomp / audit)

### 8.2 ACTIVE

* Kill container / cordon node (configurable)
* Block via NetworkPolicy
* Snapshot forensic data

---

## 9. Database / Schema bổ sung

### 9.1 pod_risk_profile

```json
{
  "pod_uid": "",
  "static_risk": 40,
  "runtime_score": 75,
  "capabilities": ["ESC_RUNTIME_PROBE"],
  "last_event_ts": ""
}
```

### 9.2 runtime_event

```json
{
  "pod_uid": "",
  "syscall": "setns",
  "target": "/proc/1/exe",
  "capability": "SYS_ADMIN",
  "ts": ""
}
```

---

## 10. Unit Test

### 10.1 Rule Engine

* Input: syscall event mock
* Expect: rule match / no match

### 10.2 Scoring

* Feed event sequence
* Validate score threshold

---

## 11. End-to-End Test

### Scenario 1: Benign privileged pod

* hostPID=true
* No syscall abuse
  → No alert

### Scenario 2: Probe simulation

* hostPID=true
* `nsenter --target 1 --mount`
  → ESC_RUNTIME_PROBE

### Scenario 3: Exploit-like behavior

* overwrite runc binary
  → ESC_RUNTIME_ACTIVE + response

---

## 12. Kết quả cần đạt

* Phát hiện **pre-exploit behavior**
* Không phụ thuộc CVE feed
* Giảm false positive so với capability-only
* Có thể mở rộng sang eBPF

---

## 13. Nhận xét thẳng thắn

> Capability-only detection **không đủ**.
> Runtime CVE không chờ SBOM cập nhật.
> Probe luôn xảy ra **trước exploit** – nếu không bắt được probe, bạn chỉ đang xem hậu quả.

Spec này đủ để **bắt đầu dev ngay**, không màu mè, không lý thuyết suông.
