# Commit: SBOM write guard + idempotent matcher (A1–A4, B1–B3)

## Summary

- **Write guard (immutability):** Single SBOM write entrypoint via `SBOMRepository`, context-based mutation flag, `assertMutable` with row lock and strict status validation.
- **Idempotent matcher:** Table `sbom_match_runs`, `EnsureMatchRun` with crash recovery and duplicate-key handling; worker gate before `MatchSBOM`.

## Changes

### A. Write guard (immutability)

- **A1** `core/internal/contextkeys/sbom.go`
  - Typed context key: `WithSBOMMutationAllowed(ctx)`, `IsSBOMMutationAllowed(ctx)` (default deny).

- **A2–A3** `core/internal/repository/sbom_repository.go`
  - `SBOMRepository` with `UpsertSBOMWithComponents(ctx, sbom, components)` as single write path.
  - `assertMutable(ctx, sbom)`: allow only `""`/`pending`; `finalized` requires context flag; unknown status returns error.
  - Row-level lock (`FOR UPDATE`) when loading existing SBOM by `pod_uid`.
  - Component insert: `ON CONFLICT (sbom_id, purl) DO UPDATE` (no silent drop).

- **A4** `core/internal/grpc/handler_sbom.go`
  - `SendSBOMFinding` uses `contextkeys.WithSBOMMutationAllowed(ctx)` and `repo.UpsertSBOMWithComponents`; no direct DB writes in handler.

### B. Idempotent matcher

- **B1** `core/pkg/models/sbom_match_run.go`, `core/migrations/083_add_sbom_match_runs.go`
  - Model `SBOMMatchRun`; table `sbom_match_runs` with PK `(sbom_id, version, mirror_version)`, index on `sbom_id`.
  - Migration 083 registered in `migrations.go`.

- **B2** `core/internal/repository/sbom_repository.go`
  - `EnsureMatchRun(ctx, sbomID, version, mirrorVersion) (bool, error)`:
    - First run: insert row `status=running` → return true.
    - Existing: if `running` and &lt; 15m → return false (duplicate); else update and return true (re-run/crash recovery).
    - On INSERT duplicate key: treat as skip (return false, nil) via `isDuplicateKey(err)` (GORM + "duplicate key value" / "violates unique constraint" / "23505").

- **B3** `core/pkg/worker/cve_matcher_worker.go`
  - Before `MatchSBOM`: `mirrorVersion = "ts-{hour}"` (placeholder); call `EnsureMatchRun`; if !ok log and skip.

### Hardening

- `assertMutable`: switch on status, reject invalid values.
- Components: conflict → UPDATE assigned columns instead of DO NOTHING.
- `staleAfter` 15m with comment: must be >= max expected match duration.
- `isDuplicateKey`: specific phrases to avoid false positives.

## Not done (deferred)

- C1–C3: matcher metrics + standardized log format.
- Tests: guard, EnsureMatchRun idempotency, crash recovery, concurrent SBOM update.
- Mirror version from `mirror_state` (replace ts-hour); roadmap: mirror-driven idempotency, `last_matched_mirror_version`, reprocessing.

## Files touched

- `core/internal/contextkeys/sbom.go` (new)
- `core/internal/repository/sbom_repository.go` (new)
- `core/internal/grpc/handler_sbom.go` (refactor)
- `core/pkg/models/sbom_match_run.go` (new)
- `core/migrations/083_add_sbom_match_runs.go` (new)
- `core/migrations/migrations.go` (register 083)
- `core/pkg/worker/cve_matcher_worker.go` (gate + mirrorVersion)
