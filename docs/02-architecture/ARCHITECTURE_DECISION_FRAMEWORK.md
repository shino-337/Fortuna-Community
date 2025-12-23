# Fortuna Architecture Decision Framework

**Date:** December 23, 2024  
**Status:** ✅ Updated - Agent-Based is REQUIRED  
**Purpose:** Understand WHY Agent-Based is the only correct architecture  
**References:** ADR-0010

---

## Decision Tree (Simplified)

```
START: Are you building a production security platform?

┌─────────────────────────────────────────────────────────────┐
│ Is this for production use?                                 │
└─────────────────┬───────────────────────────────────────────┘
                  │
        ┌─────────┴─────────┐
        │                   │
       YES                 NO
        │                   │
        ↓                   ↓
┌───────────────┐   ┌───────────────────────────┐
│ AGENT-BASED   │   │ POC/Demo: Core-Only OK    │
│ REQUIRED      │   │ (But plan Agent migration)│
│               │   └───────────────────────────┘
│ Data Gravity  │
│ Network Eff.  │
│ Scalability   │
│ Industry Std  │
└───────────────┘

For Production → AGENT-BASED (no other option)
```

**Simple Rule**: If you're serious about this platform, use Agent-Based from day 1.

---

## The ONLY Correct Architecture: Agent-Based

### Why Agent-Based is Mandatory

| Aspect | Agent-Based (CORRECT) | Core-Only (WRONG) |
|--------|----------------------|------------------|
| **Data Gravity** | ✅ Compute moves to data | ❌ Data moves to compute (inefficient) |
| **Network** | ✅ 50KB findings | ❌ 4GB image transfer (99% waste) |
| **Scalability** | ✅ 10,000+ nodes | ❌ Max ~100 nodes (bottleneck) |
| **SBOM/CVE Location** | ✅ On-node (fast, local) | ❌ Centralized (slow, network) |
| **Resource Distribution** | ✅ Balanced across nodes | ❌ Single pod overload |
| **Industry Standard** | ✅ How all tools work | ❌ Anti-pattern |
| **Cache Efficiency** | ✅ Per-node SBOM cache | ❌ No efficient caching |
| **Fault Tolerance** | ✅ Node-level isolation | ❌ Single point of failure |
| **Production Ready** | ✅ YES | ❌ NO (dev/POC only) |

**Verdict**: Core-Only is architecturally incorrect for production security platforms.

**Legend:** ⭐⭐⭐⭐⭐ = Excellent, ⭐⭐⭐⭐ = Good, ⭐⭐⭐ = Acceptable, ⭐⭐ = Poor, ⭐ = Very Poor

---

## Use Case: Production Security Platform

### For ALL Sizes: Agent-Based Required

**Why not "wait until you're bigger":**

❌ **Migration is harder later**
- More data to migrate
- More risk of downtime
- Team has learned wrong patterns
- Technical debt accumulated

❌ **Core-Only creates bad habits**
- Code structured wrong
- Patterns violate data gravity
- Future refactoring is expensive

✅ **Agent-Based from start**
- Scales from 10 to 10,000 nodes
- No migration needed
- Correct patterns from day 1
- Industry-standard approach

**Example: POC/Development**

Even for POC, use Agent-Based:
- Validates production architecture
- No "surprise" migration later
- Proves scalability from start
- DaemonSet is standard K8s (not complex)

---

### Industry Examples (All Use Agent-Based)

**Security & Observability Tools:**

| Tool | Architecture | Why |
|------|-------------|-----|
| **Falco** | DaemonSet agents | Runtime security, node-level monitoring |
| **Trivy Operator** | In-cluster scanning | SBOM + CVE on-node |
| **Aqua Security** | Agent-based sensors | Workload scanning locally |
| **Sysdig** | Agent collectors | eBPF + local processing |
| **Datadog** | Node agents | Metrics/logs from source |
| **Prometheus** | Per-node exporters | Data collected where it lives |

**Pattern**: Heavy data collection happens at the edge.

**Why Fortuna Should Follow**:
- SBOM extraction: I/O + CPU intensive
- CVE scanning: CPU intensive
- Image data: GBs per image
- Network: Expensive resource
- Cache: More efficient per-node

**Conclusion**: Agent-Based is not "our preference", it's **industry consensus**.

---

### Multi-Cluster (Enterprise)

**Requirement:** Agent-Based (Only Option)

**Architecture:**
```
┌────────────────┐   ┌────────────────┐   ┌────────────────┐
│  Cluster A     │   │  Cluster B     │   │  Cluster C     │
│  (us-east-1)   │   │  (us-west-1)   │   │  (eu-west-1)   │
│                │   │                │   │                │
│  ┌──────────┐  │   │  ┌──────────┐  │   │  ┌──────────┐  │
│  │  Agents  │  │   │  │  Agents  │  │   │  │  Agents  │  │
│  │(DaemonSet)│  │   │  │(DaemonSet)│  │   │  │(DaemonSet)│  │
│  └─────┬────┘  │   │  └─────┬────┘  │   │  └─────┬────┘  │
└────────┼───────┘   └────────┼───────┘   └────────┼───────┘
         │                    │                    │
         │                    │                    │
         └────────────────────┼────────────────────┘
                              │
                              │ gRPC (over VPN/VPC peering)
                              │
                              ↓
                   ┌──────────────────┐
                   │   Central Core   │
                   │  (Separate VPC)  │
                   │                  │
                   │  • PostgreSQL    │
                   │  • NATS          │
                   │  • API Server    │
                   │  • Dashboard     │
                   └──────────────────┘
```

**Rationale:**
- Core cannot directly access multiple cluster APIs
- Agents deployed in each cluster
- Central Core for unified view
- Network resilience (agents buffer if Core unreachable)

**Example Use Cases:**
- Multi-region deployments
- Disaster recovery setups
- Hybrid cloud (on-prem + cloud)
- Multi-tenant with cluster per tenant

---

### Scenario 5: Edge/IoT Deployments

**Recommended:** Agent-Based with Offline Buffer

**Special Considerations:**
- Unreliable network to Core
- Agents must buffer locally
- Disk spillover critical
- Periodic sync when connected

**Architecture Additions:**
- Larger disk buffer (100GB+)
- SQLite local cache
- Batch compression
- Prioritized sync (critical insights first)

---

## Decision Factors Deep Dive

### Factor 1: Cluster Size

| Nodes | Pods | Recommendation | Justification |
|-------|------|----------------|---------------|
| < 10 | < 100 | Core-Only | Simplicity outweighs scalability |
| 10-50 | 100-500 | Either | Depends on growth plans |
| 50-200 | 500-2000 | Agent-Based | Better scaling, fault tolerance |
| 200+ | 2000+ | Agent-Based (Required) | Core-only will bottleneck |

---

### Factor 2: Growth Rate

**Slow Growth (< 20% per year):**
- Core-Only acceptable if currently small
- Plan migration to Agent-Based when hit limits

**Medium Growth (20-50% per year):**
- Agent-Based recommended
- Avoid re-architecture later

**Fast Growth (> 50% per year):**
- Agent-Based required
- Build for scale from day one

---

### Factor 3: Availability Requirements

| SLA | Downtime Tolerance | Recommendation | Why |
|-----|-------------------|----------------|-----|
| 99% | 7h/month | Core-Only OK | Pod restart acceptable |
| 99.9% | 43m/month | Agent-Based | Fault isolation needed |
| 99.99% | 4m/month | Agent-Based + HA Core | Multi-pod Core required |

---

### Factor 4: Budget Constraints

**Low Budget (Startup/POC):**
- Start with Core-Only
- Plan migration path
- Monitor resource usage
- Migrate when needed (50+ nodes)

**Medium Budget (Growing Business):**
- Agent-Based from start
- Avoid re-architecture costs
- Better long-term TCO

**High Budget (Enterprise):**
- Agent-Based with full HA
- Multi-region Core
- Dedicated monitoring
- Comprehensive testing

---

### Factor 5: Team Expertise

**Small Team (1-2 engineers):**
- Core-Only initially
- Lower operational burden
- Easier troubleshooting

**Medium Team (3-5 engineers):**
- Agent-Based recommended
- Team can handle complexity
- Better division of labor

**Large Team (6+ engineers):**
- Agent-Based
- Dedicated agent/core teams
- Advanced features (multi-cluster)

---

## Migration Decision Matrix

| Current State | Target State | When to Migrate | Migration Path |
|---------------|--------------|-----------------|----------------|
| Core-Only | Agent-Based | > 50 nodes OR Multi-cluster | Hybrid → Agent |
| Core-Only | Core-Only | < 50 nodes AND Single cluster | Stay (monitor) |
| Agent-Based | Core-Only | Never recommended | N/A |
| Hybrid | Agent-Based | Immediately | Disable Core collection |
| Hybrid | Core-Only | Only if rollback needed | Disable agents |

---

## Cost Analysis

### Total Cost of Ownership (3 Years)

#### Core-Only Architecture

**Infrastructure Costs:**
- Core pod: 2 CPU, 4GB RAM = $150/month
- PostgreSQL: 2 CPU, 4GB RAM, 100GB = $200/month
- NATS: 1 CPU, 2GB RAM = $75/month
- **Total:** $425/month = $15,300/3 years

**Development Costs:**
- Initial setup: 2 weeks = $8,000
- Maintenance: 10 hours/month = $60,000/3 years
- **Total:** $68,000

**Incident Costs:**
- Outages (assume 2/year, 4h each): 24 hours = $48,000
- **Total:** $48,000

**TOTAL 3-YEAR TCO: $131,300**

---

#### Agent-Based Architecture

**Infrastructure Costs:**
- Core pod (2 replicas): 2x(1 CPU, 2GB RAM) = $200/month
- Agents (50 nodes): 50x(100m CPU, 128MB RAM) = $150/month
- PostgreSQL: Same = $200/month
- NATS: Same = $75/month
- **Total:** $625/month = $22,500/3 years

**Development Costs:**
- Initial setup: 6 weeks = $24,000
- Maintenance: 5 hours/month = $30,000/3 years
- **Total:** $54,000

**Incident Costs:**
- Fewer outages (assume 0.5/year, 1h each): 1.5 hours = $3,000
- **Total:** $3,000

**TOTAL 3-YEAR TCO: $79,500**

**SAVINGS: $51,800 (39% lower)**

---

### Break-Even Analysis

Agent-Based architecture costs more upfront but saves over time:

```
Month 0:  Core-Only cheaper by $16,000 (dev time)
Month 6:  Core-Only cheaper by $14,800
Month 12: Core-Only cheaper by $12,600
Month 18: Core-Only cheaper by $10,400
Month 24: BREAK-EVEN
Month 30: Agent-Based cheaper by $2,000
Month 36: Agent-Based cheaper by $51,800
```

**Conclusion:** If you plan to run KSAM for 2+ years, Agent-Based is more cost-effective.

---

## Risk Assessment

### Core-Only Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Core pod crash | Medium | High | Pod disruption budget, auto-restart |
| Scalability limit | High | Critical | Monitor, plan migration |
| API server overload | Medium | High | Rate limiting, caching |
| Development bottleneck | Medium | Medium | Code modularity |

**Overall Risk:** **MEDIUM-HIGH**

---

### Agent-Based Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Agent pod crash | Medium | Low | DaemonSet auto-restart, isolated |
| Network partition | Low | Medium | Local buffering, retry |
| Complex debugging | Medium | Low | Good logging, metrics |
| Higher initial cost | High | Low | Budget planning |

**Overall Risk:** **LOW-MEDIUM**

---

## Final Recommendation Algorithm

```python
def recommend_architecture(cluster_count, node_count, pod_count, growth_rate, budget, team_size):
    """
    Returns: "core-only" or "agent-based"
    """

    # Multi-cluster always requires agents
    if cluster_count > 1:
        return "agent-based"

    # Large clusters require agents
    if node_count > 200 or pod_count > 2000:
        return "agent-based"

    # Fast growth requires agents
    if growth_rate > 0.5:  # 50% per year
        return "agent-based"

    # Small cluster + low budget + small team = Core-Only
    if node_count < 50 and budget == "low" and team_size < 3:
        return "core-only"

    # Medium cluster + medium budget = Agent-Based
    if node_count >= 50 or budget == "medium":
        return "agent-based"

    # Default to Agent-Based for future-proofing
    return "agent-based"
```

---

## Checklist: Are You Ready for Agent-Based?

Before migrating to Agent-Based, ensure:

**Technical Readiness:**
- [ ] Team understands DaemonSet management
- [ ] Team understands gRPC debugging
- [ ] Monitoring infrastructure in place
- [ ] Network allows pod-to-pod communication
- [ ] TLS certificates can be provisioned
- [ ] Helm/kubectl deployment automation

**Operational Readiness:**
- [ ] Backup/restore procedures tested
- [ ] Rollback plan documented
- [ ] Runbooks created
- [ ] On-call rotation defined
- [ ] Incident response process

**Business Readiness:**
- [ ] Budget approved (initial dev cost)
- [ ] Stakeholders aligned
- [ ] Maintenance window scheduled
- [ ] Success metrics defined
- [ ] Rollback criteria agreed

**If < 80% checked:** Stick with Core-Only for now

**If ≥ 80% checked:** Proceed with Agent-Based migration

---

## Common Mistakes to Avoid

### Mistake 1: Premature Optimization

**Problem:** Implementing Agent-Based for 10-node cluster

**Better:** Start with Core-Only, migrate when needed

**When to optimize:** When you feel the pain (slow collection, high resource usage)

---

### Mistake 2: Analysis Paralysis

**Problem:** Spending 3 months analyzing which architecture

**Better:** Start with Core-Only if < 50 nodes, Agent-Based if ≥ 50 nodes

**Rule:** Make a decision in 1 week, adjust later if needed

---

### Mistake 3: No Migration Plan

**Problem:** Building Core-Only with no path to Agent-Based

**Better:** Design Core-Only with feature flags for future migration

**Key:** Use hybrid mode for gradual migration

---

### Mistake 4: Ignoring Growth

**Problem:** Building for current size (20 nodes), ignoring 200% annual growth

**Better:** Build for 2-year projected size

**Calculation:** If 20 nodes today and 200% growth, expect 180 nodes in 2 years → Use Agent-Based

---

## Summary: ONE Decision

| Scenario | Architecture | Rationale |
|----------|-------------|-----------|
| **Production** | Agent-Based | Only correct option |
| **POC/Demo** | Agent-Based (preferred) | Validates production arch |
| **POC/Demo (lazy)** | Core-Only | OK for throw-away demo, BUT plan migration |
| **Multi-cluster** | Agent-Based | Only option |
| **Enterprise** | Agent-Based | Only option |
| **Any size** | Agent-Based | Scales from 10 to 10,000 nodes |

**Rule**: If you plan to use this in production, use Agent-Based from day 1.

**No "wait until you're bigger".** Start correctly.

---

## Next Steps (Agent-Based Implementation)

**You chose correctly. Now implement it:**

### Phase 1: Move SBOM to Agent (Week 1-2)

1. Read `REFACTORING_PLAN_AGENT_BASED.md`
2. Read `IMPLEMENTATION_CODE_EXAMPLES.md`
3. Migrate `pkg/sbom/extractor/` to `agent/`
4. Define proto contract (Agent→Core)
5. Agent generates + sends SBOM findings
6. Core receives + stores

### Phase 2: Move CVE Scan to Agent (Week 2-3)

1. Embed trivy/grype library in agent
2. Agent runs local CVE scan
3. Agent sends vulnerability findings
4. Core processes insights
5. Remove CVE logic from Core

### Phase 3: Production Ready (Week 3-4)

1. Add caching (SBOM per digest)
2. Rate limiting (scans per minute)
3. Metrics + monitoring
4. Health checks
5. Load testing (1000 pods)

**Timeline**: 3-4 weeks  
**Team**: 2 engineers  
**Cost**: $40k (one-time)

See **ADR-0010** for detailed rationale.

---

**Remember:** 

> "Architecture should be correct first, simple second."

For security platforms: Agent-Based is architecturally correct.

**Don't compromise on correctness for perceived simplicity.**

Core-Only might seem "simpler" but creates technical debt immediately.

---

*Fortuna K8s Management Platform*  
*Agent-Based Architecture - The Only Correct Choice*  
*December 23, 2024*
