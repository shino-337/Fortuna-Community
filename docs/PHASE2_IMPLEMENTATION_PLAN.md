# Phase 2 Implementation Plan

## Overview

Phase 2 focuses on implementing the Risk Engine, Graph Engine (Apache AGE), and API Layer to enable risk analysis, graph queries, and API access to insights.

## Phase 2 Components

### 2.1 Risk Engine ⭐ CRITICAL
**Priority**: P0  
**Duration**: 1 week

#### Objectives
- Implement risk evaluation rules
- Create insights automatically
- Integrate with data pipeline
- Store insights in database

#### Tasks

1. **Risk Engine Package Structure**
   - Create `pkg/riskengine/` package
   - Define rule structures
   - Define insight structures
   - Create evaluation engine

2. **Risk Rules Implementation**
   - Cluster-admin bindings detection
   - Wildcard permissions detection
   - Orphan ServiceAccount detection
   - Overprivileged roles detection
   - Overprivileged bindings detection

3. **Risk Evaluation Logic**
   - Rule evaluation engine
   - Risk scoring (CVSS-like 0-10)
   - Condition matching
   - Aggregation logic

4. **Insights Management**
   - Create insights from rule matches
   - Prevent duplicates
   - Update existing insights
   - Store affected resources

5. **Risk Engine Worker**
   - Subscribe to correlation events
   - Evaluate risks on resource changes
   - Create insights
   - Publish to insights stream

6. **Integration**
   - Integrate with Correlator via NATS
   - Subscribe to normalized stream
   - Async evaluation
   - Error handling

#### Deliverables
- ✅ Risk rules working
- ✅ Insights created automatically
- ✅ Risk evaluation integrated
- ✅ Insights stored in database

### 2.2 Graph Engine (Apache AGE)
**Priority**: P1  
**Duration**: 1 week

#### Objectives
- Setup Apache AGE extension
- Create graph schema
- Implement graph operations
- Dual-write with Correlator

#### Tasks

1. **Apache AGE Setup**
   - Install AGE extension in PostgreSQL
   - Create graph schema
   - Create vertex labels
   - Create edge labels

2. **AgeGraphEngine Implementation**
   - Graph engine struct
   - Vertex operations
   - Edge operations
   - Query operations

3. **Dual-Write Integration**
   - Update Correlator to write to graph
   - Maintain consistency
   - Handle errors

#### Deliverables
- ✅ AGE graph working
- ✅ Relationships stored in graph
- ✅ Graph queries functional

### 2.3 API Layer
**Priority**: P1  
**Duration**: 1 week

#### Objectives
- Implement REST API endpoints
- Add authentication/authorization
- Graph API endpoints
- Insights API endpoints

#### Tasks

1. **Insights API**
   - GET /api/v1/insights
   - GET /api/v1/insights/:id
   - POST /api/v1/insights/evaluate
   - DELETE /api/v1/insights/:id

2. **Graph API**
   - GET /api/v1/graph
   - POST /api/v1/graph/query
   - GET /api/v1/graph/path

3. **Authentication**
   - JWT implementation
   - Middleware
   - Authorization checks

#### Deliverables
- ✅ All API endpoints working
- ✅ Authentication functional
- ✅ Authorization enforced

## Implementation Order

1. **Risk Engine** (Week 1)
   - Most critical component
   - Enables risk analysis
   - Foundation for insights

2. **API Layer** (Week 2)
   - Expose insights via API
   - Enable dashboard integration
   - Graph API for visualization

3. **Graph Engine** (Week 3)
   - Advanced graph queries
   - Performance optimization
   - Complex relationship analysis

## Success Criteria

- Risk Engine evaluates all resources
- Insights created for all rule matches
- API endpoints return correct data
- Graph queries work efficiently
- System handles 1000+ resources

