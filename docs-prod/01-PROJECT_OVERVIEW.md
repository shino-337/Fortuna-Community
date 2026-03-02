# Giới thiệu dự án Fortuna (KSAM)

## 1. Tổng quan

**Fortuna** (mã nguồn KSAM) là nền tảng bảo mật và quản lý rủi ro cho Kubernetes, cung cấp:

- **SBOM (Software Bill of Materials):** Trích xuất SBOM từ image container trên từng node.
- **CVE:** Ghép nối CVE với package trong SBOM, đánh giá mức độ nghiêm trọng.
- **Security Insights:** Tạo insight (critical/high/medium) từ CVE và policy.
- **Pod Capability Engine (PCE):** Phát hiện capability runtime, promotion rules, attack steps.
- **Runtime Signals:** Thu thập và tương quan tín hiệu bảo mật theo thời gian thực.
- **Dashboard:** Giao diện web thống kê, Risk Center, SBOM, PCE, runtime signals.

Hệ thống gồm **Core** (trung tâm xử lý + API), **Agent** (DaemonSet trên mỗi node), **Dashboard** (React), và hạ tầng **PostgreSQL**, **NATS JetStream**.

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

- **Core:** Service trung tâm (Go), API REST + gRPC, CVE matching, PCE, insights.
- **Agent:** DaemonSet (Go), theo dõi pod, trích xuất SBOM, gửi dữ liệu lên Core qua gRPC (mTLS).
- **PCE:** Pod Capability Engine – động cơ đánh giá capability và promotion rules.
- **SBOM:** Software Bill of Materials – danh sách package/version trong image.
- **Insight:** Bản ghi rủi ro/khuyến nghị (critical/high/medium) gắn với resource (pod/cluster).

---

**Tiếp theo:** [Kiến trúc hệ thống](02-ARCHITECTURE.md)
