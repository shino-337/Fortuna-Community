package ingest

import (
	"context"
	"fmt"
	"log"

	"github.com/ksam/core/pkg/messaging"
	fortuna "github.com/ksam/core/proto/gen/proto"
)

// IngestAPI handles incoming data from agents
type IngestAPI struct {
	publisher    *messaging.Publisher
	rateLimiter  *RateLimiter
	globalLimiter *GlobalRateLimiter
}

// NewIngestAPI creates a new Ingest API
func NewIngestAPI(natsClient *messaging.NATSClient) *IngestAPI {
	config := DefaultRateLimitConfig()
	return &IngestAPI{
		publisher:     messaging.NewPublisher(natsClient),
		rateLimiter:   NewRateLimiter(config),
		globalLimiter: NewGlobalRateLimiter(config),
	}
}

// GetPublisher returns the publisher (for use in handlers)
func (api *IngestAPI) GetPublisher() *messaging.Publisher {
	return api.publisher
}

// RegisterAgent handles agent registration
func (api *IngestAPI) RegisterAgent(ctx context.Context, req *fortuna.RegisterRequest) (*fortuna.RegisterResponse, error) {
	log.Printf("[IngestAPI] Agent registration: cluster=%s, node=%s, agent=%s", req.ClusterId, req.NodeName, req.AgentId)

	// TODO: Store agent info in database
	// For now, just acknowledge

	return &fortuna.RegisterResponse{
		Ok:      true,
		Message: "Agent registered successfully",
	}, nil
}

// StreamInventory handles inventory items from agent
func (api *IngestAPI) StreamInventory(stream fortuna.AgentService_StreamInventoryServer) error {
	// Extract agent ID from context (set by gRPC handler)
	ctx := stream.Context()
	agentID := "unknown"
	if md, ok := ctx.Value("agent_id").(string); ok {
		agentID = md
	} else {
		// Try to extract from peer info as fallback
		// This will be set by handler_new.go
		agentID = "unknown"
	}

	log.Printf("[IngestAPI] Starting inventory stream from agent: %s", agentID)

	itemCount := 0
	rateLimitedCount := 0

	for {
		item, err := stream.Recv()
		if err != nil {
			log.Printf("[IngestAPI] Stream closed from agent %s: %v (processed %d items, rate limited %d)",
				agentID, err, itemCount, rateLimitedCount)
			break
		}

		// Check global rate limit
		if !api.globalLimiter.Allow() {
			rateLimitedCount++
			log.Printf("[IngestAPI] Global rate limit exceeded, dropping item from agent %s", agentID)
			// Continue to next item (don't block stream)
			continue
		}

		// Check per-agent rate limit
		if !api.rateLimiter.Allow(agentID) {
			rateLimitedCount++
			log.Printf("[IngestAPI] Rate limit exceeded for agent %s, dropping item", agentID)
			// Continue to next item (don't block stream)
			continue
		}

		// Publish to NATS based on item type
		itemType := getItemType(item.Kind)
		if err := api.publisher.PublishInventory(itemType, item); err != nil {
			log.Printf("[IngestAPI] Failed to publish item from agent %s: %v", agentID, err)
			// Continue processing other items
			continue
		}

		itemCount++
		if itemCount%100 == 0 {
			log.Printf("[IngestAPI] Processed %d items from agent %s (rate limited: %d)",
				itemCount, agentID, rateLimitedCount)
		}
	}

	return stream.SendAndClose(&fortuna.RegisterResponse{
		Ok:      true,
		Message: fmt.Sprintf("Inventory stream completed (processed %d items)", itemCount),
	})
}

// StreamEvents handles runtime events from agent
func (api *IngestAPI) StreamEvents(stream fortuna.AgentService_StreamEventsServer) error {
	log.Printf("[IngestAPI] Starting event stream")

	for {
		event, err := stream.Recv()
		if err != nil {
			log.Printf("[IngestAPI] Event stream closed: %v", err)
			break
		}

		// Publish to NATS events stream
		if err := api.publisher.PublishEvent(event); err != nil {
			log.Printf("[IngestAPI] Failed to publish event: %v", err)
			continue
		}

		log.Printf("[IngestAPI] Published event: type=%s, pod=%s", event.EventType, event.PodUid)
	}

	return stream.SendAndClose(&fortuna.RegisterResponse{
		Ok:      true,
		Message: "Event stream completed",
	})
}

// Heartbeat handles agent heartbeat
func (api *IngestAPI) Heartbeat(ctx context.Context, req *fortuna.RegisterRequest) (*fortuna.RegisterResponse, error) {
	// TODO: Update agent last seen timestamp in database
	log.Printf("[IngestAPI] Heartbeat from agent: %s, cluster: %s", req.AgentId, req.ClusterId)

	return &fortuna.RegisterResponse{
		Ok:      true,
		Message: "Heartbeat received",
	}, nil
}

// getItemType maps proto Kind to NATS subject type
func getItemType(kind string) string {
	switch kind {
	case "Pod":
		return "pods"
	case "ServiceAccount":
		return "serviceaccounts"
	case "Role":
		return "roles"
	case "ClusterRole":
		return "roles" // ClusterRoles also go to roles stream
	case "RoleBinding":
		return "rolebindings"
	case "ClusterRoleBinding":
		return "rolebindings" // ClusterRoleBindings also go to rolebindings stream
	default:
		return "unknown"
	}
}
