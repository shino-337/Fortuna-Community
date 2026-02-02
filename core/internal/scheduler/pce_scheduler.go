package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/fortuna/core/pkg/capability"
	"gorm.io/gorm"
)

// PCEScheduler schedules periodic PodCapabilityEngine evaluation
type PCEScheduler struct {
	db       *gorm.DB
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	stopChan chan struct{}
}

// NewPCEScheduler creates a new PCE scheduler
func NewPCEScheduler(db *gorm.DB, interval time.Duration) *PCEScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &PCEScheduler{
		db:       db,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
		stopChan: make(chan struct{}),
	}
}

// Start starts the scheduler
func (s *PCEScheduler) Start() {
	log.Printf("[PCEScheduler] Starting PCE scheduler with interval %v", s.interval)

	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		// Run immediately on start
		s.runEvaluation()

		for {
			select {
			case <-s.ctx.Done():
				log.Printf("[PCEScheduler] Scheduler stopped")
				return
			case <-ticker.C:
				s.runEvaluation()
			case <-s.stopChan:
				log.Printf("[PCEScheduler] Scheduler stopped via stop channel")
				return
			}
		}
	}()
}

// Stop stops the scheduler
func (s *PCEScheduler) Stop() {
	log.Printf("[PCEScheduler] Stopping scheduler...")
	s.cancel()
	close(s.stopChan)
}

func (s *PCEScheduler) runEvaluation() {
	log.Printf("[PCEScheduler] Starting scheduled PCE evaluation...")
	startTime := time.Now()
	if err := capability.EvaluateAllPods(s.ctx, s.db); err != nil {
		log.Printf("[PCEScheduler] Error during PCE evaluation: %v", err)
		return
	}
	log.Printf("[PCEScheduler] PCE evaluation completed in %v", time.Since(startTime))
}
