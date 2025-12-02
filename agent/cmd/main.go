package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ksam/agent/internal/collector"
	"github.com/ksam/agent/internal/config"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize collector with new implementation
	collector, err := collector.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create collector: %v", err)
	}

	// Start collector (watchers run continuously)
	if err := collector.Start(); err != nil {
		log.Fatalf("Failed to start collector: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	collector.Stop()
}

