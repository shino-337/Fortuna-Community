# Run GitHub CI locally

`scripts/verify/run-local-ci.sh` runs the functional steps in `.github/workflows/ci.yml`
sequentially. The default `native` backend uses host Go/Node and small isolated
Python/PostgreSQL containers; the optional `act` backend uses
[`act`](https://nektosact.com/). It is a preflight for the current working tree,
not a replacement for the required GitHub check on the exact PR head.
PR #54 has a specific owner-approved local-gate exception while hosted jobs
fail before execution because of account entitlement; see
[`NEXT_AUDIT_PLAN.md`](../maintainers/NEXT_AUDIT_PLAN.md#historical-54-local-exact-head-verification).
That exception requires results for the exact committed SHA and does not make
an interrupted `act` run or earlier working-tree run sufficient evidence.

## Requirements

### Native backend (default on the constrained Kubernetes VM)

- Bash, Git, Python 3 with PyYAML for reading the workflow, Go and Docker.
- Go uses the exact `setup-go` toolchain from the workflow with `GOWORK=off`,
  serial package builds and `GOMAXPROCS=1` by default.
- The workflow Node major is checked. If the host version differs, the runner
  downloads an official Node archive into a dedicated cache and verifies its
  SHA-256; it does not replace the host Node installation. Use
  `LOCAL_CI_NODE_BIN=/absolute/path/to/node/bin` for a pre-provisioned runtime.
- Script checks run in Python 3.12 with the workflow's pinned PyYAML. PostgreSQL
  uses a unique temporary container and a dynamically allocated loopback port,
  never Fortuna's live database. Both containers are cleaned up by the runner.
- Dashboard browser dependencies are installed by the workflow's own Playwright
  step. Unknown workflow execution controls/jobs/actions/conditions fail rather
  than being omitted. Inherited Go test-selection flags and a live PostgreSQL
  test URL are discarded so they cannot skip tests or redirect DB writes.

### Optional act backend

- Linux x86-64, a working Docker daemon/socket, and `act` on `PATH`.
- Enough free disk for the medium `catthehacker/ubuntu:act-latest` runner image,
  action caches, Go/npm downloads, and Playwright Chromium. The full runner
  image is deliberately not used. Keep adequate free space for the running
  Kubernetes node.
- Network access on the first run to download the runner image, public actions,
  packages, and browser dependencies.
- Host loopback TCP port 5432 free while running `cluster-identity-postgres`,
  because the workflow maps its temporary PostgreSQL 16 service to that port.
  The binding is limited to `127.0.0.1`; do not point the CI test URL at
  Fortuna's live database.

Install `act` using the [official instructions](https://nektosact.com/installation/)
or a verified release binary. The repository `.actrc` selects the medium image,
limits act to one concurrent job, and reuses an already downloaded image.

## Commands

Run from anywhere in the checkout:

```bash
./scripts/verify/run-local-ci.sh list
./scripts/verify/run-local-ci.sh hygiene
./scripts/verify/run-local-ci.sh scripts
./scripts/verify/run-local-ci.sh go-test
./scripts/verify/run-local-ci.sh cluster-identity-postgres
./scripts/verify/run-local-ci.sh dashboard
./scripts/verify/run-local-ci.sh all
./scripts/verify/run-local-ci.sh --backend act all
```

`LOCAL_CI_BACKEND=native` (default) or `LOCAL_CI_BACKEND=act` selects the backend.
`LOCAL_CI_OUTPUT_DIR=/absolute/path/outside/the/repo` sets the parent for unique
native evidence directories; otherwise they are created under the system temp
directory. `LOCAL_CI_CACHE_DIR` controls the dedicated Node cache.

Native runs produce per-job logs and `results.json` containing the Git SHA,
dirty-worktree flag, source fingerprint, workflow hash, individual outcomes and
log hashes. A failure returns nonzero and preserves the failed log. Source
changes during a run invalidate the result. `publishable=true` requires all job
groups passing on a clean, unchanged commit; a dirty working-tree pass does not
authorize publishing success for its parent SHA. The runner does not publish
statuses, push, merge, deploy or reset live data automatically.

`all` executes the five workflow job groups sequentially and stops at the first
failure. The act helper runs the Go matrix (`core`, `agent`, `api`) one entry at a
time with `--matrix`, because act may initialize matrix entries concurrently
even when its concurrent-job limit is one. The
PostgreSQL service/container is temporary and separate from the Kubernetes
deployment; the script does not reset or deploy Fortuna.

The medium runner is not identical to GitHub's hosted `ubuntu-latest` image, so
an `act` pass is useful local evidence but GitHub Actions remains authoritative.
If the image or dependencies cannot be downloaded, run the desired job later
when network access and disk space are available; avoid switching to a full
runner image on a storage-constrained Kubernetes VM.

If a run is interrupted, inspect `docker ps -a --filter name=act-` and remove
only the containers/networks left by that specific run after confirming their
identities. In particular, confirm that the temporary PostgreSQL service and
its loopback port have stopped.
