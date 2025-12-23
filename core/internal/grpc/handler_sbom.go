package grpc

import (
	"context"
	"fmt"
	"io"
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
func (s *SBOMServiceServer) SendSBOMFinding(ctx context.Context, req *pb.SBOMFinding) (*pb.SBOMFindingResponse, error) {
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
			// Generate PURL
			purl := fmt.Sprintf("pkg:%s/%s@%s", pkg.Type.String(), pkg.Name, pkg.Version)
			
			component := &models.SBOMComponent{
				SBOMID:           sbom.ID,
				ComponentType:    mapComponentType(pkg.Type),
				ComponentName:    pkg.Name,
				ComponentVersion: pkg.Version,
				PURL:             purl,
				Licenses:         models.ToJSONBString(pkg.Licenses),
				Source:           pkg.Source,
				Description:      pkg.Description,
				Homepage:         pkg.Homepage,
				Maintainer:       pkg.Maintainer,
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

		// Publish SBOM_CREATED event to NATS (for CVE matching worker)
		if s.natsClient != nil {
			eventData := fmt.Sprintf(`{"sbom_id":%d,"pod_uid":"%s","image_digest":"%s","package_count":%d}`, 
				sbom.ID, sbom.PodUID, sbom.ImageDigest, len(req.Packages))
			if err := s.natsClient.Publish("fortuna.sbom.created", []byte(eventData)); err != nil {
				log.Printf("[SBOM] WARNING: Failed to publish SBOM_CREATED event: %v", err)
				// Non-fatal, continue
			} else {
				log.Printf("[SBOM] Published SBOM_CREATED event for sbom_id=%d", sbom.ID)
			}
		}

	log.Printf("[SBOM] Successfully stored SBOM id=%d with %d components", sbom.ID, len(req.Packages))

	return &pb.SBOMFindingResponse{
		Success:    true,
		Message:    "SBOM received and stored",
		SbomId:     fmt.Sprintf("%d", sbom.ID),
		ReceivedAt: timestamppb.New(time.Now()),
	}, nil
}

// BatchSendSBOMFindings handles multiple SBOM findings in a stream
// Note: Proto uses stream, not BatchSBOMRequest
func (s *SBOMServiceServer) BatchSendSBOMFindings(stream pb.AgentService_BatchSendSBOMFindingsServer) error {
	// Process stream of SBOM findings
	var successCount int32
	var totalCount int32
	
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		
		totalCount++
		
		// Process each SBOM finding
		_, err = s.SendSBOMFinding(stream.Context(), req)
		if err == nil {
			successCount++
		}
	}
	
	log.Printf("[SBOM] Batch processed: total=%d, success=%d", totalCount, successCount)
	
	resp := &pb.BatchSBOMFindingResponse{
		Success:        true,
		Message:        fmt.Sprintf("Processed %d/%d SBOM findings", successCount, totalCount),
		ReceivedCount:  totalCount,
		ProcessedCount: successCount,
	}
	
	return stream.SendAndClose(resp)
}

// Ping handles health check from agent
func (s *SBOMServiceServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{
		Status:  "healthy",
		Version: "1.0.0", // TODO: Get from build info
	}, nil
}

// RegisterAgent handles agent registration
func (s *SBOMServiceServer) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	log.Printf("[Agent] Register: id=%s, node=%s, version=%s, capabilities=%v",
		req.AgentId, req.NodeName, req.Version, req.Capabilities)

	// TODO: Store agent registration in database

	return &pb.RegisterAgentResponse{
		Success:   true,
		Message:   "Agent registered successfully",
		ClusterId: "default", // TODO: Get from config
	}, nil
}

// Helper function
func mapComponentType(t pb.PackageType) string {
	switch t {
	case pb.PackageType_PACKAGE_TYPE_DEB, pb.PackageType_PACKAGE_TYPE_RPM, pb.PackageType_PACKAGE_TYPE_APK:
		return "os-package"
	case pb.PackageType_PACKAGE_TYPE_NPM, pb.PackageType_PACKAGE_TYPE_PYPI, pb.PackageType_PACKAGE_TYPE_GEM,
		pb.PackageType_PACKAGE_TYPE_GO_MOD, pb.PackageType_PACKAGE_TYPE_MAVEN, pb.PackageType_PACKAGE_TYPE_CARGO:
		return "language-package"
	default:
		return "unknown"
	}
}

