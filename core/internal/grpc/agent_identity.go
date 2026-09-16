package grpc

import (
	"context"
	"errors"

	"github.com/fortuna/core/pkg/agentidentity"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type grpcAgentPrincipalKey struct{}

// grpcAgentPrincipalFromContext returns only the principal installed by the
// transport interceptor. Request metadata and protobuf fields never populate it.
func grpcAgentPrincipalFromContext(ctx context.Context) (agentidentity.Principal, bool) {
	principal, ok := ctx.Value(grpcAgentPrincipalKey{}).(agentidentity.Principal)
	if !ok || principal.CredentialID == "" || principal.ClusterID == "" || principal.AgentID == "" {
		return agentidentity.Principal{}, false
	}
	return principal, true
}

func grpcAgentAuthError(err error) error {
	if errors.Is(err, agentidentity.ErrUnavailable) {
		return status.Error(codes.Unavailable, "agent credential registry unavailable")
	}
	return status.Error(codes.Unauthenticated, "agent credential rejected")
}

// authenticateGRPCAgent derives identity only from the verified TLS transport
// state supplied by gRPC. agentidentity.Store rechecks certificate validity and
// reloads the registry on every call so revocation takes effect without restart.
func authenticateGRPCAgent(ctx context.Context, store agentidentity.Store) (agentidentity.Principal, error) {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil || p.AuthInfo == nil {
		return agentidentity.Principal{}, grpcAgentAuthError(agentidentity.ErrUnauthenticated)
	}

	var tlsInfo credentials.TLSInfo
	switch info := p.AuthInfo.(type) {
	case credentials.TLSInfo:
		tlsInfo = info
	case *credentials.TLSInfo:
		if info == nil {
			return agentidentity.Principal{}, grpcAgentAuthError(agentidentity.ErrUnauthenticated)
		}
		tlsInfo = *info
	default:
		return agentidentity.Principal{}, grpcAgentAuthError(agentidentity.ErrUnauthenticated)
	}

	principal, err := store.AuthenticateTLS(tlsInfo.State)
	if err != nil {
		return agentidentity.Principal{}, grpcAgentAuthError(err)
	}
	return principal, nil
}

func grpcAgentUnaryAuthInterceptor(store agentidentity.Store) ggrpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *ggrpc.UnaryServerInfo, handler ggrpc.UnaryHandler) (any, error) {
		principal, err := authenticateGRPCAgent(ctx, store)
		if err != nil {
			return nil, err
		}
		return handler(context.WithValue(ctx, grpcAgentPrincipalKey{}, principal), req)
	}
}

type grpcAgentAuthenticatedStream struct {
	ggrpc.ServerStream
	store     agentidentity.Store
	principal agentidentity.Principal
}

func (s *grpcAgentAuthenticatedStream) Context() context.Context {
	return context.WithValue(s.ServerStream.Context(), grpcAgentPrincipalKey{}, s.principal)
}

// RecvMsg reauthenticates before every received stream message. A registry
// revocation/expiry or certificate expiry therefore terminates an established
// client stream before the next message reaches the handler.
func (s *grpcAgentAuthenticatedStream) RecvMsg(m any) error {
	principal, err := authenticateGRPCAgent(s.ServerStream.Context(), s.store)
	if err != nil {
		return err
	}
	if principal != s.principal {
		return status.Error(codes.Unauthenticated, "agent credential identity changed during stream")
	}
	return s.ServerStream.RecvMsg(m)
}

func grpcAgentStreamAuthInterceptor(store agentidentity.Store) ggrpc.StreamServerInterceptor {
	return func(srv any, stream ggrpc.ServerStream, info *ggrpc.StreamServerInfo, handler ggrpc.StreamHandler) error {
		principal, err := authenticateGRPCAgent(stream.Context(), store)
		if err != nil {
			return err
		}
		wrapped := &grpcAgentAuthenticatedStream{
			ServerStream: stream,
			store:        store,
			principal:    principal,
		}
		return handler(srv, wrapped)
	}
}
