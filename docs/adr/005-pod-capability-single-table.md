# ADR-005: Pod capability storage — keep single table for Phase P1

## Status
Accepted (2026-03-28)

## Context
G-DB-01 asked whether `declared` / `observed` / `effective` should split across tables or materialized views.

## Decision
**Retain `pod_capabilities` single table** with `capability_class` + `derived_from` + ADR-001 semantics through at least P1.

Rationale:
- Compatibility with existing APIs, CSC, and dashboard filters.
- Lower migration risk while semantics are being locked.

## Future
Revisit when:
- Volume forces partition by class, or
- Different retention policies per class, or
- Materialized views needed for analytics.

Add a **view** (`pod_capabilities_effective`) later without breaking writers.

## Related
- G-DB-01, ADR-001
