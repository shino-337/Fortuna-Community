I'll analyze the current open-source solutions for runtime security on container infrastructures like Kubernetes. Let me search for specific implementation details across these repositories.Based on my analysis of the major open-source runtime security solutions, here's a comprehensive assessment of how they handle runtime security on container and Kubernetes infrastructures:

Runtime Security Solutions Analysis
1. FALCO (Cloud Native Runtime Security)
Architecture & Approach:

Detection Method: System call monitoring via eBPF (kernel-level) and traditional syscall capture
Rules Engine: Lua-based rule language with event-driven detection
Policy System:
Pre-built rule sets (system call patterns, privilege escalation, data exfiltration)
Custom rule development via Lua expressions
Default rules covering suspicious behavior patterns
Key Rules/Policies:

Code
- Unauthorized process execution
- Privilege escalation detection
- Data exfiltration patterns
- Container escape attempts
- Suspicious library loading
- Unauthorized file modifications
Strengths:

Deep kernel-level visibility
Real-time threat detection
Lightweight footprint
Multi-dataplane support
2. NEUVECTOR (Container & Kubernetes Security Platform)
Architecture & Approach:

Detection Method: Agent-based (per-node) with network policy enforcement
Network Security: Layer 7 deep packet inspection (DPI)
Policy Model:
Network policies (ingress/egress rules)
Process behavior profiles (whitelist-based)
File access controls
System call profiling
Key Rules/Policies:

Code
- Network microsegmentation
- Process whitelisting (automatic learning mode)
- Suspicious process detection
- File access violation detection
- DLP (Data Loss Prevention)
- Vulnerability scanning integration
Architecture: Agent → Manager → Controller pattern

Per-node agents monitor and enforce policies
Central manager handles policy distribution
3. KYVERNO (Unified Policy-as-Code)
Architecture & Approach:

Policy Engine: CEL-based (Common Expression Language) evaluation
Policy Types:
ValidatingPolicy: Admission control (denies non-compliant resources)
MutatingPolicy: Auto-remediation of resources
GeneratingPolicy: Auto-provision compliance resources
ImageValidatingPolicy: Container image verification
Key Rules/Policies:

YAML
# Pod Security Standards (PSS)
- Privileged container detection
- Capability restrictions
- Read-only filesystem enforcement
- RunAsNonRoot validation
- Network policy enforcement
- Resource quota validation

# Image Verification
- Container image signature verification
- Registry whitelist enforcement
- Image scanning (Trivy integration)
- Supply chain security

# Example Policy Structure:
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: pod-security-standards
spec:
  validationActions: [Deny]
  matchConstraints: [Pod resources]
  validations:
    - expression: "!object.spec.containers[0].securityContext.privileged"
Strengths:

Kubernetes-native (CRD-based)
Pre/post-admission control
Auto-remediation capabilities
Supports complex CEL expressions
4. PROJECT CALICO (Cloud-Native Networking & Security)
Architecture & Approach:

Dataplane Choices: eBPF, Standard Linux, Windows, VPP
Network Policies: Kubernetes NetworkPolicy + Calico NetworkPolicy
Security Model: Network microsegmentation + encryption
Key Rules/Policies:

YAML
# Network Policies
- Ingress/Egress rules
- Policy tiers (precedence management)
- Protocol matching (TCP/UDP/ICMP)
- Service account-based selection
- DNS policies (DNS Security)

# Example Policy:
apiVersion: projectcalico.org/v3
kind: NetworkPolicy
metadata:
  name: deny-all-ingress
spec:
  selector: all()
  types:
  - Ingress
  ingress: []

# Advanced Features:
- Application-layer policies (WAF integration)
- Encryption (WireGuard)
- Flow logs for compliance
Scale: Supports 8M+ nodes daily

5. KUBESCAPE (Comprehensive Kubernetes Security)
Architecture & Approach:

Multi-faceted: Risk analysis + Security + Compliance + Misconfiguration scanning
Policy Source: CIS Kubernetes Benchmark + NSA/CISA guidelines
Frameworks: Multiple compliance frameworks
CIS Kubernetes
NIST
SOC 2
PCI-DSS
HIPAA
Key Rules/Policies:

Code
- RBAC misconfigurations
- Pod security violations
- Network policy gaps
- Secret management issues
- Supply chain security (image scanning)
- Vulnerability assessment
Comparative Policy Framework Summary
Solution	Policy Type	Evaluation	Scope	Update Method
Falco	Rules (Lua)	Runtime	System calls	Event-driven
NeuVector	Profiles + Rules	Runtime + Network	Container/Node	Learning mode
Kyverno	Policy-as-Code (CEL)	Admission	Kubernetes objects	Declarative
Calico	Network Rules	Dataplane	Network traffic	ConfigMap/CRD
Kubescape	Framework Rules	Static + Runtime	K8s objects	Compliance scan
Key Security Dimensions Addressed
Container Runtime Security (Falco + NeuVector)
Process execution control
File system integrity
Network behavior monitoring
Privilege escalation prevention
Kubernetes-Level Security (Kyverno + Kubescape)
Pod security standards
Image verification
RBAC enforcement
Resource limits validation
Network policy enforcement
Network Segmentation (Calico + NeuVector)
Microsegmentation
Encryption in transit
Layer 7 inspection
DNS security

## Fortuna (đã triển khai và lộ trình)

**Ngắn hạn (đã có trong code):**

- Risk engine áp dụng rule cho **Pod** theo category (`pod-security`, `runtime-behavior`, `network-policy`, `compliance`, `secrets`) và **RBAC chỉ khi rule có tag `pod`** (tránh CEL kiểu `object.rules` trên Pod).
- **Enrich `object.fortuna.*`** trước khi evaluate: đếm tín hiệu `runtime_signals` theo `FORTUNA_RUNTIME_RISK_LOOKBACK_HOURS` (mặc định 24, tối đa 168), cờ `has_suspicious_exec`, `has_network_queue_anomaly`, `has_escape_related`, và host flags từ bảng `pods`.
- **YAML rules** mới: `runtime-signals-recent`, `runtime-hostnetwork-network-anomaly`, `runtime-privileged-with-signals`, `runtime-escape-class-signals` (CEL trên `object` + `fortuna`).
- **Risk worker / historical evaluator** gọi `YAMLEngine.EvaluateResource` khi có YAML engine để biểu thức CEL thực sự chạy (tránh mất override khi chỉ gọi trên `*Engine` nhúng).

**Trung hạn / dài hạn (roadmap sản phẩm):**

- **Correlation SA↔cluster-admin (đã triển khai trong risk engine, không chỉ correlator):** `object.fortuna.service_account_bound_to_cluster_admin` được tính từ `role_bindings` + `cluster_role_bindings` trong DB (ClusterRole `cluster-admin`). Rule YAML `cluster-admin-pod` chỉ fire khi cờ này true — bỏ heuristic “mọi SA khác default”.
- **Admission / PSS (Kiểu Kyverno) — lớp sync/post-admission:** Fortuna vẫn đánh giá trên object Pod đã sync (NATS/correlator), không phải webhook admission. Đã thêm rule `pss-host-namespaces`, `pss-privileged-container` (CEL an toàn với `has()`). **Chưa có** ValidatingWebhookConfiguration / chặn tạo workload tại API server.
- **eBPF / syscall (Falco-style):** Agent có sensor gửi `POST /api/v1/runtime/events`; REP → `runtime_signals`. Mở rộng probe/kernel coverage = roadmap R9; không đổi trong bước này.

**Đánh giá GAP đã xử lý (verify trong code):**

| GAP | Trạng thái | Kiểm tra |
|-----|------------|----------|
| Message normalized → risk → insight | **Đã đóng** (contract cùng payload JetStream) | `TestRiskWorker_Process_NormalizedPodMessage_EndToEnd`, `TestRiskWorker_Process_IdempotentInsightDedup` — `nats.Msg{Data}` = JSON correlator, `YAMLEngine`, `BatchCreateOrUpdateInsights`, `defer Stop()` |
| InsightStatusUpdater Pod | **Đã đóng** | Synthetic `raw_json` từ hàng `pods` (align `prepareEnrichedResourceData`); `TestInsightStatusUpdater_PodResolvesWhenPSSNoLongerApplies` |
| Binding re-eval | **Đã đóng** | `RoleBinding` / `ClusterRoleBinding` load theo `resource_uid`, `evaluateResource` + khớp title/type như Pod |
| API pod report + runtime signals (dashboard) | **Đã đóng** | E2E **TC-17** `GET /risk/pods/:uid/report` (summary runtime), **TC-18** `GET /runtime/pods/:uid/signals?sinceMinutes=1440` — `scripts/e2e/e2e-risk-center-full.sh` |

**GAP tiếp theo (ưu tiên còn lại):**

1. **JetStream / NATS server tích hợp** — Test hiện tại không chạy `nats-server` + consumer durable; chỉ chứng minh **cùng byte message** mà worker nhận sau subscribe. Bước sau: CI job với container NATS + publish `fortuna.normalized.*` (tùy chọn).  
2. **Lưu full Pod manifest** — Bảng `pods` không giữ `raw_json`; re-eval dựa trên trường đã sync. Nếu cần khớp 100% với API server, thêm cột hoặc object store (roadmap).  
3. **ValidatingWebhook (Kyverno-style)** — Chặn tại admission; Fortuna vẫn là post-sync.  
4. **eBPF R9** — Mở rộng probe; pipeline REP hiện đủ cho syscall → signal.

**GAP đã đóng (verify):**

- **Agent → Core**: contract JSON `pod.uid` + `syscall` + `target` / `target_path` — test `TestPostRuntimeEvents_AgentPayloadCreatesSemanticSignal`, `TestRuntimeEventPayload_UnmarshalAgentShape` (core), `TestEvent_JSON_CoreIngestContract` (agent).
- **Core → DB → API**: `GetPodRiskReport` trả `summary.runtimeSignals24h`, `podDirectInsightCount`, `runtimePolicyInsightCount` (cùng cửa sổ 24h với `FORTUNA_RUNTIME_RISK_LOOKBACK_HOURS` mặc định); `GET /runtime/pods/:uid/signals?sinceMinutes=1440` — test `TestGetPodRiskReport_*`, `TestGetRuntimeSignalsByPod_*`.
- **Dashboard**: `RUNTIME_SIGNALS_LOOKBACK_MINUTES` = 24×60 đồng bộ với báo cáo; Overview Pod hiển thị summary từ cùng API với tab Risks.