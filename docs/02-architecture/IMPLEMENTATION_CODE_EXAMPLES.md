# KSAM Agent-Based Architecture - Code Examples

**Date:** 2025-12-22
**Purpose:** Concrete code examples for implementing the agent-based refactoring

---

## Table of Contents
1. [Agent Enhancement Examples](#agent-enhancement-examples)
2. [Core gRPC Server Examples](#core-grpc-server-examples)
3. [Feature Flag Implementation](#feature-flag-implementation)
4. [Worker Pool Refactoring](#worker-pool-refactoring)
5. [Feedback Loop Implementation](#feedback-loop-implementation)
6. [Configuration Examples](#configuration-examples)

---

## Agent Enhancement Examples

### 1. Watcher Manager with Shared Informers

**File:** `/agent/internal/watcher/manager.go` (NEW)

```go
package watcher

import (
    "context"
    "fmt"
    "sync"
    "time"

    corev1 "k8s.io/api/core/v1"
    rbacv1 "k8s.io/api/rbac/v1"
    "k8s.io/client-go/informers"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/cache"
    "github.com/rs/zerolog/log"
)

// WatchEvent represents a Kubernetes resource change event
type WatchEvent struct {
    Type         string      // "ADDED", "MODIFIED", "DELETED"
    ResourceType string      // "pod", "serviceaccount", "role", etc.
    Resource     interface{} // The actual K8s resource
    Timestamp    time.Time
    NodeID       string      // For node-local filtering
    AgentID      string
}

// WatcherManager coordinates all resource watchers using shared informers
type WatcherManager struct {
    clientset      kubernetes.Interface
    factory        informers.SharedInformerFactory
    nodeID         string
    agentID        string
    eventChan      chan *WatchEvent
    batcher        *EventBatcher
    stopChan       chan struct{}
    wg             sync.WaitGroup

    // Watchers
    podWatcher     cache.SharedIndexInformer
    saWatcher      cache.SharedIndexInformer
    rbacWatcher    cache.SharedIndexInformer
}

// NewWatcherManager creates a new watcher manager
func NewWatcherManager(clientset kubernetes.Interface, nodeID, agentID string, eventChan chan *WatchEvent) *WatcherManager {
    // Create shared informer factory with 5 minute resync
    factory := informers.NewSharedInformerFactory(clientset, 5*time.Minute)

    return &WatcherManager{
        clientset: clientset,
        factory:   factory,
        nodeID:    nodeID,
        agentID:   agentID,
        eventChan: eventChan,
        stopChan:  make(chan struct{}),
        batcher:   NewEventBatcher(eventChan, 100, 5*time.Second),
    }
}

// Start initializes and starts all watchers
func (m *WatcherManager) Start(ctx context.Context) error {
    log.Info().
        Str("node_id", m.nodeID).
        Str("agent_id", m.agentID).
        Msg("Starting watcher manager")

    // Initialize watchers
    if err := m.initPodWatcher(); err != nil {
        return fmt.Errorf("failed to init pod watcher: %w", err)
    }

    if err := m.initServiceAccountWatcher(); err != nil {
        return fmt.Errorf("failed to init service account watcher: %w", err)
    }

    // Only one agent (leader) should watch cluster-scoped resources
    if m.isLeader() {
        if err := m.initRBACWatcher(); err != nil {
            return fmt.Errorf("failed to init RBAC watcher: %w", err)
        }
    }

    // Start the shared informer factory
    m.factory.Start(m.stopChan)

    // Wait for caches to sync
    log.Info().Msg("Waiting for informer caches to sync")
    if !cache.WaitForCacheSync(m.stopChan,
        m.podWatcher.HasSynced,
        m.saWatcher.HasSynced,
    ) {
        return fmt.Errorf("failed to sync informer caches")
    }

    log.Info().Msg("Informer caches synced successfully")

    // Start event batcher
    m.wg.Add(1)
    go func() {
        defer m.wg.Done()
        m.batcher.Start(ctx)
    }()

    return nil
}

// Stop gracefully stops all watchers
func (m *WatcherManager) Stop() {
    log.Info().Msg("Stopping watcher manager")
    close(m.stopChan)
    m.batcher.Stop()
    m.wg.Wait()
    log.Info().Msg("Watcher manager stopped")
}

// initPodWatcher sets up the pod watcher with node-local filtering
func (m *WatcherManager) initPodWatcher() error {
    m.podWatcher = m.factory.Core().V1().Pods().Informer()

    // Add event handlers
    _, err := m.podWatcher.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            pod := obj.(*corev1.Pod)

            // CRITICAL: Only watch pods on this node
            if !m.shouldProcessPod(pod) {
                return
            }

            log.Debug().
                Str("pod", pod.Name).
                Str("namespace", pod.Namespace).
                Str("node", pod.Spec.NodeName).
                Msg("Pod added")

            m.batcher.Add(&WatchEvent{
                Type:         "ADDED",
                ResourceType: "pod",
                Resource:     pod,
                Timestamp:    time.Now(),
                NodeID:       m.nodeID,
                AgentID:      m.agentID,
            })
        },
        UpdateFunc: func(oldObj, newObj interface{}) {
            pod := newObj.(*corev1.Pod)

            if !m.shouldProcessPod(pod) {
                return
            }

            oldPod := oldObj.(*corev1.Pod)

            // Only send if resource version changed (real update)
            if pod.ResourceVersion == oldPod.ResourceVersion {
                return
            }

            log.Debug().
                Str("pod", pod.Name).
                Str("namespace", pod.Namespace).
                Msg("Pod updated")

            m.batcher.Add(&WatchEvent{
                Type:         "MODIFIED",
                ResourceType: "pod",
                Resource:     pod,
                Timestamp:    time.Now(),
                NodeID:       m.nodeID,
                AgentID:      m.agentID,
            })
        },
        DeleteFunc: func(obj interface{}) {
            pod := obj.(*corev1.Pod)

            if !m.shouldProcessPod(pod) {
                return
            }

            log.Debug().
                Str("pod", pod.Name).
                Str("namespace", pod.Namespace).
                Msg("Pod deleted")

            m.batcher.Add(&WatchEvent{
                Type:         "DELETED",
                ResourceType: "pod",
                Resource:     pod,
                Timestamp:    time.Now(),
                NodeID:       m.nodeID,
                AgentID:      m.agentID,
            })
        },
    })

    return err
}

// initServiceAccountWatcher sets up the service account watcher
func (m *WatcherManager) initServiceAccountWatcher() error {
    m.saWatcher = m.factory.Core().V1().ServiceAccounts().Informer()

    _, err := m.saWatcher.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            sa := obj.(*corev1.ServiceAccount)

            log.Debug().
                Str("serviceaccount", sa.Name).
                Str("namespace", sa.Namespace).
                Msg("ServiceAccount added")

            m.batcher.Add(&WatchEvent{
                Type:         "ADDED",
                ResourceType: "serviceaccount",
                Resource:     sa,
                Timestamp:    time.Now(),
                NodeID:       m.nodeID,
                AgentID:      m.agentID,
            })
        },
        UpdateFunc: func(oldObj, newObj interface{}) {
            sa := newObj.(*corev1.ServiceAccount)
            oldSA := oldObj.(*corev1.ServiceAccount)

            if sa.ResourceVersion == oldSA.ResourceVersion {
                return
            }

            m.batcher.Add(&WatchEvent{
                Type:         "MODIFIED",
                ResourceType: "serviceaccount",
                Resource:     sa,
                Timestamp:    time.Now(),
                NodeID:       m.nodeID,
                AgentID:      m.agentID,
            })
        },
        DeleteFunc: func(obj interface{}) {
            sa := obj.(*corev1.ServiceAccount)

            m.batcher.Add(&WatchEvent{
                Type:         "DELETED",
                ResourceType: "serviceaccount",
                Resource:     sa,
                Timestamp:    time.Now(),
                NodeID:       m.nodeID,
                AgentID:      m.agentID,
            })
        },
    })

    return err
}

// initRBACWatcher sets up RBAC watchers (leader only)
func (m *WatcherManager) initRBACWatcher() error {
    log.Info().Msg("This agent is the leader, watching RBAC resources")

    // Watch ClusterRoles
    crInformer := m.factory.Rbac().V1().ClusterRoles().Informer()
    _, err := crInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            cr := obj.(*rbacv1.ClusterRole)
            m.batcher.Add(&WatchEvent{
                Type:         "ADDED",
                ResourceType: "clusterrole",
                Resource:     cr,
                Timestamp:    time.Now(),
                NodeID:       m.nodeID,
                AgentID:      m.agentID,
            })
        },
        UpdateFunc: func(oldObj, newObj interface{}) {
            cr := newObj.(*rbacv1.ClusterRole)
            oldCR := oldObj.(*rbacv1.ClusterRole)

            if cr.ResourceVersion == oldCR.ResourceVersion {
                return
            }

            m.batcher.Add(&WatchEvent{
                Type:         "MODIFIED",
                ResourceType: "clusterrole",
                Resource:     cr,
                Timestamp:    time.Now(),
                NodeID:       m.nodeID,
                AgentID:      m.agentID,
            })
        },
        DeleteFunc: func(obj interface{}) {
            cr := obj.(*rbacv1.ClusterRole)
            m.batcher.Add(&WatchEvent{
                Type:         "DELETED",
                ResourceType: "clusterrole",
                Resource:     cr,
                Timestamp:    time.Now(),
                NodeID:       m.nodeID,
                AgentID:      m.agentID,
            })
        },
    })

    return err
}

// shouldProcessPod determines if this agent should process a pod
func (m *WatcherManager) shouldProcessPod(pod *corev1.Pod) bool {
    // Only process pods scheduled on this node
    return pod.Spec.NodeName == m.nodeID
}

// isLeader determines if this agent should watch cluster-scoped resources
// Simple implementation: agent on alphabetically first node is leader
// In production, use leader election
func (m *WatcherManager) isLeader() bool {
    // TODO: Implement proper leader election using client-go/tools/leaderelection
    // For now, simple heuristic based on agent ID
    return m.agentID == "agent-0" // First agent in DaemonSet
}
```

---

### 2. Event Batcher

**File:** `/agent/internal/watcher/batcher.go` (NEW)

```go
package watcher

import (
    "context"
    "sync"
    "time"

    "github.com/rs/zerolog/log"
)

// EventBatcher batches watch events to reduce gRPC calls
type EventBatcher struct {
    events      []*WatchEvent
    maxBatch    int
    maxWait     time.Duration
    eventChan   chan *WatchEvent
    flushChan   chan []*WatchEvent
    inputChan   chan *WatchEvent
    stopChan    chan struct{}
    mu          sync.Mutex
    ticker      *time.Ticker
}

// NewEventBatcher creates a new event batcher
func NewEventBatcher(eventChan chan *WatchEvent, maxBatch int, maxWait time.Duration) *EventBatcher {
    return &EventBatcher{
        events:      make([]*WatchEvent, 0, maxBatch),
        maxBatch:    maxBatch,
        maxWait:     maxWait,
        eventChan:   eventChan,
        inputChan:   make(chan *WatchEvent, 1000),
        flushChan:   make(chan []*WatchEvent, 10),
        stopChan:    make(chan struct{}),
        ticker:      time.NewTicker(maxWait),
    }
}

// Add adds an event to the batch
func (b *EventBatcher) Add(event *WatchEvent) {
    select {
    case b.inputChan <- event:
    default:
        log.Warn().Msg("Event batcher input channel full, dropping event")
    }
}

// Start begins batching events
func (b *EventBatcher) Start(ctx context.Context) {
    log.Info().
        Int("max_batch", b.maxBatch).
        Dur("max_wait", b.maxWait).
        Msg("Starting event batcher")

    for {
        select {
        case event := <-b.inputChan:
            b.mu.Lock()
            b.events = append(b.events, event)

            // Flush if batch is full
            if len(b.events) >= b.maxBatch {
                batch := b.flush()
                b.mu.Unlock()

                if len(batch) > 0 {
                    b.sendBatch(batch)
                }
            } else {
                b.mu.Unlock()
            }

        case <-b.ticker.C:
            // Flush on timer
            b.mu.Lock()
            batch := b.flush()
            b.mu.Unlock()

            if len(batch) > 0 {
                log.Debug().
                    Int("count", len(batch)).
                    Msg("Flushing batch on timer")
                b.sendBatch(batch)
            }

        case <-ctx.Done():
            b.Stop()
            return

        case <-b.stopChan:
            return
        }
    }
}

// flush returns current batch and resets
func (b *EventBatcher) flush() []*WatchEvent {
    if len(b.events) == 0 {
        return nil
    }

    batch := make([]*WatchEvent, len(b.events))
    copy(batch, b.events)
    b.events = b.events[:0]

    return batch
}

// sendBatch sends a batch to the event channel
func (b *EventBatcher) sendBatch(batch []*WatchEvent) {
    for _, event := range batch {
        select {
        case b.eventChan <- event:
        case <-time.After(5 * time.Second):
            log.Warn().Msg("Timeout sending event to channel")
        }
    }
}

// Stop stops the batcher
func (b *EventBatcher) Stop() {
    log.Info().Msg("Stopping event batcher")
    b.ticker.Stop()
    close(b.stopChan)
}
```

---

### 3. Enhanced gRPC Client with Retry

**File:** `/agent/internal/client/grpc_client_enhanced.go` (NEW)

```go
package client

import (
    "context"
    "fmt"
    "io"
    "sync/atomic"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/metadata"
    "github.com/rs/zerolog/log"

    pb "ksam/api/proto"
    "ksam/agent/internal/watcher"
)

// EnhancedClient is an improved gRPC client with retry and backpressure handling
type EnhancedClient struct {
    conn           *grpc.ClientConn
    client         pb.AgentServiceClient
    agentID        string
    nodeID         string
    serverAddr     string
    tlsConfig      *credentials.TransportCredentials

    // Channels
    sendChan       chan *pb.InventoryItem
    stopChan       chan struct{}

    // Retry config
    retryConfig    *RetryConfig

    // Backpressure
    backpressure   atomic.Bool

    // Metrics
    itemsSent      atomic.Uint64
    itemsFailed    atomic.Uint64
    reconnects     atomic.Uint64
}

// RetryConfig configures retry behavior
type RetryConfig struct {
    MaxRetries     int
    InitialBackoff time.Duration
    MaxBackoff     time.Duration
    Multiplier     float64
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() *RetryConfig {
    return &RetryConfig{
        MaxRetries:     10,
        InitialBackoff: 1 * time.Second,
        MaxBackoff:     60 * time.Second,
        Multiplier:     2.0,
    }
}

// NewEnhancedClient creates a new enhanced gRPC client
func NewEnhancedClient(agentID, nodeID, serverAddr string, tlsConfig *credentials.TransportCredentials) *EnhancedClient {
    return &EnhancedClient{
        agentID:      agentID,
        nodeID:       nodeID,
        serverAddr:   serverAddr,
        tlsConfig:    tlsConfig,
        sendChan:     make(chan *pb.InventoryItem, 10000), // Large buffer
        stopChan:     make(chan struct{}),
        retryConfig:  DefaultRetryConfig(),
    }
}

// Connect establishes connection to Core with retry
func (c *EnhancedClient) Connect(ctx context.Context) error {
    log.Info().
        Str("server", c.serverAddr).
        Str("agent_id", c.agentID).
        Msg("Connecting to Core")

    var err error
    backoff := c.retryConfig.InitialBackoff

    for i := 0; i < c.retryConfig.MaxRetries; i++ {
        // Dial options
        opts := []grpc.DialOption{
            grpc.WithBlock(),
            grpc.WithTimeout(10 * time.Second),
        }

        if c.tlsConfig != nil {
            opts = append(opts, grpc.WithTransportCredentials(*c.tlsConfig))
        } else {
            opts = append(opts, grpc.WithInsecure())
        }

        c.conn, err = grpc.DialContext(ctx, c.serverAddr, opts...)
        if err == nil {
            c.client = pb.NewAgentServiceClient(c.conn)
            log.Info().Msg("Connected to Core successfully")
            return nil
        }

        log.Warn().
            Err(err).
            Int("attempt", i+1).
            Dur("backoff", backoff).
            Msg("Failed to connect, retrying")

        select {
        case <-time.After(backoff):
            // Calculate next backoff
            backoff = time.Duration(float64(backoff) * c.retryConfig.Multiplier)
            if backoff > c.retryConfig.MaxBackoff {
                backoff = c.retryConfig.MaxBackoff
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }

    return fmt.Errorf("failed to connect after %d retries: %w", c.retryConfig.MaxRetries, err)
}

// Register registers the agent with Core
func (c *EnhancedClient) Register(ctx context.Context) (*pb.RegisterAgentResponse, error) {
    req := &pb.RegisterAgentRequest{
        AgentInfo: &pb.AgentInfo{
            AgentId:      c.agentID,
            NodeId:       c.nodeID,
            Version:      "1.0.0",
            Capabilities: []string{"pods", "serviceaccounts", "rbac"},
        },
    }

    resp, err := c.client.RegisterAgent(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("failed to register: %w", err)
    }

    log.Info().
        Interface("config", resp.Config).
        Msg("Agent registered successfully")

    return resp, nil
}

// StreamInventory starts bidirectional streaming
func (c *EnhancedClient) StreamInventory(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-c.stopChan:
            return nil
        default:
            err := c.streamLoop(ctx)
            if err != nil {
                log.Error().Err(err).Msg("Stream error, reconnecting")
                c.reconnects.Add(1)

                // Exponential backoff before reconnect
                time.Sleep(c.retryConfig.InitialBackoff)

                // Reconnect
                if err := c.Connect(ctx); err != nil {
                    log.Error().Err(err).Msg("Reconnect failed")
                    time.Sleep(5 * time.Second)
                }
            }
        }
    }
}

// streamLoop is the main streaming loop
func (c *EnhancedClient) streamLoop(ctx context.Context) error {
    // Add agent metadata to context
    md := metadata.New(map[string]string{
        "agent-id": c.agentID,
        "node-id":  c.nodeID,
    })
    ctx = metadata.NewOutgoingContext(ctx, md)

    // Create stream
    stream, err := c.client.StreamInventory(ctx)
    if err != nil {
        return fmt.Errorf("failed to create stream: %w", err)
    }

    log.Info().Msg("Stream established")

    // Send goroutine
    sendCtx, sendCancel := context.WithCancel(ctx)
    defer sendCancel()

    go func() {
        for {
            select {
            case item := <-c.sendChan:
                // Check backpressure
                if c.backpressure.Load() {
                    log.Warn().Msg("Backpressure detected, buffering locally")
                    time.Sleep(100 * time.Millisecond)
                }

                if err := stream.Send(item); err != nil {
                    log.Error().Err(err).Msg("Failed to send item")
                    c.itemsFailed.Add(1)

                    // Put back in channel if possible
                    select {
                    case c.sendChan <- item:
                    default:
                        log.Error().Msg("Send channel full, item lost")
                    }

                    return
                }

                c.itemsSent.Add(1)

            case <-sendCtx.Done():
                return
            }
        }
    }()

    // Receive loop (for ACKs and backpressure signals)
    for {
        resp, err := stream.Recv()
        if err == io.EOF {
            log.Info().Msg("Stream closed by server")
            return nil
        }
        if err != nil {
            return fmt.Errorf("receive error: %w", err)
        }

        c.handleResponse(resp)
    }
}

// handleResponse processes server responses
func (c *EnhancedClient) handleResponse(resp *pb.StreamResponse) {
    switch resp.Type {
    case pb.StreamResponse_ACK:
        log.Debug().
            Int64("message_count", resp.MessageCount).
            Msg("Received ACK")

    case pb.StreamResponse_BACKPRESSURE:
        log.Warn().
            Str("message", resp.Message).
            Msg("Backpressure signal received")
        c.backpressure.Store(true)

        // Reduce send rate
        go func() {
            time.Sleep(5 * time.Second)
            c.backpressure.Store(false)
            log.Info().Msg("Backpressure cleared")
        }()

    case pb.StreamResponse_HEARTBEAT:
        log.Debug().Msg("Heartbeat received")
    }
}

// Send queues an item for sending
func (c *EnhancedClient) Send(item *pb.InventoryItem) error {
    select {
    case c.sendChan <- item:
        return nil
    case <-time.After(1 * time.Second):
        return fmt.Errorf("send channel full")
    }
}

// Close closes the client
func (c *EnhancedClient) Close() error {
    log.Info().Msg("Closing gRPC client")
    close(c.stopChan)

    if c.conn != nil {
        return c.conn.Close()
    }

    return nil
}

// GetMetrics returns client metrics
func (c *EnhancedClient) GetMetrics() map[string]uint64 {
    return map[string]uint64{
        "items_sent":   c.itemsSent.Load(),
        "items_failed": c.itemsFailed.Load(),
        "reconnects":   c.reconnects.Load(),
    }
}
```

---

## Core gRPC Server Examples

### 4. Enhanced gRPC Server with Session Management

**File:** `/core/internal/grpc/session_manager.go` (NEW)

```go
package grpc

import (
    "sync"
    "time"

    "golang.org/x/time/rate"
    "github.com/rs/zerolog/log"
)

// Session represents an agent connection session
type Session struct {
    AgentID        string
    NodeID         string
    ConnectedAt    time.Time
    LastSeen       time.Time
    MessageCount   int64
    ErrorCount     int64
    RateLimit      *rate.Limiter

    mu             sync.RWMutex
}

// AgentSessionManager manages all active agent sessions
type AgentSessionManager struct {
    sessions sync.Map // map[agentID]*Session
    metrics  *SessionMetrics
}

// SessionMetrics tracks session statistics
type SessionMetrics struct {
    TotalSessions     int64
    ActiveSessions    int64
    TotalMessages     int64
    TotalErrors       int64
}

// NewAgentSessionManager creates a new session manager
func NewAgentSessionManager() *AgentSessionManager {
    return &AgentSessionManager{
        metrics: &SessionMetrics{},
    }
}

// GetOrCreate gets an existing session or creates a new one
func (m *AgentSessionManager) GetOrCreate(agentID, nodeID string) *Session {
    if s, ok := m.sessions.Load(agentID); ok {
        session := s.(*Session)
        session.mu.Lock()
        session.LastSeen = time.Now()
        session.mu.Unlock()
        return session
    }

    // Create new session
    session := &Session{
        AgentID:     agentID,
        NodeID:      nodeID,
        ConnectedAt: time.Now(),
        LastSeen:    time.Now(),
        RateLimit:   rate.NewLimiter(1000, 5000), // 1000/s, burst 5000
    }

    m.sessions.Store(agentID, session)
    m.metrics.TotalSessions++
    m.metrics.ActiveSessions++

    log.Info().
        Str("agent_id", agentID).
        Str("node_id", nodeID).
        Msg("New agent session created")

    return session
}

// Remove removes a session
func (m *AgentSessionManager) Remove(agentID string) {
    if _, ok := m.sessions.LoadAndDelete(agentID); ok {
        m.metrics.ActiveSessions--

        log.Info().
            Str("agent_id", agentID).
            Msg("Agent session removed")
    }
}

// Get retrieves a session
func (m *AgentSessionManager) Get(agentID string) (*Session, bool) {
    s, ok := m.sessions.Load(agentID)
    if !ok {
        return nil, false
    }
    return s.(*Session), true
}

// CountActive returns the number of active sessions
func (m *AgentSessionManager) CountActive() int {
    count := 0
    m.sessions.Range(func(key, value interface{}) bool {
        count++
        return true
    })
    return count
}

// GetAllSessions returns all active sessions
func (m *AgentSessionManager) GetAllSessions() []*Session {
    sessions := []*Session{}

    m.sessions.Range(func(key, value interface{}) bool {
        sessions = append(sessions, value.(*Session))
        return true
    })

    return sessions
}

// CleanupStale removes stale sessions (no activity for 5 minutes)
func (m *AgentSessionManager) CleanupStale() {
    staleThreshold := time.Now().Add(-5 * time.Minute)

    m.sessions.Range(func(key, value interface{}) bool {
        session := value.(*Session)
        session.mu.RLock()
        lastSeen := session.LastSeen
        session.mu.RUnlock()

        if lastSeen.Before(staleThreshold) {
            agentID := key.(string)
            log.Warn().
                Str("agent_id", agentID).
                Time("last_seen", lastSeen).
                Msg("Removing stale session")

            m.Remove(agentID)
        }

        return true
    })
}

// StartCleanupTask starts periodic cleanup of stale sessions
func (m *AgentSessionManager) StartCleanupTask() {
    ticker := time.NewTicker(1 * time.Minute)

    go func() {
        for range ticker.C {
            m.CleanupStale()
        }
    }()
}

// UpdateMessageCount increments message count for a session
func (s *Session) UpdateMessageCount() {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.MessageCount++
    s.LastSeen = time.Now()
}

// UpdateErrorCount increments error count for a session
func (s *Session) UpdateErrorCount() {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.ErrorCount++
    s.LastSeen = time.Now()
}

// CheckRateLimit checks if this session is within rate limits
func (s *Session) CheckRateLimit() bool {
    return s.RateLimit.Allow()
}
```

---

### 5. Enhanced gRPC Handler

**File:** `/core/internal/grpc/handler_enhanced.go` (MODIFY)

```go
package grpc

import (
    "context"
    "fmt"
    "io"
    "time"

    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "github.com/rs/zerolog/log"

    pb "ksam/api/proto"
    "ksam/core/internal/ingest"
)

// Server implements the AgentService gRPC server
type Server struct {
    pb.UnimplementedAgentServiceServer

    sessionMgr *AgentSessionManager
    ingestAPI  *ingest.IngestAPI

    // Config
    ackInterval int64 // Send ACK every N messages
}

// NewServer creates a new gRPC server
func NewServer(ingestAPI *ingest.IngestAPI) *Server {
    return &Server{
        sessionMgr:  NewAgentSessionManager(),
        ingestAPI:   ingestAPI,
        ackInterval: 100,
    }
}

// RegisterAgent handles agent registration
func (s *Server) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
    log.Info().
        Str("agent_id", req.AgentInfo.AgentId).
        Str("node_id", req.AgentInfo.NodeId).
        Str("version", req.AgentInfo.Version).
        Strs("capabilities", req.AgentInfo.Capabilities).
        Msg("Agent registration request")

    // Validate agent info
    if req.AgentInfo.AgentId == "" {
        return nil, status.Error(codes.InvalidArgument, "agent_id is required")
    }
    if req.AgentInfo.NodeId == "" {
        return nil, status.Error(codes.InvalidArgument, "node_id is required")
    }

    // Create session
    session := s.sessionMgr.GetOrCreate(req.AgentInfo.AgentId, req.AgentInfo.NodeId)

    // Return config
    return &pb.RegisterAgentResponse{
        Success: true,
        Message: "Agent registered successfully",
        Config: &pb.AgentConfig{
            RateLimit:       1000, // items per second
            BatchSize:       100,
            BatchTimeout:    5000, // milliseconds
            EnabledWatchers: []string{"pods", "serviceaccounts", "rbac"},
            HeartbeatInterval: 30, // seconds
        },
    }, nil
}

// StreamInventory handles bidirectional streaming from agents
func (s *Server) StreamInventory(stream pb.AgentService_StreamInventoryServer) error {
    // Extract agent metadata
    md, ok := metadata.FromIncomingContext(stream.Context())
    if !ok {
        return status.Error(codes.InvalidArgument, "missing metadata")
    }

    agentIDSlice := md.Get("agent-id")
    if len(agentIDSlice) == 0 {
        return status.Error(codes.InvalidArgument, "missing agent-id in metadata")
    }
    agentID := agentIDSlice[0]

    nodeIDSlice := md.Get("node-id")
    if len(nodeIDSlice) == 0 {
        return status.Error(codes.InvalidArgument, "missing node-id in metadata")
    }
    nodeID := nodeIDSlice[0]

    log.Info().
        Str("agent_id", agentID).
        Str("node_id", nodeID).
        Msg("Agent stream connected")

    // Get or create session
    session := s.sessionMgr.GetOrCreate(agentID, nodeID)
    defer s.sessionMgr.Remove(agentID)

    // Start heartbeat sender
    ctx, cancel := context.WithCancel(stream.Context())
    defer cancel()

    go s.sendHeartbeats(ctx, stream, session)

    // Receive loop
    for {
        item, err := stream.Recv()
        if err == io.EOF {
            log.Info().
                Str("agent_id", agentID).
                Msg("Agent stream closed")
            return nil
        }
        if err != nil {
            log.Error().
                Err(err).
                Str("agent_id", agentID).
                Msg("Stream receive error")
            return err
        }

        // Check rate limit
        if !session.CheckRateLimit() {
            log.Warn().
                Str("agent_id", agentID).
                Msg("Rate limit exceeded, sending backpressure signal")

            session.UpdateErrorCount()

            // Send backpressure signal
            if err := stream.Send(&pb.StreamResponse{
                Type:    pb.StreamResponse_BACKPRESSURE,
                Message: "rate_limit_exceeded",
            }); err != nil {
                log.Error().Err(err).Msg("Failed to send backpressure signal")
            }

            continue
        }

        // Process item
        if err := s.processInventoryItem(item, agentID, nodeID); err != nil {
            log.Error().
                Err(err).
                Str("agent_id", agentID).
                Str("type", item.Type).
                Str("uid", item.Uid).
                Msg("Failed to process item")

            session.UpdateErrorCount()
            continue
        }

        session.UpdateMessageCount()

        // Send ACK periodically
        if session.MessageCount%s.ackInterval == 0 {
            if err := stream.Send(&pb.StreamResponse{
                Type:         pb.StreamResponse_ACK,
                MessageCount: session.MessageCount,
            }); err != nil {
                log.Error().Err(err).Msg("Failed to send ACK")
            }
        }
    }
}

// processInventoryItem processes a single inventory item
func (s *Server) processInventoryItem(item *pb.InventoryItem, agentID, nodeID string) error {
    // Add agent metadata
    if item.Metadata == nil {
        item.Metadata = make(map[string]string)
    }
    item.Metadata["source_agent_id"] = agentID
    item.Metadata["source_node_id"] = nodeID
    item.Metadata["received_at"] = time.Now().Format(time.RFC3339)

    // Pass to IngestAPI
    return s.ingestAPI.ProcessInventoryItem(item)
}

// sendHeartbeats sends periodic heartbeat messages
func (s *Server) sendHeartbeats(ctx context.Context, stream pb.AgentService_StreamInventoryServer, session *Session) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := stream.Send(&pb.StreamResponse{
                Type: pb.StreamResponse_HEARTBEAT,
            }); err != nil {
                log.Error().
                    Err(err).
                    Str("agent_id", session.AgentID).
                    Msg("Failed to send heartbeat")
                return
            }
        }
    }
}

// Heartbeat handles explicit heartbeat requests
func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
    session, ok := s.sessionMgr.Get(req.AgentId)
    if !ok {
        return nil, status.Error(codes.NotFound, "agent not registered")
    }

    session.mu.Lock()
    session.LastSeen = time.Now()
    session.mu.Unlock()

    return &pb.HeartbeatResponse{
        Success:   true,
        Timestamp: time.Now().Unix(),
    }, nil
}
```

---

## Feature Flag Implementation

### 6. Configuration with Collection Mode

**File:** `/core/internal/config/config.go` (MODIFY)

```go
package config

import (
    "os"
    "strconv"
    "time"

    "github.com/rs/zerolog/log"
)

// CollectionMode defines how Core collects data
type CollectionMode string

const (
    CollectionModeAgent  CollectionMode = "agent"  // Agent-based collection only
    CollectionModeCore   CollectionMode = "core"   // Core direct collection (legacy)
    CollectionModeHybrid CollectionMode = "hybrid" // Both (migration mode)
)

// Config holds all configuration
type Config struct {
    // Collection settings
    CollectionMode      CollectionMode
    CoreDirectCollection bool // Enable K8s client in Core
    AgentEnabled        bool // Enable agent DaemonSet

    // Database
    DatabaseHost     string
    DatabasePort     int
    DatabaseUser     string
    DatabasePassword string
    DatabaseName     string

    // NATS
    NATSUrl          string
    NATSClusterID    string

    // gRPC Server
    GRPCPort         int
    GRPCMaxConnections int
    GRPCMaxStreams   int

    // REST API
    HTTPPort         int
    AuthEnabled      bool

    // Worker Pool
    WorkerConcurrency int

    // Ingestion
    IngestionRateLimit     int // items per second per agent
    IngestionBurstSize     int
    DeduplicationEnabled   bool
    DeduplicationTTL       time.Duration
    ValidationEnabled      bool
    EnrichmentEnabled      bool

    // Kubernetes (if CoreDirectCollection = true)
    KubeConfigPath   string
    InCluster        bool
}

// Load loads configuration from environment variables
func Load() *Config {
    cfg := &Config{
        // Collection mode
        CollectionMode:       CollectionMode(getEnv("COLLECTION_MODE", "agent")),
        CoreDirectCollection: getBoolEnv("CORE_DIRECT_COLLECTION", false),
        AgentEnabled:         getBoolEnv("AGENT_ENABLED", true),

        // Database
        DatabaseHost:     getEnv("DB_HOST", "postgres"),
        DatabasePort:     getIntEnv("DB_PORT", 5432),
        DatabaseUser:     getEnv("DB_USER", "ksam"),
        DatabasePassword: getEnv("DB_PASSWORD", "ksam"),
        DatabaseName:     getEnv("DB_NAME", "ksam"),

        // NATS
        NATSUrl:       getEnv("NATS_URL", "nats://nats:4222"),
        NATSClusterID: getEnv("NATS_CLUSTER_ID", "ksam-cluster"),

        // gRPC
        GRPCPort:           getIntEnv("GRPC_PORT", 9090),
        GRPCMaxConnections: getIntEnv("GRPC_MAX_CONNECTIONS", 1000),
        GRPCMaxStreams:     getIntEnv("GRPC_MAX_STREAMS", 100),

        // HTTP
        HTTPPort:    getIntEnv("HTTP_PORT", 8080),
        AuthEnabled: getBoolEnv("AUTH_ENABLED", true),

        // Workers
        WorkerConcurrency: getIntEnv("WORKER_CONCURRENCY", 5),

        // Ingestion
        IngestionRateLimit:   getIntEnv("INGESTION_RATE_LIMIT", 1000),
        IngestionBurstSize:   getIntEnv("INGESTION_BURST_SIZE", 5000),
        DeduplicationEnabled: getBoolEnv("DEDUPLICATION_ENABLED", true),
        DeduplicationTTL:     getDurationEnv("DEDUPLICATION_TTL", 5*time.Minute),
        ValidationEnabled:    getBoolEnv("VALIDATION_ENABLED", true),
        EnrichmentEnabled:    getBoolEnv("ENRICHMENT_ENABLED", true),

        // Kubernetes
        KubeConfigPath: getEnv("KUBECONFIG", ""),
        InCluster:      getBoolEnv("IN_CLUSTER", true),
    }

    cfg.validate()
    cfg.log()

    return cfg
}

// validate validates configuration
func (c *Config) validate() {
    // Validate collection mode
    switch c.CollectionMode {
    case CollectionModeAgent:
        c.CoreDirectCollection = false
        c.AgentEnabled = true
    case CollectionModeCore:
        c.CoreDirectCollection = true
        c.AgentEnabled = false
    case CollectionModeHybrid:
        c.CoreDirectCollection = true
        c.AgentEnabled = true
    default:
        log.Fatal().
            Str("mode", string(c.CollectionMode)).
            Msg("Invalid collection mode, must be: agent, core, or hybrid")
    }
}

// log logs the configuration
func (c *Config) log() {
    log.Info().
        Str("collection_mode", string(c.CollectionMode)).
        Bool("core_direct_collection", c.CoreDirectCollection).
        Bool("agent_enabled", c.AgentEnabled).
        Int("grpc_port", c.GRPCPort).
        Int("http_port", c.HTTPPort).
        Msg("Configuration loaded")
}

// Helper functions
func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
    if value := os.Getenv(key); value != "" {
        if intVal, err := strconv.Atoi(value); err == nil {
            return intVal
        }
    }
    return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
    if value := os.Getenv(key); value != "" {
        if boolVal, err := strconv.ParseBool(value); err == nil {
            return boolVal
        }
    }
    return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
    if value := os.Getenv(key); value != "" {
        if duration, err := time.ParseDuration(value); err == nil {
            return duration
        }
    }
    return defaultValue
}
```

---

### 7. Conditional K8s Client in Main

**File:** `/core/cmd/main.go` (MODIFY)

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/rs/zerolog/log"

    "ksam/core/internal/config"
    "ksam/core/internal/grpc"
    "ksam/core/internal/api"
    "ksam/core/pkg/worker"
    "ksam/core/pkg/messaging"
    // ... other imports
)

func main() {
    // Load configuration
    cfg := config.Load()

    // Setup database
    db := setupDatabase(cfg)

    // Setup NATS
    natsClient := setupNATS(cfg)

    // Conditional K8s client initialization
    var k8sCollector *collector.K8sCollector
    if cfg.CoreDirectCollection {
        log.Info().Msg("Initializing Kubernetes client for direct collection")
        k8sCollector = collector.NewK8sCollector(cfg, natsClient.Publisher())

        if err := k8sCollector.Start(context.Background()); err != nil {
            log.Fatal().Err(err).Msg("Failed to start K8s collector")
        }
        defer k8sCollector.Stop()
    } else {
        log.Info().Msg("Core direct collection disabled, relying on agents")
    }

    // ALWAYS start gRPC server (for agent communication or health checks)
    grpcServer := grpc.NewServer(cfg, db, natsClient)
    go func() {
        if err := grpcServer.Start(); err != nil {
            log.Fatal().Err(err).Msg("Failed to start gRPC server")
        }
    }()

    // Start worker pool
    workerPool := setupWorkerPool(cfg, db, natsClient)
    if err := workerPool.Start(context.Background()); err != nil {
        log.Fatal().Err(err).Msg("Failed to start worker pool")
    }

    // Start REST API
    apiServer := api.NewServer(cfg, db)
    go func() {
        if err := apiServer.Start(); err != nil {
            log.Fatal().Err(err).Msg("Failed to start API server")
        }
    }()

    // Wait for shutdown signal
    waitForShutdown()

    log.Info().Msg("Shutting down gracefully")
}

func waitForShutdown() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
}
```

---

This document provides concrete code examples for the key components of the agent-based refactoring. Each example includes proper error handling, logging, metrics, and production-ready patterns.

**Next Steps:**
1. Review code examples with team
2. Adapt to your specific project structure
3. Implement in phases as outlined in REFACTORING_PLAN_AGENT_BASED.md
4. Test thoroughly in each phase
