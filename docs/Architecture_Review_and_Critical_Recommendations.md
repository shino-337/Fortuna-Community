# KSAM Architecture Review & Critical Recommendations

**Date**: 2025-11-29  
**Reviewer**: Technical Architect  
**Architecture Version**: Post YAML-Rule Integration  
**Status**: ⚠️ REQUIRES IMMEDIATE ATTENTION

---

## Executive Summary

**Overall Assessment**: 🟡 **GOOD with CRITICAL GAPS**

The architecture shows excellent design principles and comprehensive planning. However, there are **7 CRITICAL issues** and **12 HIGH-priority gaps** that must be addressed before production deployment.

**Score**: 7.5/10

### Breakdown
- ✅ **Strengths** (8/10): Excellent core design, YAML rules, event-driven
- ⚠️ **Concerns** (6/10): Missing critical components, inconsistencies
- 🔴 **Critical Issues** (5/10): Data flow breaks, security gaps, performance risks

---

## 🔴 CRITICAL ISSUES (Must Fix Immediately)

### Issue #1: YAML Rule System NOT Integrated in Data Flow

**Severity**: 🔴 CRITICAL  
**Impact**: HIGH - Rules documented but not in architecture flow  
**Found At**: Lines 285-790 (Risk Engine section)

**Problem**:
```
# Current Architecture (Lines 420-510)
Risk Engine Worker → evaluateResourceCondition()
                   → Manual string matching
                   → Hardcoded rules

# But YAML System is documented separately (Lines 755-790)
Rule Loader (YAML) → Exists but not connected
CEL Engine         → Mentioned but not in flow
```

**Evidence**:
```yaml
# Line 433: Risk Engine still using old approach
- Evaluates resources against **built-in rules**
- Simple expression evaluation
- Manual condition matching

# But Line 755: YAML rules exist
rule_loader:
  rules_dir: "/etc/ksam/rules"
  schema_path: "/etc/ksam/config/rule-schema.json"
```

**Impact**:
- ❌ YAML rules cannot be loaded at runtime
- ❌ CEL expressions not evaluated
- ❌ Hot-reload doesn't work
- ❌ Rule versioning non-functional

**Required Fix**:

```diff
# BEFORE (Current - Lines 420-510)
┌──────────────┐
│ Risk Worker  │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Built-in     │ ← Hardcoded rules
│ Rules (Go)   │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ String Match │ ← Manual evaluation
└──────────────┘

# AFTER (Required)
┌──────────────┐
│ Risk Worker  │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Rule Loader  │ ← Load YAML files
│ (Startup +   │ ← Watch for changes
│  Hot-reload) │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ CEL Engine   │ ← Pre-compile expressions
│ + JSONPath   │ ← Evaluate conditions
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Insights     │
└──────────────┘
```

**Action Items**:
1. ✅ Update Risk Worker to use RuleLoader (Week 1)
2. ✅ Add CEL engine initialization (Week 1)
3. ✅ Implement hot-reload in Risk Worker (Week 1)
4. ✅ Update architecture diagram lines 420-510 (Week 1)

**Priority**: 🔴 P0 - BLOCKING

---

### Issue #2: Inconsistent Event Flow Between Components

**Severity**: 🔴 CRITICAL  
**Impact**: HIGH - Events may be lost or duplicated  
**Found At**: Lines 285-380 (Ingest API), Lines 380-510 (Workers)

**Problem**:

Current architecture shows **3 DIFFERENT event flows**:

**Flow 1** (Lines 285-295): Direct API to Workers
```
Ingest API → NATS → Workers
```

**Flow 2** (Lines 380-420): Normalizer Worker intermediate
```
Ingest API → NATS → NormalizerWorker → NATS → Other Workers
```

**Flow 3** (Lines 510-580): Database intermediate
```
Ingest API → Database → Workers poll database
```

**Contradiction**:
```yaml
# Line 293: Says event-driven
"Event-driven Architecture"
Message Queue → Workers

# But Line 420: Normalizer reads from queue
NormalizerWorker:
  subscribes: "ksam.raw.>"
  publishes: "ksam.normalized.>"

# But Line 470: Risk Worker also reads raw?
RiskWorker:
  subscribes: "ksam.raw.>" # ← WRONG! Should be normalized
```

**Impact**:
- ❌ Events processed multiple times
- ❌ Workers compete for same messages
- ❌ No clear data ownership
- ❌ Race conditions possible

**Required Fix**:

**STANDARD EVENT FLOW** (Must be enforced):

```
┌─────────────┐
│ Agent       │
└──────┬──────┘
       │ gRPC
       ▼
┌─────────────┐
│ Ingest API  │
└──────┬──────┘
       │ Publish: ksam.raw.{type}
       ▼
┌─────────────────────────────────┐
│ NATS Stream: ksam.raw           │
│ Retention: 1 hour               │
│ Subjects: pods, sas, roles, etc │
└──────┬──────────────────────────┘
       │ Subscribe: ksam.raw.>
       ▼
┌─────────────┐
│ Normalizer  │ ← SINGLE CONSUMER (durable)
│ Worker      │
└──────┬──────┘
       │ Publish: ksam.normalized.{type}
       ▼
┌─────────────────────────────────┐
│ NATS Stream: ksam.normalized    │
│ Retention: 24 hours             │
└──────┬──────────────────────────┘
       │
       ├────────────┬────────────┬────────────┐
       │            │            │            │
       ▼            ▼            ▼            ▼
┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│ Risk     │ │Correlator│ │ Policy   │ │ Baseline │
│ Worker   │ │ Worker   │ │ Engine   │ │ Learner  │
└──────────┘ └──────────┘ └──────────┘ └──────────┘
   │            │            │            │
   └────────────┴────────────┴────────────┘
                │
                ▼
        ┌──────────────┐
        │  Database    │
        │  (Postgres)  │
        └──────────────┘
```

**Rules to Enforce**:

1. ✅ **One Stream per Stage**:
   - `ksam.raw.*` - Only for raw events
   - `ksam.normalized.*` - Only for normalized data
   - `ksam.enriched.*` - Only for enriched data (optional)

2. ✅ **Durable Consumers**:
   - Each worker = unique durable consumer
   - Name format: `{worker-type}-{hostname}`
   - Example: `risk-worker-core-0`

3. ✅ **No Database Polling**:
   - Workers ONLY read from NATS
   - Workers write to Database
   - Database is write-only from workers

4. ✅ **Clear Subject Hierarchy**:
   ```
   ksam.raw.pods
   ksam.raw.serviceaccounts
   ksam.raw.roles
   ksam.raw.rolebindings
   
   ksam.normalized.pods
   ksam.normalized.serviceaccounts
   ksam.normalized.roles
   ksam.normalized.rolebindings
   ```

**Action Items**:
1. ✅ Update Lines 285-380 with correct flow (Week 1)
2. ✅ Fix all worker subscriptions (Week 1)
3. ✅ Add NATS subject hierarchy doc (Week 1)
4. ✅ Implement durable consumer naming (Week 1)

**Priority**: 🔴 P0 - BLOCKING

---

### Issue #3: Missing Error Handling & Retry Strategy

**Severity**: 🔴 CRITICAL  
**Impact**: HIGH - Data loss on failures  
**Found At**: Entire Worker section (Lines 380-510)

**Problem**:

Architecture mentions workers but **ZERO** error handling specification:

```yaml
# Lines 420-510: Workers documented
NormalizerWorker: ✅ Exists
CorrelatorWorker: ✅ Exists
RiskWorker: ✅ Exists

# But NO mention of:
- Message acknowledgment strategy ❌
- Retry policies ❌
- Dead letter queues ❌
- Error recovery ❌
- Circuit breakers ❌
```

**Real-world Scenarios NOT Handled**:

1. **Database Down**:
   ```
   Risk Worker → Create Insight → DB Error
   
   Current: ❌ Message lost
   Required: ✅ Retry with backoff → DLQ after 5 attempts
   ```

2. **Invalid Data**:
   ```
   Normalizer → Parse JSON → Invalid Format
   
   Current: ❌ Worker crashes
   Required: ✅ Log + Skip + Metrics
   ```

3. **CEL Evaluation Error**:
   ```
   Risk Worker → Evaluate Rule → CEL Error
   
   Current: ❌ Silent failure
   Required: ✅ Log + Alert + Skip rule
   ```

**Required Addition**:

```yaml
error_handling:
  
  # Message Acknowledgment
  ack_policy: explicit  # Manual ack only after success
  
  # Retry Strategy
  retry:
    max_attempts: 5
    backoff: exponential
    initial_delay: 1s
    max_delay: 60s
    jitter: true
  
  # Dead Letter Queue
  dlq:
    enabled: true
    stream: ksam.dlq
    retention: 7d
    alert_threshold: 100  # Alert if >100 messages in DLQ
  
  # Circuit Breaker
  circuit_breaker:
    enabled: true
    failure_threshold: 10
    timeout: 30s
    half_open_requests: 3
  
  # Error Classification
  errors:
    retryable:  # Retry these
      - database_connection_error
      - network_timeout
      - temporary_failure
    
    non_retryable:  # Skip these
      - invalid_data_format
      - schema_validation_error
      - cel_compilation_error
    
    fatal:  # Crash worker
      - nats_connection_lost
      - out_of_memory
  
  # Monitoring
  metrics:
    - message_processing_duration
    - message_retry_count
    - dlq_message_count
    - error_rate_by_type
```

**Action Items**:
1. ✅ Add error handling section to architecture (Week 1)
2. ✅ Implement retry logic in all workers (Week 2)
3. ✅ Set up DLQ stream (Week 1)
4. ✅ Add circuit breaker library (Week 2)
5. ✅ Create error classification (Week 1)

**Priority**: 🔴 P0 - BLOCKING

---

### Issue #4: No Rate Limiting or Backpressure

**Severity**: 🔴 CRITICAL  
**Impact**: HIGH - System overwhelm  
**Found At**: Lines 285-295 (Ingest API)

**Problem**:

Ingest API accepts unlimited events:

```go
// Line 293: Current implementation (implied)
func (s *IngestAPI) StreamInventory(stream pb.AgentService_StreamInventoryServer) {
    for {
        event, _ := stream.Recv()
        
        // Publish to NATS
        nats.Publish("ksam.raw.pods", event)  // ← NO RATE LIMIT
    }
}
```

**Attack Scenarios**:

1. **Malicious Agent**:
   ```
   Agent sends 1,000,000 events/sec
   → NATS queue fills up
   → Workers overwhelmed
   → System crashes
   ```

2. **Legitimate Spike**:
   ```
   1000 pods deployed simultaneously
   → 1000 * 100 events = 100,000 events
   → Workers can't keep up
   → Event backlog grows
   → Memory exhaustion
   ```

3. **Slow Consumer**:
   ```
   Risk Worker slow (database overloaded)
   → Queue backs up
   → Memory grows
   → OOM killed
   ```

**Required Fix**:

**1. Ingest API Rate Limiting**:

```go
type IngestAPI struct {
    rateLimiters map[string]*rate.Limiter  // Per-agent limiter
    
    globalLimiter *rate.Limiter  // Global limit
    
    config RateLimitConfig
}

type RateLimitConfig struct {
    GlobalLimit     int  // 100,000 events/sec
    PerAgentLimit   int  // 10,000 events/sec/agent
    BurstSize       int  // 50,000
    
    BackpressureThreshold float64  // 0.8 = slow down at 80%
}

func (s *IngestAPI) StreamInventory(stream pb.AgentService_StreamInventoryServer) error {
    agentID := getAgentID(stream)
    
    limiter := s.rateLimiters[agentID]
    
    for {
        event, _ := stream.Recv()
        
        // Check rate limit
        if !limiter.Allow() {
            return status.Error(codes.ResourceExhausted, "Rate limit exceeded")
        }
        
        // Check backpressure
        if s.isBackpressured() {
            return status.Error(codes.Unavailable, "System overloaded")
        }
        
        nats.Publish("ksam.raw.pods", event)
    }
}

func (s *IngestAPI) isBackpressured() bool {
    // Check NATS queue depth
    queueDepth := nats.StreamInfo("ksam.raw").State.Msgs
    maxQueueDepth := 100000
    
    return float64(queueDepth) / float64(maxQueueDepth) > s.config.BackpressureThreshold
}
```

**2. NATS Stream Limits**:

```yaml
nats:
  streams:
    ksam.raw:
      max_msgs: 100000        # Max 100k messages
      max_bytes: 10GB         # Max 10GB
      max_age: 1h             # Retain 1 hour
      discard: old            # Discard oldest on full
      
    ksam.normalized:
      max_msgs: 500000
      max_bytes: 50GB
      max_age: 24h
      discard: old
```

**3. Worker Backpressure**:

```go
type RiskWorker struct {
    maxConcurrent   int     // 100 concurrent processing
    currentLoad     int32   // Atomic counter
    backpressureCh  chan struct{}
}

func (w *RiskWorker) processMessage(msg *nats.Msg) error {
    // Check load
    if atomic.LoadInt32(&w.currentLoad) >= int32(w.maxConcurrent) {
        // Send backpressure signal
        select {
        case w.backpressureCh <- struct{}{}:
        default:
        }
        
        // NAK message for redelivery
        msg.Nak()
        return nil
    }
    
    atomic.AddInt32(&w.currentLoad, 1)
    defer atomic.AddInt32(&w.currentLoad, -1)
    
    // Process message
    // ...
}
```

**Action Items**:
1. ✅ Implement rate limiting in Ingest API (Week 1)
2. ✅ Configure NATS stream limits (Week 1)
3. ✅ Add backpressure handling to workers (Week 2)
4. ✅ Add monitoring for queue depth (Week 1)
5. ✅ Document rate limits in architecture (Week 1)

**Priority**: 🔴 P0 - BLOCKING

---

### Issue #5: Security Gaps in mTLS Implementation

**Severity**: 🔴 CRITICAL  
**Impact**: HIGH - Authentication bypass possible  
**Found At**: Lines 199-203, 2950-2968 (Security section)

**Problem**:

mTLS mentioned but **incomplete specification**:

```yaml
# Line 200: Mentioned
"mTLS with automatic cert rotation"

# Line 2953: Mentioned
"mTLS for all communication"

# But MISSING:
- Certificate authority setup ❌
- Cert issuance process ❌
- Rotation mechanism ❌
- Revocation strategy ❌
- Bootstrap security ❌
```

**Security Risks**:

1. **Initial Bootstrap**:
   ```
   Agent starts → Needs cert → How does it authenticate?
   
   Current: ❌ No specification
   Risk: Rogue agents can join
   ```

2. **Certificate Rotation**:
   ```
   Cert expires → Agent needs new cert → How?
   
   Current: "automatic cert rotation" (no details)
   Risk: Service disruption or insecure fallback
   ```

3. **Certificate Revocation**:
   ```
   Agent compromised → Need to revoke cert → How?
   
   Current: ❌ Not mentioned
   Risk: Compromised agents continue operating
   ```

4. **Private Key Protection**:
   ```
   Agent private key → Stored where? Protected how?
   
   Current: ❌ Not specified
   Risk: Key theft from pod filesystem
   ```

**Required Addition**:

```yaml
mtls_security:
  
  # Certificate Authority
  ca:
    type: internal  # internal, vault, cert-manager
    
    internal:
      root_ca: /etc/ksam/ca/root-ca.crt
      root_key: /etc/ksam/ca/root-ca.key  # Kubernetes Secret
      intermediate_ca: true
    
    vault:  # Alternative
      address: https://vault.company.com
      path: pki/ksam
      role: ksam-agent
  
  # Agent Bootstrap
  bootstrap:
    method: csr  # Certificate Signing Request
    
    process:
      1. Agent generates key pair on startup
      2. Agent sends CSR to Control Plane
      3. Control Plane validates (node identity, namespace)
      4. Control Plane signs cert (if valid)
      5. Agent receives signed cert
      6. Agent establishes mTLS connection
    
    validation:
      - node_name: Required (from downward API)
      - namespace: Required (must be ksam)
      - cluster_id: Required (from ConfigMap)
      - jwt_token: Required (ServiceAccount token)
  
  # Certificate Rotation
  rotation:
    validity: 90d
    rotation_threshold: 30d  # Rotate 30 days before expiry
    
    process:
      1. Agent monitors cert expiry
      2. At threshold, request new cert
      3. Receive new cert
      4. Establish new connection with new cert
      5. Close old connection
      6. Delete old cert
    
    graceful_rotation: true  # Keep old cert during transition
  
  # Certificate Revocation
  revocation:
    method: crl  # Certificate Revocation List
    
    crl:
      url: https://ksam-core/api/v1/crl
      refresh_interval: 1h
      
    check_on_connection: true
    
    triggers:
      - agent_compromised
      - node_decommissioned
      - security_policy_violation
  
  # Private Key Protection
  private_key:
    storage: memory  # Never write to disk
    algorithm: ecdsa
    size: 256
    
    protection:
      - stored_in_memory_only: true
      - encrypted_at_rest: false  # Not applicable (memory only)
      - no_export: true
  
  # Monitoring
  metrics:
    - cert_expiry_days
    - cert_rotation_success_rate
    - mtls_handshake_failures
    - revoked_cert_connection_attempts
```

**Implementation**:

```go
// Agent bootstrap
func (a *Agent) bootstrap() error {
    // 1. Generate key pair
    privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    
    // 2. Create CSR
    csrTemplate := &x509.CertificateRequest{
        Subject: pkix.Name{
            CommonName:   a.nodeID,
            Organization: []string{"ksam"},
        },
        DNSNames: []string{a.nodeName},
    }
    
    csrBytes, _ := x509.CreateCertificateRequest(rand.Reader, csrTemplate, privateKey)
    
    // 3. Send CSR to Core with authentication
    req := &pb.CSRRequest{
        Csr:         csrBytes,
        NodeName:    a.nodeName,
        Namespace:   a.namespace,
        ClusterID:   a.clusterID,
        JwtToken:    a.serviceAccountToken,  // From /var/run/secrets
    }
    
    resp, err := a.coreClient.SignCSR(ctx, req)
    if err != nil {
        return err
    }
    
    // 4. Store cert (in memory only)
    a.certificate = resp.Certificate
    a.privateKey = privateKey
    
    return nil
}
```

**Action Items**:
1. ✅ Document complete mTLS flow (Week 1)
2. ✅ Implement CSR-based bootstrap (Week 2)
3. ✅ Add cert rotation logic (Week 2)
4. ✅ Implement CRL checking (Week 2)
5. ✅ Add mTLS metrics (Week 2)
6. ✅ Security audit of implementation (Week 3)

**Priority**: 🔴 P0 - BLOCKING for production

---

### Issue #6: Incomplete Apache AGE Graph Integration

**Severity**: 🔴 CRITICAL  
**Impact**: HIGH - Graph queries broken  
**Found At**: Lines 570-655 (Graph Query Engine)

**Problem**:

Apache AGE mentioned but **implementation details missing**:

```yaml
# Line 600: Graph Query Engine mentioned
"Graph Query Engine powered by Apache AGE"

# Line 620: Example queries shown
"MATCH (sa:ServiceAccount)-[:USES]->(pod:Pod)"

# But MISSING:
- AGE extension installation ❌
- Graph schema creation ❌
- Data sync strategy ❌
- Query performance optimization ❌
- Backup/recovery ❌
```

**Critical Gaps**:

1. **Data Synchronization**:
   ```
   PostgreSQL tables (pods, service_accounts, roles)
           ↓
           ? How to sync to AGE graph?
           ↓
   AGE graph (vertices, edges)
   
   Current: ❌ No sync mechanism
   Required: Event-driven sync or triggers
   ```

2. **Graph Schema**:
   ```yaml
   # What is the graph schema?
   Vertices:
     - ServiceAccount: ❌ Not defined
     - Pod: ❌ Not defined
     - Role: ❌ Not defined
     - ClusterRole: ❌ Not defined
   
   Edges:
     - USES: ❌ Not defined
     - BOUND_TO: ❌ Not defined
     - HAS_PERMISSION: ❌ Not defined
   ```

3. **Query Performance**:
   ```sql
   -- Complex graph query
   MATCH (sa:ServiceAccount)-[:USES]->(pod:Pod),
         (sa)-[:BOUND_TO]->(role:Role),
         (role)-[:HAS_PERMISSION]->(resource:Resource)
   WHERE pod.namespace = 'production'
   RETURN sa, pod, role, resource
   
   Performance: ❌ Unknown
   Indexes: ❌ Not specified
   Query plan: ❌ Not optimized
   ```

**Required Addition**:

```yaml
apache_age_integration:
  
  # Installation
  setup:
    extension: age
    version: "1.4.0"
    
    installation:
      - CREATE EXTENSION IF NOT EXISTS age;
      - LOAD 'age';
      - SET search_path = ag_catalog, "$user", public;
  
  # Graph Schema
  schema:
    graph_name: ksam_graph
    
    vertices:
      ServiceAccount:
        properties:
          - id: uuid
          - name: string
          - namespace: string
          - risk_score: float
          - created_at: timestamp
      
      Pod:
        properties:
          - id: uuid
          - name: string
          - namespace: string
          - image: string
          - service_account: string
      
      Role:
        properties:
          - id: uuid
          - name: string
          - namespace: string
          - rules: jsonb
      
      ClusterRole:
        properties:
          - id: uuid
          - name: string
          - rules: jsonb
    
    edges:
      USES:
        from: Pod
        to: ServiceAccount
        properties:
          - created_at: timestamp
      
      BOUND_TO:
        from: ServiceAccount
        to: [Role, ClusterRole]
        properties:
          - binding_name: string
          - created_at: timestamp
      
      HAS_PERMISSION:
        from: [Role, ClusterRole]
        to: Resource
        properties:
          - api_group: string
          - resource: string
          - verbs: string[]
  
  # Data Sync Strategy
  sync:
    method: trigger  # PostgreSQL triggers
    
    triggers:
      on_service_account_insert:
        sql: |
          CREATE OR REPLACE FUNCTION sync_sa_to_graph()
          RETURNS TRIGGER AS $$
          BEGIN
            SELECT * FROM cypher('ksam_graph', $$
              CREATE (sa:ServiceAccount {
                id: $1,
                name: $2,
                namespace: $3,
                risk_score: $4
              })
            $$, NEW.id, NEW.name, NEW.namespace, NEW.risk_score);
            RETURN NEW;
          END;
          $$ LANGUAGE plpgsql;
          
          CREATE TRIGGER sa_to_graph_insert
          AFTER INSERT ON service_accounts
          FOR EACH ROW
          EXECUTE FUNCTION sync_sa_to_graph();
      
      on_service_account_update:
        sql: |
          -- Similar trigger for UPDATE
      
      on_relationship_insert:
        sql: |
          -- Trigger to create edges
  
  # Query Optimization
  optimization:
    indexes:
      - CREATE INDEX ON ksam_graph.ServiceAccount (namespace);
      - CREATE INDEX ON ksam_graph.Pod (service_account);
      - CREATE INDEX ON ksam_graph._ag_label_vertex (properties);
    
    query_hints:
      - Use parameterized queries
      - Limit graph depth (MAX 5 hops)
      - Add WHERE clauses early
      - Use EXPLAIN ANALYZE
  
  # Backup & Recovery
  backup:
    method: pg_dump with AGE
    schedule: daily
    retention: 30d
    
    commands:
      backup: |
        pg_dump -Fc -f ksam_backup.dump \
                --extension=age \
                ksam
      
      restore: |
        pg_restore -d ksam ksam_backup.dump
        SELECT * FROM ag_catalog.create_graph('ksam_graph');
```

**Action Items**:
1. ✅ Document AGE schema (Week 1)
2. ✅ Implement PostgreSQL triggers (Week 2)
3. ✅ Create sync verification tests (Week 2)
4. ✅ Add graph indexes (Week 2)
5. ✅ Performance test graph queries (Week 3)
6. ✅ Document backup procedure (Week 2)

**Priority**: 🔴 P0 - Required for graph features

---

### Issue #7: No Database Migration Strategy

**Severity**: 🔴 CRITICAL  
**Impact**: HIGH - Schema drift, data loss  
**Found At**: Lines 59-68 (Storage Layer), Missing migration section

**Problem**:

Database schema changes not managed:

```yaml
# Current: No migration strategy

# But we need to handle:
- Initial schema creation ❌
- Schema versioning ❌
- Rolling updates ❌
- Data migrations ❌
- Rollback procedures ❌
```

**Real Problems**:

1. **Adding New Fields**:
   ```sql
   -- Need to add CIS section to insights table
   ALTER TABLE insights ADD COLUMN cis_section VARCHAR(10);
   
   Current: ❌ Run manually? When? On which pod?
   Risk: Inconsistent schema across replicas
   ```

2. **Breaking Changes**:
   ```sql
   -- Change column type
   ALTER TABLE events ALTER COLUMN timestamp TYPE TIMESTAMPTZ;
   
   Current: ❌ How to handle existing data?
   Risk: Data loss or type errors
   ```

3. **Multi-Step Migrations**:
   ```sql
   -- Step 1: Add new column
   -- Step 2: Backfill data
   -- Step 3: Make NOT NULL
   -- Step 4: Remove old column
   
   Current: ❌ No orchestration
   Risk: Failed partial migrations
   ```

**Required Addition**:

```yaml
database_migrations:
  
  # Migration Tool
  tool: golang-migrate/migrate
  
  # Migration Files
  location: /migrations
  structure: |
    migrations/
      000001_initial_schema.up.sql
      000001_initial_schema.down.sql
      000002_add_cis_section.up.sql
      000002_add_cis_section.down.sql
      000003_add_age_extension.up.sql
      000003_add_age_extension.down.sql
  
  # Naming Convention
  naming: {version}_{description}.{direction}.sql
  
  # Execution Strategy
  execution:
    on_startup: true
    pod: ksam-core-0  # Only leader runs migrations
    
    leader_election:
      enabled: true
      lock_name: ksam-migration-lock
      timeout: 300s
  
  # Migration Process
  process:
    1. Check current schema version
    2. Acquire migration lock (PostgreSQL advisory lock)
    3. Run pending migrations sequentially
    4. Update schema_migrations table
    5. Release lock
    6. Signal other pods (schema version updated)
  
  # Example Migration
  example:
    file: 000004_add_yaml_rules_table.up.sql
    content: |
      -- Create rules table for YAML rules
      CREATE TABLE IF NOT EXISTS rules (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        rule_id VARCHAR(100) UNIQUE NOT NULL,
        name VARCHAR(255) NOT NULL,
        version VARCHAR(20) NOT NULL,
        category VARCHAR(50) NOT NULL,
        severity VARCHAR(20) NOT NULL,
        enabled BOOLEAN DEFAULT true,
        yaml_content TEXT NOT NULL,
        compiled_cel JSONB,
        created_at TIMESTAMPTZ DEFAULT NOW(),
        updated_at TIMESTAMPTZ DEFAULT NOW()
      );
      
      -- Create index
      CREATE INDEX idx_rules_rule_id ON rules(rule_id);
      CREATE INDEX idx_rules_category ON rules(category);
      
      -- Insert built-in rules
      INSERT INTO rules (rule_id, name, version, category, severity, yaml_content)
      SELECT 'cis-5.1.3', 
             'ServiceAccount with cluster-admin', 
             '1.0.0',
             'rbac',
             'critical',
             pg_read_file('/etc/ksam/rules/cis-5.1.3.yaml');
  
  # Rollback Strategy
  rollback:
    automatic: false  # Manual only
    
    process:
      1. Stop all Core pods
      2. Run down migration
      3. Verify data integrity
      4. Restart pods with old version
    
    commands:
      - migrate -path /migrations -database ${DB_URL} down 1
  
  # Testing
  testing:
    - Test migrations in staging
    - Verify rollback works
    - Test with production-size dataset
    - Measure migration time
  
  # Monitoring
  metrics:
    - migration_duration_seconds
    - migration_success_total
    - migration_failure_total
    - schema_version
```

**Implementation**:

```go
// In Core startup
func (c *Core) Start() error {
    // Run migrations on leader only
    if c.isLeader() {
        if err := c.runMigrations(); err != nil {
            return fmt.Errorf("migration failed: %w", err)
        }
    }
    
    // Wait for schema version sync
    c.waitForSchemaVersion()
    
    // Continue startup
    return c.startWorkers()
}

func (c *Core) runMigrations() error {
    m, err := migrate.New(
        "file:///migrations",
        c.databaseURL,
    )
    if err != nil {
        return err
    }
    
    // Acquire lock
    if _, err := c.db.Exec("SELECT pg_advisory_lock(123456)"); err != nil {
        return err
    }
    defer c.db.Exec("SELECT pg_advisory_unlock(123456)")
    
    // Run migrations
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return err
    }
    
    log.Info("Migrations completed successfully")
    return nil
}
```

**Action Items**:
1. ✅ Add golang-migrate to project (Week 1)
2. ✅ Create initial migrations (Week 1)
3. ✅ Implement leader election (Week 1)
4. ✅ Add migration testing (Week 2)
5. ✅ Document migration process (Week 1)
6. ✅ Create rollback playbook (Week 2)

**Priority**: 🔴 P0 - Required before any schema changes

---

## 🟡 HIGH-PRIORITY ISSUES (Fix Soon)

### Issue #8: Missing Prometheus Metrics Specification

**Severity**: 🟡 HIGH  
**Impact**: MEDIUM - Cannot monitor system health  
**Found At**: Lines 2939-2940

**Problem**: Metrics mentioned but not defined

**Required Addition**:

```yaml
prometheus_metrics:
  
  # Ingest API Metrics
  ingest:
    - ksam_ingest_requests_total{agent_id, status}
    - ksam_ingest_request_duration_seconds{agent_id}
    - ksam_ingest_events_received_total{resource_type}
    - ksam_ingest_rate_limit_exceeded_total{agent_id}
  
  # Worker Metrics
  workers:
    - ksam_worker_messages_processed_total{worker_type, status}
    - ksam_worker_processing_duration_seconds{worker_type}
    - ksam_worker_queue_depth{worker_type}
    - ksam_worker_errors_total{worker_type, error_type}
    - ksam_worker_concurrent_processing{worker_type}
  
  # Risk Engine Metrics
  risk_engine:
    - ksam_rules_evaluated_total{rule_id, matched}
    - ksam_rule_evaluation_duration_seconds{rule_id}
    - ksam_insights_created_total{rule_id, severity}
    - ksam_cel_compilation_errors_total{rule_id}
  
  # Database Metrics
  database:
    - ksam_db_queries_total{operation, table, status}
    - ksam_db_query_duration_seconds{operation, table}
    - ksam_db_connection_pool_size
    - ksam_db_connection_pool_active
  
  # NATS Metrics
  nats:
    - ksam_nats_stream_messages{stream}
    - ksam_nats_stream_bytes{stream}
    - ksam_nats_consumer_pending{consumer, stream}
    - ksam_nats_publish_latency_seconds
```

**Action Items**:
1. ✅ Define all metrics (Week 1)
2. ✅ Implement Prometheus client (Week 2)
3. ✅ Create Grafana dashboards (Week 2)
4. ✅ Set up alerting rules (Week 2)

**Priority**: 🟡 P1

---

### Issue #9: Incomplete eBPF Event Filtering

**Severity**: 🟡 HIGH  
**Impact**: MEDIUM - Performance issues  
**Found At**: Lines 145-164

**Problem**: Filtering strategy mentioned but not detailed

**Required Addition**:

```yaml
ebpf_filtering:
  
  # Kernel-Level Filtering (BPF Map)
  kernel:
    # Namespace filtering
    tracked_namespaces:
      type: hash_map
      key: namespace_id (u32)
      value: enabled (bool)
      max_entries: 1000
    
    # Process filtering
    skip_processes:
      type: hash_map
      key: process_name_hash (u64)
      value: enabled (bool)
      max_entries: 100
      
      blocked:
        - systemd
        - containerd
        - dockerd
        - kubelet
        - kube-proxy
    
    # Path filtering (only track sensitive paths)
    tracked_paths:
      type: trie_map
      entries:
        - /etc/shadow
        - /etc/passwd
        - /var/run/secrets/*
        - /root/.ssh/*
        - /home/*/.ssh/*
  
  # User-Space Filtering (Agent)
  userspace:
    # Rate limiting per pod
    rate_limit:
      max_events_per_pod: 100  # events/sec
      max_events_global: 10000  # events/sec/node
      window: 1s
    
    # Deduplication
    dedup:
      enabled: true
      window: 5s
      key: {pid, syscall, path}  # Dedupe same event
    
    # Aggregation
    aggregation:
      enabled: true
      window: 10s
      strategy: count  # Count repetitive events
  
  # Event Type Filtering
  event_types:
    execve:
      enabled: true
      priority: 100
      filter:
        - ignore_short_lived: true  # <100ms lifetime
        - ignore_shell_scripts: false
    
    connect:
      enabled: true
      priority: 80
      filter:
        - ignore_loopback: true
        - ignore_cluster_cidrs: true  # 10.0.0.0/8
        - track_external_only: true
    
    open:
      enabled: true
      priority: 50
      filter:
        - track_writes_only: true
        - ignore_reads: true
        - sensitive_paths_only: true
```

**Action Items**:
1. ✅ Implement BPF maps (Week 3)
2. ✅ Add user-space filtering (Week 3)
3. ✅ Performance test filtering (Week 3)

**Priority**: 🟡 P1

---

### Issue #10: No Disaster Recovery Plan

**Severity**: 🟡 HIGH  
**Impact**: MEDIUM - Data loss risk  
**Found At**: Missing section

**Required Addition**:

```yaml
disaster_recovery:
  
  # Backup Strategy
  backup:
    schedule: daily
    retention: 30d
    
    components:
      database:
        method: pg_dump
        compression: gzip
        encryption: aes-256
        location: s3://ksam-backups/
      
      nats:
        method: snapshot
        retention: 7d
      
      rules:
        method: git
        repository: git@github.com:company/ksam-rules.git
  
  # Recovery Procedures
  recovery:
    rto: 4h  # Recovery Time Objective
    rpo: 24h  # Recovery Point Objective
    
    procedures:
      database:
        - Stop all Core pods
        - Restore database from backup
        - Run migrations
        - Start Core pods
        - Verify data integrity
      
      full_cluster:
        - Provision new cluster
        - Deploy KSAM from Helm
        - Restore database
        - Restore NATS state
        - Deploy rules
        - Verify connectivity
  
  # Testing
  testing:
    schedule: quarterly
    validation:
      - Restore database
      - Verify data integrity
      - Check all services
      - Measure recovery time
```

**Action Items**:
1. ✅ Implement backup automation (Week 3)
2. ✅ Create recovery playbook (Week 3)
3. ✅ Test recovery procedure (Week 4)

**Priority**: 🟡 P1

---

### Issue #11: Unclear Multi-Cluster Architecture

**Severity**: 🟡 HIGH  
**Impact**: MEDIUM - Scalability unclear  
**Found At**: Lines 2901-2908

**Problem**: Hub-spoke mentioned but not detailed

**Required Clarification**:

```yaml
multi_cluster:
  
  # Architecture
  topology: hub-spoke
  
  # Hub Cluster
  hub:
    components:
      - Aggregator (collects from all clusters)
      - Global Database
      - Unified Dashboard
      - Cross-Cluster Correlator
    
    responsibilities:
      - Aggregate insights from all clusters
      - Cross-cluster RBAC analysis
      - Global compliance reporting
      - Multi-cluster alerting
  
  # Spoke Clusters
  spoke:
    components:
      - Local Agent (DaemonSet)
      - Local Core (optional)
      - Forwarder (to Hub)
    
    responsibilities:
      - Local data collection
      - Local processing (optional)
      - Forward to Hub
  
  # Communication
  communication:
    spoke_to_hub:
      protocol: gRPC
      encryption: mTLS
      compression: gzip
      batching: true
      batch_size: 1000
    
    rate_limiting:
      per_cluster: 10000 events/sec
      global: 100000 events/sec
  
  # Data Sync
  sync:
    strategy: push  # Spokes push to Hub
    
    what_to_sync:
      - Insights (all)
      - Events (sampled)
      - Inventory (full)
      - Baselines (aggregated)
    
    deduplication:
      enabled: true
      key: {cluster_id, resource_id, insight_id}
```

**Action Items**:
1. ✅ Document multi-cluster architecture (Week 2)
2. ✅ Design aggregator component (Week 3)
3. ✅ Implement forwarder (Week 4)

**Priority**: 🟡 P1

---

## 🔵 MEDIUM-PRIORITY ISSUES (Fix When Possible)

### Issue #12: Missing Capacity Planning Guide

**Severity**: 🔵 MEDIUM  
**Impact**: LOW - Resource over/under-provisioning

**Required Addition**:

```yaml
capacity_planning:
  
  # Small Cluster (< 100 nodes)
  small:
    agent:
      replicas: 1 per node (DaemonSet)
      resources:
        requests: {cpu: 100m, memory: 40Mi}
        limits: {cpu: 200m, memory: 128Mi}
    
    core:
      replicas: 2
      resources:
        requests: {cpu: 500m, memory: 1Gi}
        limits: {cpu: 2000m, memory: 4Gi}
    
    database:
      size: 50GB
      iops: 3000
    
    nats:
      replicas: 1
      resources:
        requests: {cpu: 250m, memory: 512Mi}
  
  # Medium Cluster (100-500 nodes)
  medium:
    agent:
      replicas: 1 per node
      resources:
        requests: {cpu: 100m, memory: 40Mi}
        limits: {cpu: 200m, memory: 128Mi}
    
    core:
      replicas: 3
      resources:
        requests: {cpu: 1000m, memory: 2Gi}
        limits: {cpu: 4000m, memory: 8Gi}
    
    database:
      size: 500GB
      iops: 10000
    
    nats:
      replicas: 3
      resources:
        requests: {cpu: 500m, memory: 1Gi}
  
  # Large Cluster (> 500 nodes)
  large:
    agent:
      replicas: 1 per node
      resources:
        requests: {cpu: 100m, memory: 40Mi}
        limits: {cpu: 200m, memory: 128Mi}
    
    core:
      replicas: 5
      resources:
        requests: {cpu: 2000m, memory: 4Gi}
        limits: {cpu: 8000m, memory: 16Gi}
    
    database:
      size: 2TB
      iops: 50000
      replicas: 3 (PostgreSQL cluster)
    
    nats:
      replicas: 5
      resources:
        requests: {cpu: 1000m, memory: 2Gi}
```

**Priority**: 🔵 P2

---

### Issue #13-19: Additional Medium-Priority Items

I can provide detailed analysis for these if needed:

13. Missing log aggregation strategy
14. Incomplete API versioning strategy
15. No user authentication/authorization details
16. Missing webhook system for external integrations
17. No traffic shaping for dashboard API
18. Incomplete testing strategy for eBPF programs
19. Missing operational runbooks

Would you like me to detail any of these?

---

## 📋 Action Plan Summary

### Week 1 (CRITICAL - Must Do)

**P0 Items**:
1. ✅ Integrate YAML rule system into Risk Worker data flow
2. ✅ Standardize event flow and NATS subject hierarchy
3. ✅ Implement error handling and retry strategy
4. ✅ Add rate limiting to Ingest API
5. ✅ Document complete mTLS flow
6. ✅ Define Apache AGE graph schema
7. ✅ Implement database migration system

**Deliverables**:
- Updated architecture diagram with YAML rules
- NATS subject hierarchy document
- Error handling specification
- Rate limiting configuration
- mTLS security specification
- Graph schema DDL
- Migration framework

**Resources**: 2 senior engineers

---

### Week 2 (HIGH - Should Do)

**P0 Completion**:
1. ✅ Implement retry logic in all workers
2. ✅ Implement CSR-based mTLS bootstrap
3. ✅ Create PostgreSQL triggers for AGE sync
4. ✅ Add migration testing

**P1 Items**:
5. ✅ Define all Prometheus metrics
6. ✅ Create Grafana dashboards
7. ✅ Implement backup automation

**Resources**: 2 senior engineers + 1 DevOps

---

### Week 3 (MEDIUM - Nice to Have)

**P1 Items**:
1. ✅ Performance test graph queries
2. ✅ Security audit of mTLS
3. ✅ Implement eBPF filtering
4. ✅ Create disaster recovery playbook

**P2 Items**:
5. ✅ Design multi-cluster aggregator
6. ✅ Create capacity planning guide

**Resources**: 1 senior engineer + 1 SRE

---

### Week 4 (OPTIONAL - Future Work)

**P2 Items**:
1. ✅ Implement multi-cluster forwarder
2. ✅ Test disaster recovery
3. ✅ Create operational runbooks
4. ✅ Write eBPF testing framework

---

## 🎯 Recommendations Priority Matrix

```
┌─────────────────────────────────────────────────────┐
│                 URGENCY vs IMPACT                   │
├─────────────────────────────────────────────────────┤
│                                                     │
│  HIGH IMPACT          │  HIGH IMPACT                │
│  LOW URGENCY          │  HIGH URGENCY              │
│  ────────────────────────────────────────          │
│  • Multi-cluster      │  • YAML Rule Integration   │
│  • Capacity planning  │  • Event Flow Fix          │
│  • Disaster recovery  │  • Error Handling          │
│                       │  • Rate Limiting           │
│                       │  • mTLS Complete           │
│                       │  • AGE Integration         │
│                       │  • DB Migrations           │
│  ────────────────────────────────────────          │
│  LOW IMPACT           │  LOW IMPACT                │
│  LOW URGENCY          │  HIGH URGENCY              │
│  ────────────────────────────────────────          │
│  • Log aggregation    │  • Metrics definition      │
│  • Webhook system     │  • eBPF filtering details  │
│  • API versioning     │                            │
│                                                     │
└─────────────────────────────────────────────────────┘
```

---

## ✅ Strengths to Maintain

1. ✅ **Event-Driven Architecture**: Clean NATS-based design
2. ✅ **Lightweight Agent**: <50MB target is excellent
3. ✅ **YAML Rules**: Flexible rule system with CEL
4. ✅ **Graph-Native**: PostgreSQL + Apache AGE is powerful
5. ✅ **CIS Compliance**: Built-in CIS benchmark support
6. ✅ **Comprehensive Coverage**: Wide range of security features

---

## 📊 Final Recommendation

**Overall**: 🟡 **GOOD ARCHITECTURE WITH CRITICAL GAPS**

**Action**:
1. ✅ **Fix 7 critical issues** before production (3 weeks)
2. ✅ **Address high-priority items** for stability (2 weeks)
3. ✅ **Medium-priority items** can wait for v1.1

**Timeline**: **5 weeks to production-ready**

**Risk Assessment**: **MEDIUM** (with fixes applied)

**Go/No-Go**: **GO** (after critical fixes)

---

**Status**: Ready for Implementation  
**Next Review**: After Week 1 fixes