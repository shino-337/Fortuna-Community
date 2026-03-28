# Risk Center – E2E Test Cases và Cách chạy

Script E2E: **`scripts/e2e/e2e-risk-center-full.sh`**  
Báo cáo chi tiết: **`test-results/risk-center-e2e-YYYYMMDD-HHMMSS.md`**

---

## Chạy E2E

```bash
# Từ repo root (cần Core pod đang chạy trong cluster)
./scripts/e2e/e2e-risk-center-full.sh

# Đổi namespace
NAMESPACE=my-ns ./scripts/e2e/e2e-risk-center-full.sh
```

Yêu cầu: `kubectl` trỏ tới cluster có namespace fortuna (hoặc NAMESPACE), Core pod Running. Script dùng JWT (admin/admin123) để gọi API.

---

## Danh sách Test Case (20)

| ID | Test case | Mô tả | Phase |
|----|-----------|--------|-------|
| **TC-01** | GET /risks (list, pagination) | Trả về danh sách risks với page, pageSize; JSON có `insights[]`, `total`. | Base |
| **TC-02** | GET /risks?withScores=1 | Unified score (Phase 3.1): response có thể có `totalScore`, `priorityLevel` từ `risk_scores`. | 3.1 |
| **TC-03** | GET /risks?priorityLevel=P0 | Lọc theo priority level (P0–P4). | 3.1 |
| **TC-04** | GET /insights/summary | Severity bar: total, critical, high, medium, low. | Base |
| **TC-05** | GET /insights/summary/by-cluster | Global view: mảng byCluster (clusterId, total, critical, …). | 2.2 |
| **TC-05b** | GET /insights/summary/global | Global summary (tổng hợp toàn cục); HTTP 200, total/critical/high/…. | #8 |
| **TC-05c** | GET /risk/histogram | Risk Score Distribution: HTTP 200, `bins[]`, `totalFindings` (Phase 3 histogram). | Phase 3 |
| **TC-06** | GET /risks/export | Export CSV (Phase 1.3); HTTP 200, body dạng CSV. | 1.3 |
| **TC-07** | GET /risks/export?format=pdf | Export HTML để in PDF (Phase 3.3); HTTP 200, body HTML. | 3.3 |
| **TC-08** | GET /risk-rules | Danh sách risk rules (DB hoặc files); response có `rules[]`, `total`, `source` (db\|files). | 4 |
| **TC-09** | GET /risk-rules/:id | Chi tiết một rule; SKIP nếu chưa có rule nào. | 4 |
| **TC-10** | POST /risk-rules | Tạo rule mới (chỉ khi source=db). | 4 CRUD |
| **TC-11** | PUT /risk-rules/:id | Cập nhật rule (chỉ khi source=db). | 4 CRUD |
| **TC-12** | DELETE /risk-rules/:id | Xóa rule (chỉ khi source=db). | 4 CRUD |
| **TC-13** | GET /pod-capabilities/trends | PCE trend 7 ngày (Phase 4). | 4 PCE |
| **TC-14** | GET /pod-capabilities/summary/namespace | PCE heatmap theo namespace. | 4 PCE |
| **TC-15** | GET /runtime-signals | Tab Reference / runtime signals. | Base |
| **TC-16** | WebSocket GET /ws/risks | Endpoint /ws/risks phản hồi (101 upgrade hoặc 400/401). | 2.3 |
| **TC-17** | GET /risk/pods/:uid/report | Pod risk report: HTTP 200, `summary` hợp lệ — **bản mới:** `runtimeSignals24h` / `podDirectInsightCount`; **Core cũ:** chỉ `riskLevel` / `clusterAdminBindings`. SKIP nếu namespace không có pod. | Runtime |
| **TC-18** | GET /runtime/pods/:uid/signals | Tín hiệu runtime theo pod (24h): HTTP 200, JSON có `signals[]`. SKIP nếu không có pod uid. | Runtime |

---

## Kết quả mẫu (chạy thực tế)

Sau khi chạy, báo cáo Markdown trong `test-results/` có dạng:

- **Header:** Thời gian, namespace, Core pod.
- **Từng TC:** Tên, Kết quả (PASS/FAIL/SKIP), Mô tả, Thực tế (HTTP code, giá trị), Chi tiết (nếu có).
- **Tổng kết:** Bảng PASS / FAIL / SKIP.

Ví dụ tổng kết:

| Kết quả | Số lượng |
|---------|----------|
| PASS    | 15 |
| FAIL    | 0 |
| SKIP    | 1 |
| **Tổng** | **20** |

TC-09 thường SKIP khi chưa có rule (total=0). TC-10/11/12 SKIP khi `source=files` (rules load từ YAML, không CRUD qua API). **TC-10:** Script xóa rule `e2e-risk-center-rule` (nếu có) trước khi POST để tránh lỗi duplicate `rule_id` từ lần chạy trước; nếu POST vẫn FAIL, báo cáo sẽ in ~300 ký tự response để debug.

---

## Liên kết

- **Go-Live Criteria:** [Risk-Center-Go-Live-Criteria.md](./Risk-Center-Go-Live-Criteria.md)  
- **Improvement plan:** [Risk-Center-Improvement-Plan.md](./Risk-Center-Improvement-Plan.md)  
- **Đồng bộ Core/Dashboard/DB:** [Risk-Center-Component-Implement.md](./Risk-Center-Component-Implement.md#kiểm-tra-đồng-bộ-core--agent--dashboard--db-cập-nhật-mới)
