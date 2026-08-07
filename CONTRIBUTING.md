# Contributing to Fortuna

Fortuna is an open-source Kubernetes security platform focused on **attack paths, runtime evidence, RBAC exposure, vulnerabilities, and unified risk**. Contributions should make the project easier to run, easier to validate, or better at answering a concrete Kubernetes security question.

## Start Here

- Read [README.md](README.md) for the product model and user paths.
- Check the [Roadmap](ROADMAP.md) for security capabilities and community priorities.
- Read [docs/README.md](docs/README.md) for the documentation map.
- For installation and deployment work, start with [Getting Started](docs/01-getting-started/README.md).
- For script changes, read [scripts/README.md](scripts/README.md) and keep scripts in the documented directory contract.

## What Contributions Are Most Valuable?

The highest-value contributions are concrete and reproducible:

- **Attack paths:** new Kubernetes privilege-escalation, lateral-movement, or blast-radius scenarios.
- **Detection engineering:** RBAC, pod-security, runtime, Falco, or eBPF detections.
- **Security research:** reproducible research that exposes a detection gap or improves risk correlation.
- **Vulnerability intelligence:** SBOM, CVE/OSV correlation, exploitability context, and evidence quality.
- **Risk analytics:** better explainability, prioritization, and attack-path scoring.
- **Validation:** regression tests and reproducible Kubernetes scenarios.
- **Documentation:** installation, troubleshooting, security concepts, and practical examples.

For attack-path or research contributions, use the GitHub issue templates to describe the scenario before implementing a large change.

## Repository Rules

- Keep examples generic. Do not commit real kubeconfigs, credentials, database dumps, private screenshots, generated reports, local test output, or node-specific files.
- Do not commit generated binaries, packaged archives, local CVE datasets, `node_modules`, dashboard build output, or container image exports.
- Prefer immutable release image tags such as `v1.0.0` or `sha-<commit>` in user-facing docs. Use local tags only in clearly marked development commands.
- Keep public docs aligned with the package path `ghcr.io/shino-337/fortuna-community`.
- Keep README concise. Put detailed operational procedures in `docs/` and link to them from README.

## Local Setup

Fortuna has three Go modules and one dashboard package:

```bash
go work use ./core ./agent ./api
```

Use Node.js 20+ for dashboard development.

## Checks

```bash
# Core
cd core
go test ./...

# Agent
cd ../agent
go test ./...

# Dashboard
cd ../dashboard
npm ci
npm run typecheck
npm run build
```

Additional checks by touched area:

| Area | Minimum check |
|------|---------------|
| Shell scripts | `find scripts -type f -name '*.sh' -print0 \| xargs -0 -n1 bash -n` |
| Kubernetes/GitHub YAML | Parse changed YAML with a YAML parser or run the relevant `kubectl --dry-run=client` check |
| Deployment flow | `./scripts/verify/check-full-deployment.sh` on a live cluster when deploy behavior changes |
| Multi-cluster flow | `./scripts/verify/verify-multicluster-sync.sh` when remote Agent behavior changes |
| Dashboard UI | `npm run typecheck` and `npm run build`; run Playwright checks when changing UI flows |
| Docs only | `git diff --check` and verify links/commands against current scripts/manifests |

Use the scripts under [scripts](scripts) for local cluster workflows and image distribution. See [scripts/README.md](scripts/README.md) for supported entrypoints.

## Pull Requests

- Keep changes focused and explain the security or operational impact.
- Use a user-facing PR title, for example `feat: detect cross-namespace privilege escalation`, rather than an internal batch/task identifier.
- Include tests or a clear verification note for behavior changes.
- Update README/docs/deploy examples when configuration or installation behavior changes.
- Do not include generated binaries, packaged archives, local CVE datasets, or node-specific files.
- For roadmap-sized work, open a feature, attack-path, or research issue before implementing.
- Keep release/package references consistent with the current public package layout.

## Documentation Changes

When changing docs:

- Update the nearest detailed guide first, then update README only if the top-level user path changes.
- Keep setup docs split between package install and local source build.
- Avoid references to local hostnames, private IPs, kubeconfigs, node credentials, or generated reports.
- Keep sample YAML under `deploy/samples/` when it helps users configure packages or image pull secrets.

## Release and Package Changes

Release-facing changes must keep these aligned:

- README Quick Start.
- [docs/01-getting-started/QUICKSTART.md](docs/01-getting-started/QUICKSTART.md).
- [deploy/README.md](deploy/README.md) and any changed manifests.
- `.github/workflows/publish-images.yml` when image publishing behavior changes.
- `scripts/README.md` when script entrypoints or package source behavior changes.

## Security Issues

Do not open a public issue with exploit details or secrets. Follow [SECURITY.md](SECURITY.md) and use private vulnerability reporting when available.
