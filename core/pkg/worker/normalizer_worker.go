package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"gorm.io/gorm"

	fortuna "github.com/ksam/core/proto/gen/proto"
)

// NormalizerWorker normalizes inventory items
type NormalizerWorker struct {
	js nats.JetStreamContext
	db *gorm.DB
}

// NewNormalizerWorker creates a new normalizer worker
func NewNormalizerWorker(js nats.JetStreamContext, db *gorm.DB) *NormalizerWorker {
	return &NormalizerWorker{
		js: js,
		db: db,
	}
}

// normalizeInventoryItem normalizes an inventory item (migrated from internal/normalizer)
func (w *NormalizerWorker) normalizeInventoryItem(item *fortuna.InventoryItem) (map[string]interface{}, error) {
	// Parse labels
	labels := make(map[string]string)
	if item.Labels != nil {
		for k, v := range item.Labels {
			labels[k] = v
		}
	}
	
	return map[string]interface{}{
		"kind":      item.Kind,
		"uid":       item.Uid,
		"name":      item.Name,
		"namespace": item.Namespace,
		"labels":    labels,
		"raw_json":  item.RawJson,
		"timestamp": item.Timestamp,
	}, nil
}

// Process processes a message from NATS
func (w *NormalizerWorker) Process(ctx context.Context, msg *nats.Msg) error {
	var item fortuna.InventoryItem
	if err := json.Unmarshal(msg.Data, &item); err != nil {
		return fmt.Errorf("failed to unmarshal inventory item: %w", err)
	}

	// Layer 1: Extract eventType from raw_json
	eventType := w.extractEventType(&item)
	
	// Layer 1: Skip old DELETE events (> 1 hour) to prevent ghost pods
	if eventType == "Deleted" {
		age := time.Now().Unix() - item.Timestamp
		if age > 3600 { // 1 hour
			log.Printf("[NormalizerWorker] Skipping old DELETE event for %s/%s (age: %d seconds, Layer 1: Prevention)",
				item.Namespace, item.Name, age)
			return nil
		}
		log.Printf("[NormalizerWorker] Processing DELETE event for %s/%s (age: %d seconds)", 
			item.Namespace, item.Name, age)
	}

	log.Printf("[NormalizerWorker] Processing item: kind=%s, name=%s/%s, eventType=%s", 
		item.Kind, item.Namespace, item.Name, eventType)

	// Normalize the item (migrated from internal/normalizer)
	normalized, err := w.normalizeInventoryItem(&item)
	if err != nil {
		return fmt.Errorf("failed to normalize item: %w", err)
	}

	// Enrich with additional metadata
	enriched := w.enrichNormalizedItem(normalized, &item)
	
	// Layer 1: Add eventType to enriched data
	enriched["event_type"] = eventType

	// Publish normalized item to next stream
	normalizedData, err := json.Marshal(enriched)
	if err != nil {
		return fmt.Errorf("failed to marshal normalized item: %w", err)
	}

	// Publish to normalized stream
	subject := fmt.Sprintf("ksam.normalized.%s", getItemType(item.Kind))
	if _, err := w.js.Publish(subject, normalizedData); err != nil {
		return fmt.Errorf("failed to publish normalized item: %w", err)
	}

	log.Printf("[NormalizerWorker] Normalized and published: kind=%s, name=%s/%s, eventType=%s to %s",
		item.Kind, item.Namespace, item.Name, eventType, subject)
	return nil
}

// extractEventType extracts eventType from raw_json metadata (Layer 1)
func (w *NormalizerWorker) extractEventType(item *fortuna.InventoryItem) string {
	if item.RawJson == "" {
		return "Added" // Default
	}
	
	var podJSON map[string]interface{}
	if err := json.Unmarshal([]byte(item.RawJson), &podJSON); err != nil {
		return "Added" // Default if parse fails
	}
	
	if eventType, ok := podJSON["_ksam_event_type"].(string); ok {
		return eventType
	}
	
	return "Added" // Default
}

// enrichNormalizedItem adds additional metadata to normalized item
func (w *NormalizerWorker) enrichNormalizedItem(normalized map[string]interface{}, original *fortuna.InventoryItem) map[string]interface{} {
	enriched := make(map[string]interface{})
	for k, v := range normalized {
		enriched[k] = v
	}
	enriched["processed_at"] = time.Now().Unix()
	enriched["cluster_id"] = w.extractClusterID(original)
	return enriched
}

// extractClusterID extracts cluster ID from item labels or metadata
func (w *NormalizerWorker) extractClusterID(item *fortuna.InventoryItem) string {
	// Try to get from labels first
	if item.Labels != nil {
		if clusterID, ok := item.Labels["cluster"]; ok {
			return clusterID
		}
		if clusterID, ok := item.Labels["cluster-id"]; ok {
			return clusterID
		}
	}
	// Default to "default" if not found
	return "default"
}

// Subject returns the NATS subject to subscribe to
// Subscribes to ksam.raw.* as per Architecture Review Issue #2
func (w *NormalizerWorker) Subject() string {
	return "ksam.raw.>"
}

// Name returns the worker name
func (w *NormalizerWorker) Name() string {
	return "normalizer"
}

// getItemType maps proto Kind to stream type
func getItemType(kind string) string {
	switch kind {
	case "Pod":
		return "pods"
	case "ServiceAccount":
		return "serviceaccounts"
	case "Role", "ClusterRole":
		return "roles"
	case "RoleBinding", "ClusterRoleBinding":
		return "rolebindings"
	default:
		return "unknown"
	}
}
