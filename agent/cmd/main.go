package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ksam/agent/internal/collector"
	"github.com/ksam/agent/internal/config"
	"github.com/ksam/agent/internal/watcher"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize collector
	collector, err := collector.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create collector: %v", err)
	}

	// Initialize watcher
	watcher, err := watcher.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}

	// Start collector
	go func() {
		ticker := time.NewTicker(cfg.SyncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := collector.Collect(ctx); err != nil {
					log.Printf("Error collecting data: %v", err)
				}
			}
		}
	}()

	// Start watcher
	go func() {
		if err := watcher.Watch(ctx); err != nil {
			log.Printf("Error watching changes: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()
}

