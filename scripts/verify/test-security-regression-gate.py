#!/usr/bin/env python3
"""Guard the CI parser against false success, including skipped/missing subtests."""
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("gate", Path(__file__).with_name("check-security-regressions.py"))
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)

class GateTest(unittest.TestCase):
    def test_requires_execution_and_pass(self):
        self.assertTrue(gate.validate_events([], ["TestRequired"]))
        self.assertTrue(gate.validate_events([{"Action":"pass","Test":"TestRequired"}], ["TestRequired"]))
        self.assertFalse(gate.validate_events([{"Action":"run","Test":"TestRequired"},{"Action":"pass","Test":"TestRequired"}], ["TestRequired"]))

    def test_rejects_skipped_subtests(self):
        events = [{"Action":"run","Test":"TestRequired"},{"Action":"skip","Test":"TestRequired/critical-case"},{"Action":"pass","Test":"TestRequired"}]
        self.assertTrue(gate.validate_events(events, ["TestRequired", "TestRequired/critical-case"]))

    def test_rejects_deleted_required_subtest(self):
        events = [{"Action":"run","Test":"TestRequired"},{"Action":"pass","Test":"TestRequired"}]
        problems = gate.validate_events(events, ["TestRequired", "TestRequired/critical-case"])
        self.assertTrue(any("critical-case" in problem for problem in problems))

    def test_rejects_package_failure(self):
        events = [{"Action":"run","Test":"TestRequired"},{"Action":"pass","Test":"TestRequired"},{"Action":"fail","Package":"example"}]
        self.assertTrue(gate.validate_events(events, ["TestRequired"]))

    def test_run_pattern_uses_parent_for_subtests(self):
        pattern = gate.run_pattern(["TestRequired", "TestRequired/critical-case", "TestOther"])
        self.assertIn("TestRequired", pattern)
        self.assertIn("TestOther", pattern)
        self.assertNotIn("critical\\-case", pattern)

if __name__ == "__main__":
    unittest.main()
