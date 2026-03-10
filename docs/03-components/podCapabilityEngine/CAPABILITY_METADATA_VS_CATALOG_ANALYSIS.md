# Phân tích: Capability Metadata vs Capability Catalog

**Mục đích:** Làm rõ sự trùng lặp giữa "Capability Metadata" và "Capability Catalog" trong UI/UX, thống nhất thuật ngữ và tối ưu một nguồn hiển thị.

---

## 1. Hiện trạng

### 1.1 Backend (một nguồn dữ liệu)

- **Bảng:** `capability_metadata` (migrations 047, 050, 060, 061).
- **API:** `GET /api/v1/capability-metadata`, `GET /api/v1/capability-metadata/:capabilityId`.
- **Nội dung:** Định nghĩa capability (domain, category, description, severity_base, confidence_base, preconditions, produces_attack_steps, …) cộng các cột mở rộng từ spec (name, summary, full_description, mitre_*, kill_chain_stage, impact, recommended_mitigations, refs, …).

→ Chỉ có **một** khái niệm dữ liệu: **capability metadata** (dùng cho PCE và cho tham chiếu).

### 1.2 UI – hai chỗ hiển thị cùng nội dung

| Vị trí | Nhãn hiển thị | Component | Ghi chú |
|--------|----------------|-----------|--------|
| **Trang Capabilities** (`/capabilities`) | "Capability Catalog" | `CapabilityMetadataBrowser` | Trang riêng, full browser (search, filter domain, danh sách đầy đủ). |
| **Risk Center → tab Reference** (`/risks`, tab Reference) | "Capability Metadata" | `CapabilityMetadataBrowser` | Cùng component, full browser nhúng trong tab. |

Hệ quả:

- Hai tên cho cùng một dữ liệu: "Catalog" vs "Metadata" gây lẫn lộn.
- Cùng một bảng/catalog được mở full ở hai nơi → trùng lặp UX, không rõ "nơi chính thức" để xem catalog.

---

## 2. Phân biệt thuật ngữ (đề xuất)

| Thuật ngữ | Ý nghĩa | Dùng ở đâu |
|-----------|--------|------------|
| **Capability Metadata** | Dữ liệu kỹ thuật (bảng, API, model). Mô tả capability: severity_base, confidence_base, preconditions, produces_attack_steps, và các trường mở rộng từ spec. | Backend, code, tài liệu kỹ thuật. |
| **Capability Catalog** | Danh mục tham chiếu dùng trong UI: danh sách định nghĩa capability để người dùng duyệt (definitions, severity, preconditions, MITRE, attack steps). | UI: tên trang, tên section, breadcrumb, link. |

→ **Trong UI chỉ dùng một tên:** **Capability Catalog**. Không dùng "Capability Metadata" làm tiêu đề màn hình hay section.

---

## 3. Điều chỉnh UI/UX đã thực hiện

### 3.1 Một nơi “full” catalog

- **Trang Capabilities** (`/capabilities`) là **nơi duy nhất** có full browser (search, filter domain, danh sách đầy đủ, expand chi tiết).
- Tiêu đề trang: **"Capability Catalog"**.
- Mô tả ngắn: "Definitions, severity, preconditions, MITRE mapping, and attack steps." (bỏ "capability metadata" trong copy user-facing).

### 3.2 Tab Reference trên Risk Center

- **Không** nhúng lại full `CapabilityMetadataBrowser` trong tab Reference.
- Thay bằng **một block ngắn**:
  - Tiêu đề: **"Capability Catalog"** (thống nhất với trang Capabilities).
  - Mô tả: Catalog là danh mục tham chiếu định nghĩa capability (severity, preconditions, attack steps, MITRE).
  - CTA: link **"Open Capability Catalog →"** dẫn đến `/capabilities`.
- Tab Reference giữ **Runtime Signals** là nội dung chính; Capability Catalog chỉ là lối vào nhanh tới trang catalog, tránh trùng nội dung.

### 3.3 Lợi ích

- Một tên trong UI: **Capability Catalog**.
- Một nơi xem đầy đủ: `/capabilities`; Risk Center chỉ dẫn tới đó.
- Giảm trùng lặp, rõ ràng hơn cho người dùng và dễ bảo trì (chỉ một component full browser).

---

## 4. Tóm tắt

- **Backend:** Giữ nguyên `capability_metadata` (bảng + API); đây là nguồn dữ liệu duy nhất.
- **UI:** Dùng thống nhất **"Capability Catalog"** cho trang và section; bỏ tiêu đề "Capability Metadata" trên màn hình.
- **Risk Center → Reference:** Chỉ còn summary + link sang Capability Catalog; không nhúng lại full browser.

File liên quan:

- `dashboard/pages/Capabilities.tsx` – Trang Capability Catalog (full browser).
- `dashboard/pages/Insights.tsx` – Tab Reference: block Capability Catalog (summary + link), không dùng full browser.
- `dashboard/components/CapabilityMetadataBrowser.tsx` – Chỉ dùng trong trang Capabilities.
