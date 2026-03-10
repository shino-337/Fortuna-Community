# PodCapabilityEngine – Phase 1.5 Enhancement Specification

## 1. Mục tiêu Phase 1.5

Phase 1.5 tập trung **hoàn thiện PodCapabilityEngine (PCE)** để:

* Chuẩn hóa Capability ID (ổn định, không thay đổi theo implementation)
* Giới thiệu **Attack Step** – đơn vị logic trung gian giữa capability và attack graph
* Định nghĩa inference rules rõ ràng, deterministic
* Sẵn sàng tích hợp với Attack Graph Engine (Phase 2)

Phase này được triển khai **song song** với việc điều chỉnh storage & processing.

---

## 2. Chuẩn hóa Capability ID

### 2.1 Vấn đề hiện tại

* Capability ID đang mang tính implementation-driven
* Khó maintain khi mở rộng rule
* Dễ trùng hoặc mơ hồ khi map ATT&CK

### 2.2 Nguyên tắc đặt Capability ID

```
<DOMAIN>_<VECTOR>_<SCOPE>[_<QUALIFIER>]
```

* DOMAIN: ESC, ID, NET, FS, API, OBS
* VECTOR: PRIV, TOKEN, NET, PROC, API, RBAC
* SCOPE: POD, NODE, CLUSTER
* QUALIFIER (optional): RUNTIME, STATIC, WRITE, READ

### 2.3 Capability ID tối thiểu (Phase 1.5)

| Capability ID          | Mô tả                      | ATT&CK    |
| ---------------------- | -------------------------- | --------- |
| ESC_PRIV_POD           | Privileged container       | T1611     |
| ESC_HOSTPID_POD        | hostPID enabled            | T1611     |
| ESC_HOSTPATH_NODE      | hostPath mount             | T1611.001 |
| ESC_RUNTIME_PROC_ROOT  | /proc/1/root pivot         | T1611.001 |
| ID_TOKEN_POD           | ServiceAccount token steal | T1528     |
| API_RBAC_WRITE_CLUSTER | RBAC write access          | T1068     |
| NET_HOSTNETWORK        | hostNetwork                | T1609     |

---

## 3. Attack Step – Khái niệm trung gian

### 3.1 Vì sao cần Attack Step

* Capability ≠ hành động tấn công
* Attack Graph không nên nối trực tiếp capability

Attack Step = **"Attacker có thể thực hiện hành động gì tiếp theo"**

---

### 3.2 Attack Step Schema

```yaml
AttackStep:
  step_id: string
  description: string
  category: ESCAPE | LATERAL | CREDENTIAL | IMPACT | RECON
  required_capabilities:
    - capability_id
  confidence: float
  mitre_techniques:
    - Txxxx
```

### 3.3 Attack Step ví dụ

```yaml
step_id: STEP_POD_TO_NODE_FS
category: ESCAPE
description: Read/write host filesystem from container
required_capabilities:
  - ESC_HOSTPATH_NODE
  - ESC_PRIV_POD
mitre_techniques:
  - T1611.001
confidence: 0.9
```

---

## 4. Attack Step Persistence

### 4.1 Schema đề xuất

```sql
CREATE TABLE pod_attack_steps (
  pod_uid TEXT,
  step_id TEXT,
  confidence FLOAT,
  evidence JSONB,
  created_at TIMESTAMP,
  PRIMARY KEY (pod_uid, step_id)
);
```

---

## 5. Inference Rules

### 5.1 Nguyên tắc inference

* Deterministic (không ML)
* Explainable
* Không suy đoán nếu thiếu capability

---

### 5.2 Rule types

#### 5.2.1 Capability → Attack Step

```text
IF ESC_PRIV_POD AND ESC_HOSTPATH_NODE
THEN STEP_POD_TO_NODE_FS
```

#### 5.2.2 Runtime Confirmation

```text
IF STEP_POD_TO_NODE_FS
AND runtime_signal == PROC_ROOT_PIVOT
THEN confidence += 0.2
```

#### 5.2.3 Chaining Preparation

```text
IF STEP_POD_TO_NODE_FS
THEN unlock potential steps:
  - STEP_NODE_CRED_DUMP
  - STEP_NODE_KUBELET_ACCESS
```

---

## 6. Luồng xử lý PCE Phase 1.5

```text
Normalized Pod + RBAC
        ↓
Capability Detection (static + runtime)
        ↓
Capability State Update
        ↓
Attack Step Inference
        ↓
Persist pod_attack_steps
        ↓
Publish fortuna.pod.attackstep.created
```

---

## 7. Tác động tới các component khác

### 7.1 Risk Engine

* Risk score dựa trên Attack Step thay vì raw capability
* Step category → weight

### 7.2 Attack Graph Engine (Phase 2)

* Node = Attack Step
* Edge = Inference unlock

---

## 8. Thứ tự triển khai khuyến nghị

### Phase 1.5 – tuần tự

* [ ] Freeze Capability ID list
* [ ] Implement Attack Step schema
* [ ] Implement inference engine
* [ ] Emit attackstep events

---

## 9. Anti-patterns cần tránh

❌ Map capability trực tiếp thành attack graph
❌ Capability name phụ thuộc code path
❌ Inference dựa trên heuristic mơ hồ

---

## 10. Kết luận

Phase 1.5 biến PCE từ **"capability detector"** thành **"attack potential analyzer"**.

Nếu làm đúng:

* Attack graph Phase 2 chỉ còn là nối node
* Dashboard explainable
* Risk score có ngữ cảnh attacker

---

**Status**: Ready for implementation (Phase 1.5)
