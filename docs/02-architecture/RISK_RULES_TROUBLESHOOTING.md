# Risk Rules — Xử lý sự cố

## Hai bảng rule khác nhau (quan trọng)

| Trang | API cần gọi | Nguồn dữ liệu |
|-------|----------------|----------------|
| **Rules** (menu Rules) | **GET `/api/v1/policy/rules`** | Policy rules (YAML, FORTUNA_RULES_DIR). 7 rule mặc định ở đây. |
| **Settings → Risk Rules** | **GET `/api/v1/risk/rules`** | Risk rules: bảng DB **`risk_rules`** (hoặc fallback files). Rule bạn thêm nằm ở đây. |

- Rule thêm ở **Settings → Risk Rules** được lưu vào bảng **`risk_rules`** và **chỉ** trả về bởi **GET `/api/v1/risk/rules`**. Nó **không** xuất hiện trong **GET `/api/v1/policy/rules`**.
- Nếu bạn gọi **GET /api/v1/policy/rules** và thấy 7 rule — đó là policy rules (đúng). Để xem rule vừa thêm, phải gọi **GET /api/v1/risk/rules**.

**Ví dụ:** Bạn thêm rule tại Settings → Risk Rules nhưng kiểm tra bằng `GET /api/v1/policy/rules` → API đó chỉ trả policy rules (7 rule), không trả risk rules. Hãy gọi **GET /api/v1/risk/rules** để thấy rule mới.

**So sánh chi tiết mục đích Risk Rules vs Policy Rules:** xem [RISK_VS_POLICY_RULES.md](./RISK_VS_POLICY_RULES.md).

---

## Kiểm tra danh sách risk rules (API)

Chạy script (cần `curl`, `jq`):

```bash
./scripts/verify/verify-risk-rules-api.sh http://localhost:8080
```

Hoặc thủ công:

```bash
# Danh sách risk rules (giống Settings → Risk Rules)
curl -s "http://localhost:8080/api/v1/risk/rules" | jq '{ source, total, rule_ids: [.rules[].id] }'
```

- **source: "db"** — danh sách lấy từ bảng `risk_rules`. Số rule = số dòng trong DB (sau khi Add thành công sẽ tăng).
- **source: "files"** — bảng không tồn tại hoặc không dùng; danh sách đọc từ YAML (read-only). Khi đó rule thêm trong Settings sẽ **không** xuất hiện ở đây (vì Create ghi DB nhưng list đang đọc từ files).

---

## Khi thêm rule nhưng bảng vẫn không đổi

### 1. Đang xem đúng trang chưa?

- Vào **Settings** → tab **Risk Rules**.
- Xem dòng **Source:** — nếu là **db** và có **(N rules)** thì N là số rule đang hiển thị từ DB.

### 2. Save có báo lỗi không?

- Nếu Save trả lỗi (validation, duplicate `rule_id`, v.v.) rule sẽ không được tạo. Kiểm tra thông báo đỏ trong modal.
- **Rule ID** không được trùng với rule đã có (trong DB). Nếu trùng, server trả 500/409 và rule mới không được thêm.

### 3. Sau khi Save thành công

- Modal đóng và `loadRiskRules()` được gọi; danh sách sẽ refresh. Nếu vẫn không thấy rule mới:
  - Hard refresh trình duyệt (Ctrl+F5) hoặc mở lại tab Risk Rules.
  - Gọi trực tiếp API: `curl -s "BASE_URL/api/v1/risk/rules" | jq .total` — nếu total tăng thì DB đã có rule, vấn đề có thể do cache/UI.

### 4. Source = "files" và không thấy rule mới

- Nghĩa là Core đang trả danh sách từ **file YAML** (fallback), không từ DB.
- Nguyên nhân thường gặp: bảng `risk_rules` **chưa có** (migration 075 chưa chạy) hoặc Core dùng DB khác so với lúc tạo rule.
- Cần:
  - Kiểm tra migration: bảng `risk_rules` đã được tạo chưa (trong DB mà Core đang kết nối).
  - Đảm bảo Core dùng đúng DB (biến môi trường / config kết nối DB).

### 5. Kiểm tra trong database

Nếu có quyền truy cập DB (PostgreSQL):

```sql
SELECT rule_id, name, enabled, created_at FROM risk_rules WHERE deleted_at IS NULL ORDER BY rule_id;
```

- Số dòng = số rule mà API sẽ trả khi **source: "db"**.
- Nếu bạn vừa Add 1 rule mà query vẫn chỉ có 7 dòng thì Create có thể thất bại (xem log Core hoặc lỗi trả về từ API khi Save).

---

## Tóm tắt

1. **Rules** (menu) = policy rules (YAML). **Reload Rules Engine** chỉ áp dụng cho danh sách này.
2. **Settings → Risk Rules** = risk rules (DB). Thêm rule ở đây thì chỉ thấy ở đây; không cần reload, chỉ cần Save thành công và xem đúng tab.
3. Nếu Settings → Risk Rules hiển thị **Source: db** và **(7 rules)** thì hiện có 7 bản ghi trong `risk_rules`. Thêm thành công sẽ thành 8 và số này cập nhật sau khi refresh.
4. Nếu **Source: files** thì list đang đọc từ YAML; cần kiểm tra migration và kết nối DB để list chuyển sang **db** và hiển thị rule đã thêm.
