# MITRE ATT&CK – Container Escape Runtime Detection Spec

## 1. Mục tiêu

Bổ sung **Runtime Detection Capability** cho Agent nhằm phát hiện **Container / Pod Escape** theo MITRE ATT&CK (Container & Kubernetes) và cung cấp tín hiệu đầu vào (high-signal) cho **PCE (Policy / Pod Capability Engine)** để scoring và response.

Phạm vi:

* Không khai thác CVE
* Không exploit thực sự
* Detect **pre-condition**, **attempt**, và **post-condition** của escape

---

## 2. MITRE ATT&CK Mapping (Escape Cluster)

| MITRE ID  | Tactic               | Technique                  | Ý nghĩa          |
| --------- | -------------------- | -------------------------- | ---------------- |
| T1611     | Privilege Escalation | Escape to Host             | Container escape |
| T1611.001 | PrivEsc              | Exploit Container Runtime  | runc/containerd  |
| T1611.002 | PrivEsc              | Abuse Privileged Container | SYS_ADMIN        |
| T1610     | Defense Evasion      | Modify Container Runtime   | Namespace / FS   |
| T1055     | PrivEsc              | Process Injection          | Namespace pivot  |

---

## 3. Detection Strategy (3-Layer Model)

```
[Kernel / eBPF]
      ↓
[Agent Runtime Detector]
      ↓
[PCE Scoring & Response]
```

Agent chỉ **observe + normalize**, không quyết định block.

---

## 4. Runtime Signals theo MITRE Technique

### 4.1 T1611.001 – Exploit Container Runtime

#### Signal: PROC_ROOT_PIVOT

**Syscall**:

* open / openat
* stat / readlink

**Target Paths**:

* /proc/1/root
* /proc/self/exe
* /proc/1/exe

**Rule**:

```
IF
  syscall IN [open, openat, stat, readlink]
  AND path MATCHES /proc/*/root OR /proc/*/exe
THEN
  emit PROC_ROOT_PIVOT
```

---

### 4.2 T1611.002 – Abuse Privileged Container

#### Signal: CAPABILITY_MISUSE

**Syscall**:

* mount
* setns
* pivot_root

**Condition**:

* Capability SYS_ADMIN
* Pod spec không yêu cầu capability này

**Rule**:

```
IF
  syscall IN [mount, setns, pivot_root]
  AND capability == SYS_ADMIN
  AND pod.capability_expected == false
THEN
  emit CAPABILITY_MISUSE
```

---

### 4.3 T1055 – Process / Namespace Injection

#### Signal: NAMESPACE_ESCAPE

**Syscall**:

* setns
* unshare
* clone

**Rule**:

```
IF
  syscall == setns
  AND target_namespace NOT IN pod.namespaces
THEN
  emit NAMESPACE_ESCAPE
```

---

### 4.4 T1610 – Filesystem Manipulation

#### Signal: FS_ESCAPE_ATTEMPT

**Syscall**:

* mount
* pivot_root

**Target FS**:

* /proc
* /sys
* /dev
* /run

**Rule**:

```
IF
  syscall IN [mount, pivot_root]
  AND mount_target IN [/proc, /sys, /dev, /run]
THEN
  emit FS_ESCAPE_ATTEMPT
```

---

## 5. Active Runtime Probe (Optional, Early Phase)

### Trigger

* Pod age < 30s
* Runtime = runc / containerd
* Capability ESC_RUNTIME_PROBE = true

### Probe Actions (Read-only)

```
stat("/proc/1/root")
readlink("/proc/self/exe")
```

### Result

```
IF probe succeeds (no EPERM)
THEN emit PROBE_ESCAPE_POSSIBLE
```

---

## 6. Event Schema (Agent → PCE)

```json
{
  "event_type": "RUNTIME_ESCAPE_SIGNAL",
  "mitre_technique": "T1611.001",
  "signal": "PROC_ROOT_PIVOT",
  "severity": "HIGH",
  "pod": {
    "name": "api-xyz",
    "namespace": "prod",
    "uid": "..."
  },
  "runtime": "containerd",
  "syscall": "openat",
  "target": "/proc/1/root",
  "capabilities": ["SYS_ADMIN"],
  "timestamp": 1737429012
}
```

### 6.1 Agent Ingestion (Phase 1)

Trong Phase 1, Agent đọc runtime signal từ file JSONL (1 event / line) do Falco/auditd/custom sensor ghi ra.

Env:
- `RUNTIME_EVENTS_ENABLED=true`
- `RUNTIME_EVENTS_PATH=/var/log/fortuna/runtime-events.log`
- `RUNTIME_EVENTS_POLL=5s`

Agent sẽ POST batch events lên Core:
`POST /api/v1/runtime-events`

---

## 7. Scoring Input cho PCE

| Signal                | MITRE       | Base Score |
| --------------------- | ----------- | ---------- |
| PROC_ROOT_PIVOT       | T1611.001   | 90         |
| FS_ESCAPE_ATTEMPT     | T1610       | 95         |
| NAMESPACE_ESCAPE      | T1055       | 85         |
| CAPABILITY_MISUSE     | T1611.002   | 60         |
| PROBE_ESCAPE_POSSIBLE | Pre-Exploit | 70         |

---

## 8. Noise Control & Dedup

* Deduplicate theo (podUID + signal + target)
* Suppress repeat events trong 10s
* Ignore initContainers nếu allowlisted

---

## 9. Unit Test

### Input

* Fake syscall event: open("/proc/1/root")

### Expected

* Emit PROC_ROOT_PIVOT
* mitre_technique = T1611.001

---

## 10. End-to-End Test

1. Deploy pod có SYS_ADMIN
2. Thực hiện read /proc/1/root
3. Agent emit signal
4. PCE score >= CRITICAL threshold
5. Response triggered (alert / quarantine)

---

## 11. Kết quả cần đạt

* Detect escape attempt **trước khi exploit hoàn chỉnh**
* Không phụ thuộc CVE database
* Mapping rõ ràng MITRE ATT&CK
* Agent đơn giản, PCE quyết định

---

## 12. Nhận xét (nói thẳng)

* Capability-only là **chưa đủ**
* CVE scanning là **quá muộn**
* Runtime behavior + MITRE mapping là con đường đúng

**Spec này đủ để bắt đầu dev ngay.**
