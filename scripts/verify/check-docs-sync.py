#!/usr/bin/env python3
"""Keep docs, manifests and source in step.

1. Every repository path that a Markdown file names (deploy/..., scripts/...,
   docs/..., and relative links) exists.
2. Every environment variable that deploy/ sets on the Core and Agent
   containers is read somewhere in that component's Go source.
3. Every environment variable the Core and Agent manifests set is listed in
   docs/reference/CONFIGURATION.md.
"""
from pathlib import Path
import re
import subprocess
import sys

import yaml

REPO = Path(__file__).resolve().parents[2]
SKIP_DIRS = {"node_modules", ".git", "dist", "vendor"}
TOP_DIRS = ("deploy/", "scripts/", "docs/", "core/", "agent/", "api/", "dashboard/", "scenarios/")
PATH_IN_CODE = re.compile(r"`((?:%s)[^`\s]*)`" % "|".join(re.escape(d) for d in TOP_DIRS))
MD_LINK = re.compile(r"\]\(([^)\s#]+)(?:#[^)]*)?\)")
PLACEHOLDER = re.compile(r"[<>*{}$]|\.\.\.")
CONFIG_DOC = REPO / "docs" / "reference" / "CONFIGURATION.md"
# Set for libraries and tools in the container, not read by Fortuna code.
PROCESS_ENV = {"HOME", "TMPDIR", "XDG_CACHE_HOME"}
CONTAINERS = {
    ("Deployment", "fortuna-core", "core"): "core",
    ("DaemonSet", "fortuna-agent", "agent"): "agent",
    ("DaemonSet", "fortuna-agent", "image-export"): "agent",
}


def markdown_files():
    for path in REPO.rglob("*.md"):
        if not SKIP_DIRS.intersection(path.relative_to(REPO).parts):
            yield path


def repo_path_exists(ref):
    if (REPO / ref).exists():
        return True
    # Go symbols such as core/pkg/agentidentity.Store name their package directory.
    head, _, symbol = ref.rpartition(".")
    if symbol[:1].isupper() and (REPO / head).is_dir():
        return True
    # Local files the docs ask you to create, such as ignored config files.
    return subprocess.run(["git", "check-ignore", "-q", ref], cwd=REPO).returncode == 0


def check_paths(errors):
    for md in markdown_files():
        text = md.read_text(encoding="utf-8")
        for match in PATH_IN_CODE.finditer(text):
            ref = match.group(1).rstrip(".,:;)")
            if PLACEHOLDER.search(ref):
                continue
            if not repo_path_exists(ref.split(":")[0].split("#")[0]):
                errors.append(f"{md.relative_to(REPO)}: names missing path `{ref}`")
        for match in MD_LINK.finditer(text):
            target = match.group(1)
            if re.match(r"^[a-z]+:", target) or PLACEHOLDER.search(target):
                continue
            if not (md.parent / target).resolve().exists():
                errors.append(f"{md.relative_to(REPO)}: broken link ({target})")


def manifest_env():
    env = {}
    for path in (REPO / "deploy").rglob("*.yaml"):
        if (REPO / "deploy" / "helm") in path.parents:
            continue
        for doc in yaml.safe_load_all(path.read_text(encoding="utf-8")):
            if not isinstance(doc, dict):
                continue
            spec = ((doc.get("spec") or {}).get("template") or {}).get("spec") or {}
            for container in spec.get("containers") or []:
                key = (doc.get("kind"), (doc.get("metadata") or {}).get("name"), container.get("name"))
                component = CONTAINERS.get(key)
                if component:
                    for item in container.get("env") or []:
                        env.setdefault(item["name"], set()).add((component, path.relative_to(REPO)))
    return env


def go_source(component):
    return "\n".join(p.read_text(encoding="utf-8") for p in (REPO / component).rglob("*.go"))


def check_env(errors):
    sources = {c: go_source(c) for c in set(CONTAINERS.values())}
    documented = set(re.findall(r"`([A-Z][A-Z0-9_]+)`", CONFIG_DOC.read_text(encoding="utf-8")))
    for name, uses in sorted(manifest_env().items()):
        for component, path in sorted(uses):
            if name not in PROCESS_ENV and f'"{name}"' not in sources[component]:
                errors.append(f"{path}: sets {name}, which {component}/ never reads")
        if name not in documented:
            errors.append(f"{name} is set in deploy/ but missing from {CONFIG_DOC.relative_to(REPO)}")


def main():
    errors = []
    check_paths(errors)
    check_env(errors)
    for error in errors:
        print(error)
    if errors:
        print(f"\n{len(errors)} docs/source mismatches", file=sys.stderr)
        return 1
    print("docs, manifests and source are in sync")
    return 0


if __name__ == "__main__":
    sys.exit(main())
