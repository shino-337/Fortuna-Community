#!/usr/bin/env python3
"""Prevent retired API consumers and handlers from returning."""
from pathlib import Path
import re
import sys

RETIRED_HANDLERS = (
    "GetBlastRadius", "GetShortestPath", "GetAccessibleResources",
    "ExecuteGraphQuery", "GetServiceAccountPermissionsGraph", "GetRiskyPods",
    "requireUnrestrictedLegacyGraph", "PostRuntimeEventsScoped",
    "bindRuntimeV1Payloads", "GetPodNetworkTopDestinationsByUID",
    "GetPodSpecYAML", "GetInsightsSummary", "QueryPrometheusMetrics",
    "GetServiceAccount", "UpdateServiceAccount", "DeleteServiceAccount", "GetServiceAccountPermissions",
    "GetClusterInfo", "GetClusterNodes", "GetTechniqueOverlayDigest", "EmergencyAccessPlaceholder",
    "TriggerCVEMatch", "BulkDeleteServiceAccounts", "BulkDisableServiceAccounts",
    "DisableInactiveServiceAccounts", "GetPolicyEvaluationCost", "GetRBACPermissionCatalog",
    "GetRuntimeRiskSummary", "GetTopRuntimeRisks", "GetPodAssetSecurityState",
)
RETIRED_ALIASES = (
    "/bulk/serviceaccounts/", "/monitoring/agents", "/policy/rules/:id",
    "/serviceaccounts/bulk/", "/serviceaccounts/disable-inactive", "/cluster/info",
    "/debug/technique-overlay", "/governance/emergency-access", "/trigger-cve-match",
    "/malware/check", "/malware/stats", "/malware/db/upload", "/policy-evaluation-cost",
    "/rbac/permission-catalog", "/risk/runtime/summary", "/risk/runtime/top", "/security-state",
)
RETIRED_GRAPH = ("blast-radius", "shortest-path", "accessible", "query", "permissions", "risky-pods")


def violations(path, source):
    errors = []
    for alias in RETIRED_ALIASES:
        if alias in source:
            errors.append("retired alias: " + alias)
    if path.endswith("routes_policy.go") and "/rules/:id" in source:
        errors.append("retired policy rule ID registration")
    if re.search(r"/cluster/(?::id|\$\{[^}]*\}|[\w-]+)/nodes\b", source):
        errors.append("retired cluster nodes endpoint")
    if "/api/v1/runtime/events" in source:
        errors.append("retired runtime v1 ingest endpoint")
    if re.search(r"/(?:api/v1/)?graph/(?:" + "|".join(RETIRED_GRAPH) + r")(?:[/:\s\"'`]|$)", source):
        errors.append("retired graph endpoint")
    if path.startswith("core/internal/api/"):
        for name in RETIRED_HANDLERS:
            if re.search(r"\b" + name + r"\s*\(", source):
                errors.append("retired handler: " + name)
    return errors


def main():
    root = Path(__file__).resolve().parents[2]
    errors = []
    for directory in ("core/internal/api", "agent", "dashboard", "scripts/e2e", "deploy"):
        for path in (root / directory).rglob("*"):
            if not path.is_file() or path.suffix not in {".go", ".ts", ".tsx", ".js", ".sh", ".yaml", ".yml"}:
                continue
            if any(part in {"node_modules", "dist", "vendor"} for part in path.parts):
                continue
            # Negative route regression intentionally names removed routes.
            if path.name == "retired_routes_test.go":
                continue
            rel = path.relative_to(root).as_posix()
            errors.extend(f"{rel}: {error}" for error in violations(rel, path.read_text()))
    if errors:
        print("\n".join(errors))
        return 1
    print("Retired API route contract: PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
