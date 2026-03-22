# SBOM / CVE pipeline — Alerting & SLO (OBS-1)

Fortuna Core đăng ký metric Prometheus (client); **scrape / rule** do Fortuna vận hành. Tài liệu này định nghĩa **ngưỡng gợi ý** và **SLO** để biến metric thành hành động.

## Metric gợi ý (labels quan trọng)

| Metric | Ý nghĩa | Gợi ý cảnh báo |
|--------|---------|----------------|
| `fortuna_component_unknown_version_ratio` | Tỷ lệ component `version=unknown` | `> 0.4` trong 30m **và** `resolver_version` không đổi → nghi parser/regression |
| `fortuna_component_unknown_version_total` | Đếm theo `reason` (distroless, inferred, …) | Spike theo một `reason` |
| `fortuna_sbom_created_dlq_consumed_total` | DLQ consumer đã ack (publish primary thất bại) | `rate() > 0` liên tục → kiểm tra NATS/JetStream |
| `fortuna_sbom_created_dlq_stream_messages` | Ước lượng message DLQ còn trên stream `fortuna-events` (poll `StreamInfo.State.Subjects`) | > 0 kéo dài → backlog DLQ; tắt poll: `FORTUNA_SBOM_DLQ_DEPTH_POLL_INTERVAL=0` |
| `fortuna_sbom_store_upsert_commits_total` | Mỗi lần commit upsert SBOM (`is_new` true/false) | Dùng làm mẫu số drift / ingest |
| `fortuna_cve_matcher_runs_total{result="skipped"}` | Gồm schema mismatch | Spike `skipped` → kiểm tra phiên bản agent event |
| `fortuna_sbom_drift_total` | `unexpected_same_version` | Bất kỳ increment nào → điều tra nondeterminism |
| `fortuna_epss_*` | EPSS lookup (khi bật `FORTUNA_EPSS_ENABLED`) | Spike `fortuna_epss_lookup_errors_total` → kiểm tra API/rate limit |
| `fortuna_kev_catalog_refresh_errors_total` | Lỗi tải/parse feed KEV | > 0 liên tục → kiểm tra URL/mạng |
| `fortuna_kev_catalog_cve_entries` | Số CVE trong catalog sau refresh thành công | Drop đột ngột → nghi feed |

## PromQL mẫu (tùy chỉnh theo scrape interval)

```promql
# Unknown version ratio cao (effective SBOM quality)
avg_over_time(fortuna_component_unknown_version_ratio[1h]) > 0.4

# DLQ traffic
increase(fortuna_sbom_created_dlq_consumed_total[15m]) > 0

# DLQ backlog (stream subject count — best-effort)
fortuna_sbom_created_dlq_stream_messages > 50

# Drift rate trên upsert (dùng counter drift / commits store)
sum(rate(fortuna_sbom_drift_total[1h])) / sum(rate(fortuna_sbom_store_upsert_commits_total[1h])) > 0.01
```

## SLO đề xuất (tham chiếu)

| SLO | Mục tiêu | Đo |
|-----|----------|-----|
| Component có version (effective) | ≥ 85% | `1 - fortuna_component_unknown_version_ratio` (theo label phù hợp) |
| Match confidence ≥ MEDIUM | ≥ 60% | Từ `fortuna_risk_confidence_distribution_ratio` hoặc log pipeline |

SLO không thay thế kiểm tra nghiệp vụ; dùng kết hợp drift + DLQ + unknown ratio.

## Ghi chú

- Multi-replica: một số gauge có thể phản ánh **run gần nhất trên replica đó** (xem MET-1 trong `SBOM_PIPELINE_REMAINING_PLAN.md`).
- Recording rules / alert routing: cấu hình trên Fortuna monitoring, không commit rule YAML cố định trong repo này.

### Đã đóng (gap OBS-1)

- **DLQ depth:** gauge `fortuna_sbom_created_dlq_stream_messages` (poll `StreamInfo` stream `fortuna-events`, subject `fortuna.sbom.created.dlq`). Interval: `FORTUNA_SBOM_DLQ_DEPTH_POLL_INTERVAL` (mặc định `30s`, `0`/`off` = tắt).
- **Drift ratio:** counter `fortuna_sbom_store_upsert_commits_total` làm mẫu số; PromQL mẫu ở trên.
