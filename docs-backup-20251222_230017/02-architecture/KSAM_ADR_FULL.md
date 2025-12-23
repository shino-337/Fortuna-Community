# KSAM Architecture Decision Records (ADR)

This document consolidates all major architectural decisions for the KSAM
(Container Security & Observability Platform). These ADRs are intended to be
the single source of truth for design, implementation, and future evolution.

---

## KSAM – Detailed Architecture Decision Records

### Mục tiêu của bộ ADR này

- Xác định điểm bất biến (invariants) của kiến trúc
- Làm rõ vì sao chọn – vì sao không chọn
- Tránh “thiết kế theo cảm tính” khi mở rộng

### Làm nền tảng cho

- Attack Path
- Risk Prioritization
- Multi-cluster / Multi-tenant
- Runtime Security

---

## ADR-0001: Core Architecture Pattern

### Status

**Accepted**

### Decision

Sử dụng Modular Monolith cho Control Plane, kết hợp Selective Microservices cho các workload nặng hoặc có profile tài nguyên khác biệt.

### Detailed Context

KSAM xử lý các loại workload rất khác nhau:

| Workload | Đặc điểm |
| --- | --- |
| Inventory ingest | I/O bound, frequent |
| Risk scoring | CPU light, transactional |
| Policy evaluation | CPU light, latency sensitive |
| Graph traversal | Memory heavy, long-running |
| SBOM generation | CPU + IO heavy |
| CVE matching | CPU moderate, batch |

- Nếu ép tất cả chạy trong một process → resource contention
- Nếu tách tất cả thành microservice → overhead & inconsistency

### Detailed Decision

#### Control Plane (Modular Monolith)

**Bao gồm:**
- Ingest API
- Inventory Engine
- Risk Engine
- Policy Engine
- Correlation / Orchestration layer

**Đặc điểm:**
- Chia module rõ ràng ở code-level
- Chung DB transaction
- Không gRPC nội bộ

**Lý do bắt buộc:**
- Risk cần inventory + policy + RBAC tại cùng thời điểm
- Attack Path cần snapshot nhất quán
- Giảm latency & race condition

#### Isolated Components (Selective Microservices)

| Component | Vì sao phải tách |
| --- | --- |
| Graph Engine | Query dài, RAM cao |
| SBOM/CVE Engine | CPU/IO burst |
| Runtime/eBPF | Kernel-level, crash isolation |

#### Trade-offs

**Chấp nhận:**
- Một số refactor khi scale lớn

**Tránh được:**
- Microservice hell
- Eventual consistency sai lệch risk

---

## ADR-0002: Data Storage & Persistence Strategy

### Status

**Accepted**

### Detailed Context

KSAM có 2 loại dữ liệu bản chất khác nhau:

- State data (what exists)
- Relationship data (how things connect / can be abused)

Không có 1 DB duy nhất xử lý tốt cả hai.

### Decision

Áp dụng Polyglot Persistence có kiểm soát:

- PostgreSQL: state, risk, policy
- Apache AGE (graph extension): attack path

### Detailed Design

#### PostgreSQL – Source of Truth

- ACID transactions
- Referential integrity
- Audit-friendly

**Ví dụ bảng bắt buộc:**
- `workloads`
- `identities` (SA, users)
- `permissions`
- `risks`
- `insights`
- `sboms`
- `cves`

#### Graph Layer – Derived View

- Graph không phải source of truth
- Graph được build từ relational state

**Graph Nodes:**
- Pod
- ServiceAccount
- Role
- Node
- Image
- CVE

**Graph Edges:**
- RUNS_AS
- CAN_ACCESS
- BINDS_TO
- EXPOSES
- AFFECTED_BY

### Consequences

- Có độ trễ nhỏ giữa relational → graph
- Nhưng attack path đúng và explainable

---

## ADR-0003: Agent Design & Data Collection Model

### Status

**Accepted**

### Detailed Context

Agent là thành phần nguy hiểm nhất nếu làm sai:

- Chạy trên mọi node
- Có quyền cao
- Có thể làm sập cluster

### Decision

Agent phải là Data Plane, không phải Decision Plane.

### Detailed Agent Responsibilities

- Watch K8s API
- Thu thập metadata
- Resolve container → pod → SA
- Thu thập runtime signal (Phase 2)
- Forward dữ liệu có buffer + retry

### Explicit Non-Goals

Agent **KHÔNG BAO GIỜ**:

- Chạy risk scoring
- Chạy policy engine
- Build attack path
- Quyết định block workload

### Rationale

- Zero-trust với agent
- Control Plane là nơi duy nhất ra quyết định
- Dễ chứng minh an toàn với auditor

---

## ADR-0004: SBOM Generation & CVE Matching Strategy

### Status

**Accepted**

### Detailed Context

Scan CVE là nguyên nhân #1 gây:

- High CPU
- High IO
- Chậm pod lifecycle
- Support ticket

### Decision

- Scan image (digest-based), không scan pod runtime
- Async, cache-first, non-blocking, event-driven

### Detailed Flow

1. Pod created/updated (inventory event)
2. Resolve image digest (immutable identifier)
3. Lookup SBOM cache by digest (`sboms.image_digest`)
4. Nếu có SBOM → skip generation, chỉ “attach” (link pod → SBOM) và tiếp tục matching/insights
5. Nếu không có SBOM → generate SBOM (extract layers) → persist `sboms` + `sbom_components`
6. Emit `SBOM_CREATED` event → CVE matching async → persist `cve_matches` → create insights → risk scoring

### CVE Database Strategy

- Offline mirror OSV.dev JSON dataset stored at `/cve-data/all`
- Load into PostgreSQL (source of truth) via `core/cmd/cve-loader`:
  - Tables: `cves`, `package_vulnerabilities`
  - Version range fields normalized for fast matching

**Pre-index theo:**
- Ecosystem (debian/ubuntu/alpine/npm/pypi/go…)
- Package name
- Version range boundaries (start/end include/exclude)

### Why not inline scanner (Trivy/Grype)?

- Thêm dependency runtime
- DB sync phức tạp
- Không kiểm soát được perf

### Implementation Notes (CURRENT)

- Pipeline is implemented as event-driven workers inside Core (can be extracted to a dedicated service later for scaling/isolation):
  - **SBOMWorker**: consumes `ksam.normalized.pods` → digest resolution → cache-first SBOM ensure/persist → publish `ksam.sbom.created`
  - **CVEMatcherWorker**: consumes `ksam.sbom.created` → query PostgreSQL CVE tables → persist `cve_matches` (dedup) → create HIGH/CRITICAL vulnerability insights
- RiskWorker is kept **pure** (risk rules + insights lifecycle) and does not perform SBOM/CVE work to avoid contention.

### Event & Storage Contracts (Invariants)

- **Digest-based identity**: SBOM cache key is `image_digest` (immutable); never key by tag.
- **Non-blocking**: SBOM/CVE work must not block admission webhook nor inventory ingest.
- **Dedup & idempotency**:
  - `sboms.image_digest` is unique
  - unique index on `sbom_components(sbom_id, purl)` for safe upserts
  - unique index on `cve_matches(sbom_id, component_id, cve_id)` for safe upserts
- **Traceability**:
  - `pod_image_scans.sbom_id` links pod container → SBOM
  - `insights.sbom_id` and `insights.cve_match_id` link insight → evidence

### Operational Controls

- CVE data source is **PostgreSQL (OSV-loaded)** by default.
- Go toolchain is pinned via `toolchain` directive (requires Go ≥ 1.21 to parse `go.work`/`go.mod` in older environments).

---

## ADR-0005: Risk Scoring & Prioritization Model

### Status

**Accepted**

### Detailed Context

90% alert từ các tool hiện nay không actionable.

### Decision

Risk = Severity × Context × Reachability

### Detailed Factors

| Factor | Ví dụ |
| --- | --- |
| Severity | CVSS |
| Exploitability | Known exploit |
| Exposure | Internet-facing |
| Privilege | Cluster-admin SA |
| Business impact | Prod namespace |

**Output:**
- FinalRiskScore (0–100)
- Priority bucket
- Explanation (why this is high)

---

## ADR-0006: Attack Path Graph & Simulation

### Status

**Accepted**

### Detailed Context

Attack Path là USP thực sự, không phải dashboard đẹp.

### Decision

Attack Path = Graph traversal + rule-based pruning

### Inputs

- Inventory
- RBAC
- Network
- CVE
- Runtime (sau)

### Outputs cho user

- Entry point
- Path steps
- Required capability mỗi bước
- Blast radius
- Suggested break point

### Why graph, not rule chain?

- Rule chain không scale
- Graph explain được “why”

---

## ADR-0007: Policy Engine & Enforcement Strategy

### Status

**Accepted**

### Detailed Context

- Enforce sai → outage
- Không enforce → useless

### Decision

Policy-as-Code + staged enforcement

### Policy Lifecycle

- Detect
- Observe
- Recommend
- Enforce (opt-in)

### Enforcement Points

- Admission Controller
- Runtime (Phase 2)
- Network policy generator

---

## ADR-0008: Deployment, Scaling & Multi-Cluster Strategy

### Status

**Accepted**

### Detailed Context

KSAM hướng tới:

- SaaS
- On-prem
- Hybrid

### Decision

Central Control Plane + Distributed Agents

### Scaling Strategy

| Component | Scale by |
| --- | --- |
| API | Replicas |
| Graph | Memory |
| CVE scan | Workers |
| Agent | Node count |

### Final Architecture Invariant

All security decisions happen in the Control Plane, based on correlated, contextual data.

Nếu sau này có ai đề xuất:
- “Cho agent tự block”
- “Scan pod realtime”
- “Mỗi engine là 1 service”

→ Chỉ cần trỏ họ về file này.