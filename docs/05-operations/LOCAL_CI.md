# Run GitHub CI locally

`scripts/verify/run-local-ci.sh` runs the jobs in `.github/workflows/ci.yml` with
[`act`](https://nektosact.com/). It is a preflight for the current working tree,
not a replacement for the required GitHub check on the exact PR head.
PR #54 has a specific owner-approved local-gate exception while hosted jobs
fail before execution because of account entitlement; see
[`NEXT_AUDIT_PLAN.md`](../06-reference/NEXT_AUDIT_PLAN.md#54-local-exact-head-verification).
That exception requires results for the exact committed SHA and does not make
an interrupted `act` run or earlier working-tree run sufficient evidence.

## Requirements

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
```

`all` executes the five workflow job groups sequentially and stops at the first
failure. The helper runs the Go matrix (`core`, `agent`, `api`) one entry at a
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
