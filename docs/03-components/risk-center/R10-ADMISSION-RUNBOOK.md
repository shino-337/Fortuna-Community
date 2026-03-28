# R10 — Admission risk gate runbook (G-R10)

## Env vars (Core pod)
| Variable | Role | Example |
|----------|------|---------|
| `ADMISSION_RISK_GATE_ENABLED` | Master switch | `true` |
| `ADMISSION_RISK_SENSITIVE_NAMESPACES` | Comma-separated NS where block/escalate applies | `prod,fortuna` |
| `ADMISSION_RISK_BLOCK_THRESHOLD` | Numeric risk threshold for block path | `70` |
| `ADMISSION_RISK_GATE_MODE` | `audit` (log only) vs enforce (when supported) | `audit` |

Baseline in repo: `deploy/fortuna-core-deployment.yaml`.

## Verify
```bash
bash scripts/verify/verify-admission-risk-gate.sh
```
Requires `kubectl` + namespace `fortuna` (or `FORTUNA_NAMESPACE`). Without cluster: script exits 0 and skips.

## CI
`.github/workflows/build.yml` runs the script after Go tests (optional cluster).

## Webhook
ValidatingWebhookConfiguration `fortuna-policy-webhook` and Service `fortuna-webhook` must be installed for enforcement; see `deploy/webhook-config.yaml` when enabling beyond audit.
