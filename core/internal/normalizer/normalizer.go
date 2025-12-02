package normalizer

import (
	"encoding/json"
	"log"

	"gorm.io/gorm"

	"github.com/ksam/core/pkg/models"
	fortuna "github.com/ksam/core/proto/gen/proto"
)

// Normalizer converts incoming proto/json into canonical internal schema
type Normalizer struct {
	db *gorm.DB
}

// NewNormalizer creates a new normalizer
func NewNormalizer(db *gorm.DB) *Normalizer {
	return &Normalizer{db: db}
}

// NormalizedInventoryItem represents normalized inventory item
type NormalizedInventoryItem struct {
	Kind      string
	UID       string
	Name      string
	Namespace string
	Labels    map[string]string
	RawJSON   string
	Timestamp int64
}

// NormalizedEvent represents normalized runtime event
type NormalizedEvent struct {
	EventID     string
	Timestamp   int64
	ClusterID   string
	NodeID      uint
	PodUID      string
	ContainerID string
	ProcessID   string
	ProcessName string
	SyscallName string
	EventType   string
	Severity    string
	RawPayload  string
}

// NormalizeInventoryItem normalizes an inventory item
func (n *Normalizer) NormalizeInventoryItem(item *fortuna.InventoryItem) (*NormalizedInventoryItem, error) {
	// Parse labels
	labels := make(map[string]string)
	if item.Labels != nil {
		for k, v := range item.Labels {
			labels[k] = v
		}
	}
	
	return &NormalizedInventoryItem{
		Kind:      item.Kind,
		UID:       item.Uid,
		Name:      item.Name,
		Namespace: item.Namespace,
		Labels:    labels,
		RawJSON:   item.RawJson,
		Timestamp: item.Timestamp,
	}, nil
}

// NormalizeEvent normalizes a runtime event
func (n *Normalizer) NormalizeEvent(event *fortuna.Event) (*NormalizedEvent, error) {
	// Parse raw payload if needed
	var rawPayload string
	if event.RawPayload != "" {
		rawPayload = event.RawPayload
	} else {
		// Generate JSON from event fields
		payload := map[string]interface{}{
			"event_id":     event.EventId,
			"timestamp":    event.Timestamp,
			"cluster_id":   event.ClusterId,
			"node_id":      event.NodeId,
			"pod_uid":      event.PodUid,
			"container_id": event.ContainerId,
			"process_id":   event.ProcessId,
			"process_name": event.ProcessName,
			"syscall_name": event.SyscallName,
			"event_type":   event.EventType,
			"severity":     event.Severity,
		}
		jsonBytes, _ := json.Marshal(payload)
		rawPayload = string(jsonBytes)
	}
	
	// Resolve node_id from node_name if needed
	var nodeID uint
	if event.NodeId != "" {
		// Try to parse as integer first (if it's already an ID)
		// Otherwise, query nodes table by node_name
		var node models.Node
		if err := n.db.Where("node_name = ?", event.NodeId).First(&node).Error; err == nil {
			nodeID = node.ID
		} else {
			log.Printf("[Normalizer] Could not resolve node_id for node: %s", event.NodeId)
		}
	}
	
	return &NormalizedEvent{
		EventID:     event.EventId,
		Timestamp:   event.Timestamp,
		ClusterID:   event.ClusterId,
		NodeID:      nodeID,
		PodUID:      event.PodUid,
		ContainerID: event.ContainerId,
		ProcessID:   event.ProcessId,
		ProcessName: event.ProcessName,
		SyscallName: event.SyscallName,
		EventType:   event.EventType,
		Severity:    event.Severity,
		RawPayload:  rawPayload,
	}, nil
}

