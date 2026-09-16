package grpc

import (
	"context"
	"errors"
	"strings"

	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/models"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
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

// authorizeScopedGRPCAgentRecord prevents a scoped credential from updating an
// existing Agent row owned by another cluster. Empty ClusterID is accepted only
// as a legacy migration state and is bound after a successful control RPC.
func authorizeScopedGRPCAgentRecord(db *gorm.DB, principal agentidentity.Principal) error {
	if db == nil {
		return nil
	}
	var agent models.Agent
	err := db.Where("agent_id = ?", principal.AgentID).First(&agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return scopedGRPCUnavailable()
	}
	if agent.ClusterID != "" && agent.ClusterID != principal.ClusterID {
		return scopedGRPCPermissionDenied()
	}
	return nil
}

func bindScopedGRPCAgentRecord(db *gorm.DB, principal agentidentity.Principal) error {
	if db == nil {
		return nil
	}
	res := db.Model(&models.Agent{}).
		Where("agent_id = ? AND (cluster_id = '' OR cluster_id = ?)", principal.AgentID, principal.ClusterID).
		Update("cluster_id", principal.ClusterID)
	if res.Error != nil {
		return scopedGRPCUnavailable()
	}
	if res.RowsAffected == 0 {
		var count int64
		if err := db.Model(&models.Agent{}).
			Where("agent_id = ? AND cluster_id = ?", principal.AgentID, principal.ClusterID).
			Count(&count).Error; err != nil {
			return scopedGRPCUnavailable()
		}
		if count == 0 {
			return scopedGRPCPermissionDenied()
		}
	}
	return nil
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
	if req.Cve.PodUid != req.Sbom.PodUid ||
		(req.Cve.Namespace != "" && req.Cve.Namespace != req.Sbom.Namespace) ||
		(req.Cve.ImageDigest != "" && req.Sbom.ImageDigest != "" && req.Cve.ImageDigest != req.Sbom.ImageDigest) {
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
		resp, err := handler(trustedCtx, req)
		if err != nil {
			return resp, err
		}

		switch info.FullMethod {
		case pb.AgentService_RegisterAgent_FullMethodName, pb.AgentService_Ping_FullMethodName:
			if err := bindScopedGRPCAgentRecord(db, principal); err != nil {
				return nil, err
			}
		}
		if info.FullMethod == pb.AgentService_RegisterAgent_FullMethodName {
			if registerResp, ok := resp.(*pb.RegisterAgentResponse); ok && registerResp != nil {
				registerResp.ClusterId = principal.ClusterID
			}
		}
		return resp, nil
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
