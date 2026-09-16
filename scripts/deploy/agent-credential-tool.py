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
SHA256_HEX = re.compile(r"^[0-9a-f]{64}$")


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


def rfc3339(value: datetime) -> str:
    return value.astimezone(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def validate_identity(value: str, label: str) -> str:
    if not value or value != value.strip() or len(value) > 255:
        raise ValueError(f"invalid {label}")
    return value


def validate_node(node: str) -> str:
    if not node or node != node.strip() or not SAFE_NODE.fullmatch(node):
        raise ValueError(f"invalid node name: {node!r}")
    return node


def requested_identities(args: argparse.Namespace) -> list[tuple[str, str]]:
    identities: list[tuple[str, str]] = []
    for raw_node in args.node or []:
        node = validate_node(raw_node)
        identities.append((node, validate_identity(f"{node}-agent", "agent id")))
    for raw_mapping in args.identity or []:
        if "=" not in raw_mapping:
            raise ValueError("--identity must use NODE=AGENT_ID")
        raw_node, raw_agent = raw_mapping.split("=", 1)
        identities.append((validate_node(raw_node), validate_identity(raw_agent, "agent id")))
    if not identities:
        raise ValueError("at least one --node or --identity is required")
    nodes = [node for node, _ in identities]
    agents = [agent for _, agent in identities]
    if len(nodes) != len(set(nodes)):
        raise ValueError("duplicate node name")
    if len(agents) != len(set(agents)):
        raise ValueError("duplicate agent id")
    return identities


def load_registry(path: Path | None) -> dict:
    if path is None:
        return {"credentials": []}
    data = json.loads(path.read_text(encoding="utf-8"))
    if set(data.keys()) != {"credentials"} or not isinstance(data["credentials"], list):
        raise ValueError("registry must contain only a credentials array")
    seen_ids: set[str] = set()
    seen_tokens: set[str] = set()
    seen_certs: set[str] = set()
    for item in data["credentials"]:
        if not isinstance(item, dict):
            raise ValueError("credential entry must be an object")
        cid = validate_identity(str(item.get("id", "")), "credential id")
        validate_identity(str(item.get("cluster_id", "")), "cluster id")
        validate_identity(str(item.get("agent_id", "")), "agent id")
        if cid in seen_ids:
            raise ValueError(f"duplicate credential id: {cid!r}")
        token_raw = str(item.get("token_sha256", ""))
        cert_raw = str(item.get("certificate_sha256", ""))
        if bool(token_raw) == bool(cert_raw):
            raise ValueError(f"credential {cid!r} must contain exactly one digest type")
        if token_raw:
            if token_raw != token_raw.strip() or not SHA256_HEX.fullmatch(token_raw):
                raise ValueError(f"credential {cid!r} has invalid token_sha256")
            if token_raw in seen_tokens:
                raise ValueError("duplicate token digest in existing registry")
            seen_tokens.add(token_raw)
        if cert_raw:
            if cert_raw != cert_raw.strip() or not SHA256_HEX.fullmatch(cert_raw):
                raise ValueError(f"credential {cid!r} has invalid certificate_sha256")
            if cert_raw in seen_certs:
                raise ValueError("duplicate certificate digest in existing registry")
            seen_certs.add(cert_raw)
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
    identities = requested_identities(args)
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
    for node, agent_id in identities:
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
    issue_parser.add_argument("--node", action="append", default=[], help="node using default AGENT_ID=<node>-agent; repeat as needed")
    issue_parser.add_argument("--identity", action="append", default=[], help="explicit NODE=AGENT_ID mapping for AGENT_ID overrides")
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
