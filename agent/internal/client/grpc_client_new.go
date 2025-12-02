package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/ksam/agent/internal/config"
	fortuna "github.com/ksam/agent/proto/gen/proto"
)

// NewGRPCClient creates a new gRPC client using fortuna_agent.proto
type NewGRPCClient struct {
	conn       *grpc.ClientConn
	client     fortuna.AgentServiceClient
	config     *config.Config
	agentID    string
	clusterID  string
	nodeName   string
	version    string
	registered bool
}

// NewNewGRPCClient creates a new gRPC client
func NewNewGRPCClient(cfg *config.Config) (*NewGRPCClient, error) {
	// Configure gRPC connection
	var creds credentials.TransportCredentials
	var err error

	if cfg.TLSEnabled {
		creds, err = loadTLSClientCredentials(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		log.Println("gRPC client configured with mTLS")
	} else {
		creds = insecure.NewCredentials()
		log.Println("gRPC client running without TLS (insecure)")
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    2 * time.Minute,
			Timeout: 20 * time.Second,
		}),
		grpc.WithBlock(),
	}

	// Connect to Core with retry
	var conn *grpc.ClientConn

	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		conn, err = grpc.DialContext(ctx, cfg.CoreEndpoint, opts...)
		cancel()

		if err == nil {
			break
		}

		if attempt < 2 {
			waitTime := time.Duration(attempt+1) * 5 * time.Second
			log.Printf("Failed to connect to core (attempt %d/3): %v. Retrying in %v...", attempt+1, err, waitTime)
			time.Sleep(waitTime)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to core after retries: %w", err)
	}

	// Get agent ID, cluster ID, node name
	agentID := os.Getenv("HOSTNAME")
	if agentID == "" {
		agentID = fmt.Sprintf("agent-%d", time.Now().Unix())
	}

	clusterID := cfg.ClusterID
	if clusterID == "" {
		clusterID = "default"
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		nodeName = "unknown"
	}

	client := &NewGRPCClient{
		conn:      conn,
		client:    fortuna.NewAgentServiceClient(conn),
		config:    cfg,
		agentID:   agentID,
		clusterID: clusterID,
		nodeName:  nodeName,
		version:   "1.0.0",
	}

	// Register with core
	req := &fortuna.RegisterRequest{
		AgentId:   agentID,
		ClusterId: clusterID,
		NodeName:  nodeName,
		Version:   "1.0.0",
	}
	if _, err := client.Register(context.Background(), req); err != nil {
		return nil, fmt.Errorf("failed to register: %w", err)
	}

	// Start heartbeat
	go client.startHeartbeat()

	return client, nil
}

// Register registers the agent with the core
func (c *NewGRPCClient) Register(ctx context.Context, req *fortuna.RegisterRequest) (*fortuna.RegisterResponse, error) {
	resp, err := c.client.Register(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("register failed: %w", err)
	}

	if !resp.Ok {
		return nil, fmt.Errorf("register failed: %s", resp.Message)
	}

	c.registered = true
	log.Printf("Successfully registered agent: %s, cluster: %s, node: %s", req.AgentId, req.ClusterId, req.NodeName)
	return resp, nil
}

// startHeartbeat starts periodic heartbeat
func (c *NewGRPCClient) startHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req := &fortuna.RegisterRequest{
			AgentId:   c.agentID,
			ClusterId: c.clusterID,
			NodeName:  c.nodeName,
			Version:   c.version,
		}

		_, err := c.client.Heartbeat(ctx, req)
		cancel()

		if err != nil {
			log.Printf("Heartbeat failed: %v", err)
		}
	}
}

// StreamInventory streams inventory items to core
func (c *NewGRPCClient) StreamInventory(ctx context.Context, items []*fortuna.InventoryItem) error {
	if !c.registered {
		req := &fortuna.RegisterRequest{
			AgentId:   c.agentID,
			ClusterId: c.clusterID,
			NodeName:  c.nodeName,
			Version:   c.version,
		}
		if _, err := c.Register(ctx, req); err != nil {
			return err
		}
	}

	stream, err := c.client.StreamInventory(ctx)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	// Send all inventory items
	for _, item := range items {
		if err := stream.Send(item); err != nil {
			return fmt.Errorf("failed to send item: %w", err)
		}
	}

	// Close stream
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("failed to close stream: %w", err)
	}

	if !resp.Ok {
		return fmt.Errorf("stream failed: %s", resp.Message)
	}

	log.Printf("Successfully streamed %d inventory items to core", len(items))
	return nil
}

// Heartbeat sends heartbeat to core
func (c *NewGRPCClient) Heartbeat(ctx context.Context, req *fortuna.RegisterRequest) error {
	_, err := c.client.Heartbeat(ctx, req)
	return err
}

// Close closes the gRPC connection
func (c *NewGRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// loadTLSClientCredentials loads TLS credentials for mTLS client
func loadTLSClientCredentials(cfg *config.Config) (credentials.TransportCredentials, error) {
	// Load CA certificate for server verification
	caCert, err := os.ReadFile(cfg.TLSCACertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	// Load client certificate and key
	clientCert, err := tls.LoadX509KeyPair(cfg.TLSCertPath, cfg.TLSKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client cert/key: %w", err)
	}

	// Create TLS config
	// Extract server name from endpoint (remove port)
	serverName := cfg.CoreEndpoint
	if idx := strings.LastIndex(serverName, ":"); idx > 0 {
		serverName = serverName[:idx]
	}

	tlsConfig := &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{clientCert},
		MinVersion:   tls.VersionTLS13,
		ServerName:   serverName, // Set server name for hostname verification
	}

	return credentials.NewTLS(tlsConfig), nil
}
