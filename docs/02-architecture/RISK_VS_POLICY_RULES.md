# Risk Rules vs Policy Rules — Khác nhau và mục đích

Hệ thống có **hai danh sách rule** dùng chung một kiểu dữ liệu (rule detection) nhưng **khác nguồn lưu trữ, API và mục đích**.

---

## 1. So sánh nhanh

| | **Risk Rules** | **Policy Rules** (danh mục rule) |
|---|----------------|-----------------------------------|
| **API** | `GET/POST/PUT/DELETE /api/v1/risk/rules` | `GET /api/v1/policy/rules` (và CRUD/reload theo file) |
| **Trang UI** | **Settings → Risk Rules** | **Rules** (menu Rules) |
| **Lưu trữ** | **DB** (bảng `risk_rules`), kèm export vào `FORTUNA_RULES_DIR/risk/` | **File YAML** trong `FORTUNA_RULES_DIR` (thư mục gốc) |
| **Mục đích** | Rule **thực sự dùng để đánh giá risk** và tạo **Insights** (phát hiện rủi ro) | **Danh mục / catalog** rule: xem, test từng rule, xem metrics (số lần match) |
| **Engine dùng** | **Risk engine** (RiskWorker): load từ DB ưu tiên, rồi YAML → `EvaluateResource()` → tạo **insights** | **RulesManager / YAMLEngine**: đọc YAML để trả danh sách cho Rules page, test rule, metrics |
| **Thêm/sửa từ UI** | Có: Settings → Risk Rules (Add, Edit, Import YAML) | Có: qua API ghi file YAML (Rules page có thể dùng API create/update) |
| **Reload** | Tự reload sau mỗi Create/Update/Delete (engine load lại từ DB) | Nút **Reload Rules Engine** trên trang Rules (reload từ file YAML) |

---

## 2. Risk Rules — Mục đích

- **Dùng để làm gì:** Là bộ rule **vận hành** (operational) cho **đánh giá rủi ro**.
- **Luồng:**
  1. Resource (Pod, RoleBinding, ServiceAccount, …) được chuẩn hóa và đẩy vào pipeline (ví dụ qua NATS).
  2. **RiskWorker** nhận message, gọi **risk engine** với danh sách rule (load từ bảng `risk_rules` hoặc từ YAML nếu DB trống).
  3. Engine chạy **EvaluateResource** → so khớp từng rule (conditions, severity, category) → nếu match thì tạo **Insight** (phát hiện rủi ro).
  4. Insights được lưu DB và hiển thị ở **Risk / Insights** trên dashboard (finding, severity, recommendation).
- **Kết quả:** Người dùng thấy **các phát hiện rủi ro** (insights) trên cluster; rule thêm trong **Settings → Risk Rules** (DB) sẽ được engine dùng ngay cho lần đánh giá tiếp theo.

**Tóm lại:** Risk rules = rule **chạy thật** để tạo **insights** (risk findings).

---

## 3. Policy Rules (danh mục trên Rules page) — Mục đích

- **Dùng để làm gì:** Là **catalog / danh mục** rule: xem danh sách, **test** một rule với payload mẫu, xem **số lần match** (metrics).
- **Nguồn:** File YAML trong `FORTUNA_RULES_DIR` (cùng có thể dùng built-in). API `GET /policy/rules` trả về từ **RulesManager** (YAMLEngine) — không đọc bảng `risk_rules`.
- **Trang Rules:** Hiển thị danh sách này, có nút **Reload Rules Engine** (reload từ YAML), **Test rule**, **Metrics** (match count). Phù hợp để soạn/thử rule trước khi đưa vào vận hành.
- **Không** trực tiếp điều khiển risk engine: Khi risk engine dùng **DB** (bảng `risk_rules`), nó **không** lấy danh sách từ API policy/rules hay từ thư mục gốc của policy rules; nó lấy từ `risk_rules` (và chỉ fallback YAML khi DB trống). Do đó rule chỉ có trên **Rules page** (YAML) mà không nằm trong **risk_rules** thì **sẽ không** tạo insights.

**Tóm lại:** Policy rules (API `/policy/rules`) = **catalog rule từ YAML** để xem, test, metrics; không phải nguồn rule vận hành khi đã dùng DB.

---

## 4. Policy Templates / Instances (policy enforcement) — Khác với cả hai

Ngoài hai loại trên, còn có **Policy templates & instances** (API `/policy/templates`, `/policy/instances`):

- **Mục đích:** Chính sách **enforcement** (cho phép/chặn, CEL), tạo **violations** khi resource không đạt điều kiện.
- **Engine:** `policy.Evaluator` (CEL), dùng template + instance từ DB.
- **Khác với Risk Rules và Policy Rules:** Đây là **policy compliance / admission**, không phải “detection rule” hay “catalog rule” như hai mục trên.

---

## 5. Khi nào dùng Risk Rules, khi nào dùng Policy Rules (catalog)

- **Muốn thêm rule và rule đó thực sự tạo ra phát hiện rủi ro (insights):**  
  Dùng **Risk Rules** — thêm/sửa trong **Settings → Risk Rules** (lưu DB). Rule này sẽ được risk engine dùng để đánh giá và tạo insights.

- **Muốn có một bộ rule “mẫu” trong file YAML, xem danh sách, test từng rule, xem metrics:**  
  Dùng **Policy Rules** — quản lý file YAML trong `FORTUNA_RULES_DIR`, xem trên trang **Rules**, dùng Reload / Test / Metrics. Để rule này **cũng** tạo insights thì cần đưa vào DB (ví dụ qua Import YAML trong Settings → Risk Rules hoặc seed từ `FORTUNA_RULES_DIR/risk/`).

---

## 6. Tóm tắt một dòng

- **Risk rules** = rule **vận hành** (DB): engine dùng để **đánh giá resource** và tạo **insights** (risk findings). UI: **Settings → Risk Rules**.
- **Policy rules** (API `/policy/rules`) = **catalog** rule từ **YAML**: xem, test, metrics trên trang **Rules**; không phải nguồn rule chạy khi engine đã lấy rule từ DB.
