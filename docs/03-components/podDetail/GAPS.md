# Pod Detail — Known Gaps

**Last Updated**: 2026-03-28

> Full GAP roadmap by phase: see `docs/03-components/risk-center/GAPS.md`.

## Completed

| ID | Description | Status |
|----|-------------|--------|
| R8 | Host mode default for Pod Detail runtime (`hostPID`, `POD_DETAIL_RUNTIME_SOURCE=host`, `POD_DETAIL_PROC_ROOT=/host/proc`) | ✅ Done |
| R2 | Full command line collection (`args`/`/proc/<pid>/cmdline`) instead of truncated process name | ✅ Done |
| R3 | Runtime identity (`uid/gid`, `working_dir`) collected end-to-end: agent → core → DB → UI | ✅ Done |
| R4 | `CapEff` from `/proc/<pid>/status` collected and displayed in UI | ✅ Done |
| R7 | Process diff detection (current vs previous snapshot) creates `runtime_events` with `capability=PROCESS_SNAPSHOT_DIFF` | ✅ Done |
| R1 | CPU/memory from kubelet summary; process-level `%CPU/%Memory` from host `/proc` | ✅ Done |
| R9-p1 | Runtime event ingest pipeline + event richness (`runtime_events` with pod_name/node_name/runtime/event_type/signal/mitre_technique/severity) | ✅ Done |
| R9-p1b | Falco integration: `FalcoReader` reads Falco JSONL → Core; fallback syscall=falco.alert; daemonset mount hostPath `/var/log/falco`; UI: Pod → tab Events → Security runtime events | ✅ Done |
| R6-p1 | `runtime_signals.count` for real occurrence counting (dedup by day, increment count); CSC uses `SUM(count)` for `MinOccurrences` | ✅ Done |

## Open Gaps

| ID | Description | Priority | Status |
|----|-------------|----------|--------|
| R5 | Network packet/throughput counters per flow — baseline + anomaly + cooldown suppression done; packet counters per-flow still needed | P2 | In Progress |
| R6 | Multi-signal chain scoring — dynamic scoring for network spike done; need multi-signal chain (required capabilities), advanced calibration, and process/network correlation | P2 | In Progress |
| R9 | eBPF sensor — currently only noop tracepoint (health-check). Need real event collection (pid/args/dst) + PID→podUID mapping (host mode) for pod-context events | P2 | Planned |
| R10 | Admission risk gate tuning — hybrid gate by sensitive namespace + risk threshold + mode `audit|enforce` done; need production threshold/namespace tuning | P2 | In Progress |
| — | Redis dedup/cache dependencies not available in current deployment | P3 | Planned |
| — | Prometheus metrics not instrumented for Pod Detail pipelines | P3 | Planned |
| — | NATS async/DLQ for Pod Detail events not implemented | P3 | Planned |
| — | Informer-based pod sync (currently polling) | P3 | Planned |
| — | Containers table (normalized) not created | P3 | Planned |
| — | DTO layer for read API missing | P3 | Planned |
| — | Exposure resolution layer missing | P3 | Planned |
| — | Multi-cluster mTLS identity not enforced | P3 | Planned |
| — | E2E test for distroless host inspection pending | P3 | Planned |

## Operational Notes

- DaemonSet agent: `deploy/fortuna-agent-daemonset.yaml` — memory limit **6Gi**, `SBOM_WORKERS=1`
- Build: `nerdctl -n k8s.io build -t docker.io/library/fortuna-agent:latest -f agent/Dockerfile .`
- Falco install: `bash scripts/deploy/install-falco-fortuna.sh`; enable: `FALCO_EVENTS_ENABLED=true`
- `bytesSent/bytesRecv` in `pod_network_connections` are `tx_queue/rx_queue` from `/proc/net/*` snapshot, not cumulative session bytes
- Key env vars:
  - Core: `POD_DETAIL_NET_SPIKE_COOLDOWN_MINUTES`, `ADMISSION_RISK_GATE_ENABLED`, `ADMISSION_RISK_SENSITIVE_NAMESPACES`, `ADMISSION_RISK_BLOCK_THRESHOLD`
  - Agent: `EBPF_ENABLED` (default `false`), `FALCO_EVENTS_ENABLED`, `FALCO_EVENTS_PATH`, `FALCO_EVENTS_POLL`
