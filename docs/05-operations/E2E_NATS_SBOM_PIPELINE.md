# E2E — NATS JetStream + SBOM/CVE pipeline (smoke)

Mục đích: xác minh **Core** publish/consume trên JetStream ngoài cluster (dev/staging), không thay thế unit test `publishSBOMCreatedWithRetry`.

## Tiền đề

- NATS Server **2.10+** với JetStream bật.
- Core image/binary với `NATS_URL`/`FORTUNA_NATS` trỏ tới broker (xem config repo).

## Chạy NATS nhanh (Docker)

```bash
docker run --rm -p 4222:4222 -p 8222:8222 nats:2.10-alpine -js -m 8222
```

Hoặc dùng manifest trong cluster (Helm `nats`); đảm bảo stream `fortuna-events` tồn tại (Core `SetupStreams` khi kết nối).

## Smoke checklist

| Bước | Việc | Kỳ vọng |
|------|------|---------|
| 1 | Core khởi động, log `NATS client initialized` | Không loop reconnect vô hạn |
| 2 | Ingest SBOM qua gRPC (hoặc API) | Publish `fortuna.sbom.created` (hoặc DLQ nếu primary fail) |
| 3 | `CVE matcher` consumer | Log xử lý event, không lỗi subscribe |
| 4 | Metric | `fortuna_cve_matcher_runs_total` / `fortuna_sbom_processed_total` tăng (theo scrape) |

## Gỡ lỗi

- **`consumer is already bound`**: tắt durable tạm (`FORTUNA_JS_DURABLES=false`) hoặc xóa consumer cũ trên JetStream.
- **Không thấy message**: kiểm tra subject nằm trong stream `fortuna-events` (`fortuna.sbom.>`).
- **DLQ traffic**: `fortuna_sbom_created_dlq_consumed_total` / gauge `fortuna_sbom_created_dlq_stream_messages` (xem `SBOM_ALERTING_SLO.md`).

## Tự động hóa

Unit tests trong repo mock publish/retry (`sbom_publish_retry_test.go`); **E2E đầy đờ** (container NATS + Core) nên chạy trong CI có service `nats` hoặc `testcontainers` — backlog optional.
