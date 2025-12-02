package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ksam/core/internal/api"
	"github.com/ksam/core/internal/config"
	"github.com/ksam/core/internal/grpc"
	"github.com/ksam/core/internal/health"
	"github.com/ksam/core/internal/middleware"
	"github.com/ksam/core/internal/scheduler"
	"github.com/ksam/core/internal/storage"
	"github.com/ksam/core/migrations"
	"github.com/ksam/core/pkg/messaging"
	"github.com/ksam/core/pkg/worker"

	_ "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// CRITICAL: Use os.Stdout directly first to ensure logs appear
	fmt.Fprintf(os.Stdout, "========================================\n")
	fmt.Fprintf(os.Stdout, "[MAIN] Starting KSAM Core...\n")
	fmt.Fprintf(os.Stdout, "========================================\n")

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
	if err := storage.Migrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
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
	workerPool.AddWorker(worker.NewNormalizerWorker(js, db))
	workerPool.AddWorker(worker.NewCorrelatorWorker(js, db))
	workerPool.AddWorker(worker.NewRiskWorker(js, db)) // Add Risk Engine worker

	if err := workerPool.Start(); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}
	defer workerPool.Stop()

	// Start risk evaluation scheduler (runs every 6 hours)
	riskScheduler := scheduler.NewRiskScheduler(db, 6*time.Hour)
	riskScheduler.Start()
	defer riskScheduler.Stop()
	log.Printf("Risk evaluation scheduler started (interval: 6 hours)")

	// Initialize gRPC server
	log.Printf("[Main] Creating gRPC server with TLS_ENABLED=%v", cfg.TLSEnabled)
	grpcServer, err := grpc.NewServer(cfg, db, natsClient)
	if err != nil {
		log.Fatalf("Failed to create gRPC server: %v", err)
	}
	log.Printf("[Main] gRPC server created successfully")
	go func() {
		if err := grpcServer.Start(ctx); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	// Initialize REST API
	router := gin.Default()

	// Add middleware
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.CORS())
	router.Use(middleware.MetricsMiddleware())

	// Health endpoints (no auth required)
	router.GET("/health", health.HealthCheck(db))
	router.GET("/ready", health.ReadinessCheck(db))
	router.GET("/live", health.LivenessCheck())

	// Setup routes with certificate manager (if available)
	certManager := grpcServer.GetCertManager()
	log.Printf("========================================")
	log.Printf("[Main] CertManager Status Check")
	log.Printf("[Main]   certManager == nil: %v", certManager == nil)
	log.Printf("[Main]   TLS_ENABLED: %v", cfg.TLSEnabled)
	if certManager != nil {
		log.Printf("[Main] ✅ CertManager available - certificate routes will be registered")
	} else {
		log.Printf("[Main] ⚠️  CertManager is nil - certificate routes will NOT be registered")
		log.Printf("[Main]   This may be expected if TLS is not enabled")
	}
	log.Printf("========================================")
	api.SetupRoutesWithCertManager(router, db, cfg, certManager)

	httpServer := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
	}

	go func() {
		log.Printf("Starting HTTP server on port %s", cfg.HTTPPort)
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

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}

	grpcServer.Stop()
}
