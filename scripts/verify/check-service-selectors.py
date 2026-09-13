#!/usr/bin/env python3
"""Validate bundled Services against bundled workload labels; no cluster access."""
from pathlib import Path
import yaml

root = Path(__file__).resolve().parents[2]
objects = []
for path in (root / 'deploy').glob('*.yaml'):
    objects.extend((path, doc) for doc in yaml.safe_load_all(path.read_text()) if isinstance(doc, dict))
workloads = [doc for _, doc in objects if doc.get('kind') in ('Deployment', 'DaemonSet', 'StatefulSet')]
errors = []
for path, doc in objects:
    if doc.get('kind') != 'Service':
        continue
    selector = doc.get('spec', {}).get('selector')
    if not selector:
        continue
    namespace = doc['metadata'].get('namespace', 'default')
    matches = [w for w in workloads if w['metadata'].get('namespace', 'default') == namespace
               and all(w['spec']['template']['metadata'].get('labels', {}).get(k) == v for k, v in selector.items())]
    if not matches:
        errors.append(f'{path.relative_to(root)}: selector {selector} matches no bundled workload')
if errors:
    raise SystemExit('\n'.join(errors))
print('Bundled Service selectors match workload labels.')
