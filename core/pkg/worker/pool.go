package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// Worker represents a single worker that processes messages
type Worker interface {
	Process(ctx context.Context, msg *nats.Msg) error
	Subject() string
	Name() string
}

// Pool manages a pool of workers
type Pool struct {
	workers     []Worker
	concurrency int
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	js          nats.JetStreamContext
	retryConfig RetryConfig
	dlqManager  *DLQManager
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
		concurrency: concurrency,
		ctx:         ctx,
		cancel:      cancel,
		js:          js,
		retryConfig: DefaultRetryConfig(),
		dlqManager:  dlqManager,
	}, nil
}

// SetRetryConfig sets the retry configuration
func (p *Pool) SetRetryConfig(config RetryConfig) {
	p.retryConfig = config
}

// AddWorker adds a worker to the pool
func (p *Pool) AddWorker(worker Worker) {
	p.workers = append(p.workers, worker)
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

	sub, err := p.js.Subscribe(worker.Subject(), func(msg *nats.Msg) {
		start := time.Now()

		// Get delivery count from NATS metadata
		attempts := 1
		if metadata, err := msg.Metadata(); err == nil && metadata != nil {
			attempts = int(metadata.NumDelivered)
		}

		// Process the message
		processErr := worker.Process(p.ctx, msg)

		if processErr != nil {
			// Classify error
			errorType := ClassifyError(processErr)

			if errorType == ErrorTypeFatal {
				log.Printf("[WorkerPool] Worker %s-%d FATAL error: %v", worker.Name(), id, processErr)
				// Send to DLQ and ack to prevent infinite retry
				if p.dlqManager != nil {
					if dlqErr := p.dlqManager.SendToDLQ(p.ctx, msg, processErr, attempts, worker.Name(), nil); dlqErr != nil {
						log.Printf("[WorkerPool] Failed to send to DLQ: %v", dlqErr)
					}
				}
				msg.Ack() // Ack to prevent infinite redelivery
				return
			}

			if errorType == ErrorTypeNonRetryable {
				log.Printf("[WorkerPool] Worker %s-%d non-retryable error: %v", worker.Name(), id, processErr)
				// Send to DLQ if enabled
				if p.dlqManager != nil {
					if dlqErr := p.dlqManager.SendToDLQ(p.ctx, msg, processErr, attempts, worker.Name(), nil); dlqErr != nil {
						log.Printf("[WorkerPool] Failed to send to DLQ: %v", dlqErr)
					}
				}
				// Ack to prevent infinite redelivery
				msg.Ack()
				return
			}

			// Retryable error - check if we've exceeded max attempts
			if attempts >= p.retryConfig.MaxAttempts {
				log.Printf("[WorkerPool] Worker %s-%d max attempts (%d) reached: %v", worker.Name(), id, attempts, processErr)
				// Send to DLQ
				if p.dlqManager != nil {
					if dlqErr := p.dlqManager.SendToDLQ(p.ctx, msg, processErr, attempts, worker.Name(), nil); dlqErr != nil {
						log.Printf("[WorkerPool] Failed to send to DLQ: %v", dlqErr)
					}
				}
				// Ack to prevent infinite redelivery
				msg.Ack()
				return
			}

			// Retryable error - don't ack, let NATS redeliver with backoff
			log.Printf("[WorkerPool] Worker %s-%d retryable error (attempt %d/%d): %v. Will retry via NATS redelivery.",
				worker.Name(), id, attempts, p.retryConfig.MaxAttempts, processErr)
			// Don't ack - NATS will redeliver
			return
		}

		// Success - ack the message
		msg.Ack()
		duration := time.Since(start)
		if attempts > 1 {
			log.Printf("[WorkerPool] Worker %s-%d processed message in %v (after %d attempts)", worker.Name(), id, duration, attempts)
		} else {
			log.Printf("[WorkerPool] Worker %s-%d processed message in %v", worker.Name(), id, duration)
		}
	}, nats.Durable(fmt.Sprintf("%s-worker-%d", worker.Name(), id)), nats.ManualAck())

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
