# FORTUNA_RUNTIME_DETECTOR_CATALOG.md

## Status
Draft v0.1

## Purpose
Catalog này liệt kê “detector đầu tiên cần build” theo đúng yêu cầu:
- shell exec
- serviceaccount token read
- external egress
- payload fetch
- tmp exec
- host path access
- recon burst
- exfil-like

Mỗi detector mô tả:
- detector_id + type (stateless/stateful)
- input behavior facts (Layer 2)
- output runtime signal / runtime incident
- window + group_by (nếu stateful)
- evidence schema (refs)
- confidence model (hint rule)

Ghi chú quan trọng:
- Fortuna hiện tại đang mapping event → signal trực tiếp (REP v1). Catalog này chuẩn hóa theo REP v2: facts → signals/incidents.
- Khi triển khai P0/P1, detector catalogue này là “source of truth” để implement và test.

---

## Common Types
- Fact types (ví dụ): `PROCESS_EXEC`, `INTERACTIVE_SHELL`, `SERVICEACCOUNT_TOKEN_READ`, `NETWORK_CONNECT`, `EXTERNAL_EGRESS`, `REMOTE_PAYLOAD_FETCH`, `TMP_BINARY_EXEC`, `HOST_PATH_ACCESS`, `NETWORK_RECON`…
- Signal types: là security semantics, dùng cho rules/scoring/capability promotion.
- Incident types: correlated, stateful condition để risk engine dùng.

---

## Detector List (Priority - build first)

### D1: shell exec
- detector_id: `SHELL_EXEC_DETECTED`
- type: stateless
- inputs:
  - `PROCESS_EXEC`
  - (optional but preferred) `INTERACTIVE_SHELL`
- match (fact extraction assumptions):
  - `PROCESS_EXEC.exe` in shell interpreters: `/bin/sh`, `bash`, `dash`, `busybox sh`
  - `PROCESS_EXEC.cmdline` contains interactive markers (`-c`, `-i`, `exec` redirections) OR parent-chain contains known shells
- emit_signal:
  - `SUSPICIOUS_SHELL_EXEC`
- expected domain:
  - `Execution`
- confidence:
  - base: 0.7
- evidence_refs:
  - `fact_ref: [fact_id]`
  - `event_ref: [event_id]` (thông qua fact)

### D2: serviceaccount token read
- detector_id: `SERVICEACCOUNT_TOKEN_READ_DETECTED`
- type: stateless
- inputs:
  - `FILE_READ` + sensitive path match
- match:
  - file path equals `/var/run/secrets/kubernetes.io/serviceaccount/token` (hoặc các biến thể mount)
  - operation read
- emit_signal:
  - `SERVICEACCOUNT_TOKEN_READ` (domain credentials)
- confidence:
  - base: 0.8
- evidence_refs:
  - `fact_ref`: FILE_READ facts + derived token fact

### D3: external egress
- detector_id: `EXTERNAL_EGRESS_DETECTED`
- type: stateless
- inputs:
  - `NETWORK_CONNECT`
- match:
  - `NETWORK_CONNECT.dst_ip` is public (not in podCIDR/serviceCIDR/node CIDR whitelist)
  - protocol tcp/udp
- emit_signal:
  - `EXTERNAL_EGRESS`
- confidence:
  - base: 0.65
- evidence_refs:
  - include dst/proto tuples through fact attributes

### D4: payload fetch
- detector_id: `REMOTE_PAYLOAD_FETCH_DETECTED`
- type: stateless (có thể nâng lên stateful ở P1)
- inputs:
  - `NETWORK_CONNECT`
  - (optional) `FILE_WRITE` to tmp/exec locations
  - (optional) `PROCESS_EXEC` showing downloader tooling
- match:
  - downloader process hints: `curl|wget|busybox wget|python requests|perl|ruby`
  - destination resembles external http(s) endpoints
  - if available: fact `REMOTE_PAYLOAD_FETCH` already derived by fact extractor
- emit_signal:
  - `REMOTE_PAYLOAD_FETCH`
- confidence:
  - base: 0.7
- evidence_refs:
  - network fact + process exec fact (if present)

### D5: tmp exec
- detector_id: `TMP_EXEC_DETECTED`
- type: stateless
- inputs:
  - `PROCESS_EXEC`
  - fact attribute indicates executable path under writable staging:
    - `/tmp/*`
    - `/dev/shm/*`
    - optionally `/var/tmp/*`
- emit_signal:
  - `TMP_BINARY_EXEC`
- confidence:
  - base: 0.75
- evidence_refs:
  - include `exec_path`, `cmdline`, `parent exe` if available

### D6: host path access
- detector_id: `HOST_PATH_ACCESS_DETECTED`
- type: stateless
- inputs:
  - `FILE_READ` / `FILE_WRITE` / `HOST_PATH_TOUCH`
- match:
  - file path touches known host-mounted namespaces/paths (e.g. `/var/lib/kubelet`, `/etc`, `/run/containerd/containerd.sock`, `/var/run/docker.sock`)
  - hostPath mount evidence present in facts or from enrichment
- emit_signal:
  - `HOST_PATH_ACCESS` (domain: filesystem/escape)
- confidence:
  - base: 0.75
- evidence_refs:
  - mount evidence ref + file fact ref

### D7: recon burst
- detector_id: `RECON_BURST_DETECTED`
- type: stateful
- window:
  - 60s (P0 recommendation), tune later
- group_by:
  - pod_uid
  - (optional) container_id
- requires:
  - repeated `NETWORK_CONNECT` to many distinct dsts OR repeated `DNS_QUERY`
  - or combined `PROCESS_ENUMERATION` fact
- correlation logic (P0 minimal):
  - if unique destinations >= N (e.g. N=10) within window → emit incident
- emit_incident:
  - `RECON_BURST`
- confidence:
  - high: if dst variety AND repeated DNS/connect both exist
  - medium otherwise
- suppression:
  - cooldown 30m
- evidence_refs:
  - list of signal refs contributing within the window

### D8: exfil-like (P1.2 target)
- detector_id: `EXFIL_LIKE_SEQUENCE_DETECTED`
- type: stateful
- window:
  - 5m
- group_by:
  - pod_uid
  - (optional) container_id
- requires (minimal chain):
  - sensitive credential/secret read facts:
    - `SERVICEACCOUNT_TOKEN_READ` OR `SECRET_MATERIAL_READ` OR `K8S_API_TOKEN_USE`
  - followed by external egress:
    - `EXTERNAL_EGRESS` with public dsts
  - optional enrichment:
    - large payload fetch/write facts if you derive bytes/size
- emit_incident:
  - `EXFIL_LIKE_SEQUENCE`
- confidence:
  - high if chain is present in order (best-effort)
  - medium if only conjunction within window
- suppression:
  - cooldown 30m
- evidence_refs:
  - fact refs for token read + egress signals within window

---

## Rollout note
- P0 ship set: D1..D7
- P1.2 add-on: D8 (`EXFIL_LIKE_SEQUENCE_DETECTED`) sau khi network/file semantics ổn định và giảm false positives.

## Implementation Notes (how to map from current event pipeline)
Trong hệ thống hiện tại, bạn chưa có `runtime_behavior_facts` và `runtime_incidents`.
Khi implement P0/P1:
- Fact extractor có thể “backfill facts” từ `runtime_events` bằng deterministic mapping:
  - `execve` → `PROCESS_EXEC` (+ derived `INTERACTIVE_SHELL` / `TMP_BINARY_EXEC`)
  - `openat/read` sensitive path → `FILE_READ` → `SERVICEACCOUNT_TOKEN_READ`
  - `connect` → `NETWORK_CONNECT` → `EXTERNAL_EGRESS` + (option) parse dst/proto
- Stateful detectors (D7/D8) chạy job/worker theo window bucketing:
  - periodic scan new facts/signals in last X minutes
  - emit idempotent incidents

