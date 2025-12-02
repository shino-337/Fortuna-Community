package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"gorm.io/gorm"

	"github.com/ksam/core/internal/normalizer"
	fortuna "github.com/ksam/core/proto/gen/proto"
)

// NormalizerWorker normalizes inventory items
type NormalizerWorker struct {
	js         nats.JetStreamContext
	db         *gorm.DB
	normalizer *normalizer.Normalizer
}

// NewNormalizerWorker creates a new normalizer worker
func NewNormalizerWorker(js nats.JetStreamContext, db *gorm.DB) *NormalizerWorker {
	return &NormalizerWorker{
		js:         js,
		db:         db,
		normalizer: normalizer.NewNormalizer(db),
	}
}

// Process processes a message from NATS
func (w *NormalizerWorker) Process(ctx context.Context, msg *nats.Msg) error {
	var item fortuna.InventoryItem
	if err := json.Unmarshal(msg.Data, &item); err != nil {
		return fmt.Errorf("failed to unmarshal inventory item: %w", err)
	}

	log.Printf("[NormalizerWorker] Processing item: kind=%s, name=%s/%s", item.Kind, item.Namespace, item.Name)

	// Normalize the item using normalizer package
	normalized, err := w.normalizer.NormalizeInventoryItem(&item)
	if err != nil {
		return fmt.Errorf("failed to normalize item: %w", err)
	}

	// Enrich with additional metadata
	enriched := w.enrichNormalizedItem(normalized, &item)

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

	log.Printf("[NormalizerWorker] Normalized and published: kind=%s, name=%s/%s to %s",
		item.Kind, item.Namespace, item.Name, subject)
	return nil
}

// enrichNormalizedItem adds additional metadata to normalized item
func (w *NormalizerWorker) enrichNormalizedItem(normalized *normalizer.NormalizedInventoryItem, original *fortuna.InventoryItem) map[string]interface{} {
	enriched := map[string]interface{}{
		"kind":         normalized.Kind,
		"uid":          normalized.UID,
		"name":         normalized.Name,
		"namespace":    normalized.Namespace,
		"labels":       normalized.Labels,
		"raw_json":     normalized.RawJSON,
		"timestamp":    normalized.Timestamp,
		"processed_at": time.Now().Unix(),
		"cluster_id":   w.extractClusterID(original),
	}
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
