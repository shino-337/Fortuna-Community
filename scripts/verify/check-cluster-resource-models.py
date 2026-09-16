#!/usr/bin/env python3
"""Fail when persisted workload-owned models lose cluster-qualified identity.

This is a ratchet, not proof that every query is already cluster-qualified. C3c
query migration expands the gate to read/write paths. At the model boundary, any
persisted struct carrying PodUID must also carry ClusterID.
"""
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[2]
MODELS = ROOT / "core" / "pkg" / "models"

STRUCT_RE = re.compile(r"type\s+(\w+)\s+struct\s*\{(.*?)\n\}", re.S)
FIELD_RE = lambda name: re.compile(rf"\b{name}\s+string\b")

EXPLICIT_RESOURCE_MODELS = {
    "Insight",
    "RiskScore",
    "SBOM",
    "CVEMatch",
}


def main() -> int:
    errors = []
    seen = set()
    for path in sorted(MODELS.glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        text = path.read_text(encoding="utf-8")
        for match in STRUCT_RE.finditer(text):
            name, body = match.group(1), match.group(2)
            seen.add(name)
            has_cluster = bool(FIELD_RE("ClusterID").search(body))
            persisted_pod_uid = bool(FIELD_RE("PodUID").search(body)) and "gorm:" in body
            if persisted_pod_uid and not has_cluster:
                errors.append(f"{path.relative_to(ROOT)}:{name}: persisted PodUID requires ClusterID")
            if name in EXPLICIT_RESOURCE_MODELS and not has_cluster:
                errors.append(f"{path.relative_to(ROOT)}:{name}: security-critical resource model requires ClusterID")

    missing = sorted(EXPLICIT_RESOURCE_MODELS - seen)
    errors.extend(f"required resource model not found: {name}" for name in missing)

    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    print("PASS cluster-qualified workload model contract")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
