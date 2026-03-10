# PodCapabilityEngine – Storage & Processing Improvement Specification

## 1. Mục tiêu tài liệu

Tài liệu này mô tả **đặc tả kỹ thuật chi tiết** cho việc cải tiến cơ chế **xử lý và lưu trữ dữ liệu** phục vụ PodCapabilityEngine (PCE), nhằm:

* Loại bỏ ghost data, risk sai ngữ cảnh
* Chuẩn hóa lifecycle của Pod
* Chuẩn bị nền tảng dữ liệu cho Attack Graph & Attack Path
* Giữ forensic fidelity nhưng vẫn kiểm soát được complexity

Tài liệu **dev-ready**, dùng trực tiếp để implement.

---

## 2. Hiện trạng & vấn đề cốt lõi

### 2.1 Hiện trạng

* Pod được định danh bằng `pod_uid`
* Soft-delete Pod khi DELETE event
* Runtime events, capabilities, risk profile vẫn tồn tại
* Không có ranh giới lifecycle rõ ràng giữa các lần recreate

### 2.2 Vấn đề

| Vấn đề                       | Hệ quả                         |
| ---------------------------- | ------------------------------ |
| Không có lifecycle boundary  | Ghost risk, attack-path sai    |
| Runtime event ở dạng raw     | PCE rule phức tạp, khó explain |
| Capability dạng boolean      | Không thể infer chain attack   |
| Không có continuity workload | Attack graph bị vỡ vụn         |

---

## 3. Nguyên tắc thiết kế

1. **Forensic-first**: không xóa dữ liệu sớm
2. **Explicit lifecycle**: pod phải có trạng thái rõ ràng
3. **Semantic > Raw**: chuyển raw event thành signal
4. **Backward compatible**: không phá flow hiện tại

---

## 4. Pod Lifecycle Normalization

### 4.1 Giới thiệu `pod_instances`

Tách khái niệm Pod thành **instance có lifecycle** độc lập với workload.

### 4.2 Schema đề xuất

```sql
CREATE TABLE pod_instances (
  pod_uid TEXT PRIMARY KEY,
  workload_id TEXT,
  namespace TEXT NOT NULL,
  name TEXT NOT NULL,
  generation INT DEFAULT 1,
  started_at TIMESTAMP NOT NULL,
  terminated_at TIMESTAMP NULL,
  status TEXT CHECK (status IN ('active', 'terminated')) NOT NULL
);
```

### 4.3 Quy tắc xử lý

* Pod ADD → insert `pod_instances` (status=active)
* Pod DELETE → set `terminated_at`, status=terminated
* Mọi logic PCE / Risk **chỉ xử lý status=active**

---

## 5. Runtime Signal Layer

### 5.1 Vấn đề runtime_events hiện tại

* Chỉ chứa syscall + path
* Không có semantic
* PCE phải suy diễn mỗi lần

### 5.2 Giới thiệu `runtime_signals`

Signal là **runtime behavior đã được phân loại**.

### 5.3 Schema đề xuất

```sql
CREATE TABLE runtime_signals (
  id BIGSERIAL PRIMARY KEY,
  pod_uid TEXT NOT NULL,
  signal_type TEXT NOT NULL,
  category TEXT NOT NULL,
  confidence FLOAT DEFAULT 0.5,
  evidence JSONB NOT NULL,
  created_at TIMESTAMP DEFAULT now()
);
```

### 5.4 Ví dụ signal

```json
{
  "signal_type": "PROC_ROOT_PIVOT",
  "category": "ESCAPE",
  "confidence": 0.9,
  "evidence": {
    "syscall": "openat",
    "path": "/proc/1/root",
    "capability": "SYS_ADMIN"
  }
}
```

### 5.5 Flow xử lý

1. Agent gửi raw runtime event
2. Core normalize → runtime_signal
3. PCE chỉ đọc runtime_signal

---

## 6. Capability State Machine

### 6.1 Capability không phải boolean

Mỗi capability có **trạng thái tiến triển**.

### 6.2 State model

```
DETECTED → CONFIRMED → EXPLOITED → CHAINED
```

### 6.3 Schema đề xuất

```sql
CREATE TABLE pod_capabilities (
  pod_uid TEXT,
  capability_id TEXT,
  state TEXT CHECK (state IN ('detected','confirmed','exploited','chained')),
  confidence FLOAT DEFAULT 0.5,
  first_seen_at TIMESTAMP,
  last_seen_at TIMESTAMP,
  evidence JSONB,
  PRIMARY KEY (pod_uid, capability_id)
);
```

### 6.4 Quy tắc update

* Static config → DETECTED
* Runtime signal → CONFIRMED
* Multiple corroborating signals → EXPLOITED
* Used as edge in attack-path → CHAINED

---

## 7. Cleanup & Retention Strategy

### 7.1 Nguyên tắc

* Không xóa dữ liệu active
* Không xóa capability có state ≥ CONFIRMED

### 7.2 Retention đề xuất

| Data             | Điều kiện xóa             |
| ---------------- | ------------------------- |
| runtime_events   | pod terminated > 90 ngày  |
| runtime_signals  | pod terminated > 180 ngày |
| pod_capabilities | pod terminated > 180 ngày |
| risk_profiles    | pod terminated > 30 ngày  |

---

## 8. Impact tới PodCapabilityEngine

Sau cải tiến:

* PCE chỉ xử lý pod_instances active
* Rule đơn giản, deterministic
* Capability có confidence & state
* Chuẩn bị sẵn input cho Attack Graph

---

## 9. Thứ tự triển khai khuyến nghị

### Phase 1.5 (song song PCE)

* [ ] Add pod_instances
* [ ] Refactor logic dùng status=active

### Phase 2

* [ ] runtime_signals
* [ ] Adapter raw → signal

### Phase 3

* [ ] Capability state machine
* [ ] Risk scoring theo state

### Phase 4

* [ ] Attack graph inference

---

## 10. Kết luận

Cải tiến này **không phải refactor lớn**, mà là:

> Chuẩn hóa semantic layer để PCE, Risk Engine và Attack Graph hoạt động chính xác.

Nếu không thực hiện, hệ thống sẽ **sai ngầm** khi scale logic attack-path.

---

**Status**: Ready for implementation
