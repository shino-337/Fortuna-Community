# Nguyên nhân gốc rễ: Audit Logs không sync tới Dashboard

## Tóm tắt

**Vấn đề**: Dashboard không hiển thị audit logs vì **không có audit logs nào được tạo trong database** khi agent sync ServiceAccounts từ watcher events.

**Trạng thái hiện tại**:
- ✅ Watcher phát hiện SA mới
- ✅ Watcher gửi event đến Core (với `IsDeltaSync: true`)
- ✅ Core nhận được sync request (200 OK)
- ✅ SA được tạo/cập nhật trong DB
- ❌ **Audit logs KHÔNG được tạo**

## Phân tích chi tiết

### 1. Luồng dữ liệu

```
K8s Cluster
    ↓ (SA created)
Watcher (agent/internal/watcher/watcher.go)
    ↓ (onServiceAccountAdd)
    - Log: "ServiceAccount added: default/test-debug-1763356996"
    - Set: IsDeltaSync: true, IsFullSync: false
    ↓ (sendUpdate)
gRPC Client (agent/internal/client/grpc_client.go)
    ↓ (SendData)
    - Convert to map: isDeltaSync, isFullSync
    ↓ (HTTP POST)
Core API (core/internal/api/agent_handlers.go)
    ↓ (SyncDataFromAgent)
    - Unmarshal JSON to map[string]interface{}
    ↓ (agentService.SyncData)
Core Service (core/internal/service/agent_service.go)
    ↓ (SyncData)
    - Extract isDeltaSync from data["isDeltaSync"]
    - Process ServiceAccounts
    - Create audit logs (❌ KHÔNG HOẠT ĐỘNG)
```

### 2. Kiểm tra từng bước

#### ✅ Bước 1: Watcher phát hiện và gửi event

**File**: `agent/internal/watcher/watcher.go:142-163`

```go
func (w *Watcher) onServiceAccountAdd(obj interface{}) {
    saData := types.ConvertServiceAccount(sa)
    log.Printf("ServiceAccount added: %s/%s", saData.Namespace, saData.Name)
    
    data := &types.CollectedData{
        ClusterID:       w.config.ClusterID,
        ServiceAccounts: []types.ServiceAccountData{saData},
        IsFullSync:       false,
        IsDeltaSync:      true, // ✅ Set đúng
    }
    w.sendUpdate(ctx, data)
}
```

**Log thực tế**: `2025/11/17 05:23:16 ServiceAccount added: default/test-debug-1763356996`
**Trạng thái**: ✅ Hoạt động

#### ✅ Bước 2: Agent client gửi đến Core

**File**: `agent/internal/client/grpc_client.go:93-104`

```go
dataMap := map[string]interface{}{
    "clusterId":           data.ClusterID,
    "serviceAccounts":     convertServiceAccountsToMap(data.ServiceAccounts),
    // ...
    "isFullSync":          data.IsFullSync,  // ✅ Gửi
    "isDeltaSync":         data.IsDeltaSync, // ✅ Gửi
}
```

**Trạng thái**: ✅ Code đúng

#### ✅ Bước 3: Core nhận request

**File**: `core/internal/api/agent_handlers.go:12-34`

```go
func SyncDataFromAgent(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            ClusterID string                 `json:"clusterId"`
            Data      map[string]interface{} `json:"data"` // ✅ Nhận đúng
        }
        // ...
        agentService.SyncData(req.ClusterID, req.Data)
    }
}
```

**Log thực tế**: `[GIN] 2025/11/17 - 05:27:27 | 200 | 695.683084ms | POST "/api/v1/agent/sync"`
**Trạng thái**: ✅ Nhận được request

#### ❌ Bước 4: Core xử lý và tạo audit log

**File**: `core/internal/service/agent_service.go:116-123`

```go
// Check IsDeltaSync flag from data
isDeltaSync := false
if deltaSync, ok := data["isDeltaSync"].(bool); ok {
    isDeltaSync = deltaSync
    log.Printf("SyncData: isDeltaSync = %v (from data)", isDeltaSync) // ❌ KHÔNG THẤY LOG
} else {
    log.Printf("SyncData: isDeltaSync not found in data or not bool, defaulting to false") // ❌ KHÔNG THẤY LOG
}
```

**Vấn đề**: Logs không xuất hiện, có nghĩa là:
1. Code không chạy đến đây (có lỗi trước đó)
2. Logs bị filter hoặc không được in
3. Type assertion fail nhưng không log được

### 3. Nguyên nhân có thể

#### Nguyên nhân 1: Type assertion fail

**Vấn đề**: `data["isDeltaSync"].(bool)` có thể fail nếu:
- JSON unmarshal thành `float64` (số) thay vì `bool`
- JSON unmarshal thành `string` ("true"/"false")
- Key không tồn tại

**Giải pháp**: Kiểm tra type và convert đúng cách:

```go
isDeltaSync := false
if deltaSyncVal, ok := data["isDeltaSync"]; ok {
    switch v := deltaSyncVal.(type) {
    case bool:
        isDeltaSync = v
    case float64:
        isDeltaSync = v != 0
    case string:
        isDeltaSync = (v == "true" || v == "1")
    }
}
```

#### Nguyên nhân 2: Data structure không đúng

**Vấn đề**: `req.Data` có thể không chứa `isDeltaSync` nếu:
- JSON structure không đúng
- Unmarshal không đúng

**Giải pháp**: Log toàn bộ data để kiểm tra:

```go
log.Printf("SyncData: Received data keys: %v", getKeys(data))
log.Printf("SyncData: isDeltaSync value: %v (type: %T)", data["isDeltaSync"], data["isDeltaSync"])
```

#### Nguyên nhân 3: Logic không được gọi

**Vấn đề**: Code có thể không chạy đến phần tạo audit log nếu:
- Có lỗi trước đó (nhưng không được log)
- Early return
- Panic (nhưng được recover)

**Giải pháp**: Thêm logging ở mọi điểm quan trọng

### 4. Giải pháp đề xuất

#### Giải pháp 1: Sửa type assertion

```go
// Check IsDeltaSync flag from data (for watcher events)
isDeltaSync := false
if deltaSyncVal, ok := data["isDeltaSync"]; ok {
    switch v := deltaSyncVal.(type) {
    case bool:
        isDeltaSync = v
        log.Printf("SyncData: isDeltaSync = %v (bool)", isDeltaSync)
    case float64:
        isDeltaSync = v != 0
        log.Printf("SyncData: isDeltaSync = %v (converted from float64)", isDeltaSync)
    case string:
        isDeltaSync = (v == "true" || v == "1")
        log.Printf("SyncData: isDeltaSync = %v (converted from string)", isDeltaSync)
    default:
        log.Printf("SyncData: isDeltaSync type not supported: %T, value: %v", v, v)
    }
} else {
    log.Printf("SyncData: isDeltaSync not found in data")
}
```

#### Giải pháp 2: Thêm logging chi tiết

```go
// Log toàn bộ data structure
log.Printf("SyncData: Received data with %d keys", len(data))
for key := range data {
    log.Printf("SyncData: Data key: %s, type: %T", key, data[key])
}
```

#### Giải pháp 3: Test trực tiếp

Tạo test case để kiểm tra:

```bash
# Test với curl
curl -X POST http://localhost:8080/api/v1/agent/sync \
  -H "Content-Type: application/json" \
  -d '{
    "clusterId": "minikube",
    "data": {
      "serviceAccounts": [{
        "name": "test-manual",
        "namespace": "default",
        "uid": "test-uid-123",
        "labels": {},
        "secrets": []
      }],
      "isDeltaSync": true,
      "isFullSync": false
    }
  }'
```

### 5. Bước tiếp theo

1. ✅ **Đã thêm logging** - Cần rebuild và test lại
2. ⏳ **Sửa type assertion** - Xử lý nhiều kiểu dữ liệu
3. ⏳ **Thêm logging chi tiết** - Log toàn bộ data structure
4. ⏳ **Test trực tiếp** - Gọi API với payload có isDeltaSync
5. ⏳ **Kiểm tra JSON unmarshal** - Xem data có đúng format không

## Kết luận

**Nguyên nhân gốc rễ**: Type assertion `data["isDeltaSync"].(bool)` có thể fail do JSON unmarshal không đúng type, nhưng không có logging để xác định.

**Giải pháp**: 
1. Sửa type assertion để xử lý nhiều kiểu dữ liệu
2. Thêm logging chi tiết để debug
3. Test trực tiếp với payload có isDeltaSync

**Ưu tiên**: Sửa type assertion và thêm logging chi tiết ngay lập tức.

