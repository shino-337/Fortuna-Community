package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/fortuna/agent/internal/client"
	"github.com/fortuna/agent/internal/cluster"
	"github.com/fortuna/agent/internal/config"
	"github.com/fortuna/agent/internal/k8s"
	"github.com/fortuna/agent/internal/poddetail"
	"github.com/fortuna/agent/internal/runtime"
	ebpfruntime "github.com/fortuna/agent/internal/runtime/ebpf"
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

// coreConnectHint returns a short diagnostic hint for Core connection failures.
func coreConnectHint(err error) string {
	if err == nil {
		return "ok"
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "connection refused"):
		return "Core pod may not be Ready yet or gRPC not listening on 9090; check: kubectl get pods -n fortuna -l app.kubernetes.io/component=core && kubectl get endpoints -n fortuna fortuna-core"
	case strings.Contains(s, "connection reset"), strings.Contains(s, "EOF"):
		return "Core may have restarted; retry will reconnect"
	case strings.Contains(s, "i/o timeout"), strings.Contains(s, "deadline exceeded"):
		return "Network/DNS issue; from agent pod try: nslookup fortuna-core.fortuna.svc.cluster.local"
	case strings.Contains(s, "no such host"), strings.Contains(s, "Temporary failure in name resolution"):
		return "DNS cannot resolve Core service; ensure Core Service exists in namespace fortuna"
	case strings.Contains(s, "tls:"), strings.Contains(s, "handshake"), strings.Contains(s, "certificate"), strings.Contains(s, "x509"):
		return "mTLS failure; ensure fortuna-agent-tls and fortuna-core-tls are signed by same CA (fortuna-ca-cert)"
	default:
		return "Check Core logs and Service/Endpoints; Agent will retry."
	}
}

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
	autoSyncer := syncer.NewSyncer(syncClientset, cfg.CoreHTTPEndpoint, clusterInfo, cfg.SyncInterval, cfg.WatchNamespace, cfg.AgentID, cfg.NodeName, BuildVersion)
	autoSyncer.Start(ctx)

	// Pod Detail reporter: runtime metrics + process snapshots to Core
	podDetailInterval := 2 * time.Minute
	if d := os.Getenv("POD_DETAIL_REPORT_INTERVAL"); d != "" {
		if dur, err := time.ParseDuration(d); err == nil && dur > 0 {
			podDetailInterval = dur
		}
	}
	podDetailReporter := poddetail.NewReporter(syncClientset, k8sClient.Config, cfg.CoreHTTPEndpoint, clusterInfo.ID, cfg.NodeName, podDetailInterval)
	go podDetailReporter.Start(ctx)

	// K8s Events collector: informer → batch POST to Core (Phase 2.1)
	eventsCollector := poddetail.NewEventsCollector(syncClientset, cfg.CoreHTTPEndpoint, clusterInfo.ID)
	go eventsCollector.Start(ctx)

	// Initialize gRPC client with mTLS (use interface so Reconnect can be called from heartbeat)
	var grpcClient client.GRPCClient = client.NewMTLSClient(
		cfg.CoreGRPCEndpoint,
		cfg.TLSEnabled,
		cfg.TLSCertPath,
		cfg.TLSKeyPath,
		cfg.TLSCACertPath,
		clusterInfo.ID, // for Core per-cluster rate limit (Finding #6)
	)

	// Connect to Core with retry (never exit: DNS/network may be slow after deploy/restart)
	const connectBackoff = 15 * time.Second
	for {
		log.Printf("🔗 Connecting to Core at %s...", cfg.CoreGRPCEndpoint)
		if err := grpcClient.Connect(ctx); err != nil {
			log.Printf("⚠️  Connect failed: %v", err)
			log.Printf("   Diagnostic: %s (retrying in %v)", coreConnectHint(err), connectBackoff)
			select {
			case <-ctx.Done():
				log.Fatalf("❌ Context cancelled before Core connection")
			case <-time.After(connectBackoff):
				continue
			}
		}
		break
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
	// Workers: default 2; set SBOM_WORKERS=1 to reduce peak memory (e.g. avoid OOM when limit is low)
	workers := 2
	if w := os.Getenv("SBOM_WORKERS"); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n >= 1 && n <= 4 {
			workers = n
		}
	}
	sbomQueue := sbom.NewWorkQueue(sbomProcessor, workers)
	sbomQueue.Start()
	sbomQueue.StartReconciliation(10 * time.Minute)
	log.Printf("✅ SBOM work queue started with %d workers (reconciliation every 10m)", workers)
	defer sbomQueue.Stop()

	// Create pod event handler (for backward compatibility, but won't be used if queue is provided)
	podHandler := func(ctx context.Context, pod *corev1.Pod) error {
		return sbomProcessor.ProcessPod(ctx, pod)
	}

	// Initialize local pod watcher with work queue for async processing
	// This allows the informer to continue detecting new pods while SBOM extraction happens
	podWatcher := watcher.NewLocalPodWatcher(syncClientset, cfg.NodeName, podHandler, sbomQueue)
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

	// Start heartbeat goroutine so Core updates last_seen_at and dashboard shows agents in time.
	// On 3 consecutive Ping failures (e.g. DNS "no such host" after Core restart), reconnect and re-register.
	go func() {
		interval := cfg.HeartbeatInterval
		if interval < 5*time.Second {
			interval = 5 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		successCount := 0
		failCount := 0
		const reconnectAfterFailures = 3

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := pingCore(ctx, grpcClient, cfg); err != nil {
					failCount++
					log.Printf("⚠️  Heartbeat failed: %v", err)
					if failCount >= reconnectAfterFailures {
						log.Printf("🔄 Reconnecting to Core after %d consecutive failures...", failCount)
						if reconnErr := grpcClient.Reconnect(ctx); reconnErr != nil {
							log.Printf("⚠️  Reconnect failed: %v", reconnErr)
						} else {
							if regErr := registerAgent(ctx, grpcClient, cfg); regErr != nil {
								log.Printf("⚠️  Re-register after reconnect failed: %v", regErr)
							} else {
								log.Printf("✅ Reconnected and re-registered")
							}
							failCount = 0
						}
					}
				} else {
					failCount = 0
					successCount++
					// Log only every 20th success (~5 min at 15s) to avoid flooding
					if successCount%20 == 1 {
						log.Printf("✅ Heartbeat OK (interval=%s)", interval)
					}
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
	if cfg.FalcoEventsEnabled {
		reader := runtime.NewFalcoReader(cfg.FalcoEventsPath, cfg.FalcoEventsPoll, cfg.CoreHTTPEndpoint, cfg.NodeName, k8sClient.Clientset)
		go reader.Start(ctx)
		log.Printf("✅ Falco events reader enabled (path=%s poll=%s)", cfg.FalcoEventsPath, cfg.FalcoEventsPoll)
	}
	if cfg.EBPFEnabled {
		sensor := ebpfruntime.NewSensor(
			cfg.EBPFMode,
			cfg.CoreHTTPEndpoint,
			cfg.NodeName,
			cfg.EBPFEventFlushInterval,
			cfg.EBPFEventBufferSize,
			cfg.EBPFSimulate,
		)
		go sensor.Start(ctx)
		log.Printf("✅ eBPF sensor enabled (mode=%s flush=%s simulate=%v)", cfg.EBPFMode, cfg.EBPFEventFlushInterval, cfg.EBPFSimulate)
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
	agentID := cfg.AgentID
	if agentID == "" {
		agentID = cfg.NodeName + "-agent"
	}
	req := &pb.PingRequest{
		AgentId:  agentID,
		NodeName: cfg.NodeName,
	}

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
