#!/usr/bin/env python3
"""Provision operator-managed per-node HTTP credentials for Fortuna agents.

Plaintext tokens are written only to node-local output files. The Core registry
contains SHA-256 digests and identity bindings, never plaintext tokens.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import sys
import tempfile
from datetime import datetime, timedelta, timezone

SAFE_NODE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,252}[A-Za-z0-9]$|^[A-Za-z0-9]$")


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


def rfc3339(value: datetime) -> str:
    return value.astimezone(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def validate_identity(value: str, label: str) -> str:
    value = value.strip()
    if not value or value != value.strip() or len(value) > 255:
        raise ValueError(f"invalid {label}")
    return value


def validate_node(node: str) -> str:
    node = node.strip()
    if not SAFE_NODE.fullmatch(node):
        raise ValueError(f"invalid node name: {node!r}")
    return node


def load_registry(path: Path | None) -> dict:
    if path is None:
        return {"credentials": []}
    data = json.loads(path.read_text(encoding="utf-8"))
    if set(data.keys()) != {"credentials"} or not isinstance(data["credentials"], list):
        raise ValueError("registry must contain only a credentials array")
    seen_ids: set[str] = set()
    seen_digests: set[str] = set()
    for item in data["credentials"]:
        if not isinstance(item, dict):
            raise ValueError("credential entry must be an object")
        cid = str(item.get("id", "")).strip()
        digest = str(item.get("token_sha256", "")).strip()
        if not cid or cid in seen_ids:
            raise ValueError(f"duplicate or empty credential id: {cid!r}")
        if digest:
            if digest in seen_digests:
                raise ValueError("duplicate token digest in existing registry")
            seen_digests.add(digest)
        seen_ids.add(cid)
    return data


def atomic_write(path: Path, data: bytes, mode: int) -> None:
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    try:
        os.chmod(path.parent, 0o700)
    except OSError:
        pass
    fd, tmp_name = tempfile.mkstemp(prefix=f".{path.name}.", dir=str(path.parent))
    tmp_path = Path(tmp_name)
    try:
        os.fchmod(fd, mode)
        with os.fdopen(fd, "wb") as handle:
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(tmp_path, path)
        try:
            os.chmod(path, mode)
        except OSError:
            pass
    finally:
        if tmp_path.exists():
            tmp_path.unlink()


def registry_bytes(registry: dict) -> bytes:
    return (json.dumps(registry, indent=2, sort_keys=True) + "\n").encode("utf-8")


def issue(args: argparse.Namespace) -> int:
    cluster_id = validate_identity(args.cluster_id, "cluster id")
    nodes = [validate_node(node) for node in args.node]
    if len(nodes) != len(set(nodes)):
        raise ValueError("duplicate node name")
    if args.ttl_hours <= 0:
        raise ValueError("ttl-hours must be positive")

    existing = Path(args.existing_registry) if args.existing_registry else None
    registry = load_registry(existing)
    credentials = registry["credentials"]
    existing_ids = {str(item.get("id", "")) for item in credentials}
    existing_digests = {str(item.get("token_sha256", "")) for item in credentials if item.get("token_sha256")}

    now = utc_now()
    not_before = now - timedelta(minutes=5)
    expires_at = now + timedelta(hours=args.ttl_hours)
    stamp = now.strftime("%Y%m%dT%H%M%SZ")
    output_dir = Path(args.output_dir).expanduser().resolve()
    output_dir.mkdir(parents=True, exist_ok=True, mode=0o700)
    try:
        os.chmod(output_dir, 0o700)
    except OSError:
        pass

    issued = []
    for node in nodes:
        agent_id = validate_identity(f"{node}-agent", "agent id")
        token = secrets.token_urlsafe(48)
        digest = hashlib.sha256(token.encode("utf-8")).hexdigest()
        if digest in existing_digests:
            raise RuntimeError("unexpected generated token digest collision")
        credential_id = f"{node}-{stamp}-{secrets.token_hex(4)}"
        if credential_id in existing_ids:
            raise RuntimeError("unexpected generated credential id collision")
        entry = {
            "id": credential_id,
            "cluster_id": cluster_id,
            "agent_id": agent_id,
            "token_sha256": digest,
            "not_before": rfc3339(not_before),
            "expires_at": rfc3339(expires_at),
            "revoked": False,
        }
        credentials.append(entry)
        existing_ids.add(credential_id)
        existing_digests.add(digest)

        token_path = output_dir / "nodes" / node / "token"
        atomic_write(token_path, (token + "\n").encode("utf-8"), 0o600)
        issued.append({
            "node": node,
            "agent_id": agent_id,
            "credential_id": credential_id,
            "token_file": str(token_path),
        })

    registry_path = output_dir / "registry.json"
    atomic_write(registry_path, registry_bytes(registry), 0o600)
    print(json.dumps({"registry": str(registry_path), "issued": issued}, indent=2))
    return 0


def revoke(args: argparse.Namespace) -> int:
    registry_path = Path(args.registry).expanduser().resolve()
    registry = load_registry(registry_path)
    found = False
    for entry in registry["credentials"]:
        if str(entry.get("id", "")) == args.credential_id:
            entry["revoked"] = True
            found = True
            break
    if not found:
        raise ValueError(f"credential id not found: {args.credential_id}")
    output = Path(args.output).expanduser().resolve() if args.output else registry_path
    atomic_write(output, registry_bytes(registry), 0o600)
    print(json.dumps({"registry": str(output), "revoked": args.credential_id}, indent=2))
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)

    issue_parser = sub.add_parser("issue", help="issue new per-node tokens and build/extend a Core registry")
    issue_parser.add_argument("--cluster-id", required=True)
    issue_parser.add_argument("--node", action="append", required=True, help="Kubernetes node name; repeat for multiple nodes")
    issue_parser.add_argument("--output-dir", required=True, help="private operator output directory")
    issue_parser.add_argument("--existing-registry", help="optional current registry to preserve for overlap rotation")
    issue_parser.add_argument("--ttl-hours", type=int, default=2160, help="credential lifetime; default 90 days")
    issue_parser.set_defaults(func=issue)

    revoke_parser = sub.add_parser("revoke", help="mark one credential revoked in a registry")
    revoke_parser.add_argument("--registry", required=True)
    revoke_parser.add_argument("--credential-id", required=True)
    revoke_parser.add_argument("--output", help="write to a new registry path instead of replacing the input atomically")
    revoke_parser.set_defaults(func=revoke)
    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    try:
        return args.func(args)
    except (OSError, ValueError, json.JSONDecodeError, RuntimeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
