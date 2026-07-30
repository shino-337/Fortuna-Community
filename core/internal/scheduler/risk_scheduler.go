package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/fortuna/core/pkg/worker"
	"gorm.io/gorm"
)

// RiskScheduler schedules periodic risk evaluations
type RiskScheduler struct {
	db         *gorm.DB
	interval   time.Duration
	ctx        context.Context
	cancel     context.CancelFunc
	stopChan   chan struct{}
}

// NewRiskScheduler creates a new risk scheduler
func NewRiskScheduler(db *gorm.DB, interval time.Duration) *RiskScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &RiskScheduler{
		db:       db,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
		stopChan: make(chan struct{}),
	}
}

// Start starts the scheduler
func (s *RiskScheduler) Start() {
	log.Printf("[RiskScheduler] Starting risk evaluation scheduler with interval %v", s.interval)
	
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		// Run immediately on start
		s.runEvaluation()

		for {
			select {
			case <-s.ctx.Done():
				log.Printf("[RiskScheduler] Scheduler stopped")
				return
			case <-ticker.C:
				s.runEvaluation()
			case <-s.stopChan:
				log.Printf("[RiskScheduler] Scheduler stopped via stop channel")
				return
			}
		}
	}()
}

// Stop stops the scheduler
func (s *RiskScheduler) Stop() {
	log.Printf("[RiskScheduler] Stopping scheduler...")
	s.cancel()
	close(s.stopChan)
}

// runEvaluation runs a single risk evaluation
func (s *RiskScheduler) runEvaluation() {
	log.Printf("[RiskScheduler] Starting scheduled risk evaluation...")
	startTime := time.Now()

	// Step 1: Re-evaluate all resources (creates/updates insights for existing risks)
	evaluator := worker.NewHistoricalRiskEvaluator(s.db)
	if err := evaluator.EvaluateAllResources(s.ctx); err != nil {
		log.Printf("[RiskScheduler] Error during risk evaluation: %v", err)
		return
	}

	// Step 2: Auto-resolve insights where risks no longer exist
	statusUpdater := worker.NewInsightStatusUpdater(s.db)
	if err := statusUpdater.UpdateStatusForResolvedRisks(s.ctx); err != nil {
		log.Printf("[RiskScheduler] Error updating insight status: %v", err)
		// Don't return - evaluation was successful, status update is secondary
	}

	duration := time.Since(startTime)
	log.Printf("[RiskScheduler] Risk evaluation completed in %v", duration)
}

