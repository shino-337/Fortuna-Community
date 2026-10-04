# Maintainer records

These documents track how Fortuna's security hardening was planned, verified and released. They are written for maintainers and reviewers, not for people installing or using Fortuna. To get started, go to the [documentation index](../README.md).

| Document | Contents |
|---|---|
| [Audit implementation plan](NEXT_AUDIT_PLAN.md) | Work packages A–I, dependencies, acceptance gates and deployment criteria |
| [Audit remediation status](AUDIT_REMEDIATION_STATUS.md) | PR order, behavior changes, mitigations and lab validation |
| [Integration acceptance 2026-09-29](INTEGRATION_ACCEPTANCE_20260929.md) | Live two-cluster backend acceptance gate and its recorded results |
| [Local CI](LOCAL_CI.md) | Run the GitHub CI workflow locally, including the PR #54 exact-head exception |
| [Performance baseline 2026-09-29](PERFORMANCE_BASELINE_20260929.md) | Measured performance changes and the raw [measurement artifact](performance/20260929.json) |
| [Public release checklist](PUBLIC_RELEASE_CHECKLIST.md) | Full-history secret scan, history rewrite, credential rotation and release hygiene |

Design contracts that describe how the product behaves stay in [Reference](../reference/README.md).
