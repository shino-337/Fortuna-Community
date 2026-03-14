<!--
Spec: Distroless & System Pod SBOM Coverage
References:
- docs/02-architecture/Architecture_Finding_Remediation_Plan.md#Finding-8
- agent/pkg/sbom/extractor/extractor.go
- agent/internal/sbom/processor.go
- core/pkg/cve/database/manager.go
-->

# Fortuna SBOM spec: Distroless + system pods

## Purpose

Enable Fortuna to generate repeatable SBOMs for distroless and control-plane pods such as kube-apiserver, kube-proxy, CoreDNS, etc., without adding an external dependency. SBOMs must include enough metadata (name/version/ecosystem/PURL) so the existing CVE database (OSV + NVD) can still match vulnerabilities and insights can surface on the Risk Center.

## Requirements

1. **Image coverage**
   - Handle OCI images that lack `/etc/os-release` or package manager databases (distroless, scratch, minimal system images).
   - Support system pods that run as containers but do not expose shell utilities.
2. **Metadata quality**
   - Each SBOM entry must include `name`, `version`, `ecosystem`, and valid PURL (pkg:...) so SQL joins to `package_vulnerabilities` succeed.
   - Record source (`parsers`, `label-metadata`, `distroless-heuristic`) and confidence (`low/medium/high`).
3. **Performance**
   - Cache SBOMs by image digest.
   - Avoid repeated downloads by reusing containerd/registry artifacts.
4. **Traceability**
   - Tag SBOMs with signature DB version and inference rules version to know when caches expire.

## Architecture changes

### Agent extractor
1. **Enhanced detection**
   - Inside `agent/pkg/sbom/extractor/extractor.go`:
     - Read OCI config labels (e.g., `org.opencontainers.image.ref.name`) and layer history to detect distroless images.
     - When `detectOS` returns `unknown`, fall back to label-based heuristics and mark OS name `"distroless"`.
2. **Distroless cataloger**
   - Add new parser in `agent/pkg/sbom/extractor/parsers` (e.g., `distroless.go`) implementing `Parser`.
   - Logic: walk virtual FS (bin/lib directories, layer metadata) and create synthetic `Package` records:
     - `Name` derived from binary name or OCI label.
     - `Version` from image tag/digest/layer metadata; fallback to `unknown`.
     - `Ecosystem` set to `generic`.
     - Generate PURL `pkg:generic/<name>@<version>`.
   - Emit a `source` tag (`distroless-heuristic`) and set `confidence` based on how much metadata is available (label-based high, unknown low).
3. **Synthetic fallback**
   - Keep existing logic that adds one synthetic package when all parsers fail; ensure it now attaches the new metadata fields (source/confidence, PURL) and caches by digest.
4. **Signature DB**
   - Maintain JSON signature files under `agent/pkg/sbom/signatures/` enumerating known distroless binaries with canonical names/versions.
   - Log the signature DB version in SBOM metadata so future rules can invalidate caches.
5. **Cache**
   - Introduce `/var/lib/fortuna/sbom-cache` (or configurable path). Cache key = image digest + signature version. Agent tries cached SBOM before running parsers.

### Agent → Core payload

Extend protobuf `SBOMFinding` (and any related models) to include:

```proto
message SBOMFinding {
  ...
  string source = ...; // "parsers", "label-metadata", "distroless-heuristic"
  string confidence = ...; // "low", "medium", "high"
  repeated SBOMPackage packages = ...; // each package has purl + source
}
```

`SBOMPackage` should already contain name/version/type/architecture. Ensure we populate `ecosystem` and `purl`.

### Core ingest & CVE matching

1. **Schema**
   - Add columns to `sbom_findings`/`sbom_components` for `source`, `confidence`, `purl`, and `signature_version`.
2. **Matching logic**
   - In `core/pkg/cve/database/manager.go`, allow `Source` to inform fallback strategy:
     - `parsers` → use OSV dataset first (`core/pkg/cve/database/osv/` reader) so we stay offline.
     - `distroless-heuristic` or `label-metadata` with `confidence` < `high` → prefer NVD API query when OSV lacks a match.
3. **Dashboard signage**
   - Pod Detail page should display badge `Distroless SBOM (heuristic)` whenever `source` ≠ `parsers`. Provide panel explaining inference level using `confidence`.

## Data flow

1. Agent detects new pod, materializes image.
2. Check cache by digest + signature version → reuse if hit.
3. Run enhanced `detectOS`, built-in parsers, and fallback parser.
4. If heuristics triggered, build `SBOMPackage` list with `source/confidence/purl`.
5. Send `SBOMFinding` to Core via gRPC.
6. Core stores SBOM, triggers CVE matcher that consults the OSV dataset or NVD API depending on `source/confidence`.
7. Dashboard/Risk Center reads insights and shows distroless badge.

## Storage & refresh

| Item | Location | Refresh |
| ---- | -------- | ------- |
| Signature rules | `agent/pkg/sbom/signatures/*.json` | Bump version, redeploy agent |
| SBOM cache | `/var/lib/fortuna/sbom-cache` | Evict when digest/rule version changes |
| OSV dataset | `core/pkg/cve/database/osv/*.json` (or equivalent reader) | Refresh whenever new OSV feed is imported (daily/weekly) |
| NVD data | PostgreSQL `cves`, `package_vulnerabilities` | Updated daily via `cve_updater` |

## Testing & validation

1. Add E2E job that deploys `registry.k8s.io/coredns:latest` and a distroless test image (e.g., `gcr.io/distroless/static:nonroot`). Ensure agent sends SBOM with non-empty packages, `source=distroless-heuristic`, and `purl`.
2. Verify Core matches at least one CVE for these packages (OSV or NVD).
3. Dashboard pod detail shows badge with explanation.

## Rollout

1. Merge signature files + heuristics.
2. Deploy agent update; ensure caches expire by bumping signature version.
3. Monitor Core logs for `distroless-heuristic` entries and CVE matching via fallback.
4. Update docs (`docs/AGENT_CORE_CONNECTIVITY.md`, Pod Detail spec, SBOM custom plan) to describe new metadata and workflows.
