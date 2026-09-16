#!/usr/bin/env python3
"""Run named security regressions; fail on removal, renaming, skip or failure.

Run from any directory. Requires Go on PATH. CI uses this in addition to go test
./...; an empty -run selection must never silently satisfy this gate. Critical
subtests are listed explicitly so deleting one case cannot hide behind a passing
parent test.
"""
import json
from pathlib import Path
import re
import subprocess
import sys

REQUIRED = {
    "./pkg/riskengine": [
        "TestRuntimeInputFailureReachesEvaluators",
        "TestRuntimeInputRejectsCorruptSnapshot",
        "TestRuntimeInputRejectsMalformedBindings",
        "TestClusterAdminBindingForPod",
        "TestClusterAdminBindingForPod/namespace-only",
        "TestClusterAdminBindingForPod/foreign",
        "TestClusterAdminBindingForPod/group",
        "TestClusterAdminBindingForPod/wrong-role-kind",
        "TestEvaluationReportsRuleFailure",
        "TestConfiguredCatalogRejectsPartialAndEmptyLoad",
    ],
    "./pkg/worker": [
        "TestReconciliationPreservesFindingOnRuntimeInputFailure",
        "TestReconciliationAuditRollbackAndCatalogFailure",
        "TestReconciliationPreservesDisabledDetector",
    ],
    "./internal/api": [
        "TestServiceAccountRBACResolution",
        "TestPodRiskReportUsesResolvedRBACScope",
        "TestServiceAccountInventoryScope",
        "TestServiceAccountMutationsProtectIdentity",
        "TestServiceAccountBulkFailuresPreserveInventory",
        "TestInventoryWorkloadCapabilityScope",
        "TestAgentClusterIdentityIsolation",
        "TestLegacyGraphFailsBeforeGlobalQuery",
        "TestNetworkServiceCacheSeparatesClustersAndCredentials",
        "TestAggregateCacheIsolation",
        "TestRuntimeScopeAndFindingActions",
        "TestBulkRequiresActionPermissionAndNonemptySelection",
    ],
    "./pkg/agentidentity": [
        "TestCredentialIdentityIsolation",
        "TestCredentialRotationRevocationAndExpiry",
        "TestCredentialRegistryFailsClosed",
        "TestCredentialRequiresVerifiedTLS",
    ],
}


def validate_events(events, required):
    ran, passed = set(), set()
    problems = []
    for event in events:
        name, action = event.get("Test"), event.get("Action")
        if action == "run":
            ran.add(name)
        elif action == "pass":
            passed.add(name)
        elif action in ("skip", "fail"):
            problems.append(f"{action}: {name or event.get('Package', 'package')}")
    for name in required:
        if name not in ran or name not in passed:
            problems.append(f"required test did not run and pass: {name}")
    return problems


def run_pattern(names):
    # Go's -run matches slash-separated test/subtest components independently.
    # Prefixing the escaped name and allowing an optional descendant suffix runs
    # the required parent plus explicitly named subtests without silently
    # broadening to unrelated tests.
    roots = sorted({name.split("/", 1)[0] for name in names})
    return "^(" + "|".join(re.escape(name) for name in roots) + ")$"


def main():
    core = Path(__file__).resolve().parents[2] / "core"
    errors = []
    for package, names in REQUIRED.items():
        result = subprocess.run(
            ["go", "test", "-json", "-count=1", "-timeout=2m", "-run", run_pattern(names), package],
            cwd=core, capture_output=True, text=True, check=False,
        )
        events = []
        for line in result.stdout.splitlines():
            try:
                events.append(json.loads(line))
            except json.JSONDecodeError:
                errors.append(f"{package}: unexpected non-JSON test output")
        problems = validate_events(events, names)
        if result.returncode:
            problems.append(f"go test exited {result.returncode}")
        if problems:
            errors.extend(f"{package}: {p}" for p in problems)
            print(result.stderr, file=sys.stderr)
            for event in events:
                if event.get("Action") == "output":
                    print(event.get("Output", ""), end="", file=sys.stderr)
        else:
            print(f"PASS {package}: {len(names)} required regression tests/subtests")
    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
