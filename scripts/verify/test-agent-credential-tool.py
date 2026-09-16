#!/usr/bin/env python3
import hashlib
import json
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest

REPO = Path(__file__).resolve().parents[2]
TOOL = REPO / "scripts" / "deploy" / "agent-credential-tool.py"


class AgentCredentialToolTest(unittest.TestCase):
    def run_tool(self, *args, expect=0):
        result = subprocess.run(
            [sys.executable, str(TOOL), *args],
            cwd=REPO,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, expect, msg=result.stderr + result.stdout)
        return result

    def test_issue_rotation_and_revoke(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            first = root / "first"
            result = self.run_tool(
                "issue",
                "--cluster-id", "cluster-a",
                "--node", "node-a",
                "--node", "node-b",
                "--output-dir", str(first),
                "--ttl-hours", "24",
            )
            summary = json.loads(result.stdout)
            registry_path = Path(summary["registry"])
            registry_text = registry_path.read_text(encoding="utf-8")
            registry = json.loads(registry_text)
            self.assertEqual(len(registry["credentials"]), 2)

            by_agent = {entry["agent_id"]: entry for entry in registry["credentials"]}
            for node in ("node-a", "node-b"):
                token_path = first / "nodes" / node / "token"
                token = token_path.read_text(encoding="utf-8").strip()
                self.assertGreaterEqual(len(token), 32)
                self.assertNotIn(token, registry_text)
                self.assertEqual(stat.S_IMODE(token_path.stat().st_mode), 0o600)
                digest = hashlib.sha256(token.encode("utf-8")).hexdigest()
                self.assertEqual(by_agent[f"{node}-agent"]["token_sha256"], digest)

            old_ids = {entry["id"] for entry in registry["credentials"]}
            second = root / "second"
            result2 = self.run_tool(
                "issue",
                "--cluster-id", "cluster-a",
                "--node", "node-a",
                "--output-dir", str(second),
                "--existing-registry", str(registry_path),
                "--ttl-hours", "24",
            )
            summary2 = json.loads(result2.stdout)
            registry2_path = Path(summary2["registry"])
            registry2 = json.loads(registry2_path.read_text(encoding="utf-8"))
            self.assertEqual(len(registry2["credentials"]), 3)
            self.assertTrue(old_ids.issubset({entry["id"] for entry in registry2["credentials"]}))
            new_id = summary2["issued"][0]["credential_id"]

            self.run_tool(
                "revoke",
                "--registry", str(registry2_path),
                "--credential-id", new_id,
            )
            revoked = json.loads(registry2_path.read_text(encoding="utf-8"))
            match = [entry for entry in revoked["credentials"] if entry["id"] == new_id]
            self.assertEqual(len(match), 1)
            self.assertTrue(match[0]["revoked"])

    def test_supports_explicit_agent_id_override(self):
        with tempfile.TemporaryDirectory() as td:
            out = Path(td) / "out"
            result = self.run_tool(
                "issue",
                "--cluster-id", "cluster-a",
                "--identity", "node-a=custom-agent-a",
                "--output-dir", str(out),
            )
            summary = json.loads(result.stdout)
            self.assertEqual(summary["issued"][0]["node"], "node-a")
            self.assertEqual(summary["issued"][0]["agent_id"], "custom-agent-a")
            registry = json.loads((out / "registry.json").read_text(encoding="utf-8"))
            self.assertEqual(registry["credentials"][0]["agent_id"], "custom-agent-a")

    def test_rejects_path_traversal_node_name(self):
        with tempfile.TemporaryDirectory() as td:
            out = Path(td) / "out"
            self.run_tool(
                "issue",
                "--cluster-id", "cluster-a",
                "--node", "../evil",
                "--output-dir", str(out),
                expect=2,
            )
            self.assertFalse((Path(td) / "evil").exists())

    def test_rejects_outer_whitespace_in_cluster_and_agent_identity(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            self.run_tool(
                "issue",
                "--cluster-id", " cluster-a",
                "--node", "node-a",
                "--output-dir", str(root / "cluster"),
                expect=2,
            )
            self.run_tool(
                "issue",
                "--cluster-id", "cluster-a",
                "--identity", "node-a= agent-a",
                "--output-dir", str(root / "agent"),
                expect=2,
            )

    def test_rejects_existing_registry_digest_not_accepted_by_core(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            bad_registry = root / "registry.json"
            bad_registry.write_text(json.dumps({
                "credentials": [{
                    "id": "old",
                    "cluster_id": "cluster-a",
                    "agent_id": "node-a-agent",
                    "token_sha256": "A" * 64,
                    "expires_at": "2099-01-01T00:00:00Z",
                }]
            }), encoding="utf-8")
            self.run_tool(
                "issue",
                "--cluster-id", "cluster-a",
                "--node", "node-a",
                "--existing-registry", str(bad_registry),
                "--output-dir", str(root / "out"),
                expect=2,
            )


if __name__ == "__main__":
    unittest.main()
