#!/usr/bin/env python3
"""Fail when the Agent regains a privilege it must not hold.

The Agent runs on every node, so its ServiceAccount and pod spec are reachable by
anyone who compromises one node. This test pins what it may hold:

- RBAC: read-only verbs, and none of the subresources that run code in pods
  (pods/exec, pods/attach, pods/portforward, nodes/proxy and friends) or read
  credentials (secrets, serviceaccounts/token).
- Agent Go code: no remotecommand client and no exec/attach/proxy subresource calls.
- DaemonSet: no privileged mode, host network or host IPC; capability additions
  limited to the current eBPF set; hostPath volumes limited to a known list.
"""
from pathlib import Path
import re
import unittest

import yaml

REPO = Path(__file__).resolve().parents[2]
DEPLOY = REPO / "deploy"
AGENT_SRC = REPO / "agent"
AGENT_SA = "fortuna-agent"

READ_VERBS = {"get", "list", "watch"}
FORBIDDEN_RESOURCES = {
    "pods/exec",
    "pods/attach",
    "pods/portforward",
    "pods/proxy",
    "pods/ephemeralcontainers",
    "nodes/proxy",
    "services/proxy",
    "secrets",
    "serviceaccounts/token",
}
# The eBPF sensor's capabilities. Shrinking this set is tracked follow-up work;
# growing it needs a deliberate change here.
ALLOWED_CAPABILITY_ADDS = {"SYS_BPF", "SYS_RESOURCE", "PERFMON", "SYS_ADMIN"}
# hostPath -> whether the Agent may mount it writable.
ALLOWED_HOST_PATHS = {
    "/proc": False,
    "/run/containerd/containerd.sock": False,
    "/var/log/falco": False,
    "/var/lib/fortuna-agent": True,
    # Per-node credentials from deploy/scoped-agent-credentials/.
    "/etc/fortuna/agent-mtls": False,
    "/etc/fortuna/agent-credentials/token": False,
}
FORBIDDEN_GO = [
    re.compile(r'"k8s\.io/client-go/tools/remotecommand"'),
    re.compile(r'SubResource\(\s*"(exec|attach|portforward|proxy)"'),
    re.compile(r'/proxy/'),
]


def load_docs():
    for path in sorted(DEPLOY.rglob("*.yaml")):
        for doc in yaml.safe_load_all(path.read_text(encoding="utf-8")):
            if isinstance(doc, dict):
                yield path, doc


def agent_role_names(docs):
    """Roles and ClusterRoles bound to the Agent ServiceAccount."""
    names = set()
    for _, doc in docs:
        if doc.get("kind") not in ("RoleBinding", "ClusterRoleBinding"):
            continue
        for subject in doc.get("subjects") or []:
            if subject.get("kind") == "ServiceAccount" and subject.get("name") == AGENT_SA:
                ref = doc.get("roleRef") or {}
                names.add((ref.get("kind"), ref.get("name")))
    return names


def rule_violations(rule):
    problems = []
    verbs = set(rule.get("verbs") or [])
    resources = set(rule.get("resources") or [])
    groups = set(rule.get("apiGroups") or [])
    if "*" in verbs or "*" in resources or "*" in groups:
        problems.append(f"wildcard in rule {rule}")
    extra = verbs - READ_VERBS
    if extra:
        problems.append(f"non-read verbs {sorted(extra)} on {sorted(resources)}")
    bad = resources & FORBIDDEN_RESOURCES
    if bad:
        problems.append(f"forbidden resources {sorted(bad)}")
    return problems


def pod_spec_violations(pod_spec):
    problems = []
    for key in ("hostNetwork", "hostIPC"):
        if pod_spec.get(key) is True:
            problems.append(f"{key}: true")
    host_paths = {}
    for volume in pod_spec.get("volumes") or []:
        hp = volume.get("hostPath")
        if hp is None:
            continue
        path = hp.get("path")
        if path not in ALLOWED_HOST_PATHS:
            problems.append(f"hostPath {path!r} is not on the allowed list")
        host_paths[volume.get("name")] = path
    for container in (pod_spec.get("containers") or []) + (pod_spec.get("initContainers") or []):
        sc = container.get("securityContext") or {}
        if sc.get("privileged") is True:
            problems.append(f"container {container.get('name')} is privileged")
        caps = sc.get("capabilities") or {}
        added = set(caps.get("add") or [])
        if added - ALLOWED_CAPABILITY_ADDS:
            problems.append(f"container {container.get('name')} adds {sorted(added - ALLOWED_CAPABILITY_ADDS)}")
        if added and "ALL" not in (caps.get("drop") or []):
            problems.append(f"container {container.get('name')} adds capabilities without dropping ALL")
        for mount in container.get("volumeMounts") or []:
            path = host_paths.get(mount.get("name"))
            if path in ALLOWED_HOST_PATHS and not ALLOWED_HOST_PATHS[path] and mount.get("readOnly") is not True:
                problems.append(f"hostPath {path!r} must be mounted readOnly")
    return problems


def agent_pod_specs(docs):
    for path, doc in docs:
        if doc.get("kind") == "DaemonSet" and (doc.get("metadata") or {}).get("name") == AGENT_SA:
            yield path, ((doc.get("spec") or {}).get("template") or {}).get("spec") or {}


class AgentPrivilegeTest(unittest.TestCase):
    def setUp(self):
        self.docs = list(load_docs())

    def test_agent_rbac_is_read_only_and_cannot_reach_pods(self):
        roles = agent_role_names(self.docs)
        self.assertIn(("ClusterRole", AGENT_SA), roles)
        found = set()
        for path, doc in self.docs:
            key = (doc.get("kind"), (doc.get("metadata") or {}).get("name"))
            if key not in roles:
                continue
            found.add(key)
            for rule in doc.get("rules") or []:
                for problem in rule_violations(rule):
                    self.fail(f"{path.relative_to(REPO)} {key[0]}/{key[1]}: {problem}")
        self.assertEqual(found, roles, "every role bound to the Agent must be defined under deploy/")

    def test_agent_pod_spec_stays_within_allowed_host_access(self):
        specs = list(agent_pod_specs(self.docs))
        self.assertTrue(specs, "fortuna-agent DaemonSet not found under deploy/")
        for path, spec in specs:
            for problem in pod_spec_violations(spec):
                self.fail(f"{path.relative_to(REPO)}: {problem}")

    def test_agent_code_never_execs_or_proxies_into_pods(self):
        for path in sorted(AGENT_SRC.rglob("*.go")):
            rel = path.relative_to(REPO)
            if "third_party" in rel.parts or path.name.endswith("_test.go"):
                continue
            text = path.read_text(encoding="utf-8")
            for pattern in FORBIDDEN_GO:
                match = pattern.search(text)
                if match:
                    self.fail(f"{rel}: forbidden pattern {match.group(0)!r}")

    def test_checks_reject_known_bad_grants(self):
        self.assertTrue(rule_violations({"apiGroups": [""], "resources": ["nodes/proxy"], "verbs": ["get"]}))
        self.assertTrue(rule_violations({"apiGroups": [""], "resources": ["pods/exec"], "verbs": ["create"]}))
        self.assertTrue(rule_violations({"apiGroups": [""], "resources": ["secrets"], "verbs": ["list"]}))
        self.assertTrue(rule_violations({"apiGroups": ["*"], "resources": ["pods"], "verbs": ["get"]}))
        self.assertTrue(rule_violations({"apiGroups": ["rbac.authorization.k8s.io"], "resources": ["clusterroles"], "verbs": ["bind"]}))
        self.assertFalse(rule_violations({"apiGroups": ["metrics.k8s.io"], "resources": ["pods"], "verbs": ["get", "list"]}))
        bad_spec = {
            "hostNetwork": True,
            "volumes": [{"name": "root", "hostPath": {"path": "/"}}, {"name": "proc", "hostPath": {"path": "/proc"}}],
            "containers": [{
                "name": "agent",
                "securityContext": {"privileged": True, "capabilities": {"add": ["SYS_PTRACE"]}},
                "volumeMounts": [{"name": "proc", "mountPath": "/host/proc"}],
            }],
        }
        self.assertEqual(len(pod_spec_violations(bad_spec)), 6)


if __name__ == "__main__":
    unittest.main()
