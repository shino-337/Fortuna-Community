package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/fortuna/core/internal/api"
	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/internal/grpc"
	"github.com/fortuna/core/internal/health"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/internal/scheduler"
	"github.com/fortuna/core/internal/storage"
	"github.com/fortuna/core/internal/webhook"
	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/messaging"
	"github.com/fortuna/core/pkg/policy"
	"github.com/fortuna/core/pkg/worker"
	"github.com/nats-io/nats.go"

	_ "github.com/fortuna/core/pkg/metrics" // Import to register admission metrics
	_ "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Build info (set via -ldflags at build time)
var (
	BuildVersion = "dev"
	BuildCommit  = "none"
	BuildTime    = "unknown"
)

func main() {
	// CRITICAL: Use os.Stdout directly first to ensure logs appear
	fmt.Fprintf(os.Stdout, "========================================\n")
	fmt.Fprintf(os.Stdout, "[MAIN] Starting KSAM Core...\n")
	fmt.Fprintf(os.Stdout, "========================================\n")
	log.Printf("[Build] version=%s commit=%s time=%s", BuildVersion, BuildCommit, BuildTime)

	// Ensure standard logger writes to stdout with timestamps for easier kubectl logs viewing
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	log.Printf("[MAIN] Loading configuration...")
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Debug: Log TLS configuration - MUST appear before any other logs
	log.Printf("========================================")
	log.Printf("[Config] TLS_ENABLED=%v", cfg.TLSEnabled)
	log.Printf("[Config] TLS_CERT_PATH=%s", cfg.TLSCertPath)
	log.Printf("[Config] TLS_CA_CERT_PATH=%s", cfg.TLSCACertPath)
	log.Printf("[Config] TLS_KEY_PATH=%s", cfg.TLSKeyPath)
	log.Printf("========================================")

	// Initialize database
	db, err := storage.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Get underlying sql.DB for proper cleanup
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get database connection: %v", err)
	}
	defer sqlDB.Close()

	// Run migrations
	// Note: Migration may fail with "insufficient arguments" error (known GORM/PostgreSQL issue)
	// This is non-fatal and tables may still be created
	if err := storage.Migrate(db); err != nil {
		errStr := err.Error()
		// Check if error is the known "insufficient arguments" issue
		if errStr != "" && (strings.Contains(errStr, "insufficient arguments") ||
			strings.Contains(errStr, "migration 1 failed") ||
			strings.Contains(errStr, "Migration 1 failed")) {
			log.Printf("WARNING: Migration failed with known GORM/PostgreSQL compatibility issue: %v", err)
			log.Printf("WARNING: Continuing despite migration error - tables may still be created")
			// Verify tables exist
			var tableExists bool
			if checkErr := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'clusters')").Scan(&tableExists).Error; checkErr == nil && tableExists {
				log.Printf("INFO: Tables verified to exist, continuing...")
			} else {
				log.Printf("WARNING: Tables may not exist - application may have limited functionality")
			}
		} else {
			log.Fatalf("Failed to run migrations: %v", err)
		}
	}

	// Run post-migrations (create default admin, etc.)
	if err := migrations.RunPostMigrations(db); err != nil {
		log.Printf("Warning: Failed to run post-migrations: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize NATS client
	natsClient, err := messaging.NewNATSClient(cfg.NATSEndpoint)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer natsClient.Close()

	js := natsClient.JetStream()

	// Initialize worker pool
	workerPool, err := worker.NewPool(js, 5) // 5 concurrent workers per worker type
	if err != nil {
		log.Printf("Warning: Failed to create worker pool with DLQ: %v. Continuing without DLQ.", err)
		// Create pool without DLQ if setup fails
		workerPool, err = worker.NewPool(js, 5)
		if err != nil {
			log.Fatalf("Failed to create worker pool: %v", err)
		}
	}
	log.Printf("[Main] ========================================")
	log.Printf("[Main] About to add workers to pool...")
	log.Printf("[Main] WorkerPool check: workerPool == nil: %v", workerPool == nil)
	log.Printf("[Main] Adding workers to pool...")
	workerPool.AddWorker(worker.NewNormalizerWorker(js, db))
	log.Printf("[Main] ✅ Added NormalizerWorker")
	workerPool.AddWorker(worker.NewCorrelatorWorker(js, db))
	log.Printf("[Main] ✅ Added CorrelatorWorker")
	workerPool.AddWorker(worker.NewRiskWorker(js, db)) // Add Risk Engine worker
	log.Printf("[Main] ✅ Added RiskWorker")
	log.Printf("[Main] ✅ Added 3 workers to pool (normalizer, correlator, risk)")
	log.Printf("[Main] ========================================")

	// Phase 2.7: Initialize Policy Evaluator and Worker
	log.Printf("[Main] ========================================")
	log.Printf("[Main] Phase 2.7: Starting Policy Evaluator initialization...")
	log.Printf("[Main] About to call policy.NewEvaluator(db)...")
	log.Printf("[Main] Database connection check: db == nil: %v", db == nil)

	// Add panic recovery to catch any silent failures
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Main] ❌ PANIC in Policy Evaluator initialization: %v", r)
			panic(r) // Re-panic to ensure we see it
		}
	}()

	log.Printf("[Main] Calling policy.NewEvaluator(db)...")
	policyEvaluator, err := policy.NewEvaluator(db)
	if err != nil {
		log.Fatalf("[Main] ❌ Failed to create policy evaluator: %v", err)
	}
	log.Printf("[Main] ✅ Policy Evaluator initialized successfully")
	log.Printf("[Main] Policy Evaluator pointer: %p", policyEvaluator)
	log.Printf("[Main] ========================================")

	// Add Policy Worker to worker pool (for slow path processing)
	log.Printf("[Main] Creating Policy Worker...")
	policyWorker := policy.NewPolicyWorker(db, policyEvaluator)
	log.Printf("[Main] ✅ Policy Worker created")
	// Subscribe to violation events
	sub, err := js.Subscribe("ksam.policy.violation.detected", func(msg *nats.Msg) {
		ctx := context.Background()
		if err := policyWorker.ProcessViolationEvent(ctx, msg.Data); err != nil {
			log.Printf("[PolicyWorker] Failed to process violation event: %v", err)
		}
		msg.Ack()
	}, nats.Durable("ksam-policy-worker"))
	if err != nil {
		log.Printf("[Main] Warning: Failed to subscribe to policy violation events: %v", err)
	} else {
		log.Printf("[Main] ✅ Policy Worker subscribed to violation events")
		defer sub.Unsubscribe()
	}

	if err := workerPool.Start(); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}
	defer workerPool.Stop()

	// ============================================================
	// SBOM/CVE pipeline (event-driven, non-duplicated consumption)
	// ============================================================
	// NOTE: We intentionally run SBOM/CVE consumers outside the generic worker pool
	// because the pool creates one durable consumer per concurrency slot, which would
	// duplicate expensive SBOM generation and CVE matching.
	//
	// IMPORTANT (dev/e2e): Using JetStream *push* durables via js.Subscribe is fragile across fast rollouts:
	// the durable consumer retains its deliver subject (inbox) and subsequent restarts can fail with:
	// "consumer is already bound to a subscription".
	// For now we run these consumers as *ephemeral* by default, and only enable durables when explicitly requested.
	useDurables := strings.EqualFold(strings.TrimSpace(os.Getenv("KSAM_JS_DURABLES")), "true")
	sbomDurable := "sbom-worker"
	cveDurable := "cve-matcher-worker"

	sbomWorker := worker.NewSBOMWorker(js, db, natsClient)
	sbomOpts := []nats.SubOpt{
		nats.ManualAck(),
		nats.DeliverAll(),      // Changed from DeliverNew() to process all messages, including those published before subscription
		nats.MaxAckPending(10), // Increased from 1 to 10 to prevent slow consumer message drops
		nats.AckWait(10 * time.Minute),
	}
	if useDurables {
		sbomOpts = append(sbomOpts, nats.Durable(sbomDurable))
	}
	sbomSub, err := js.Subscribe(sbomWorker.Subject(), func(msg *nats.Msg) {
		if err := sbomWorker.Process(ctx, msg); err != nil {
			log.Printf("[SBOMWorker] Error: %v (will retry via NATS redelivery)", err)
			return
		}
		msg.Ack()
	}, sbomOpts...)
	if err != nil {
		log.Printf("[Main] Warning: Failed to subscribe SBOM worker: %v", err)
	} else {
		log.Printf("[Main] ✅ SBOMWorker subscribed to %s", sbomWorker.Subject())
		defer sbomSub.Unsubscribe()
	}

	cveWorker := worker.NewCVEMatcherWorker(db)
	cveOpts := []nats.SubOpt{
		nats.ManualAck(),
		nats.DeliverAll(), // Changed from DeliverNew() to process all messages, including those published before subscription
		nats.MaxAckPending(50),
		nats.AckWait(2 * time.Minute),
	}
	if useDurables {
		cveOpts = append(cveOpts, nats.Durable(cveDurable))
	}
	cveSub, err := js.Subscribe(cveWorker.Subject(), func(msg *nats.Msg) {
		if err := cveWorker.Process(ctx, msg); err != nil {
			log.Printf("[CVEMatcherWorker] Error: %v (will retry via NATS redelivery)", err)
			return
		}
		msg.Ack()
	}, cveOpts...)
	if err != nil {
		log.Printf("[Main] Warning: Failed to subscribe CVE matcher worker: %v", err)
	} else {
		log.Printf("[Main] ✅ CVEMatcherWorker subscribed to %s", cveWorker.Subject())
		defer cveSub.Unsubscribe()
	}

	// Start queue depth monitoring
	workerPool.StartQueueDepthMonitoring(ctx)
	log.Printf("Queue depth monitoring started for all workers")

	// Start risk evaluation scheduler (runs every 6 hours)
	riskScheduler := scheduler.NewRiskScheduler(db, 6*time.Hour)
	riskScheduler.Start()
	defer riskScheduler.Stop()
	log.Printf("Risk evaluation scheduler started (interval: 6 hours)")

	// Layer 3: Start pod cleanup job (runs every 5 minutes)
	// CRITICAL: Must run in goroutine - Start() has infinite loop that blocks!
	podCleanupJob := scheduler.NewPodCleanupJob(db)
	go func() {
		log.Printf("[Main] Starting pod cleanup job in goroutine...")
		podCleanupJob.Start() // This blocks forever, so must be in goroutine
	}()
	defer podCleanupJob.Stop()
	log.Printf("Pod cleanup job started (interval: 5 minutes) - Layer 3: Background Cleanup")

	// Start insights cleanup job (runs every 24 hours)
	// CRITICAL: Must run in goroutine - Start() has infinite loop that blocks!
	insightsCleanupJob := scheduler.NewInsightsCleanupJob(db)
	go func() {
		log.Printf("[Main] Starting insights cleanup job in goroutine...")
		insightsCleanupJob.Start() // This blocks forever, so must be in goroutine
	}()
	defer insightsCleanupJob.Stop()
	log.Printf("Insights cleanup job started (interval: 24 hours)")

	// Initialize gRPC server
	log.Printf("[Main] Creating gRPC server with TLS_ENABLED=%v", cfg.TLSEnabled)
	grpcServer, err := grpc.NewServer(cfg, db, natsClient)
	if err != nil {
		log.Fatalf("Failed to create gRPC server: %v", err)
	}
	log.Printf("[Main] gRPC server created successfully")

	// Start gRPC server in goroutine
	log.Printf("[Main] Starting gRPC server goroutine on port %s", cfg.GRPCPort)
	go func() {
		log.Printf("[Main] gRPC server goroutine started, calling grpcServer.Start()...")
		if err := grpcServer.Start(ctx); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()
	log.Printf("[Main] gRPC server goroutine launched (non-blocking)")

	// Initialize REST API
	// Use gin.New() instead of gin.Default() to avoid default middleware that might interfere
	router := gin.New()

	// Add recovery middleware (from gin.Default())
	router.Use(gin.Recovery())

	// Add logger middleware (from gin.Default())
	router.Use(gin.Logger())

	// Add middleware - CORS must be first to handle preflight
	// IMPORTANT: Order matters! CORS must be first, then SecurityHeaders
	router.Use(middleware.CORS())
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.MetricsMiddleware())

	// Health endpoints (no auth required)
	router.GET("/health", health.HealthCheck(db))
	router.GET("/ready", health.ReadinessCheck(db))
	router.GET("/live", health.LivenessCheck())

	// Phase 2.7: Initialize Admission Webhook
	log.Printf("[Main] ========================================")
	log.Printf("[Main] Phase 2.7: Setting up admission webhook...")
	log.Printf("[Main] Policy Evaluator check: policyEvaluator == nil: %v", policyEvaluator == nil)
	log.Printf("[Main] Database check: db == nil: %v", db == nil)
	log.Printf("[Main] NATS client check: natsClient == nil: %v", natsClient == nil)
	log.Printf("[Main] Creating AdmissionWebhook instance...")

	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Main] ❌ PANIC in Admission Webhook initialization: %v", r)
			panic(r) // Re-panic to ensure we see it
		}
	}()

	admissionWebhook := webhook.NewAdmissionWebhook(db, policyEvaluator, natsClient)
	log.Printf("[Main] ✅ AdmissionWebhook instance created")
	log.Printf("[Main] AdmissionWebhook pointer: %p", admissionWebhook)
	log.Printf("[Main] ========================================")

	// Setup routes with certificate manager (if available)
	certManager := grpcServer.GetCertManager()
	if certManager != nil {
		log.Printf("[Main] ✅ CertManager available - certificate routes will be registered")
	} else {
		log.Printf("[Main] ⚠️  CertManager is nil - certificate routes will NOT be registered")
	}
	api.SetupRoutesWithCertManager(router, db, cfg, certManager)

	// Phase 2.7: Dedicated HTTPS server for admission webhook
	log.Printf("[Main] ========================================")
	log.Printf("[Main] Phase 2.7: Setting up HTTPS server for webhook...")
	log.Printf("[Main] gRPC TLS_ENABLED: %v", cfg.TLSEnabled)
	log.Printf("[Main] gRPC TLS_CERT_PATH: %s", cfg.TLSCertPath)
	log.Printf("[Main] Webhook TLS_CERT_PATH: %s", cfg.WebhookTLSCertPath)
	log.Printf("[Main] Webhook TLS_KEY_PATH: %s", cfg.WebhookTLSKeyPath)
	log.Printf("[Main] ========================================")

	var webhookServer *http.Server
	if cfg.WebhookTLSCertPath != "" && cfg.WebhookTLSKeyPath != "" {
		log.Printf("[Webhook] ========================================")
		log.Printf("[Webhook] Setting up dedicated HTTPS server for admission webhook...")
		log.Printf("[Webhook] Webhook Certificate: %s", cfg.WebhookTLSCertPath)
		log.Printf("[Webhook] Webhook Key: %s", cfg.WebhookTLSKeyPath)

		// Create dedicated router for webhook
		webhookRouter := gin.New()
		webhookRouter.Use(gin.Recovery())
		webhookRouter.Use(gin.Logger())

		// Only webhook endpoints - NO auth middleware needed (K8s handles auth)
		webhookRouter.POST("/admission/validate", gin.WrapF(admissionWebhook.Handle))
		webhookRouter.GET("/admission/health", gin.WrapF(admissionWebhook.HealthCheck))

		// Dedicated HTTPS server on port 8443 (standard webhook port)
		webhookServer = &http.Server{
			Addr:    ":8443",
			Handler: webhookRouter,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				},
			},
		}

		go func() {
			log.Printf("[Webhook] Starting webhook HTTPS server on :8443")
			if err := webhookServer.ListenAndServeTLS(cfg.WebhookTLSCertPath, cfg.WebhookTLSKeyPath); err != nil && err != http.ErrServerClosed {
				log.Fatalf("[Webhook] Failed to start webhook HTTPS server: %v", err)
			}
		}()

		log.Printf("[Webhook] ✅ Webhook HTTPS server started on :8443")
		log.Printf("[Webhook] ========================================")
	} else {
		log.Printf("[Webhook] ========================================")
		log.Printf("[Webhook] ❌ WARNING: Webhook TLS certificates not configured!")
		log.Printf("[Webhook] Admission webhook will NOT work without HTTPS!")
		log.Printf("[Webhook] Set WEBHOOK_TLS_CERT_PATH and WEBHOOK_TLS_KEY_PATH")
		log.Printf("[Webhook] ========================================")

		// Fallback: Register webhook on main HTTP router (will NOT work with K8s)
		log.Printf("[Webhook] ⚠️  Registering webhook on HTTP router (NOT RECOMMENDED)")
		router.POST("/admission/validate", gin.WrapF(admissionWebhook.Handle))
		router.GET("/admission/health", gin.WrapF(admissionWebhook.HealthCheck))
	}

	// REST API HTTP server (port 8080)
	httpServer := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
	}

	go func() {
		log.Printf("[Main] Starting HTTP server on port %s (REST API)", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown REST API server
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}

	// Shutdown webhook server if running
	if webhookServer != nil {
		if err := webhookServer.Shutdown(ctx); err != nil {
			log.Printf("[Webhook] Error shutting down webhook server: %v", err)
		}
	}

	grpcServer.Stop()
}
