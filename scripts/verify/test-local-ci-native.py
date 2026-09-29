#!/usr/bin/env python3
import importlib.util
import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("local_ci", ROOT / "scripts/verify/run-local-ci-native.py")
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


class LocalCIGateTests(unittest.TestCase):
    def test_workflow_groups_and_matrix_remain_complete(self):
        jobs = ci.yaml.safe_load((ROOT / ".github/workflows/ci.yml").read_text())["jobs"]
        self.assertEqual(set(jobs), set(ci.GROUPS))
        self.assertEqual(jobs["go-test"]["strategy"]["matrix"]["module"], ["core", "agent", "api"])
        for group, job in jobs.items():
            for module in (["core", "agent", "api"] if group == "go-test" else [None]):
                self.assertGreater(len(list(ci.step_runs(job, module))), 0)

    def test_core_only_steps_and_working_directory_resolution(self):
        job = {"steps": [{"run": "go test ./...", "working-directory": "${{ matrix.module }}"}, {"run": "security", "if": "matrix.module == 'core'"}]}
        self.assertEqual(len(list(ci.step_runs(job, "core"))), 2)
        agent = list(ci.step_runs(job, "agent"))
        self.assertEqual(len(agent), 1)
        self.assertEqual(agent[0][2], "agent")

    def test_unknown_condition_action_and_expression_fail_closed(self):
        for step in [{"run": "true", "if": "unknown"}, {"uses": "unknown/action@v1"}, {"run": "${{ secrets.TOKEN }}"}, {"uses": "actions/checkout@v6", "if": "unknown"}, {"run": "true", "shell": "pwsh"}, {"run": "true", "env": {"TOKEN": "${{ secrets.TOKEN }}"}}]:
            with self.assertRaises(ValueError): list(ci.step_runs({"steps": [step]}))

    def test_unknown_workflow_execution_controls_fail_closed(self):
        workflow = ci.yaml.safe_load((ROOT / ".github/workflows/ci.yml").read_text())
        ci.validate_workflow(workflow)
        for path, key, value in [((), "env", {"GOFLAGS": "-run=^$"}), (("jobs", "dashboard"), "strategy", {"matrix": {"browser": ["chromium", "firefox"]}}), (("jobs", "scripts"), "if", "false")]:
            changed = copy.deepcopy(workflow)
            target = changed
            for part in path: target = target[part]
            target[key] = value
            with self.assertRaises(ValueError): ci.validate_workflow(changed)

    def test_inherited_test_selection_and_live_database_are_not_used(self):
        with tempfile.TemporaryDirectory() as folder, mock.patch.dict(ci.os.environ, {"GOFLAGS": "-run=^$", "FORTUNA_TEST_POSTGRES_URL": "postgres://live-db"}):
            runner = ci.Runner(ROOT, "all", Path(folder))
            self.assertEqual(runner.env["GOFLAGS"], "-p=1")
            self.assertNotIn("FORTUNA_TEST_POSTGRES_URL", runner.env)

    def test_failure_and_source_change_write_unpublishable_reports(self):
        for failed in (True, False):
            with self.subTest(failed=failed), tempfile.TemporaryDirectory() as folder:
                runner = ci.Runner(ROOT, "hygiene", Path(folder))
                def run_group(group):
                    runner.results.append({"job": group, "passed": not failed, "log": "hygiene.log"})
                    (runner.output / "hygiene.log").write_text("failure\n" if failed else "passed\n")
                    if failed: raise RuntimeError("intentional fixture failure")
                finished = runner.source if failed else {**runner.source, "fingerprint": "changed"}
                with mock.patch.object(runner, "run_group", side_effect=run_group), mock.patch.object(ci, "source_identity", return_value=finished), self.assertRaises(RuntimeError):
                    runner.run()
                report = json.loads((runner.output / "results.json").read_text())
                self.assertFalse(report["passed"])
                self.assertFalse(report["publishable"])
                self.assertEqual(report["jobs"][0]["sha256"], hashlib.sha256((runner.output / "hygiene.log").read_bytes()).hexdigest())

    def test_only_all_clean_stable_success_is_publishable(self):
        clean = {"sha": "head", "dirty": False, "fingerprint": "content"}
        self.assertTrue(ci.publishable("all", clean, clean, True))
        self.assertFalse(ci.publishable("all", clean, clean, False))
        self.assertFalse(ci.publishable("go-test", clean, clean, True))
        dirty = {**clean, "dirty": True}
        self.assertFalse(ci.publishable("all", dirty, dirty, True))
        self.assertFalse(ci.publishable("all", clean, {**clean, "fingerprint": "changed"}, True))


if __name__ == "__main__": unittest.main()
