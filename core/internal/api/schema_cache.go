package api

import (
	"log"
	"sync"

	"gorm.io/gorm"
)

// SchemaCache caches HasTable results so request handlers avoid
// querying information_schema on every HTTP request.
// Call Warm() once after migrations complete.
type SchemaCache struct {
	mu     sync.RWMutex
	tables map[string]bool
	db     *gorm.DB
}

var globalSchemaCache *SchemaCache

func NewSchemaCache(db *gorm.DB) *SchemaCache {
	sc := &SchemaCache{db: db, tables: make(map[string]bool)}
	globalSchemaCache = sc
	return sc
}

// GetSchemaCache returns the process-wide schema cache (nil before Warm).
func GetSchemaCache() *SchemaCache {
	return globalSchemaCache
}

// Warm probes all tables that handlers frequently check.
func (sc *SchemaCache) Warm() {
	names := []string{
		"insights", "risk_scores", "agents", "pod_capabilities",
		"attack_paths", "runtime_signals", "pod_instances",
		"runtime_signal_step_mappings", "malware_matches",
		"risk_rules", "exception_policies",
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	for _, n := range names {
		sc.tables[n] = sc.db.Migrator().HasTable(n)
	}
	log.Printf("[SchemaCache] warmed %d table checks", len(sc.tables))
}

// Has returns cached HasTable result; falls back to live check on cache miss.
func (sc *SchemaCache) Has(name string) bool {
	sc.mu.RLock()
	v, ok := sc.tables[name]
	sc.mu.RUnlock()
	if ok {
		return v
	}
	live := sc.db.Migrator().HasTable(name)
	sc.mu.Lock()
	sc.tables[name] = live
	sc.mu.Unlock()
	return live
}
