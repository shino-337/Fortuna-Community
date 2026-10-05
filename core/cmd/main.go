package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fortuna/core/internal/api"
	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/internal/grpc"
	"github.com/fortuna/core/internal/health"
	"github.com/fortuna/core/internal/ingest"
	internalmetrics "github.com/fortuna/core/internal/metrics"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/internal/scheduler"
	"github.com/fortuna/core/internal/storage"
	"github.com/fortuna/core/internal/webhook"
	"github.com/fortuna/core/migrations"
	cvedb "github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/kev"
	malwarePkg "github.com/fortuna/core/pkg/malware"
	"github.com/fortuna/core/pkg/messaging"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/mutations"
	"github.com/fortuna/core/pkg/policy"
	"github.com/fortuna/core/pkg/reconciler"
	"github.com/fortuna/core/pkg/riskengine"
	"github.com/fortuna/core/pkg/security"
	"github.com/fortuna/core/pkg/worker"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"

	_ "github.com/fortuna/core/pkg/metrics" // Import to register admission metrics
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

	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

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

	// Step 1: Ensure database is available BEFORE starting gRPC/HTTP servers
	//
	// The system relies on DB-backed handlers (gRPC SBOM ingestion, REST APIs). Starting servers
	// with a nil DB causes permanent "database not available" behavior because handlers capture
	// the initial nil pointer.
	log.Printf("[MAIN] ========================================")
	log.Printf("[MAIN] Step 1: Connecting database (blocking) BEFORE starting servers")
	log.Printf("[MAIN] ========================================")

	db, err := storage.New(cfg)
	if err != nil {
		log.Fatalf("[MAIN] ❌ Failed to connect to database: %v", err)
	}
	log.Printf("[MAIN] ✅ Database connection established successfully")

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("[MAIN] ❌ Failed to get underlying sql.DB: %v", err)
	}

	log.Printf("[MAIN] ========================================")
	log.Printf("[MAIN] Starting database migrations...")
	log.Printf("[MAIN] ========================================")
	if err := storage.Migrate(db); err != nil {
		log.Fatalf("[MAIN] ❌ Failed to run migrations: %v", err)
	}
	log.Printf("[MAIN] ✅ Database migrations completed successfully")

	sc := api.NewSchemaCache(db)
	sc.Warm()

	if err := migrations.RunPostMigrations(db); err != nil {
		log.Printf("[MAIN] ⚠️  Warning: Failed to run post-migrations: %v", err)
	}

	if err := riskengine.RunMalwareInsightMaintenance(db); err != nil {
		log.Printf("[MAIN] ⚠️  supply_chain_malware insight maintenance: %v", err)
	} else {
		log.Printf("[MAIN] ✅ Malware insight maintenance done (backfill + reactivate if match still present)")
	}

	if n, err := riskengine.SeedRiskRulesFromExportDir(db); err != nil {
		log.Printf("[MAIN] ⚠️  Risk rules seed from folder failed: %v", err)
	} else if n > 0 {
		log.Printf("[MAIN] ✅ Seeded %d risk rules from %s (DB was empty)", n, riskengine.GetRiskRulesExportDir())
	}
	bootstrapVulnCatalog(db)

	log.Printf("[MAIN] ✅ Database is now available for use")
	defer sqlDB.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go mutations.Start(ctx, db)
	defer cancel()

	if kev.Enabled() {
		go kev.StartBackgroundRefresh(ctx, kev.DefaultCatalog())
		log.Printf("[Main] ✅ CISA KEV catalog background refresh enabled (FORTUNA_KEV_ENABLED)")
	}

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

	// Admission policy evaluator (fast path) and the worker that records violations (slow path).
	policyEvaluator, err := policy.NewEvaluator(db)
	if err != nil {
		log.Printf("[Main] ⚠️  WARNING: Failed to create policy evaluator: %v (admission policies disabled)", err)
		policyEvaluator = nil
	} else {
		log.Printf("[Main] ✅ Policy Evaluator initialized")
	}
	policyWorker := policy.NewPolicyWorker(db, policyEvaluator)

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

	// ============================================================
	// SBOM/CVE pipeline (event-driven, non-duplicated consumption)
	// ============================================================
	// One CVE matcher consumer per Core instance, so expensive matching is not duplicated.
	//
	// IMPORTANT (dev/e2e): Using JetStream *push* durables via js.Subscribe is fragile across fast rollouts:
	// the durable consumer retains its deliver subject (inbox) and subsequent restarts can fail with:
	// "consumer is already bound to a subscription".
	// For now we run these consumers as *ephemeral* by default, and only enable durables when explicitly requested.
	// SBOM/CVE pipeline (only if NATS is available)
	if js != nil && natsClient != nil {
		useDurables := strings.EqualFold(strings.TrimSpace(os.Getenv("FORTUNA_JS_DURABLES")), "true")
		cveDurable := "cve-matcher-worker"

		// Insight updates are fanned out over core NATS to the Risk Center WebSocket.
		var cvePublishInsights worker.PublishInsightsUpdatedFunc
		if natsClient != nil {
			cvePublishInsights = func(data []byte) error {
				return natsClient.PublishCore(worker.SubjectInsightsUpdated, data)
			}
		}
		cveWorker := worker.NewCVEMatcherWorker(js, db, cvePublishInsights)
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

		// SBOM_CREATED DLQ: events that failed primary publish after retries (handler_sbom.go).
		sbomDLQ := worker.NewSBOMDLQWorker(js)
		dlqDurable := "sbom-created-dlq"
		dlqOpts := []nats.SubOpt{
			nats.ManualAck(),
			nats.DeliverAll(),
			nats.MaxAckPending(50),
			nats.AckWait(2 * time.Minute),
		}
		if useDurables {
			dlqOpts = append(dlqOpts, nats.Durable(dlqDurable))
		}
		dlqSub, err := js.Subscribe(sbomDLQ.Subject(), func(msg *nats.Msg) {
			if err := sbomDLQ.Process(ctx, msg); err != nil {
				log.Printf("[SBOMDLQWorker] Error: %v", err)
				return
			}
			msg.Ack()
		}, dlqOpts...)
		if err != nil {
			log.Printf("[Main] Warning: Failed to subscribe SBOM DLQ worker: %v", err)
		} else {
			log.Printf("[Main] ✅ SBOMDLQWorker subscribed to %s (dead-letter visibility)", sbomDLQ.Subject())
			defer dlqSub.Unsubscribe()
		}

		if pollEvery := worker.SBOMDLQDepthPollInterval(); pollEvery > 0 {
			go worker.RunSBOMDLQStreamDepthPoller(ctx, js, pollEvery, log.Default())
			log.Printf("[Main] ✅ SBOM DLQ JetStream depth gauge poll interval=%v (FORTUNA_SBOM_DLQ_DEPTH_POLL_INTERVAL)", pollEvery)
		}

		// Risk Center (Finding #1.3): subscribe via core NATS so every Core replica receives the message and broadcasts to its local WS clients (fan-out).
		if natsClient != nil {
			nc := natsClient.Conn()
			if nc != nil {
				insightsSub, err := nc.Subscribe(worker.SubjectInsightsUpdated, func(msg *nats.Msg) {
					var payload api.RisksUpdatePayload
					if len(msg.Data) > 0 && json.Valid(msg.Data) {
						_ = json.Unmarshal(msg.Data, &payload)
					}
					api.BroadcastRisksUpdateWithPayload(&payload)
				})
				if err != nil {
					log.Printf("[Main] Warning: Failed to subscribe to %s (core NATS): %v", worker.SubjectInsightsUpdated, err)
				} else {
					log.Printf("[Main] ✅ Subscribed to %s (Risk Center broadcast, core NATS fan-out)", worker.SubjectInsightsUpdated)
					defer insightsSub.Unsubscribe()
				}
			}
		}
	} else {
		log.Printf("[Main] ⚠️  WARNING: Skipping JetStream SBOM/CVE consumers (NATS unavailable). SBOM gRPC ingest will still run CVE matching in-process after each upsert.")
	}

	// Background jobs
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

	// PCE cleanup job: delete stale pod_capabilities (Phase 3, env PCE_CLEANUP_RETENTION_DAYS)
	pceCleanupJob := scheduler.NewPCECleanupJob(db)
	go func() {
		log.Printf("[Main] Starting PCE cleanup job in goroutine...")
		pceCleanupJob.Start()
	}()
	defer pceCleanupJob.Stop()
	log.Printf("PCE cleanup job started (interval: 24 hours)")

	// Pod network connections retention (bucket_5m; env POD_NETWORK_*)
	podNetRetention := scheduler.NewPodNetworkRetentionJob(db)
	go func() {
		log.Printf("[Main] Starting pod network retention job in goroutine...")
		podNetRetention.Start()
	}()
	defer podNetRetention.Stop()
	log.Printf("Pod network retention job started (see POD_NETWORK_RETENTION_HOURS / POD_NETWORK_CLEANUP_INTERVAL)")

	// Pod process snapshots retention (env POD_PROCESS_*)
	podProcRetention := scheduler.NewPodProcessRetentionJob(db)
	go podProcRetention.Start()
	defer podProcRetention.Stop()

	// Start SBOM reconciliation loop (runs every hour)
	// OPTIMIZATION: Automatically detects missing/orphaned SBOMs and reconciles state
	sbomReconciler := reconciler.NewSBOMReconciler(db, 1*time.Hour)
	go func() {
		log.Printf("[Main] Starting SBOM reconciliation loop in goroutine...")
		sbomReconciler.Start(ctx) // This blocks forever, so must be in goroutine
	}()
	log.Printf("SBOM reconciliation loop started (interval: 1 hour)")

	// Layer 3 reconciliation: periodically rebuild attack paths to heal drift.
	attackPathInterval := scheduler.AttackPathReconcileIntervalFromEnv()
	attackPathReconcileJob := scheduler.NewAttackPathReconcileJob(db, attackPathInterval)
	go func() {
		log.Printf("[Main] Starting attack path reconcile job in goroutine...")
		attackPathReconcileJob.Start()
	}()
	defer attackPathReconcileJob.Stop()
	log.Printf("Attack path reconcile job started (interval: %s)", attackPathInterval)

	// Periodic V3 risk_scores backfill: all pods + insight-bearing resources (see RISK_SCORE_V3_BACKFILL_* env).
	if scheduler.RiskScoreV3BackfillEnabled() {
		v3BackfillInterval := scheduler.RiskScoreV3BackfillIntervalFromEnv()
		v3ItemDelay := scheduler.RiskScoreV3BackfillItemDelay()
		v3BackfillJob := scheduler.NewRiskScoreV3BackfillJob(db, v3BackfillInterval, v3ItemDelay, 2*time.Minute)
		go func() {
			log.Printf("[Main] Starting risk score V3 cluster backfill job in goroutine...")
			v3BackfillJob.Start()
		}()
		defer v3BackfillJob.Stop()
		log.Printf("[Main] Risk score V3 backfill job started (interval=%s item_delay=%s, first run after 2m)", v3BackfillInterval, v3ItemDelay)
	} else {
		log.Printf("[Main] Risk score V3 backfill job disabled (RISK_SCORE_V3_BACKFILL_ENABLED=false)")
	}

	// Start Aikido malware feed sync (runs every 6 hours)
	go func() {
		aikidoSyncer := malwarePkg.NewAikidoSyncer(db)
		log.Printf("[Main] Starting Aikido malware feed initial sync...")
		results, err := aikidoSyncer.SyncAll(ctx)
		if err != nil {
			log.Printf("[Main] Aikido malware initial sync error: %v", err)
		}
		for _, r := range results {
			log.Printf("[Main] Aikido sync %s: total=%d upserted=%d errors=%d duration=%v",
				r.Source, r.Total, r.Upserted, r.Errors, r.Duration)
		}

		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Printf("[Main] Running periodic Aikido malware sync...")
				results, err := aikidoSyncer.SyncAll(ctx)
				if err != nil {
					log.Printf("[Main] Aikido periodic sync error: %v", err)
				}
				for _, r := range results {
					log.Printf("[Main] Aikido sync %s: total=%d upserted=%d", r.Source, r.Total, r.Upserted)
				}
			}
		}
	}()
	log.Printf("[Main] Aikido malware feed sync enabled (interval: 6 hours)")

	// Per-cluster rate limiter for sync and SBOM ingest (Finding #6)
	clusterLimiter := ingest.NewClusterRateLimiter(ingest.ClusterLimitConfig{
		SyncRPS:   cfg.RateLimitSyncPerClusterRPS,
		SyncBurst: cfg.RateLimitSyncPerClusterBurst,
		SBOMRPS:   cfg.RateLimitSBOMPerClusterRPS,
		SBOMBurst: cfg.RateLimitSBOMPerClusterBurst,
		Enabled:   cfg.RateLimitPerClusterEnabled,
	})
	log.Printf("[Main] Per-cluster rate limit: enabled=%v sync_rps=%.0f sbom_rps=%.0f", cfg.RateLimitPerClusterEnabled, cfg.RateLimitSyncPerClusterRPS, cfg.RateLimitSBOMPerClusterRPS)

	// Initialize gRPC server
	log.Printf("[Main] Creating gRPC server with TLS_ENABLED=%v", cfg.TLSEnabled)
	grpcServer, err := grpc.NewServer(cfg, db, natsClient, clusterLimiter)
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

	// Only trust forwarding headers from configured proxies; otherwise ClientIP()
	// (rate limits, audit IPs) would come from attacker-controlled headers.
	if err := router.SetTrustedProxies(middleware.TrustedProxiesFromEnv()); err != nil {
		log.Fatalf("[Main] invalid FORTUNA_TRUSTED_PROXIES: %v", err)
	}

	// Add recovery middleware (from gin.Default())
	router.Use(gin.Recovery())

	// Access log with ?token= values redacted (WebSocket auth must not reach logs).
	router.Use(middleware.RedactingLogger())

	router.Use(middleware.MaxRequestBody(middleware.MaxRequestBodyBytesFromEnv()))

	// Add middleware - CORS must be first to handle preflight
	router.Use(middleware.CORS())
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.RateLimiting())
	// Sanitize 5xx responses so internal error details are not sent to clients (log server-side only)
	router.Use(middleware.ErrorSanitize())

	// Health endpoints (no auth required)
	// /healthz: Liveness probe (process alive)
	// /ready: Readiness probe (can accept requests, does NOT check DB/NATS)
	// /live: Alias for liveness (backward compatibility)
	// /status: Full status check (includes DB, NATS - for observability only)
	router.GET("/healthz", health.LivenessCheck())
	router.GET("/health", health.HealthCheck(db))                 // Legacy endpoint
	router.GET("/ready", health.ReadinessCheck(db, cfg.GRPCPort)) // Readiness: HTTP + gRPC listening (avoids Agent "connection refused" on 9090)
	router.GET("/live", health.LivenessCheck())                   // Alias for /healthz
	router.GET("/status", health.StatusCheck(db))                 // Full status: includes DB, NATS

	// Phase 2.7: Initialize Admission Webhook
	log.Printf("[Main] ========================================")
	log.Printf("[Main] Phase 2.7: Setting up admission webhook...")
	log.Printf("[Main] Policy Evaluator check: policyEvaluator == nil: %v", policyEvaluator == nil)
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
	// Finding #1.4: shared dedup for pod detail ingest (X-Idempotency-Key → NATS KV)
	if natsClient != nil {
		api.SetPodDetailDedupChecker(natsClient)
	}
	api.SetupRoutesWithCertManager(router, db, cfg, certManager, clusterLimiter)

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
			Addr:              ":8443",
			Handler:           webhookRouter,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
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
		log.Printf("[Webhook] Admission webhook is disabled; HTTPS is required.")
		log.Printf("[Webhook] Set WEBHOOK_TLS_CERT_PATH and WEBHOOK_TLS_KEY_PATH")
		log.Printf("[Webhook] ========================================")
	}

	// REST API HTTP server (port 8080)
	// No Read/WriteTimeout: WebSocket streams are long-lived and manage their own
	// deadlines. Header and idle timeouts still bound slow or abandoned clients.
	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("[Main] Starting HTTP server on port %s (REST API)", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Main] ⚠️  WARNING: HTTP server stopped: %v (non-fatal)", err)
		}
	}()
	log.Printf("[Main] ✅ HTTP server goroutine launched (non-blocking)")

	// Prometheus endpoint on its own listener (disabled unless FORTUNA_METRICS_ADDR is set).
	var metricsServer *http.Server
	if addr := internalmetrics.AddrFromEnv(); addr != "" {
		metricsServer = internalmetrics.NewServer(addr)
		go func() {
			log.Printf("[Main] Serving Prometheus metrics on %s/metrics", addr)
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("[Main] ⚠️  WARNING: metrics server stopped: %v (non-fatal)", err)
			}
		}()
	}

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

	if metricsServer != nil {
		if err := metricsServer.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down metrics server: %v", err)
		}
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

// bootstrapVulnCatalog loads OSV JSON from FORTUNA_OSV_SOURCE_DIR when the mirror or legacy catalog is empty.
// Previously this only ran when package_vulnerabilities was empty; supplemental rows can populate PV while osv_packages
// stays empty, so we also bootstrap when osv_packages exists and has zero rows.
func bootstrapVulnCatalog(db *gorm.DB) {
	if db == nil {
		return
	}
	sourceDir := strings.TrimSpace(os.Getenv("FORTUNA_OSV_SOURCE_DIR"))
	if sourceDir == "" {
		log.Printf("[MAIN] FORTUNA_OSV_SOURCE_DIR not set; skip OSV mirror bootstrap (set it to a directory of OSV *.json to autoload on startup)")
		return
	}
	if st, err := os.Stat(sourceDir); err != nil || !st.IsDir() {
		log.Printf("[MAIN] OSV source dir missing or not a directory (%q); skip bootstrap: %v", sourceDir, err)
		return
	}

	var pvCount int64
	if err := db.Model(&models.PackageVulnerability{}).Where("deleted_at IS NULL").Count(&pvCount).Error; err != nil {
		log.Printf("[MAIN] ⚠️  Unable to count package_vulnerabilities: %v", err)
	}

	var osvPkgCount int64
	osvTableReady := db.Migrator().HasTable("osv_packages")
	var osvCountErr error
	if osvTableReady {
		osvCountErr = db.Model(&models.OSVPackage{}).Count(&osvPkgCount).Error
		if osvCountErr != nil {
			log.Printf("[MAIN] ⚠️  Unable to count osv_packages: %v", osvCountErr)
		}
	}
	// Treat missing table, count error, or zero rows as "mirror not loaded" so we retry OSV ingest on startup.
	osvMirrorEmpty := !osvTableReady || osvCountErr != nil || osvPkgCount == 0

	catalogEmpty := pvCount == 0

	if !catalogEmpty && !osvMirrorEmpty {
		log.Printf("[MAIN] OSV mirror populated (osv_packages=%d) and package_vulnerabilities=%d; skip OSV bootstrap", osvPkgCount, pvCount)
		return
	}
	// Avoid re-reading every OSV JSON on each restart when mirror is already filled (PV may stay empty if only osv_* is used).
	if catalogEmpty && !osvMirrorEmpty {
		log.Printf("[MAIN] package_vulnerabilities empty but OSV mirror has %d osv_packages rows; skip OSV bootstrap on this boot", osvPkgCount)
		return
	}

	if catalogEmpty && osvMirrorEmpty {
		log.Printf("[MAIN] Fresh DB: package_vulnerabilities=0 and OSV mirror empty; bootstrapping OSV from %q ...", sourceDir)
	} else if !catalogEmpty && osvMirrorEmpty {
		log.Printf("[MAIN] OSV mirror empty (osv_packages=%d) while package_vulnerabilities=%d — ingesting OSV from %q ...", osvPkgCount, pvCount, sourceDir)
	}

	manager := cvedb.NewPostgresManager(db)
	if err := manager.UpdateDatabase(context.Background()); err != nil {
		log.Printf("[MAIN] ⚠️  OSV bootstrap UpdateDatabase failed: %v", err)
		return
	}

	var afterOSV int64
	if osvTableReady {
		if err := db.Model(&models.OSVPackage{}).Count(&afterOSV).Error; err != nil {
			log.Printf("[MAIN] ⚠️  Unable to recount osv_packages after bootstrap: %v", err)
		} else {
			log.Printf("[MAIN] ✅ OSV bootstrap finished (osv_packages=%d)", afterOSV)
		}
	}
	if err := db.Model(&models.PackageVulnerability{}).Where("deleted_at IS NULL").Count(&pvCount).Error; err != nil {
		log.Printf("[MAIN] ⚠️  Unable to recount package_vulnerabilities after bootstrap: %v", err)
		return
	}
	log.Printf("[MAIN] ✅ Catalog state after bootstrap: package_vulnerabilities=%d", pvCount)
}
