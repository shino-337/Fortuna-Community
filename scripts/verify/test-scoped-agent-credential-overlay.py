#!/usr/bin/env python3
from pathlib import Path
import unittest

import yaml

REPO = Path(__file__).resolve().parents[2]
CORE_PATCH = REPO / "deploy" / "scoped-agent-credentials" / "core-registry-patch.yaml"
AGENT_PATCH = REPO / "deploy" / "scoped-agent-credentials" / "agent-token-file-patch.yaml"
AGENT_BASE = REPO / "deploy" / "fortuna-agent-daemonset.yaml"


def load_one(path: Path):
    docs = [doc for doc in yaml.safe_load_all(path.read_text(encoding="utf-8")) if doc]
    if len(docs) != 1:
        raise AssertionError(f"{path} must contain exactly one YAML document")
    return docs[0]


def by_field(items, field, value):
    for item in items or []:
        if item.get(field) == value:
            return item
    raise AssertionError(f"missing item with {field}={value!r}")


def named(items, name):
    return by_field(items, "name", name)


class ScopedAgentCredentialOverlayTest(unittest.TestCase):
    def test_core_patch_mounts_digest_only_registry_and_switches_mode(self):
        doc = load_one(CORE_PATCH)
        self.assertEqual(doc["kind"], "Deployment")
        self.assertEqual(doc["metadata"]["name"], "fortuna-core")
        pod_spec = doc["spec"]["template"]["spec"]
        volume = named(pod_spec["volumes"], "agent-credential-registry")
        self.assertEqual(volume["secret"]["secretName"], "fortuna-agent-credential-registry")
        item = by_field(volume["secret"]["items"], "key", "registry.json")
        self.assertEqual(item["path"], "registry.json")

        core = named(pod_spec["containers"], "core")
        env = named(core["env"], "FORTUNA_AGENT_CREDENTIAL_REGISTRY")
        self.assertEqual(env["value"], "/etc/fortuna/agent-credentials/registry.json")
        mount = named(core["volumeMounts"], "agent-credential-registry")
        self.assertTrue(mount["readOnly"])

    def test_agent_patch_mounts_one_node_local_token_and_preserves_legacy_base_mode(self):
        doc = load_one(AGENT_PATCH)
        self.assertEqual(doc["kind"], "DaemonSet")
        self.assertEqual(doc["metadata"]["name"], "fortuna-agent")
        pod_spec = doc["spec"]["template"]["spec"]
        volume = named(pod_spec["volumes"], "agent-http-credential")
        self.assertEqual(volume["hostPath"]["path"], "/etc/fortuna/agent-credentials/token")
        self.assertEqual(volume["hostPath"]["type"], "File")

        agent = named(pod_spec["containers"], "agent")
        env = named(agent["env"], "FORTUNA_AGENT_TOKEN_FILE")
        self.assertEqual(env["value"], "/etc/fortuna/agent-credentials/token")
        mount = named(agent["volumeMounts"], "agent-http-credential")
        self.assertTrue(mount["readOnly"])

        # The base DaemonSet intentionally keeps FORTUNA_INGEST_TOKEN so existing
        # installations remain on explicit legacy mode until operators opt into
        # both the Core registry and Agent token-file overlay. In scoped mode the
        # shared token is not used by Agent or runtime HTTP ingest routes.
        base_docs = [doc for doc in yaml.safe_load_all(AGENT_BASE.read_text(encoding="utf-8")) if doc]
        daemonset = next(doc for doc in base_docs if doc.get("kind") == "DaemonSet")
        base_agent = named(daemonset["spec"]["template"]["spec"]["containers"], "agent")
        shared = named(base_agent["env"], "FORTUNA_INGEST_TOKEN")
        self.assertEqual(shared["valueFrom"]["secretKeyRef"]["key"], "ingest-token")


if __name__ == "__main__":
    unittest.main()
