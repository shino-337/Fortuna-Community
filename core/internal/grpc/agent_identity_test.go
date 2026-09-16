package grpc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/pkg/agentidentity"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func testGRPCClientCertificate(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func writeGRPCCredentialRegistry(t *testing.T, cert *x509.Certificate, revoked bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "grpc-agent-credentials.json")
	writeGRPCCredentialRegistryAt(t, path, cert, revoked)
	return path
}

func writeGRPCCredentialRegistryAt(t *testing.T, path string, cert *x509.Certificate, revoked bool) {
	t.Helper()
	digest := sha256.Sum256(cert.Raw)
	registry := map[string]any{
		"credentials": []map[string]any{{
			"id":                 "grpc-cert-a",
			"cluster_id":         "cluster-a",
			"agent_id":           "agent-a",
			"certificate_sha256": hex.EncodeToString(digest[:]),
			"expires_at":         time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
			"revoked":            revoked,
		}},
	}
	data, err := json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func grpcTLSContext(cert *x509.Certificate, verified bool) context.Context {
	state := tls.ConnectionState{
		HandshakeComplete: true,
		PeerCertificates:  []*x509.Certificate{cert},
	}
	if verified {
		state.VerifiedChains = [][]*x509.Certificate{{cert}}
	}
	info := credentials.TLSInfo{State: state}
	return peer.NewContext(context.Background(), &peer.Peer{AuthInfo: info})
}

func TestGRPCAgentUnaryInterceptorAuthenticatesVerifiedCertificate(t *testing.T) {
	cert := testGRPCClientCertificate(t)
	store := agentidentity.Store{Path: writeGRPCCredentialRegistry(t, cert, false)}
	interceptor := grpcAgentUnaryAuthInterceptor(store)
	called := false

	_, err := interceptor(grpcTLSContext(cert, true), struct{}{}, &ggrpc.UnaryServerInfo{FullMethod: "/fortuna.agent.v1.AgentService/Ping"}, func(ctx context.Context, req any) (any, error) {
		called = true
		principal, ok := grpcAgentPrincipalFromContext(ctx)
		if !ok {
			t.Fatal("trusted gRPC principal missing from handler context")
		}
		if principal.ClusterID != "cluster-a" || principal.AgentID != "agent-a" || principal.CredentialID != "grpc-cert-a" {
			t.Fatalf("unexpected principal: %+v", principal)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("verified registered certificate rejected: %v", err)
	}
	if !called {
		t.Fatal("authenticated unary request did not reach handler")
	}
}

func TestGRPCAgentUnaryInterceptorRejectsUnverifiedRevokedAndUnavailableIdentity(t *testing.T) {
	cert := testGRPCClientCertificate(t)
	path := writeGRPCCredentialRegistry(t, cert, false)
	store := agentidentity.Store{Path: path}
	interceptor := grpcAgentUnaryAuthInterceptor(store)
	handler := func(ctx context.Context, req any) (any, error) {
		t.Fatal("rejected request reached handler")
		return nil, nil
	}

	_, err := interceptor(grpcTLSContext(cert, false), struct{}{}, &ggrpc.UnaryServerInfo{}, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unverified certificate code=%v err=%v", status.Code(err), err)
	}

	writeGRPCCredentialRegistryAt(t, path, cert, true)
	_, err = interceptor(grpcTLSContext(cert, true), struct{}{}, &ggrpc.UnaryServerInfo{}, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("revoked certificate code=%v err=%v", status.Code(err), err)
	}

	if err := os.WriteFile(path, []byte(`{"credentials":[{"broken":true}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = interceptor(grpcTLSContext(cert, true), struct{}{}, &ggrpc.UnaryServerInfo{}, handler)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("invalid registry code=%v err=%v", status.Code(err), err)
	}
}

type testServerStream struct {
	ctx       context.Context
	recvCalls int
}

func (s *testServerStream) SetHeader(metadata.MD) error  { return nil }
func (s *testServerStream) SendHeader(metadata.MD) error { return nil }
func (s *testServerStream) SetTrailer(metadata.MD)       {}
func (s *testServerStream) Context() context.Context     { return s.ctx }
func (s *testServerStream) SendMsg(any) error            { return nil }
func (s *testServerStream) RecvMsg(any) error {
	s.recvCalls++
	return nil
}

func TestGRPCAgentStreamReauthenticatesEveryReceivedMessage(t *testing.T) {
	cert := testGRPCClientCertificate(t)
	path := writeGRPCCredentialRegistry(t, cert, false)
	store := agentidentity.Store{Path: path}
	interceptor := grpcAgentStreamAuthInterceptor(store)
	raw := &testServerStream{ctx: grpcTLSContext(cert, true)}

	err := interceptor(nil, raw, &ggrpc.StreamServerInfo{FullMethod: "/fortuna.agent.v1.AgentService/BatchSendSBOMFindings", IsClientStream: true}, func(srv any, stream ggrpc.ServerStream) error {
		principal, ok := grpcAgentPrincipalFromContext(stream.Context())
		if !ok || principal.AgentID != "agent-a" {
			t.Fatalf("stream handler missing authenticated principal: %+v ok=%v", principal, ok)
		}
		if err := stream.RecvMsg(&struct{}{}); err != nil {
			return err
		}
		writeGRPCCredentialRegistryAt(t, path, cert, true)
		err := stream.RecvMsg(&struct{}{})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("revocation between stream messages code=%v err=%v", status.Code(err), err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream interceptor returned unexpected error: %v", err)
	}
	if raw.recvCalls != 1 {
		t.Fatalf("revoked second message reached underlying stream: recvCalls=%d", raw.recvCalls)
	}
}

func TestNewServerRejectsScopedGRPCIdentityWithoutTLS(t *testing.T) {
	_, err := NewServer(&config.Config{
		TLSEnabled:                     false,
		GRPCAgentCredentialRegistryPath: "/tmp/grpc-agent-registry.json",
	}, nil, nil, nil)
	if err == nil {
		t.Fatal("scoped gRPC identity without TLS must fail server creation")
	}
}
