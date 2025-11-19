# 🐛 DEBUG: Namespace Filter Issue

## Vấn đề hiện tại
- ✅ Backend API filter hoạt động (`?namespace=default` → 21 nodes)
- ❌ Frontend UI không cập nhật khi thay đổi namespace filter

## Đã triển khai Debug Version

**Version:** `v1.0.2-debug`  
**Deployed:** Pods đã restart với image mới

### Debug Logs được thêm:

#### 1. `useGraph` hook (hooks/useGraph.ts):
```
[DEBUG useGraph] Called with params: { cluster, namespace }
[DEBUG useGraph] Fetching data with params: ...
[DEBUG useGraph] Response: { nodes: X, edges: Y }
```

#### 2. `GraphView` component (pages/GraphView.tsx):
```
[DEBUG GraphView] Cluster changed to: ...
[DEBUG GraphView] Namespace changed to: ...
```

## Cách kiểm tra

### Bước 1: Truy cập Dashboard
```bash
minikube service ksam-dashboard -n ksam --url
# Hoặc sử dụng URL đang chạy
```

### Bước 2: Mở Browser Console
1. Nhấn **F12** để mở DevTools
2. Chuyển sang tab **Console**
3. Clear console: Click icon 🚫 hoặc nhấn `Ctrl+L`

### Bước 3: Test Namespace Filter

#### Test Case 1: Nhập "default"
1. Trong Graph View, nhập `default` vào Namespace input
2. **Quan sát Console logs:**
   - Bạn sẽ thấy gì?
   - `[DEBUG GraphView] Namespace changed to: default` ← State có update?
   - `[DEBUG useGraph] Called with params: { namespace: 'default' }` ← Hook có được gọi?
   - `[DEBUG useGraph] Fetching data...` ← API có được call?
   - `[DEBUG useGraph] Response: { nodes: 21, edges: 20 }` ← Data có về đúng?

#### Test Case 2: Clear filter
1. Xóa text trong Namespace input (để trống)
2. **Quan sát Console logs:**
   - `[DEBUG GraphView] Namespace changed to: ''`
   - `[DEBUG useGraph] Called with params: {}`
   - `[DEBUG useGraph] Response: { nodes: 86, edges: 85 }`

#### Test Case 3: Đổi qua lại giữa các namespaces
1. Nhập `default` → Xem console
2. Nhập `kube-system` → Xem console
3. Xóa (clear) → Xem console

### Bước 4: Screenshot các logs

Chụp lại màn hình console cho từng test case:
- Test 1: Filter by `default`
- Test 2: Clear filter
- Test 3: Filter by `kube-system`

## Các kịch bản có thể xảy ra

### Kịch bản A: State không update
**Logs thấy:**
- ❌ KHÔNG thấy `[DEBUG GraphView] Namespace changed to: ...`

**Nguyên nhân:**
- `GraphFilters` component không gọi `onNamespaceChange` callback
- Debounce timer bị cancel trước khi fire

**Giải pháp:**
- Kiểm tra `GraphFilters.tsx` → `handleNamespaceInputChange`
- Kiểm tra `selectNamespace` function
- Có thể cần bỏ debounce hoặc giảm thời gian từ 500ms xuống 100ms

---

### Kịch bản B: State update nhưng không re-render
**Logs thấy:**
- ✅ `[DEBUG GraphView] Namespace changed to: default`
- ❌ KHÔNG thấy `[DEBUG useGraph] Called with params: ...`

**Nguyên nhân:**
- `GraphVisualization` component không re-render khi `namespace` prop thay đổi
- Props không được pass đúng cách

**Giải pháp:**
- Kiểm tra `GraphView.tsx` → Đảm bảo `namespace={namespace}` được truyền vào `<GraphVisualization>`
- Có thể cần thêm `key={namespace}` để force re-render

---

### Kịch bản C: useGraph được gọi nhưng không fetch
**Logs thấy:**
- ✅ `[DEBUG GraphView] Namespace changed to: default`
- ✅ `[DEBUG useGraph] Called with params: { namespace: 'default' }`
- ❌ KHÔNG thấy `[DEBUG useGraph] Fetching data...`

**Nguyên nhân:**
- React Query cache hit - không fetch lại vì nghĩ data không thay đổi
- Query key không đúng

**Giải pháp:**
- ✅ ĐÃ FIX: Changed `queryKey` from `['graph', params]` to `['graph', params?.cluster, params?.namespace]`
- Nếu vẫn bị, cần thêm `gcTime: 0` hoặc manually invalidate query

---

### Kịch bản D: Fetch thành công nhưng UI không update
**Logs thấy:**
- ✅ `[DEBUG GraphView] Namespace changed to: default`
- ✅ `[DEBUG useGraph] Called with params: { namespace: 'default' }`
- ✅ `[DEBUG useGraph] Fetching data...`
- ✅ `[DEBUG useGraph] Response: { nodes: 21, edges: 20 }`
- ❌ Graph vẫn hiển thị 86 nodes (không update)

**Nguyên nhân:**
- Cytoscape không update elements khi data thay đổi
- `useMemo` dependencies không đúng
- Update useEffect không trigger

**Giải pháp:**
- Kiểm tra `GraphVisualization.tsx`:
  - `useMemo` for `nodes` and `edges` - dependencies có bao gồm `data` không?
  - Update `useEffect` - dependencies có bao gồm `nodes`, `edges` không?
- Có thể cần force Cytoscape reload: `cy.elements().remove()` trước khi add mới

---

## Quick Fixes dựa trên logs

### Fix 1: Force invalidate query khi namespace thay đổi
Nếu vấn đề là React Query cache, thêm vào `GraphView.tsx`:

```typescript
import { useQueryClient } from '@tanstack/react-query'

const GraphView = () => {
  const queryClient = useQueryClient()
  
  const handleNamespaceChange = (value: string) => {
    setNamespace(value)
    queryClient.invalidateQueries({ queryKey: ['graph'] }) // Force refetch
  }
}
```

### Fix 2: Bỏ debounce trong namespace input
Nếu vấn đề là debounce, trong `GraphFilters.tsx`:

```typescript
const handleNamespaceInputChange = (value: string) => {
  setLocalNamespace(value)
  onNamespaceChange(value) // Gọi ngay, không đợi debounce
  // ... suggestions logic ...
}
```

### Fix 3: Force re-render với key
Nếu vấn đề là component không re-render, trong `GraphView.tsx`:

```tsx
<GraphVisualization 
  key={`${cluster}-${namespace}`} // Force remount when filters change
  cluster={cluster || undefined} 
  namespace={namespace || undefined} 
  // ... other props
/>
```

---

## Sau khi xác định nguyên nhân

1. **Chụp và gửi logs** để xác định đúng kịch bản
2. **Apply fix tương ứng**
3. **Xóa debug logs** (revert về production)
4. **Rebuild và deploy** final version

---

## Remove Debug Logs (sau khi fix)

```bash
# Revert các file đã sửa
git diff HEAD -- dashboard/src/hooks/useGraph.ts
git diff HEAD -- dashboard/src/pages/GraphView.tsx

# Hoặc manual:
# - Xóa các console.log trong useGraph.ts
# - Xóa các console.log trong GraphView.tsx
# - Rebuild: npm run build
# - Deploy: docker build + minikube image load + kubectl patch
```

---

**Version:** v1.0.2-debug  
**Status:** 🔍 Waiting for test results  
**Date:** 2024-11-19

