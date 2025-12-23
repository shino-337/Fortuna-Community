# SBOM-BASED SCANNING - COMPREHENSIVE ANALYSIS
## Replacing Trivy with Lightweight SBOM Approach

**Date**: 2025-12-12  
**Purpose**: Phân tích flow mới dùng SBOM thay vì Trivy API

---

## 📋 EXECUTIVE SUMMARY

### **VẤN ĐỀ HIỆN TẠI**

```
Current Flow (Trivy-based):
════════════════════════════════════════════════════════════════

Pod Created → Pod Watcher → Image Scanner → Trivy API
                                              ↓
                                    [Full image scan]
                                    [Heavy: 30s/image]
                                    [Download layers]
                                              ↓
Scan Result → CVE Processor → Filter (CRITICAL+HIGH)
                                              ↓
                              Create Insights
                                              ↓
Insight Manager → Auto-trigger Risk Scoring
                                              ↓
Risk Scorer → Separate CVE/Policy → Calculate → Save Score

PROBLEMS:
❌ Heavy dependency on Trivy API/Server
❌ Full image scan every time (30s average)
❌ Network overhead (download layers)
❌ Resource intensive
❌ Tight coupling (Trivy changes = KSAM breaks)
```

### **GIẢI PHÁP ĐỀ XUẤT**

```
Proposed Flow (SBOM-based):
════════════════════════════════════════════════════════════════

Pod Created → Pod Watcher → SBOM Extractor → SBOM Uploader
                                              ↓
                              [Lightweight: 5-10s]
                              [Just package list]
                              [No layer download]
                                              ↓
Core SBOM Processor → CVE Matching Engine → Insights
        ↓                     ↓
   [Parse SBOM]      [Match against CVE DB]
   [One time]        [Fast: <1s]
                                              ↓
                         Risk Engine → Scoring → Dashboard

BENEFITS:
✅ No Trivy dependency
✅ Lightweight (5-10s vs 30s)
✅ Decoupled architecture
✅ Industry standard (SBOM)
✅ Reusable (cache SBOM, re-match CVEs)
✅ Scalable
```

---

## PART 1: UNDERSTANDING SBOM

### **1.1 SBOM là gì?**

```
┌─────────────────────────────────────────────────────────────────┐
│              SBOM - Software Bill of Materials                   │
└─────────────────────────────────────────────────────────────────┘

DEFINITION:
══════════════════════════════════════════════════════════════════
SBOM là "danh sách nguyên liệu" của software, tương tự như:
├─ Bill of Materials trong manufacturing (danh sách linh kiện)
├─ Ingredient list trên nhãn thực phẩm
└─ Parts catalog của ô tô

For container images, SBOM lists:
├─ All packages (OS packages, libraries, dependencies)
├─ Package versions
├─ Package managers (apt, yum, npm, pip, etc.)
├─ Dependencies tree
└─ Metadata (licenses, authors, etc.)


EXAMPLE SBOM (CycloneDX JSON):
══════════════════════════════════════════════════════════════════
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.4",
  "version": 1,
  "metadata": {
    "component": {
      "type": "container",
      "name": "nginx",
      "version": "1.19.0"
    }
  },
  "components": [
    {
      "type": "library",
      "name": "openssl",
      "version": "1.1.1d",
      "purl": "pkg:deb/debian/openssl@1.1.1d"
    },
    {
      "type": "library",
      "name": "nginx",
      "version": "1.19.0",
      "purl": "pkg:deb/debian/nginx@1.19.0"
    },
    {
      "type": "library",
      "name": "zlib",
      "version": "1.2.11",
      "purl": "pkg:deb/debian/zlib@1.2.11"
    }
    // ... more packages
  ]
}

KEY FIELDS:
├─ name: Package name (e.g., "openssl")
├─ version: Exact version (e.g., "1.1.1d")
├─ purl: Package URL (universal identifier)
└─ type: library, application, operating-system, etc.


WHY SBOM IS BETTER:
══════════════════════════════════════════════════════════════════
✅ Lightweight
├─ Just package list (JSON ~100KB)
├─ vs Full image scan (download GB of layers)
└─ 10x faster to generate

✅ Portable
├─ Generate once, use many times
├─ Share across teams/tools
└─ Store in artifact registry

✅ Industry Standard
├─ US Executive Order 14028 (May 2021)
├─ NTIA SBOM minimum requirements
├─ Supply chain security mandate
└─ Widely adopted (Kubernetes, CNCF)

✅ Decoupled
├─ SBOM generation ≠ CVE matching
├─ Can use different tools
├─ Update CVE DB without re-scanning images
└─ Clear separation of concerns
```

### **1.2 SBOM Formats**

```
┌─────────────────────────────────────────────────────────────────┐
│                    SBOM FORMAT STANDARDS                         │
└─────────────────────────────────────────────────────────────────┘

FORMAT 1: CycloneDX ⭐⭐⭐⭐⭐ RECOMMENDED
══════════════════════════════════════════════════════════════════
Developed by: OWASP
Current version: 1.5
Format: JSON, XML
Focus: Security, vulnerability management

Strengths:
✅ Designed for security use cases
✅ Rich vulnerability information
✅ Excellent tooling support
✅ Widely adopted in security space
✅ KSAM-friendly (easy to parse)

Example tools:
├─ Syft (generate)
├─ Grype (consume for CVE matching)
├─ Trivy (can output CycloneDX)
└─ KSAM (we'll use this!)


FORMAT 2: SPDX ⭐⭐⭐⭐
══════════════════════════════════════════════════════════════════
Developed by: Linux Foundation
Current version: 2.3
Format: JSON, YAML, tag-value
Focus: Licensing, compliance

Strengths:
✅ ISO/IEC standard (ISO/IEC 5962:2021)
✅ Strong licensing information
✅ Government/enterprise preferred
✅ Mature ecosystem

Less ideal for KSAM:
⚠️ More complex structure
⚠️ Heavier focus on licensing vs security


FORMAT 3: SWID Tags ⭐⭐⭐
══════════════════════════════════════════════════════════════════
Developed by: NIST
Format: XML
Focus: Software identification

Less common for containers
⚠️ Primarily for installed software
⚠️ Less tooling support


RECOMMENDATION FOR KSAM:
══════════════════════════════════════════════════════════════════
Use CycloneDX ✅

Why:
├─ Perfect for security/CVE use case
├─ Excellent tooling (Syft + Grype)
├─ Simple JSON structure
├─ Active development
└─ CNCF/Kubernetes ecosystem standard
```

---

## PART 2: FLOW COMPARISON

### **2.1 Current Flow (Trivy-based) - DETAILED**

```
┌─────────────────────────────────────────────────────────────────┐
│                    CURRENT FLOW ANALYSIS                         │
└─────────────────────────────────────────────────────────────────┘

STEP-BY-STEP BREAKDOWN:
══════════════════════════════════════════════════════════════════

Step 1: Pod Created
├─ User: kubectl apply -f pod.yaml
├─ Kubernetes: Creates pod
├─ Pod spec: image: nginx:1.19.0
└─ Time: 0s

Step 2: Pod Watcher detects event
├─ KSAM watches pod creation events
├─ Extract image name: nginx:1.19.0
├─ Check if already scanned (cache lookup)
└─ Time: +0.1s

Step 3: Image Scanner calls Trivy API
├─ Request: trivy-server:4954/scan?image=nginx:1.19.0
├─ Trivy pulls image manifest
├─ Trivy downloads all layers (~500MB for nginx)
├─ Trivy extracts filesystem
├─ Trivy parses package managers (dpkg, rpm, etc.)
├─ Trivy queries vulnerability DB
├─ Trivy returns: 245 vulnerabilities
└─ Time: +30s (HEAVY!)

Step 4: CVE Processor
├─ Receive scan result from Trivy
├─ Filter severity: CRITICAL + HIGH only
├─ Enrich with metadata
├─ Result: 19 critical/high vulnerabilities
└─ Time: +0.5s

Step 5: Create Insights
├─ For each CVE, create Insight object
├─ Save to database (insights table)
├─ Extract affected resources
└─ Time: +1s

Step 6: Auto-trigger Risk Scoring
├─ Insight Manager triggers scoring
├─ Async job queued
└─ Time: +0.1s

Step 7: Risk Scorer
├─ Get all insights for pod
├─ Separate CVE vs Policy violations
├─ Calculate CVE score (V2 formula)
├─ Calculate Policy score
├─ Combine with context multiplier
├─ Save to risk_scores table
└─ Time: +2s

TOTAL TIME: ~34 seconds
BOTTLENECK: Step 3 (Trivy scan) = 88% of time!


RESOURCE USAGE:
══════════════════════════════════════════════════════════════════
Trivy Server (per scan):
├─ CPU: 500m-2000m (varies by image size)
├─ Memory: 1-2Gi (layer extraction)
├─ Network: 500MB-2GB (download layers)
└─ Disk I/O: Heavy (extract/parse)

KSAM Components:
├─ Pod Watcher: 50m CPU, 128Mi RAM
├─ CVE Processor: 100m CPU, 256Mi RAM
├─ Insight Manager: 100m CPU, 256Mi RAM
└─ Risk Scorer: 200m CPU, 512Mi RAM


PROBLEMS:
══════════════════════════════════════════════════════════════════
❌ Tight coupling
├─ KSAM depends on Trivy API availability
├─ Trivy updates can break KSAM
└─ Single point of failure

❌ Heavy resource usage
├─ Download full image layers every time
├─ Parse filesystem every time
├─ High memory usage (layer extraction)
└─ Network bandwidth intensive

❌ Slow
├─ 30s average per image
├─ 100 pods = 50 minutes sequential
└─ Need parallel scanning (complex)

❌ Redundant work
├─ Same image scanned multiple times
├─ Cache helps but limited (24h TTL)
└─ CVE DB updates require re-scanning all images

❌ Hard to extend
├─ Want to add custom CVE source? → Hard
├─ Want custom matching logic? → Can't
└─ Locked into Trivy's behavior
```

### **2.2 Proposed Flow (SBOM-based) - DETAILED**

```
┌─────────────────────────────────────────────────────────────────┐
│                    PROPOSED FLOW ANALYSIS                        │
└─────────────────────────────────────────────────────────────────┘

STEP-BY-STEP BREAKDOWN:
══════════════════════════════════════════════════════════════════

Step 1: Pod Created
├─ User: kubectl apply -f pod.yaml
├─ Kubernetes: Creates pod
├─ Pod spec: image: nginx:1.19.0
└─ Time: 0s

Step 2: Pod Watcher detects event
├─ KSAM watches pod creation events
├─ Extract image name: nginx:1.19.0
├─ Check SBOM cache (PostgreSQL)
└─ Time: +0.1s

Step 3: SBOM Extractor (NEW!)
├─ If SBOM exists in cache → Use it ✅
├─ If not → Generate new SBOM:
│  ├─ Tool: Syft (lightweight scanner)
│  ├─ Syft pulls only manifest (not layers!)
│  ├─ Syft analyzes package managers
│  ├─ Syft generates CycloneDX JSON
│  └─ Result: SBOM (~100KB JSON)
└─ Time: +5-10s (vs 30s with Trivy!)

Step 4: SBOM Uploader (NEW!)
├─ Save SBOM to database (sboms table)
├─ Index by: image name + digest
├─ Store: CycloneDX JSON
└─ Time: +0.2s

Step 5: Core SBOM Processor (NEW!)
├─ Parse SBOM (extract components)
├─ For each component:
│  ├─ Extract: package name, version, purl
│  └─ Prepare for CVE matching
└─ Time: +0.5s

Step 6: CVE Matching Engine (NEW!)
├─ For each package in SBOM:
│  ├─ Query CVE database (Trivy DB or custom)
│  ├─ Match: pkg:deb/debian/openssl@1.1.1d
│  ├─ Find applicable CVEs
│  └─ Apply version constraints
├─ Result: List of matched CVEs
└─ Time: +1s (FAST - just DB queries!)

Step 7: Create Insights
├─ For each matched CVE, create Insight
├─ Same as before
└─ Time: +1s

Step 8: Risk Engine → Scoring
├─ Same as before (use V2 formula)
└─ Time: +2s

TOTAL TIME:
├─ First time: ~10 seconds (generate SBOM)
├─ Cached: ~5 seconds (reuse SBOM)
└─ vs Current: 34 seconds

SPEEDUP: 3-7x faster! ✅


RESOURCE USAGE:
══════════════════════════════════════════════════════════════════
SBOM Extractor (Syft):
├─ CPU: 200m-500m (lightweight)
├─ Memory: 256-512Mi (no layer extraction!)
├─ Network: 10-50MB (only manifest)
└─ Disk I/O: Minimal

SBOM Processor:
├─ CPU: 100m
├─ Memory: 256Mi
└─ Operation: Pure JSON parsing

CVE Matching Engine:
├─ CPU: 200m
├─ Memory: 512Mi
└─ Operation: Database queries only

TOTAL: 60% less resources than Trivy-based ✅


BENEFITS:
══════════════════════════════════════════════════════════════════
✅ Decoupled architecture
├─ SBOM generation independent of CVE matching
├─ Can swap tools easily
├─ Clear boundaries
└─ No tight coupling

✅ Lightweight & Fast
├─ 3-7x faster than Trivy-based
├─ 60% less resource usage
├─ No layer download
└─ Minimal network traffic

✅ Reusable SBOMs
├─ Generate once, use many times
├─ Cache indefinitely (image digest-based)
├─ CVE DB updates → Just re-match (no re-scan!)
└─ Share across teams/tools

✅ Flexible
├─ Easy to add custom CVE sources
├─ Custom matching logic possible
├─ Support multiple SBOM formats
└─ Extensible architecture

✅ Industry standard
├─ Compliance friendly (NTIA, EO 14028)
├─ Portable SBOMs
├─ Tool agnostic
└─ Future-proof
```

### **2.3 Side-by-Side Comparison**

```
┌─────────────────────────────────────────────────────────────────┐
│              CURRENT vs PROPOSED - HEAD TO HEAD                  │
└─────────────────────────────────────────────────────────────────┘

┌────────────────┬──────────────────┬──────────────────────────┐
│ Metric         │ Current (Trivy)  │ Proposed (SBOM)          │
├────────────────┼──────────────────┼──────────────────────────┤
│ Scan Time      │ 30s average      │ 5-10s (first) / 1s (cached)│
│ (first time)   │                  │ 3-7x faster! ✅           │
├────────────────┼──────────────────┼──────────────────────────┤
│ CPU Usage      │ 500-2000m        │ 200-500m                 │
│                │                  │ 60% reduction ✅          │
├────────────────┼──────────────────┼──────────────────────────┤
│ Memory Usage   │ 1-2Gi            │ 256-512Mi                │
│                │                  │ 75% reduction ✅          │
├────────────────┼──────────────────┼──────────────────────────┤
│ Network I/O    │ 500MB-2GB        │ 10-50MB                  │
│                │ (download layers)│ (only manifest)          │
│                │                  │ 95% reduction ✅          │
├────────────────┼──────────────────┼──────────────────────────┤
│ Caching        │ 24h TTL          │ Indefinite (digest-based)│
│                │ (limited)        │ Never expires! ✅         │
├────────────────┼──────────────────┼──────────────────────────┤
│ CVE DB Update  │ Re-scan all      │ Just re-match SBOMs      │
│ Impact         │ images (heavy)   │ (fast!) ✅                │
├────────────────┼──────────────────┼──────────────────────────┤
│ Coupling       │ Tight (Trivy)    │ Loose (tools swappable)  │
│                │                  │ ✅                        │
├────────────────┼──────────────────┼──────────────────────────┤
│ Extensibility  │ ❌ Limited        │ ✅ High                   │
│                │ (locked to Trivy)│ (custom logic easy)      │
├────────────────┼──────────────────┼──────────────────────────┤
│ Standards      │ ⚠️ Tool-specific  │ ✅ Industry standard      │
│ Compliance     │                  │ (SBOM mandates)          │
├────────────────┼──────────────────┼──────────────────────────┤
│ Portability    │ ❌ Locked to Trivy│ ✅ Tool-agnostic SBOMs   │
├────────────────┼──────────────────┼──────────────────────────┤
│ Debugging      │ ⚠️ Black box      │ ✅ Clear (inspect SBOM)  │
│                │ (Trivy internal) │                          │
├────────────────┼──────────────────┼──────────────────────────┤
│ OVERALL        │ ⭐⭐⭐             │ ⭐⭐⭐⭐⭐                    │
│                │ Works but heavy  │ RECOMMENDED ✅            │
└────────────────┴──────────────────┴──────────────────────────┘
```

---

## PART 3: SBOM TOOLING ECOSYSTEM

### **3.1 SBOM Generation Tools**

```
┌─────────────────────────────────────────────────────────────────┐
│                    SBOM GENERATION TOOLS                         │
└─────────────────────────────────────────────────────────────────┘

TOOL 1: Syft ⭐⭐⭐⭐⭐ RECOMMENDED FOR KSAM
══════════════════════════════════════════════════════════════════
Developed by: Anchore
Language: Go
License: Apache 2.0
Current version: 0.98.0

Features:
✅ Fast (~5-10s per image)
✅ CycloneDX & SPDX support
✅ Multiple package ecosystems (OS, npm, pip, go, java, etc.)
✅ Can work with: images, filesystems, archives
✅ Lightweight (no heavy scanning)
✅ Well-maintained (active development)
✅ CLI + Go library
✅ Kubernetes/CNCF ecosystem

Usage:
# CLI
syft nginx:1.19.0 -o cyclonedx-json > sbom.json

# Go library
import "github.com/anchore/syft/syft"

sbom, err := syft.GetSBOM(
    context.Background(),
    "nginx:1.19.0",
    syft.DefaultGetSBOMConfig(),
)

Output size: ~100KB JSON
Time: 5-10 seconds


TOOL 2: Trivy (SBOM mode)
══════════════════════════════════════════════════════════════════
Developed by: Aqua Security
Note: Can use Trivy just for SBOM generation!

Features:
✅ Same Trivy you know
✅ Can output SBOM (CycloneDX, SPDX)
✅ No need for separate tool

Usage:
trivy image --format cyclonedx nginx:1.19.0

Pros:
✅ Already familiar with Trivy
✅ One tool for SBOM + scanning

Cons:
❌ Still heavier than Syft
❌ Still couples you to Trivy
❌ Defeats purpose of decoupling


TOOL 3: Microsoft SBOM Tool
══════════════════════════════════════════════════════════════════
Developed by: Microsoft
Language: .NET
License: MIT

Features:
✅ SPDX focus
✅ Good for Windows containers
⚠️ Less common in K8s/Linux world


TOOL 4: Tern
══════════════════════════════════════════════════════════════════
Developed by: VMware (now part of Anchore)
Language: Python
Note: Less actively maintained


RECOMMENDATION FOR KSAM:
══════════════════════════════════════════════════════════════════
Use Syft ✅

Why:
├─ Lightweight & fast (5-10s)
├─ Perfect CycloneDX support
├─ Go library (easy KSAM integration)
├─ Active development
├─ CNCF/K8s ecosystem standard
└─ NOT Trivy (decouples as desired!)
```

### **3.2 CVE Matching Tools**

```
┌─────────────────────────────────────────────────────────────────┐
│                    CVE MATCHING ENGINES                          │
└─────────────────────────────────────────────────────────────────┘

OPTION 1: Grype ⭐⭐⭐⭐⭐ RECOMMENDED FOR KSAM
══════════════════════════════════════════════════════════════════
Developed by: Anchore (same as Syft!)
Language: Go
License: Apache 2.0
Current version: 0.74.0

Features:
✅ Consumes SBOM (CycloneDX, SPDX, Syft)
✅ Fast CVE matching (<1s)
✅ Multiple vulnerability databases:
   ├─ NVD
   ├─ GitHub Security Advisories
   ├─ Alpine SecDB
   ├─ Debian Security Tracker
   ├─ Red Hat Security Data
   └─ ... (10+ sources!)
✅ Accurate version matching
✅ CLI + Go library
✅ Pairs perfectly with Syft

Usage:
# CLI (from SBOM file)
grype sbom:sbom.json

# Go library
import "github.com/anchore/grype/grype"

matches, err := grype.FindVulnerabilitiesForSBOM(
    context.Background(),
    sbom,
    grype.MatcherConfig{
        Matchers: []string{"all"},
    },
)

Perfect for KSAM! ✅


OPTION 2: Custom CVE Matcher (with Trivy DB)
══════════════════════════════════════════════════════════════════
Build your own matcher using Trivy DB

Features:
✅ Full control
✅ Can reuse Trivy DB (don't throw it away!)
✅ Custom matching logic
⚠️ More work (implement yourself)

Implementation:
1. Parse SBOM (extract packages)
2. Query Trivy DB (BoltDB)
3. Match versions
4. Return CVEs

Code: See Part 1 (Direct DB Access option)


OPTION 3: OSV (Open Source Vulnerabilities)
══════════════════════════════════════════════════════════════════
Developed by: Google
API: https://api.osv.dev/

Features:
✅ REST API (no local DB)
✅ Fast queries
✅ Multiple ecosystems
⚠️ Network dependency
⚠️ Rate limits

Usage:
POST https://api.osv.dev/v1/querybatch
{
  "queries": [
    {
      "package": {"name": "openssl", "ecosystem": "Debian:10"},
      "version": "1.1.1d"
    }
  ]
}


RECOMMENDATION FOR KSAM:
══════════════════════════════════════════════════════════════════
Use Grype ✅

Why:
├─ Pairs perfectly with Syft (same company)
├─ Fast (<1s matching)
├─ Multiple CVE sources (not just NVD)
├─ Go library (easy integration)
├─ Active development
└─ Industry standard
```

Continue to Part 4 with architecture designs...
