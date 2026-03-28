# FORTUNA_RUNTIME_IMPLEMENTATION_BACKLOG.md

## Status
Draft v0.2

### Progress update (2026-03-28)
- Da tong hop toan bo GAP con mo vao **Muc 9** (cross-doc: backlog P1/P2, Pod Detail R5–R10, van hanh Falco/agent).
- **Phase A (van hanh) — da thuc hien trong repo:** dong bo `deploy/fortuna-agent-daemonset.yaml` (memory **6Gi** limit, CPU **1**, request memory **1Gi**, `SBOM_WORKERS=1`, comment Falco tail EOF + rotate); code agent `FalcoReader` **tail tu EOF** lan dau, **resolve pod UID** bang `Pods.List` + `fieldSelector=metadata.name` khi `Get` loi RBAC; tai lieu build **`nerdctl -n k8s.io`** + tag `docker.io/library/fortuna-agent:latest` trong `agent/README.md`, `deploy/README.md` (bang checklist Agent), comment trong `scripts/utils/push-images-to-workers.sh`; **Muc 9.6** ghi chi tiet + bang API source-of-truth (phan nhe G-API-01).
- Cac muc Phase A con lai (tuy moi truong): baseline Helm values neu tach khoi `deploy/fortuna-core-deployment.yaml`; cac buoc clean/pipeline day du khi can rebuild.
- Da them: G-R10 buoc CI `verify-admission-risk-gate.sh`; G-API-01 route `/api/v2/runtime/pods/:uid/capabilities`.
- **Muc 9.0:** dieu chinh uu tien backlog — **semantic lock truoc scorer v3 / graph / coverage formal**; map GAP moi (G-SEM-*, G-REP-GOV); **9.3** ke hoach phase cap nhat theo thu tu nay.
- **2026-03-28 (session):** Phase S + G-REP-GOV + G-EXP-01 MVP + G-RE-01 scaffold: xem **9.7** va `GAP_IMPLEMENTATION_STATUS.md`; ADR `docs/adr/001-005`; test bundle `go test ./core/pkg/capability/... ./pkg/models/... ./pkg/rep/... ./pkg/explainability/... ./pkg/riskengine/... ./internal/api/...`.
- **2026-03-28 (tiep):** G-REP-01 confidence merge; G-RE-01 `score_v3_preview` trong `risk_scores.factors`; G-EXP-01 `?enrich=1`; cap nhat `GAP_IMPLEMENTATION_STATUS.md`.

### Progress update (2026-03-26)
- Da hoan thanh P0.2 nen tang DB/model cho `runtime_behavior_facts`, `runtime_incidents`.
- Da hoan thanh P0.5/P0.5b ban toi thieu: `asset_security_state` + compatibility bridge cho `object.fortuna.*`.
- Da hoan thanh P0.3 ban toi thieu: runtime-first capability init trong REP (`InitializeCapability` truoc `PromoteCapability`).
- Da hoan thanh P0.4 ban toi thieu: signal registry cho REP v2 compare-path.
- Da mo read API layer moi (`/api/v2/runtime/pods/:uid/{security-state,facts,incidents}`) va giu on dinh API v1.
- Da implement REP-C minimal detector `RECON_BURST` (stateful) va persist vao `runtime_incidents`.
- Da bo sung REP-C detector `POST_EXPLOIT_EXEC_CHAIN` (stateful) vao `runtime_incidents`.
- Da bo sung REP-C detector `EXFIL_LIKE_SEQUENCE` (stateful, minimal ordering) vao `runtime_incidents`.
- REP-C hien o muc mo rong; da bo sung cooldown/suppression de giam incident spam (con tuning threshold/suppression fine-grained).
- Ghi chu verify: `go test ./core/internal/api/...`, `./core/pkg/rep/...`, `./core/pkg/capability/...`, `./core/pkg/riskengine/...`, `./core/migrations/...` pass sau cac thay doi P0.
- Ghi chu verify full flow Agent/Core/DB/API: `go test ./agent/internal/runtime/...`, `./core/pkg/rep/...`, `./core/internal/api/...`, `./core/migrations/...` pass; co test dong bo luong `runtime_v2_flow_sync_test.go`.
- Verify contract v2 with agent payload changes: `go test ./agent/internal/runtime/...` pass; Core v2 ingest tests pass (bao gom nested source + flattened source compatibility).
- Mo rong verify API v2 `security-state`: da co test cho ca 2 case table missing (404) va table+data present (200).
- Mo rong quality REP-C: da bo sung negative tests cho 3 detector (`RECON_BURST`, `POST_EXPLOIT_EXEC_CHAIN`, `EXFIL_LIKE_SEQUENCE`) de dam bao khong emit incident khi khong du dieu kien.
- Agent changes (P0.1 contract emit): da cap nhat runtime sensors (FalcoReader + RuntimeEvents Reader) co them event_id/observed_at/ingested_at/source_* + payload_json/payload_hash voi fallback POST v2->v1.
- Core v2 ingest: da bo sung kha nang chap nhan flattened source fields (`source_kind/source_rule/...`) cho tuong thich agent payload.
- Deferred check: loi `core/pkg/epss/lookup_many_test.go` (`undefined: http`) duoc dua vao danh sach kiem tra sau theo yeu cau.
- Deferred check (RESOLVED): correlator RECON_BURST duplicate-count edge case da duoc fix (COUNT DISTINCT fact_id).

## Purpose
Chuyển target architecture/runtime models trong các tài liệu:
- `FORTUNA_RUNTIME_EVENT_MODEL.md`
- `FORTUNA_RUNTIME_SIGNAL_MODEL.md`
- `FORTUNA_CAPABILITY_MODEL.md`
- `FORTUNA_RUNTIME_RISK_ENGINE_ARCHITECTURE.md`

thành một kế hoạch triển khai cụ thể cho Fortuna hiện tại.

Tài liệu này chia theo 6 mảng đúng yêu cầu:
1. DB changes
2. protobuf/API changes
3. core services
4. agent changes
5. UI changes
6. E2E test matrix

## Current Reality (đối chiếu nhanh với code)
Fortuna hiện có:
- `runtime_events` (gồm pod_uid/namespace/runtime/event_type/signal/mitre_technique/severity/syscall/target_path/capability)
- `runtime_signals` (dedupe theo ngày + `count`)
- Capability promotion qua `pod_capabilities` (state: detected/confirmed/exploited/chained)
- Risk engine enrich Pod bằng `object.fortuna` (các boolean tổng hợp như `has_network_queue_anomaly`, `has_escape_related`)

Thiếu so với target:
- `runtime_behavior_facts` (Layer 2)
- `runtime_incidents` (Layer 2/3 correlation)
- `asset_security_state` (Layer 4 backbone)
- `declared / observed / effective` capability model (Layer 3)
- REP v2 thực sự tách: runtime_events → facts → signals → incidents (hiện tại gộp nhiều vào `classifyEventToSignal` + heuristics)

---

## Milestones (ưu tiên theo kiến trúc)

### P0 (Lay foundation, no breakage)
P0.1 Khóa canonical RuntimeEventDTO (version contract)
- Mục tiêu: runtime ingestion trở thành deterministic, replay-safe.
- Chấp nhận: có versioned contract + contract metrics (accepted/rejected/partial/unresolved).

P0.2 Tách 4 tầng runtime data
- `runtime_events` (raw evidence, immutable/append-only)
- `runtime_behavior_facts` (deterministic, source-independent facts)
- `runtime_signals` (semantic layer, taxonomy formalized, evidence refs)
- `runtime_incidents` (stateful correlated runtime conditions)

P0.3 Runtime-first capability init (không phụ thuộc “pod_capabilities row đã tồn tại”)
- Nếu runtime signal mapping ra capability, CSC/Capability Engine phải có cơ chế initialize on-demand.

P0.4 Formalize runtime taxonomy (signal registry + metadata)
- Mỗi `signal_type` có domain, severity_hint, mitre tactic/technique, promotable capabilities, default decay window.

P0.5 Tạo `asset_security_state` (minimal) và đổi input của Risk Engine sang layer này
- UI/Rules/risk scoring consume `asset_security_state` thay vì boolean tổng hợp trong `object.fortuna`.

P0.5b Compatibility bridge
- Giữ `object.fortuna.*` để không gãy UI/rules cũ, nhưng derive từ `asset_security_state` thay vì raw joins heuristic.

### P1 (Add intelligence)
*(Thu tu trien khai thuc te: **muc 9.0** — semantic lock + explainability + detector governance **truoc** scorer v3 cut-over.)*

P1.1 REP v2 stateful detectors tối thiểu
- exec chain / credential access chain / recon burst / exfil-like

P1.2 Incident lifecycle + suppression model
- P0/P0.5/REP-C: đã có suppression/cooldown tối thiểu (30m) để tránh incident spam theo detector window/bucket.
- Còn lại: tuning fine-grained + lifecycle nâng cao (cooldown theo evidence quality/incident confidence decay).

P1.3 Explainability builder v2
- evidence refs: event → fact → signal → incident → capability → insight

P1.4 Scorer v3 factorized + freshness/confidence/decay

### P2 (platform value)
P2.1 Runtime coverage model
- source coverage → signal coverage → MITRE tactic coverage → detector coverage → scenario coverage

P2.2 Attack-path substrate (relational graph đủ dùng trước)

P2.3 UI Coverage View + Runtime Timeline

---

## 1) DB changes

### 1.1 Introduce new tables (Layer 2: facts + incidents)
#### a) `runtime_behavior_facts`
Mục tiêu:
- tách raw event ra “hành vi chuẩn hóa” (deterministic)

Chìa khóa join:
- `pod_uid` (primary asset join key khi resolvable)
- `event_id` (link back để explainability)

#### b) `runtime_incidents`
Mục tiêu:
- stateful correlation output (higher-order hơn signals)
- chứa evidence_refs đến signals/facts

Index gợi ý:
- `(pod_uid, incident_type, last_seen_at DESC)`

### 1.2 Evolve existing tables (fit target schema)
#### a) `runtime_events`
Bổ sung/chuẩn hóa:
- `event_id` (uuid)
- `observed_at` (RFC3339 UTC)
- `ingested_at`
- `resolution_state` (resolved|partial|unresolved)
- `source.kind`, `source.sensor_id`, `source.raw_rule/raw_category/raw_severity`
- `payload_json` (hoặc raw ref) + `payload_hash` (sha256)

Giữ lại tương thích:
- alias fields map từ current payload (syscall/target_path/runtime/source) sang canonical fields.

#### b) `runtime_signals`
Bổ sung:
- lifecycle: `first_seen_at`, `last_seen_at`
- evidence refs (mảng hoặc JSONB): `evidence_refs: [event_id, fact_id]`
- taxonomy metadata fields (hoặc join sang registry table)

Giảm coupling:
- Không dedupe theo ngày “thô” nữa; chuyển sang rolling semantic state theo `last_seen_at`.

### 1.3 Replace/extend capability model
Hiện tại:
- `pod_capabilities` chỉ là “offensive capability states” (detected/confirmed/…)

Target:
- declared_capabilities / observed_capabilities / effective_capabilities
- mỗi capability có `capability_class`, `state`, `confidence`, `derived_from`…

Khuyến nghị triển khai:
- P0: mở rộng `pod_capabilities` để thêm `capability_class` và `derived_from` (tránh làm quá nhiều migration trong 1 nhịp).
- P1/P2: tách dần sang bảng mới (nếu cần) hoặc giữ single-table với lớp semantics.

### 1.4 Add `asset_security_state`
Mục tiêu:
- backbone dùng bởi Risk Engine

Schema tối thiểu:
- asset identity (pod_uid + metadata)
- exposure + privilege + software risk (SBOM/vuln)
- runtime recent signals + incidents
- effective capabilities (list)
- blast radius (JSON)
- freshness last update timestamp

### 1.5 Insigths & Risk score schema upgrades
Hiện tại:
- `insights` có Evidence/ViolatedRules dạng jsonb string
- `risk_scores` có factors jsonb cho V2

Target:
- factorized scoring + explainable refs (why risky đến từ incident/signal/capability)
- risk score có `scorer_version` và factorized dimensions (exposure/exploitability/privilege/confidence/decay)

---

## 2) protobuf/API changes

### 2.1 Versioned runtime ingest contract
Hiện tại:
- `POST /api/v1/runtime/events` nhận `runtimeEventPayload` (syscall/target_path/pod_uid/runtime/event_type/signal/mitre/severity…)

Target:
- `RuntimeEventDTO` canonical theo `FORTUNA_RUNTIME_EVENT_MODEL.md`
- include: event_id, observed_at, ingested_at, source + resolution_state, nested process/file/network/security sections

Khuyến nghị:
- Giữ endpoint cũ để tương thích tạm thời:
  - `POST /api/v1/runtime/events` → map sang canonical và lưu tiếp
- Thêm v2:
  - `POST /api/v2/runtime/events` hoặc `/api/v1/runtime/events?version=2`

Chấp nhận:
- contract metrics: accepted/rejected/partial/unresolved và adapter parse failures.

### 2.2 New APIs for layered views
Dashboard cần endpoint để hiển thị rõ layering:
- `GET /api/v2/risk/pods/:uid/runtime/facts`
- `GET /api/v2/risk/pods/:uid/runtime/incidents`
- `GET /api/v2/risk/pods/:uid/capabilities?class=declared|observed|effective`
- `GET /api/v2/risk/pods/:uid/security-state` (asset_security_state)

### 2.3 Explainability refs
Insight endpoints cần:
- evidence_refs structured để UI render “why” chi tiết.

---

## 3) Core services (server-side modules)

### 3.1 Behavior Fact Extractor (REP-A)
Input:
- runtime_events (canonical DTO)
Output:
- runtime_behavior_facts

Thiết kế:
- Fact extraction deterministic và idempotent: event_id → facts (n định).
- Fact types theo taxonomy domains (Execution/Filesystem/Network/Privilege/Creds/Evasion/Recon/Persistence…).

### 3.2 Signal Synthesizer (REP-B)
Input:
- runtime_behavior_facts
Output:
- runtime_signals (semantic layer)

Stateless detectors:
- 1 fact → 1 signal (hoặc signal set).

### 3.3 Stateful Correlators (REP-C)
Input:
- facts/signals theo window + group_by
Output:
- runtime_incidents

State:
- có thể dùng DB materialized view/rolling last_seen + query windows
- idempotent output incident per `(pod_uid, incident_type, window bucket)`

### 3.4 Capability Engine (Layer 3)
Input:
- declared/static inventory (RBAC/exposure/host)
- observed runtime_signals/incidents
Output:
- declared / observed / effective capabilities

Chìa khóa:
- runtime-first init + decay/suppression
- promotable capabilities registry.

### 3.5 Security State Projector (Layer 4)
Input:
- capabilities + runtime signals/incidents + exposure/privacy/software risk
Output:
- asset_security_state

### 3.6 Risk Engine v2
Input:
- asset_security_state
Output:
- insights + risk_scores

Giải thích:
- Risk rules CEL consume `asset_security_state.runtime.recent_signals/incidents` và `capabilities.effective`.
- Toxic combinations cần support bằng correlation input/incident list.

---

## 4) Agent changes

### 4.1 Adapter phải cung cấp canonical DTO
Falco/eBPF adapters:
- tạo event_id
- set observed_at và runtime/source metadata
- attach resolution state (resolved/partial/unresolved)
- payload_hash để audit/replay

### 4.2 Ingest reliability
- retry/batching/backpressure giữ nguyên concept
- nhưng core contract metrics để phân tích ingestion quality.

### 4.3 Contract determinism
Agent không nên “tự suy nghĩa” về risk/capability sâu.
- Agent chỉ normalize nhẹ và attach context.
- Core làm correlation/detection/capability reasoning.

---

## 5) UI changes

### 5.1 Pod Detail layering
- Tab hoặc sections tách:
  - Runtime Events (raw)
  - Behavior Facts (Layer 2)
  - Runtime Signals (Layer 2)
  - Runtime Incidents (Layer 2)
  - Capabilities (declared/observed/effective)
  - Insights (final conclusions)

### 5.2 Coverage View
- MITRE tactic coverage (Execution/Creds/Network…)
- signal coverage theo domain
- source coverage theo sensor type (falco/ebpf/...)

### 5.3 Runtime Timeline
- incidents timeline artifacts
- show which facts/signals contributed.

---

## 6) E2E test matrix

### 6.1 Test strategy
Mỗi detector trong `FORTUNA_RUNTIME_DETECTOR_CATALOG.md` phải có:
- (a) testcase tạo hành vi trên một pod/workload
- (b) verify runtime_events accepted
- (c) verify runtime_behavior_facts produced
- (d) verify runtime_signals / runtime_incidents
- (e) verify capability effective/promotion
- (f) verify insight created and visible on UI.

### 6.2 Minimal matrix (P0/P1)
- shell exec → signal: `SUSPICIOUS_SHELL_EXEC` (hoặc domain Execution) + expected capability init
- serviceaccount token read → fact `SERVICEACCOUNT_TOKEN_READ` + signal `CREDENTIAL_ACCESS` hoặc tương đương
- external egress → facts network connect + signal `EXTERNAL_EGRESS`
- payload fetch → fact `REMOTE_PAYLOAD_FETCH` + incident/execution chain
- tmp exec → signal/incident “tmp binary execution”
- host path access → signal `HOST_PATH_ACCESS` + escape/host privilege related capability
- recon burst → incident `RECON_BURST` (stateful)
- exfil-like → incident `EXFIL_LIKE_SEQUENCE` (stateful)

### 6.3 Các chỉ số pass/fail
- signalType phải match đúng taxonomy registry (không allowed UNKNOWN ngoại trừ test negative cases)
- evidence_refs phải có đường link event_id → fact_id → signal/incident
- UI render: PodDetail shows evidence.syscall/target + incident timeline.

---

## 7) P0 detector pack + fact mapping matrix

Mục tiêu: xuyên end-to-end toàn pipeline mới nhưng giữ scope vừa đủ để ship.

| Detector (P0) | Required facts | Output | Stateful | Capability impact | UI surface |
|---|---|---|---|---|---|
| INTERACTIVE_SHELL_EXEC | PROCESS_EXEC, INTERACTIVE_SHELL | Signal: `INTERACTIVE_SHELL_EXEC` | No | `OBSERVED_PROCESS_EXECUTION` | Pod Detail > Runtime Signals |
| SERVICEACCOUNT_TOKEN_READ | FILE_READ, SERVICEACCOUNT_TOKEN_READ | Signal: `SERVICEACCOUNT_TOKEN_READ` | No | `OBSERVED_K8S_API_ACCESS` (candidate) | Pod Detail > Runtime Signals/Events |
| EXTERNAL_EGRESS | NETWORK_CONNECT, EXTERNAL_CONNECT | Signal: `EXTERNAL_EGRESS` | No | `OBSERVED_EXTERNAL_EGRESS` | Pod Detail + Coverage |
| REMOTE_PAYLOAD_FETCH | NETWORK_CONNECT + downloader exec hint | Signal: `REMOTE_PAYLOAD_FETCH` | No | `OBSERVED_REMOTE_STAGING` | Pod Detail + Timeline |
| TMP_BINARY_EXECUTION | PROCESS_EXEC, TMP_BINARY_EXEC | Signal: `TMP_BINARY_EXECUTION` | No | `OBSERVED_POST_EXPLOIT_EXEC` | Pod Detail + Capability panel |
| HOST_PATH_ACCESS | FILE_READ/WRITE + HOST_PATH_TOUCH | Signal: `HOST_PATH_ACCESS` | No | `OBSERVED_HOST_FILE_TOUCH` | Pod Detail + Risk context |
| POST_EXPLOIT_EXEC_CHAIN | INTERACTIVE_SHELL_EXEC + REMOTE_PAYLOAD_FETCH + TMP_BINARY_EXECUTION | Incident: `POST_EXPLOIT_EXEC_CHAIN` | Yes | `EFFECTIVE_EXTERNAL_COMMAND_AND_CONTROL` (candidate) | Runtime Timeline + Insights |
| RECON_BURST | NETWORK_RECON + HIGH_FANOUT_CONNECT/DNS bursts | Incident: `RECON_BURST` | Yes | `EFFECTIVE_LATERAL_MOVEMENT_PRIMITIVE` (candidate) | Coverage + Insights |

Ghi chú:
- `EXFIL_LIKE_ACTIVITY` đưa vào P1.2 khi network/file semantics đủ chắc.
- P0 cần dual-path compare (old adapter vs new facts-based path) để không gãy runtime signals hiện có.

---

## 8) Implementation checklist theo repo

Checklist này là bản thi công theo module thực tế trong repo.

### 8.1 Migrations (`core/migrations`)
- [x] Add migration: `runtime_events` canonical columns (event_id, observed_at, ingested_at, source_*, resolution_state, payload_hash/payload_json) (P0.1 minimal)
- [x] Add table `runtime_behavior_facts`
- [x] Add table `runtime_incidents`
- [x] Add table `asset_security_state`
- [x] Add capability class columns / tables (`declared/observed/effective`) theo chiến lược compatibility
- [x] Add indexes cho timeline + joins + domain filters (muc toi thieu cho facts/incidents/asset_security_state)

Chi tiet da thuc hien:
- Bo sung migration `109_add_runtime_signal_lifecycle.go`: them `runtime_signals.first_seen_at`, `last_seen_at`, `evidence_refs` + index + backfill tu `created_at`.
- Bo sung migration `110_add_pod_capability_class_derived_from.go`: them `pod_capabilities.capability_class`, `derived_from` + index class + default `effective`.
- Dang o muc compatibility rollout (`[~]`): chua tach bang rieng declared/observed/effective, hien dang mo rong single-table.

### 8.2 Domain models (`core/pkg/models`)
- [x] Add models: `RuntimeBehaviorFact`, `RuntimeIncident`, `AssetSecurityState`
- [x] Evolve `RuntimeEvent` model toward canonical fields (backward compatible) (added canonical contract columns + v2 ingest)
- [x] Evolve `RuntimeSignal` lifecycle fields (`first_seen_at`, `last_seen_at`, `evidence_refs`)
- [x] Capability model extension (`capability_class`, `derived_from`, state lifecycle)

Chi tiet da thuc hien:
- `models.RuntimeSignal`: them `EvidenceRefs`, `FirstSeenAt`, `LastSeenAt` (de phuc vu lifecycle + explainability refs).
- `models.PodCapability`: them `CapabilityClass`, `DerivedFrom` theo huong compatibility truoc khi split model day du.

### 8.3 Core services (`core/pkg/rep`, `core/pkg/capability`, `core/pkg/riskengine`)
- [x] Implement REP-A Fact extractor: `runtime_events -> runtime_behavior_facts`
- [x] Implement REP-B Signal synthesizer: `facts -> runtime_signals` (persist path minimal, co chong double-count)
- [x] Implement REP-C Stateful correlators: `facts/signals -> runtime_incidents` (done minimal set `RECON_BURST`, `POST_EXPLOIT_EXEC_CHAIN`, `EXFIL_LIKE_SEQUENCE` + cooldown/suppression)
- [x] Implement runtime-first capability init + promotion (P0 minimal)
- [x] Implement `asset_security_state` projector (P0 minimal)
- [x] Refactor RiskEngine to consume `asset_security_state` as primary input (partial: `object.fortuna.*` da doi sang state; scoring v2 chua cut-over)
- [x] Keep compatibility projection for `object.fortuna.*` derived from `asset_security_state`

Chi tiet da thuc hien:
- REP-A: persisted `runtime_behavior_facts` va tra facts cho cac phase sau.
- REP-B: them persist path tu facts -> runtime_signals, co co che khong tang `count` de tranh double-count voi legacy path.
- REP-C: bo sung correlators stateful + cooldown/suppression, co test positive/negative/suppressed cases.
- RiskEngine v2 scaffold: bo sung `object.securityState.*` tu `asset_security_state`, migrate runtime rules theo state object.

### 8.4 API/handlers (`core/internal/api`)
- [x] Add v2 runtime ingest contract endpoint (or versioned mode) (`POST /api/v2/runtime/events`, P0.1 minimal)
- [x] Add APIs for facts/incidents/capabilities/security-state (facts/incidents/security-state + **capabilities v2** `GET /api/v2/runtime/pods/:uid/capabilities`)
- [x] Keep v1 APIs stable for dashboard compatibility
- [x] Add explainability refs in insight responses

Chi tiet da thuc hien:
- Bo sung truong `evidence_refs` co cau truc trong response cua `GET /risk/insights` va `GET /risk/insights/:id`.
- Parse refs tu `insight.evidence` + `insight.violated_rules` (event/fact/signal/incident/capability/rule) theo best-effort va dedupe.
- Bo sung test API/handler cho explainability refs.
- Mo rong filter `class` cho endpoint capabilities (`GET /inventory/pods/:uid/capabilities` va list) de support capability_class rollout.

### 8.5 Agent changes (`agent/internal/runtime`, `agent/internal/config`, `agent/cmd`)
- [x] Emit canonical DTO fields (event_id/observed_at/source/resolution/payload_hash) cho runtime sensors (FalcoReader + RuntimeEvents Reader) theo shape v2; có fallback v1.
- [x] Keep old payload fallback while Core supports dual-path (POST v2, fallback v1 on 404).
- [x] Add ingestion quality metrics logs/counters (Runtime Reader + Falco Reader: sent/failed batches-events, invalid lines, v2 success, v1 fallback)
- [x] Ensure partial/unresolved resolution events are preserved (normalize + preserve from Runtime Reader input; FalcoReader read from output_fields/tags, default unresolved).
- [x] Phase A ops (2026-03-28): FalcoReader **tail-from-EOF** on first read; **pod UID** resolve via **List** fallback; manifest `deploy/fortuna-agent-daemonset.yaml` + README build **`nerdctl -n k8s.io`** (xem 9.6).

Chi tiet da thuc hien:
- Runtime Reader: normalize `resolution_state` (`resolved|partial|unresolved`) va khong overwrite gia tri hop le tu nguon event.
- Falco Reader: map `resolution_state` tu `fortuna.resolution_state`/`resolution_state` trong `output_fields` hoac tags `resolution_state=...`.
- Bo sung ingestion quality logs `[IngestQuality]` cho batch success/failure va fallback quality.

### 8.6 UI changes (`dashboard/pages`, `dashboard/lib/api`, `dashboard/types`)
- [x] PodDetail: separate sections for Events/Facts/Signals/Incidents/Capabilities/Insights
- [x] Add Runtime Timeline view for incidents
- [x] Add Coverage view (signal/domain/source/mitre)
- [x] Keep old tabs usable during migration
- [x] PodDetail UX cleanup + data-empty diagnostics (Timeline/Coverage hien thi du shell nhung thieu data thuc te)

Chi tiet da thuc hien:
- PodDetail tab Events duoc tach section ro rang: raw runtime events, runtime signals, runtime facts, runtime incidents, capabilities, insights.
- Runtime incidents section bo sung timeline view toi thieu (sort theo `lastSeenAt`, hien first->last, window/confidence).
- Dashboard client bo sung APIs v2 cho facts/incidents (`/api/v2/runtime/pods/:uid/facts|incidents`) va type mapping tuong ung.
- Bo sung coverage snapshot card ngay trong PodDetail Events (source, layer, MITRE distinct count) de user co goi y coverage nhanh khi dieu tra runtime.
- Mo rong coverage cards theo fact domain + signal type de dat muc signal/domain/source/mitre ngay tren PodDetail.
- Bo sung `Legacy quick view` (runtime signals + k8s events) de giu troubleshooting flow cu trong giai doan migrate.
- Tach view thanh tab rieng trong PodDetail: `Runtime Timeline` va `Coverage`, giu tab `Events` cho raw/legacy troubleshooting.
- Giu migration-safe: neu v2 endpoint chua san sang thi fallback hien thi danh sach rong, khong vo UI.
- GAP moi can xu ly: UI PodDetail hien tai bi roi (nhieu card/trung lap), can gom nhom thong tin theo use-case dieu tra va bo sung hint "vi sao khong co data" cho Timeline/Coverage (agent ingest, v2 facts/incidents, lookback, pod uid mapping).
- Da don gon tab Events (bo cac khoi trung lap facts/incidents/capabilities/insights), giu Events tap trung cho raw runtime + signals + k8s events.
- Bo sung diagnostics cho tab Timeline/Coverage de giai thich ly do "khong co data" theo tung layer.

### 8.7 Rules/CEL (`core/rules`, `core/pkg/riskengine`)
- [x] Introduce state-based CEL inputs from `asset_security_state` (added `object.securityState.*` projection; runtime+cluster-admin rules migrated with fallback)
- [x] Add toxic combination rule set (runtime + static combo) (added `toxic-combo-clusteradmin-hostnetwork-runtime`)
- [x] Keep legacy rules running via compatibility projection until parity

Chi tiet da thuc hien:
- Chuyen load rule ve YAML-only source (`/core/rules`, co the override qua `FORTUNA_RULES_DIR`), bo hardcoded fallback path.
- Migrate cac runtime rules tu `object.fortuna.*` sang uu tien `object.securityState.*` (van co fallback fortuna de rollout an toan).
- Bo sung toxic combo rule: `cluster-admin + hostNetwork + runtime activity`.
- Bo sung unit test guardrail `yaml_rules_compat_projection_test.go` de dam bao rule nao dung `object.securityState` deu giu fallback `object.fortuna` trong giai doan parity.
- Mo rong toxic combo voi 2 rule moi: `toxic-combo-clusteradmin-escape-runtime`, `toxic-combo-hostns-escape-runtime` + tests expression match.

### 8.8 E2E tests (`scripts/e2e`, `core/internal/api/*_test.go`, `core/pkg/*_test.go`)
- [x] Test canonical ingest (contract + quality metrics)
- [x] Test event->fact->signal chain for 6 stateless detectors
- [x] Test incident synthesis for 2 stateful incidents
- [x] Test capability runtime-first init/promotion
- [x] Test asset_security_state projection correctness (basic unit test for `object.securityState.*` shape)
- [x] Test risk rule evaluation via state object (CEL expression match; rule scoring v2 end-to-end chưa cut-over)
- [x] Test UI surfaces (Pod detail + coverage + timeline)

Chi tiet da thuc hien:
- Canonical ingest v2: test persist canonical fields (event_id/observed_at/ingested_at/source*/payload_hash), flattened source compatibility, va `resolution_state=partial`.
- Agent quality metrics: test Runtime Reader v2-success + v1-fallback counters (`v2_success`, `v1_fallback`) va canonical enrichment in send path.
- Stateful incidents: bo sung flow test sinh dong thoi `RECON_BURST` + `POST_EXPLOIT_EXEC_CHAIN` qua ingest v1 -> REP -> v2 incidents API.
- Stateless chain: bo sung test end-to-end event -> fact -> signal cho 6 detector (`INTERACTIVE_SHELL_EXEC`, `SERVICEACCOUNT_TOKEN_READ`, `EXTERNAL_EGRESS`, `REMOTE_PAYLOAD_FETCH`, `TMP_BINARY_EXECUTION`, `HOST_PATH_ACCESS`).
- Capability runtime-first: bo sung test initialize + promote qua CSC (seed `promotion_rules`) de xac nhan capability row duoc tao va len state cao hon khi co runtime signal.
- Bo sung unit tests cho `object.securityState.*` projection (bao gom maps/arrays fields tu `asset_security_state`).
- Bo sung test CEL rules theo state object cho runtime + cluster-admin + toxic-combo, giu backward-compat fallback behavior.
- Bo sung `pod_detail_runtime_surfaces_sync_test.go` de verify data cho 4 runtime UI surfaces: events, signals, facts, incidents.

---

## 9) Tong hop GAP da nhan dien + ke hoach hoan thien

Muc nay gom cac khoang trong **con lai** sau khi checklist 8.x da danh dau [x] cho nen tang P0/P0.5, va dong bo voi `docs/03-components/podDetail/GAP_STATUS_RUNTIME.md`, `Runtime_Security_Rule_category.md`, va milestone P1/P2 o tren.

### 9.0 Dieu chinh uu tien — Semantic lock & backlog reorder (2026-03-28)

**Van de:** Nen tang **data pipeline** (events → facts → signals → incidents → state) da dung huong; rui ro lon nhat khong con la “thieu bang/API” ma la **drift ngu nghia (semantics)** giua cac lop (signal vs incident vs capability vs `asset_security_state`) va **scoring/explainability** dung tren semantics chua khoa.

**Nguyen tac (gate):**
- **Khong** coi **G-RE-01 (scorer v3 cut-over)** la “san sang” neu chua co **G-SEM-01** (capability reasoning) va **G-SEM-02** (signal/incident lifecycle) o muc ADR + test/contract toi thieu.
- **G-EXP-01 (explainability builder v2)** uu tien **ngang hoac truoc** vong tinh chinh weight cua scorer v3 (user chap nhan diem tho hon; kho chap nhan “nguy hiem” khong co chuoi nhan qua ro).
- **G-P2-COV / G-P2-GRAPH** chi “dau tu nang luc” sau khi: capability class + incident confidence + projection state on dinh trong 1–2 sprint thuc thi (tranh coverage/graph dep tren semantics mo).

**Thu tu uu tien da dieu chinh (tom tat):**

| Thu tu | Nhom | Noi dung | GAP chinh |
|--------|------|----------|-----------|
| **1** | Semantic lock | Capability ontology + rulebook `declared` / `observed` / `effective`; tach ro **progression state** vs **capability class**; `derived_from` la contract, khong chi JSON tu do | G-SEM-01 |
| **1b** | Semantic lock | Dinh nghia **signal = assertion** (metadata + lifecycle); khi nao emit moi / update / escalate **incident**; tranh tu duy “signal = hang dem/dedupe” | G-SEM-02 |
| **1c** | Schema discipline | `asset_security_state` chi mo rong theo **5 nhom** co dinh (identity / exposure / software_risk / runtime_security / effective_capability); field moi = ADR hoac reject | G-SEM-03 |
| **2** | Detector governance | Metadata contract moi detector/correlator; confidence (fact optional, signal quan trong, incident bat buoc); replay/idempotency/out-of-order/negative/suppression tests | G-REP-GOV-01 (+ G-REP-01) |
| **3** | Explainability | Builder v2: chuoi event → fact → signal → incident → capability → rule → insight (khong chi `evidence_refs` roi) | G-EXP-01 |
| **4** | Scoring | Scorer v3 **theo dimension + provenance**, roi moi weight tuning | G-RE-01 |
| **5** | Sensor | Fidelity exec/connect, PID→pod, source health (R9, R5, R6 lien quan) | G-R9, G-R6, G-R5 |
| **6** | Platform | Coverage **formal** (khong chi snapshot UI), attack-path substrate **sau** khi gate tren on | G-P2-COV, G-P2-GRAPH |

**Deliverable “khoa he” toi thieu:** 2–3 ADR ngan trong `docs/` (hoac mo rong `FORTUNA_*_MODEL.md`) + bang audit **detector × nguon** (Falco / eBPF / mixed) + checklist review PR (“field moi vao state thuoc nhom nao?”).

### 9.1 Phan loai nhanh

| Nhom | Trang thai tong quan | Ghi chu |
|------|----------------------|---------|
| P0 checklist (muc 8) | Da dong | Van **partial**: scoring v2 full cut-over; **capability single-table** + class — can **G-SEM-01** truoc khi mo rong scoring/graph |
| **Semantic lock (9.0)** | **ADR + test co ban (2026-03-28)** | `docs/adr/001-003`, validators + state JSON check; **gate** G-RE-01 cut-over van hieu luc |
| P1 (intelligence) | **Mo** | Thu tu moi: **governance + explainability** truoc **scorer v3**; incident lifecycle tinh vi |
| P2 (platform) | **Mo** | Coverage formal, attack-path — **sau** semantic lock + explainability v2 co nen |
| Pod Detail runtime (R*) | **Mo mot phan** | R5, R6, R9 eBPF that, R10 tuning — xem 9.2 |
| Van hanh / ingest | **Phase A: manifest + doc da dong bo** | Code tail EOF + LIST UID; DaemonSet 6Gi + SBOM_WORKERS; build `k8s.io` documented |

### 9.2 Bang GAP chi tiet (nguon + muc tieu)

| ID | Nguon | Mo ta GAP | Muc tieu hoan thien | Uu tien |
|----|-------|-----------|---------------------|---------|
| G-SEM-01 | 9.0 + capability | Capability reasoning (class vs progression) | [x] ADR-001, `capability/semantics.go`, `FORTUNA_CAPABILITY_MODEL.md` | **P1** |
| G-SEM-02 | 9.0 + signal | Signal vs incident lifecycle | [x] ADR-002, `FORTUNA_RUNTIME_SIGNAL_MODEL.md` (mermaid + headings) | **P1** |
| G-SEM-03 | 9.0 + state | `asset_security_state` five groups | [x] ADR-003, `models/asset_security_state_discipline.go`, projector validate | **P1** |
| G-REP-GOV-01 | 9.0 + REP | Detector registry + replay tests | [x] ADR-004, `rep/detector_registry.go`, duplicate + out-of-order tests | **P1** |
| G-API-01 | 8.4 | Route formal `GET /api/v2/runtime/pods/:uid/capabilities?class=…` (dong bo voi facts/incidents); v1 inventory giu lam alias | OpenAPI/doc nguoi dung (tuy chon) | P2 |
| G-DB-01 | 8.1 | Single-table decision | [x] ADR-005: giu `pod_capabilities` + class qua P1 | P1 |
| G-RE-01 | 8.3 + 8.7 | Scorer v3 | [~] `factors.score_v3_preview` (Pod) + types; **chua** thay TotalScore/worker cut-over | P1 |
| G-REP-01 | P1.2 + catalog | Confidence khi suppress incident | [x] Merge/reinforce/decay theo so evidence_refs; cooldown van static trong registry | P1 |
| G-EXP-01 | P1.3 | Explainability | [~] `explanation_chain` + `GET .../insights/:id?enrich=1` (facts); con events/signals join + UI | P1 |
| G-P2-COV | P2.1 + 8.6 | Coverage UI la **snapshot** theo dem; chua “coverage model” luu tru / trend / scenario | Bang hoac metric time-series + API coverage theo pod/cluster | P2 |
| G-P2-GRAPH | P2.2 | Chua co attack-path substrate (quan he asset → signal → incident) cho graph | Schema edge + query toi thieu cho “blast path” | P2 |
| G-UI-01 | 8.6 note | Pod Detail van **day thong tin** cho nguoi moi; can information architecture (dieu tra vs van hanh) | Wireframe: mot luong “triage” + collapse advanced | P1 |
| G-R5 | GAP_STATUS R5 | Network metrics: queue snapshot khong thay **byte/flow that** theo connection | ebpf/conntrack hoac kube proxy stats (pham vi ro) | P2 |
| G-R6 | GAP_STATUS R6 | Multi-signal chain, calibration, correlation process/network | Rule chain trong CSC + risk worker; test voi `risk-center-pod-matrix` | P1 |
| G-R9 | GAP_STATUS R9 | eBPF sensor: tracepoint noop; chua **event that** + map PID→pod | Implement exec/connect hooks + pod resolution (host proc + cgroup) | P1 |
| G-R10 | GAP_STATUS R10 | Admission gate | [x] Runbook `R10-ADMISSION-RUNBOOK.md` + CI script; Helm tuy moi truong | P1 |
| G-OPS-01 | Thuc te trien khai | Image agent: build/tag **`docker.io/library/fortuna-agent:latest`** trong namespace **k8s.io** | [x] `agent/README.md` Build, `deploy/README.md` checklist Agent, comment `push-images-to-workers.sh` | P0.5 ops |
| G-OPS-02 | Thuc te trien khai | Falco JSONL lon: **tail EOF**, **memory** agent, rotate host log | [x] Code `falco_reader.go`; manifest limits **6Gi** + `SBOM_WORKERS=1` + comments; README troubleshooting | P0.5 ops |
| G-E2E-01 | 6.x | Matrix P0 catalog: chua **automate het** tren cluster that cho tung dong detector catalog | Mo rong `e2e-risk-center-full.sh` + pod trigger theo detector | P1 |

### 9.3 Ke hoach theo phase (thu tu thuc hien — dong bo 9.0)

**Phase S — Semantic lock (1–2 sprint; bat buoc truoc G-RE-01 cut-over)**  
1. [x] **G-SEM-01:** ADR `docs/adr/001-capability-reasoning-semantics.md` + `core/pkg/capability/semantics.go` + link trong `FORTUNA_CAPABILITY_MODEL.md`.  
2. [x] **G-SEM-02:** ADR `docs/adr/002-runtime-signal-incident-lifecycle.md` + sua `FORTUNA_RUNTIME_SIGNAL_MODEL.md` (dong mermaid + heading).  
3. [x] **G-SEM-03:** ADR `docs/adr/003-asset-security-state-schema-groups.md` + `models/asset_security_state_discipline.go` + `ValidateAssetSecurityStateJSON` goi tu `asset_security_state_projector.go`.  
4. [x] **G-REP-GOV-01:** ADR `docs/adr/004-rep-detector-governance.md` + `rep/detector_registry.go` (3 correlator) + test duplicate replay / out-of-order / registry; correlator doc metadata `version`.

**Phase A — Van hanh (song song S; ngan)**  
1. [x] G-OPS-01, G-OPS-02 (2026-03-28).  
2. [x] G-API-01 route v2 capabilities + CI verify admission (baseline).  
3. [x] G-R10 (baseline): `R10-ADMISSION-RUNBOOK.md` + [x] CI `verify-admission-risk-gate.sh`; Helm tach rieng tuy moi truong.

**Phase B — Governance + sensor (2–3 sprint)**  
1. [ ] Hoan thien **G-REP-GOV-01** cho bo 3 correlator hien co + signal synthesizer path (confidence tren signal/incident).  
2. [ ] **G-REP-01:** cooldown/decay theo confidence (sau khi co model).  
3. [ ] **G-R9:** eBPF exec/connect that + PID→pod (song song Falco).  
4. [ ] **G-R6:** multi-signal chain + calibration; **G-E2E-01** mo rong matrix.

**Phase C — Explainability roi moi scoring (2 sprint; thu tu cot loi)**  
1. [x] **G-EXP-01 (MVP):** `core/pkg/explainability/insight_chain.go` + field `explanation_chain` tren `GET /risk/insights` va `GET /risk/insights/:id` (thu tu pipeline); chua DB join day du.  
2. [ ] **G-RE-01:** Scorer v3 **cut-over** + worker (hien co scaffold `core/pkg/riskengine/score_v3_types.go`).  
3. [x] **G-DB-01:** ADR `docs/adr/005-pod-capability-single-table.md` — giu single-table Phase P1.

**Phase D — Platform / UX (sau gate C)**  
1. [ ] **G-P2-COV:** Coverage model formal (khong chi snapshot UI); phan biet source vs telemetry vs detector vs scenario.  
2. [ ] **G-P2-GRAPH:** Attack-path substrate chi khi capability + state + explainability on dinh.  
3. [ ] **G-UI-01:** IA Pod Detail (triage vs van hanh).  
4. [ ] **G-R5:** byte/flow khi co nguon on dinh.

### 9.3.1 Phan tich GAP (tom tat theo truc)

| Truc | Diem manh hien tai | GAP chinh | Hanh dong trong backlog |
|------|-------------------|-----------|-------------------------|
| Data / architecture | 4 tang + `asset_security_state`; ingest v2 | Signal con dau vet legacy; state de thanh “tu do” | G-SEM-02, G-SEM-03, G-REP-GOV-01 |
| Detection / REP | REP-A/B/C + 3 correlator | Thieu contract + confidence + replay test day du | G-REP-GOV-01, G-REP-01 |
| Capability | Runtime-first + `capability_class` | Ontology chua “cung”; state vs class de lon | G-SEM-01, G-DB-01 |
| Risk / scoring | State-based CEL + toxic combo | Scorer v3 chua cut-over; khong nen tac dong weight truoc semantics | Gate 9.0 → G-RE-01 sau G-SEM-* + G-EXP-01 |
| Explainability | `evidence_refs` best-effort | Chua causal chain cho user | G-EXP-01 |
| Sensor / ops | Falco tail EOF, UID list, RAM | eBPF / flow that con yeu | G-R9, G-R5 |
| Platform | UI Timeline/Coverage | Coverage snapshot; graph som = rui ro | G-P2-COV, G-P2-GRAPH sau Phase C |

### 9.4 Tieu chi “dong GAP” (DoD ngan)

- **G-SEM-01 / G-SEM-02 / G-SEM-03**: ADR merged + link tu `FORTUNA_CAPABILITY_MODEL.md` / `FORTUNA_RUNTIME_SIGNAL_MODEL.md`; checklist PR cho field `asset_security_state`.
- **G-REP-GOV-01**: contract template + it nhat 1 correlator co confidence + bo test replay/out-of-order theo spec.
- **G-API / G-DB**: co migration hoac ADR + test API + cap nhat dashboard.  
- **G-R6 / G-R9**: co E2E hoac script verify + pod matrix coverage rule id.  
- **G-RE-01**: `risk_scores` co version + snapshot test golden.  
- **G-OPS**: rollout DaemonSet khong CrashLoop voi Falco JSONL >100MB; ingest Falco co ban ghi trong `runtime_events` cho pod test trong <2 phut sau trigger.  
- **G-UI-01**: checklist UX + 1 vong user test noi bo.

### 9.5 Tai lieu lien quan (khong trung lap noi dung)

- `GAP_IMPLEMENTATION_STATUS.md` — **trang thai GAP vs code/test** (cap nhat 2026-03-28).  
- `docs/adr/README.md` — chi muc ADR 001–005.  
- `docs/03-components/podDetail/GAP_STATUS_RUNTIME.md` — R1–R10 chi tiet agent/Pod Detail.  
- `FORTUNA_RUNTIME_DETECTOR_CATALOG.md` — danh detector P0/P1.  
- `Runtime_Security_Rule_category.md` — doi chieu category rule vs Fortuna.  
- `Risk-Center-Go-Live-Criteria.md` — tieu chi go-live tong.

### 9.7 Ban ghi trien khai GAP (2026-03-28) — ma, test, tai lieu

**Nguyen tac:** Khong the “hoan tat tat ca GAP” trong mot phien (eBPF, graph DB, scorer cut-over, UI redesign la multi-sprint). Phien nay **hoan tat Phase S + G-REP-GOV + MVP G-EXP-01 + scaffold G-RE-01 + G-DB-01 + runbook R10**; cac muc con lai ghi trong `GAP_IMPLEMENTATION_STATUS.md`.

| GAP | Ma chinh | Test |
|-----|----------|------|
| G-SEM-01 | `core/pkg/capability/semantics.go` | `go test ./core/pkg/capability/...` |
| G-SEM-02 | ADR-002, sua `FORTUNA_RUNTIME_SIGNAL_MODEL.md` | (spec) |
| G-SEM-03 | `core/pkg/models/asset_security_state_discipline.go`, projector | `go test ./core/pkg/models/... -run AssetSecurity` |
| G-REP-GOV-01 | `core/pkg/rep/detector_registry.go`, `incident_correlator.go` | `go test ./core/pkg/rep/...` |
| G-EXP-01 MVP | `core/pkg/explainability/`, `insights_handlers.go` | `go test ./core/internal/api/... -run Explain` |
| G-RE-01 scaffold | `core/pkg/riskengine/score_v3_types.go` | `go test ./core/pkg/riskengine/... -run MergeDimension` |
| G-DB-01 | `docs/adr/005-pod-capability-single-table.md` | — |
| G-R10 | `R10-ADMISSION-RUNBOOK.md` | script + CI |

**Ket qua test (mau):** `ok` cho `./pkg/capability`, `./pkg/models`, `./pkg/rep`, `./pkg/explainability`, `./pkg/riskengine`, `./internal/api` voi `-count=1` tren moi truong dev (xem log CI/local).

**API:** Response insight them `explanation_chain` (mang `{layer, refs}` theo thu tu event → fact → signal → incident → capability → rule).

### 9.6 Phase A da thuc hien (2026-03-28) — chi tiet

**Code (agent) — truoc do / ghi nhan trong session:**
- `agent/internal/runtime/falco_reader.go`: lan doc dau tien voi `offset==0` va file lon thi **dat offset = fileSize** (chi doc ban ghi moi sau khi agent start); tranh `io.ReadAll` ca file JSONL hang tram MB → OOM.
- `resolvePodUID`: neu `CoreV1().Pods(ns).Get` loi, fallback `Pods.List` voi `fieldSelector=metadata.name=<podName>` de lay UID khi RBAC chi cho **list** khong cho **get**.

**Manifest `deploy/fortuna-agent-daemonset.yaml`:**
- `FALCO_EVENTS_ENABLED=true` (giu nguyen).
- Comment giai thich tail EOF + khuyen nghi rotate/truncate `events.jsonl` tren host.
- `SBOM_WORKERS=1` giam dinh song song SBOM, giam peak RAM.
- `resources.requests.memory: 1Gi`, `limits.memory: 6Gi`, `limits.cpu: "1"` (chuoi hop le YAML).

**Tai lieu / script:**
- `agent/README.md`: build `nerdctl -n k8s.io` tu repo root, tag `docker.io/library/fortuna-agent:latest`; troubleshooting Falco ingest + OOM.
- `deploy/README.md`: dong checklist Agent (6Gi, SBOM_WORKERS, build k8s.io).
- `scripts/utils/push-images-to-workers.sh`: comment build tip.

**G-API-01 — Pod Detail / Dashboard goi endpoint (uu tien v2 runtime):**

| Surface | Method | Path (prefix `/api/v1` hoac `/api/v2` tuy cau hinh Core) | Ghi chu |
|--------|--------|-----------------------------------------------------------|---------|
| Security runtime events (Falco, REP raw) | GET | `/api/v1/risk/pods/:uid/runtime/events` | JWT theo policy Core |
| Runtime signals | GET | `/api/v1/runtime/pods/:uid/signals` | |
| Behavior facts | GET | `/api/v2/runtime/pods/:uid/facts` | Dashboard `requestV2` |
| Incidents | GET | `/api/v2/runtime/pods/:uid/incidents` | |
| Security state | GET | `/api/v2/runtime/pods/:uid/security-state` | |
| Capabilities (+ filter class) | GET | `/api/v2/runtime/pods/:uid/capabilities?class=effective` (alias v1: `/api/v1/inventory/pods/:uid/capabilities`) | G-API-01: handler giong v1; Dashboard uu tien v2 |

Verify sau deploy: `kubectl apply -f deploy/fortuna-agent-daemonset.yaml` + `kubectl rollout status daemonset/fortuna-agent -n fortuna`.

