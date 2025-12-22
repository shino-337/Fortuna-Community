# MVP2 Phase 2.2: CEL Evaluator Implementation Plan

**Date**: 2025-12-08  
**Status**: 🚀 **STARTING**  
**Phase**: MVP2 Phase 2.2 - CEL Evaluator with Caching

---

## 🎯 Objectives

Implement a high-performance CEL evaluator with multi-level caching to support policy evaluation at scale.

## 📋 Requirements

### Core Features
1. **CEL Program Caching**
   - In-memory cache for compiled CEL programs
   - Optional database cache for fast restart
   - Cache invalidation on template updates

2. **Fast Evaluation Path**
   - Pre-compiled CEL programs (no compilation during evaluation)
   - In-memory template/instance lookup
   - Scope matching optimization

3. **Template/Instance Management**
   - Load templates and compile CEL on startup
   - Load instances into memory
   - Periodic refresh (every 5 minutes)

4. **Scope Matching**
   - Cluster pattern matching (e.g., "prod-*")
   - Namespace matching
   - Resource type matching
   - Label selector matching

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────┐
│              CEL Evaluator Architecture              │
├─────────────────────────────────────────────────────┤
│                                                      │
│  ┌──────────────────────┐                          │
│  │  Policy Evaluator    │                          │
│  ├──────────────────────┤                          │
│  │ • CEL Environment    │                          │
│  │ • Program Cache      │ (In-memory)              │
│  │ • Template Cache     │ (In-memory)              │
│  │ • Instance Cache     │ (In-memory)              │
│  └──────────────────────┘                          │
│           │                                          │
│           ├── EvaluateFast()                        │
│           │   (No I/O, <100ms)                      │
│           │                                          │
│           └── LoadTemplates()                       │
│               (On startup, periodic refresh)         │
│                                                      │
└──────────────────────────────────────────────────────┘
```

---

## 📝 Implementation Steps

### Step 1: CEL Environment Setup (1 hour)
- Initialize CEL environment with custom functions
- Define variable types (resource, cluster, namespace)
- Add helper functions (has, matches, etc.)

### Step 2: Program Cache Implementation (2 hours)
- In-memory cache structure
- Cache key generation (template_id:version)
- Cache invalidation logic
- Optional database cache (for fast restart)

### Step 3: Template/Instance Loading (2 hours)
- Load templates from database
- Compile CEL expressions
- Store compiled programs in cache
- Load instances into memory

### Step 4: Scope Matching Logic (2 hours)
- Pattern matching for clusters/namespaces
- Resource type matching
- Label selector matching
- Fast lookup optimization

### Step 5: Fast Evaluation Path (2 hours)
- EvaluateFast() function
- Pre-compiled program execution
- Violation creation
- Error handling

### Step 6: Cache Refresh Mechanism (1 hour)
- Periodic refresh (every 5 minutes)
- Reload templates/instances
- Invalidate stale cache entries

**Total Effort**: 10 hours (1.5 days)

---

## 📄 Files to Create

1. `pkg/policy/evaluator.go` - Main evaluator with caching
2. `pkg/policy/scope_matcher.go` - Scope matching logic
3. `pkg/policy/cel_functions.go` - Custom CEL functions

---

## 🧪 Testing

- Unit tests for CEL compilation
- Unit tests for scope matching
- Performance benchmarks
- Integration tests with real policies

---

**Status**: Ready to implement

