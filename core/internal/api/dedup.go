package api

import (
	"github.com/gin-gonic/gin"
)

// DedupChecker is used for shared idempotency across Core replicas (Finding #1.4).
// DedupSeen(key) returns true if key was already processed (skip), false if this replica should process.
type DedupChecker interface {
	DedupSeen(key string) (seen bool, err error)
}

var podDetailDedupChecker DedupChecker

// SetPodDetailDedupChecker sets the dedup store for pod detail ingest. Call from main when NATS is available.
func SetPodDetailDedupChecker(c DedupChecker) {
	podDetailDedupChecker = c
}

// tryDedup checks X-Idempotency-Key; if present and already seen, responds 200 and returns true. Otherwise returns false.
func tryDedup(c *gin.Context, endpoint string) bool {
	key := c.GetHeader("X-Idempotency-Key")
	if key == "" || podDetailDedupChecker == nil {
		return false
	}
	fullKey := "pod_detail:" + endpoint + ":" + key
	seen, err := podDetailDedupChecker.DedupSeen(fullKey)
	if err != nil {
		return false // fail open: no dedup on error
	}
	if seen {
		c.JSON(200, gin.H{"ok": true, "dedup": true})
		return true
	}
	return false
}
