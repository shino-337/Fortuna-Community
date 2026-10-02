package api

import (
	"context"
	"github.com/fortuna/core/pkg/capability"
	"github.com/fortuna/core/pkg/worker"
	"gorm.io/gorm"
	"log"
	"sync"
	"time"
)

// These evaluators read all stored clusters. Coalesce per database, rather than
// starting two global scans for every Agent's full sync. A sync received during
// evaluation requests one subsequent pass, so newer inventory is not stranded.
var fullSyncEvaluations sync.Map

func coalescedEvaluation(run func()) func() {
	var mu sync.Mutex
	running, pending := false, false
	return func() {
		mu.Lock()
		if running {
			pending = true
			mu.Unlock()
			return
		}
		running = true
		mu.Unlock()
		go func() {
			for {
				run()
				mu.Lock()
				if pending {
					pending = false
					mu.Unlock()
					continue
				}
				running = false
				mu.Unlock()
				return
			}
		}()
	}
}

func triggerFullSyncEvaluation(db *gorm.DB) {
	trigger, _ := fullSyncEvaluations.LoadOrStore(db, coalescedEvaluation(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := worker.NewHistoricalRiskEvaluator(db).EvaluateAllResources(ctx); err != nil {
			log.Printf("[AgentAPI] Risk evaluation failed: %v", err)
		}
		if err := capability.EvaluateAllPods(ctx, db); err != nil {
			log.Printf("[AgentAPI] PCE evaluation failed: %v", err)
		}
	}))
	trigger.(func())()
}
