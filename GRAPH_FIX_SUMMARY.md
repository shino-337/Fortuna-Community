# Graph View Filter Fix - v1.0.1

## Vấn đề đã phát hiện

### Lỗi chính
```
Uncaught TypeError: can't access property "x1", n is undefined
Uncaught TypeError: can't access property "h", i is undefined
Uncaught TypeError: can't access property "w", s is undefined
```

### Nguyên nhân
1. **Layout timing issues**: Cytoscape layout algorithms (cose, fcose, etc.) cố truy cập vào node dimensions/positions trước khi nodes được render đầy đủ
2. **Fit() bounding box errors**: Khi gọi `fit()`, Cytoscape cần tính bounding box từ node positions, nhưng nodes chưa có positions hợp lệ
3. **Filter change race condition**: Khi user filter/reset, nodes được remove và add lại, nhưng layout và fit() được gọi trước khi nodes ổn định

## Giải pháp đã triển khai

### 1. Đơn giản hóa Layout Logic
```typescript
// TRƯỚC: Sử dụng random layout trước, sau đó main layout
// => Quá nhiều steps, dễ gây timing issues

// SAU: Chạy trực tiếp main layout với animation disabled
const layoutOptions: LayoutOptions = {
  ...baseOptions,
  nodeDimensionsIncludeLabels: false,
  animate: false, // Disable animation để tránh timing issues
}
```

### 2. Wrap tất cả fit() calls trong try-catch
```typescript
// Verify nodes có valid positions trước khi fit
const nodesWithPositions = currentNodes.filter(n => {
  try {
    const pos = n.position()
    return pos && !isNaN(pos.x) && !isNaN(pos.y)
  } catch {
    return false
  }
})

if (nodesWithPositions.length === currentNodes.length) {
  try {
    cyRef.current.resize()
    cyRef.current.fit(undefined, 50)
    setZoomLevel(cyRef.current.zoom())
  } catch (fitError) {
    console.error('❌ Fit error:', fitError)
  }
}
```

### 3. Protected fit() trong tất cả handlers
- `handleFit()`: Wrapped trong try-catch
- `handleReset()`: Wrapped trong try-catch
- Layout callbacks: Verify nodes trước khi fit

### 4. Tăng delays và wait times
- Delay sau layout completion: 100ms → 200ms
- Delay khi nodes chưa có dimensions: 200ms → 500ms
- Delay sau filter changes: 150ms → 300ms

## Code Changes

### File: `dashboard/src/components/Graph/GraphVisualization.tsx`

#### 1. Simplified runLayout()
```typescript
// Lines 581-625
// Removed random layout pre-initialization
// Direct layout run with animation disabled
// Added position verification before fit()
```

#### 2. Protected handleFit()
```typescript
// Lines 1592-1600
const handleFit = useCallback(() => {
  if (!cyRef.current) return
  try {
    cyRef.current.fit(undefined, 50)
    setZoomLevel(cyRef.current.zoom())
  } catch (error) {
    console.error('❌ [GraphVisualization] Fit error:', error)
  }
}, [])
```

#### 3. Protected handleReset()
```typescript
// Lines 1602-1621
const handleReset = useCallback(() => {
  if (!cyRef.current) return
  
  try {
    cyRef.current.fit(undefined, 50)
    setZoomLevel(cyRef.current.zoom())
  } catch (error) {
    console.error('❌ [GraphVisualization] Reset fit error:', error)
  }
  
  // Reset selection and classes...
}, [])
```

#### 4. Protected fallback layouts
```typescript
// Lines 645-695
// All fallback layout fit() calls wrapped in try-catch
```

## Build & Deploy

### 1. Clean build
```bash
cd dashboard
rm -rf node_modules/.vite dist
npm run build
```

### 2. Clean Docker images
```bash
# Xóa tất cả old dashboard images
docker images | grep ksam-dashboard | awk '{print $3}' | xargs -I {} docker rmi -f {}
```

### 3. Build new image
```bash
docker build --no-cache -t ksam-dashboard:v1.0.1 -f dashboard/Dockerfile dashboard/
```

### 4. Deploy to Minikube
```bash
# Xóa old images trong minikube
minikube image rm $(minikube image ls | grep ksam-dashboard | awk '{print $1}')

# Load new image
minikube image load ksam-dashboard:v1.0.1

# Update deployment
kubectl set image deployment/ksam-dashboard -n ksam dashboard=ksam-dashboard:v1.0.1
kubectl rollout restart deployment/ksam-dashboard -n ksam
```

## Verification

### Verify code trong pod
```bash
kubectl exec -n ksam POD_NAME -- sh -c "ls /usr/share/nginx/html/assets/*.js"
# Expected: index-BQBrWWL3.js (new build)

kubectl exec -n ksam POD_NAME -- sh -c "grep -o 'Fit error' /usr/share/nginx/html/assets/*.js"
# Expected: "Fit error" (new error handling code)
```

### Test trên browser
1. Hard refresh: Ctrl+Shift+R
2. Open console (F12)
3. Test các cases:
   - Filter by namespace
   - Reset filters
   - Change node type filters
   - Change connection type filters
   - Zoom and Fit controls

### Expected logs
```
🎨 [GraphVisualization] Running layout: cose with 86 nodes
✅ [GraphVisualization] Layout completed
✅ [GraphVisualization] Fit completed
```

### ❌ Không còn errors
```
# Không còn:
TypeError: can't access property "x1", n is undefined
TypeError: can't access property "h", i is undefined
TypeError: can't access property "w", s is undefined
```

## Deployment Info

- **Image**: `ksam-dashboard:v1.0.1`
- **Pod**: `ksam-dashboard-6bb8c78475-*`
- **JS bundle**: `index-BQBrWWL3.js`
- **URL**: http://192.168.49.2:30080/graph

## Next Steps

Nếu vẫn còn lỗi:
1. Check browser console logs chi tiết
2. Verify node positions ngay sau khi add: `cyRef.current.nodes().forEach(n => console.log(n.id(), n.position()))`
3. Check layout options cho từng layout type
4. Consider fallback to simpler layouts (grid, random) for large graphs

## Key Takeaways

1. **Always wrap fit() calls**: Bounding box calculation có thể fail nếu nodes chưa có valid positions
2. **Disable animations during layout**: Animations có thể cause timing issues với filter changes
3. **Verify node state before layout**: Check dimensions và positions trước khi run layout
4. **Use simpler fallbacks**: Khi complex layouts fail, fallback to simple layouts (random, grid)
5. **Proper error handling**: Catch và log errors thay vì crash toàn bộ graph

