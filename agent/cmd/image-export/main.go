// Command fortuna-image-export is the only Agent-pod process with access to the
// containerd socket. It streams image archives to the Agent container over a
// pod-local unix socket and holds no Kubernetes or Core credentials.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/fortuna/agent/pkg/sbom/imageexport"
)

func main() {
	logger := log.New(os.Stderr, "[image-export] ", log.LstdFlags)
	socket := env("IMAGE_EXPORT_SOCKET", imageexport.DefaultSocket)
	maxBytes, err := strconv.ParseInt(env("IMAGE_EXPORT_MAX_BYTES", strconv.FormatInt(8<<30, 10)), 10, 64)
	if err != nil || maxBytes <= 0 {
		logger.Fatalf("invalid IMAGE_EXPORT_MAX_BYTES")
	}

	if err := os.Remove(socket); err != nil && !errors.Is(err, fs.ErrNotExist) {
		logger.Fatalf("remove stale socket: %v", err)
	}
	old := syscall.Umask(0o177)
	ln, err := net.Listen("unix", socket)
	syscall.Umask(old)
	if err != nil {
		logger.Fatalf("listen %s: %v", socket, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	server := &imageexport.Server{
		Exporter: imageexport.ContainerdExporter{
			Socket:    env("CONTAINERD_SOCKET", "/run/containerd/containerd.sock"),
			Namespace: env("CONTAINERD_NAMESPACE", "k8s.io"),
		},
		Timeout:     10 * time.Minute,
		MaxBytes:    maxBytes,
		Concurrency: 2,
		Logger:      logger,
	}
	logger.Printf("listening on %s", socket)
	if err := server.Serve(ctx, ln); err != nil {
		logger.Fatalf("serve: %v", err)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
