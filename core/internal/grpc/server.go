package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gorm.io/gorm"

	agentpb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/internal/ingest"
	"github.com/fortuna/core/pkg/messaging"
	"github.com/fortuna/core/pkg/security"
)

type Server struct {
	config      *config.Config
	db          *gorm.DB
	server      *grpc.Server
	certManager *security.CertManager
}

func NewServer(cfg *config.Config, db *gorm.DB, natsClient *messaging.NATSClient, clusterLimiter *ingest.ClusterRateLimiter) (*Server, error) {
	var opts []grpc.ServerOption

	log.Printf("========================================")
	log.Printf("[gRPC] NewServer called with TLSEnabled=%v", cfg.TLSEnabled)
	log.Printf("========================================")

	// Setup TLS/mTLS if enabled
	var certManager *security.CertManager
	if cfg.TLSEnabled {
		log.Printf("[gRPC] TLS enabled, loading TLS configuration...")
		log.Printf("[gRPC] CertPath: %s, KeyPath: %s, CACertPath: %s",
			cfg.TLSCertPath, cfg.TLSKeyPath, cfg.TLSCACertPath)

		// Create certificate manager for dynamic loading
		var err error
		certManager, err = security.NewCertManager(
			cfg.TLSCertPath,
			cfg.TLSKeyPath,
			cfg.TLSCACertPath,
		)
		if err != nil {
			log.Printf("[gRPC] ERROR: Failed to create certificate manager: %v", err)
			return nil, fmt.Errorf("failed to create certificate manager: %w", err)
		}
		log.Printf("[gRPC] ✅ CertManager created successfully")

		// Start expiry monitoring
		certManager.StartExpiryMonitoring()
		log.Printf("[gRPC] ✅ CertManager expiry monitoring started")

		// Load CA certificate for client verification
		caCert, err := os.ReadFile(cfg.TLSCACertPath)
		if err != nil {
			log.Printf("[gRPC] ERROR: Failed to read CA cert from %s: %v", cfg.TLSCACertPath, err)
			return nil, fmt.Errorf("failed to read CA cert: %w", err)
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			log.Printf("[gRPC] ERROR: Failed to parse CA certificate")
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		log.Printf("[gRPC] CA certificate loaded successfully")

		// Create TLS config with dynamic certificate loading
		tlsConfig := &tls.Config{
			GetCertificate: certManager.GetTLSCertificate, // Dynamic cert loading
			ClientAuth:     tls.RequireAndVerifyClientCert,
			ClientCAs:      certPool,
			MinVersion:     tls.VersionTLS13,
		}

		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.Creds(creds))
		log.Printf("[gRPC] ✅ gRPC server configured with mTLS and certificate rotation")
	} else {
		log.Printf("[gRPC] WARNING: gRPC server running without TLS (insecure)")
	}

	grpcServer := grpc.NewServer(opts...)

	// Register SBOM service (Phase 1: Agent→Core SBOM ingestion); per-cluster rate limit (Finding #6)
	sbomServiceServer := NewSBOMServiceServer(db, natsClient, clusterLimiter)
	agentpb.RegisterAgentServiceServer(grpcServer, sbomServiceServer)

	log.Printf("[gRPC] ✅ Registered AgentService (SBOM ingestion)")
	if db == nil {
		log.Printf("[gRPC] ⚠️  WARNING: Database is nil - SBOM service will be in degraded mode")
	}

	return &Server{
		config:      cfg,
		db:          db,
		server:      grpcServer,
		certManager: certManager,
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	// Bind to all interfaces (0.0.0.0) to ensure service connectivity
	addr := "0.0.0.0:" + s.config.GRPCPort
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[gRPC] ERROR: Failed to listen on %s: %v", addr, err)
		return err
	}

	// Log TLS status when starting server
	if s.config.TLSEnabled {
		log.Printf("Starting gRPC server on %s WITH mTLS", addr)
	} else {
		log.Printf("Starting gRPC server on %s WITHOUT TLS (INSECURE)", addr)
	}
	
	log.Printf("[gRPC] ✅ gRPC server listening on %s", addr)
	err = s.server.Serve(lis)
	if err != nil {
		log.Printf("[gRPC] ERROR: gRPC server stopped: %v", err)
	}
	return err
}

func (s *Server) Stop() {
	if s.certManager != nil {
		s.certManager.Stop()
	}
	s.server.GracefulStop()
}

// GetCertManager returns the certificate manager (for API handlers)
func (s *Server) GetCertManager() *security.CertManager {
	return s.certManager
}
