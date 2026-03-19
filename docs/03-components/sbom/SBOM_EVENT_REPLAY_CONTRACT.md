# SBOM Event Replay Contract

This contract locks deterministic behavior for `sbom.created` processing.

## Required event fields

- `event_id`: immutable identifier for one emitted event.
- `timestamp`: event time used by replay guard (`sbom_processing_state`).
- `schema_version`: snapshot schema marker (`v1` now).
- `components_snapshot`: canonical component list.

## Snapshot canonical fields (must be treated as source-of-truth)

Each snapshot component carries:

- `purl`, `name`, `version`
- `normalized_name`
- `version_class` (`STRICT|LOOSE|INVALID|UNKNOWN`)
- `ecosystem`, `namespace`, `arch`
- `source`, `trust_level`, `original_purl`, `purl_validated`

Worker and matcher should prefer these canonical values over re-deriving from raw strings whenever available.

## Replay guard rule

`ClaimSBOMEvent(sbom_id, event_id, timestamp)` is atomic and last-write-wins:

- newer `timestamp` wins
- same `timestamp` + different `event_id` is treated as a distinct event
- same `timestamp` + same `event_id` is idempotent replay (skip)

This prevents stale events from overwriting newer processing outcomes.

