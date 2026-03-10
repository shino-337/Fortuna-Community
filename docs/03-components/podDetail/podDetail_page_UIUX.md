Pod Detail Page – Full UI/UX Specification

Version: 1.0
Audience: Frontend Engineers, Product, Backend
Scope: Pod Detail View + Runtime Process Monitoring + Incident Workflow

1. Design Principles
1.1 Core Objectives

Tối ưu cho incident investigation

Giảm cognitive load

Ưu tiên thông tin actionable

Phân tách rõ: Static Config vs Runtime vs Security vs Network

Thông tin critical luôn visible above-the-fold

1.2 Visual Hierarchy Priority

Pod Status

Restart Count

CPU / Memory Usage

Suspicious Process

Security Risk

Metadata

1.3 Design Language
Color System
State	Color	Usage
Running	Green	Status badge
Warning	Orange	Restart spike, risk medium
Critical	Red	CrashLoop, OOM, high risk
Info	Blue	Metadata, secondary info
Neutral	Gray	Labels, background
Background

Dark theme (default)

High contrast for critical info

Status badge must visually pop

Animation Policy

Use animation only for:

Status change

Resource spike transition

Expand/collapse drawer

Tab switch (fade 150ms)

No decorative animation.

2. Global Layout
┌───────────────────────────────────────────────┐
│ Breadcrumb + Cluster + Namespace Switcher    │
├───────────────────────────────────────────────┤
│ Sticky Pod Header                            │
├───────────────────────────────────────────────┤
│ Horizontal Tab Navigation                    │
├───────────────────────────────────────────────┤
│ Tab Content Area                             │
└───────────────────────────────────────────────┘
3. Sticky Pod Header
3.1 Layout Structure

Left:

Status Badge

Pod Name (Large, Bold)

Namespace Tag

Node (Clickable)

Owner (Clickable)

Right:

Restart Count (Badge)

Age

Risk Score

Action Buttons

3.2 Header Fields
Field	UI Type	Behavior
Status	Badge	Real-time color change
Pod Name	Text	Copyable
Namespace	Tag	Click → filter by namespace
Node	Link	Navigate to Node Detail
Owner	Link	Navigate to ReplicaSet/Deployment
Restart Count	Badge	Click → open restart timeline modal
Risk Score	Colored badge	Hover → show risk breakdown
3.3 Action Buttons
[Exec]
[Logs]
[Describe]
[Export YAML ▼]
[Delete]
Export YAML Dropdown

Options:

Download Full YAML

Download Sanitized YAML

Copy YAML to Clipboard

Export as JSON

Click → opens side drawer preview before download

Drawer includes:

Syntax highlight

Copy button

Download button

4. Tabs Structure
Overview
Configuration (future)
Runtime — GET /api/v1/pods/:id/runtime-metrics (container CPU/memory/state)
Runtime Process Monitoring — GET /api/v1/pods/:id/processes (process snapshot)
Network — GET /api/v1/pods/:id/network-connections (connection snapshot)
Security (SBOM + Related Risks in current impl)
Events — GET /api/v1/pods/:id/events (K8s events); filter Warning/Normal
Logs (future)
Related Resources (future)
5. Tab: Overview
Purpose

Quick health summary.

Layout

Two-column grid:

Left:

Basic Info Card

Labels & Annotations

Right:

Resource Usage Chart

Restart Timeline

Health Indicators

5.1 Resource Usage

Graph:

CPU

Memory

Time range selector (1h / 6h / 24h)

If spike detected:

Highlight spike in orange/red

Tooltip shows timestamp

5.2 Restart Timeline

Mini horizontal bar:

Each restart = red dot

Hover → timestamp + exit reason

Click → open modal with restart logs

6. Tab: Configuration
Sections
6.1 Containers

Accordion per container.

Collapsed view:

Container Name

Image

Resource limits

Expanded view:

Image digest

Pull policy

Command / Args

Ports

Env vars

Volume mounts

Security context

6.2 Volumes

Table:
| Name | Type | Source | ReadOnly |

Click → expand details

6.3 Probes

Display:

Liveness

Readiness

Startup

Color code:

Healthy → Green

Failing → Red

7. Tab: Runtime
Layout

Top:

Container Runtime Summary Table

Bottom:

Resource charts

7.1 Container Runtime Table

| Container | Ready | Restart | State | Exit Code | Reason |

State color-coded:

Running → Green

Waiting → Orange

Terminated → Red

Click container → open side drawer with:

Last termination reason

OOM flag

Started at

Finished at

8. Tab: Runtime Process Monitoring

This is a primary operational feature.

8.1 Layout
-------------------------------------------------
| Filter Bar                                    |
-------------------------------------------------
| Process Table                                 |
-------------------------------------------------
| Optional: Process Tree View (toggle)          |
-------------------------------------------------
8.2 Filter Bar

Filters:

High CPU (> X%)

Long Running (> X hours)

Running as root

Suspicious binary

Search by command

Auto-refresh indicator:

Live (green dot)

Paused (gray)

Refresh interval: 5s default

8.3 Process Table

Columns:

| PID | PPID | User | CPU% | MEM% | Start Time | Command | Container |

Rules:

Sortable

Sticky header

Highlight:

CPU > 70% → Orange

CPU > 90% → Red

User root → Red text

Click row → open side drawer

8.4 Process Detail Drawer

Right-side slide panel.

Sections:

Process Metadata

Full command

Binary path

Binary hash

Start time

Working directory

Parent Chain

Process tree up to root

Click parent to inspect

Network Connections

| Destination | Port | Protocol | State |

Open Files

| Path | Mode |

Security Insights

Executed from writable path?

Running as root?

Has dangerous capabilities?

Known suspicious hash?

Color coding:

Safe → Green

Suspicious → Orange

Dangerous → Red

9. Tab: Network

Table:

| Source | Destination | Port | Protocol | Bytes | State |

Features:

Filter internal/external

Highlight unknown external IP

Hover → geo + ASN info (if available)

10. Tab: Security
Risk Summary Card

Display:

CVE count

Privileged container?

Run as root?

Writable root FS?

Cap_sys_admin?

Each item:

Badge color-coded

Click → show explanation panel

SBOM Section

Table:
| Package | Version | Vulnerability | Severity |

Filter by severity.

11. Tab: Events

Raw K8s events.

Columns:
| Time | Type | Reason | Message |

Filter:

Warning

Error

Normal

12. Tab: Logs

Log viewer features:

Container selector dropdown

Live toggle

Time range filter

Regex search

Auto-scroll toggle

Color highlight:

ERROR → Red

WARN → Orange

13. Tab: Related Resources

Grid cards:

Node

ReplicaSet

Deployment

Service

ConfigMap

Secret

PVC

Click → navigate.

14. Incident Investigation Workflow
Step 1: Dashboard Alert

User sees:

High CPU

CrashLoop

Suspicious traffic

Click → Pod

Step 2: Overview

Check:

Restart spike?

Resource spike?

Risk score?

Step 3: Runtime

Check:

OOM?

Exit code?

Container unstable?

Step 4: Runtime Process Monitoring

Check:

Unknown binary?

Reverse shell?

Crypto miner?

High CPU PID?

Inspect:

Parent process

Network connection

Step 5: Network

Check:

External IP?

Unexpected egress?

Step 6: Security

Check:

Privileged?

Running as root?

Step 7: Root Cause Classification
Pattern	Likely Cause
CPU spike + unknown binary	Compromise
OOM + memory growth	App bug
Restart + config change	Bad deploy
Suspicious network	Data exfiltration
15. UX Guardrails

Do NOT:

Overload above-the-fold

Show raw YAML inline

Animate unnecessarily

Must:

Highlight critical states immediately

Make investigation linear and logical

Allow deep inspection without page reload

16. Technical Sync Model (Frontend Perspective)

Static config → fetch on load

Runtime metrics → poll 10s

Process monitoring → WebSocket

Logs → streaming

Network → poll 15s

Show sync state indicator:

Synced

Delayed

Disconnected

17. Performance Considerations

Process table must virtualize rows

Logs must stream chunked

Avoid blocking UI on heavy JSON

18. Accessibility

Color not sole indicator

Icon + text for critical state

Keyboard navigable tables

Copy-to-clipboard for all critical IDs

19. Final Outcome

Frontend must deliver:

Clear operational visibility

Fast incident triage

Deep runtime inspection

Config export capability

Zero ambiguity about pod health