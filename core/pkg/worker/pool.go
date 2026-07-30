package worker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/fortuna/core/pkg/metrics"
)

// Worker represents a single worker that processes messages
type Worker interface {
	Process(ctx context.Context, msg *nats.Msg) error
	Subject() string
	Name() string
}

// Pool manages a pool of workers
type Pool struct {
	workers         []Worker
	concurrency     int
	wg              sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
	js              nats.JetStreamContext
	retryConfig     RetryConfig
	dlqManager      *DLQManager
	backpressureConfig BackpressureConfig
	loadTrackers    map[string]*WorkerLoadTracker // Per-worker-type load tracking
	loadTrackersMu  sync.RWMutex
}

// NewPool creates a new worker pool
func NewPool(js nats.JetStreamContext, concurrency int) (*Pool, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Setup DLQ manager
	dlqManager, err := NewDLQManager(js, DefaultDLQConfig())
	if err != nil {
		log.Printf("[WorkerPool] Warning: Failed to setup DLQ manager: %v", err)
		// Continue without DLQ if setup fails
		dlqManager = nil
	}

	return &Pool{
		concurrency:        concurrency,
		ctx:                ctx,
		cancel:             cancel,
		js:                 js,
		retryConfig:        DefaultRetryConfig(),
		dlqManager:         dlqManager,
		backpressureConfig: DefaultBackpressureConfig(),
		loadTrackers:       make(map[string]*WorkerLoadTracker),
	}, nil
}

// SetRetryConfig sets the retry configuration
func (p *Pool) SetRetryConfig(config RetryConfig) {
	p.retryConfig = config
}

// SetBackpressureConfig sets the backpressure configuration and propagates the policy
// to any already-created load trackers.
func (p *Pool) SetBackpressureConfig(config BackpressureConfig) {
	p.backpressureConfig = config
	// Propagate the new policy to existing trackers
	p.loadTrackersMu.RLock()
	for _, tracker := range p.loadTrackers {
		tracker.SetPolicy(config.Policy)
	}
	p.loadTrackersMu.RUnlock()
}

// getOrCreateLoadTracker gets or creates a load tracker for a worker type
func (p *Pool) getOrCreateLoadTracker(workerType string) *WorkerLoadTracker {
	p.loadTrackersMu.RLock()
	tracker, exists := p.loadTrackers[workerType]
	p.loadTrackersMu.RUnlock()

	if exists {
		return tracker
	}

	// Create new tracker
	p.loadTrackersMu.Lock()
	defer p.loadTrackersMu.Unlock()

	// Double-check after acquiring write lock
	if tracker, exists := p.loadTrackers[workerType]; exists {
		return tracker
	}

	tracker = NewWorkerLoadTracker(workerType, p.backpressureConfig.MaxConcurrent)
	tracker.SetPolicy(p.backpressureConfig.Policy)
	p.loadTrackers[workerType] = tracker
	log.Printf("[WorkerPool] Created load tracker for worker type %s (max concurrent: %d, policy: %s)",
		workerType, p.backpressureConfig.MaxConcurrent, p.backpressureConfig.Policy)

	return tracker
}

// AddWorker adds a worker to the pool
func (p *Pool) AddWorker(worker Worker) {
	p.workers = append(p.workers, worker)
	// Initialize load tracker for this worker type
	p.getOrCreateLoadTracker(worker.Name())
}

// Start starts all workers in the pool
func (p *Pool) Start() error {
	for _, worker := range p.workers {
		for i := 0; i < p.concurrency; i++ {
			p.wg.Add(1)
			go p.runWorker(worker, i)
		}
	}
	log.Printf("[WorkerPool] Started %d workers with concurrency %d", len(p.workers), p.concurrency)
	return nil
}

// runWorker runs a single worker instance
func (p *Pool) runWorker(worker Worker, id int) {
	defer p.wg.Done()

	// Get load tracker for this worker type
	tracker := p.getOrCreateLoadTracker(worker.Name())

	// For WorkQueuePolicy streams, we can only have ONE consumer per stream
	// So we create only the first worker (id=0) for WorkQueuePolicy streams
	// Other workers will be skipped to avoid "filtered consumer not unique" error
	streamName := getStreamNameFromSubject(worker.Subject())
	isWorkQueueStream := streamName == "fortuna-raw" || streamName == "fortuna-normalized"
	if isWorkQueueStream && id > 0 {
		log.Printf("[WorkerPool] Skipping worker %s-%d: WorkQueuePolicy stream '%s' only allows one consumer", worker.Name(), id, streamName)
		// Wait for context cancellation without subscribing
		<-p.ctx.Done()
		return
	}
	
	// For WorkQueuePolicy streams, also ensure we don't create multiple consumers
	// by using a durable consumer name that's unique per worker type but same for all instances
	// This ensures only one consumer exists even if multiple workers try to subscribe
	var durableName string
	if isWorkQueueStream {
		// Use worker type name as durable name (same for all instances of this worker type)
		durableName = fmt.Sprintf("fortuna-%s-worker", worker.Name())
	} else {
		// For non-WorkQueue streams, use unique durable name per worker instance
		durableName = fmt.Sprintf("fortuna-%s-worker-%d", worker.Name(), id)
	}

	opts := []nats.SubOpt{
		nats.ManualAck(),
		nats.DeliverAll(),
		nats.MaxAckPending(10),
	}
	
	// For WorkQueuePolicy streams, use durable consumer to ensure only one consumer exists
	if isWorkQueueStream {
		opts = append(opts, nats.Durable(durableName))
	}

	sub, err := p.js.Subscribe(worker.Subject(), func(msg *nats.Msg) {
		start := time.Now()

		// Get delivery count from NATS metadata
		attempts := 1
		if metadata, err := msg.Metadata(); err == nil && metadata != nil {
			attempts = int(metadata.NumDelivered)
		}

		// Check backpressure before processing
		if !tracker.TryAcquire() {
			// Worker is overloaded - apply backpressure
			ApplyBackpressure(p.ctx, msg, worker.Name(), tracker)
			return // Don't process, message will be redelivered later
		}
		
		// Release slot when done (success or error)
		defer tracker.Release()

		// Process the message
		processErr := worker.Process(p.ctx, msg)

		if processErr != nil {
			// Classify error
			errorType := ClassifyError(processErr)

			// Update error metrics
			errorTypeLabel := "retryable"
			if errorType == ErrorTypeNonRetryable {
				errorTypeLabel = "non_retryable"
			} else if errorType == ErrorTypeFatal {
				errorTypeLabel = "fatal"
			}
			metrics.WorkerErrorsTotal.WithLabelValues(worker.Name(), errorTypeLabel).Inc()

			if errorType == ErrorTypeFatal {
				log.Printf("[WorkerPool] Worker %s-%d FATAL error: %v", worker.Name(), id, processErr)
				// Send to DLQ and ack to prevent infinite retry
				if p.dlqManager != nil {
					if dlqErr := p.dlqManager.SendToDLQ(p.ctx, msg, processErr, attempts, worker.Name(), nil); dlqErr != nil {
						log.Printf("[WorkerPool] Failed to send to DLQ: %v", dlqErr)
					} else {
						metrics.DLQMessagesTotal.WithLabelValues(worker.Name()).Inc()
					}
				}
				msg.Ack() // Ack to prevent infinite redelivery
				metrics.WorkerMessagesProcessedTotal.WithLabelValues(worker.Name(), "error").Inc()
				return
			}

			if errorType == ErrorTypeNonRetryable {
				log.Printf("[WorkerPool] Worker %s-%d non-retryable error: %v", worker.Name(), id, processErr)
				// Send to DLQ if enabled
				if p.dlqManager != nil {
					if dlqErr := p.dlqManager.SendToDLQ(p.ctx, msg, processErr, attempts, worker.Name(), nil); dlqErr != nil {
						log.Printf("[WorkerPool] Failed to send to DLQ: %v", dlqErr)
					} else {
						metrics.DLQMessagesTotal.WithLabelValues(worker.Name()).Inc()
					}
				}
				// Ack to prevent infinite redelivery
				msg.Ack()
				metrics.WorkerMessagesProcessedTotal.WithLabelValues(worker.Name(), "error").Inc()
				return
			}

			// Retryable error - check if we've exceeded max attempts
			if attempts >= p.retryConfig.MaxAttempts {
				log.Printf("[WorkerPool] Worker %s-%d max attempts (%d) reached: %v", worker.Name(), id, attempts, processErr)
				// Send to DLQ
				if p.dlqManager != nil {
					if dlqErr := p.dlqManager.SendToDLQ(p.ctx, msg, processErr, attempts, worker.Name(), nil); dlqErr != nil {
						log.Printf("[WorkerPool] Failed to send to DLQ: %v", dlqErr)
					} else {
						metrics.DLQMessagesTotal.WithLabelValues(worker.Name()).Inc()
					}
				}
				// Ack to prevent infinite redelivery
				msg.Ack()
				metrics.WorkerMessagesProcessedTotal.WithLabelValues(worker.Name(), "error").Inc()
				return
			}

			// Retryable error - don't ack, let NATS redeliver with backoff
			log.Printf("[WorkerPool] Worker %s-%d retryable error (attempt %d/%d): %v. Will retry via NATS redelivery.",
				worker.Name(), id, attempts, p.retryConfig.MaxAttempts, processErr)
			metrics.WorkerRetryCountTotal.WithLabelValues(worker.Name()).Inc()
			// Don't ack - NATS will redeliver
			metrics.WorkerMessagesProcessedTotal.WithLabelValues(worker.Name(), "error").Inc()
			return
		}

		// Success - ack the message
		msg.Ack()
		duration := time.Since(start)
		
		// Update metrics
		metrics.WorkerMessagesProcessedTotal.WithLabelValues(worker.Name(), "success").Inc()
		metrics.WorkerProcessingDuration.WithLabelValues(worker.Name()).Observe(duration.Seconds())
		
		if attempts > 1 {
			log.Printf("[WorkerPool] Worker %s-%d processed message in %v (after %d attempts)", worker.Name(), id, duration, attempts)
		} else {
			log.Printf("[WorkerPool] Worker %s-%d processed message in %v", worker.Name(), id, duration)
		}
	}, opts...)

	if err != nil {
		log.Printf("[WorkerPool] Failed to subscribe worker %s-%d: %v", worker.Name(), id, err)
		return
	}

	log.Printf("[WorkerPool] Worker %s-%d started, subscribed to %s", worker.Name(), id, worker.Subject())

	// Wait for context cancellation
	<-p.ctx.Done()
	sub.Unsubscribe()
	log.Printf("[WorkerPool] Worker %s-%d stopped", worker.Name(), id)
}

// Stop stops all workers gracefully
func (p *Pool) Stop() {
	log.Printf("[WorkerPool] Stopping worker pool...")
	p.cancel()
	p.wg.Wait()
	log.Printf("[WorkerPool] All workers stopped")
}

// SetMaxConcurrent sets the maximum concurrent processing for a worker type
func (p *Pool) SetMaxConcurrent(workerType string, maxConcurrent int) {
	tracker := p.getOrCreateLoadTracker(workerType)
	atomic.StoreInt32(&tracker.maxConcurrent, int32(maxConcurrent))
	log.Printf("[WorkerPool] Set max concurrent for worker %s to %d", workerType, maxConcurrent)
}

// GetLoadTracker returns the load tracker for a worker type
func (p *Pool) GetLoadTracker(workerType string) *WorkerLoadTracker {
	return p.getOrCreateLoadTracker(workerType)
}

// StartQueueDepthMonitoring starts monitoring queue depth for all workers
func (p *Pool) StartQueueDepthMonitoring(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				p.updateQueueDepthMetrics()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// updateQueueDepthMetrics updates queue depth metrics for all workers
func (p *Pool) updateQueueDepthMetrics() {
	for _, worker := range p.workers {
		// Get stream info for this worker's subject
		streamName := getStreamNameFromSubject(worker.Subject())
		if streamName == "" {
			continue
		}
		
		streamInfo, err := p.js.StreamInfo(streamName)
		if err != nil {
			// Stream might not exist yet, skip
			continue
		}
		
		// Update queue depth metric
		metrics.WorkerQueueDepth.WithLabelValues(worker.Name()).Set(float64(streamInfo.State.Msgs))
	}
}

// getStreamNameFromSubject extracts stream name from subject pattern
func getStreamNameFromSubject(subject string) string {
	// Map subject patterns to stream names (matching actual stream names in nats_client.go)
	if subject == "fortuna.raw.>" {
		return "fortuna-raw"  // Fixed: use hyphen, not underscore
	}
	if subject == "fortuna.normalized.>" {
		return "fortuna-normalized"  // Fixed: use hyphen, not underscore
	}
	// Default: try to infer from subject
	if len(subject) > 0 {
		// Remove wildcards and convert to stream name
		streamName := subject
		streamName = strings.ReplaceAll(streamName, ".>", "")
		streamName = strings.ReplaceAll(streamName, ".", "-")  // Use hyphen to match stream names
		return streamName
	}
	return ""
}
