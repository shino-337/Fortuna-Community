package grpc

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/pkg/messaging"
	"github.com/fortuna/core/pkg/models"
)

// SBOMServiceServer implements the SBOM-related RPCs from AgentService
type SBOMServiceServer struct {
	db         *gorm.DB
	natsClient *messaging.NATSClient
	pb.UnimplementedAgentServiceServer
}

// NewSBOMServiceServer creates a new SBOM service server
func NewSBOMServiceServer(db *gorm.DB, natsClient *messaging.NATSClient) *SBOMServiceServer {
	return &SBOMServiceServer{
		db:         db,
		natsClient: natsClient,
	}
}

// SendSBOMFinding handles a single SBOM finding from Agent
func (s *SBOMServiceServer) SendSBOMFinding(ctx context.Context, req *pb.SBOMFinding) (*pb.SBOMResponse, error) {
	log.Printf("[SBOM] Received SBOM from agent=%s, pod=%s, image=%s",
		req.AgentId, req.PodName, req.ImageDigest)

	// Convert proto to internal model
	sbom := &models.SBOM{
		PodUID:        req.PodUid,
		PodName:       req.PodName,
		Namespace:     req.Namespace,
		ContainerName: req.ContainerName,
		ImageName:     req.ImageName,
		ImageDigest:   req.ImageDigest,
		ImageTag:      req.ImageTag,
		GeneratedAt:   req.GeneratedAt.AsTime(),
		AgentID:       req.AgentId,
		NodeID:        req.NodeId,
	}

	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Insert SBOM
	if err := tx.Create(sbom).Error; err != nil {
		tx.Rollback()
		log.Printf("[SBOM] Failed to insert SBOM: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to insert SBOM: %v", err)
	}

	// Insert SBOM components
	for _, pkg := range req.Packages {
		component := &models.SBOMComponent{
			SBOMID:       sbom.ID,
			Name:         pkg.Name,
			Version:      pkg.Version,
			Type:         pkg.Type.String(),
			Architecture: pkg.Architecture,
			Licenses:     pkg.Licenses,
			Source:       pkg.Source,
			Description:  pkg.Description,
			Homepage:     pkg.Homepage,
			Maintainer:   pkg.Maintainer,
		}
		if err := tx.Create(component).Error; err != nil {
			tx.Rollback()
			log.Printf("[SBOM] Failed to insert component %s: %v", pkg.Name, err)
			return nil, status.Errorf(codes.Internal, "failed to insert component: %v", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("[SBOM] Failed to commit transaction: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to commit: %v", err)
	}

	// Publish SBOM_CREATED event to NATS
	if s.natsClient != nil {
		event := map[string]interface{}{
			"sbom_id":       sbom.ID,
			"pod_uid":       sbom.PodUID,
			"image_digest":  sbom.ImageDigest,
			"component_count": len(req.Packages),
			"generated_at":  sbom.GeneratedAt,
		}
		if err := s.natsClient.PublishJSON("ksam.sbom.created", event); err != nil {
			log.Printf("[SBOM] WARNING: Failed to publish SBOM_CREATED event: %v", err)
			// Non-fatal, continue
		} else {
			log.Printf("[SBOM] Published SBOM_CREATED event for sbom_id=%d", sbom.ID)
		}
	}

	log.Printf("[SBOM] Successfully stored SBOM id=%d with %d components", sbom.ID, len(req.Packages))

	return &pb.SBOMResponse{
		Success:    true,
		Message:    "SBOM received and stored",
		SbomId:     fmt.Sprintf("%d", sbom.ID),
		ReceivedAt: timestamppb.New(time.Now()),
	}, nil
}

// BatchSendSBOMFindings handles multiple SBOM findings in a single request
func (s *SBOMServiceServer) BatchSendSBOMFindings(ctx context.Context, req *pb.BatchSBOMRequest) (*pb.BatchSBOMResponse, error) {
	log.Printf("[SBOM] Batch request from agent=%s, node=%s, count=%d",
		req.AgentId, req.NodeId, len(req.Findings))

	resp := &pb.BatchSBOMResponse{
		Total:        int32(len(req.Findings)),
		SuccessCount: 0,
		ErrorCount:   0,
		SbomIds:      []string{},
		Errors:       []string{},
	}

	for _, finding := range req.Findings {
		sbomResp, err := s.SendSBOMFinding(ctx, finding)
		if err != nil {
			resp.ErrorCount++
			resp.Errors = append(resp.Errors, err.Error())
			log.Printf("[SBOM] Batch item failed: %v", err)
		} else {
			resp.SuccessCount++
			resp.SbomIds = append(resp.SbomIds, sbomResp.SbomId)
		}
	}

	log.Printf("[SBOM] Batch complete: success=%d, errors=%d", resp.SuccessCount, resp.ErrorCount)

	return resp, nil
}

// Ping handles health check from agent
func (s *SBOMServiceServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{
		Healthy:   true,
		Version:   "1.0.0", // TODO: Get from build info
		Timestamp: timestamppb.New(time.Now()),
		Message:   "Fortuna Core is healthy",
	}, nil
}

// RegisterAgent handles agent registration
func (s *SBOMServiceServer) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	log.Printf("[Agent] Register: id=%s, node=%s, version=%s, capabilities=%v",
		req.AgentId, req.NodeName, req.AgentVersion, req.Capabilities)

	// TODO: Store agent registration in database

	// Return default config
	config := &pb.AgentConfig{
		RateLimit:          100,
		BatchSize:          50,
		BatchTimeoutMs:     5000,
		HeartbeatIntervalS: 30,
		EnabledWatchers:    []string{"pods"},
	}

	return &pb.RegisterAgentResponse{
		Success:      true,
		Message:      "Agent registered successfully",
		Config:       config,
		RegisteredAt: timestamppb.New(time.Now()),
	}, nil
}

