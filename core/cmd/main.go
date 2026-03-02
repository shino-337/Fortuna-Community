package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
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
	"github.com/fortuna/core/pkg/reconciler"
	"github.com/fortuna/core/pkg/security"
	"github.com/fortuna/core/pkg/worker"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"

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
	fmt.Fprintf(os.Stdout, "[MAIN] Starting Fortuna Core...\n")
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

	// 🥇 BƯỚC 1: Ensure database is available BEFORE starting gRPC/HTTP servers
	//
	// The system relies on DB-backed handlers (gRPC SBOM ingestion, REST APIs). Starting servers
	// with a nil DB causes permanent "database not available" behavior because handlers capture
	// the initial nil pointer.
	log.Printf("[MAIN] ========================================")
	log.Printf("[MAIN] 🥇 BƯỚC 1: Connecting database (blocking) BEFORE starting servers")
	log.Printf("[MAIN] ========================================")

	var db *gorm.DB
	var sqlDB *sql.DB
	var dbMutex sync.RWMutex
	dbReady := make(chan bool, 1)

	tempDB, err := storage.New(cfg)
	if err != nil {
		log.Fatalf("[MAIN] ❌ Failed to connect to database: %v", err)
	}
	log.Printf("[MAIN] ✅ Database connection established successfully")

	tempSQLDB, err := tempDB.DB()
	if err != nil {
		log.Fatalf("[MAIN] ❌ Failed to get underlying sql.DB: %v", err)
	}

	log.Printf("[MAIN] ========================================")
	log.Printf("[MAIN] Starting database migrations...")
	log.Printf("[MAIN] ========================================")
	if err := storage.Migrate(tempDB); err != nil {
		log.Fatalf("[MAIN] ❌ Failed to run migrations: %v", err)
	}
	log.Printf("[MAIN] ✅ Database migrations completed successfully")

	if err := migrations.RunPostMigrations(tempDB); err != nil {
		log.Printf("[MAIN] ⚠️  Warning: Failed to run post-migrations: %v", err)
	}

	dbMutex.Lock()
	db = tempDB
	sqlDB = tempSQLDB
	dbMutex.Unlock()

	log.Printf("[MAIN] ✅ Database is now available for use")
	dbReady <- true
	
	// Set up cleanup for database (will be set when connection is established)
	defer func() {
		dbMutex.RLock()
		if sqlDB != nil {
			sqlDB.Close()
		}
		dbMutex.RUnlock()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize NATS client with retry logic
	// NATS connection failures are non-fatal for initial startup
	// Core can continue without NATS, but some features will be limited
	log.Printf("[MAIN] Initializing NATS client...")
	natsClient, err := messaging.NewNATSClient(cfg.NATSEndpoint)
	var js nats.JetStreamContext
	if err != nil {
		log.Printf("⚠️  WARNING: Failed to connect to NATS: %v", err)
		log.Printf("⚠️  WARNING: Core will continue without NATS, but messaging features will be unavailable")
		log.Printf("⚠️  WARNING: This may be due to NATS storage issues - check NATS pod and PVC")
		// Don't fatal - allow Core to start without NATS for now
		// natsClient will be nil, handlers should check for nil before use
		natsClient = nil
		js = nil
	} else {
		log.Printf("[MAIN] NATS client initialized successfully")
		defer natsClient.Close()
		js = natsClient.JetStream()
	}

	// Initialize worker pool (only if NATS is available)
	var workerPool *worker.Pool
	if js != nil {
		workerPool, err = worker.NewPool(js, 5) // 5 concurrent workers per worker type
		if err != nil {
			log.Printf("Warning: Failed to create worker pool with DLQ: %v. Continuing without DLQ.", err)
			// Create pool without DLQ if setup fails
			workerPool, err = worker.NewPool(js, 5)
			if err != nil {
				log.Printf("⚠️  WARNING: Failed to create worker pool: %v. Continuing without worker pool.", err)
				workerPool = nil
			}
		}
	} else {
		log.Printf("⚠️  WARNING: Skipping worker pool initialization (NATS unavailable)")
		workerPool = nil
	}
	if workerPool != nil && js != nil {
		log.Printf("[Main] ========================================")
		log.Printf("[Main] About to add workers to pool...")
		log.Printf("[Main] WorkerPool check: workerPool == nil: %v", workerPool == nil)
		log.Printf("[Main] Adding workers to pool...")
		// NormalizerWorker removed - normalization done in handlers
		// workerPool.AddWorker(worker.NewNormalizerWorker(js, db))
		log.Printf("[Main] ✅ NormalizerWorker skipped (handled in handlers)")
		workerPool.AddWorker(worker.NewCorrelatorWorker(js, db))
		log.Printf("[Main] ✅ Added CorrelatorWorker")
		workerPool.AddWorker(worker.NewRiskWorker(js, db)) // Add Risk Engine worker
		log.Printf("[Main] ✅ Added RiskWorker")
		log.Printf("[Main] ✅ Added 3 workers to pool (normalizer, correlator, risk)")
		log.Printf("[Main] ========================================")
	} else {
		log.Printf("[Main] ⚠️  WARNING: Skipping worker pool setup (NATS unavailable)")
	}

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

	// Policy Evaluator will be initialized when database is ready
	var policyEvaluator *policy.Evaluator
	var policyWorker *policy.PolicyWorker
	
	dbMutex.RLock()
	dbReadyNow := db != nil
	dbMutex.RUnlock()
	
	if dbReadyNow {
		log.Printf("[Main] Calling policy.NewEvaluator(db)...")
		policyEvaluator, err = policy.NewEvaluator(db)
		if err != nil {
			log.Printf("[Main] ⚠️  WARNING: Failed to create policy evaluator: %v", err)
			log.Printf("[Main] ⚠️  WARNING: Policy features will be unavailable, but Core will continue")
			log.Printf("[Main] ⚠️  WARNING: This may be due to missing policy_templates table - check migrations")
			policyEvaluator = nil
		} else {
			log.Printf("[Main] ✅ Policy Evaluator initialized successfully")
			log.Printf("[Main] Policy Evaluator pointer: %p", policyEvaluator)
		}
		
		// Add Policy Worker to worker pool (for slow path processing)
		log.Printf("[Main] Creating Policy Worker...")
		policyWorker = policy.NewPolicyWorker(db, policyEvaluator)
		log.Printf("[Main] ✅ Policy Worker created")
	} else {
		log.Printf("[Main] ⚠️  WARNING: Database not ready, Policy Evaluator will be initialized when DB is available")
		// Initialize policy evaluator when database becomes available
		go func() {
			select {
			case <-dbReady:
				dbMutex.RLock()
				currentDB := db
				dbMutex.RUnlock()
				if currentDB != nil {
					log.Printf("[Main] [Background] Database ready, initializing Policy Evaluator...")
					pe, err := policy.NewEvaluator(currentDB)
					if err != nil {
						log.Printf("[Main] [Background] ⚠️  WARNING: Failed to create policy evaluator: %v", err)
					} else {
						policyEvaluator = pe
						policyWorker = policy.NewPolicyWorker(currentDB, policyEvaluator)
						log.Printf("[Main] [Background] ✅ Policy Evaluator initialized")
					}
				}
			case <-time.After(5 * time.Minute):
				log.Printf("[Main] [Background] Database connection timeout, Policy Evaluator not initialized")
			}
		}()
	}
	log.Printf("[Main] ========================================")
	// Subscribe to violation events (only if NATS is available)
	if js != nil {
		sub, err := js.Subscribe("fortuna.policy.violation.detected", func(msg *nats.Msg) {
			ctx := context.Background()
			if err := policyWorker.ProcessViolationEvent(ctx, msg.Data); err != nil {
				log.Printf("[PolicyWorker] Failed to process violation event: %v", err)
			}
			msg.Ack()
		}, nats.Durable("fortuna-policy-worker"))
		if err != nil {
			log.Printf("[Main] Warning: Failed to subscribe to policy violation events: %v", err)
		} else {
			log.Printf("[Main] ✅ Policy Worker subscribed to violation events")
			defer sub.Unsubscribe()
		}
	} else {
		log.Printf("[Main] ⚠️  WARNING: Skipping policy violation subscription (NATS unavailable)")
	}

	if workerPool != nil {
		if err := workerPool.Start(); err != nil {
			log.Printf("⚠️  WARNING: Failed to start worker pool: %v. Continuing without worker pool.", err)
		} else {
			defer workerPool.Stop()
		}
	} else {
		log.Printf("[Main] ⚠️  WARNING: Worker pool is nil, skipping start")
	}

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
	// SBOM/CVE pipeline (only if NATS is available)
	if js != nil && natsClient != nil {
		useDurables := strings.EqualFold(strings.TrimSpace(os.Getenv("FORTUNA_JS_DURABLES")), "true")
		sbomDurable := "sbom-worker"
		cveDurable := "cve-matcher-worker"

		// db may be nil initially - worker will handle it gracefully
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

		// db may be nil initially - worker will handle it gracefully
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
	} else {
		log.Printf("[Main] ⚠️  WARNING: Skipping SBOM/CVE pipeline setup (NATS unavailable)")
	}

	// Start queue depth monitoring (only if worker pool is available)
	if workerPool != nil {
		workerPool.StartQueueDepthMonitoring(ctx)
		log.Printf("Queue depth monitoring started for all workers")
	} else {
		log.Printf("[Main] ⚠️  WARNING: Skipping queue depth monitoring (worker pool unavailable)")
	}

	// Start background jobs (only if database is ready)
	// These will be started after database connection is established
	if db != nil {
		// Start risk evaluation scheduler (runs every 6 hours)
		riskScheduler := scheduler.NewRiskScheduler(db, 6*time.Hour)
		riskScheduler.Start()
		defer riskScheduler.Stop()
		log.Printf("Risk evaluation scheduler started (interval: 6 hours)")

		// Start PCE scheduler if enabled
		if cfg.PCESchedulerEnabled {
			pceScheduler := scheduler.NewPCEScheduler(db, cfg.PCESchedulerInterval)
			pceScheduler.Start()
			defer pceScheduler.Stop()
			log.Printf("PCE scheduler started (interval: %s)", cfg.PCESchedulerInterval)
		} else {
			log.Printf("[Main] PCE scheduler disabled via config")
		}

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

		// Start SBOM reconciliation loop (runs every hour)
		// OPTIMIZATION: Automatically detects missing/orphaned SBOMs and reconciles state
		sbomReconciler := reconciler.NewSBOMReconciler(db, 1*time.Hour)
		go func() {
			log.Printf("[Main] Starting SBOM reconciliation loop in goroutine...")
			sbomReconciler.Start(ctx) // This blocks forever, so must be in goroutine
		}()
		log.Printf("SBOM reconciliation loop started (interval: 1 hour)")
	} else {
		log.Printf("[Main] ⚠️  WARNING: Skipping background jobs (database not ready)")
		// Start background jobs when database becomes available
		go func() {
			select {
			case <-dbReady:
				log.Printf("[Main] Database connection established, starting background jobs...")
				// Start jobs here when db is ready
			case <-time.After(5 * time.Minute):
				log.Printf("[Main] Database connection still not ready after 5 minutes")
			}
		}()
	}

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
	// /healthz: Liveness probe (process alive)
	// /ready: Readiness probe (can accept requests, does NOT check DB/NATS)
	// /live: Alias for liveness (backward compatibility)
	// /status: Full status check (includes DB, NATS - for observability only)
	router.GET("/healthz", health.LivenessCheck())
	router.GET("/health", health.HealthCheck(db)) // Legacy endpoint
	router.GET("/health/dashboard-data-integrity", api.DashboardDataIntegrity(db)) // Dashboard data traceability
	router.GET("/ready", health.ReadinessCheck(db, cfg.GRPCPort)) // Readiness: HTTP + gRPC listening (avoids Agent "connection refused" on 9090)
	router.GET("/live", health.LivenessCheck()) // Alias for /healthz
	router.GET("/status", health.StatusCheck(db)) // Full status: includes DB, NATS

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
	var certManager *security.CertManager
	if grpcServer != nil {
		certManager = grpcServer.GetCertManager()
		if certManager != nil {
			log.Printf("[Main] ✅ CertManager available - certificate routes will be registered")
		} else {
			log.Printf("[Main] ⚠️  CertManager is nil - certificate routes will NOT be registered")
		}
	} else {
		log.Printf("[Main] ⚠️  gRPC server is nil - certificate routes will NOT be registered")
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
				log.Printf("[Webhook] ⚠️  Failed to start webhook HTTPS server: %v (non-fatal, continuing)", err)
				// Don't exit - webhook is optional, main HTTP server can still run
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
			log.Printf("[Main] ⚠️  WARNING: HTTP server stopped: %v (non-fatal)", err)
		}
	}()
	log.Printf("[Main] ✅ HTTP server goroutine launched (non-blocking)")

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

	if grpcServer != nil {
		grpcServer.Stop()
	}
}
