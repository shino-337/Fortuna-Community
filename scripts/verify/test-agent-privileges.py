#!/usr/bin/env python3
"""Fail when the Agent regains a privilege it must not hold.

The Agent runs on every node, so its ServiceAccount and pod spec are reachable by
anyone who compromises one node. This test pins what it may hold:

- RBAC: read-only verbs, and none of the subresources that run code in pods
  (pods/exec, pods/attach, pods/portforward, nodes/proxy and friends) or read
  credentials (secrets, serviceaccounts/token).
  Every (apiGroup, resource) must also be on ALLOWED_RBAC.
- Agent Go code: no remotecommand client and no exec/attach/proxy subresource calls.
- DaemonSet: no privileged mode, host namespaces or added capabilities; hostPath
  volumes limited to a known list. The base manifest must also drop ALL
  capabilities, forbid privilege escalation, use a read-only root filesystem and
  the RuntimeDefault seccomp profile, and project the ServiceAccount token into
  the agent container only.
- The containerd socket is mounted only into the image-export container, which
  must hold no token, no secret and no other mount.

Adding a privilege means changing this file in review, together with the
"Agent privileges" section of docs/reference/SECURITY.md.
"""
import copy
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
ALLOWED_RBAC = {
    ("", "pods"), ("", "events"), ("", "nodes"), ("", "namespaces"), ("", "serviceaccounts"),
    ("rbac.authorization.k8s.io", "roles"), ("rbac.authorization.k8s.io", "rolebindings"),
    ("rbac.authorization.k8s.io", "clusterroles"), ("rbac.authorization.k8s.io", "clusterrolebindings"),
    ("apps", "deployments"), ("apps", "replicasets"),
    ("metrics.k8s.io", "pods"),
}
BASE_DAEMONSET = DEPLOY / "fortuna-agent-daemonset.yaml"
CONTAINERD_SOCKET = "/run/containerd/containerd.sock"
SOCKET_CONTAINER = "image-export"
SOCKET_CONTAINER_VOLUMES = {"containerd-socket", "image-export"}
# hostPath -> whether the Agent may mount it writable.
ALLOWED_HOST_PATHS = {
    "/proc": False,
    CONTAINERD_SOCKET: False,  # image-export only
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
    if rule.get("nonResourceURLs"):
        problems.append(f"nonResourceURLs {rule['nonResourceURLs']}")
    unlisted = sorted(f"{g or 'core'}/{r}" for g in groups for r in resources
                      if (g, r) not in ALLOWED_RBAC and r not in FORBIDDEN_RESOURCES and "*" not in (g, r))
    if unlisted:
        problems.append(f"resources not on ALLOWED_RBAC {unlisted}")
    return problems


def pod_spec_violations(pod_spec, strict=False):
    """strict: the base manifest must set the safe values; overlays must not undo them."""
    problems = []
    for key in ("hostNetwork", "hostIPC", "hostPID"):
        if pod_spec.get(key) is True:
            problems.append(f"{key}: true")
    if strict and ((pod_spec.get("securityContext") or {}).get("seccompProfile") or {}).get("type") != "RuntimeDefault":
        problems.append("pod seccompProfile must be RuntimeDefault")
    if strict and pod_spec.get("automountServiceAccountToken") is not False:
        problems.append("automountServiceAccountToken must be false; project the token into the agent container only")
    host_paths = {}
    token_volumes = set()
    for volume in pod_spec.get("volumes") or []:
        hp = volume.get("hostPath")
        sources = (volume.get("projected") or {}).get("sources") or []
        if "secret" in volume or any("serviceAccountToken" in src or "secret" in src for src in sources):
            token_volumes.add(volume.get("name"))
        if hp is None:
            continue
        path = hp.get("path")
        if path not in ALLOWED_HOST_PATHS:
            problems.append(f"hostPath {path!r} is not on the allowed list")
        host_paths[volume.get("name")] = path
    for container in (pod_spec.get("containers") or []) + (pod_spec.get("initContainers") or []):
        name = container.get("name")
        sc = container.get("securityContext") or {}
        caps = sc.get("capabilities") or {}
        if sc.get("privileged") is True:
            problems.append(f"container {name} is privileged")
        if caps.get("add"):
            problems.append(f"container {name} adds capabilities {sorted(caps['add'])}")
        if strict and "ALL" not in (caps.get("drop") or []):
            problems.append(f"container {name} must drop ALL capabilities")
        if sc.get("allowPrivilegeEscalation") is True or (strict and sc.get("allowPrivilegeEscalation") is not False):
            problems.append(f"container {name} must set allowPrivilegeEscalation: false")
        if sc.get("readOnlyRootFilesystem") is False or (strict and sc.get("readOnlyRootFilesystem") is not True):
            problems.append(f"container {name} must set readOnlyRootFilesystem: true")
        if (sc.get("seccompProfile") or {}).get("type") == "Unconfined":
            problems.append(f"container {name} runs seccomp Unconfined")
        mounts = {m.get("name") for m in container.get("volumeMounts") or []}
        for mount in container.get("volumeMounts") or []:
            path = host_paths.get(mount.get("name"))
            if path in ALLOWED_HOST_PATHS and not ALLOWED_HOST_PATHS[path] and mount.get("readOnly") is not True:
                problems.append(f"hostPath {path!r} must be mounted readOnly")
            if path == CONTAINERD_SOCKET and name != SOCKET_CONTAINER:
                problems.append(f"only {SOCKET_CONTAINER} may mount the containerd socket, not {name}")
        if name == SOCKET_CONTAINER:
            if mounts - SOCKET_CONTAINER_VOLUMES or mounts & token_volumes:
                problems.append(f"{name} may mount only {sorted(SOCKET_CONTAINER_VOLUMES)}, found {sorted(mounts)}")
            if container.get("envFrom") or any("valueFrom" in e for e in container.get("env") or []):
                problems.append(f"{name} must not receive secrets or other env from references")
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
        self.assertIn(BASE_DAEMONSET, [path for path, _ in specs], "fortuna-agent DaemonSet not found under deploy/")
        for path, spec in specs:
            for problem in pod_spec_violations(spec, strict=path == BASE_DAEMONSET):
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
        self.assertTrue(rule_violations({"apiGroups": [""], "resources": ["configmaps"], "verbs": ["get"]}))
        self.assertTrue(rule_violations({"nonResourceURLs": ["/metrics"], "verbs": ["get"]}))

    def test_checks_reject_each_pod_escalation(self):
        base = next(spec for path, spec in agent_pod_specs(self.docs) if path == BASE_DAEMONSET)
        self.assertEqual(pod_spec_violations(base, strict=True), [])
        agent, helper = (lambda s: s["containers"][0]), (lambda s: s["containers"][1])
        mount = lambda volume, path: {"name": volume, "mountPath": path, "readOnly": True}
        cases = {
            "hostPID": lambda s: s.update(hostPID=True),
            "hostNetwork": lambda s: s.update(hostNetwork=True),
            "no seccomp": lambda s: s.pop("securityContext"),
            "token automount": lambda s: s.update(automountServiceAccountToken=True),
            "host root": lambda s: s["volumes"].append({"name": "root", "hostPath": {"path": "/"}}),
            "writable /proc": lambda s: [m.pop("readOnly") for m in agent(s)["volumeMounts"] if m["name"] == "host-proc"],
            "privileged": lambda s: agent(s)["securityContext"].update(privileged=True),
            "SYS_ADMIN": lambda s: agent(s)["securityContext"]["capabilities"].update(add=["SYS_ADMIN"]),
            "escalation": lambda s: agent(s)["securityContext"].update(allowPrivilegeEscalation=True),
            "writable rootfs": lambda s: agent(s)["securityContext"].update(readOnlyRootFilesystem=False),
            "no drop ALL": lambda s: agent(s)["securityContext"]["capabilities"].update(drop=[]),
            "socket in agent": lambda s: agent(s)["volumeMounts"].append(mount("containerd-socket", CONTAINERD_SOCKET)),
            "token in helper": lambda s: helper(s)["volumeMounts"].append(mount("kube-api-access", "/var/run/secrets/kubernetes.io/serviceaccount")),
            "host path in helper": lambda s: helper(s)["volumeMounts"].append(mount("host-proc", "/host/proc")),
            "secret env in helper": lambda s: helper(s)["env"].append(
                {"name": "T", "valueFrom": {"secretKeyRef": {"name": "fortuna-secrets", "key": "ingest-token"}}}),
        }
        for name, change in cases.items():
            spec = copy.deepcopy(base)
            change(spec)
            with self.subTest(name):
                self.assertTrue(pod_spec_violations(spec, strict=True), f"accepted {name}")
        overlay = {"hostPID": True, "containers": [{"name": "agent", "securityContext": {"readOnlyRootFilesystem": False}}]}
        self.assertEqual(len(pod_spec_violations(overlay)), 2)


if __name__ == "__main__":
    unittest.main()
