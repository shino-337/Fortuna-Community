package poddetail

import "strings"

// isExecToolNotFound returns true if the error indicates a CLI tool (ss, netstat, ps, sh) was not found.
// Minimal/distroless images (etcd, kube-apiserver, etc.) often lack these; we skip without spamming logs.
func isExecToolNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "executable file not found") || strings.Contains(s, "not found in $PATH")
}
