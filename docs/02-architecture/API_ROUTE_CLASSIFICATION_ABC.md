# Phân loại route API: A / B / C (Clean Architecture)

Version: 1.0  
Date: 2026-03  
Tham chiếu: [ROUTE_MIGRATION_MAPPING.md](./ROUTE_MIGRATION_MAPPING.md), [ROUTE_SYNC_ANALYSIS.md](./ROUTE_SYNC_ANALYSIS.md)

---

## Nguyên tắc kiến trúc (3 nguyên tắc cố định)

1. **Domain API rõ ràng** — inventory, runtime, risk, graph, audit, policy.
2. **Ingest pipeline tách riêng** — agent/ingest không trộn với user API.
3. **Dashboard API là aggregate** — /dashboard/* tổng hợp, cache, pre-compute.

---

## Nhóm A — Đúng kiến trúc, giữ nguyên

| Path | Lý do |
|------|--------|
| **/api/v1/auth/** (login, register, [refresh]) | Auth = platform concern, không phải domain business. Chuẩn Okta/Auth0. |
| **/api/v1/health/** (dashboard-data-integrity, [live], [ready]) | Chuẩn K8s liveness/readiness; không đưa vào domain. |
| **/api/v1/me**, **/api/v1/users**, **/api/v1/change-password** | Identity management; có thể tách /iam/* sau nhưng giữ /users ổn. |
| **/api/v1/ws/** (pod/:uid, risks) | WebSocket ngoài domain REST; chuẩn /ws/*, /stream/*. |

✔ Không đổi path.

---

## Nhóm B — Chấp nhận tạm thời

| Path hiện tại | Ghi chú | Roadmap |
|---------------|---------|--------|
| **/api/v1/agent/** (sync, pod-*-metrics, pod-events) | Agent đã hardcode; đúng trong migration. | Về lâu dài: /ingest/v1/* (telemetry pipeline). Có thể thêm alias /ingest/v1/* rồi deprecate /agent/*. |
| **/api/v1/dashboard/** (stats, metrics/threat-velocity) | Aggregate/cache; không thuộc domain cụ thể. | ✔ Giữ /dashboard/*. |

---

## Nhóm C — Nên chỉnh sớm ✅ (đã thực hiện)

| Trước (root) | Sau (domain) | Lý do |
|--------------|--------------|--------|
| /pod-capabilities, /pod-capabilities/summary/*, /pod-capabilities/trends | **/inventory/pod-capabilities** (+ summary, trends) | Capabilities = inventory metadata (securityContext, privileged, hostNetwork…) → inventory analysis. |
| /insights, /insights/:id, …/resolve, …/acknowledge, …/dismiss, …/evaluate | **/risk/insights** (+ :id, resolve, acknowledge, dismiss, evaluate) | Tránh 2 source of truth; list đã là /risk/insights → CRUD thống nhất dưới risk. |
| /rules, /rules/:id, …/reload, …/metrics, …/matches | **/policy/rules** (+ :id, reload, metrics, matches) | Rules = policy engine. |
| /policies/templates, /policies/instances | **/policy/templates**, **/policy/instances** | Policy domain thống nhất. |
| /certificates (info, rotate, rotation/history) | **/cluster/certificates** (info, rotate, rotation/history) | Cluster security metadata. |

---

## Kiến trúc API mục tiêu (sau chỉnh Nhóm C)

```
/api/v1
├── auth/*
├── health/*
├── me, users, change-password
├── ws/*
├── cluster/
│   └── certificates/*
├── dashboard/*
├── inventory/*
│   ├── pods, sbom, serviceaccounts, clusters, deployments, replicasets
│   └── pod-capabilities, pod-capabilities/summary/*, pod-capabilities/trends
├── runtime/*
├── risk/*
│   └── insights (list + :id + resolve, acknowledge, dismiss, evaluate)
├── graph/*
├── audit/*
└── policy/*
    ├── rules, rules/:id, rules/reload, rules/:id/metrics, rules/:id/matches
    ├── templates/*
    └── instances/*
```

(Tương lai: **/ingest/** thay/alias cho /agent khi chuẩn hóa telemetry.)

---

## Đánh giá maturity

| Level | Đặc điểm |
|-------|----------|
| 1 Prototype | Route lộn xộn |
| 2 Early platform | Partial domain |
| 3 Production ready | Domain-based API |
| 4 Enterprise | Versioned + contract |

Sau khi áp dụng Nhóm C: **~Level 3 (production-ready)**; chuẩn hóa ingest + OpenAPI/contract → hướng Level 4.

---

## Sai lầm cần tránh

- Trộn domain API với ingest API.
- Trộn domain API với dashboard aggregate.
- Để capabilities/insights/rules/certificates ở root lâu dài → technical debt khi platform mở rộng.

---

## Tài liệu liên quan

- [ROUTE_MIGRATION_MAPPING.md](./ROUTE_MIGRATION_MAPPING.md) — mapping legacy → domain.
- [ROUTE_SYNC_ANALYSIS.md](./ROUTE_SYNC_ANALYSIS.md) — đồng bộ agent/core/dashboard.
- [API-ARCHITECT_AND_ROUTE_STANDARD.md](./API-ARCHITECT_AND_ROUTE_STANDARD.md) — chuẩn domain.
