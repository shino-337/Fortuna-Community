#!/usr/bin/env python3
"""Fail when persisted workload-owned models lose cluster-qualified identity.

This is a ratchet, not proof that every query is already cluster-qualified. C3c
query migration expands the gate to read/write paths. At the model boundary, any
persisted struct carrying PodUID must also carry ClusterID, and every such model
must remain represented in the startup cluster-resource migration manifest.
Resource-typed security models that can refer to Pods are mapped explicitly too.
"""
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[2]
MODELS = ROOT / "core" / "pkg" / "models"
FOUNDATION = ROOT / "core" / "migrations" / "cluster_resource_identity_foundation.go"

STRUCT_RE = re.compile(r"type\s+(\w+)\s+struct\s*\{(.*?)\n\}", re.S)
FIELD_RE = lambda name: re.compile(rf"\b{name}\s+string\b")
TARGET_RE = re.compile(r'\{table:\s*"([^"]+)"\s*,\s*uidColumn:\s*"([^"]+)"')

EXPLICIT_RESOURCE_MODELS = {
    "Insight",
    "PolicyViolation",
    "RiskScore",
    "SBOM",
    "CVEMatch",
}

# Explicit mapping is intentional. A new persisted PodUID model must be classified
# here and in the migration manifest instead of silently inheriting a guessed GORM
# table name. This turns model/schema drift into a CI failure.
POD_MODEL_TABLES = {
    "AssetSecurityState": "asset_security_state",
    "AttackPath": "attack_paths",
    "CVEMatch": "cve_matches",
    "EventIndex": "events_index",
    "MalwareMatch": "malware_matches",
    "PodAttackStep": "pod_attack_steps",
    "PodCapability": "pod_capabilities",
    "PodImageScan": "pod_image_scans",
    "PodInstance": "pod_instances",
    "PodNetworkConnection": "pod_network_connections",
    "PodProcess": "pod_processes",
    "PodRiskProfile": "pod_risk_profiles",
    "PodRuntimeMetrics": "pod_runtime_metrics",
    "RuntimeBehaviorFact": "runtime_behavior_facts",
    "RuntimeEvent": "runtime_events",
    "RuntimeIncident": "runtime_incidents",
    "RuntimeSignal": "runtime_signals",
    "SBOM": "sboms",
}

# These models use ResourceUID rather than PodUID and carry ResourceType, so the
# migration applies the Pod ownership invariant only to resource_type=pod rows.
SPECIAL_FOUNDATION_MODELS = {
    "Insight": ("insights", "resource_uid"),
    "PolicyViolation": ("policy_violations", "resource_uid"),
    "RiskScore": ("risk_scores", "resource_uid"),
}


def migration_targets() -> dict[str, str]:
    text = FOUNDATION.read_text(encoding="utf-8")
    targets: dict[str, str] = {}
    for table, uid_column in TARGET_RE.findall(text):
        if table in targets:
            raise ValueError(f"duplicate cluster-resource migration target: {table}")
        targets[table] = uid_column
    return targets


def main() -> int:
    errors = []
    seen = set()
    persisted_pod_models = set()

    try:
        targets = migration_targets()
    except (OSError, ValueError) as exc:
        print(str(exc), file=sys.stderr)
        return 1

    for path in sorted(MODELS.glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        text = path.read_text(encoding="utf-8")
        for match in STRUCT_RE.finditer(text):
            name, body = match.group(1), match.group(2)
            seen.add(name)
            has_cluster = bool(FIELD_RE("ClusterID").search(body))
            persisted_pod_uid = bool(FIELD_RE("PodUID").search(body)) and "gorm:" in body
            if persisted_pod_uid:
                persisted_pod_models.add(name)
                if not has_cluster:
                    errors.append(f"{path.relative_to(ROOT)}:{name}: persisted PodUID requires ClusterID")
                table = POD_MODEL_TABLES.get(name)
                if table is None:
                    errors.append(
                        f"{path.relative_to(ROOT)}:{name}: persisted PodUID model is not mapped to a cluster-resource migration target"
                    )
                elif targets.get(table) != "pod_uid":
                    errors.append(
                        f"{path.relative_to(ROOT)}:{name}: migration target {table} must use uidColumn pod_uid"
                    )
            if name in EXPLICIT_RESOURCE_MODELS and not has_cluster:
                errors.append(f"{path.relative_to(ROOT)}:{name}: security-critical resource model requires ClusterID")

    missing = sorted(EXPLICIT_RESOURCE_MODELS - seen)
    errors.extend(f"required resource model not found: {name}" for name in missing)

    stale_model_mappings = sorted(set(POD_MODEL_TABLES) - persisted_pod_models)
    errors.extend(
        f"cluster-resource model mapping no longer matches a persisted PodUID model: {name}"
        for name in stale_model_mappings
    )

    expected_targets = {table: "pod_uid" for table in POD_MODEL_TABLES.values()}
    expected_targets.update(SPECIAL_FOUNDATION_MODELS.values())

    for name, (table, uid_column) in SPECIAL_FOUNDATION_MODELS.items():
        if name not in seen:
            errors.append(f"required foundation resource model not found: {name}")
        if targets.get(table) != uid_column:
            errors.append(
                f"{name}: migration target {table} must use uidColumn {uid_column}"
            )

    for table, uid_column in sorted(expected_targets.items()):
        if targets.get(table) != uid_column:
            errors.append(
                f"cluster-resource migration manifest missing required target {table}:{uid_column}"
            )

    unexpected_targets = sorted(set(targets) - set(expected_targets))
    errors.extend(
        f"cluster-resource migration target has no model contract mapping: {table}"
        for table in unexpected_targets
    )

    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    print("PASS cluster-qualified workload model and migration coverage contract")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
