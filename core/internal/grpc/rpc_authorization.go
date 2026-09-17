package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/models"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func scopedGRPCPermissionDenied() error {
	return status.Error(codes.PermissionDenied, "agent resource scope mismatch")
}

func scopedGRPCUnavailable() error {
	return status.Error(codes.Unavailable, "agent ownership state unavailable")
}

func trustedScopedGRPCContext(ctx context.Context, principal agentidentity.Principal) (context.Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	md = md.Copy()
	for _, claimed := range md.Get("x-cluster-id") {
		if claimed != "" && claimed != principal.ClusterID {
			return nil, scopedGRPCPermissionDenied()
		}
	}
	md.Set("x-cluster-id", principal.ClusterID)
	return metadata.NewIncomingContext(ctx, md), nil
}

func authorizeScopedGRPCAgentClaim(principal agentidentity.Principal, agentID string) error {
	if err := principal.CheckClaims(principal.ClusterID, agentID); err != nil {
		return scopedGRPCPermissionDenied()
	}
	return nil
}

func authorizeScopedGRPCPod(db *gorm.DB, principal agentidentity.Principal, podUID, namespace, podName string) error {
	if db == nil {
		return scopedGRPCUnavailable()
	}
	if strings.TrimSpace(podUID) == "" {
		return scopedGRPCPermissionDenied()
	}

	var pod models.Pod
	err := db.Where("uid = ? AND cluster_id = ? AND deleted_at IS NULL", podUID, principal.ClusterID).First(&pod).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return scopedGRPCPermissionDenied()
	}
	if err != nil {
		return scopedGRPCUnavailable()
	}
	if namespace != "" && namespace != pod.Namespace {
		return scopedGRPCPermissionDenied()
	}
	if podName != "" && podName != pod.Name {
		return scopedGRPCPermissionDenied()
	}
	return nil
}

// authorizeScopedGRPCAgentRecord verifies that the ownership store is available.
// AgentID is not globally unique: exact ownership is the pair {cluster_id,agent_id}.
// A legacy empty-cluster row may be claimed by the authenticated principal during
// the scoped control-RPC write path; a same AgentID in another cluster is valid.
func authorizeScopedGRPCAgentRecord(db *gorm.DB, principal agentidentity.Principal) error {
	if db == nil {
		return scopedGRPCUnavailable()
	}
	var agent models.Agent
	err := db.Unscoped().Where(
		"agent_id = ? AND (cluster_id = ? OR cluster_id = '')",
		principal.AgentID, principal.ClusterID,
	).First(&agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return scopedGRPCUnavailable()
	}
	return nil
}

// upsertScopedAgentRecord persists control-RPC state using the canonical
// {cluster_id,agent_id} key. Scoped mode deliberately bypasses legacy handlers
// that still use agent_id alone. Legacy empty-cluster rows are claimed atomically
// before a new composite row is created.
func upsertScopedAgentRecord(db *gorm.DB, principal agentidentity.Principal, nodeName, version, statusValue string, capabilities []string) error {
	if db == nil {
		return scopedGRPCUnavailable()
	}
	now := time.Now()
	return db.Transaction(func(tx *gorm.DB) error {
		var legacy models.Agent
		legacyErr := tx.Unscoped().Where("agent_id = ? AND cluster_id = ''", principal.AgentID).First(&legacy).Error
		if legacyErr != nil && !errors.Is(legacyErr, gorm.ErrRecordNotFound) {
			return scopedGRPCUnavailable()
		}
		if legacyErr == nil {
			updates := map[string]interface{}{
				"cluster_id":   principal.ClusterID,
				"status":       statusValue,
				"last_seen_at": now,
				"deleted_at":   nil,
				"updated_at":   now,
			}
			if nodeName != "" {
				updates["node_name"] = nodeName
			}
			if version != "" {
				updates["version"] = version
			}
			if capabilities != nil {
				encoded, err := json.Marshal(capabilities)
				if err != nil {
					return status.Error(codes.InvalidArgument, "invalid agent capabilities")
				}
				updates["capabilities"] = string(encoded)
			}
			res := tx.Model(&models.Agent{}).
				Unscoped().
				Where("id = ? AND cluster_id = ''", legacy.ID).
				Updates(updates)
			if res.Error != nil {
				return scopedGRPCUnavailable()
			}
			if res.RowsAffected == 1 {
				return nil
			}
			// Another concurrent claimant moved the legacy row. Continue with the
			// authenticated cluster's independent composite identity.
		}

		agent := models.Agent{
			ClusterID:  principal.ClusterID,
			AgentID:    principal.AgentID,
			NodeName:   nodeName,
			Version:    version,
			Status:     statusValue,
			LastSeenAt: &now,
		}
		if capabilities != nil {
			encoded, err := json.Marshal(capabilities)
			if err != nil {
				return status.Error(codes.InvalidArgument, "invalid agent capabilities")
			}
			agent.Capabilities = string(encoded)
		}

		updates := map[string]interface{}{
			"status":       statusValue,
			"last_seen_at": now,
			"deleted_at":   nil,
			"updated_at":   now,
		}
		if nodeName != "" {
			updates["node_name"] = nodeName
		}
		if version != "" {
			updates["version"] = version
		}
		if capabilities != nil {
			updates["capabilities"] = agent.Capabilities
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "cluster_id"}, {Name: "agent_id"}},
			DoUpdates: clause.Assignments(updates),
		}).Create(&agent).Error; err != nil {
			return scopedGRPCUnavailable()
		}
		return nil
	})
}

func authorizeScopedGRPCSBOM(db *gorm.DB, principal agentidentity.Principal, req *pb.SBOMFinding) error {
	if req == nil {
		return scopedGRPCPermissionDenied()
	}
	if err := authorizeScopedGRPCAgentClaim(principal, req.AgentId); err != nil {
		return err
	}
	return authorizeScopedGRPCPod(db, principal, req.PodUid, req.Namespace, req.PodName)
}

func authorizeScopedGRPCCVE(db *gorm.DB, principal agentidentity.Principal, req *pb.CVEFinding) error {
	if req == nil {
		return scopedGRPCPermissionDenied()
	}
	if err := authorizeScopedGRPCAgentClaim(principal, req.AgentId); err != nil {
		return err
	}
	return authorizeScopedGRPCPod(db, principal, req.PodUid, req.Namespace, req.PodName)
}

func authorizeScopedGRPCCombined(db *gorm.DB, principal agentidentity.Principal, req *pb.CombinedFinding) error {
	if req == nil || req.Sbom == nil {
		return scopedGRPCPermissionDenied()
	}
	if err := authorizeScopedGRPCSBOM(db, principal, req.Sbom); err != nil {
		return err
	}
	if req.Cve == nil {
		return nil
	}
	if err := authorizeScopedGRPCCVE(db, principal, req.Cve); err != nil {
		return err
	}
	// CVE evidence carried in CombinedFinding must describe exactly the same
	// workload image as the SBOM. Allowing an empty digest/container here would
	// let evidence for another container in the same Pod be linked to this SBOM.
	if req.Cve.PodUid != req.Sbom.PodUid ||
		(req.Cve.Namespace != "" && req.Cve.Namespace != req.Sbom.Namespace) ||
		strings.TrimSpace(req.Cve.ContainerName) == "" ||
		strings.TrimSpace(req.Sbom.ContainerName) == "" ||
		req.Cve.ContainerName != req.Sbom.ContainerName ||
		strings.TrimSpace(req.Cve.ImageDigest) == "" ||
		strings.TrimSpace(req.Sbom.ImageDigest) == "" ||
		req.Cve.ImageDigest != req.Sbom.ImageDigest {
		return scopedGRPCPermissionDenied()
	}
	return nil
}

func authorizeScopedGRPCUnaryRequest(ctx context.Context, db *gorm.DB, fullMethod string, req any) (context.Context, agentidentity.Principal, error) {
	principal, ok := grpcAgentPrincipalFromContext(ctx)
	if !ok {
		return nil, agentidentity.Principal{}, status.Error(codes.Unauthenticated, "trusted agent principal missing")
	}
	trustedCtx, err := trustedScopedGRPCContext(ctx, principal)
	if err != nil {
		return nil, agentidentity.Principal{}, err
	}

	switch fullMethod {
	case pb.AgentService_RegisterAgent_FullMethodName:
		r, ok := req.(*pb.RegisterAgentRequest)
		if !ok || r == nil {
			return nil, agentidentity.Principal{}, scopedGRPCPermissionDenied()
		}
		if err := authorizeScopedGRPCAgentClaim(principal, r.AgentId); err != nil {
			return nil, agentidentity.Principal{}, err
		}
		if err := authorizeScopedGRPCAgentRecord(db, principal); err != nil {
			return nil, agentidentity.Principal{}, err
		}
	case pb.AgentService_Ping_FullMethodName:
		r, ok := req.(*pb.PingRequest)
		if !ok || r == nil {
			return nil, agentidentity.Principal{}, scopedGRPCPermissionDenied()
		}
		if err := authorizeScopedGRPCAgentClaim(principal, r.AgentId); err != nil {
			return nil, agentidentity.Principal{}, err
		}
		if err := authorizeScopedGRPCAgentRecord(db, principal); err != nil {
			return nil, agentidentity.Principal{}, err
		}
	case pb.AgentService_Heartbeat_FullMethodName:
		r, ok := req.(*pb.HeartbeatRequest)
		if !ok || r == nil {
			return nil, agentidentity.Principal{}, scopedGRPCPermissionDenied()
		}
		if err := authorizeScopedGRPCAgentClaim(principal, r.AgentId); err != nil {
			return nil, agentidentity.Principal{}, err
		}
		if err := authorizeScopedGRPCAgentRecord(db, principal); err != nil {
			return nil, agentidentity.Principal{}, err
		}
	case pb.AgentService_SendSBOMFinding_FullMethodName:
		r, ok := req.(*pb.SBOMFinding)
		if !ok {
			return nil, agentidentity.Principal{}, scopedGRPCPermissionDenied()
		}
		if err := authorizeScopedGRPCSBOM(db, principal, r); err != nil {
			return nil, agentidentity.Principal{}, err
		}
	case pb.AgentService_SendCVEFinding_FullMethodName:
		r, ok := req.(*pb.CVEFinding)
		if !ok {
			return nil, agentidentity.Principal{}, scopedGRPCPermissionDenied()
		}
		if err := authorizeScopedGRPCCVE(db, principal, r); err != nil {
			return nil, agentidentity.Principal{}, err
		}
	case pb.AgentService_SendCombinedFinding_FullMethodName:
		r, ok := req.(*pb.CombinedFinding)
		if !ok {
			return nil, agentidentity.Principal{}, scopedGRPCPermissionDenied()
		}
		if err := authorizeScopedGRPCCombined(db, principal, r); err != nil {
			return nil, agentidentity.Principal{}, err
		}
	default:
		return nil, agentidentity.Principal{}, status.Error(codes.PermissionDenied, "scoped gRPC method authorization missing")
	}
	return trustedCtx, principal, nil
}

func grpcAgentUnaryAuthorizationInterceptor(db *gorm.DB) ggrpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *ggrpc.UnaryServerInfo, handler ggrpc.UnaryHandler) (any, error) {
		trustedCtx, principal, err := authorizeScopedGRPCUnaryRequest(ctx, db, info.FullMethod, req)
		if err != nil {
			return nil, err
		}

		// Scoped control RPCs use a cluster-qualified persistence path and do not
		// call legacy handlers that still key Agent rows by agent_id alone.
		switch info.FullMethod {
		case pb.AgentService_RegisterAgent_FullMethodName:
			r := req.(*pb.RegisterAgentRequest)
			if err := upsertScopedAgentRecord(db, principal, r.NodeName, r.Version, "ready", r.Capabilities); err != nil {
				return nil, err
			}
			return &pb.RegisterAgentResponse{Success: true, Message: "Agent registered successfully", ClusterId: principal.ClusterID}, nil
		case pb.AgentService_Ping_FullMethodName:
			r := req.(*pb.PingRequest)
			if err := upsertScopedAgentRecord(db, principal, r.NodeName, "", "ready", nil); err != nil {
				return nil, err
			}
			return &pb.PingResponse{Status: "healthy", Version: "1.0.0"}, nil
		case pb.AgentService_Heartbeat_FullMethodName:
			r := req.(*pb.HeartbeatRequest)
			statusValue := strings.TrimSpace(r.Status)
			if statusValue == "" {
				statusValue = "ready"
			}
			if err := upsertScopedAgentRecord(db, principal, "", "", statusValue, nil); err != nil {
				return nil, err
			}
			return &pb.HeartbeatResponse{Success: true, Message: "heartbeat received"}, nil
		}

		return handler(trustedCtx, req)
	}
}

type grpcAgentAuthorizationStream struct {
	ggrpc.ServerStream
	db        *gorm.DB
	principal agentidentity.Principal
	ctx       context.Context
}

func (s *grpcAgentAuthorizationStream) Context() context.Context { return s.ctx }

func (s *grpcAgentAuthorizationStream) RecvMsg(m any) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	switch req := m.(type) {
	case *pb.SBOMFinding:
		if err := authorizeScopedGRPCSBOM(s.db, s.principal, req); err != nil {
			return err
		}
		return nil
	default:
		return status.Error(codes.PermissionDenied, "scoped gRPC stream message authorization missing")
	}
}

func grpcAgentStreamAuthorizationInterceptor(db *gorm.DB) ggrpc.StreamServerInterceptor {
	return func(srv any, stream ggrpc.ServerStream, info *ggrpc.StreamServerInfo, handler ggrpc.StreamHandler) error {
		if info.FullMethod != pb.AgentService_BatchSendSBOMFindings_FullMethodName {
			return status.Error(codes.PermissionDenied, "scoped gRPC stream method authorization missing")
		}
		principal, ok := grpcAgentPrincipalFromContext(stream.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "trusted agent principal missing")
		}
		trustedCtx, err := trustedScopedGRPCContext(stream.Context(), principal)
		if err != nil {
			return err
		}
		wrapped := &grpcAgentAuthorizationStream{
			ServerStream: stream,
			db:           db,
			principal:    principal,
			ctx:          trustedCtx,
		}
		return handler(srv, wrapped)
	}
}
