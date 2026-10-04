# Performance baseline — 2026-09-29

Five repeated runs compare `7ee43a9d8` with the query/cache changes in
`0470c7f97`, using identical new benchmark fixtures. Raw samples, medians and
log hashes are versioned in [the measurement artifact](performance/20260929.json).
The shared AMD Ryzen 7 8845HS VM used Go 1.26.8, GOMAXPROCS=1 and an isolated
PostgreSQL 16/AGE 1.6 database. Elapsed times describe this test environment;
they are not production SLOs.

| Operation | Before median | After median | Go allocated bytes/op before → after | Allocations/op before → after |
| --- | ---: | ---: | ---: | ---: |
| Trends: 50,000 rows, 30 days | 473.7 ms | 74.3 ms | 155,784,709 → 60,554 | 3,000,127 → 971 |
| Cached graph: 500 paths, two nodes plus nested properties | 13.75 ms | 8.16 ms | 1,599,797 → 1,154,708 | 31,013 → 23,010 |

Trend response memory now depends on UTC calendar buckets rather than loading
all matching RiskScore models. Cluster allowlists, namespace filters, cutoff and
soft deletion remain inside the query. Daily, Monday-start weekly, monthly and
yearly boundaries and failures are tested on SQLite and PostgreSQL. Response
fields and prior priority/min/max behavior are retained.

The graph cache encodes immutable results once on write and decodes independent
response objects on read. Nested redaction cannot mutate later callers' data;
failed encoding cannot cache an empty success. Byte allocation fell about 28%
and allocation count about 26%. Expired-entry removal also uses compare-and-delete,
so a stale reader cannot delete a concurrent replacement.

The six-Agent integration gate exposed overlapping full-sync historical/PCE
scans. They now coalesce per database: one pass runs, and syncs received during
that pass request one subsequent refresh. A burst of 200 triggers has a permanent
regression for maximum concurrency and retained pending work. This bounds scan
fanout; it does not guarantee a scan can process an arbitrarily large cluster.

## Repeat the measurements

From `core`, point only at an isolated test database:

```bash
FORTUNA_TEST_POSTGRES_URL='postgres://test-user:test-password@127.0.0.1:TEST_PORT/fortuna_test?sslmode=disable' GOWORK=off GOTOOLCHAIN=go1.26.8 GOMAXPROCS=1 GOFLAGS=-p=1 go test ./internal/api/risk -run '^$' -bench '^BenchmarkRiskTrends50000$' -benchmem -benchtime=3x -count=5 -cpu=1

GOWORK=off GOTOOLCHAIN=go1.26.8 GOMAXPROCS=1 GOFLAGS=-p=1 go test ./pkg/graph -run '^$' -bench '^BenchmarkAttackPathCacheRead$' -benchmem -benchtime=10x -count=5 -cpu=1
```

The trend fixture creates/drops its own schema and is opt-in; require five
benchmark result rows rather than accepting a skipped benchmark. It measures the
local Gin handler/query/response, excluding network and JWT verification. Allocated
bytes are Go allocations per operation, not peak RSS or PostgreSQL server memory.
Cache write cost is outside the measured read loop.

Security-state caching defaults to five minutes. For an explicit fresh-read lab,
`FORTUNA_SECURITY_STATE_CACHE_TTL=0s` recomputes state; accepted overrides cannot
exceed five minutes. The live gate records this setting, 30-second Agent sync and
25 database connections. A production workload must measure its own cadence,
concurrency, query plan and cold-cache costs.

The receipt retention/partition/archive and storage-capacity follow-up in
[NEXT_AUDIT_PLAN.md](NEXT_AUDIT_PLAN.md#52-production-operations-follow-up-not-a-correctness-merge-blocker)
remains open. These benchmarks do not enable runtime absence resolution or prove
large-scale storage readiness. The backend first-investigation flow is recorded
in [integration acceptance](../06-reference/INTEGRATION_ACCEPTANCE.md); live Dashboard navigation
and the S2 browser walkthrough remain deployment validation.
