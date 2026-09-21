package grpc

import (
	"context"
	"testing"

	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestScopedGRPCAgentRecordUnavailableFailsClosed(t *testing.T) {
	principal := scopedGRPCTestPrincipal("cluster-a", "agent-a")
	interceptor := grpcAgentUnaryAuthorizationInterceptor(nil)

	for _, tc := range []struct {
		name   string
		method string
		req    any
	}{
		{name: "register", method: pb.AgentService_RegisterAgent_FullMethodName, req: &pb.RegisterAgentRequest{AgentId: "agent-a"}},
		{name: "ping", method: pb.AgentService_Ping_FullMethodName, req: &pb.PingRequest{AgentId: "agent-a"}},
		{name: "heartbeat", method: pb.AgentService_Heartbeat_FullMethodName, req: &pb.HeartbeatRequest{AgentId: "agent-a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			_, err := interceptor(
				scopedGRPCTestContext(principal, nil),
				tc.req,
				&ggrpc.UnaryServerInfo{FullMethod: tc.method},
				func(context.Context, any) (any, error) {
					called = true
					return nil, nil
				},
			)
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.False(t, called, "ownership-store failure must stop before handler effects")
		})
	}
}

func TestScopedGRPCControlRPCSeparatesDuplicateAgentIDs(t *testing.T) {
	db := openScopedGRPCAuthorizationDB(t)
	svc := NewSBOMServiceServer(db, nil, nil)
	interceptor := grpcAgentUnaryAuthorizationInterceptor(db)
	for _, cluster := range []string{"cluster-a", "cluster-b"} {
		ctx := scopedGRPCTestContext(scopedGRPCTestPrincipal(cluster, "shared-agent"), nil)
		_, err := interceptor(ctx, &pb.RegisterAgentRequest{AgentId: "shared-agent", NodeName: cluster}, &ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_RegisterAgent_FullMethodName}, func(ctx context.Context, req any) (any, error) {
			return svc.RegisterAgent(ctx, req.(*pb.RegisterAgentRequest))
		})
		require.NoError(t, err)
	}
	var rows []models.Agent
	require.NoError(t, db.Order("cluster_id").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Equal(t, "cluster-a", rows[0].NodeName)
	require.Equal(t, "cluster-b", rows[1].NodeName)
}

func TestScopedGRPCCombinedFindingRequiresExactContainerAndDigest(t *testing.T) {
	db := openScopedGRPCAuthorizationDB(t)
	require.NoError(t, db.Create(&models.Pod{
		ClusterID: "cluster-a", UID: "pod-a", Name: "app", Namespace: "ns-a", ServiceAccount: "default",
	}).Error)
	principal := scopedGRPCTestPrincipal("cluster-a", "agent-a")
	interceptor := grpcAgentUnaryAuthorizationInterceptor(db)
	baseSBOM := &pb.SBOMFinding{
		AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a",
		ContainerName: "frontend", ImageDigest: "sha256:a",
	}

	for _, tc := range []struct {
		name string
		cve  *pb.CVEFinding
	}{
		{name: "digest mismatch", cve: &pb.CVEFinding{AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a", ContainerName: "frontend", ImageDigest: "sha256:b"}},
		{name: "container mismatch", cve: &pb.CVEFinding{AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a", ContainerName: "backend", ImageDigest: "sha256:a"}},
		{name: "missing digest", cve: &pb.CVEFinding{AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a", ContainerName: "frontend"}},
		{name: "missing container", cve: &pb.CVEFinding{AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a", ImageDigest: "sha256:a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := interceptor(
				scopedGRPCTestContext(principal, nil),
				&pb.CombinedFinding{Sbom: baseSBOM, Cve: tc.cve},
				&ggrpc.UnaryServerInfo{FullMethod: pb.AgentService_SendCombinedFinding_FullMethodName},
				func(context.Context, any) (any, error) {
					t.Fatal("mismatched CombinedFinding reached handler")
					return nil, nil
				},
			)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
		})
	}

	good := &pb.CombinedFinding{
		Sbom: baseSBOM,
		Cve: &pb.CVEFinding{
			AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a",
			ContainerName: "frontend", ImageDigest: "sha256:a",
		},
	}
	called := false
	_, err := interceptor(
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
