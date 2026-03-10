# ĐÁNH GIÁ HIỆN TRẠNG AGENT – CORE – DATABASE & ĐỀ XUẤT ĐIỀU CHỈNH TỔNG THỂ

---

## 0. Executive Summary (Nói thẳng)

Hệ thống hiện tại **đã vượt qua giai đoạn MVP scanner**, nhưng đang mắc kẹt ở trạng thái:

* Agent làm quá nhiều việc *không liên quan đến attack reasoning*
* Core mạnh ở **ingest + rule evaluation**, yếu ở **context & inference**
* Database mang tính **log + index**, chưa phải **knowledge store cho attacker view**

Nếu giữ nguyên kiến trúc hiện tại:

* Attack-path chỉ có thể *vẽ minh hoạ*, không thể *suy luận thật*
* Agent ngày càng phình to nhưng giá trị tăng chậm

---

## 1. Đánh giá hiện trạng Agent

### 1.1 Điểm mạnh (giữ nguyên)

* Node-local SBOM extraction (đúng thiết kế, không cần cluster-wide)
* Auto-sync RBAC + inventory qua HTTP (ổn định, dễ kiểm soát)
* Không làm CVE matching tại agent (đúng vai trò)

### 1.2 Vấn đề cốt lõi

#### ❌ Agent thiếu **runtime / security context signals**

Agent **không thu thập**:

* PodSpec security flags (privileged, hostPID, hostNetwork…)
* Volume semantics (hostPath, projected SA token)
* Node placement intent (nodeName, tolerations, taints)

→ Core **không có dữ liệu để suy luận capability**.

#### ❌ Legacy code gây nhiễu

* collector / converter / retry tồn tại nhưng không dùng
* tăng cognitive load, giảm độ tin cậy codebase

---

## 2. Đánh giá hiện trạng Core

### 2.1 Điểm mạnh

* Pipeline SBOM → CVE → Insight rõ ràng
* Dedup logic tốt (SBOM, CVE, insight)
* Risk engine cho RBAC đã ổn định

### 2.2 Khoảng trống nghiêm trọng

#### ❌ Core **không có lớp Capability**

* Risk = kết quả rule
* Không có khái niệm: "Pod này CÓ THỂ làm gì"

#### ❌ Insight bị flatten

* Vulnerability, RBAC, runtime risk đều dồn vào 1 bảng
* Không phân biệt:

  * fact (capability)
  * inference (attack path)
  * outcome (impact)

#### ❌ Graph chỉ là storage, không phải reasoning engine

* Có lưu edge
* Không có inference rule
* Không có confidence / evidence

---

## 3. Đánh giá Database hiện tại

### 3.1 Nhận định tổng thể

Database hiện tại phù hợp cho:

* Inventory
* CVE correlation
* Rule-based insight

**Chưa phù hợp cho attack-path reasoning.**

### 3.2 Thiếu các lớp dữ liệu sau

| Lớp         | Trạng thái | Hệ quả                                |
| ----------- | ---------- | ------------------------------------- |
| Capability  | ❌          | Không suy luận được attack            |
| Attack Edge | ❌          | Không nối được chuỗi                  |
| Evidence    | ❌          | Dashboard thiếu thuyết phục           |
| Confidence  | ❌          | Không phân biệt giả định vs chắc chắn |

---

## 4. Đề xuất điều chỉnh tổng thể (Architecture Shift)

### 4.1 Nguyên tắc thiết kế mới (BẮT BUỘC)

1. **Agent chỉ thu thập FACT**
2. **Core suy luận (Capability → Attack)**
3. **Database lưu KNOWLEDGE, không chỉ LOG**
4. **Dashboard hiển thị PATH, không list**

---

## 5. Điều chỉnh Agent (CỤ THỂ)

### 5.1 Bổ sung dữ liệu Agent phải gửi

#### Pod Runtime Security Context

```json
{
  "securityContext": {
    "privileged": true,
    "hostPID": true,
    "hostNetwork": false,
    "capabilities": ["NET_RAW"]
  },
  "volumes": [
    {"type": "hostPath", "path": "/"}
  ]
}
```

#### Node placement intent

* nodeName
* tolerations
* affinity

👉 Không đánh giá, **chỉ gửi raw data**.

### 5.2 Dọn dẹp bắt buộc

* ❌ Remove `internal/collector`
* ❌ Remove `internal/converter`
* ❌ Remove unused retry / models

---

## 6. Điều chỉnh Core (TRỌNG TÂM)

### 6.1 Thêm PodCapabilityEngine (new module)

Pipeline mới:

```
PodSpec + Runtime
  → Capability rules
  → pod_capabilities table
  → publish capability.evaluated
```

### 6.2 Tách Insight thành 3 lớp

| Layer      | Bảng                        |
| ---------- | --------------------------- |
| Capability | pod_capabilities            |
| Attack     | attack_edges / attack_paths |
| Outcome    | insights                    |

### 6.3 Attack Graph Builder

* Build edge **chỉ khi có capability**
* Gắn evidence + confidence
* Không suy luận mơ hồ

---

## 7. Điều chỉnh Database (Schema proposal)

### 7.1 Bảng mới bắt buộc

#### pod_capabilities

| Field      | Type  |
| ---------- | ----- |
| pod_uid    | text  |
| capability | enum  |
| severity   | enum  |
| evidence   | jsonb |
| confidence | enum  |

#### attack_edges

| from | to | edge_type | evidence | confidence |

---

## 8. Luồng logic tổng thể (NEW)

```text
Agent
  → Pod raw facts
  → Core correlator
  → PodCapabilityEngine
  → AttackGraph inference
  → Insight synthesis
  → API / Dashboard
```

---

## 9. Lộ trình triển khai đề xuất

### Phase 1 – Capability foundation (BẮT BUỘC)

* Agent gửi PodSpec security fields
* Core implement PodCapabilityEngine

### Phase 2 – Attack inference

* Attack edges
* Pod → Node / SA / Cluster

### Phase 3 – Dashboard

* Read-only attack-path
* Evidence-first UI

---

## 10. Kết luận thẳng thắn

Nếu không làm các bước trên:

* Core = scanner nâng cao
* Attack-path = slide minh hoạ

Nếu làm đúng:

* Core trở thành **attack-aware reasoning engine**
* Agent gọn, Core thông minh, DB có giá trị dài hạn

