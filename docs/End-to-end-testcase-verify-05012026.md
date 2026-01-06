# KSAM End-to-End (E2E) Security Test Specification

## 1. Mục tiêu

Tài liệu này chuẩn hóa **toàn bộ testcase E2E** để xác minh luồng xử lý từ **Agent → Core → Event Bus (NATS JetStream) → Worker → Database → Query API** cho các năng lực chính:

* SBOM ingestion
* CVE detection & enrichment
* Insight generation
* Độ bền hệ thống (resilience) thông qua Chaos Testing

Mục tiêu cuối cùng: đảm bảo hệ thống **đúng – đủ – bền – quan sát được**, sẵn sàng cho môi trường production.

---

## 2. Phạm vi & Nguyên tắc

### 2.1 Phạm vi

* Agent
* Core Ingest API
* Admission / Policy / Insight Worker
* NATS JetStream
* Database (raw + aggregated)
* Query API

**Ngoài phạm vi:** UI, IAM, billing.

### 2.2 Nguyên tắc đánh giá

* Fail ở bất kỳ bước nào → **Testcase FAIL**
* Không chấp nhận “eventually maybe correct” ngoài SLA cho phép
* Ưu tiên data consistency và async correctness

---

## 3. Môi trường Test

| Thành phần     | Yêu cầu                         |
| -------------- | ------------------------------- |
| Core           | Running, health = OK            |
| Agent          | Có thể cấu hình gửi SBOM        |
| NATS JetStream | Stream & Consumer active        |
| Worker         | ≥ 1 replica                     |
| Database       | Clean state hoặc known baseline |
| CVE DB         | Đã sync dữ liệu                 |

---

## 4. Testcase E2E Chuẩn Hóa

> **Ghi chú chung về CVE dùng cho test:**
>
> * CVE mặc định dùng cho các testcase happy path: **CVE-2021-3711** (OpenSSL 1.1.1f – HIGH).
> * Đây là CVE phổ biến, ổn định, xuất hiện trong hầu hết các CVE database (NVD, osv, Trivy, Grype), phù hợp cho E2E automation.
> * Các testcase mở rộng sẽ kiểm tra **nhiều CVE trên cùng component** và **nhiều component khác nhau**.

### E2E-SEC-001: SBOM → CVE → Insight (Happy Path)

**Mục tiêu:** Xác minh luồng chuẩn từ SBOM ingestion đến query API với **1 CVE duy nhất**.

#### Input

Agent gửi SBOM payload hợp lệ cho workload chứa component OpenSSL có CVE đã biết.

**SBOM mẫu:**

```json
{
  "components": [
    {
      "name": "openssl",
      "version": "1.1.1f",
      "type": "library"
    }
  ]
}
```

#### CVE mong đợi

* CVE ID: **CVE-2021-3711**
* Severity: HIGH

#### Steps

1. Agent POST SBOM → Core
2. Core trả HTTP 202, publish event `ksam.sbom.ingested`
3. Worker consume event, map CVE
4. Worker persist SBOM, CVE, Insight
5. Query API: SBOM / CVE / Insight

#### Kết quả mong đợi

* 1 SBOM record được lưu
* 1 CVE record với `cve_id = CVE-2021-3711`
* 1 Insight được tạo:

  * type: `VULNERABLE_WORKLOAD`
  * severity: HIGH

**Pass Criteria:** Dữ liệu nhất quán giữa DB và API.

---

### E2E-SEC-002: Duplicate SBOM Submission

**Mục tiêu:** Đảm bảo idempotency khi Agent gửi SBOM trùng lặp.

#### Input

Agent gửi **cùng một SBOM** (openssl 1.1.1f) hai lần liên tiếp.

#### Kết quả mong đợi

* SBOM không bị nhân bản
* CVE-2021-3711 chỉ tồn tại **1 record logic**
* Insight không bị duplicate

**Pass Criteria:** Không có record trùng theo workload + component + version.

---

### E2E-SEC-003: SBOM Không Có CVE

**Mục tiêu:** Xác minh hệ thống xử lý component không có CVE.

#### Input

SBOM chỉ chứa component không tồn tại CVE trong database.

#### Kết quả mong đợi

* SBOM được lưu
* Không có CVE record
* Insight được tạo với severity = LOW hoặc NONE

---

### E2E-SEC-004: Invalid SBOM Payload

**Mục tiêu:** Validate input handling và schema validation.

#### Input

SBOM thiếu field bắt buộc (ví dụ: `component.version`).

#### Kết quả mong đợi

* Core trả HTTP 400 hoặc 422
* Không publish event vào NATS
* Không có record nào được ghi vào DB

---

## 5. Testcase E2E Mở Rộng – Multi-CVE & Aggregation

### E2E-SEC-001-B: Multiple CVEs on Same Component

**Mục tiêu:** Xác minh hệ thống xử lý **nhiều CVE trên cùng một component**.

#### Input

SBOM với `openssl 1.1.1f` (có nhiều CVE).

#### CVE mong đợi (ví dụ)

* CVE-2021-3711 (HIGH)
* CVE-2021-3712 (MEDIUM)
* CVE-2020-1971 (HIGH)

#### Kết quả mong đợi

* Mỗi CVE = 1 record riêng trong `cve_findings`
* **Chỉ 1 Insight duy nhất** cho workload
* Insight severity = **HIGH** (max severity)
* Insight metadata chứa `total_cves >= 3`

---

### E2E-SEC-001-C: Multiple Components, Mixed Severity

**Mục tiêu:** Xác minh aggregation khi workload có **nhiều component khác nhau với mức độ nghiêm trọng khác nhau**.

#### Image test cụ thể (khuyến nghị)

* **Image:** `debian:10-slim`

Lý do chọn:

* Image phổ biến, dễ pull
* Chứa OpenSSL và zlib phiên bản cũ
* CVE ổn định, dễ tái hiện trên nhiều scanner

---

#### SBOM mong đợi (logical view)

```json
{
  "components": [
    {
      "name": "openssl",
      "version": "1.1.1d",
      "type": "library"
    },
    {
      "name": "zlib",
      "version": "1.2.11",
      "type": "library"
    }
  ]
}
```

---

#### CVE mong đợi

* **OpenSSL 1.1.1d**

  * CVE-2021-3711 (HIGH)
  * CVE-2020-1971 (HIGH)

* **zlib 1.2.11**

  * CVE-2018-25032 (LOW / MEDIUM tùy DB)

---

#### Kết quả mong đợi

* CVE records được tạo cho **cả hai component**
* Tổng số CVE ≥ 3
* **Chỉ 1 Insight duy nhất** cho workload
* Insight severity = **HIGH** (max severity)
* Insight metadata:

  * `total_components = 2`
  * `total_cves >= 3`
  * `highest_severity = HIGH`

**Pass Criteria:** Insight phản ánh đúng risk posture tổng thể của workload, không tạo insight trùng lặp.

---

## 6. Chaos Test Specification

### CHAOS-SEC-001: Worker Down During Ingestion

**Mục tiêu:** Đảm bảo async decoupling.

#### Setup

* Stop toàn bộ Policy/Insight Worker

#### Steps

1. Agent gửi SBOM
2. Core accept và publish event
3. Quan sát NATS backlog
4. Restart Worker

#### Kết quả mong đợi

* Core vẫn trả HTTP 202
* Event tồn tại trong stream
* Worker xử lý backlog sau khi up
* Không mất dữ liệu

---

### CHAOS-SEC-002: NATS JetStream Restart

**Mục tiêu:** Kiểm tra durability của stream.

#### Setup

* Restart NATS node (1 node trong cluster)

#### Kết quả mong đợi

* Stream không mất message
* Publish bị delay nhưng không crash Core
* Worker resume consume

---

### CHAOS-SEC-003: Database Unavailable

**Mục tiêu:** Kiểm tra isolation giữa ingestion và persistence.

#### Setup

* Stop Database

#### Steps

1. Agent gửi SBOM
2. Core publish event
3. Worker attempt persist → fail

#### Kết quả mong đợi

* Core vẫn accept request
* Event retry hoặc giữ trong backlog
* Khi DB up → dữ liệu được persist

---

### CHAOS-SEC-004: Event Flood / Burst Load

**Mục tiêu:** Đánh giá back-pressure & stability.

#### Input

* 10k SBOM events trong < 2 phút

#### Kết quả mong đợi

* Core latency < SLA
* Stream backlog tăng nhưng không overflow
* Worker drain backlog trong SLA
* Không crash service

---

## 6. SLA & Pass/Fail Matrix

| Nhóm        | Tiêu chí                   |
| ----------- | -------------------------- |
| Correctness | Data đúng, không thiếu     |
| Durability  | Không mất event            |
| Performance | Admission/Core không block |
| Resilience  | Hệ thống tự phục hồi       |

Fail bất kỳ tiêu chí nào → **Release Blocker**.

---

## 7. Ghi chú vận hành

* Chaos test **bắt buộc chạy trước production rollout**
* Kết quả test dùng làm input cho capacity planning
* Không đạt chaos test → không scale user

---

## 8. Kết luận

Bộ test spec này đảm bảo KSAM không chỉ **chạy đúng**, mà còn **chịu lỗi và chịu tải**.

Nói thẳng: nếu hệ thống pass toàn bộ spec này, bạn có quyền tự tin đưa vào môi trường enterprise thực tế.
