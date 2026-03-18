# SBOM_COMPONENT_SPEC.md

**Status:** official (living spec)  
**Owners:** Core + Agent  
**Last updated:** 2026-03-18

Tài liệu này là **nguồn chuẩn** cho component **SBOM** trong Fortuna. Mọi cải tiến/thay đổi tiếp theo (Agent/Core/DB/Event/Matcher) phải cập nhật tại đây trước hoặc cùng lúc với code.

---

## 1. Scope & non-goals

Tài liệu cover:

- **SBOM extraction** (Agent): parser selection, PURL contract, confidence/source.
- **SBOM ingest** (Core): sanitize, write-guard/immutability, versioning, persistence.
- **Eventing**: `fortuna.sbom.created` payload và mục đích.
- **Downstream**: CVE matcher idempotency, query DB, cache epoch (`mirror_state`).
- **Testing contract**: những testcase bắt buộc để tránh “pipeline đẹp trên giấy”.

Non-goals:

- Không mô tả chi tiết Risk Center UI.
- Không thay thế tài liệu CVE engine; chỉ mô tả các điểm giao nhau (PURL contract, event, idempotency).

---

## 2. Terminology

- **SBOM**: snapshot packages của 1 pod/container (key thực tế là `pod_uid`, không phải image digest).
- **Component**: 1 package/module trong SBOM (`sbom_components`).
- **PURL**: canonical package URL, **source-of-truth** cho ecosystem/name/version khi matching.
- **SBOMSource/Confidence**: provenance + độ tin cậy (SBOM-level và package-level).
- **Finalized**: SBOM snapshot immutable (chỉ mutate khi có context flag).

---

## 3. End-to-end data flow (canonical)

```
Agent Extractor
  └─ emits SBOMFinding{ packages[] with canonical PURLs }
        ↓ (gRPC: SendSBOMFinding)
Core Ingest
  ├─ sanitize PURLs (best-effort, never trust raw blindly)
  ├─ persist SBOM + components (finalized snapshot + version++)
  └─ publish event fortuna.sbom.created (includes components_snapshot)
        ↓ (JetStream)
CVE Matcher Worker
  ├─ idempotency gate per (sbom_id, sbom.version, mirror_state.osv.version)
  ├─ match packages using PURL contract (Go multi-segment supported)
  └─ persist cve_matches + insights
```

---

## 4. PURL contract (CRITICAL)

### 4.1 Source of truth

- **PURL là nguồn chuẩn** cho ecosystem/name/version trong matcher.
- `type` chỉ là metadata, **không** dùng để “đoán” ecosystem nếu đã có PURL.

### 4.2 Go ecosystem (multi-segment must work)

**Canonical format (bắt buộc):**

- `pkg:go/<full-module-path>@<version>`

Ví dụ:

- `pkg:go/github.com/coreos/etcd/client/v3@v3.3.0`
- `pkg:go/k8s.io/kubernetes/cmd/kube-apiserver@v1.29.2`

**Backward compatibility:**

- Core/matcher phải accept `pkg:golang/...` như alias của `pkg:go/...` cho dữ liệu lịch sử.

### 4.3 Distroless/heuristic

Khi không thể xác định package manager/module graph:

- `pkg:generic/<name>@<version>`

Ví dụ:

- `pkg:generic/coredns@1.11.0`

---

## 5. Agent implementation (as-built)

### 5.1 Parsers (high-level)

- OS packages: `dpkg`, `apk`, `rpm`
- Language packages: `npm`, `pip`, `gomod`
- **Go binary analyzer**: `gobinary` đọc `debug/buildinfo`
- Distroless heuristic: `distroless` (synthetic component)

### 5.2 Go binary analyzer (gobinary)

Output:

- `Type="go"` deps + **PURL `pkg:go/...`**
- `Type="go-binary"` main module + **PURL `pkg:go/...`** (inventory)

Giới hạn:

- buildinfo là best-effort; có thể không phản ánh đầy đủ replace/indirect graph.

---

## 6. Core ingest & persistence (as-built)

### 6.1 Sanitize PURL (trust boundary)

- Nếu agent gửi `Purl` nhưng **invalid format** → log warning và **regenerate** theo `(type,name,version)` thay vì persist malformed.
- Nếu agent không gửi `Purl` → Core generate best-effort.

### 6.2 Immutability & versioning

- SBOM snapshot được lưu với:
  - `status=finalized`
  - `version` tăng khi upsert cùng `pod_uid`
- **Write-guard**: nếu SBOM đã finalized, mọi mutate phải có context flag `WithSBOMMutationAllowed(ctx)`.

### 6.3 Storage schema (core tables)

- `sboms` (status, version, image/pod metadata)
- `sbom_components` (sbom_id, component_name, component_version, purl, source, metadata…)
- `sbom_match_runs` (idempotency key: sbom_id + version + mirror_version)

---

## 7. Event contract: fortuna.sbom.created

Core publish event sau commit DB.

Payload (rút gọn):

- `sbom_id`
- `pod_uid`, `namespace`, `container_name`, `image_digest`, …
- `components_snapshot[]`: `{name, version, purl}`

Mục tiêu:

- worker match deterministic, tránh race soft-delete/replace components.

---

## 8. Matcher/Worker contract (as-built)

### 8.1 Idempotency gate

- Worker chỉ chạy matching lần đầu cho key:
  - `(sbom_id, sbom.version, mirror_state(osv).version)`

Nếu duplicate event hoặc retry:

- `EnsureMatchRun()` trả `ok=false` → skip.

### 8.2 Mirror version

- `mirror_state.osv.version` là cache epoch + matcher run epoch.
- Khi sync OSV mirror xong, bump version → worker cho phép re-run.

---

## 9. Required tests (contract locks)

### 9.1 Agent

- GoBinaryParser phải emit `pkg:go/...` PURL cho deps (và main nếu giữ inventory).

### 9.2 Core

- **Passthrough**: không override PURL hợp lệ.
- **Fallback**: PURL malformed → regenerate.
- Core-level E2E: `SendSBOMFinding` → store → matcher alias-match multi-segment Go module.

### 9.3 Matcher

- Parse `pkg:go/<multi-segment>@vX` giữ nguyên full path.
- Parse `pkg:golang/...` alias → ecosystem `go` và giữ full path.

---

## 10. Roadmap (update continuously)

### 10.1 Multi-source reconciliation (priority)

Hợp nhất các nguồn package cho cùng một component:

- `gomod` vs `gobinary` vs OS packages vs distroless synthetic
- tránh double-count và resolve conflicts theo:
  - PURL presence (ưu tiên)
  - confidence/source precedence
  - deterministic rule set

### 10.2 Main go-binary semantics (noise control)

Định nghĩa rõ:

- main binary có match CVE trực tiếp hay chỉ inventory?
- nếu chỉ inventory: cần skip hoặc lower priority trong matcher.

### 10.3 Version normalization policy (Go)

Thiết kế normalize chỉ cho comparator/matching, không mutate dữ liệu lưu.

### 10.4 Hardening

- rate-limit warn logs khi sanitize PURL
- stricter validation cho PURL inputs theo allowlist ecosystems

