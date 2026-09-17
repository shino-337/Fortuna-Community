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

// TestScopedGRPCCombinedFindingRequiresOneOwnedResource preserves the original
// C3b regression while the stricter container/digest test extends it. A later
// hardening change must not silently delete the earlier mixed-resource boundary.
func TestScopedGRPCCombinedFindingRequiresOneOwnedResource(t *testing.T) {
	db := openScopedGRPCAuthorizationDB(t)
	require.NoError(t, db.Create(&models.Pod{
		ClusterID: "cluster-a", UID: "pod-a", Name: "app", Namespace: "ns-a", ServiceAccount: "default",
	}).Error)
	principal := scopedGRPCTestPrincipal("cluster-a", "agent-a")
	interceptor := grpcAgentUnaryAuthorizationInterceptor(db)

	bad := &pb.CombinedFinding{
		Sbom: &pb.SBOMFinding{
			AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a",
			ContainerName: "frontend", ImageDigest: "sha256:a",
		},
		Cve: &pb.CVEFinding{
			AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a",
			ContainerName: "frontend", ImageDigest: "sha256:b",
		},
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
		Sbom: &pb.SBOMFinding{
			AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a",
			ContainerName: "frontend", ImageDigest: "sha256:a",
		},
		Cve: &pb.CVEFinding{
			AgentId: "agent-a", PodUid: "pod-a", PodName: "app", Namespace: "ns-a",
			ContainerName: "frontend", ImageDigest: "sha256:a",
		},
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
