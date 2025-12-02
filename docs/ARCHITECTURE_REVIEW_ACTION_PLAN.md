# Architecture Review Action Plan

**Date**: 2025-11-30  
**Status**: 🟡 IN PROGRESS  
**Priority**: P0 - CRITICAL

---

## Executive Summary

Based on the Architecture Review document, there are **7 CRITICAL issues** and **12 HIGH-priority gaps** that must be addressed. This document outlines the implementation plan to fix these issues.

**Timeline**: 5 weeks to production-ready

---

## ✅ COMPLETED

### Agent Connection Fix
- ✅ Fixed service name mismatch (core → ksam-core)
- ✅ Added ServerName to TLS config for proper hostname verification
- ✅ Agent now successfully connects and streams data to Core

---

## 🔴 CRITICAL ISSUES (P0 - Week 1)

### Issue #1: YAML Rule System Integration ✅ PARTIALLY DONE

**Status**: ✅ YAML rules implemented, ⚠️ Need to verify integration in data flow

**Current State**:
- ✅ YAML rules loading implemented
- ✅ YAMLEngine created
- ✅ Rules loaded from `core/rules/` directory
- ⚠️ Need to verify Risk Worker uses YAML rules correctly

**Action Items**:
1. ✅ Verify Risk Worker loads YAML rules (DONE - already implemented)
2. ⏳ Add hot-reload capability (Week 1)
3. ⏳ Add CEL expression evaluation (Week 2)
4. ⏳ Update architecture diagram (Week 1)

**Files to Update**:
- `KSAM/docs/ARCHITECTURE.md` - Update Risk Engine flow diagram

---

### Issue #2: Inconsistent Event Flow

**Status**: ⚠️ NEEDS FIX

**Problem**: Multiple conflicting event flow patterns documented

**Required Fix**:
```
Agent → Ingest API → NATS (ksam.raw.*) → Normalizer → NATS (ksam.normalized.*) → Workers
```

**Action Items**:
1. ⏳ Standardize NATS subject hierarchy (Week 1)
2. ⏳ Fix all worker subscriptions (Week 1)
3. ⏳ Create NATS subject documentation (Week 1)
4. ⏳ Implement durable consumer naming (Week 1)

**Files to Update**:
- `KSAM/core/pkg/worker/*.go` - Fix subscriptions
- `KSAM/core/pkg/messaging/publisher.go` - Standardize subjects
- `KSAM/docs/ARCHITECTURE.md` - Update event flow diagram

---

### Issue #3: Missing Error Handling & Retry Strategy

**Status**: ⚠️ NEEDS IMPLEMENTATION

**Action Items**:
1. ⏳ Add message acknowledgment strategy (Week 1)
2. ⏳ Implement retry logic with exponential backoff (Week 2)
3. ⏳ Set up Dead Letter Queue (DLQ) (Week 1)
4. ⏳ Add circuit breaker (Week 2)
5. ⏳ Create error classification (Week 1)

**Files to Create/Update**:
- `KSAM/core/pkg/worker/retry.go` - Retry logic
- `KSAM/core/pkg/worker/circuit_breaker.go` - Circuit breaker
- `KSAM/core/pkg/messaging/dlq.go` - Dead letter queue

---

### Issue #4: No Rate Limiting or Backpressure

**Status**: ⚠️ NEEDS IMPLEMENTATION

**Action Items**:
1. ⏳ Implement rate limiting in Ingest API (Week 1)
2. ⏳ Configure NATS stream limits (Week 1)
3. ⏳ Add backpressure handling to workers (Week 2)
4. ⏳ Add monitoring for queue depth (Week 1)

**Files to Create/Update**:
- `KSAM/core/internal/ingest/rate_limiter.go` - Rate limiting
- `KSAM/core/pkg/worker/backpressure.go` - Backpressure handling

---

### Issue #5: Security Gaps in mTLS Implementation

**Status**: ✅ BASIC mTLS DONE, ⚠️ NEEDS ENHANCEMENT

**Current State**:
- ✅ mTLS implemented and working
- ✅ Certificates generated and mounted
- ⚠️ Missing: CSR-based bootstrap, cert rotation, CRL

**Action Items**:
1. ⏳ Document complete mTLS flow (Week 1)
2. ⏳ Implement CSR-based bootstrap (Week 2)
3. ⏳ Add cert rotation logic (Week 2)
4. ⏳ Implement CRL checking (Week 2)

**Files to Create/Update**:
- `KSAM/core/internal/grpc/csr_handler.go` - CSR processing
- `KSAM/agent/internal/cert/rotation.go` - Cert rotation
- `KSAM/docs/MTLS_IMPLEMENTATION.md` - Complete documentation

---

### Issue #6: Incomplete Apache AGE Graph Integration

**Status**: ⚠️ NEEDS IMPLEMENTATION

**Action Items**:
1. ⏳ Document AGE schema (Week 1)
2. ⏳ Implement PostgreSQL triggers for sync (Week 2)
3. ⏳ Create graph indexes (Week 2)
4. ⏳ Performance test graph queries (Week 3)

**Files to Create/Update**:
- `KSAM/core/migrations/003_age_schema.sql` - Graph schema
- `KSAM/core/migrations/004_age_triggers.sql` - Sync triggers
- `KSAM/docs/GRAPH_DATABASE.md` - Documentation

---

### Issue #7: No Database Migration Strategy

**Status**: ⚠️ NEEDS IMPLEMENTATION

**Current State**:
- ✅ Basic migrations exist
- ⚠️ No versioning, no rollback, no leader election

**Action Items**:
1. ⏳ Add golang-migrate to project (Week 1)
2. ⏳ Implement leader election for migrations (Week 1)
3. ⏳ Create migration testing (Week 2)
4. ⏳ Document rollback procedure (Week 2)

**Files to Create/Update**:
- `KSAM/core/migrations/migrate.go` - Migration runner with leader election
- `KSAM/core/migrations/README.md` - Migration guide

---

## 🟡 HIGH-PRIORITY ISSUES (P1 - Week 2-3)

### Issue #8: Missing Prometheus Metrics

**Action Items**:
1. ⏳ Define all metrics (Week 1)
2. ⏳ Implement Prometheus client (Week 2)
3. ⏳ Create Grafana dashboards (Week 2)

### Issue #9: Incomplete eBPF Event Filtering

**Action Items**:
1. ⏳ Implement BPF maps (Week 3)
2. ⏳ Add user-space filtering (Week 3)

### Issue #10: No Disaster Recovery Plan

**Action Items**:
1. ⏳ Implement backup automation (Week 3)
2. ⏳ Create recovery playbook (Week 3)

### Issue #11: Unclear Multi-Cluster Architecture

**Action Items**:
1. ⏳ Document multi-cluster architecture (Week 2)
2. ⏳ Design aggregator component (Week 3)

---

## 📋 Implementation Schedule

### Week 1 (CRITICAL - Must Do)
**Focus**: Fix blocking issues

1. ✅ **Agent Connection** - DONE
2. ⏳ **Event Flow Standardization** (2 days)
3. ⏳ **Error Handling Framework** (2 days)
4. ⏳ **Rate Limiting** (1 day)
5. ⏳ **mTLS Documentation** (1 day)
6. ⏳ **AGE Schema Documentation** (1 day)
7. ⏳ **Migration Framework** (2 days)

**Deliverables**:
- Standardized NATS subject hierarchy
- Error handling framework
- Rate limiting in Ingest API
- Complete mTLS documentation
- AGE graph schema
- Migration framework with leader election

---

### Week 2 (HIGH - Should Do)
**Focus**: Complete critical implementations

1. ⏳ **Retry Logic Implementation** (2 days)
2. ⏳ **CSR-based mTLS Bootstrap** (2 days)
3. ⏳ **AGE Triggers** (2 days)
4. ⏳ **Prometheus Metrics** (2 days)
5. ⏳ **Migration Testing** (1 day)

---

### Week 3 (MEDIUM - Nice to Have)
**Focus**: Performance and reliability

1. ⏳ **Cert Rotation** (2 days)
2. ⏳ **CRL Implementation** (2 days)
3. ⏳ **eBPF Filtering** (2 days)
4. ⏳ **Backup Automation** (1 day)

---

## 🎯 Priority Matrix

```
HIGH IMPACT + HIGH URGENCY (Do First):
- Event Flow Fix
- Error Handling
- Rate Limiting
- Migration Framework

HIGH IMPACT + LOW URGENCY (Do Second):
- CSR Bootstrap
- AGE Integration
- Cert Rotation

LOW IMPACT + HIGH URGENCY (Do Third):
- Metrics Definition
- Documentation

LOW IMPACT + LOW URGENCY (Do Last):
- Multi-cluster
- Capacity Planning
```

---

## 📊 Progress Tracking

### Week 1 Progress
- [x] Agent connection fixed
- [ ] Event flow standardized
- [ ] Error handling framework
- [ ] Rate limiting
- [ ] mTLS documentation
- [ ] AGE schema
- [ ] Migration framework

### Week 2 Progress
- [ ] Retry logic
- [ ] CSR bootstrap
- [ ] AGE triggers
- [ ] Prometheus metrics
- [ ] Migration testing

---

## 🚀 Next Steps

1. **Immediate** (Today):
   - ✅ Fix agent connection - DONE
   - ⏳ Start event flow standardization
   - ⏳ Begin error handling framework

2. **This Week**:
   - Complete all Week 1 critical items
   - Test and verify fixes

3. **Next Week**:
   - Begin Week 2 items
   - Performance testing

---

**Status**: Ready to begin Week 1 implementation  
**Last Updated**: 2025-11-30


