package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/fortuna/agent/internal/client"
	"github.com/fortuna/agent/internal/cluster"
	"github.com/fortuna/agent/internal/config"
	"github.com/fortuna/agent/internal/k8s"
	"github.com/fortuna/agent/internal/runtime"
	"github.com/fortuna/agent/internal/sbom"
	"github.com/fortuna/agent/internal/syncer"
	"github.com/fortuna/agent/internal/watcher"
	pb "github.com/fortuna/api/proto/agent"
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
	k8sClient, err := k8s.NewClient(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to create Kubernetes client: %v", err)
	}
	log.Printf("✅ Kubernetes client initialized")

	// Resolve cluster identity: auto-discovery from K8s API, or env override (optional)
	clusterInfo, err := cluster.Discover(ctx, k8sClient.Clientset, cfg.Kubeconfig)
	if err != nil {
		log.Fatalf("❌ Cluster discovery failed: %v", err)
	}
	cfg.ClusterID = clusterInfo.ID
	cfg.ClusterName = clusterInfo.Name
	log.Printf("📋 [cluster] id=%s source=%s name=%s", clusterInfo.ID, clusterInfo.Source, clusterInfo.Name)

	// Start periodic full sync to Core HTTP endpoint (pods/RBAC/resources)
	syncClientset, ok := k8sClient.Clientset.(*kubernetes.Clientset)
	if !ok {
		log.Fatalf("❌ Failed to cast Clientset to *kubernetes.Clientset")
	}
	autoSyncer := syncer.NewSyncer(syncClientset, cfg.CoreHTTPEndpoint, clusterInfo, cfg.SyncInterval, cfg.WatchNamespace)
	autoSyncer.Start(ctx)

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

	// Initialize SBOM processor (Agent = Data Plane, NO CVE matching)
	// CVE matching is done in Core, not in Agent (see LOGIC_FLOW_REFACTOR_IMPLEMENTATION.md)
	sbomProcessor := sbom.NewProcessor(grpcClient, cfg.AgentID, cfg.NodeID, cfg.NodeName)
	log.Printf("✅ SBOM processor initialized (CVE matching done in Core)")

	// Create SBOM work queue for async processing
	// This prevents blocking the informer during slow SBOM extraction (2-3 min per pod)
	// Workers: Use 2 workers to reduce memory usage (reduced from 3)
	// Queue buffer: 30 pods (reduced from 100 to prevent memory buildup)
	workers := 2
	sbomQueue := sbom.NewWorkQueue(sbomProcessor, workers)
	sbomQueue.Start()
	log.Printf("✅ SBOM work queue started with %d workers", workers)
	defer sbomQueue.Stop()

	// Create pod event handler (for backward compatibility, but won't be used if queue is provided)
	podHandler := func(ctx context.Context, pod *corev1.Pod) error {
		return sbomProcessor.ProcessPod(ctx, pod)
	}

	// Initialize local pod watcher with work queue for async processing
	// This allows the informer to continue detecting new pods while SBOM extraction happens
	podWatcher := watcher.NewLocalPodWatcher(syncClientset, cfg.NodeName, podHandler, sbomQueue.Queue())
	log.Printf("✅ Local pod watcher initialized for node: %s (async SBOM processing enabled)", cfg.NodeName)

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
				} else {
					// Log successful heartbeat periodically (every 10th heartbeat = 5 minutes)
					// This helps verify heartbeat is working without flooding logs
					log.Printf("✅ Heartbeat successful")
				}
			}
		}
	}()

	// Start runtime events reader (optional)
	if cfg.RuntimeEventsEnabled {
		reader := runtime.NewReader(cfg.RuntimeEventsPath, cfg.RuntimeEventsPoll, cfg.CoreHTTPEndpoint)
		go reader.Start(ctx)
		log.Printf("✅ Runtime events reader enabled (path=%s poll=%s)", cfg.RuntimeEventsPath, cfg.RuntimeEventsPoll)
	}

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
	// RegisterAgentRequest fields: AgentId, Hostname, NodeName, Version, Capabilities
	req := &pb.RegisterAgentRequest{
		AgentId:      cfg.AgentID,
		Hostname:     cfg.NodeID, // Use NodeID as Hostname
		NodeName:     cfg.NodeName,
		Version:      BuildVersion,
		Capabilities: []string{"sbom", "pod-watcher"}, // CVE matching done in Core
	}

	resp, err := grpcClient.RegisterAgent(ctx, req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("registration failed: %s", resp.Message)
	}

	log.Printf("📋 Agent registered with Core (Cluster ID: %s)", resp.ClusterId)

	return nil
}

func pingCore(ctx context.Context, grpcClient client.GRPCClient, cfg *config.Config) error {
	// PingRequest fields may vary - check proto definition
	// For now, use minimal request
	req := &pb.PingRequest{}

	resp, err := grpcClient.Ping(ctx, req)
	if err != nil {
		return err
	}

	// PingResponse fields may vary - check proto definition
	// For now, assume success if no error
	if resp == nil {
		return fmt.Errorf("ping response is nil")
	}

	return nil
}

func logConfig(cfg *config.Config) {
	log.Printf("========================================")
	log.Printf("📋 Configuration:")
	log.Printf("   Agent ID: %s", cfg.AgentID)
	log.Printf("   Cluster ID: %s", cfg.ClusterID)
	if cfg.ClusterName != "" && cfg.ClusterName != cfg.ClusterID {
		log.Printf("   Cluster Name (display): %s", cfg.ClusterName)
	}
	log.Printf("   Node Name: %s", cfg.NodeName)
	log.Printf("   Node ID: %s", cfg.NodeID)
	log.Printf("   Core Endpoint: %s", cfg.CoreGRPCEndpoint)
	log.Printf("   Core HTTP Endpoint: %s", cfg.CoreHTTPEndpoint)
	log.Printf("   Sync Interval: %s", cfg.SyncInterval)
	log.Printf("   TLS Enabled: %v", cfg.TLSEnabled)
	if cfg.TLSEnabled {
		log.Printf("   TLS Cert: %s", cfg.TLSCertPath)
		log.Printf("   TLS Key: %s", cfg.TLSKeyPath)
		log.Printf("   TLS CA: %s", cfg.TLSCACertPath)
	}
	log.Printf("========================================")
}
