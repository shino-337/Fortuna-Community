package poddetail

import (
	"log"
	"os"
	"strings"
)

// isExecToolNotFound returns true if the error indicates a CLI tool (ss, netstat, ps, sh) was not found.
// Minimal/distroless images (etcd, kube-apiserver, etc.) often lack these; we skip without spamming logs.
func isExecToolNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "executable file not found") || strings.Contains(s, "not found in $PATH")
}

// logExecToolNotFoundDebug logs a single DEBUG line when a container is skipped because it does not
// provide shell utilities (distroless/minimal). Only logs when LOG_LEVEL=debug (or trace).
// Use when exec fails with isExecToolNotFound(err) to avoid error-level noise; production-grade
// collection will use host inspection instead of exec.
func logExecToolNotFoundDebug(namespace, podName, containerName, collectType string) {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug", "trace":
		log.Printf("[PodDetail] DEBUG: %s/%s/%s — container does not provide shell utilities (%s), skipping exec (fallback: host inspection when available)", namespace, podName, containerName, collectType)
	}
}
