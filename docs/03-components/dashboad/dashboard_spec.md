Tài liệu Đặc tả API - Fortuna Security Dashboard

1. Tổng quan (General)
Base URL: /api/v1
Authentication: Bearer Token (JWT)
Format: JSON
2. Nhóm API: Phân tích SBOM & Lỗ hổng (Phần cốt lõi mới)
Cung cấp dữ liệu cho trang SBOM Analysis để theo dõi các thành phần phần mềm và rủi ro từ thư viện/package.
2.1. Lấy danh sách SBOM của các Pod
Endpoint: `GET /api/v1/sbom`
Mục tiêu: Liệt kê các Pod đã được agent scan, kèm summary số lượng lỗ hổng theo mức severity.
Params (optional):
  * `limit` - giới hạn số records (mặc định 50, tối đa 200).
Phản hồi mẫu:

```json
{
  "sboms": [
    {
      "podId": "ad42e32e-...",
      "podName": "e2e-pod",
      "namespace": "production",
      "image": "fortuna/core:latest",
      "containerName": "core",
      "lastScan": "2026-01-24T16:32:50Z",
      "packageCount": 91,
      "vulnerabilitySummary": {
        "critical": 5,
        "high": 12,
        "medium": 47,
        "low": 102
      }
    }
  ]
}
```

2.2. Lấy chi tiết SBOM của một Pod cụ thể
Endpoint: `GET /api/v1/sbom/{podId}`
Mục tiêu: Trả về SBOM components + mỗi component kèm danh sách CVE tương ứng (mô tả, điểm CVSS, giải pháp).
Phản hồi mẫu:

```json
{
  "podId": "ad42e32e-...",
  "namespace": "production",
  "podName": "e2e-pod",
  "container": "core",
  "image": "fortuna/core:latest",
  "generatedAt": "2026-01-24T16:32:50Z",
  "packageCount": 91,
  "components": [
    {
      "id": 4,
      "name": "Kernel",
      "version": "6.6.83",
      "type": "os-package",
      "purl": "pkg:linux/kernel@6.6.83",
      "vulnerabilities": [
        {
          "id": "CVE-2025-40268",
          "severity": "medium",
          "cvssScore": 5.0,
          "description": "...",
          "fixedVersion": "6.17.9"
        }
      ]
    }
  ]
}
```
3. Nhóm API: Dashboard & Tổng quan
3.1. Lấy thống kê tổng hợp (Stats)
Endpoint: `GET /api/v1/dashboard/stats`
Mục tiêu: Hiển thị các con số lớn ở đầu trang Dashboard.
Dữ liệu: Tổng số cluster, tổng rủi ro, rủi ro critical, số lượng pod, agent đang hoạt động.
3.2. Dữ liệu biểu đồ rủi ro (Threat Velocity)
Endpoint: `GET /api/v1/dashboard/metrics/threat-velocity?days=7`
Mục tiêu: Vẽ biểu đồ Area Chart về xu hướng rủi ro theo thời gian.
4. Nhóm API: Risk Center (Insights)
4.1. Danh sách rủi ro (Insights)
Endpoint: `GET /api/v1/risks`
Query Params: severity, category, status, search.
Mục tiêu: Cung cấp dữ liệu cho bảng triage.
4.2. Cập nhật trạng thái rủi ro
Endpoint: `PATCH /api/v1/risks/{riskId}`
Body: { "status": "acknowledged" | "resolved" }
5. Nhóm API: Attack Paths (Đồ thị tấn công)
5.1. Dữ liệu đồ thị
Endpoint: `GET /api/v1/attack-paths/graph`
Mục tiêu: Cung cấp Nodes (Pod, Svc, DB) và Links cho thư viện D3.js.
Dữ liệu:
code
JSON
{
  "nodes": [ { "id": "pod-1", "type": "pod", "risk": "critical" } ],
  "links": [ { "source": "internet", "target": "pod-1" } ]
}
6. Nhóm API: Quản trị Tài nguyên & Cấu hình
6.1. Resources Explorer
Endpoint: GET /resources
Query Params: kind (Pod, ServiceAccount, Role, etc.), namespace.
6.2. Quản lý Rule (Chính sách)
Endpoint: GET /rules
Mục tiêu: Danh sách các chính sách bảo mật (RBAC, Network...).
Endpoint: POST /rules/test
Mục tiêu: Chạy thử logic CEL (Common Expression Language) với payload YAML giả lập.
7. Nhóm API: Vận hành (Operations)
GET /monitoring/agents: Trạng thái các agent cài trên Node.
GET /monitoring/certificates: Theo dõi hạn dùng TLS/SSL.
GET /audit-logs: Nhật ký thao tác người dùng.
GET /reports: Danh sách các file PDF/CSV compliance đã tạo.
GET /notifications: Danh sách thông báo (unread/read).
8. Cấu trúc dữ liệu CVE (Vulnerability Object)
Để giao diện hiển thị rõ ràng như yêu cầu, mỗi đối tượng lỗ hổng cần các trường:
| Trường | Kiểu | Mô tả |
| :--- | :--- | :--- |
| id | String | Mã CVE (Ví dụ: CVE-2023-1234) |
| severity | Enum | critical, high, medium, low |
| cvssScore | Number | Điểm số từ 0.0 - 10.0 |
| description | String | Mô tả chi tiết lỗ hổng |
| fixedVersion | String | Phiên bản phần mềm đã vá lỗi (nếu có) |
| links | Array | Các liên kết tham khảo (NVD, Vendor advisory) |
Tài liệu này hỗ trợ đội ngũ Backend xây dựng đúng các schema cần thiết để Frontend có thể render dữ liệu SBOM và CVE một cách trực quan, giúp người dùng cuối dễ dàng đưa ra quyết định khắc phục rủi ro.