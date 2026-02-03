# Phân tích và tổng hợp luồng Capability (PCE)

*Tài liệu tổng hợp: logic, flow, rules, kiểm soát và cơ chế.*

---

## 1. Tổng quan

**Pod Capability Engine (PCE)** là bộ phận đánh giá và quản lý **offensive capabilities** của pod: từ cấu hình tĩnh (spec) và tín hiệu runtime (syscall/target) → khởi tạo/khuyến mãi trạng thái capability → ghi DB (`pod_capabilities`, `runtime_signals`, `pod_attack_steps`) và phục vụ API/Dashboard.

| Thành phần | Vai trò |
|------------|---------|
| **Evaluator** | Đánh giá capability từ pod spec (static) → gọi CSC khởi tạo với state `detected`. |
| **CapabilityStateController (CSC)** | Chủ sở hữu duy nhất cập nhật state: `InitializeCapability` (detected), `PromoteCapability` (confirmed/exploited/chained). |
| **REP (Runtime Event Processor)** | Nhận runtime event → signal → CSC.PromoteCapability. |
| **SignalAdapter** | Event thô → signal ngữ nghĩa (PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT, …) → ghi `runtime_signals`. |
| **Promotion rules** | Quy tắc: signal_type + min_occurrences + required_capabilities → promote_to. |
| **Capability metadata** | Mô tả capability, severity_base, confidence_base, produces_attack_steps. |
| **AttackStepInference** | Từ capability state `exploited` → tạo `pod_attack_steps` theo metadata. |

---

## 2. Luồng tổng thể (Flow)

### 2.1 Luồng Static (cấu hình pod → capability detected)

```
[Agent sync pods] → Core lưu pods (DB)
         │
         ├─► [PCE Scheduler] (định kỳ, mặc định 6h)
         │       └─► capability.EvaluateAllPods()
         │
         └─► [Agent API / Sync] khi có pod mới/cập nhật
                 └─► agent_service: capability.EvaluateAndUpsertPod(pod)
                         │
                         ▼
[capability.EvaluateAndUpsertPod]
    1. Kiểm tra pod active (pod_instances.status = active hoặc deleted_at IS NULL)
    2. EvaluatePod(db, pod) → []Capability (từ spec: hostPID, hostPath, privileged, RBAC, …)
    3. Với mỗi Capability: CSC.InitializeCapability(podUID, namespace, capabilityID, group, severity, evidence)
       → INSERT/ON CONFLICT pod_capabilities (state = "detected", confidence từ capability_metadata)
    4. upsertPodRiskProfile(pod, caps) → pod_risk_profiles
```

### 2.2 Luồng Runtime (event → signal → promotion)

```
[Agent / Sensor] đọc runtime events (file hoặc probe)
         │
         ▼
POST /api/v1/runtime-events  (payload: syscall, target, capability, pod_uid, namespace, …)
         │
[PostRuntimeEvents] → rep.ProcessRuntimeEvent(ctx, db, input)
         │
         ├─► 1. CREATE runtime_events (raw)
         │
         ├─► 2. SignalAdapter.AdaptAndPersist(event)
         │       • AdaptEvent: classifyEventToSignal(syscall, target, capability) → signal_type, category, confidence
         │       • Persist: runtime_signals (dedup pod_uid + signal_type + cùng ngày → update confidence nếu cao hơn)
         │
         ├─► 3. classifySignal(syscall, target, capability) → signal, mitre, baseScore
         │       • PROC_ROOT_PIVOT (open/openat/stat/readlink + /proc/1/root, …) → 90
         │       • FS_ESCAPE_ATTEMPT (mount/pivot_root + /proc,/sys,/dev,…) → 95
         │       • NAMESPACE_ESCAPE (setns/unshare/clone + /proc/…/ns/) → 85
         │       • CAPABILITY_MISUSE (mount/setns/pivot_root + SYS_ADMIN, pod không allow SYS_ADMIN) → 60
         │
         ├─► 4. ensurePodRiskProfile + upsertRuntimeScore (pod_risk_profiles.runtime_score, capabilities[])
         │
         └─► 5. scoreToCapability(runtimeScore) → capabilityID (ESC_RUNTIME_ACTIVE nếu ≥90, ESC_RUNTIME_PROBE nếu ≥60)
                   CSC.PromoteCapability(ctx, podUID, capabilityID, signalType, confidence)
                         │
                         ▼
[CSC.PromoteCapability]
    1. Đọc pod_capabilities (pod_uid, capability_id) → phải tồn tại (đã được InitializeCapability trước đó)
    2. Đọc promotion_rules WHERE capability_id = ? AND signal_type = ?
    3. Đếm runtime_signals (pod_uid, signal_type) → signalCount
    4. Với mỗi rule:
       • Chỉ promote nếu promote_to > current state (detected < confirmed < exploited < chained)
       • signalCount >= rule.MinOccurrences
       • Nếu rule.RequiredCapabilities: pod phải có đủ các capability đó trong pod_capabilities
       • Chọn rule tốt nhất (promote_to cao nhất thỏa điều kiện)
    5. Cập nhật pod_capabilities: state = bestRule.PromoteTo, confidence += ConfidenceBoost, evidence (promoted_by, promoted_at)
    6. Nếu promote_to == "exploited" → AttackStepInference.InferAttackSteps(podUID) → tạo pod_attack_steps từ capability_metadata.produces_attack_steps
```

### 2.3 Sơ đồ luồng tóm tắt

```
                    ┌─────────────────┐
                    │  Pod Spec (K8s)  │
                    └────────┬─────────┘
                             │ Agent sync / PCE Scheduler
                             ▼
                    ┌─────────────────┐     InitializeCapability
                    │  PCE Evaluator   │ ───────────────────────► pod_capabilities (state=detected)
                    └─────────────────┘     + pod_risk_profiles
                             │
                    ┌────────┴────────┐
                    │ Runtime events  │  (syscall, target, capability)
                    └────────┬────────┘
                             │ POST /runtime-events
                             ▼
                    ┌─────────────────┐     runtime_events (raw)
                    │  REP Processor   │ ──► runtime_signals (SignalAdapter)
                    └────────┬────────┘     classifySignal → score → capabilityID
                             │
                             ▼
                    ┌─────────────────┐     promotion_rules
                    │  CSC Promote    │ ──► pod_capabilities (state → confirmed/exploited)
                    └────────┬────────┘     confidence boost, evidence
                             │
                             │ if promoted to exploited
                             ▼
                    ┌─────────────────┐     capability_metadata.produces_attack_steps
                    │ AttackStepInfer │ ──► pod_attack_steps
                    └─────────────────┘
```

---

## 3. Logic chi tiết

### 3.1 Static evaluation (Evaluator)

- **Đầu vào**: Pod (DB) với các trường: HostPID, HostIPC, HostNetwork, Namespace, Volumes, VolumeMounts, ContainerSecurityContexts, ServiceAccount, ClusterID, …
- **Đầu ra**: Danh sách `Capability` (ID, Group, Severity, Evidence, Mitre).

| Điều kiện | Capability ID | Group | Severity |
|-----------|----------------|-------|----------|
| namespace == "kube-system" | CTRL_CONTROL_PLANE_POD | CTRL | MEDIUM |
| HostNetwork | NET_HOSTNETWORK | NET | MEDIUM |
| HostPID | ESC_HOSTPID_POD | ESC | HIGH |
| HostIPC | ESC_HOSTIPC_POD | ESC | HIGH |
| securityContext.privileged == true | ESC_PRIV_POD | ESC | CRITICAL |
| hostPath mount (/, /path) | ESC_HOSTPATH_NODE | ESC | CRITICAL |
| hostPID hoặc hostPath nhạy cảm (/proc, /sys, /run, /var/run, /dev) | ESC_RUNTIME_PROBE | ESC | HIGH |
| automountServiceAccountToken | ID_TOKEN_POD | ID | MEDIUM |
| RBAC: Role/ClusterRole có write verbs (create, update, delete, patch, *) | API_RBAC_WRITE_CLUSTER | API | HIGH |

- **Khởi tạo**: Mỗi capability → `CSC.InitializeCapability` → upsert `pod_capabilities` (state = `detected`, confidence từ `capability_metadata.confidence_base` hoặc 0.5).

### 3.2 Runtime classification (REP)

- **classifySignal(syscall, target, capability)**:
  - **PROC_ROOT_PIVOT**: open/openat/stat/readlink + target chứa `/proc/1/root`, `/proc/self/exe`, `/proc/1/exe`, hoặc `/proc/…/root`, `/proc/…/exe` → score 90.
  - **FS_ESCAPE_ATTEMPT**: mount/pivot_root + target bắt đầu /proc, /sys, /dev, /run, /var/run → score 95.
  - **NAMESPACE_ESCAPE**: setns/unshare/clone + target chứa /proc/…/ns/ → score 85.
  - **CAPABILITY_MISUSE**: mount/setns/pivot_root + capability SYS_ADMIN và pod không allow SYS_ADMIN → score 60.

- **scoreToCapability(runtimeScore)**:
  - score ≥ 90 → ESC_RUNTIME_ACTIVE (CRITICAL)
  - score ≥ 60 → ESC_RUNTIME_PROBE (HIGH)
  - còn lại → không trả capability (chỉ cập nhật runtime score trong pod_risk_profiles).

### 3.3 CapabilityStateController (CSC)

- **InitializeCapability**: Tạo mới hoặc ON CONFLICT cập nhật (group, severity, evidence). State luôn `detected`. Confidence từ `capability_metadata.confidence_base` hoặc 0.5.
- **PromoteCapability**:
  - Capability phải đã tồn tại (do static evaluation).
  - Lấy rules: `promotion_rules` WHERE capability_id AND signal_type.
  - Thứ tự state: detected (1) < confirmed (2) < exploited (3) < chained (4); chỉ promote lên cao hơn.
  - Đếm số lần signal: `runtime_signals` WHERE pod_uid AND signal_type.
  - Với mỗi rule: kiểm tra signalCount >= MinOccurrences và (nếu có) RequiredCapabilities đều có trong pod.
  - Chọn rule có promote_to cao nhất thỏa điều kiện → cập nhật state, confidence += ConfidenceBoost, evidence.
  - Nếu promote_to == exploited → gọi AttackStepInference.InferAttackSteps(podUID).

### 3.4 AttackStepInference

- Chỉ xử lý capability có state = `exploited`.
- Với mỗi capability: đọc `capability_metadata` → `produces_attack_steps` (mảng step_id).
- Với mỗi step_id: nếu chưa có `pod_attack_steps` thì tạo mới; nếu đã có thì có thể cập nhật confidence/evidence nếu cao hơn.
- Step có category và description cố định (NODE_FS_WRITE, PROC_ROOT_PIVOT, RBAC_ABUSE, …).

---

## 4. Rules: Promotion Rules và Capability Metadata

### 4.1 Bảng promotion_rules

| capability_id | signal_type | min_occurrences | required_capabilities | promote_to | confidence_boost |
|---------------|-------------|-----------------|------------------------|------------|------------------|
| ESC_HOSTPATH_NODE | PROC_ROOT_PIVOT | 1 | [] | confirmed | 0.2 |
| ESC_HOSTPATH_NODE | PROC_ROOT_PIVOT | 2 | [SYS_ADMIN] | exploited | 0.3 |
| ESC_HOSTPATH_NODE | FS_ESCAPE_ATTEMPT | 1 | [] | exploited | 0.3 |
| ESC_HOSTPID_POD | NAMESPACE_ESCAPE | 1 | [] | confirmed | 0.2 |
| ESC_HOSTIPC_POD | NAMESPACE_ESCAPE | 1 | [] | confirmed | 0.2 |
| ESC_PRIV_POD | CAPABILITY_MISUSE | 1 | [SYS_ADMIN] | confirmed | 0.2 |
| ESC_RUNTIME_PROBE | PROC_ROOT_PIVOT | 1 | [] | confirmed | 0.2 |
| ESC_RUNTIME_ACTIVE | PROC_ROOT_PIVOT | 3 | [] | exploited | 0.3 |
| ESC_RUNTIME_ACTIVE | FS_ESCAPE_ATTEMPT | 1 | [] | exploited | 0.3 |

- **required_capabilities**: Trong CSC được so khớp với `pod_capabilities.capability_id` (pod phải có đủ các row với capability_id nằm trong mảng này). Seed 051 dùng cả `"SYS_ADMIN"` (tên Linux cap) và mảng rỗng; để rule hoạt động đúng, giá trị nên là capability_id thực tế có trong hệ thống (vd. ESC_HOSTPATH_NODE, ESC_PRIV_POD).

### 4.2 Bảng capability_metadata

- **capability_id**, **domain**, **category**, **description**, **severity_base**, **confidence_base**.
- **preconditions**: Mảng capability_id cần có (cho mô tả/attack path).
- **produces_attack_steps**: Mảng step_id (NODE_FS_WRITE, PROC_ROOT_PIVOT, RBAC_ABUSE, …) dùng khi state = exploited để tạo pod_attack_steps.
- **expires_with_instance**, **supports_runtime_promotion**: Dùng cho lifecycle và logic (CSC chỉ promote capability đã tồn tại; metadata gợi ý capability có hỗ trợ promotion hay không).

(Các capability được seed: ESC_PRIV_POD, ESC_HOSTPID_POD, ESC_HOSTIPC_POD, ESC_HOSTPATH_NODE, ESC_RUNTIME_PROC_ROOT, ESC_RUNTIME_PROBE, ESC_RUNTIME_ACTIVE, ID_TOKEN_POD, NET_HOSTNETWORK, API_RBAC_WRITE_CLUSTER, CTRL_CONTROL_PLANE_POD.)

---

## 5. Kiểm soát và cơ chế

### 5.1 Kiểm soát đầu vào

| Điểm | Cơ chế |
|------|--------|
| Pod chỉ được đánh giá khi active | Evaluator: pod_instances.status = active hoặc deleted_at IS NULL. |
| Runtime event hợp lệ | PostRuntimeEvents: pod_uid, syscall bắt buộc; target/capability tùy loại signal. |
| Signal dedup | SignalAdapter: cùng pod_uid + signal_type + ngày → update confidence nếu cao hơn, không tạo bản ghi mới. |

### 5.2 Kiểm soát promotion

| Điểm | Cơ chế |
|------|--------|
| Chỉ promote capability đã tồn tại | CSC: không tạo mới trong PromoteCapability; capability phải đã có từ InitializeCapability (static). |
| State chỉ đi lên | detected → confirmed → exploited → chained; ruleStateOrder > currentStateOrder. |
| Số lần signal đủ | signalCount >= rule.MinOccurrences. |
| Required capabilities | Nếu rule có RequiredCapabilities thì pod phải có đủ các capability_id đó trong pod_capabilities. |
| Một rule tốt nhất | Chọn rule có promote_to cao nhất thỏa tất cả điều kiện. |

### 5.3 Kiểm soát confidence và evidence

- **InitializeCapability**: confidence = capability_metadata.confidence_base (hoặc 0.5); evidence = JSON từ evaluator.
- **PromoteCapability**: confidence += rule.ConfidenceBoost (cap 1.0); evidence thêm promoted_by (signal_type), promoted_at.

### 5.4 Cơ chế lên lịch và kích hoạt

| Kích hoạt | Nơi gọi | Điều kiện |
|-----------|---------|-----------|
| PCE Scheduler | core/cmd/main.go | PCESchedulerEnabled (mặc định true), interval (mặc định 6h) → EvaluateAllPods(). |
| Agent sync | agent_service | Khi xử lý pod (sau khi có pod từ cluster) → EvaluateAndUpsertPod(pod). |
| Agent API | agent_handlers | Có thể gọi EvaluateAllPods khi cần (vd. trigger thủ công). |
| Runtime | PostRuntimeEvents | Mỗi event → ProcessRuntimeEvent → CSC.PromoteCapability (nếu có capabilityID từ score). |

### 5.5 Bảng DB liên quan

| Bảng | Mục đích |
|------|----------|
| pod_capabilities | Trạng thái capability từng pod (state, confidence, evidence). |
| runtime_events | Event thô (syscall, target_path, capability, pod_uid). |
| runtime_signals | Signal ngữ nghĩa (signal_type, category, confidence), dedup theo pod+signal+ngày. |
| promotion_rules | Quy tắc signal → state (capability_id, signal_type, min_occurrences, required_capabilities, promote_to). |
| capability_metadata | Mô tả capability (severity_base, confidence_base, produces_attack_steps). |
| pod_attack_steps | Attack step sinh ra từ capability exploited (step_id, evidence). |
| pod_risk_profiles | Static risk, runtime score, danh sách capability_id. |

### 5.6 API phục vụ Dashboard / UI

- GET /api/v1/pod-capabilities (list)
- GET /api/v1/pod-capabilities/summary, /summary/capability, /summary/cluster, /summary/namespace, /summary/severity
- GET /api/v1/pod-capabilities/trends?days=7
- GET /api/v1/pods/:id/capabilities
- GET /api/v1/capability-metadata, /api/v1/capability-metadata/:id
- GET /api/v1/promotion-rules, /api/v1/promotion-rules/capability/:id, /api/v1/promotion-rules/signal/:type
- GET /api/v1/runtime-signals, /api/v1/runtime-signals/pods/:podUid
- GET /api/v1/attack-steps/pods/:podUid, /api/v1/attack-steps/summary

---

## 6. Tóm tắt

- **Logic**: Static từ pod spec → detected; runtime từ event → signal → score → capabilityID → promotion theo rules (confirmed/exploited); exploited → attack steps từ metadata.
- **Flow**: Agent/PCE Scheduler → Evaluator → CSC.InitializeCapability; Runtime events → REP → SignalAdapter → classifySignal → CSC.PromoteCapability → (nếu exploited) AttackStepInference.
- **Rules**: promotion_rules (signal_type + min_occurrences + required_capabilities → promote_to); capability_metadata (severity_base, confidence_base, produces_attack_steps).
- **Kiểm soát**: Chỉ pod active; chỉ promote capability đã tồn tại; state tăng dần; đủ số signal và required capabilities; dedup signal theo ngày; confidence và evidence có cập nhật có kiểm soát.
- **Cơ chế**: PCE Scheduler định kỳ, Agent sync/API trigger static; POST /runtime-events trigger runtime; CSC là điểm duy nhất cập nhật state capability; AttackStepInference chỉ chạy khi promote to exploited.

Tài liệu này tổng hợp từ: `core/pkg/capability/`, `core/pkg/rep/`, `core/internal/api/runtime_event_handlers.go`, `core/internal/scheduler/pce_scheduler.go`, migrations 041–051, và docs RUNTIME_SIGNALS_FLOW.
