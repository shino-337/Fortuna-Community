package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/fortuna/core/internal/service"
	"gorm.io/gorm"
)

// FortunaServiceServer implements the gRPC service
type FortunaServiceServer struct {
	db            *gorm.DB
	agentService  *service.AgentService
	// UnimplementedFortunaServiceServer must be embedded for forward compatibility
	// pb.UnimplementedFortunaServiceServer
}

// NewFortunaServiceServer creates a new gRPC service server
func NewFortunaServiceServer(db *gorm.DB) *FortunaServiceServer {
	return &FortunaServiceServer{
		db:           db,
		agentService: service.NewAgentService(db),
	}
}

// SyncData syncs collected data from agent
// This is a placeholder - actual implementation will use generated proto types
func (s *FortunaServiceServer) SyncData(ctx context.Context, req interface{}) (interface{}, error) {
	// TODO: Replace with actual proto types after generating from proto file
	// For now, we'll use a generic approach
	
	// Convert request to map[string]interface{}
	reqData, ok := req.(map[string]interface{})
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid request format")
	}

	clusterID, ok := reqData["cluster_id"].(string)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "cluster_id is required")
	}

	data, ok := reqData["data"].(map[string]interface{})
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "data is required")
	}

	// gRPC path: cluster_id only; mutable fields (name, source, etc.) left empty so Core keeps existing or uses id as name
	if err := s.agentService.SyncData(clusterID, "", "", "", "", data); err != nil {
		return map[string]interface{}{
			"success": false,
			"message": err.Error(),
		}, status.Error(codes.Internal, err.Error())
	}

	return map[string]interface{}{
		"success": true,
		"message": "Data synced successfully",
	}, nil
}

// HealthCheck checks if service is healthy
func (s *FortunaServiceServer) HealthCheck(ctx context.Context, req interface{}) (interface{}, error) {
	// Check database connection
	sqlDB, err := s.db.DB()
	if err != nil {
		return map[string]interface{}{
			"healthy": false,
			"message": "Database connection error",
		}, nil
	}

	if err := sqlDB.Ping(); err != nil {
		return map[string]interface{}{
			"healthy": false,
			"message": "Database ping failed",
		}, nil
	}

	return map[string]interface{}{
		"healthy": true,
		"message": "Service is healthy",
	}, nil
}

