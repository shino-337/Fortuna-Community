# PodCapabilityEngine – Phase 1.5

## Mục tiêu tài liệu

Tài liệu này mô tả **các điều chỉnh bắt buộc cần bổ sung ngay** cho PodCapabilityEngine (PCE) trong Phase 1.5 nhằm:

* Ổn định logic suy luận capability
* Chuẩn hóa vòng đời capability & state
* Tránh nợ kỹ thuật khi mở rộng sang attack graph / attack path

Phạm vi tài liệu **chỉ tập trung vào Phase 1.5**, không bao gồm full attack graph Phase 2.

---

## 1. CapabilityStateController

### 1.1. Vấn đề hiện tại

* Capability state (`detected / confirmed / exploited / chained`) đang bị ảnh hưởng bởi nhiều nguồn:

  * Static config
  * Runtime signal
  * Risk scoring
* Không có **single owner** chịu trách nhiệm cập nhật state

Điều này dẫn tới:

* Race condition
* State không nhất quán
* Khó debug / giải thích

---

### 1.2. Nguyên tắc thiết kế

**Chỉ duy nhất một component được phép cập nhật capability state:**

> **CapabilityStateController (CSC)** – nằm trong Core / PCE

Các thành phần khác:

* Agent
* Runtime probe
* Signal adapter

❌ **Tuyệt đối không được update state trực tiếp**

---

### 1.3. Trách nhiệm CapabilityStateController

CSC chịu trách nhiệm:

* Nhận input:

  * Static capability (spec)
  * Runtime signals
  * Capability metadata
  * Historical state
* Áp dụng rule promotion
* Cập nhật capability state
* Ghi nhận evidence & timestamps

---

### 1.4. Luồng xử lý chuẩn

```
Agent → raw runtime event
Core → normalize → runtime_signal
PCE → CapabilityStateController
CSC → evaluate → update capability.state
DB → persist
Dashboard → read-only
```

---

## 2. capability_metadata (BẮT BUỘC)

### 2.1. Mục đích

`capability_metadata` cung cấp **ngữ nghĩa (semantic layer)** cho capability, phục vụ:

* Inference rules
* Attack step mapping
* Explainability

Không có metadata → logic inference sẽ hardcode và khó mở rộng.

---

### 2.2. Schema đề xuất

```yaml
capability_metadata:
  id: ESC_HOSTPATH_NODE
  domain: ESC
  category: Privilege Escalation
  description: Pod có khả năng truy cập filesystem node thông qua hostPath

  severity_base: CRITICAL
  confidence_base: 0.7

  preconditions:
    - ESC_PRIV_POD

  produces_attack_steps:
    - NODE_FS_WRITE

  expires_with_instance: true
  supports_runtime_promotion: true
```

---

### 2.3. Nguyên tắc sử dụng

* Mỗi `capability_id` **phải có metadata tương ứng**
* Metadata **không thay đổi theo runtime**
* Runtime chỉ ảnh hưởng tới:

  * state
  * confidence
  * evidence

---

## 3. Signal → State Promotion Rules

### 3.1. Vấn đề

Hiện tại runtime signal chỉ được ghi nhận, chưa có rule rõ ràng để:

* Khi nào nâng state
* Nâng lên mức nào

---

### 3.2. Định nghĩa Runtime Signal

```yaml
runtime_signal:
  signal_type: PROC_ROOT_PIVOT
  syscall: openat
  target: /proc/1/root
  capability_hint: SYS_ADMIN
  confidence: 0.9
  timestamp: ...
```

---

### 3.3. Promotion Rule Schema

```yaml
promotion_rule:
  capability_id: ESC_HOSTPATH_NODE
  signal_type: PROC_ROOT_PIVOT

  conditions:
    min_occurrences: 1
    required_capability:
      - SYS_ADMIN

  promote_to: confirmed
```

---

### 3.4. Ví dụ bảng rule

| Capability        | Signal                      | Điều kiện | State mới |
| ----------------- | --------------------------- | --------- | --------- |
| ESC_HOSTPATH_NODE | PROC_ROOT_PIVOT             | ≥1 lần    | confirmed |
| ESC_HOSTPATH_NODE | PROC_ROOT_PIVOT + SYS_ADMIN | ≥2 lần    | exploited |
| ID_TOKEN_POD      | TOKEN_READ                  | ≥1        | confirmed |

---

### 3.5. Nguyên tắc áp dụng

* Promotion **chỉ đi lên**, không rollback
* Không downgrade state tự động
* Mọi promotion phải lưu:

  * triggering signal
  * timestamp

---

## 4. Minimal AttackStep Model (ưu tiên thấp)

### 4.1. Mục đích

AttackStep là lớp trung gian giữa:

* Capability (điều kiện)
* Attack Path (chuỗi tấn công)

Phase 1.5 **chỉ cần minimal model**, chưa cần graph engine.

---

### 4.2. AttackStep Schema (tối thiểu)

```yaml
attack_step:
  id: NODE_FS_WRITE
  description: Attacker có thể ghi vào filesystem node

  requires_capabilities:
    - ESC_HOSTPATH_NODE

  enables:
    - NODE_PERSISTENCE
    - KUBELET_CRED_ACCESS
```

---

### 4.3. Quan hệ Capability → AttackStep

* Capability **không phải attack step**
* Capability đạt state `exploited` → sinh attack step tương ứng

Ví dụ:

```
ESC_HOSTPATH_NODE (exploited)
   → AttackStep: NODE_FS_WRITE
```

---

### 4.4. Sử dụng trong Phase 1.5

* Chỉ dùng để:

  * Hiển thị dashboard (textual)
  * Chuẩn bị dữ liệu cho Phase 2

Không:

* Tự động chain phức tạp
* Không cross-resource inference

---

## 5. Tóm tắt điều chỉnh Phase 1.5

### BẮT BUỘC

* CapabilityStateController
* capability_metadata
* Promotion rules tập trung trong PCE

### NÊN LÀM

* Minimal AttackStep model

### CHƯA CẦN

* Full attack graph
* Operator-driven confirmation
* Auto remediation

---

## 6. Nguyên tắc thiết kế xuyên suốt

* Deterministic > Clever
* Explainable > Automated
* Single-owner state
* Dashboard = read-only insight

---

*Tài liệu này là nền tảng bắt buộc trước khi triển khai Attack Graph & Attack Path Dashboard (Phase 2).*
