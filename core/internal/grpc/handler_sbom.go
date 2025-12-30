package grpc

import (
	"context"
	"encoding/json"
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
	// Initialize all JSONB fields properly to avoid PostgreSQL errors
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
		PackageCount:  len(req.Packages),
		SBOMFormat:    "fortuna-agent",
		SBOMContent:   "{}", // Initialize as empty JSON object string for jsonb column
		Labels:        make(map[string]string), // Initialize empty map to avoid JSONB serialization error
		Annotations:   make(map[string]string), // Initialize empty map to avoid JSONB serialization error
		LastUsedAt:    time.Now(),
		UseCount:      1,
	}

	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Insert or update SBOM (UPSERT) - handle duplicate image_digest gracefully
	// Use ON CONFLICT to update LastUsedAt and UseCount if SBOM already exists
	var existingSBOM models.SBOM
	isNewSBOM := false
	err := tx.Where("image_digest = ? AND deleted_at IS NULL", sbom.ImageDigest).First(&existingSBOM).Error
	
	if err == nil {
		// SBOM already exists - update LastUsedAt and increment UseCount
		sbom.ID = existingSBOM.ID
		sbom.UseCount = existingSBOM.UseCount + 1
		sbom.LastUsedAt = time.Now()
		if err := tx.Model(&existingSBOM).Updates(map[string]interface{}{
			"last_used_at": sbom.LastUsedAt,
			"use_count":    sbom.UseCount,
			"pod_uid":      sbom.PodUID,
			"pod_name":     sbom.PodName,
			"namespace":    sbom.Namespace,
			"container_name": sbom.ContainerName,
		}).Error; err != nil {
			tx.Rollback()
			log.Printf("[SBOM] Failed to update existing SBOM: %v", err)
			return nil, status.Errorf(codes.Internal, "failed to update SBOM: %v", err)
		}
		log.Printf("[SBOM] Updated existing SBOM id=%d (use_count=%d)", existingSBOM.ID, sbom.UseCount)
		isNewSBOM = false
	} else if err == gorm.ErrRecordNotFound {
		// SBOM doesn't exist - create new one
		if err := tx.Create(sbom).Error; err != nil {
			tx.Rollback()
			log.Printf("[SBOM] Failed to insert SBOM: %v", err)
			return nil, status.Errorf(codes.Internal, "failed to insert SBOM: %v", err)
		}
		log.Printf("[SBOM] Created new SBOM id=%d", sbom.ID)
		isNewSBOM = true
	} else {
		// Database error
		tx.Rollback()
		log.Printf("[SBOM] Database error checking SBOM: %v", err)
		return nil, status.Errorf(codes.Internal, "database error: %v", err)
	}

		// Insert SBOM components (skip if already exist for this SBOM)
		// For existing SBOMs, components should already exist, so we skip insertion
		// Only insert components for new SBOMs
		if isNewSBOM {
			// This is a new SBOM - insert all components
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
				// Use FirstOrCreate to handle duplicates gracefully
				if err := tx.Where("sbom_id = ? AND purl = ?", sbom.ID, purl).FirstOrCreate(component).Error; err != nil {
					tx.Rollback()
					log.Printf("[SBOM] Failed to insert component %s: %v", pkg.Name, err)
					return nil, status.Errorf(codes.Internal, "failed to insert component: %v", err)
				}
			}
		} else {
			// Existing SBOM - components already exist, skip insertion
			log.Printf("[SBOM] Skipping component insertion for existing SBOM id=%d", sbom.ID)
		}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("[SBOM] Failed to commit transaction: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to commit: %v", err)
	}

		// Publish SBOM_CREATED event to NATS (for CVE matching worker)
		// Use subject 'fortuna.sbom.created' to match stream pattern 'fortuna.sbom.>' in 'fortuna-events' stream
		// Include all required fields for worker to create insights with new schema
		// IMPORTANT: Use PodUID from request (req.PodUid), not from SBOM record (sbom.PodUID)
		// This ensures insights are created for the CURRENT pod, not the pod that first created the SBOM
		if s.natsClient != nil {
			// Use PodUID from request to ensure insights are created for the current pod
			// even when SBOM is reused (same image_digest)
			podUID := req.PodUid
			podName := req.PodName
			podNamespace := req.Namespace
			containerName := req.ContainerName
			
			// Create proper JSON event with all required fields using map to avoid import issues
			event := map[string]interface{}{
				"type":           "sbom.created",
				"timestamp":      time.Now().Unix(),
				"cluster_id":     "default", // TODO: Get from config
				"pod_uid":        podUID,    // Use from request, not from SBOM record
				"pod_name":       podName,   // Use from request, not from SBOM record
				"pod_namespace":  podNamespace, // Use from request, not from SBOM record
				"container_name": containerName, // Use from request, not from SBOM record
				"container_image": fmt.Sprintf("%s:%s", sbom.ImageName, sbom.ImageTag),
				"sbom_id":        sbom.ID,
				"image_digest":   sbom.ImageDigest,
			}
			eventJSON, err := json.Marshal(event)
			if err != nil {
				log.Printf("[SBOM] WARNING: Failed to marshal SBOM_CREATED event: %v", err)
			} else {
				if err := s.natsClient.Publish("fortuna.sbom.created", eventJSON); err != nil {
					log.Printf("[SBOM] WARNING: Failed to publish SBOM_CREATED event: %v", err)
					// Non-fatal, continue
				} else {
					log.Printf("[SBOM] Published SBOM_CREATED event for sbom_id=%d (pod_uid=%s, reused=%v)", 
						sbom.ID, podUID, !isNewSBOM)
				}
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

