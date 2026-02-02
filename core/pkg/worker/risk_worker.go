package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/fortuna/core/pkg/riskengine"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
)

// RiskWorker evaluates risks and creates insights
type RiskWorker struct {
	js         nats.JetStreamContext
	db         *gorm.DB
	riskEngine *riskengine.Engine
	insightMgr *riskengine.InsightManager
	yamlEngine *riskengine.YAMLEngine
	watcher    *riskengine.RuleWatcher
}

// NewRiskWorker creates a new risk worker
func NewRiskWorker(js nats.JetStreamContext, db *gorm.DB) *RiskWorker {
	log.Printf("[RiskWorker] Creating new RiskWorker...")
	// Try to create YAML engine if rules directory is configured
	rulesDir := os.Getenv("FORTUNA_RULES_DIR")
	var engine *riskengine.Engine
	var yamlEngine *riskengine.YAMLEngine
	var watcher *riskengine.RuleWatcher

	if rulesDir != "" {
		log.Printf("[RiskWorker] Attempting to create YAML engine...")
		if ye, err := riskengine.NewYAMLEngine(db, rulesDir); err == nil {
			yamlEngine = ye
			engine = ye.Engine
			log.Printf("[RiskWorker] Using YAML engine with %d rules", len(ye.GetRules()))

			// Start file watcher for hot-reload
			if w, err := riskengine.NewRuleWatcher(rulesDir, ye); err == nil {
				watcher = w
				watcher.Start()
				log.Printf("[RiskWorker] ✅ Rule hot-reload enabled")
			} else {
				log.Printf("[RiskWorker] WARNING: Failed to start rule watcher: %v", err)
				log.Printf("[RiskWorker] Hot-reload will not be available")
			}
		} else {
			log.Printf("[RiskWorker] Failed to create YAML engine: %v, using standard engine", err)
			engine = riskengine.NewEngine(db)
		}
	} else {
		engine = riskengine.NewEngine(db)
	}

	return &RiskWorker{
		js:         js,
		db:         db,
		riskEngine: engine,
		insightMgr: riskengine.NewInsightManager(db),
		yamlEngine: yamlEngine,
		watcher:    watcher,
	}
}

// Process processes a normalized message and evaluates risks
func (w *RiskWorker) Process(ctx context.Context, msg *nats.Msg) error {
	var normalizedData map[string]interface{}
	if err := json.Unmarshal(msg.Data, &normalizedData); err != nil {
		return fmt.Errorf("failed to unmarshal normalized item: %w", err)
	}

	kind, _ := normalizedData["kind"].(string)
	name, _ := normalizedData["name"].(string)
	namespace, _ := normalizedData["namespace"].(string)
	clusterID, _ := normalizedData["cluster_id"].(string)
	if clusterID == "" {
		if v := os.Getenv("DEFAULT_CLUSTER_ID"); v != "" {
			clusterID = v
		} else {
			clusterID = "unknown"
		}
	}

	// Evaluate risks for this resource (policy-based)
	insights, err := w.riskEngine.EvaluateResource(ctx, kind, normalizedData)
	if err != nil {
		log.Printf("[RiskWorker] Error evaluating risks for %s/%s: %v", namespace, name, err)
		return fmt.Errorf("failed to evaluate risks: %w", err)
	}

	// Create or update insights (use batch processing for efficiency)
	if len(insights) > 0 {
		if err := w.insightMgr.BatchCreateOrUpdateInsights(insights); err != nil {
			log.Printf("[RiskWorker] Failed to batch create/update insights: %v", err)
			// Fallback to individual processing
			createdCount := 0
			for _, insight := range insights {
				if err := w.insightMgr.CreateOrUpdateInsight(insight); err != nil {
					log.Printf("[RiskWorker] Failed to create/update insight: %v", err)
					continue
				}
				createdCount++
			}
			log.Printf("[RiskWorker] Created %d insights for %s/%s (total evaluated: %d)",
				createdCount, namespace, name, len(insights))
		} else {
			log.Printf("[RiskWorker] Batch created/updated %d insights for %s/%s",
				len(insights), namespace, name)
		}
	}

	if len(insights) == 0 {
		log.Printf("[RiskWorker] No risks found for %s/%s", namespace, name)
	}

	return nil
}

// Subject returns the NATS subject to subscribe to
func (w *RiskWorker) Subject() string {
	return "fortuna.normalized.>"
}

// Name returns the worker name
func (w *RiskWorker) Name() string {
	return "risk"
}

// Stop stops the risk worker and cleans up resources
func (w *RiskWorker) Stop() {
	if w.watcher != nil {
		w.watcher.Stop()
		log.Printf("[RiskWorker] Rule watcher stopped")
	}
	if w.insightMgr != nil {
		w.insightMgr.Stop()
		log.Printf("[RiskWorker] InsightManager stopped")
	}
}
