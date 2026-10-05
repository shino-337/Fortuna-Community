#!/usr/bin/env python3
"""Split `helm template` output into one file per chart template.

Reads the rendered stream on stdin and writes <out_dir>/<template path relative
to templates/>, keeping the documents of each template in render order.
"""
import os
import re
import sys

HEADER = (
    "# Generated from deploy/helm/fortuna by scripts/build/render-manifests.sh.\n"
    "# Do not edit: change the chart and re-render. CI fails when they differ.\n"
)
SOURCE = re.compile(r"^# Source: [^/]+/templates/(.+)$")


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: split_rendered_manifests.py OUT_DIR", file=sys.stderr)
        return 2
    out_dir = sys.argv[1]
    docs: dict[str, list[str]] = {}
    for doc in re.split(r"^---\s*$", sys.stdin.read(), flags=re.M):
        lines = doc.strip("\n").splitlines()
        if not lines:
            continue
        match = SOURCE.match(lines[0])
        if not match:
            print(f"rendered document without a Source line: {lines[0]!r}", file=sys.stderr)
            return 1
        body = "\n".join(lines[1:]).strip("\n")
        if body:
            docs.setdefault(match.group(1), []).append(body)
    for rel, bodies in docs.items():
        path = os.path.join(out_dir, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(HEADER + "".join(f"---\n{b}\n" for b in bodies))
    return 0


if __name__ == "__main__":
    sys.exit(main())
