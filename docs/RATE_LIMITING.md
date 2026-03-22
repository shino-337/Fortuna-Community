# API rate limiting (ingest / HTTP)

## Policy (defaults)

| Setting | Default | Meaning |
|--------|---------|---------|
| `RequestsPerSecond` | `100` | Sustained token refill rate per client key (IP or authenticated user). |
| `BurstSize` | `200` | Maximum burst of requests allowed before sustained limit applies. |
| `Enabled` | `true` | Global switch; when `false`, middleware is a no-op. |

Implementation: `golang.org/x/time/rate` per client key in `core/internal/middleware/rate_limiter.go`.  
Wiring: `core/internal/middleware/security.go` and `core/cmd/main.go` (configurable via env where exposed).

## Client key

1. If `user_id` is set on the Gin context → key `user:<id>`.
2. Else → key `ip:<ClientIP>` (fallback `RemoteIP`, then `unknown`).

## HTTP behavior on exceed

- Status **429 Too Many Requests**
- JSON body includes `error` and `message` with limit/burst hints for operators.

## Tests

Contract tests for burst exhaustion and post-burst refill live in `core/internal/middleware/rate_limiter_test.go` (aligned with `DefaultRateLimiterConfig`).

## Operations

- Tune RPS/burst for your ingress capacity and expected fan-in from agents.
- For stricter per-route limits, use `PerEndpointRateLimiter` (same package).
