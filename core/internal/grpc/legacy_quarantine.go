package grpc

import (
	"context"
	pb "github.com/fortuna/api/proto/agent"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func quarantinedLegacyUnaryInterceptor(ctx context.Context, req any, info *ggrpc.UnaryServerInfo, handler ggrpc.UnaryHandler) (any, error) {
	if info.FullMethod == pb.AgentService_Ping_FullMethodName {
		return &pb.PingResponse{Status: "identity_required", Version: "1.0.0"}, nil
	}
	return nil, status.Error(codes.Unauthenticated, "AgentService writes require scoped mTLS credentials")
}
func quarantinedLegacyStreamInterceptor(srv any, stream ggrpc.ServerStream, info *ggrpc.StreamServerInfo, handler ggrpc.StreamHandler) error {
	return status.Error(codes.Unauthenticated, "AgentService streams require scoped mTLS credentials")
}
