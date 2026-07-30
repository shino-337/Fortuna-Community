package graph

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Short-lived cache so parallel dashboard calls (graph + summary + objectives + chains)
// do not each recompute full-cluster attack paths.
var attackPathBuildCache sync.Map // key: cluster cache key -> attackPathBuildCacheEntry

// attackPathBuildFlight coalesces concurrent BuildAllPaths for the same (cluster, persist) key
// so N parallel HTTP handlers share one compute pass before the TTL cache is populated.
var attackPathBuildFlight singleflight.Group

// buildAllPathsSingleflightKey isolates read builds vs reconcile (persist) builds.
func buildAllPathsSingleflightKey(clusterID string, persist bool) string {
	return fmt.Sprintf("%s|persist=%v", attackPathCacheKey(clusterID), persist)
}

type attackPathBuildCacheEntry struct {
	paths     []AttackPath
	expiresAt time.Time
}

func attackPathBuildCacheTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("FORTUNA_ATTACK_PATH_BUILD_CACHE_TTL"))
	if raw == "" {
		return 45 * time.Second
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return 45 * time.Second
	}
	return d
}

func attackPathCacheKey(clusterID string) string {
	if strings.TrimSpace(clusterID) == "" {
		return "__all_clusters__"
	}
	return strings.TrimSpace(clusterID)
}

func clearAttackPathBuildCache() {
	attackPathBuildCache.Range(func(key, _ any) bool {
		attackPathBuildCache.Delete(key)
		return true
	})
}

func getCachedAttackPathsAll(clusterID string) ([]AttackPath, bool) {
	ttl := attackPathBuildCacheTTL()
	if ttl <= 0 {
		return nil, false
	}
	key := attackPathCacheKey(clusterID)
	v, ok := attackPathBuildCache.Load(key)
	if !ok {
		return nil, false
	}
	ent := v.(attackPathBuildCacheEntry)
	if time.Now().After(ent.expiresAt) {
		attackPathBuildCache.Delete(key)
		return nil, false
	}
	return ent.paths, true
}

func setCachedAttackPathsAll(clusterID string, paths []AttackPath) {
	ttl := attackPathBuildCacheTTL()
	if ttl <= 0 {
		return
	}
	key := attackPathCacheKey(clusterID)
	attackPathBuildCache.Store(key, attackPathBuildCacheEntry{
		paths:     paths,
		expiresAt: time.Now().Add(ttl),
	})
}
