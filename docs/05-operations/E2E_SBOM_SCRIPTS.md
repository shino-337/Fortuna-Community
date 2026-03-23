# E2E SBOM — script trên cluster (luồng hiện tại)

Tài liệu này mô tả các script bash trong `scripts/e2e/` dùng để **kiểm thử end-to-end** luồng SBOM (Agent → Core → API) trên Kubernetes thật.

## Entry point

```bash
# Toàn bộ pipeline SBOM E2E (khuyến nghị sau deploy)
./scripts/e2e/run-e2e.sh --suite=sbom-full
```

Tương đương (wrapper):

```bash
./scripts/e2e/e2e-sbom-verify.sh --full-pipeline
# hoặc: --pipeline
```

Biến môi trường thường dùng:

| Biến | Mô tả |
|------|--------|
| `NAMESPACE` | Namespace Fortuna/Core (mặc định `fortuna`) |
| `SKIP_IF_NO_PULL` | Đặt `1` trên script CoreDNS nếu cluster không pull được image (bỏ qua bước) |

## Suite `sbom-full` — thứ tự script

1. **`test-sbom-pod-flow.sh`** — Pod **busybox** (hoặc tương đương) trong user namespace: tạo workload, chờ SBOM, gọi API detail, in `components`, `goVersion`, `sbomSource`.
2. **`test-sbom-distroless-hello.sh`** — Image **distroless** (heuristic): assert `sbomSource=distroless-heuristic`, PURL `pkg:generic/...`.
3. **`test-sbom-control-plane-coredns.sh`** — Deploy **CoreDNS** (`registry.k8s.io/coredns/coredns`) với Corefile, port **1053** — mô phỏng workload kiểu **control-plane / system** (không nhất thiết là pod trong `kube-system`).

## Biến Agent (daemonset) — A5 VFS / streaming

Các script có thể chạy khi Agent đã cấu hình (Helm values / DaemonSet env):

| Biến | Gợi ý |
|------|--------|
| `SBOM_FS_MODE` | `indexed` hoặc `materialize` — chế độ VFS (indexed giảm RAM) |
| `SBOM_FS_METRICS` | `off` — tắt log metric `[SBOM FS]` nếu cần log sạch |
| `SBOM_FS_SKIP_PATH_PREFIXES` | Prefix bỏ qua entry tar không khớp (A5 slice) |

Chi tiết kiến trúc: `docs/03-components/sbom/A5_STREAMING_VFS_PLAN.md`.

## Script đơn lẻ

| Script | Mục đích |
|--------|-----------|
| `e2e-sbom-verify.sh` | Kiểm tra pod có trong list SBOM API; `--full` chỉ chạy `test-sbom-pod-flow.sh` |
| `test-sbom-pod-flow.sh` | Luồng pod chuẩn + assert API |
| `test-sbom-distroless-hello.sh` | Distroless heuristic |
| `test-sbom-control-plane-coredns.sh` | CoreDNS (kiểu control-plane workload) |

## Tiền đề

- Node có **Agent** chạy và quét được namespace test.
- **Core** reachable trong `NAMESPACE` (login `admin` / `admin123` mặc định trong script — điều chỉnh nếu đổi secret).
- Cluster pull được image test (busybox, distroless, `registry.k8s.io/coredns/coredns`).

## Liên quan

- NATS smoke (khác phạm vi): [E2E_NATS_SBOM_PIPELINE.md](E2E_NATS_SBOM_PIPELINE.md)
- Backlog CI E2E: [SBOM_PIPELINE_REMAINING_PLAN.md](SBOM_PIPELINE_REMAINING_PLAN.md)
