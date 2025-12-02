# KSAM Fresh Start Implementation Plan

## Executive Summary

Sau khi clean toàn bộ môi trường, phân tích documentation, và xác định requirements, đây là kế hoạch chi tiết để tạo mới dự án KSAM từ đầu theo đúng specifications.

## Documentation Analysis Summary

### Key Findings

#### 1. ARCHITECTURE.md
**Key Requirements**:
- Event-driven architecture với NATS JetStream
- Apache AGE cho graph database
- TimescaleDB cho time-series events
- Redis cho caching và baselines
- Worker pool pattern cho processing
- mTLS cho agent-core communication
- eBPF integration cho runtime events

**Components**:
- Agent: DaemonSet với eBPF, watchers, gRPC client
- Core: Ingest API → NATS → Worker Pool → Storage
- Dashboard: React với graph visualization, CIS compliance

#### 2. API_SPECIFICATION.md
**Key Requirements**:
- REST API với JWT authentication
- gRPC API cho agent communication
- WebSocket cho real-time updates
- OpenAPI 3.0 specification
- Rate limiting
- Comprehensive error handling

**APIs Required**:
- Authentication API
- Clusters API
- ServiceAccounts API
- Insights API
- Graph API
- Attack Simulation API
- Policies API
- Compliance API
- Audit Logs API
- Users API

#### 3. SECURITY_GUIDE.md
**Key Requirements**:
- mTLS cho tất cả inter-component communication
- JWT authentication với refresh tokens
- Role-based access control (admin, user, viewer)
- Secrets management (Vault/Kubernetes Secrets)
- Network policies
- Audit logging
- Security scanning (Trivy, Falco)

#### 4. IMPLEMENTATION_PLAN.md
**Key Requirements**:
- Sprint S0: NATS JetStream setup (CRITICAL)
- Sprint S1: mTLS implementation (CRITICAL)
- Sprint S2: Apache AGE integration (CRITICAL)
- Event-driven architecture từ đầu
- Worker pool pattern
- TimescaleDB cho events
- Baseline learning engine

## Critical Differences from Previous Implementation

### Architecture Changes
1. **Event-Driven**: Thay vì direct calls, sử dụng NATS JetStream
2. **Graph Database**: Apache AGE thay vì chỉ PostgreSQL
3. **Time-Series**: TimescaleDB cho events thay vì chỉ PostgreSQL
4. **Worker Pool**: Async processing với worker pool
5. **Baseline Learning**: ML-based anomaly detection

### Technology Stack Updates
- **Message Queue**: NATS JetStream (NEW)
- **Graph DB**: Apache AGE extension cho PostgreSQL (NEW)
- **Time-Series**: TimescaleDB extension (NEW)
- **Cache**: Redis (NEW)
- **Security**: mTLS từ đầu (REQUIRED)

## Implementation Phases

### Phase 0: Project Setup & Infrastructure (Week 1)

#### Day 1-2: Project Structure
**Tasks**:
1. Verify và clean project structure
2. Setup Go modules (agent, core)
3. Setup React project (dashboard)
4. Setup proto definitions
5. Setup scripts và tooling

**Deliverables**:
- Clean project structure
- All modules buildable
- Development environment ready

#### Day 3-4: Infrastructure Setup
**Tasks**:
1. **PostgreSQL + Extensions**
   - PostgreSQL deployment
   - TimescaleDB extension
   - Apache AGE extension
   - Database schema design

2. **NATS JetStream**
   - NATS StatefulSet deployment
   - Stream definitions
   - Retention policies
   - Monitoring setup

3. **Redis**
   - Redis deployment
   - Configuration
   - Connection pooling

4. **Kubernetes Setup**
   - Namespace creation
   - RBAC setup
   - Network policies
   - Secrets management

**Deliverables**:
- All infrastructure components running
- Database accessible
- NATS streams configured
- Redis accessible

#### Day 5: Core Skeleton
**Tasks**:
1. Core component structures
2. Proto definitions
3. Database models
4. Basic build scripts

**Deliverables**:
- Component skeletons
- Proto definitions
- Build scripts

### Phase 1: Core Data Pipeline (Week 2-3)

#### Sprint 1.1: Agent Implementation
**Priority**: HIGH

**Tasks**:
1. **K8s Watchers**
   - Pod watcher
   - ServiceAccount watcher
   - Role/RoleBinding watchers
   - ClusterRole/ClusterRoleBinding watchers

2. **gRPC Client với mTLS**
   - mTLS client setup
   - Certificate management
   - Registration logic
   - Inventory streaming
   - Heartbeat

3. **Agent Deployment**
   - DaemonSet manifest
   - RBAC configuration
   - ConfigMap
   - Testing

**Deliverables**:
- Agent collecting inventory
- Agent communicating via mTLS
- Agent deployed

#### Sprint 1.2: Core Ingest & NATS Integration
**Priority**: CRITICAL

**Tasks**:
1. **Ingest API**
   - gRPC server với mTLS
   - Register handler
   - StreamInventory handler
   - StreamEvents handler

2. **NATS Integration**
   - NATS client setup
   - Publish to streams
   - Stream configuration
   - Error handling

3. **Message Formats**
   - Proto message definitions
   - JSON schemas
   - Validation

**Deliverables**:
- Ingest API receiving data
- Data published to NATS
- Streams working

#### Sprint 1.3: Worker Pool & Processing
**Priority**: CRITICAL

**Tasks**:
1. **Worker Pool**
   - Worker implementation
   - Job queue
   - Concurrency control
   - Error handling

2. **Normalizer Worker**
   - Subscribe to inventory streams
   - Normalize messages
   - Publish normalized data

3. **Correlator Worker**
   - Subscribe to normalized streams
   - Build relationships
   - Update graph (Apache AGE)
   - Update PostgreSQL

**Deliverables**:
- Worker pool processing messages
- Normalizer working
- Correlator building relationships

### Phase 2: Graph & Risk Engine (Week 4-5)

#### Sprint 2.1: Apache AGE Integration
**Priority**: CRITICAL

**Tasks**:
1. **AGE Setup**
   - AGE extension installation
   - Graph schema design
   - Connection setup

2. **Graph Engine**
   - Graph builder
   - Relationship queries
   - Graph traversal
   - Performance optimization

3. **Dual-Write**
   - Write to PostgreSQL
   - Write to AGE graph
   - Consistency handling

**Deliverables**:
- AGE graph working
- Relationships stored in graph
- Graph queries functional

#### Sprint 2.2: Risk Engine
**Priority**: HIGH

**Tasks**:
1. **Risk Rules**
   - All RBAC risk rules
   - Risk scoring
   - Insight creation

2. **Risk Worker**
   - Subscribe to correlation events
   - Evaluate risks
   - Create insights
   - Store in database

3. **Insights API**
   - REST endpoints
   - Filtering
   - Summary statistics

**Deliverables**:
- Risk rules working
- Insights created
- API functional

### Phase 3: Dashboard & API (Week 6-7)

#### Sprint 3.1: REST API
**Priority**: HIGH

**Tasks**:
1. **API Implementation**
   - All endpoints from API_SPECIFICATION.md
   - Authentication middleware
   - Authorization checks
   - Error handling

2. **Graph API**
   - Graph query endpoints
   - Cypher query support
   - Performance optimization

3. **WebSocket API**
   - Real-time updates
   - Event streaming
   - Connection management

**Deliverables**:
- All APIs functional
- Authentication working
- Graph API working

#### Sprint 3.2: Dashboard
**Priority**: MEDIUM

**Tasks**:
1. **Dashboard Foundation**
   - React setup
   - Routing
   - Authentication UI
   - Layout

2. **Core Views**
   - Dashboard overview
   - Graph visualization
   - ServiceAccounts view
   - Risk insights view
   - Audit logs view

**Deliverables**:
- Dashboard functional
- All views working
- Graph visualization

### Phase 4: Advanced Features (Week 8+)

#### Sprint 4.1: Policy Engine
**Tasks**:
- Behavior profiling
- Policy generation
- Policy preview

#### Sprint 4.2: Controller
**Tasks**:
- KubeArmor adapter
- NetworkPolicy generator
- Policy application

#### Sprint 4.3: eBPF Integration
**Tasks**:
- eBPF program
- Event collection
- Integration with agent

## Key Implementation Decisions

### 1. Start with Event-Driven Architecture
- **Decision**: Implement NATS JetStream từ đầu
- **Reason**: Scalability, decoupling, resilience
- **Impact**: All components communicate via messages

### 2. Use Apache AGE for Graph
- **Decision**: PostgreSQL + AGE extension
- **Reason**: Native graph queries, performance
- **Impact**: Dual storage (PostgreSQL + AGE)

### 3. TimescaleDB for Events
- **Decision**: TimescaleDB extension
- **Reason**: Time-series optimization
- **Impact**: Better event query performance

### 4. mTLS from Start
- **Decision**: Implement mTLS immediately
- **Reason**: Security requirement
- **Impact**: Certificate management needed

### 5. Worker Pool Pattern
- **Decision**: Async processing với workers
- **Reason**: Scalability, resilience
- **Impact**: Message-based architecture

## Immediate Next Steps

### Today (Day 1)
1. ✅ Cleanup completed
2. ✅ Documentation analyzed
3. ⏳ Create detailed task list
4. ⏳ Setup project structure
5. ⏳ Initialize infrastructure

### This Week
1. Infrastructure setup (PostgreSQL, NATS, Redis)
2. Project structure verification
3. Core component skeletons
4. Proto definitions
5. Database schema design

### Next Week
1. Agent implementation
2. Ingest API với NATS
3. Worker pool setup
4. Basic data flow

## Success Criteria

### Phase 0 Complete
- ✅ All infrastructure running
- ✅ Project structure clean
- ✅ Components buildable

### Phase 1 Complete
- ✅ Agent collecting data
- ✅ Data flowing through NATS
- ✅ Workers processing messages
- ✅ Data stored correctly

### Phase 2 Complete
- ✅ Graph database working
- ✅ Risk engine creating insights
- ✅ APIs functional

### Phase 3 Complete
- ✅ Dashboard displaying data
- ✅ All views working
- ✅ Real-time updates

## Risk Mitigation

### Technical Risks
- **Complexity**: Incremental development, thorough testing
- **Performance**: Async processing, caching, optimization
- **Security**: Security-first approach, regular audits

### Process Risks
- **Scope Creep**: Clear phases, strict acceptance criteria
- **Timeline**: Buffer time, priority-based development
- **Quality**: Testing at each phase, code reviews

## Notes

- Follow documentation specifications strictly
- Implement incrementally with testing
- Maintain code quality standards
- Document as we go
- Regular progress reviews


