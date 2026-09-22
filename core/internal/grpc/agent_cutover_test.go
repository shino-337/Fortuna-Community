package grpc

import (
	"context"
	"net"
	"testing"

	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestLegacyGRPCServerQuarantinesWrites(t *testing.T) {
	db := openScopedGRPCAuthorizationDB(t)
	server, err := NewServer(&config.Config{TLSEnabled: false}, db, nil, nil)
	require.NoError(t, err)
	listener := bufconn.Listen(1024 * 1024)
	go server.server.Serve(listener)
	defer server.Stop()
	defer listener.Close()
	conn, err := ggrpc.NewClient("passthrough:///buf", ggrpc.WithTransportCredentials(insecure.NewCredentials()), ggrpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	require.NoError(t, err)
	defer conn.Close()
	client := pb.NewAgentServiceClient(conn)
	_, err = client.RegisterAgent(context.Background(), &pb.RegisterAgentRequest{AgentId: "untrusted"})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.SendSBOMFinding(context.Background(), &pb.SBOMFinding{AgentId: "untrusted"})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	response, err := client.Ping(context.Background(), &pb.PingRequest{AgentId: "untrusted"})
	require.NoError(t, err)
	require.Equal(t, "identity_required", response.Status)
	stream, err := client.BatchSendSBOMFindings(context.Background())
	require.NoError(t, err)
	_, err = stream.CloseAndRecv()
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	var count int64
	require.NoError(t, db.Model(&models.Agent{}).Count(&count).Error)
	require.Zero(t, count)
}

type revokeWhileReceiving struct {
	*testServerStream
	revoke func()
}

func (s *revokeWhileReceiving) RecvMsg(any) error { s.revoke(); return nil }

func TestGRPCRevocationWhileReceiveBlocked(t *testing.T) {
	cert := testGRPCClientCertificate(t)
	path := writeGRPCCredentialRegistry(t, cert, false)
	raw := &revokeWhileReceiving{testServerStream: &testServerStream{ctx: grpcTLSContext(cert, true)}, revoke: func() { writeGRPCCredentialRegistryAt(t, path, cert, true) }}
	err := grpcAgentStreamAuthInterceptor(agentidentity.Store{Path: path})(nil, raw, &ggrpc.StreamServerInfo{}, func(_ any, stream ggrpc.ServerStream) error { return stream.RecvMsg(&struct{}{}) })
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
