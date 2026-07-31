# Contributing to Fortuna

Fortuna is a Kubernetes security and risk management platform. Public contributions should keep the repository runnable from the documented release packages and from source.

## Start Here

- Read [README.md](README.md) and the setup docs under [docs/01-getting-started](docs/01-getting-started).
- Check [docs/README.md](docs/README.md) for the current documentation map.
- For install or deploy work, read [deploy/README.md](deploy/README.md) and [docs/05-operations/DEPLOYMENT.md](docs/05-operations/DEPLOYMENT.md).
- For script changes, read [scripts/README.md](scripts/README.md) and keep scripts in the documented directory contract.

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

Use the scripts under [scripts](scripts) for local cluster workflows and image distribution. See [scripts/README.md](scripts/README.md) for the supported entrypoints.

## Pull Requests

- Keep changes focused and explain the operational impact.
- Include tests or a clear verification note for behavior changes.
- Update README/docs/deploy examples when configuration or installation behavior changes.
- Do not include generated binaries, packaged archives, local CVE datasets, or node-specific files.
- For roadmap-sized work, open a feature request or design issue before implementing.
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

Do not open a public issue with exploit details or secrets. Follow [SECURITY.md](SECURITY.md).
