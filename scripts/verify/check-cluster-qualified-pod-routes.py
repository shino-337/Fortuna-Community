#!/usr/bin/env python3
from pathlib import Path
import re
import sys

ROUTE_FILES = [
    Path("core/internal/api/routes.go"),
    Path("core/internal/api/routes_inventory.go"),
    Path("core/internal/api/routes_runtime.go"),
    Path("core/internal/api/routes_risk.go"),
]

FORBIDDEN_ROUTE_HANDLERS = {
    "GetPodRuntimeBehaviorFacts(": "use GetPodRuntimeBehaviorFactsScoped",
    "GetPodRuntimeIncidents(": "use GetPodRuntimeIncidentsScoped",
    "GetPodRuntimeEvents(": "use GetPodRuntimeEventsScoped",
    "GetPodCapabilities(": "use GetPodCapabilitiesScoped",
    "GetPodProcessesByUID(": "use GetPodProcessesByUIDScoped",
    "GetPodNetworkConnectionsByUID(": "use GetPodNetworkConnectionsByUIDScoped",
    "GetPodNetworkTopDestinationsByUID(": "use GetPodNetworkTopDestinationsByUIDScoped",
    "GetPodEventsByUID(": "use GetPodEventsByUIDScoped",
    "GetPodRuntimeMetricsByUID(": "use GetPodRuntimeMetricsByUIDScoped",
    "GetRuntimeSignalsByPod(": "use GetRuntimeSignalsByPodScoped",
    "PostRuntimeEvents(db)": "use PostRuntimeEventsScoped",
    "PostRuntimeEventsV2(db)": "use PostRuntimeEventsV2Scoped",
}

FORBIDDEN_PRODUCTION_SYMBOLS = {
    "GetPodRuntimeBehaviorFacts(": "legacy UID-only runtime fact handler",
    "GetPodRuntimeIncidents(": "legacy UID-only runtime incident handler",
    "GetPodRuntimeEvents(": "legacy UID-only runtime event handler",
    "GetPodCapabilities(": "legacy UID-only Pod capability handler",
    "EnsureActiveInstance(": "legacy UID-only Pod lifecycle API",
    "TerminateInstance(": "legacy UID-only Pod lifecycle API",
    "IsActive(": "legacy UID-only Pod lifecycle API",
}

REQUIRED = {
    Path("core/internal/api/routes_runtime.go"): [
        'pods.Use(middleware.RequirePodUIDClusterScope(db, "uid"))',
        "GetPodRuntimeBehaviorFactsScoped(db)",
        "GetPodRuntimeIncidentsScoped(db)",
        "GetPodCapabilitiesScoped(db)",
    ],
    Path("core/internal/api/routes_risk.go"): [
        'riskPods.Use(middleware.RequirePodUIDClusterScope(db, "uid"))',
        "GetPodRuntimeEventsScoped(db)",
        "risk.GetRiskScoreForPodIdentity(db)",
        "risk.CalculateRiskScoreForPodIdentity(db)",
    ],
    Path("core/internal/api/routes_inventory.go"): [
        "GetPodByUIDScoped(db)",
        "GetPodCapabilitiesScoped(db)",
    ],
    Path("core/internal/api/routes.go"): [
        "PostRuntimeEventsScoped(db)",
        "PostRuntimeEventsV2Scoped(db)",
    ],
}

# Detect identity-bearing direct predicates, not free-text/search filters.
POD_UID_WHERE = re.compile(
    r'Where\(\s*"([^"]*(?:[A-Za-z_][A-Za-z0-9_]*\.)?pod_uid\s*(?:=|IN)\s*[^"]*)"',
    re.IGNORECASE,
)

# Optional filters below are safe because the base query JOIN already binds the
# Pod row by both cluster_id and pod_uid before the filter is applied.
JOIN_SCOPED_FILTERS = {
    "pod_capabilities.pod_uid = ?",
    "pc.pod_uid = ?",
    "n.pod_uid = ?",
}

errors = []

for path in ROUTE_FILES:
    if not path.exists():
        errors.append(f"missing route file: {path}")
        continue
    text = path.read_text(encoding="utf-8")
    for token, replacement in FORBIDDEN_ROUTE_HANDLERS.items():
        if token in text:
            errors.append(f"{path}: legacy Pod route handler {token!r} is forbidden; {replacement}")
    for token in REQUIRED.get(path, []):
        if token not in text:
            errors.append(f"{path}: required cluster-qualified route contract missing: {token!r}")

for path in Path("core").rglob("*.go"):
    if path.name.endswith("_test.go"):
        continue
    text = path.read_text(encoding="utf-8")

    for token, reason in FORBIDDEN_PRODUCTION_SYMBOLS.items():
        if token in text:
            errors.append(f"{path}: forbidden production symbol {token!r}: {reason}")

    for match in POD_UID_WHERE.finditer(text):
        raw_sql = match.group(1)
        sql = raw_sql.lower().strip()
        if sql in JOIN_SCOPED_FILTERS:
            continue
        if "cluster_id" not in sql:
            line = text.count("\n", 0, match.start()) + 1
            errors.append(
                f"{path}:{line}: UID-only Pod storage predicate is forbidden: "
                f'Where("{raw_sql}")'
            )

if errors:
    print("Cluster-qualified Pod access contract FAILED:")
    for error in errors:
        print(f" - {error}")
    sys.exit(1)

print("Cluster-qualified Pod access contract: PASS")
