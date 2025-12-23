package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	corev1 "k8s.io/api/core/v1"

	pb "github.com/fortuna/api/proto/agent"
	"github.com/fortuna/agent/internal/client"
	"github.com/fortuna/agent/internal/config"
	"github.com/fortuna/agent/internal/k8s"
	"github.com/fortuna/agent/internal/sbom"
	"github.com/fortuna/agent/internal/watcher"
)

// Build info (set via -ldflags at build time)
var (
	BuildVersion = "dev"
	BuildCommit  = "none"
	BuildTime    = "unknown"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Printf("========================================")
	log.Printf("🚀 Starting Fortuna Agent")
	log.Printf("========================================")
	log.Printf("[Build] version=%s commit=%s time=%s", BuildVersion, BuildCommit, BuildTime)

	// Load configuration
	cfg := config.LoadConfig()
	logConfig(cfg)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("❌ Failed to create Kubernetes client: %v", err)
	}
	log.Printf("✅ Kubernetes client initialized")

	// Initialize gRPC client with mTLS
	grpcClient := client.NewMTLSClient(
		cfg.CoreGRPCEndpoint,
		cfg.TLSEnabled,
		cfg.TLSCertPath,
		cfg.TLSKeyPath,
		cfg.TLSCACertPath,
	)

	// Connect to Core
	log.Printf("🔗 Connecting to Core at %s...", cfg.CoreGRPCEndpoint)
	if err := grpcClient.Connect(ctx); err != nil {
		log.Fatalf("❌ Failed to connect to Core: %v", err)
	}
	defer grpcClient.Close()
	log.Printf("✅ Connected to Core")

	// Register agent with Core
	if err := registerAgent(ctx, grpcClient, cfg); err != nil {
		log.Printf("⚠️  Agent registration failed: %v (continuing anyway)", err)
	} else {
		log.Printf("✅ Agent registered with Core")
	}

	// Ping Core to verify connectivity
	if err := pingCore(ctx, grpcClient, cfg); err != nil {
		log.Printf("⚠️  Core ping failed: %v", err)
	} else {
		log.Printf("✅ Core is reachable")
	}

	// Get CVE data directory from env (default: /etc/fortuna/cve-data)
	cveDataDir := os.Getenv("CVE_DATA_DIR")
	if cveDataDir == "" {
		cveDataDir = "/etc/fortuna/cve-data"
	}
	log.Printf("CVE data directory: %s", cveDataDir)

	// Initialize SBOM processor with CVE matching
	sbomProcessor := sbom.NewProcessor(grpcClient, cfg.AgentID, cfg.NodeID, cfg.NodeName, cveDataDir)
	log.Printf("✅ SBOM processor initialized")

	// Create pod event handler
	podHandler := func(ctx context.Context, pod *corev1.Pod) error {
		return sbomProcessor.ProcessPod(ctx, pod)
	}

	// Initialize local pod watcher
	podWatcher := watcher.NewLocalPodWatcher(k8sClient.Clientset, cfg.NodeName, podHandler)
	log.Printf("✅ Local pod watcher initialized for node: %s", cfg.NodeName)

	// Start pod watcher
	go func() {
		if err := podWatcher.Start(ctx); err != nil {
			log.Printf("❌ Pod watcher error: %v", err)
			cancel()
		}
	}()

	// Process existing pods (initial sync)
	log.Printf("🔄 Processing existing pods on node %s...", cfg.NodeName)
	existingPods, err := podWatcher.ListCurrentPods(ctx)
	if err != nil {
		log.Printf("⚠️  Failed to list existing pods: %v", err)
	} else {
		log.Printf("Found %d existing pods to process", len(existingPods))
		for _, pod := range existingPods {
			if err := sbomProcessor.ProcessPod(ctx, pod); err != nil {
				log.Printf("⚠️  Failed to process existing pod %s/%s: %v", pod.Namespace, pod.Name, err)
			}
		}
		log.Printf("✅ Initial pod processing complete")
	}

	// Start heartbeat goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := pingCore(ctx, grpcClient, cfg); err != nil {
					log.Printf("⚠️  Heartbeat failed: %v", err)
				}
			}
		}
	}()

	log.Printf("========================================")
	log.Printf("✅ Fortuna Agent is running")
	log.Printf("   Node: %s", cfg.NodeName)
	log.Printf("   Agent ID: %s", cfg.AgentID)
	log.Printf("   Core: %s", cfg.CoreGRPCEndpoint)
	log.Printf("========================================")

	// Wait for shutdown signal
	<-sigCh
	log.Printf("🛑 Shutdown signal received")

	// Graceful shutdown
	cancel()
	podWatcher.Stop()

	log.Printf("👋 Fortuna Agent stopped")
}

func registerAgent(ctx context.Context, grpcClient client.GRPCClient, cfg *config.Config) error {
	req := &pb.RegisterAgentRequest{
		AgentId:      cfg.AgentID,
		NodeId:       cfg.NodeID,
		NodeName:     cfg.NodeName,
		AgentVersion: BuildVersion,
		Capabilities: []string{"sbom", "pod-watcher"},
		NodeLabels:   map[string]string{
			"node": cfg.NodeName,
		},
	}

	resp, err := grpcClient.RegisterAgent(ctx, req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("registration failed: %s", resp.Message)
	}

	log.Printf("📋 Received config from Core:")
	if resp.Config != nil {
		log.Printf("   Rate limit: %d/s", resp.Config.RateLimit)
		log.Printf("   Batch size: %d", resp.Config.BatchSize)
		log.Printf("   Batch timeout: %dms", resp.Config.BatchTimeoutMs)
	}

	return nil
}

func pingCore(ctx context.Context, grpcClient client.GRPCClient, cfg *config.Config) error {
	req := &pb.PingRequest{
		AgentId:   cfg.AgentID,
		NodeId:    cfg.NodeID,
		Timestamp: timestamppb.New(time.Now()),
	}

	resp, err := grpcClient.Ping(ctx, req)
	if err != nil {
		return err
	}

	if !resp.Healthy {
		return fmt.Errorf("core is not healthy: %s", resp.Message)
	}

	return nil
}

func logConfig(cfg *config.Config) {
	log.Printf("========================================")
	log.Printf("📋 Configuration:")
	log.Printf("   Agent ID: %s", cfg.AgentID)
	log.Printf("   Node Name: %s", cfg.NodeName)
	log.Printf("   Node ID: %s", cfg.NodeID)
	log.Printf("   Core Endpoint: %s", cfg.CoreGRPCEndpoint)
	log.Printf("   TLS Enabled: %v", cfg.TLSEnabled)
	if cfg.TLSEnabled {
		log.Printf("   TLS Cert: %s", cfg.TLSCertPath)
		log.Printf("   TLS Key: %s", cfg.TLSKeyPath)
		log.Printf("   TLS CA: %s", cfg.TLSCACertPath)
	}
	log.Printf("========================================")
}
