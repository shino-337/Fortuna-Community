# Shared API module

`github.com/fortuna/api` holds the types that Core and the Agent both compile against. Both modules use it through a `replace` directive (`../api`), so a change here affects both.

| Path | Contents |
|---|---|
| `proto/agent/` | gRPC service and messages between Agent and Core (`service.proto`, `sbom.proto`, `cve.proto`) and the generated Go code; regenerate with the `Makefile` there |
| `collection/` | Payloads for inventory, runtime events, runtime lifecycle and signed runtime source health |
| `cmd/source-health-sign/` | Signs a sensor's source-health report with a separately managed key; it observes nothing itself |

Run the tests with `cd api && go test ./...`; CI runs them together with `go vet` and a `gofmt` check.
