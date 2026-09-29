#!/usr/bin/env python3
import copy
import runpy
from pathlib import Path
import unittest

validate = runpy.run_path(str(Path(__file__).with_name("run-two-cluster-integration.py")))["validate_receipt"]

class LiveIntegrationGateTests(unittest.TestCase):
    def setUp(self):
        self.events = [{"Test": "TestTwoClusterDaemonSetLive", "Action": "pass"}]
        self.report = {key: True for key in ("passed", "agentInventory", "scopedHTTPJWT", "firstInvestigationRBAC", "firstFindingRule", "concurrentManualAck", "workerReconciliation", "replacementUIDProtected", "deletePersistenceRecovery", "agentRestart")}
        self.report.update(clusters=2, nodes=3, daemonSets=4, namespaceScopes=2, runtimeAutoResolutionEnabled=False, dashboardBrowserVerified=False)

    def test_complete_actual_receipt(self):
        validate(0, self.events, self.report)

    def test_removed_skipped_failed_selection_is_rejected(self):
        for events in ([], [{"Test": "Other", "Action": "pass"}], self.events + [{"Test": "TestTwoClusterDaemonSetLive", "Action": "skip"}], self.events + [{"Test": "TestTwoClusterDaemonSetLive", "Action": "fail"}]):
            with self.assertRaises(RuntimeError): validate(0, events, self.report)
        with self.assertRaises(RuntimeError): validate(1, self.events, self.report)

    def test_incomplete_or_overstated_evidence_is_rejected(self):
        for key in self.report:
            changed = copy.deepcopy(self.report)
            del changed[key]
            with self.subTest(key=key), self.assertRaises(RuntimeError): validate(0, self.events, changed)
        for key in ("dashboardBrowserVerified", "runtimeAutoResolutionEnabled"):
            changed = {**self.report, key: True}
            with self.assertRaises(RuntimeError): validate(0, self.events, changed)

if __name__ == "__main__": unittest.main()
