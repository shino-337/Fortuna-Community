package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	pb "github.com/fortuna/api/proto/agent"

	"github.com/fortuna/agent/internal/config"
)

// GRPCClient interface for Agent→Core communication
type GRPCClient interface {
	Connect(ctx context.Context) error
	Close() error
	Reconnect(ctx context.Context) error // Close then Connect; use after transient DNS/connection failure
	SendSBOMFinding(ctx context.Context, finding *pb.SBOMFinding) (*pb.SBOMFindingResponse, error)
	SendCombinedFinding(ctx context.Context, finding *pb.CombinedFinding) (*pb.CombinedFindingResponse, error)
	RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error)
	Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error)
	// StreamInventory streams inventory items to Core (no-op in current Core; collector uses HTTP syncer in main path)
	StreamInventory(ctx context.Context, items interface{}) error
	// Heartbeat sends periodic health status to Core
	Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error)
}

// MTLSClient implements GRPCClient with mTLS support
type MTLSClient struct {
	endpoint   string
	tlsEnabled bool
	certPath   string
	keyPath    string
	caPath     string
	clusterID  string // optional; sent as x-cluster-id for per-cluster rate limit (Finding #6)
	conn       *grpc.ClientConn
	client     pb.AgentServiceClient
	logger     *log.Logger
	mu         sync.Mutex // serializes Reconnect/Close vs RPCs; prevents SendSBOM during nil client window after Close()
}

// NewMTLSClient creates a new mTLS-enabled gRPC client. clusterID is optional (for Core per-cluster rate limit).
func NewMTLSClient(endpoint string, tlsEnabled bool, certPath, keyPath, caPath string, clusterID string) *MTLSClient {
	return &MTLSClient{
		endpoint:   endpoint,
		tlsEnabled: tlsEnabled,
		certPath:   certPath,
		keyPath:    keyPath,
		caPath:     caPath,
		clusterID:  clusterID,
		logger:     log.New(log.Writer(), "[gRPCClient] ", log.LstdFlags),
	}
}

// Connect establishes gRPC connection with mTLS (idempotent if already connected).
func (c *MTLSClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client != nil {
		return nil
	}
	return c.dialLocked(ctx)
}

// closeLocked drops the connection and stub; caller must hold c.mu.
func (c *MTLSClient) closeLocked() {
	if c.conn != nil {
		c.logger.Printf("Closing gRPC connection")
		_ = c.conn.Close()
		c.conn = nil
		c.client = nil
	}
}

// dialLocked dials Core; caller must hold c.mu.
func (c *MTLSClient) dialLocked(ctx context.Context) error {
	c.closeLocked()

	c.logger.Printf("Connecting to Core at %s (TLS: %v)", c.endpoint, c.tlsEnabled)

	var opts []grpc.DialOption

	if c.tlsEnabled {
		c.logger.Printf("Loading mTLS certificates...")
		c.logger.Printf("  Cert: %s", c.certPath)
		c.logger.Printf("  Key:  %s", c.keyPath)
		c.logger.Printf("  CA:   %s", c.caPath)

		cert, err := tls.LoadX509KeyPair(c.certPath, c.keyPath)
		if err != nil {
			return fmt.Errorf("failed to load client cert/key: %w", err)
		}

		caCert, err := os.ReadFile(c.caPath)
		if err != nil {
			return fmt.Errorf("failed to read CA cert: %w", err)
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to append CA cert")
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      certPool,
			ServerName:   "fortuna-core.fortuna.svc.cluster.local",
			MinVersion:   tls.VersionTLS13,
		}

		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
		c.logger.Printf("✅ mTLS configured")
	} else {
		c.logger.Printf("⚠️  WARNING: TLS disabled (insecure connection)")
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                30 * time.Second,
		Timeout:             10 * time.Second,
		PermitWithoutStream: true,
	}))

	const dialTimeout = 15 * time.Second
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	opts = append(opts, grpc.WithBlock())

	conn, err := grpc.DialContext(dialCtx, c.endpoint, opts...)
	if err != nil {
		return fmt.Errorf("dial Core: %w (hint: check Core pod Ready, Service endpoints, DNS, mTLS certs)", err)
	}

	c.conn = conn
	c.client = pb.NewAgentServiceClient(conn)
	c.logger.Printf("✅ Connected to Core at %s", c.endpoint)

	return nil
}

// Close closes the gRPC connection and clears client so Connect can be called again
func (c *MTLSClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
	return nil
}

// Reconnect closes the current connection and establishes a new one (e.g. after DNS/connection recovery)
func (c *MTLSClient) Reconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dialLocked(ctx)
}

// SendSBOMFinding sends a single SBOM finding to Core (Finding #1.2: correlation ID in metadata).
func (c *MTLSClient) SendSBOMFinding(ctx context.Context, finding *pb.SBOMFinding) (*pb.SBOMFindingResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		if err := c.dialLocked(ctx); err != nil {
			return nil, fmt.Errorf("connect before SBOM: %w", err)
		}
	}

	correlationID := uuid.New().String()
	pairs := []string{"x-correlation-id", correlationID}
	if c.clusterID != "" {
		pairs = append(pairs, "x-cluster-id", c.clusterID)
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(pairs...))
	c.logger.Printf("Sending SBOM: pod=%s/%s image=%s correlation_id=%s", finding.Namespace, finding.PodName, finding.ImageDigest, correlationID)

	resp, err := c.client.SendSBOMFinding(ctx, finding)
	if err != nil {
		return nil, fmt.Errorf("SendSBOMFinding RPC failed: %w", err)
	}

	return resp, nil
}

// SendCombinedFinding sends combined SBOM + CVE findings to Core
func (c *MTLSClient) SendCombinedFinding(ctx context.Context, finding *pb.CombinedFinding) (*pb.CombinedFindingResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		if err := c.dialLocked(ctx); err != nil {
			return nil, fmt.Errorf("connect before CombinedFinding: %w", err)
		}
	}

	c.logger.Printf("Sending CombinedFinding: pod=%s/%s", finding.Sbom.Namespace, finding.Sbom.PodName)

	resp, err := c.client.SendCombinedFinding(ctx, finding)
	if err != nil {
		return nil, fmt.Errorf("SendCombinedFinding RPC failed: %w", err)
	}

	return resp, nil
}

// RegisterAgent registers the agent with Core
func (c *MTLSClient) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		if err := c.dialLocked(ctx); err != nil {
			return nil, fmt.Errorf("connect before RegisterAgent: %w", err)
		}
	}

	c.logger.Printf("Registering agent: id=%s node=%s", req.AgentId, req.NodeName)

	resp, err := c.client.RegisterAgent(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("RegisterAgent RPC failed: %w", err)
	}

	c.logger.Printf("✅ Agent registered: %s", resp.Message)
	return resp, nil
}

// Ping sends a health check ping to Core
func (c *MTLSClient) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		if err := c.dialLocked(ctx); err != nil {
			return nil, fmt.Errorf("connect before Ping: %w", err)
		}
	}

	resp, err := c.client.Ping(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Ping RPC failed: %w", err)
	}

	return resp, nil
}

// StreamInventory is a no-op: current Core does not expose StreamInventory RPC; agent uses HTTP syncer for inventory.
func (c *MTLSClient) StreamInventory(ctx context.Context, items interface{}) error {
	return nil
}

// Heartbeat sends periodic health status to Core.
func (c *MTLSClient) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		if err := c.dialLocked(ctx); err != nil {
			return nil, fmt.Errorf("connect before Heartbeat: %w", err)
		}
	}
	return c.client.Heartbeat(ctx, req)
}

// NewNewGRPCClient creates a GRPCClient from config (MTLS client) and connects. Used by collector when instantiated.
func NewNewGRPCClient(cfg *config.Config) (GRPCClient, error) {
	clusterID := ""
	if cfg != nil {
		clusterID = cfg.ClusterID
	}
	cli := NewMTLSClient(
		cfg.CoreGRPCEndpoint,
		cfg.TLSEnabled,
		cfg.TLSCertPath,
		cfg.TLSKeyPath,
		cfg.TLSCACertPath,
		clusterID,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := cli.Connect(ctx); err != nil {
		return nil, err
	}
	return cli, nil
}
