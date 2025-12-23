# KSAM Implementation Roadmap

## Project Overview

KSAM (Kubernetes Service Account Manager) - Container Security & Observability Platform

**Goal**: Build a runtime-first container security platform: inventory → relationship graph → runtime telemetry → policy automation → enforcement.

## Current Status

### Cleanup ✅
- ✅ All Kubernetes resources cleaned
- ✅ Docker images removed
- ✅ Fresh start ready

### Documentation Analyzed ✅
- ✅ ARCHITECTURE.md - System design
- ✅ API_SPECIFICATION.md - API contracts
- ✅ SECURITY_GUIDE.md - Security requirements
- ✅ IMPLEMENTATION_PLAN.md - Implementation phases

## Implementation Phases

### Phase 1: Foundation (Week 1-2)

#### Sprint 1.1: Project Setup
**Duration**: 3 days

**Tasks**:
1. **Project Structure**
   - [ ] Verify/clean project structure
   - [ ] Setup Go modules (agent, core)
   - [ ] Setup React project (dashboard)
   - [ ] Setup proto definitions
   - [ ] Setup scripts folder

2. **Development Environment**
   - [ ] Docker setup
   - [ ] Minikube setup
   - [ ] Database setup (PostgreSQL)
   - [ ] Development tools (linters, formatters)

3. **CI/CD Basics**
   - [ ] Build scripts
   - [ ] Test scripts
   - [ ] Docker build configs

**Deliverables**:
- ✅ Clean project structure
- ✅ Development environment ready
- ✅ Basic build scripts

#### Sprint 1.2: Database & Core Skeleton
**Duration**: 4 days

**Tasks**:
1. **Database Schema**
   - [ ] Design database schema based on ARCHITECTURE.md
   - [ ] Create migration scripts
   - [ ] Setup GORM models
   - [ ] Test database connection

2. **Core Components Skeleton**
   - [ ] Ingest API structure
   - [ ] Normalizer structure
   - [ ] Correlator structure
   - [ ] Risk Engine structure
   - [ ] Policy Engine structure
   - [ ] Controller structure

3. **gRPC Proto**
   - [ ] Define proto messages
   - [ ] Generate Go code
   - [ ] Define service interfaces

**Deliverables**:
- ✅ Database schema và migrations
- ✅ Core component skeletons
- ✅ Proto definitions

### Phase 2: Core Implementation (Week 3-6)

#### Sprint 2.1: Agent Implementation
**Duration**: 1 week

**Tasks**:
1. **K8s Watchers**
   - [ ] Pod watcher
   - [ ] ServiceAccount watcher
   - [ ] Role/RoleBinding watchers
   - [ ] ClusterRole/ClusterRoleBinding watchers
   - [ ] Node watcher

2. **gRPC Client**
   - [ ] Client implementation
   - [ ] Registration logic
   - [ ] Inventory streaming
   - [ ] Event streaming
   - [ ] Heartbeat mechanism

3. **Agent Deployment**
   - [ ] DaemonSet manifest
   - [ ] RBAC configuration
   - [ ] ConfigMap for settings
   - [ ] Deployment và testing

**Deliverables**:
- ✅ Agent collecting inventory
- ✅ Agent communicating with core
- ✅ Agent deployed as DaemonSet

#### Sprint 2.2: Core Data Pipeline
**Duration**: 1 week

**Tasks**:
1. **Ingest API**
   - [ ] Register handler
   - [ ] StreamInventory handler
   - [ ] StreamEvents handler
   - [ ] Heartbeat handler
   - [ ] Error handling

2. **Normalizer**
   - [ ] NormalizeInventoryItem
   - [ ] NormalizeEvent
   - [ ] Node ID resolution
   - [ ] Label parsing

3. **Correlator**
   - [ ] Correlate Pods
   - [ ] Correlate ServiceAccounts
   - [ ] Correlate Roles
   - [ ] Build relationships
   - [ ] Update timestamps

4. **Database Integration**
   - [ ] Save normalized data
   - [ ] Update relationships
   - [ ] Transaction handling

**Deliverables**:
- ✅ Data flowing: Agent → Ingest → Normalizer → Correlator → DB
- ✅ Relationships built correctly
- ✅ Data persisted

#### Sprint 2.3: Risk Engine
**Duration**: 1 week

**Tasks**:
1. **Risk Rules Implementation**
   - [ ] Cluster-admin bindings detection
   - [ ] Wildcard permissions detection
   - [ ] Orphan ServiceAccount detection
   - [ ] Overprivileged roles detection
   - [ ] Overprivileged bindings detection

2. **Insights Management**
   - [ ] Create insights
   - [ ] Prevent duplicates
   - [ ] Update existing insights
   - [ ] Store affected resources

3. **Integration**
   - [ ] Integrate with Correlator
   - [ ] Async evaluation
   - [ ] Error handling

**Deliverables**:
- ✅ Risk rules working
- ✅ Insights created automatically
- ✅ Risk evaluation integrated

#### Sprint 2.4: API Layer
**Duration**: 1 week

**Tasks**:
1. **REST API**
   - [ ] Authentication endpoints
   - [ ] ServiceAccounts API
   - [ ] Graph API
   - [ ] Insights API
   - [ ] Audit API

2. **Authentication & Authorization**
   - [ ] JWT implementation
   - [ ] Password hashing
   - [ ] Role-based access control
   - [ ] Middleware

3. **API Documentation**
   - [ ] OpenAPI/Swagger spec
   - [ ] Endpoint documentation

**Deliverables**:
- ✅ All API endpoints working
- ✅ Authentication functional
- ✅ Authorization enforced

### Phase 3: Dashboard (Week 7-8)

#### Sprint 3.1: Dashboard Foundation
**Duration**: 1 week

**Tasks**:
1. **Project Setup**
   - [ ] React + TypeScript setup
   - [ ] Routing setup
   - [ ] State management
   - [ ] API client setup

2. **Authentication UI**
   - [ ] Login page
   - [ ] Token management
   - [ ] Protected routes

3. **Layout Components**
   - [ ] Main layout
   - [ ] Navigation
   - [ ] Sidebar
   - [ ] Header

**Deliverables**:
- ✅ Dashboard structure
- ✅ Authentication working
- ✅ Basic navigation

#### Sprint 3.2: Core Views
**Duration**: 1 week

**Tasks**:
1. **Dashboard Overview**
   - [ ] Statistics cards
   - [ ] Cluster overview
   - [ ] Recent activity

2. **Graph Visualization**
   - [ ] D3.js integration
   - [ ] Node rendering
   - [ ] Edge rendering
   - [ ] Interactive features
   - [ ] Risk indicators

3. **ServiceAccounts View**
   - [ ] List view
   - [ ] Detail view
   - [ ] Filtering
   - [ ] Search

4. **Risk Insights View**
   - [ ] Insights list
   - [ ] Severity filtering
   - [ ] Type filtering
   - [ ] Detail view

5. **Audit Logs View**
   - [ ] Logs table
   - [ ] Filtering
   - [ ] Export

**Deliverables**:
- ✅ All core views implemented
- ✅ Graph visualization working
- ✅ Data displayed correctly

### Phase 4: Advanced Features (Week 9+)

#### Sprint 4.1: Policy Engine
**Duration**: 2 weeks

**Tasks**:
1. **Behavior Profiling**
   - [ ] Event analysis
   - [ ] Pattern detection
   - [ ] Baseline creation

2. **Policy Generation**
   - [ ] LSM/KubeArmor policy
   - [ ] NetworkPolicy generation
   - [ ] Least-privilege application

3. **Policy Preview**
   - [ ] Simulation logic
   - [ ] False positive estimation
   - [ ] Preview UI

**Deliverables**:
- ✅ Policies generated
- ✅ Preview functional
- ✅ False positive rate < 10%

#### Sprint 4.2: Controller
**Duration**: 2 weeks

**Tasks**:
1. **KubeArmor Adapter**
   - [ ] Policy translation
   - [ ] CRD creation
   - [ ] Application logic

2. **NetworkPolicy Generator**
   - [ ] Policy generation
   - [ ] Application via K8s API

3. **Policy Management**
   - [ ] Monitor mode
   - [ ] Enforcement mode
   - [ ] Status tracking

**Deliverables**:
- ✅ Policies applied
- ✅ Monitor mode working
- ✅ Enforcement mode working

## Technical Specifications

### Components

#### Agent
- **Language**: Go
- **Type**: DaemonSet
- **Communication**: gRPC
- **Functions**: Inventory collection, event forwarding

#### Core
- **Language**: Go
- **Type**: Deployment
- **APIs**: gRPC (9090) + REST (8080)
- **Components**: Ingest, Normalizer, Correlator, Risk Engine, Policy Engine, Controller

#### Dashboard
- **Language**: TypeScript + React
- **Type**: Deployment
- **Port**: 80
- **Features**: Graph visualization, RBAC analysis, Risk insights

### Database

#### PostgreSQL
- **Purpose**: Normalized state, relationships, insights
- **Tables**: 17+ tables
- **Features**: JSONB support, relationships, indexes

#### ClickHouse/Timescale (Future)
- **Purpose**: High-volume events
- **Features**: TTL, compression, time-series

### Security

#### Authentication
- **Method**: JWT
- **Storage**: HTTP-only cookies or localStorage
- **Expiration**: Configurable

#### Authorization
- **Method**: Role-based (admin, user, viewer)
- **Enforcement**: Middleware

#### Communication
- **Agent-Core**: gRPC (mTLS planned)
- **Dashboard-Core**: HTTPS (TLS)

## Success Metrics

### Phase 1
- ✅ Project structure complete
- ✅ Database accessible
- ✅ Components buildable

### Phase 2
- ✅ Agent collecting data
- ✅ Core processing data
- ✅ Risk insights created
- ✅ APIs functional

### Phase 3
- ✅ Dashboard displaying data
- ✅ Graph visualization working
- ✅ All views functional

### Phase 4
- ✅ Policies generated
- ✅ Policies applied
- ✅ Monitor mode working

## Risk Mitigation

### Technical Risks
- **Complexity**: Incremental development, thorough testing
- **Performance**: Async processing, caching
- **Security**: Security-first approach, regular audits

### Process Risks
- **Scope Creep**: Clear phases, strict acceptance criteria
- **Timeline**: Buffer time, priority-based development
- **Quality**: Testing at each phase, code reviews

## Next Immediate Steps

1. **Verify Project Structure** (30 min)
   - Check current structure
   - Identify what to keep/remove

2. **Setup Development Environment** (1 hour)
   - PostgreSQL setup
   - Minikube verification
   - Docker setup

3. **Start Phase 1.1** (Today)
   - Project structure cleanup
   - Go modules setup
   - Basic build scripts

4. **Database Schema Design** (Tomorrow)
   - Review ARCHITECTURE.md requirements
   - Design schema
   - Create migration scripts

## Notes

- Follow documentation specifications strictly
- Implement incrementally with testing
- Maintain code quality standards
- Document as we go
- Regular progress reviews


