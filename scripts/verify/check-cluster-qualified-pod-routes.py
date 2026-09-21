#!/usr/bin/env python3
from pathlib import Path
import sys

ROUTE_FILES = [
    Path("core/internal/api/routes.go"),
    Path("core/internal/api/routes_inventory.go"),
    Path("core/internal/api/routes_runtime.go"),
    Path("core/internal/api/routes_risk.go"),
]

FORBIDDEN = {
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

errors = []
for path in ROUTE_FILES:
    if not path.exists():
        errors.append(f"missing route file: {path}")
        continue
    text = path.read_text(encoding="utf-8")
    for token, replacement in FORBIDDEN.items():
        if token in text:
            errors.append(f"{path}: legacy Pod route handler {token!r} is forbidden; {replacement}")
    for token in REQUIRED.get(path, []):
        if token not in text:
            errors.append(f"{path}: required cluster-qualified route contract missing: {token!r}")

if errors:
    print("Cluster-qualified Pod route contract FAILED:")
    for error in errors:
        print(f" - {error}")
    sys.exit(1)

print("Cluster-qualified Pod route contract: PASS")
