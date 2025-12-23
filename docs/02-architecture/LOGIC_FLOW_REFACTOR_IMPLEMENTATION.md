# LOGIC FLOW REFACTOR – IMPLEMENTATION GUIDE
## Correcting Agent/Core Responsibilities for Scalability & Security Intelligence

---

## 1. Mục tiêu tài liệu

Tài liệu này là **Implementation Guide**, không phải high-level architecture.

Mục tiêu:
- Phân tích **logic flow hiện tại (as-is)** một cách kỹ thuật
- Chỉ ra **điểm sai về phân vai Agent/Core**
- Đề xuất **logic flow mới (to-be)** đúng chuẩn industry
- Cung cấp **checklist, module map, data flow** để developer:
  - Bắt đầu refactor ngay
  - Không phá vỡ hệ thống đang chạy
  - Chuẩn bị cho Attack Path, Runtime, Multi-cluster

---

## 2. Logic Flow hiện tại (AS-IS)

### 2.1 Flow thực tế đang chạy

Pod Created / Updated
↓
Agent PodWatcher
↓
Agent:

Pull image layers

Parse OS package DB (dpkg/apk/rpm)

Parse language packages (node/python)

Generate PURL

Match CVE (local DB)

Create preliminary insight
↓
Send SBOM + CVE + Insight → Core
↓
Core:

Persist data

Risk scoring

Graph update

Dashboard update

yaml
Copy code

---

### 2.2 Phân tích trách nhiệm hiện tại

| Component | Thực tế đang làm | Vấn đề |
|---------|------------------|--------|
| Agent | SBOM + CVE intelligence | Nặng, khó update |
| Core | Storage + scoring | Không phải “brain” |
| CVE DB | Local per agent | Không kiểm soát |
| Flow | Sync | Dễ nghẽn |

👉 **Agent đang làm việc của Core**  
👉 **Core không giữ được security intelligence**

---

## 3. Vấn đề kiến trúc cốt lõi

### 3.1 Architectural Drift

Thiết kế ban đầu:
> Agent-based architecture

Implementation hiện tại:
> Agent-heavy + Core-passive

⛔ Đây là **architectural drift**, không phải bug.

---

### 3.2 Sai nguyên tắc Control Plane vs Data Plane

| Nguyên tắc đúng | Hiện tại |
|----------------|----------|
| Agent = collect | Agent = analyze |
| Core = decide | Core = persist |
| Central CVE DB | Distributed CVE DB |
| Async pipeline | Sync per pod |

---

### 3.3 Hệ quả nếu không sửa

- Agent CPU spike khi pod scale
- Update CVE = redeploy toàn cluster
- Không làm được multi-tenant SaaS
- Attack Path chỉ là graph vẽ đẹp, không có intelligence

---

## 4. Nguyên tắc thiết kế lại (BẮT BUỘC TUÂN THỦ)

1. Agent **KHÔNG** quyết định risk
2. Agent **KHÔNG** match CVE
3. CVE intelligence **chỉ nằm ở Core**
4. SBOM và CVE là **hai pipeline khác nhau**
5. Tất cả xử lý nặng phải **async**
6. Mọi dữ liệu phải **deduplicate + cache**

---

## 5. Logic Flow đề xuất (TO-BE)

### 5.1 SBOM Flow (Agent → Core)

Pod Created / Updated
↓
Agent:

Extract image digest

Extract raw package inventory

Generate minimal SBOM
(name, version, purl, ecosystem)
↓
Send SBOM → Core Ingest API
↓
Core:

Deduplicate by image digest

Normalize components

Persist SBOM

yaml
Copy code

#### Ghi chú implementation
- **SBOM không chứa CVE**
- SBOM cache theo `image_digest`
- Một image → một SBOM

---

### 5.2 CVE Matching Flow (Core Async)

New / Updated SBOM
↓
CVE Matching Worker

Load OSV DB (central)

Match by purl + ecosystem
↓
CVE Findings
↓
Insight Generator
↓
Risk Engine

yaml
Copy code

---

## 6. Phân vai chi tiết từng component

### 6.1 Agent – Data Plane

#### Agent PHẢI làm
- Watch K8s workload (Pod, Container)
- Extract image digest
- Extract raw package metadata
- Generate SBOM (CycloneDX/SPDX minimal)
- Collect runtime telemetry (future: eBPF)

#### Agent TUYỆT ĐỐI KHÔNG làm
- CVE matching
- Severity calculation
- Risk scoring
- Policy evaluation

> Agent = sensor + executor  
> Agent **không phải brain**

---

### 6.2 Core – Control Plane

Core chịu trách nhiệm:
- Normalize assets
- Central CVE DB (OSV)
- CVE matching
- Risk correlation
- Attack path simulation
- Policy engine
- API + Dashboard

---

## 7. Data Model cần điều chỉnh

### 7.1 SBOM Tables

- `images (image_digest, repo, tag)`
- `sboms (sbom_id, image_digest, created_at)`
- `sbom_components (sbom_id, purl, ecosystem, version)`

### 7.2 CVE Tables

- `cves (cve_id, severity, cvss)`
- `cve_matches (purl, cve_id, fixed_version)`

### 7.3 Insight Tables

- `insights (type, resource_id, severity, status)`
- `risk_scores (resource_id, score, priority)`

---

## 8. Attack Path – liên quan trực tiếp tới flow

### 8.1 Vì sao flow hiện tại không làm được Attack Path

- CVE gắn trực tiếp vào Pod
- Không có component-level graph

---

### 8.2 Graph Model đúng

CVE
→ Component
→ Image
→ Pod
→ ServiceAccount
→ RBAC
→ NetworkPolicy
→ Other Pods

yaml
Copy code

👉 **Flow mới là điều kiện bắt buộc** để:
- Simulate lateral movement
- Prioritize đúng tài nguyên

---

## 9. Refactor Plan – Step by Step

### Phase 1 – Immediate (1–2 sprint)

- [ ] Remove CVE matching khỏi agent
- [ ] Agent chỉ gửi SBOM raw
- [ ] Core implement CVE worker
- [ ] SBOM cache theo image digest

### Phase 2 – Stabilization

- [ ] Async queue Agent → Core
- [ ] Worker pool cho CVE matching
- [ ] Tách SBOM / CVE / Insight schema

### Phase 3 – Enable Advanced Features

- [ ] Attack Path Engine
- [ ] Risk prioritization nâng cao
- [ ] Runtime + SBOM correlation

---

## 10. Code Impact Map

### Agent
- Xóa CVE logic
- Simplify pipeline
- Module: `sbom/`, `watcher/`, `forwarder/`

### Core
- Thêm `cve-matcher/worker`
- Thêm `sbom-normalizer`
- Thêm `risk-correlator`

---

## 11. Architectural Decision (ADR – bắt buộc)

**Decision**  
Agent là Data Plane.  
Core là nơi duy nhất chứa Security Intelligence.

**Reason**
- Scale
- Maintainability
- SaaS / Multi-cluster readiness

---

## 12. Kết luận (nói thẳng)

- Code hiện tại **không sai**
- Nhưng phân vai **sai chỗ**
- Nếu không sửa bây giờ → chi phí refactor sau này x10

> Nếu Agent không sở hữu SBOM extraction  
> → Agent không nên tồn tại  
> Nếu Agent sở hữu CVE intelligence  
> → Kiến trúc sẽ chết sớm