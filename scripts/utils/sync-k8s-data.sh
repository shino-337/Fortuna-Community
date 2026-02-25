#!/usr/bin/env bash
set -euo pipefail

CORE_URL="${CORE_URL:-http://localhost:8080}"
CLUSTER_ID="${CLUSTER_ID:-minikube}"

python3 - <<PY | curl -s -X POST "${CORE_URL}/api/v1/agent/sync" -H "Content-Type: application/json" -d @-
import json, subprocess

def kubectl_json(args):
    cmd = ["kubectl"] + args + ["-o", "json"]
    return json.loads(subprocess.check_output(cmd).decode("utf-8"))

pods = kubectl_json(["get", "pods", "-A"]).get("items", [])
service_accounts = kubectl_json(["get", "serviceaccounts", "-A"]).get("items", [])
roles = kubectl_json(["get", "roles", "-A"]).get("items", [])
role_bindings = kubectl_json(["get", "rolebindings", "-A"]).get("items", [])
cluster_roles = kubectl_json(["get", "clusterroles"]).get("items", [])
cluster_role_bindings = kubectl_json(["get", "clusterrolebindings"]).get("items", [])

payload = {
    "clusterId": "${CLUSTER_ID}",
    "data": {
        "isFullSync": True,
        "isDeltaSync": False,
        "pods": [
            {
                "name": p["metadata"]["name"],
                "namespace": p["metadata"]["namespace"],
                "uid": p["metadata"]["uid"],
                "serviceAccountName": p["spec"].get("serviceAccountName", ""),
                "nodeName": p["spec"].get("nodeName", ""),
                "hostNetwork": bool(p["spec"].get("hostNetwork", False)),
                "hostPID": bool(p["spec"].get("hostPID", False)),
                "hostIPC": bool(p["spec"].get("hostIPC", False)),
                "automountServiceAccountToken": bool(p["spec"].get("automountServiceAccountToken", True)),
                "podSecurityContext": p["spec"].get("securityContext", {}),
                "tolerations": p["spec"].get("tolerations", []),
                "affinity": p["spec"].get("affinity", {}),
                "volumes": [
                    {
                        "name": v.get("name", ""),
                        "hostPath": v.get("hostPath", None),
                    }
                    for v in p["spec"].get("volumes", [])
                ],
                "containers": [
                    {
                        "name": c.get("name", ""),
                        "securityContext": c.get("securityContext", {}),
                        "volumeMounts": c.get("volumeMounts", []),
                    }
                    for c in p["spec"].get("containers", [])
                ],
            }
            for p in pods
        ],
        "serviceAccounts": [
            {
                "name": sa["metadata"]["name"],
                "namespace": sa["metadata"]["namespace"],
                "uid": sa["metadata"]["uid"],
                "labels": sa["metadata"].get("labels", {}),
                "secrets": [s.get("name", "") for s in sa.get("secrets", [])],
            }
            for sa in service_accounts
        ],
        "roles": [
            {
                "name": r["metadata"]["name"],
                "namespace": r["metadata"]["namespace"],
                "uid": r["metadata"]["uid"],
                "rules": r.get("rules", []),
            }
            for r in roles
        ],
        "roleBindings": [
            {
                "name": rb["metadata"]["name"],
                "namespace": rb["metadata"]["namespace"],
                "uid": rb["metadata"]["uid"],
                "roleRef": rb.get("roleRef", {}),
                "subjects": rb.get("subjects", []),
            }
            for rb in role_bindings
        ],
        "clusterRoles": [
            {
                "name": cr["metadata"]["name"],
                "uid": cr["metadata"]["uid"],
                "rules": cr.get("rules", []),
            }
            for cr in cluster_roles
        ],
        "clusterRoleBindings": [
            {
                "name": crb["metadata"]["name"],
                "uid": crb["metadata"]["uid"],
                "roleRef": crb.get("roleRef", {}),
                "subjects": crb.get("subjects", []),
            }
            for crb in cluster_role_bindings
        ],
    },
}

print(json.dumps(payload))
PY
