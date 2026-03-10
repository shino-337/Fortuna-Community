# Phase 1 – PodCapabilityEngine (PCE)

## 1. Mục tiêu & Nguyên tắc Freeze Spec

### 1.1 Mục tiêu Phase 1

PodCapabilityEngine (PCE) có **một nhiệm vụ duy nhất**:

> Trả lời câu hỏi: **“Nếu attacker chiếm được Pod này, họ có thể LÀM ĐƯỢC GÌ tiếp theo trong cluster?”**

PCE **KHÔNG**:

* Đánh giá CVE
* Dự đoán exploit
* Vẽ attack graph hoàn chỉnh

PCE **CÓ**:

* Chuẩn hoá *offensive capability* của Pod
* Làm input bắt buộc cho Attack Graph Phase 2
* Deterministic, explainable, rule-based

### 1.2 Nguyên tắc thiết kế (Freeze)

* Static analysis (YAML / runtime metadata)
* Không ML, không scoring mơ hồ
* Mỗi capability phải:

  * Có điều kiện kích hoạt rõ ràng
  * Mapping được sang MITRE ATT&CK
  * Có impact thực tế cho lateral / privilege escalation

---

## 2. Capability Taxonomy (Freeze v1)

### 2.1 Capability Group

| Group | Ý nghĩa tấn công                     |
| ----- | ------------------------------------ |
| FS    | File system / host filesystem access |
| NET   | Network pivot / sniffing             |
| API   | Kubernetes API abuse                 |
| ESC   | Container → Node escape              |
| ID    | Identity & credential abuse          |
| CTRL  | Control-plane impact                 |

---

## 3. Rule Set Tối Thiểu (Freeze)

### 3.1 FS – Filesystem Capabilities

#### FS_HOST_RW

**Ý nghĩa:** Full read/write vào node filesystem

**Trigger conditions**:

* volume.hostPath != nil
* mountPath == "/" OR path startsWith "/"

**ATT&CK**:

* T1611 – Escape to Host

---

### 3.2 ESC – Privilege Escalation

#### ESC_PRIVILEGED

* securityContext.privileged == true

ATT&CK:

* T1611

---

#### ESC_KERNEL

* hostPID == true OR hostIPC == true

ATT&CK:

* T1611
* T1068

---

### 3.3 NET – Network Capabilities

#### NET_HOST_NETWORK

* hostNetwork == true

Impact:

* Bypass network policy
* Sniff / pivot traffic

ATT&CK:

* T1046 – Network Service Discovery
* T1595 – Active Scanning

---

### 3.4 API – Kubernetes API Abuse

#### API_K8S_WRITE

* ServiceAccount bound to Role/ClusterRole
* verbs contains create/update/delete/patch

ATT&CK:

* T1609 – Container Administration Command

---

### 3.5 ID – Identity Abuse

#### ID_TOKEN_STEAL

* automountServiceAccountToken == true

ATT&CK:

* T1528 – Steal Application Access Token

---

### 3.6 CTRL – Control Plane Risk

#### CTRL_CONTROL_PLANE_POD

* namespace == kube-system

ATT&CK:

* T1496 – Resource Hijacking

---

## 4. Capability Data Model (FINAL)

```json
{
  "pod_uid": "string",
  "namespace": "string",
  "capabilities": [
    {
      "capability_id": "ESC_PRIVILEGED",
      "group": "ESC",
      "severity": "CRITICAL",
      "evidence": {
        "field": "securityContext.privileged",
        "value": true
      },
      "mitre_techniques": ["T1611"]
    }
  ],
  "evaluated_at": "timestamp"
}
```

---

## 5. Evaluation Logic (Dev-Ready)

### 5.1 Input

* NormalizedPod (from correlator)
* ServiceAccount + RBAC snapshot

### 5.2 Processing

1. Iterate rule set (static)
2. If condition match → emit capability
3. Dedup by capability_id
4. Persist snapshot

### 5.3 Output

* DB: pod_capabilities
* Event: fortuna.pod.capability.created

---

## 6. DB Schema Proposal

### pod_capabilities

| field         | type      |
| ------------- | --------- |
| id            | uuid      |
| pod_uid       | string    |
| capability_id | string    |
| group         | string    |
| severity      | string    |
| evidence      | jsonb     |
| mitre         | text[]    |
| created_at    | timestamp |

Index:

* (pod_uid)
* (capability_id)

---

## 7. MITRE ATT&CK Mapping Summary

| Capability             | Techniques   |
| ---------------------- | ------------ |
| FS_HOST_RW             | T1611        |
| ESC_PRIVILEGED         | T1611        |
| ESC_KERNEL             | T1611, T1068 |
| NET_HOST_NETWORK       | T1046, T1595 |
| API_K8S_WRITE          | T1609        |
| ID_TOKEN_STEAL         | T1528        |
| CTRL_CONTROL_PLANE_POD | T1496        |

---

## 8. Vì sao Freeze ở đây là đúng

Nếu mở rộng thêm lúc này:

* Attack graph sẽ nhiễu
* Risk bị double-count
* Agent/Core coupling tăng

Phase 1 chỉ cần trả lời **"attacker có quyền gì"**, không phải **"attacker sẽ làm gì"**.

---

## 9. Bước Dev Tiếp Theo (Concrete)

1. Implement PCE worker
2. Add pod_capabilities table
3. Emit capability events
4. Write unit test per rule
5. Freeze schema → Phase 2

---

## 10. Phase 2 Readiness Checklist (Update)

- ✅ PCE API endpoints (list, detail, summary, trends)
- ✅ Dashboard integration (summary + trend + drill-down)
- ✅ Scheduler toggle/config (enable/disable + interval)
- ✅ Unit tests for PCE rules
- ✅ Verified end-to-end with violation pod

**Phase 2 inputs now ready:**
- Capability facts persisted and queryable
- Trend + summary APIs in place
- UI surfaces for validation

---

> Nếu Phase 1 sai, Phase 2 vô nghĩa.
> Nếu Phase 1 rõ, Attack Graph gần như tự hiện ra.
