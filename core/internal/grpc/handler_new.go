package grpc

import (
	"context"
	"io"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/ksam/core/internal/ingest"
	"github.com/ksam/core/pkg/messaging"
	"gorm.io/gorm"

	fortuna "github.com/ksam/core/proto/gen/proto"
)

// AgentServiceServer implements the new AgentService from fortuna_agent.proto
type AgentServiceServer struct {
	db        *gorm.DB
	ingestAPI *ingest.IngestAPI
	// Embed for forward compatibility
	fortuna.UnimplementedAgentServiceServer
}

// Publisher returns the publisher for direct access
func (s *AgentServiceServer) Publisher() *messaging.Publisher {
	return s.ingestAPI.GetPublisher()
}

// NewAgentServiceServer creates a new AgentService server
func NewAgentServiceServer(db *gorm.DB, natsClient *messaging.NATSClient) *AgentServiceServer {
	return &AgentServiceServer{
		db:        db,
		ingestAPI: ingest.NewIngestAPI(natsClient),
	}
}

// Register handles agent registration
func (s *AgentServiceServer) Register(ctx context.Context, req *fortuna.RegisterRequest) (*fortuna.RegisterResponse, error) {
	log.Printf("[AgentService] Register: cluster=%s, node=%s, version=%s",
		req.ClusterId, req.NodeName, req.Version)

	return s.ingestAPI.RegisterAgent(ctx, req)
}

// StreamInventory handles streaming inventory items from agent
func (s *AgentServiceServer) StreamInventory(stream fortuna.AgentService_StreamInventoryServer) error {
	// Extract agent ID from peer information
	ctx := stream.Context()
	agentID := "unknown"

	// Try to get agent ID from peer metadata
	if peer, ok := peer.FromContext(ctx); ok {
		// Agent ID might be in metadata or we can use peer address as identifier
		// For now, use a combination of peer info
		agentID = peer.Addr.String()
	}

	// Try to get from context if set by interceptor
	if id, ok := ctx.Value("agent_id").(string); ok {
		agentID = id
	}

	log.Printf("[AgentService] StreamInventory started from agent: %s", agentID)

	// Create context with agent ID for IngestAPI
	ctxWithAgentID := context.WithValue(ctx, "agent_id", agentID)

	// Delegate to IngestAPI (which handles rate limiting)
	return s.ingestAPI.StreamInventory(streamWithContext{stream: stream, ctx: ctxWithAgentID})
}

// streamWithContext wraps the stream with a custom context
type streamWithContext struct {
	stream fortuna.AgentService_StreamInventoryServer
	ctx    context.Context
}

func (s streamWithContext) Context() context.Context {
	return s.ctx
}

func (s streamWithContext) Recv() (*fortuna.InventoryItem, error) {
	return s.stream.Recv()
}

func (s streamWithContext) SendAndClose(resp *fortuna.RegisterResponse) error {
	return s.stream.SendAndClose(resp)
}

func (s streamWithContext) RecvMsg(m interface{}) error {
	return s.stream.RecvMsg(m)
}

func (s streamWithContext) SendMsg(m interface{}) error {
	return s.stream.SendMsg(m)
}

func (s streamWithContext) SendHeader(md metadata.MD) error {
	return s.stream.SendHeader(md)
}

func (s streamWithContext) SetHeader(md metadata.MD) error {
	return s.stream.SetHeader(md)
}

func (s streamWithContext) SetTrailer(md metadata.MD) {
	s.stream.SetTrailer(md)
}

// StreamEvents handles streaming runtime events from agent
func (s *AgentServiceServer) StreamEvents(stream fortuna.AgentService_StreamEventsServer) error {
	log.Println("[AgentService] StreamEvents started")

	var count int
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			log.Printf("[AgentService] StreamEvents completed, processed %d events", count)
			return stream.SendAndClose(&fortuna.RegisterResponse{
				Ok:      true,
				Message: "Events stream completed",
			})
		}
		if err != nil {
			log.Printf("[AgentService] StreamEvents error: %v", err)
			return status.Error(codes.Internal, err.Error())
		}

		// Publish event to NATS
		if err := s.Publisher().PublishEvent(event); err != nil {
			log.Printf("[AgentService] Failed to publish event: %v", err)
			// Continue processing other events
			continue
		}

		count++
		if count%100 == 0 {
			log.Printf("[AgentService] Processed %d events", count)
		}
	}
}

// Heartbeat handles periodic heartbeat from agent
func (s *AgentServiceServer) Heartbeat(ctx context.Context, req *fortuna.RegisterRequest) (*fortuna.RegisterResponse, error) {
	log.Printf("[AgentService] Heartbeat: cluster=%s, node=%s", req.ClusterId, req.NodeName)

	// Update last_seen for node
	// TODO: Implement node last_seen update

	return s.ingestAPI.Heartbeat(ctx, req)
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
