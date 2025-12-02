# Project Restart Plan - KSAM

## Cleanup Status ✅

### Completed Cleanup Steps
1. ✅ Kubernetes namespace deleted
2. ✅ Helm releases removed
3. ✅ Docker images cleaned (minikube + local)
4. ✅ All resources verified cleaned

## Documentation Analysis

### 1. ARCHITECTURE.md (89KB, 3024 lines)
**Key Points**:
- System architecture với Control Plane components
- Agent (DaemonSet) design
- Core Controller với Ingest, Normalizer, Correlator, Risk Engine, Policy Engine, Controller
- Storage: PostgreSQL + ClickHouse/Timescale
- Dashboard (React + TypeScript)
- Current status: MVP-1 ~60% complete

**Components Identified**:
- Agent: Inventory watcher, gRPC client
- Core: Ingest API, Normalizer, Correlator, Risk Engine, Policy Engine, Controller
- Dashboard: React UI với graph visualization
- Storage: PostgreSQL (implemented), ClickHouse (planned)

### 2. API_SPECIFICATION.md (48KB, 2322 lines)
**Key Points**:
- REST API endpoints specification
- gRPC API specification
- Authentication & authorization
- Data models và request/response formats

**APIs Required**:
- REST API for Dashboard
- gRPC API for Agent communication
- Insights API
- Graph API
- Audit API

### 3. SECURITY_GUIDE.md (49KB, 2159 lines)
**Key Points**:
- Security requirements
- Authentication mechanisms
- Authorization (RBAC)
- mTLS for agent-core communication
- Secrets management
- Security best practices

**Security Features**:
- JWT authentication
- Role-based access control
- mTLS (planned)
- Secrets in Kubernetes/Vault

### 4. IMPLEMENTATION_PLAN.md (53KB, 2173 lines)
**Key Points**:
- Implementation phases
- Sprint planning
- Task breakdown
- Milestones
- Acceptance criteria

**Phases Identified**:
- Phase 1: Foundation
- Phase 2: Core features
- Phase 3: Advanced features
- Phase 4: Enterprise features

## Recommended Project Structure

### Phase 1: Foundation Setup (Week 1-2)

#### 1.1 Project Structure ✅
```
KSAM/
├── agent/          # Agent DaemonSet
├── core/           # Core Controller
├── dashboard/      # React Dashboard
├── proto/          # Protocol Buffers definitions
├── helm/           # Helm charts
├── deploy/         # Kubernetes manifests
├── scripts/        # Utility scripts
└── docs/           # Documentation
```

#### 1.2 Infrastructure Setup
- [ ] PostgreSQL database setup
- [ ] Database schema và migrations
- [ ] Docker images build setup
- [ ] Kubernetes namespace và RBAC
- [ ] CI/CD pipeline basics

#### 1.3 Core Components Skeleton
- [ ] Ingest API structure
- [ ] Normalizer structure
- [ ] Correlator structure
- [ ] Risk Engine structure
- [ ] Policy Engine structure
- [ ] Controller structure
- [ ] API layer structure

### Phase 2: Core Implementation (Week 3-6)

#### 2.1 Agent Implementation
- [ ] K8s watchers (Pods, ServiceAccounts, Roles, etc.)
- [ ] gRPC client implementation
- [ ] Agent registration
- [ ] Inventory streaming
- [ ] Heartbeat mechanism
- [ ] DaemonSet deployment

#### 2.2 Core Controller - Data Pipeline
- [ ] Ingest API: Register, StreamInventory, StreamEvents
- [ ] Normalizer: Convert proto/JSON to canonical format
- [ ] Correlator: Build relationships
- [ ] Database models và migrations
- [ ] Graph builder

#### 2.3 Risk Engine
- [ ] RBAC risk rules implementation
- [ ] Insights creation
- [ ] Risk evaluation pipeline
- [ ] Insights API endpoints

#### 2.4 API Layer
- [ ] REST API endpoints
- [ ] Authentication middleware
- [ ] Authorization checks
- [ ] Graph API
- [ ] Insights API
- [ ] Audit API

### Phase 3: Dashboard (Week 7-8)

#### 3.1 Dashboard Setup
- [ ] React project setup
- [ ] Routing
- [ ] Authentication UI
- [ ] Layout components

#### 3.2 Core Views
- [ ] Dashboard overview
- [ ] Graph visualization (D3.js)
- [ ] ServiceAccounts management
- [ ] RBAC analysis view
- [ ] Risk insights view
- [ ] Audit logs view

### Phase 4: Advanced Features (Week 9+)

#### 4.1 Policy Engine
- [ ] Behavior profiling
- [ ] Policy generation
- [ ] Policy preview

#### 4.2 Controller
- [ ] KubeArmor adapter
- [ ] NetworkPolicy generator
- [ ] Policy application

#### 4.3 Integrations
- [ ] eBPF integration (optional)
- [ ] Falco integration (optional)
- [ ] KubeArmor integration

## Implementation Strategy

### Approach: Clean Slate with Best Practices

1. **Start Fresh**
   - Clean codebase structure
   - Follow documentation specifications
   - Implement components incrementally

2. **Incremental Development**
   - Build and test each component
   - Integration testing after each phase
   - Continuous deployment

3. **Testing Strategy**
   - Unit tests for each component
   - Integration tests for data flow
   - End-to-end tests for critical paths

4. **Documentation**
   - Code comments
   - API documentation
   - Deployment guides
   - User guides

## Key Decisions

### 1. Technology Stack
- **Backend**: Go 1.20+
- **Frontend**: React + TypeScript
- **Database**: PostgreSQL
- **Protocol**: gRPC + REST
- **Container**: Docker
- **Orchestration**: Kubernetes

### 2. Architecture Patterns
- **Microservices**: Agent (DaemonSet) + Core (Deployment)
- **Event-driven**: Async processing for risk evaluation
- **Layered**: API → Business Logic → Data Access

### 3. Security First
- Authentication from start
- Authorization checks
- Secure communication (mTLS planned)
- Secrets management

## Next Steps

### Immediate Actions (Today)
1. ✅ Cleanup completed
2. ⏳ Analyze all documentation
3. ⏳ Create detailed implementation plan
4. ⏳ Set up project structure
5. ⏳ Initialize core components

### Week 1 Goals
- Project structure setup
- Database schema design
- Core component skeletons
- Basic CI/CD setup
- Development environment

### Week 2 Goals
- Agent basic implementation
- Core Ingest API
- Normalizer implementation
- Database setup và migrations
- Basic testing

## Success Criteria

### Phase 1 Complete When:
- ✅ Project structure in place
- ✅ Database running và accessible
- ✅ Core components can be built
- ✅ Basic deployment works

### Phase 2 Complete When:
- ✅ Agent collects inventory
- ✅ Core processes data
- ✅ Risk Engine creates insights
- ✅ API endpoints work
- ✅ Dashboard displays data

### MVP Complete When:
- ✅ Full data flow working
- ✅ Risk insights displayed
- ✅ Graph visualization working
- ✅ Basic policy generation
- ✅ All tests passing


