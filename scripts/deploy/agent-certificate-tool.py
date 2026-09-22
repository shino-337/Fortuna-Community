#!/usr/bin/env python3
"""Issue per-Agent mTLS certificates using an operator-held CA; never print keys."""
from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import tempfile

spec = importlib.util.spec_from_file_location("credentials", Path(__file__).with_name("agent-credential-tool.py"))
credentials = importlib.util.module_from_spec(spec)
spec.loader.exec_module(credentials)


def openssl(*args: str) -> bytes:
    result = subprocess.run(["openssl", *map(str, args)], capture_output=True, check=False)
    if result.returncode:
        raise ValueError("OpenSSL certificate generation/verification failed: " + result.stderr.decode("utf-8", errors="replace").strip())
    return result.stdout


def issue(args) -> int:
    cluster = credentials.validate_identity(args.cluster_id, "cluster id")
    identities = credentials.requested_identities(args)
    if not 1 <= args.ttl_days <= 365:
        raise ValueError("ttl-days must be between 1 and 365")
    registry = credentials.load_registry(Path(args.existing_registry) if args.existing_registry else None)
    ca_cert, ca_key = Path(args.ca_cert).resolve(), Path(args.ca_key).resolve()
    # Refuse issuance whose requested validity exceeds the signing CA.
    openssl("x509", "-in", ca_cert, "-checkend", str(args.ttl_days * 86400 + 60), "-noout")
    output = Path(args.output_dir).expanduser().resolve()
    if output.exists():
        raise ValueError("output directory already exists; use a new generation for rotation")
    output.parent.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix=".agent-certs-", dir=output.parent))
    issued = []
    try:
        extension = stage / "client.ext"
        extension.write_text("basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=clientAuth\n")
        for node, agent in identities:
            node_dir = stage / "nodes" / node
            node_dir.mkdir(parents=True, mode=0o700)
            key, csr, cert = node_dir / "tls.key", node_dir / "client.csr", node_dir / "tls.crt"
            credential_id = "mtls-" + secrets.token_hex(16)
            # The trusted identity binding is in the registry, not a caller-chosen CN.
            openssl("req", "-new", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes", "-keyout", key, "-out", csr, "-subj", "/CN=" + credential_id)
            os.chmod(key, 0o600)
            openssl("x509", "-req", "-in", csr, "-CA", ca_cert, "-CAkey", ca_key, "-set_serial", "0x" + secrets.token_hex(20), "-days", str(args.ttl_days), "-sha256", "-extfile", extension, "-out", cert)
            openssl("verify", "-purpose", "sslclient", "-CAfile", ca_cert, cert)
            digest = hashlib.sha256(openssl("x509", "-in", cert, "-outform", "DER")).hexdigest()
            if any(c.get("certificate_sha256") == digest or c["id"] == credential_id for c in registry["credentials"]):
                raise ValueError("generated credential collision")
            dates = dict(line.split("=", 1) for line in openssl("x509", "-in", cert, "-noout", "-startdate", "-enddate").decode().splitlines())
            not_before = datetime.strptime(dates["notBefore"], "%b %d %H:%M:%S %Y %Z").replace(tzinfo=timezone.utc)
            expires_at = datetime.strptime(dates["notAfter"], "%b %d %H:%M:%S %Y %Z").replace(tzinfo=timezone.utc)
            registry["credentials"].append({"id": credential_id, "cluster_id": cluster, "agent_id": agent, "certificate_sha256": digest, "not_before": credentials.rfc3339(not_before), "expires_at": credentials.rfc3339(expires_at), "revoked": False})
            shutil.copyfile(ca_cert, node_dir / "ca.crt")
            csr.unlink()
            issued.append({"node": node, "agent_id": agent, "credential_id": credential_id, "certificate_sha256": digest, "directory": str(output / "nodes" / node)})
        extension.unlink()
        credentials.atomic_write(stage / "registry.json", credentials.registry_bytes(registry), 0o600)
        credentials.load_registry(stage / "registry.json")
        os.rename(stage, output)
    finally:
        if stage.exists():
            shutil.rmtree(stage)
    print(json.dumps({"registry": str(output / "registry.json"), "issued": issued}, indent=2))
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    p = sub.add_parser("issue")
    p.add_argument("--cluster-id", required=True)
    p.add_argument("--node", action="append", default=[])
    p.add_argument("--identity", action="append", default=[])
    p.add_argument("--ca-cert", required=True)
    p.add_argument("--ca-key", required=True)
    p.add_argument("--output-dir", required=True)
    p.add_argument("--existing-registry")
    p.add_argument("--ttl-days", type=int, default=30)
    p.set_defaults(func=issue)
    p = sub.add_parser("revoke")
    p.add_argument("--registry", required=True)
    p.add_argument("--credential-id", required=True)
    p.add_argument("--output")
    p.set_defaults(func=credentials.revoke)
    args = parser.parse_args()
    try:
        return args.func(args)
    except (ValueError, OSError, RuntimeError) as exc:
        parser.exit(1, str(exc) + "\n")


if __name__ == "__main__":
    raise SystemExit(main())
