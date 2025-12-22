# E2E Test: Pod → SBOM → CVE → Insights API (Event-Driven, Postgres/OSV)

This end-to-end test verifies the **current KSAM design**:

`Pod event → Digest resolution → SBOM cache-first → Persist SBOM → publish ksam.sbom.created → CVE matching (Postgres/OSV) → cve_matches → vulnerability Insights → Insights API`

It does **not** require the dashboard (API-only verification).

---

## What this test guarantees

- **Rebuild + redeploy Core** with a new image tag
- **Binary is updated** (Core logs contain build marker from `-ldflags`)
- **Caches are cleared** (SBOM/CVE artifacts removed from DB)
- **CVE data is loaded** into PostgreSQL using `cve-loader` (synthetic OSV JSON for deterministic test)
- **Creating a pod triggers** SBOMWorker → `ksam.sbom.created` → CVEMatcherWorker
- **DB is updated and linked by digest** via `sboms.image_digest` and `pod_image_scans.sbom_id`
- **Insight is visible via API**: `GET /api/v1/insights`

---

## Prerequisites

- A Kubernetes cluster with KSAM installed (namespace `ksam` by default):
  - PostgreSQL service: `svc/postgres`
  - NATS service configured
  - `ksam-core` deployment
  - `ksam-agent` daemonset (required to emit Pod events)
- Tools:
  - `kubectl`
  - `docker`
  - `curl`
  - `python3`
  - Go toolchain wrapper ≥ 1.21 (recommended):

```bash
go install golang.org/dl/go1.21.13@latest
go1.21.13 download
```

---

## Run the E2E test (Real CVE demo)

From repo root:

```bash
chmod +x KSAM/test_e2e_cve_insights_v2.sh
KSAM/test_e2e_cve_insights_v2.sh
```

## Additional End-to-End Testcases (SBOM/CVE)

- **SBOM cache-hit (digest-based)**:

```bash
chmod +x KSAM/test_e2e_sbom_cache_hit.sh
KSAM/test_e2e_sbom_cache_hit.sh
```

- **CVE version boundary (below fixed vs at fixed)**:

```bash
chmod +x KSAM/test_e2e_cve_boundary_versions.sh
KSAM/test_e2e_cve_boundary_versions.sh
```

- **Multi-container Pod (2 images)**:

```bash
chmod +x KSAM/test_e2e_multi_container_sbom_cve.sh
KSAM/test_e2e_multi_container_sbom_cve.sh
```

### Optional environment variables

- `NAMESPACE` (default: `ksam`)
- `E2E_NS` (default: `ksam-e2e`)
- `CORE_DEPLOY` (default: `ksam-core`)
- `CORE_IMAGE_REPO` (default: `ksam/core`)
- `TAG` (default: auto timestamp)
- `BUILD_COMMIT` (default: `local`)
- `GO_WRAPPER` (default: `go1.21.13`)
- `KIND_CLUSTER_NAME` (optional, enables `kind load docker-image`)

Example:

```bash
NAMESPACE=ksam TAG=e2e-$(date +%s) BUILD_COMMIT=$(git rev-parse --short HEAD) KSAM/test_e2e_cve_insights_v2.sh
```

---

## What to look for (expected outputs)

- Core pod logs contain:
  - `[Build] version=<TAG> commit=<BUILD_COMMIT> time=<...>`
- DB artifacts exist after creating the test pod:
  - `sboms` has a row (with `image_digest`)
  - `pod_image_scans` has `sbom_id` set
  - `cve_matches` contains `CVE-2099-9999`
  - `insights` contains a `type=vulnerability` row with `cve_id=CVE-2099-9999`
- API returns the vulnerability insight:
  - `GET /api/v1/insights?type=vulnerability`

---

## Notes

- This test uses a **real CVE JSON** from the existing dataset:
  - Default: `KSAM/cve-data/all/CVE-2014-0011.json`
  - It affects Debian:10 package `vnc4` with a fixed version `4.1.1+X4.3.0+t-1` and includes CVSS.
- To keep the test deterministic and offline-friendly, the pod runs a small custom image that contains only a dpkg status file:
  - `Package: vnc4`
  - `Version: 4.1.1+X4.3.0+t-0` (intentionally vulnerable: `< 4.1.1+X4.3.0+t-1`)
- You can override which CVE to demo:

```bash
REAL_CVE_FILE="KSAM/cve-data/all/CVE-<YYYY>-<NNNN>.json" TEST_CVE_ID="CVE-<YYYY>-<NNNN>" KSAM/test_e2e_cve_insights_v2.sh
```


