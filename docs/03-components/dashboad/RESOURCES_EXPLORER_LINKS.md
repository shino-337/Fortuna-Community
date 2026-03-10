# Resources Explorer – Liên kết dữ liệu và tối ưu link

**Mục đích:** Đảm bảo link giữa Resources Explorer và các trang chi tiết (Pod, Identity, Cluster, Node) nhất quán, đủ và hiển thị đúng thông tin khi click.

---

## 1. Luồng từ Resources Explorer

| Tab | Hành vi click | Đích |
|-----|----------------|-----|
| **Pod** | Click row hoặc "View" | `/resources/pods/${pod.id}` → PodDetail (backend GET /pods/:id) |
| **Service Account** | Click row hoặc "View identity" | `/identities/uid/${resource.id}` → IdentityDetail (resource.id = UID từ API) |
| **Role / RoleBinding** | Click row hoặc "View cluster" | `/clusters/${resource.clusterId}` → ClusterDetail (khi có clusterId) |

- **URL đồng bộ:** Tab và namespace được ghi vào URL (`?tab=Pod`, `?namespace=default`) để share/deep-link đúng.
- **API:** `getResources(type, { cluster, namespace })` trả về `clusterId`; type `K8sResource` có `clusterId?: string`.

---

## 2. Luồng từ Pod Detail

| Nội dung | Link | Đích |
|----------|------|------|
| Node (trong Overview) | Click tên node | `/clusters/${pod.clusterId}/nodes/${nodeName}` → NodeDetail |
| Related: View cluster | Button | `/clusters/${pod.clusterId}` → ClusterDetail |
| Related: View node | Button | `/clusters/${pod.clusterId}/nodes/${nodeName}` → NodeDetail |
| Related: Back to Resources | Button | `/resources?tab=Pod` |

---

## 3. Luồng từ Cluster Detail

| Nội dung | Link | Đích |
|----------|------|------|
| Inventory – Nodes | Click tên node | `/clusters/${id}/nodes/${nodeName}` → NodeDetail |
| Inventory – Namespaces | Click tên namespace | `/resources?tab=Pod&namespace=${ns}` → Resources (Pod tab, lọc namespace) |

---

## 4. Luồng từ Risk Detail (Affected Assets)

| Kind | Button | Đích |
|------|--------|------|
| Pod | "View pod" | `/resources/pods/uid/${r.id}` → PodDetail (r.id = pod UID) |
| ServiceAccount | "View identity" | `/identities/uid/${r.id}` → IdentityDetail |

---

## 5. Luồng từ Node Detail

| Nội dung | Link | Đích |
|----------|------|------|
| Workloads – Pod row / View | Click row hoặc "View" | `/resources/pods/${pod.id}` → PodDetail |
| Back | Button | `/clusters/${clusterId}` → ClusterDetail |

---

## 6. Thay đổi kỹ thuật đã làm

- **types.ts:** `K8sResource` thêm `clusterId?: string`.
- **api.ts:** `getResources(type, params?)` nhận `params?: { cluster?, namespace? }`, map `clusterId` từ response.
- **Resources.tsx:**  
  - Đọc/ghi `tab` và `namespace` từ URL; filter Pod/Resources theo `namespace`.  
  - ServiceAccount: row + View → `/identities/uid/${id}`.  
  - Role/RoleBinding: row + View → `/clusters/${clusterId}` (khi có clusterId).  
  - Tab click cập nhật `?tab=`.
- **PodDetail.tsx:** Link Node (Overview), Related: View cluster, View node, Back to Resources.
- **ClusterDetail.tsx:** Namespace trong Inventory → `/resources?tab=Pod&namespace=...`.
- **IdentityDetail.tsx:** Đã dùng "Back to Identities" → `/resources?tab=ServiceAccount`.

---

## 7. Kiểm tra nhanh

1. Resources → Pod → View → PodDetail; Back to Resources → tab Pod.
2. Resources → Service Account → View identity → IdentityDetail; Back → tab Service Account.
3. Resources → Role/RoleBinding → View cluster → ClusterDetail.
4. PodDetail → View cluster / View node → ClusterDetail / NodeDetail.
5. ClusterDetail → Namespace → Resources với tab Pod và namespace đã chọn.
6. RiskDetail → Affected Pod / SA → PodDetail / IdentityDetail.
7. URL `/resources?tab=ServiceAccount&namespace=default` mở đúng tab và filter.
