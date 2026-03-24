# Pod Detail Runtime GAP Status

Cap nhat: 2026-03-24

## Da hoan thanh

- R8: Host mode mac dinh cho Pod Detail runtime (`hostPID`, `POD_DETAIL_RUNTIME_SOURCE=host`, `POD_DETAIL_PROC_ROOT=/host/proc`).
- R2: Thu thap full command line (`args`/`/proc/<pid>/cmdline`) thay vi truncated process name.
- R3: Thu thap runtime identity (`uid/gid`, `working_dir`) va luu xuyen suot agent -> core -> DB -> UI.
- R4: Thu thap `CapEff` tu `/proc/<pid>/status` va hien thi UI.
- R7: Process diff detection (snapshot hien tai so voi snapshot truoc) tao `runtime_events` voi `capability=PROCESS_SNAPSHOT_DIFF`.
- R1 (phan CPU/memory that): runtime metrics pod/container lay tu kubelet summary; process-level `%CPU/%Memory` tu host `/proc` (co env flag bat/tat).

## Dang hoat dong va verify gan nhat

- `runtime_events` co ban ghi `PROCESS_SNAPSHOT_DIFF`.
- `pod_runtime_metrics` co so lieu `cpu_usage_millicore`/`memory_usage_bytes` khac 0.
- `pod_processes` co `cpu_percent`/`memory_percent` khac 0.
- Agent imageID da duoc dong nhat tren master/worker sau recreate.

## Chua hoan thanh (GAP con lai)

- R5: Da co baseline + anomaly + cooldown suppression; phan packet/throughput counters theo flow van can bo sung.
- R6: Da co dynamic scoring cho network spike; van con chuoi hanh vi (multi-signal chain) va calibration nang cao.
- R9: Da co scaffold `cilium/ebpf` + env rollout fail-open (`EBPF_ENABLED`), chua attach sensor production.
- R10: Da co hybrid gate theo namespace nhay cam + threshold risk trong admission; can bo sung full policy manifest rollout va tuning.

## Ghi chu van hanh

- Push image multi-node hien mac dinh verify digest (`VERIFY_REMOTE_DIGEST=true`) trong `push-images-to-workers.sh`.
- Build script da dong bo short tag voi canonical `docker.io/library/*` de tranh lech digest khi dung `:latest`.
- Truong `bytesSent/bytesRecv` trong `pod_network_connections` hien tai la `tx_queue/rx_queue` tu `/proc/net/*` snapshot, khong phai tong byte theo flow.
- Env mac dinh moi:
  - Core: `POD_DETAIL_NET_SPIKE_COOLDOWN_MINUTES`, `ADMISSION_RISK_GATE_ENABLED`, `ADMISSION_RISK_SENSITIVE_NAMESPACES`, `ADMISSION_RISK_BLOCK_THRESHOLD`.
  - Agent: `EBPF_ENABLED` (mac dinh `false`, bat dan theo rollout).
