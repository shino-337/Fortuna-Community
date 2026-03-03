# Giới thiệu sản phẩm FortunaK8s

## 1. Tổng quan

**FortunaK8s** là nền tảng **K8S Security & Risk Management**, cung cấp:

- **SBOM (Software Bill of Materials):** Trích xuất SBOM từ image container trên từng node.
- **CVE:** Ghép nối CVE với package trong SBOM, đánh giá mức độ nghiêm trọng.
- **Security Insights:** Tạo insight (critical/high/medium) từ CVE và policy.
- **Pod Capability Engine (PCE):** Phát hiện capability runtime, promotion rules, attack steps.
- **Runtime Signals:** Thu thập và tương quan tín hiệu bảo mật theo thời gian thực.
- **Dashboard:** Giao diện web thống kê, Risk Center, SBOM, PCE, runtime signals.

Hệ thống gồm **Fortuna Core** (trung tâm xử lý + API), **Fortuna Agent** (DaemonSet trên mỗi node), **Fortuna Dashboard** (React), và hạ tầng **PostgreSQL**, **NATS JetStream**.

---

## 2. Mục tiêu

- **Visibility:** Hiển thị SBOM, CVE, insight, capability và runtime signals cho toàn cluster.
- **Risk management:** Tập trung rủi ro tại Risk Center, theo cluster/pod/severity.
- **Compliance & audit:** Dữ liệu SBOM/CVE/insight phục vụ báo cáo và kiểm toán.
- **Production-ready:** mTLS, auth, migrations, health checks, tài liệu và script vận hành chuẩn production.

---

## 3. Phạm vi

- **Trong phạm vi:** Cluster Kubernetes (on-prem hoặc cloud), Core + Agent + Dashboard + PostgreSQL + NATS, build/deploy/clean/verify qua script production.
- **Ngoài phạm vi:** CI/CD cụ thể của từng tổ chức, tích hợp SIEM/SOAR (có thể mở rộng qua API/metrics).

---

## 4. Đối tượng sử dụng tài liệu

| Đối tượng | Tài liệu gợi ý |
|-----------|----------------|
| Quản trị hệ thống / DevOps | [05-OPERATIONS](05-OPERATIONS.md), [06-CONFIGURATION](06-CONFIGURATION.md), script-prod |
| Kiến trúc / Tech lead | [02-ARCHITECTURE](02-ARCHITECTURE.md), [03-FEATURES](03-FEATURES.md) |
| Người dùng Dashboard | [04-USER_GUIDE](04-USER_GUIDE.md) |
| Developer tích hợp | [02-ARCHITECTURE](02-ARCHITECTURE.md), [06-CONFIGURATION](06-CONFIGURATION.md), API reference trong `docs/` |

---

## 5. Thuật ngữ

- **FortunaK8s:** Tên sản phẩm – K8S Security & Risk Management Platform.
- **Core (Fortuna Core):** Service trung tâm (Go), API REST + gRPC, CVE matching, PCE, insights.
- **Agent (Fortuna Agent):** DaemonSet (Go), theo dõi pod, trích xuất SBOM, gửi dữ liệu lên Core qua gRPC (mTLS).
- **PCE:** Pod Capability Engine – động cơ đánh giá capability và promotion rules.
- **SBOM:** Software Bill of Materials – danh sách package/version trong image.
- **Insight:** Bản ghi rủi ro/khuyến nghị (critical/high/medium) gắn với resource (pod/cluster).
- **Pod Detail:** Thông tin chi tiết pod (Pod IP, Start Time, Uptime, Restart Count, Owner, QoS Class) theo [POD_DETAIL_SPEC](../docs/03-components/podDetail/POD_DETAIL_SPEC.md).

---

## 6. Tài liệu kỹ thuật (specs) và test

| Tài liệu | Mô tả |
|----------|--------|
| [POD_DETAIL_SPEC](../docs/03-components/podDetail/POD_DETAIL_SPEC.md) | Spec trang Pod Detail (header, overview, tabs). |
| [POD_SYNC_ARCHITECTURE_AND_DATA_MODEL_SPEC](../docs/03-components/podDetail/POD_SYNC_ARCHITECTURE_AND_DATA_MODEL_SPEC.md) | Spec hash, PCE bất đồng bộ, race protection. |
| [testSuite.md](../docs/03-components/podDetail/testSuite.md) | Test suite integration (spec hash, PCE, race). |
| [docs/README.md](../docs/README.md) | Chỉ mục đầy đủ tài liệu dev (migrations, API, E2E). |

---

**Tiếp theo:** [Kiến trúc hệ thống](02-ARCHITECTURE.md)
