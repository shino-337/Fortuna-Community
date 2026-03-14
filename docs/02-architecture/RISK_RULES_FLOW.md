# Luồng xử lý Risk Rules (Settings → Add rule)

## Bảo mật: Validate và tạo rule chỉ ở server

- **Verify** và **tạo/sửa rule** được thực thi **hoàn toàn trên server**. Client (trình duyệt) chỉ gửi payload và hiển thị kết quả/lỗi từ server.
- Không dựa vào validation phía client để chặn dữ liệu sai (client có thể bị bỏ qua hoặc chỉnh sửa). Mọi kiểm tra chuẩn và quyết định lưu DB đều do server thực hiện để tránh lạm dụng và đảm bảo bảo mật.

## 1. Khi vào Settings → Add rule

- **UI:** Trang **Settings** → tab **Risk Rules** → nút **Add** (hoặc Edit/Delete trên từng rule).
- **Form Add:** Modal với các trường: Rule ID, Name, Severity, Category, Description, Enabled, Base score, (Conditions/aggregation/tags qua API).
- **Lưu:** Gọi `api.createRiskRule(formRule)` (create) hoặc `api.updateRiskRule(id, formRule)` (edit), `api.deleteRiskRule(id)` (delete).

## 2. API được gọi

| Hành động | HTTP | Endpoint | Handler |
|-----------|------|----------|---------|
| Danh sách | GET | `/api/v1/risk/rules` | `GetRiskRulesList` |
| Chi tiết | GET | `/api/v1/risk/rules/:id` | `GetRiskRuleByID` |
| Thêm | POST | `/api/v1/risk/rules` | `CreateRiskRule` |
| Sửa | PUT | `/api/v1/risk/rules/:id` | `UpdateRiskRule` |
| Xóa | DELETE | `/api/v1/risk/rules/:id` | `DeleteRiskRule` |
| **Kiểm tra (không lưu)** | POST | `/api/v1/risk/rules/validate` | `ValidateRiskRule` — body JSON hoặc YAML (Content-Type: application/x-yaml), trả về `{ valid, errors[] }` |
| **Import từ YAML** | POST | `/api/v1/risk/rules/import` | `ImportRiskRule` — body YAML (hoặc JSON), validate rồi tạo rule |
| **Export YAML** | GET | `/api/v1/risk/rules/export` hoặc `?id=rule_id` | `ExportRiskRulesYAML` — trả về một hoặc toàn bộ rule dạng YAML |

- **Validate:** Body JSON hoặc YAML (Content-Type: application/x-yaml). **Chỉ server** parse và chạy `riskengine.ValidateRule`. Không ghi DB. Dùng cho Verify form hoặc Verify YAML trước khi import.
- **Import:** Body YAML (hoặc JSON). **Chỉ server** parse, validate, rồi tạo rule (giống Create). Nên gọi Validate trước (Verify YAML) để xác nhận rule đúng trước khi import.
- **Export:** GET trả về YAML (một rule nếu `?id=rule_id`, không có id thì toàn bộ). Dùng để backup hoặc chỉnh sửa ngoài rồi import lại.
- **Create:** Client gửi JSON. **Chỉ server** validate rồi ghi DB. Request không hợp lệ bị từ chối 400 kèm `errors`.
- **Update:** Bind body (id có thể string hoặc number), tìm bản ghi theo `rule_id`, cập nhật và `db.Save(m)`.
- **Delete:** Soft-delete theo `rule_id`: `db.Delete(&models.RiskRule{}, "rule_id = ?", id)`.

## 3. Rule lưu ở đâu

- **Risk rules (Settings):** Lưu trong **PostgreSQL**, bảng **`risk_rules`** (namespace DB của ứng dụng, ví dụ `fortuna`).
- **Model:** `core/pkg/models/risk_rule.go` — các cột: `rule_id`, `name`, `category`, `severity`, `description`, `enabled`, `conditions` (JSON), `aggregation`, `base_score`, `tags` (JSON), `created_at`, `updated_at`, `deleted_at`.
- **Nguồn khác:** Nếu không dùng DB (bảng chưa có hoặc trống), API list có thể fallback đọc từ thư mục YAML (`FORTUNA_RULES_DIR`) — khi đó là read-only, không add/edit từ Settings.

## 4. Khi nào rule có hiệu lực (effect)

- **Risk engine** (dùng để đánh giá resource và tạo insight) load rule **khi khởi tạo**:
  - **Ưu tiên:** Nếu có bảng `risk_rules` và có bản ghi → load từ DB.
  - Nếu không: load từ YAML (`FORTUNA_RULES_DIR`) + hardcoded.
- **Nơi dùng engine:** `RiskWorker` (xử lý message normalized), `HistoricalRiskEvaluator` (đánh giá resource trong DB). Cả hai tạo engine **một lần** lúc start.
- **Sau khi thêm/sửa/xóa rule trong Settings:** Backend gọi **reload risk engine từ DB**. Engine đã đăng ký toàn cục sẽ được reload ngay; lần đánh giá risk tiếp theo (message mới hoặc job historical) sẽ dùng bộ rule mới → **rule có hiệu lực ngay** mà không cần restart core.
- Nếu không có engine nào đăng ký (ví dụ worker chưa chạy), rule vẫn được lưu DB và sẽ có hiệu lực khi worker/engine được tạo lần sau.

## 5. Tóm tắt

| Bước | Mô tả |
|------|--------|
| 1 | User vào Settings → Risk Rules → Add (hoặc Edit/Delete). |
| 2 | Dashboard gọi POST/PUT/DELETE `/api/v1/risk/rules` (hoặc GET để list/detail). |
| 3 | Core lưu vào bảng **`risk_rules`** (PostgreSQL). |
| 4 | Sau khi ghi DB thành công, core gọi **reload risk engine từ DB** (engine đã đăng ký trong worker). |
| 5 | Rule **có hiệu lực ngay** cho các lần đánh giá risk tiếp theo (stream + historical). |

## 6. Lưu trữ lâu dài (sau deploy/rebuild)

- **DB** là nguồn chạy runtime. Để rule không mất khi rebuild/deploy (hoặc khi DB không dùng volume persistent), rule được **đồng bộ ra thư mục**:
  - **Thư mục:** `FORTUNA_RULES_DIR/risk/` (subfolder `risk` để không trùng với policy YAML).
  - **Khi tạo/sửa rule:** Sau khi ghi DB, core ghi thêm file `FORTUNA_RULES_DIR/risk/<rule_id>.yaml` (nội dung rule dạng YAML).
  - **Khi xóa rule:** Sau khi xóa trong DB, core xóa file `FORTUNA_RULES_DIR/risk/<rule_id>.yaml`.
- **Khi khởi động:** Nếu bảng `risk_rules` **trống** và có thư mục `FORTUNA_RULES_DIR/risk` với file `.yaml`, core sẽ **seed** (import) các rule từ thư mục vào DB. Nhờ vậy sau deploy mới hoặc DB mới, rule vẫn được khôi phục từ folder (volume/ConfigMap/git-sync).
- **Cách dùng:** Set env `FORTUNA_RULES_DIR` trỏ tới thư mục rules (vd. `/app/rules`). Mount volume hoặc sync thư mục này để lưu trữ lâu dài.

## 7. Khác với Policy Rules (YAML)

- **Policy rules** (trang Rules / Policy): Lưu **file YAML** trong `FORTUNA_RULES_DIR`, API `/policy/rules` (và reload qua `POST /policy/rules/reload`). Dùng cho policy engine (CEL, match resource), **không** phải bảng `risk_rules`.
- **Risk rules** (Settings → Risk Rules): Lưu **DB** `risk_rules`, API `/risk/rules`, dùng cho **risk engine** (insight, severity, conditions). Reload từ DB được gọi tự động sau create/update/delete. Đồng thời được export vào `FORTUNA_RULES_DIR/risk/` để tồn tại sau deploy.

---

**Đánh giá chi tiết luồng Dashboard ↔ Core, đồng bộ và phương thức:** xem [RISK_RULES_ASSESSMENT.md](./RISK_RULES_ASSESSMENT.md).

**Xử lý sự cố (thêm rule nhưng bảng không đổi, hai loại rule, kiểm tra API/DB):** xem [RISK_RULES_TROUBLESHOOTING.md](./RISK_RULES_TROUBLESHOOTING.md).

**Risk Rules vs Policy Rules — khác nhau và mục đích:** xem [RISK_VS_POLICY_RULES.md](./RISK_VS_POLICY_RULES.md).
