# Capability Spec Implementation Summary

**Nguồn**: `docs/03-components/podCapabilityEngine/Capability_Specification–MITRE_ATT&C.md`  
**Ngày**: 2026-02-03

---

## 1. Đã thực hiện

### 1.1 Phân tích spec và bổ sung logic

- **Spec PART A** định nghĩa schema metadata: `name`, `summary`, `full_description`, `mitre` (tactic, technique, subtechnique), `kill_chain_stage`, `technical_indicators`, `impact`, `recommended_mitigations`, `false_positive_considerations`, `references`.
- **Logic hiện tại**: Bảng `capability_metadata` chỉ có `description`, `severity_base`, `confidence_base`, `preconditions`, `produces_attack_steps`. Không có trường MITRE, impact, mitigations, references.
- **Bổ sung**: Thêm cột và seed theo spec cho các capability_id hiện có (ESC_PRIV_POD, ESC_HOSTPATH_NODE, ESC_HOSTPID_POD, ESC_HOSTIPC_POD, ID_TOKEN_POD, API_RBAC_WRITE_CLUSTER, NET_HOSTNETWORK, CTRL_CONTROL_PLANE_POD, ESC_RUNTIME_PROBE, ESC_RUNTIME_ACTIVE, ESC_RUNTIME_PROC_ROOT).

### 1.2 Thay đổi code

| Thành phần | Thay đổi |
|------------|----------|
| **Migration 060** | `060_add_capability_metadata_extended_columns.go`: ADD COLUMN name, summary, full_description, mitre_tactic, mitre_technique, mitre_subtechnique, kill_chain_stage, technical_indicators (JSONB), impact (JSONB), recommended_mitigations (JSONB), false_positive_considerations (JSONB), refs (JSONB). |
| **Migration 061** | `061_seed_capability_metadata_extended.go`: UPDATE capability_metadata SET ... từ nội dung spec (name, summary, full_description, MITRE, kill_chain_stage, technical_indicators, impact, recommended_mitigations, false_positive_considerations, references) cho từng capability_id. |
| **Model** | `core/pkg/models/capability_metadata.go`: Thêm các field tương ứng (Name, Summary, FullDescription, MitreTactic, MitreTechnique, MitreSubtechnique, KillChainStage, TechnicalIndicators, Impact, RecommendedMitigations, FalsePositiveConsiderations, References). |
| **migrations.go** | Đăng ký Migration060 và Migration061. |
| **Dashboard types** | `dashboard/types.ts`: CapabilityMetadata thêm name?, summary?, fullDescription?, mitreTactic?, mitreTechnique?, mitreSubtechnique?, killChainStage?, technicalIndicators?, impact?, recommendedMitigations?, falsePositiveConsiderations?, references?. |
| **UI** | `dashboard/components/CapabilityMetadataBrowser.tsx`: Hiển thị name (hoặc capabilityId), summary (hoặc description), MITRE (tactic, technique, subtechnique), kill_chain_stage; search theo name/summary/MITRE/kill chain; expand/collapse chi tiết: full_description, technical_indicators, impact, recommended_mitigations, false_positive_considerations, references (link). Giữ preconditions và produces_attack_steps. |

### 1.3 Luồng xử lý (không đổi)

- **PCE Evaluator** → `InitializeCapability` vẫn dùng `capability_metadata.confidence_base`, `severity_base`; không đọc các cột mới.
- **CSC** → `PromoteCapability` / `InitializeCapability` không đổi.
- **API** `GET /api/v1/capability-metadata` và `GET /api/v1/capability-metadata/:id` trả về toàn bộ model GORM → frontend nhận thêm name, summary, fullDescription, mitreTactic, mitreTechnique, mitreSubtechnique, killChainStage, technicalIndicators, impact, recommendedMitigations, falsePositiveConsiderations, references.
- **Dashboard** Capabilities page → CapabilityMetadataBrowser gọi `api.getCapabilityMetadata()` và hiển thị theo các trường mới.

---

## 2. Kiểm tra luồng và rebuild/redeploy

### 2.1 Chạy migration (khi Core khởi động)

- Core chạy `RunMigrations()` → Migration060 thêm cột → Migration061 cập nhật dữ liệu.
- Kiểm tra: Sau khi deploy Core mới, vào Postgres và chạy:
  - `SELECT capability_id, name, summary, mitre_technique, kill_chain_stage FROM capability_metadata LIMIT 5;`
  - Phải thấy name, summary, mitre_technique, kill_chain_stage đã có giá trị.

### 2.2 Rebuild và deploy

```bash
# Rebuild toàn bộ (core, agent, dashboard) và load vào containerd
cd /home/k8s/KSAM
bash scripts/build-and-load-containerd.sh

# Restart Core để chạy migration 060, 061
kubectl rollout restart deployment/fortuna-core -n fortuna

# Restart Dashboard để dùng UI mới
kubectl rollout restart deployment/fortuna-dashboard -n fortuna

# (Tùy chọn) Restart Agent nếu đã build lại
kubectl rollout restart daemonset/fortuna-agent -n fortuna
```

Hoặc clean + rebuild + deploy đầy đủ:

```bash
cd /home/k8s/KSAM
# Không xóa DB nếu chỉ cần chạy migration mới
bash scripts/full-clean-database-rebuild-deploy.sh
# Nếu muốn giữ data: dùng --skip-clean hoặc chỉ rebuild + restart
```

### 2.3 Kiểm tra API và UI

1. **API**:  
   `GET /api/v1/capability-metadata` (với JWT) → response mỗi phần tử phải có `name`, `summary`, `fullDescription`, `mitreTactic`, `mitreTechnique`, `mitreSubtechnique`, `killChainStage`, `technicalIndicators`, `impact`, `recommendedMitigations`, `falsePositiveConsiderations`, `references` (nếu đã seed).

2. **UI**:  
   Vào **Capabilities** (Capability Catalog):  
   - Có tên capability (name), summary, badge MITRE, kill chain stage.  
   - Bấm expand (chevron) → hiển thị full description, technical indicators, impact, recommended mitigations, false positive considerations, references (link).

---

## 3. File đã tạo/sửa

| File | Hành động |
|------|------------|
| `core/migrations/060_add_capability_metadata_extended_columns.go` | Tạo mới |
| `core/migrations/061_seed_capability_metadata_extended.go` | Tạo mới |
| `core/migrations/migrations.go` | Thêm 060, 061 |
| `core/pkg/models/capability_metadata.go` | Thêm field extended |
| `dashboard/types.ts` | Mở rộng CapabilityMetadata |
| `dashboard/components/CapabilityMetadataBrowser.tsx` | Cập nhật UI theo spec |

---

## 4. Mapping Spec → capability_id (seed 061)

| Spec ID | capability_id (DB) |
|---------|---------------------|
| CAP-ESC-PRIV-POD | ESC_PRIV_POD |
| CAP-ESC-HOSTPATH-RW | ESC_HOSTPATH_NODE |
| CAP-ESC-HOSTPID | ESC_HOSTPID_POD |
| CAP-ESC-HOSTIPC | ESC_HOSTIPC_POD |
| CAP-ID-SA-TOKEN-ACCESS | ID_TOKEN_POD |
| CAP-RBAC-CLUSTER-WRITE | API_RBAC_WRITE_CLUSTER |
| CAP-NET-HOSTNETWORK | NET_HOSTNETWORK |
| (control plane) | CTRL_CONTROL_PLANE_POD |
| (runtime probe) | ESC_RUNTIME_PROBE |
| (runtime active) | ESC_RUNTIME_ACTIVE |
| (proc root) | ESC_RUNTIME_PROC_ROOT |

Capability chưa có trong code (CAP-DISC-ENV-ENUM, CAP-DISC-K8S-API-PROBE, CAP-LM-SA-TOKEN-REUSE) có thể bổ sung metadata sau khi có capability_id tương ứng trong evaluator và seed 050.
