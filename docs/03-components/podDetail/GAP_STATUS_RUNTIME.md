# Pod Detail Runtime GAP Status

Cap nhat: 2026-03-28

Tong hop GAP + roadmap day du theo phase: xem `docs/03-components/risk-center/FORTUNA_RUNTIME_IMPLEMENTATION_BACKLOG.md` **Muc 9**.

Cap nhat truoc: 2026-03-26 (Falco Helm + Pod Detail UI security runtime events)

## Da hoan thanh

- R8: Host mode mac dinh cho Pod Detail runtime (`hostPID`, `POD_DETAIL_RUNTIME_SOURCE=host`, `POD_DETAIL_PROC_ROOT=/host/proc`).
- R2: Thu thap full command line (`args`/`/proc/<pid>/cmdline`) thay vi truncated process name.
- R3: Thu thap runtime identity (`uid/gid`, `working_dir`) va luu xuyen suot agent -> core -> DB -> UI.
- R4: Thu thap `CapEff` tu `/proc/<pid>/status` va hien thi UI.
- R7: Process diff detection (snapshot hien tai so voi snapshot truoc) tao `runtime_events` voi `capability=PROCESS_SNAPSHOT_DIFF`.
- R1 (phan CPU/memory that): runtime metrics pod/container lay tu kubelet summary; process-level `%CPU/%Memory` tu host `/proc` (co env flag bat/tat).
- R9 (phase-1): Runtime event ingest pipeline + event richness (luu `runtime_events` co `pod_name/node_name/runtime/event_type/signal/mitre_technique/severity`), `POST /api/v1/runtime/events` khong can JWT.
- R9 (phase-1b Falco): Agent `FalcoReader` doc Falco JSONL → Core; fallback `syscall=falco.alert` khi thieu `evt.type`; daemonset mount `hostPath /var/log/falco` + env `FALCO_EVENTS_*`. **Van hanh:** reader tail tu EOF lan dau (tranh doc backlog JSONL cuc lon → OOM); resolve `pod_uid` bang Kubernetes **LIST** (`metadata.name=`) khi **GET** pod bi RBAC chan; can memory limit agent du lon + rotate/truncate `events.jsonl` tren host neu can. Cai Falco (Helm): `bash scripts/deploy/install-falco-fortuna.sh`; bat agent: `FALCO_EVENTS_ENABLED=true` (co the cap nhat trong `deploy/fortuna-agent-daemonset.yaml`). UI: Pod → tab Events → **Security runtime events** (API `GET /risk/pods/:uid/runtime/events`).
- R6 (phase-1): `runtime_signals.count` de dem occurrence that su (dedupe theo ngay nhung van tang count), CSC dung `SUM(count)` cho `MinOccurrences`.

## Dang hoat dong va verify gan nhat

- `runtime_events` co ban ghi `PROCESS_SNAPSHOT_DIFF`.
- `pod_runtime_metrics` co so lieu `cpu_usage_millicore`/`memory_usage_bytes` khac 0.
- `pod_processes` co `cpu_percent`/`memory_percent` khac 0.
- Agent imageID da duoc dong nhat tren master/worker sau recreate.

## Chua hoan thanh (GAP con lai)

- R5: Da co baseline + anomaly + cooldown suppression; phan packet/throughput counters theo flow van can bo sung.
- R6: Da co dynamic scoring cho network spike; can tiep tuc multi-signal chain (required capabilities), calibration nang cao, va correlation theo process/network.
- R9: Hien tai eBPF sensor moi attach "noop tracepoint" (health-check). Can implement thu thap event that su (pid/args/dst) + map PID -> podUID (host mode) de tao event co pod context.
- R10: Da co hybrid gate theo namespace nhay cam + threshold risk + mode `audit|enforce`; can tiep tuc tuning threshold/namespace theo moi truong production. Verify nhanh: `bash scripts/verify/verify-admission-risk-gate.sh` (can cluster + kubectl).

## Ghi chu van hanh

- DaemonSet agent: xem `deploy/fortuna-agent-daemonset.yaml` — memory limit **6Gi**, `SBOM_WORKERS=1`, Falco env + comment tail EOF; build image: `nerdctl -n k8s.io build -t docker.io/library/fortuna-agent:latest -f agent/Dockerfile .` (tu repo root). Chi tiet: `agent/README.md`, backlog **Muc 9.6**.
- Push image multi-node hien mac dinh verify digest (`VERIFY_REMOTE_DIGEST=true`) trong `push-images-to-workers.sh`.
- Build script da dong bo short tag voi canonical `docker.io/library/*` de tranh lech digest khi dung `:latest`.
- Truong `bytesSent/bytesRecv` trong `pod_network_connections` hien tai la `tx_queue/rx_queue` tu `/proc/net/*` snapshot, khong phai tong byte theo flow.
- Env mac dinh moi:
  - Core: `POD_DETAIL_NET_SPIKE_COOLDOWN_MINUTES`, `ADMISSION_RISK_GATE_ENABLED`, `ADMISSION_RISK_SENSITIVE_NAMESPACES`, `ADMISSION_RISK_BLOCK_THRESHOLD`.
  - Agent: `EBPF_ENABLED` (mac dinh `false`, bat dan theo rollout), `FALCO_EVENTS_ENABLED` / `FALCO_EVENTS_PATH` / `FALCO_EVENTS_POLL`.
