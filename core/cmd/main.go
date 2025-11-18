package main

import (
	"context"
	"flag"
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
	"github.com/ksam/core/internal/storage"
	"github.com/ksam/core/migrations"

	_ "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Ensure standard logger writes to stdout with timestamps for easier kubectl logs viewing
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

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

	if os.Getenv("KSAM_SKIP_MIGRATIONS") == "true" {
		log.Println("Skipping database migrations due to KSAM_SKIP_MIGRATIONS=true")
	} else {
		// Run migrations
		if err := storage.Migrate(db); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}

		// Run post-migrations (create default admin, etc.)
		if err := migrations.RunPostMigrations(db); err != nil {
			log.Printf("Warning: Failed to run post-migrations: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize gRPC server
	grpcServer := grpc.NewServer(cfg, db)
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

	api.SetupRoutes(router, db, cfg)

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
