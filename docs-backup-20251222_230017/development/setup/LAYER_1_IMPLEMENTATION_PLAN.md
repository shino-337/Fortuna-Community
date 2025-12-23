# Layer 1 Implementation Plan - Agent Prevention

## 📊 Hiện Trạng

### ✅ Đã có:
1. **PodWatcher**: Xử lý tất cả event types (Add/Update/Delete) ✅
2. **handlePod**: Nhận eventType từ PodWatcher ✅
3. **InventoryItem**: Đã có timestamp field ✅
4. **StreamInventory**: Agent gửi inventory items qua gRPC stream ✅

### ⚠️ Cần cải thiện:
1. **Timestamp**: Converter dùng `CreationTimestamp` thay vì `current time`
2. **EventType**: Converter nhận eventType nhưng không sử dụng
3. **DELETE Events**: Core chưa xử lý DELETE events từ inventory stream

---

## 🎯 Mục Tiêu Layer 1

**Ngăn ghost pods từ nguồn** bằng cách:
1. ✅ Agent gửi DELETE events
2. ✅ Thêm timestamp vào messages (current time)
3. ✅ Track pod lifecycle

---

## 📝 Implementation Plan

### Step 1: Update Agent Converter

**File**: `KSAM/agent/internal/converter/converter.go`

**Changes**:
1. Set `timestamp = time.Now().Unix()` thay vì `CreationTimestamp`
2. Thêm `eventType` vào metadata (có thể thêm vào raw_json hoặc tạo metadata field mới)

**Code**:
```go
func PodToInventoryItem(pod *corev1.Pod, clusterID string, eventType watch.EventType) *fortuna.InventoryItem {
    // ... existing code ...
    
    // Layer 1: Use current time instead of CreationTimestamp
    timestamp := time.Now().Unix()
    
    // Layer 1: Add eventType to raw_json metadata
    rawJSON := marshalObject(pod)
    var podJSON map[string]interface{}
    json.Unmarshal([]byte(rawJSON), &podJSON)
    podJSON["_ksam_event_type"] = string(eventType) // "Added", "Modified", "Deleted"
    podJSON["_ksam_timestamp"] = timestamp
    rawJSONBytes, _ := json.Marshal(podJSON)
    
    return &fortuna.InventoryItem{
        Kind:      "Pod",
        Uid:       string(pod.UID),
        Name:      pod.Name,
        Namespace: pod.Namespace,
        Labels:    labels,
        RawJson:   string(rawJSONBytes),
        Timestamp: timestamp, // Layer 1: Current time
    }
}
```

### Step 2: Update Core NormalizerWorker

**File**: `KSAM/core/pkg/worker/normalizer_worker.go`

**Changes**:
1. Extract eventType từ raw_json
2. Thêm eventType vào normalized item
3. Skip processing nếu eventType = "Deleted" và message cũ (> 1 hour)

**Code**:
```go
func (w *NormalizerWorker) Process(ctx context.Context, msg *nats.Msg) error {
    var item fortuna.InventoryItem
    // ... unmarshal ...
    
    // Layer 1: Extract eventType from raw_json
    eventType := extractEventType(&item)
    
    // Layer 1: Skip old DELETE events (> 1 hour)
    if eventType == "Deleted" {
        age := time.Now().Unix() - item.Timestamp
        if age > 3600 { // 1 hour
            log.Printf("[NormalizerWorker] Skipping old DELETE event for %s/%s (age: %d seconds)", 
                item.Namespace, item.Name, age)
            return nil
        }
    }
    
    // ... rest of processing ...
}
```

### Step 3: Update Core CorrelatorWorker

**File**: `KSAM/core/pkg/worker/correlator_worker.go`

**Changes**:
1. Extract eventType từ normalized data
2. Nếu eventType = "Deleted", soft delete pod trong DB
3. Nếu eventType = "Added" hoặc "Modified", xử lý bình thường

**Code**:
```go
func (w *CorrelatorWorker) processPod(data map[string]interface{}, clusterID string) error {
    // ... existing code ...
    
    // Layer 1: Extract eventType
    eventType, _ := data["event_type"].(string)
    if eventType == "" {
        // Try to extract from raw_json
        eventType = extractEventTypeFromRawJSON(rawJSON)
    }
    
    // Layer 1: Handle DELETE event
    if eventType == "Deleted" {
        // Soft delete pod
        result := w.db.Model(&models.Pod{}).Where("uid = ?", uid).Update("deleted_at", time.Now())
        if result.Error != nil {
            return fmt.Errorf("failed to soft delete pod: %w", result.Error)
        }
        log.Printf("[CorrelatorWorker] Soft-deleted pod %s/%s (UID: %s) from DELETE event", 
            namespace, name, uid)
        return nil
    }
    
    // ... rest of processing for Add/Update ...
}
```

---

## 🔄 Alternative Approach: Proto Update

Nếu muốn rõ ràng hơn, có thể thêm `eventType` field vào `InventoryItem` proto:

```protobuf
message InventoryItem {
  string kind = 1;
  string uid = 2;
  string name = 3;
  string namespace = 4;
  map<string, string> labels = 5;
  string raw_json = 6;
  int64 timestamp = 7;
  string event_type = 8;  // "Added", "Modified", "Deleted"
}
```

**Pros**: Rõ ràng, dễ xử lý
**Cons**: Cần regenerate proto, breaking change

---

## 📊 Implementation Priority

1. **High**: Update converter timestamp (current time)
2. **High**: Add eventType to metadata (raw_json)
3. **Medium**: Core xử lý DELETE events
4. **Low**: Proto update (nếu cần)

---

## ✅ Expected Results

Sau khi implement Layer 1:
- ✅ Agent gửi DELETE events với timestamp hiện tại
- ✅ Core xử lý DELETE events và soft delete pods
- ✅ Giảm ghost pods từ nguồn
- ✅ Kết hợp với Layer 2, 3, 4 tạo 4 tầng bảo vệ hoàn chỉnh

