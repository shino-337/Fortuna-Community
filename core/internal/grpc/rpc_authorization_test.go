package grpc

import (
	"context"
	"testing"

	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openScopedGRPCAuthorizationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Agent{}, &models.Pod{}))
	return db
}

func scopedGRPCTestPrincipal(clusterID, agentID string) agentidentity.Principal {
	return agentidentity.Principal{CredentialID: "cred-" + clusterID + "-" + agentID, ClusterID: clusterID, AgentID: agentID}
}

func scopedGRPCTestContext(principal agentidentity.Principal, md metadata.MD) context.Context {
	ctx := context.WithValue(context.Background(), grpcAgentPrincipalKey{}, principal)
	if md != nil {
		ctx = metadata.NewIncomingContext(ctx, md)
	}
	return ctx
}

func TestScopedGRPCControlRPCAuthorizationAndClusterBinding(t *testing.T) {
	db := openScopedGRPCAuthorizationDB(t)
	principal := scopedGRPCTestPrincipal("cluster-a", "agent-a")
	interceptor := grpcAgentUnaryAuthorizationInterceptor(db)

	called := false
	_, err := interceptor(
		scopedGRPCTestContext(principal, nil),
		&pb.RegisterAgentRequest{AgentId: "agent-b"},
		&ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_RegisterAgent_FullMethodName},
		func(context.Context, any) (any, error) {
			called = true
			return nil, nil
		},
	)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.False(t, called, "foreign agent claim reached RegisterAgent handler")

	respAny, err := interceptor(
		scopedGRPCTestContext(principal, nil),
		&pb.RegisterAgentRequest{AgentId: "agent-a", NodeName: "node-a"},
		&ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_RegisterAgent_FullMethodName},
		func(ctx context.Context, req any) (any, error) {
			r := req.(*pb.RegisterAgentRequest)
			require.NoError(t, db.Create(&models.Agent{AgentID: r.AgentId, NodeName: r.NodeName, ClusterID: ""}).Error)
			return &pb.RegisterAgentResponse{Success: true, ClusterId: "legacy-env"}, nil
		},
	)
	require.NoError(t, err)
	resp := respAny.(*pb.RegisterAgentResponse)
	require.Equal(t, "cluster-a", resp.ClusterId, "RegisterAgent response must use trusted principal cluster")

	var stored models.Agent
	require.NoError(t, db.Where("agent_id = ?", "agent-a").First(&stored).Error)
	require.Equal(t, "cluster-a", stored.ClusterID, "legacy empty cluster must be bound to trusted principal")

	foreignPrincipal := scopedGRPCTestPrincipal("cluster-b", "agent-a")
	called = false
	_, err = interceptor(
		scopedGRPCTestContext(foreignPrincipal, nil),
		&pb.PingRequest{AgentId: "agent-a"},
		&ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_Ping_FullMethodName},
		func(context.Context, any) (any, error) {
			called = true
			return &pb.PingResponse{Status: "healthy"}, nil
		},
	)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.False(t, called, "foreign cluster credential reached Ping handler for existing agent")

	_, err = interceptor(
		scopedGRPCTestContext(principal, nil),
		&pb.HeartbeatRequest{AgentId: "agent-b"},
		&ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_Heartbeat_FullMethodName},
		func(context.Context, any) (any, error) {
			t.Fatal("foreign heartbeat claim reached handler")
			return nil, nil
		},
	)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestScopedGRPCPodRPCRejectsForeignClaimsAndCanonicalizesCluster(t *testing.T) {
	db := openScopedGRPCAuthorizationDB(t)
	require.NoError(t, db.Create(&models.Pod{ClusterID: "cluster-a", UID: "pod-a", Name: "app", Namespace: "ns-a", ServiceAccount: "default"}).Error)
	require.NoError(t, db.Create(&models.Pod{ClusterID: "cluster-b", UID: "pod-b", Name: "app", Namespace: "ns-b", ServiceAccount: "default"}).Error)

	principal := scopedGRPCTestPrincipal("cluster-a", "agent-a")
	interceptor := grpcAgentUnaryAuthorizationInterceptor(db)
	valid := &pb.SBOMFinding{AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a"}

	called := false
	_, err := interceptor(
		scopedGRPCTestContext(principal, metadata.Pairs("x-cluster-id", "cluster-b")),
		valid,
		&ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_SendSBOMFinding_FullMethodName},
		func(context.Context, any) (any, error) {
			called = true
			return nil, nil
		},
	)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.False(t, called, "forged x-cluster-id reached handler")

	foreignPod := &pb.SBOMFinding{AgentId: "agent-a", PodUid: "pod-b", PodName: "app", Namespace: "ns-b"}
	_, err = interceptor(
		scopedGRPCTestContext(principal, nil),
		foreignPod,
		&ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_SendSBOMFinding_FullMethodName},
		func(context.Context, any) (any, error) {
			t.Fatal("foreign Pod reached handler")
			return nil, nil
		},
	)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	wrongAgent := &pb.SBOMFinding{AgentId: "agent-b", PodUid: "pod-a", PodName: "app", Namespace: "ns-a"}
	_, err = interceptor(
		scopedGRPCTestContext(principal, nil),
		wrongAgent,
		&ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_SendSBOMFinding_FullMethodName},
		func(context.Context, any) (any, error) {
			t.Fatal("foreign agent claim reached handler")
			return nil, nil
		},
	)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	called = false
	_, err = interceptor(
		scopedGRPCTestContext(principal, nil),
		valid,
		&ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_SendSBOMFinding_FullMethodName},
		func(ctx context.Context, req any) (any, error) {
			called = true
			md, ok := metadata.FromIncomingContext(ctx)
			require.True(t, ok)
			require.Equal(t, []string{"cluster-a"}, md.Get("x-cluster-id"), "handler must receive trusted canonical cluster metadata")
			return &pb.SBOMFindingResponse{Success: true}, nil
		},
	)
	require.NoError(t, err)
	require.True(t, called)
}

func TestScopedGRPCCombinedFindingRequiresOneOwnedResource(t *testing.T) {
	db := openScopedGRPCAuthorizationDB(t)
	require.NoError(t, db.Create(&models.Pod{ClusterID: "cluster-a", UID: "pod-a", Name: "app", Namespace: "ns-a", ServiceAccount: "default"}).Error)
	principal := scopedGRPCTestPrincipal("cluster-a", "agent-a")
	interceptor := grpcAgentUnaryAuthorizationInterceptor(db)

	bad := &pb.CombinedFinding{
		Sbom: &pb.SBOMFinding{AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a", ContainerName: "frontend", ImageDigest: "sha256:a"},
		Cve:  &pb.CVEFinding{AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a", ContainerName: "frontend", ImageDigest: "sha256:b"},
	}
	_, err := interceptor(
		scopedGRPCTestContext(principal, nil), bad,
		&ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_SendCombinedFinding_FullMethodName},
		func(context.Context, any) (any, error) {
			t.Fatal("mixed-resource CombinedFinding reached handler")
			return nil, nil
		},
	)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	good := &pb.CombinedFinding{
		Sbom: &pb.SBOMFinding{AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a", ContainerName: "frontend", ImageDigest: "sha256:a"},
		Cve:  &pb.CVEFinding{AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a", ContainerName: "frontend", ImageDigest: "sha256:a"},
	}
	called := false
	_, err = interceptor(
		scopedGRPCTestContext(principal, nil), good,
		&ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_SendCombinedFinding_FullMethodName},
		func(context.Context, any) (any, error) {
			called = true
			return &pb.CombinedFindingResponse{Success: true}, nil
		},
	)
	require.NoError(t, err)
	require.True(t, called)
}

type scopedGRPCMessageStream struct {
	ctx       context.Context
	messages  []*pb.SBOMFinding
	recvCalls int
}

func (s *scopedGRPCMessageStream) SetHeader(metadata.MD) error  { return nil }
func (s *scopedGRPCMessageStream) SendHeader(metadata.MD) error { return nil }
func (s *scopedGRPCMessageStream) SetTrailer(metadata.MD)       {}
func (s *scopedGRPCMessageStream) Context() context.Context     { return s.ctx }
func (s *scopedGRPCMessageStream) SendMsg(any) error            { return nil }
func (s *scopedGRPCMessageStream) RecvMsg(m any) error {
	if s.recvCalls >= len(s.messages) {
		return nil
	}
	target, ok := m.(*pb.SBOMFinding)
	if !ok {
		return status.Error(codes.Internal, "unexpected test message type")
	}
	proto.Reset(target)
	proto.Merge(target, s.messages[s.recvCalls])
	s.recvCalls++
	return nil
}

func TestScopedGRPCBatchStreamRejectsForeignMessageBeforeHandler(t *testing.T) {
	db := openScopedGRPCAuthorizationDB(t)
	require.NoError(t, db.Create(&models.Pod{ClusterID: "cluster-a", UID: "pod-a", Name: "app-a", Namespace: "ns-a", ServiceAccount: "default"}).Error)
	require.NoError(t, db.Create(&models.Pod{ClusterID: "cluster-b", UID: "pod-b", Name: "app-b", Namespace: "ns-b", ServiceAccount: "default"}).Error)
	principal := scopedGRPCTestPrincipal("cluster-a", "agent-a")
	raw := &scopedGRPCMessageStream{
		ctx: scopedGRPCTestContext(principal, nil),
		messages: []*pb.SBOMFinding{
			{AgentId: "agent-a", PodUid: "pod-a", PodName: "app-a", Namespace: "ns-a"},
			{AgentId: "agent-a", PodUid: "pod-b", PodName: "app-b", Namespace: "ns-b"},
		},
	}
	interceptor := grpcAgentStreamAuthorizationInterceptor(db)
	processed := 0

	err := interceptor(nil, raw, &ggrpc.StreamServerInfo{FullMethod: pb.AgentService_BatchSendSBOMFindings_FullMethodName, IsClientStream: true}, func(srv any, stream ggrpc.ServerStream) error {
		first := &pb.SBOMFinding{}
		require.NoError(t, stream.RecvMsg(first))
		processed++
		md, ok := metadata.FromIncomingContext(stream.Context())
		require.True(t, ok)
		require.Equal(t, []string{"cluster-a"}, md.Get("x-cluster-id"))

		second := &pb.SBOMFinding{}
		err := stream.RecvMsg(second)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, processed, "foreign second message must not be accepted by handler")
	require.Equal(t, 2, raw.recvCalls, "authorization may inspect the foreign message but must block it before handler success")
}

func TestScopedGRPCOwnershipStorageFailureIsUnavailable(t *testing.T) {
	db := openScopedGRPCAuthorizationDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	principal := scopedGRPCTestPrincipal("cluster-a", "agent-a")
	interceptor := grpcAgentUnaryAuthorizationInterceptor(db)
	_, err = interceptor(
		scopedGRPCTestContext(principal, nil),
		&pb.SBOMFinding{AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a"},
		&ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_SendSBOMFinding_FullMethodName},
		func(context.Context, any) (any, error) {
			t.Fatal("storage failure reached handler")
			return nil, nil
		},
	)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestScopedGRPCUnknownMethodFailsClosed(t *testing.T) {
	principal := scopedGRPCTestPrincipal("cluster-a", "agent-a")
	interceptor := grpcAgentUnaryAuthorizationInterceptor(nil)
	_, err := interceptor(
		scopedGRPCTestContext(principal, nil), struct{}{},
		&ggrpc.UnaryServerInfo{FullMethod: "/fortuna.agent.v1.AgentService/FutureMutation"},
		func(context.Context, any) (any, error) {
			t.Fatal("unknown scoped method reached handler")
			return nil, nil
		},
	)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
