# Custom SBOM Analysis & Adjustment
## Phân tích và điều chỉnh giải pháp Zero-Dependency

**Date**: 2025-12-12  
**Status**: Analysis Complete - Ready for Implementation

---

## 📊 PHÂN TÍCH HIỆN TRẠNG

### **Current Implementation (Syft/Grype-based)**

```
Current Components:
═══════════════════════════════════════════════════════════════

1. pkg/sbom/generator.go
   ├─ Uses: Syft CLI (external binary)
   ├─ Method: exec.Command("syft", image, "-o", "cyclonedx-json")
   ├─ Output: CycloneDX JSON
   └─ Dependency: Syft binary in PATH

2. pkg/sbom/matcher.go
   ├─ Uses: Grype CLI (external binary)
   ├─ Method: exec.Command("grype", "sbom:sbom.json", "-o", "json")
   ├─ Output: CVE matches
   └─ Dependency: Grype binary in PATH

3. pkg/sbom/pipeline.go
   ├─ Orchestrates: Generator → Matcher → Insights
   ├─ Interface: ProcessImage()
   └─ Status: Working but dependent on external tools

4. pkg/worker/risk_worker.go
   ├─ Integrates: SBOM Pipeline
   ├─ Triggers: On Pod resources
   └─ Status: Integrated
```

### **Issues with Current Approach**

```
❌ External Tool Dependency
├─ Syft binary must be installed
├─ Grype binary must be installed
├─ Docker image includes both (~50MB extra)
└─ Network issues during build (as experienced)

❌ Limited Control
├─ Cannot customize extraction logic
├─ Cannot customize matching logic
├─ Dependent on tool updates
└─ Black box behavior

❌ Resource Overhead
├─ Two separate binaries
├─ CLI invocation overhead
└─ Process spawning cost
```

---

## 🎯 TARGET ARCHITECTURE (Custom Zero-Dependency)

### **Proposed Custom Implementation**

```
Target Components:
═══════════════════════════════════════════════════════════════

1. pkg/sbom/extractor/ (NEW - replaces generator.go)
   ├─ extractor.go: Main orchestrator
   ├─ parsers/dpkg.go: Debian/Ubuntu parser
   ├─ parsers/rpm.go: RedHat/CentOS parser
   ├─ parsers/apk.go: Alpine parser
   ├─ parsers/npm.go: Node.js parser
   ├─ parsers/pip.go: Python parser
   ├─ parsers/gomod.go: Go parser
   └─ filesystem.go: Image filesystem access

2. pkg/sbom/normalizer/ (NEW)
   ├─ normalizer.go: Main normalizer
   ├─ cyclonedx.go: CycloneDX format
   └─ purl.go: PURL generation

3. pkg/cve/database/ (NEW - replaces matcher.go dependency on Grype)
   ├─ manager.go: Database manager
   ├─ trivy_db.go: Trivy DB reader (just data!)
   └─ nvd_api.go: NVD API client (fallback)

4. pkg/cve/matcher/ (NEW - replaces matcher.go)
   ├─ matcher.go: Main CVE matcher
   ├─ version_comparator.go: Version comparison
   ├─ purl_parser.go: PURL parsing
   └─ enricher.go: CVE enrichment

5. pkg/sbom/pipeline.go (UPDATE)
   ├─ Replace Generator with Extractor
   ├─ Replace Matcher with CVEMatcher
   └─ Keep same interface for compatibility
```

---

## 🔄 MIGRATION STRATEGY

### **Recommended: Gradual Migration with Feature Flag**

```
Phase 1: Build Custom Extractor (Week 1-2)
├─ Implement all parsers
├─ Test with real images
├─ Add feature flag: KSAM_SBOM_USE_CUSTOM_EXTRACTOR
└─ Keep Syft as fallback

Phase 2: Build Custom Normalizer (Week 3)
├─ CycloneDX conversion
├─ PURL generation
└─ Test with extractor

Phase 3: Build Custom CVE Matcher (Week 4-5)
├─ Trivy DB reader
├─ Version comparator (CRITICAL!)
├─ CVE matching logic
└─ Test thoroughly

Phase 4: Integration (Week 6)
├─ Update pipeline.go
├─ Add feature flags
├─ Parallel running (compare results)
└─ End-to-end testing

Phase 5: Production (Week 7-8)
├─ Gradual rollout
├─ Monitor vs Syft/Grype
├─ Remove Syft/Grype when confident
└─ Remove feature flags
```

---

## 🛠️ IMPLEMENTATION ADJUSTMENTS

### **Key Design Decisions**

1. **Keep Same Interface**
   - `GetOrGenerateSBOM()` → Same signature
   - `MatchSBOM()` → Same signature
   - Allows drop-in replacement

2. **Feature Flags**
   ```go
   // Environment variables
   KSAM_SBOM_USE_CUSTOM_EXTRACTOR=true/false
   KSAM_SBOM_USE_CUSTOM_MATCHER=true/false
   ```

3. **Trivy DB as Data Source**
   - Use Trivy DB (BoltDB) for CVE data
   - Just read data, NOT use Trivy code
   - Can switch to NVD API later

4. **Version Comparator Priority**
   - Most complex component
   - Needs extensive testing
   - Start with Debian (most common)

---

## 📋 ADJUSTED IMPLEMENTATION PLAN

### **Priority Order**

```
HIGH PRIORITY (Week 1-3):
1. Custom SBOM Extractor
   ├─ dpkg parser (Debian/Ubuntu - most common)
   ├─ apk parser (Alpine - lightweight)
   └─ npm parser (Node.js - common in apps)

2. Custom SBOM Normalizer
   └─ CycloneDX format

MEDIUM PRIORITY (Week 4-5):
3. Custom CVE Database Manager
   ├─ Trivy DB reader
   └─ NVD API client

4. Custom Version Comparator
   ├─ Debian version (CRITICAL!)
   ├─ Semantic versioning
   └─ RPM version

5. Custom CVE Matcher
   └─ Complete matching logic

LOW PRIORITY (Week 6+):
6. Additional parsers
   ├─ rpm parser
   ├─ pip parser
   └─ go.mod parser
```

---

## ⚙️ TECHNICAL ADJUSTMENTS

### **1. Image Access Strategy**

**Current**: Syft handles image pulling internally  
**Custom**: Use `go-containerregistry` library

```go
import "github.com/google/go-containerregistry/pkg/v1/remote"

// Pull image
img, err := remote.Image(ref)
layers, err := img.Layers()
```

### **2. Filesystem Extraction**

**Current**: Syft extracts filesystem  
**Custom**: Extract layers and build virtual filesystem

```go
// Extract each layer
for _, layer := range layers {
    uncompressed, _ := layer.Uncompressed()
    tar.NewReader(uncompressed)
    // Build filesystem tree
}
```

### **3. Package Database Parsing**

**Current**: Syft parses all formats  
**Custom**: Implement parsers for each format

```go
// dpkg: /var/lib/dpkg/status
// apk: /lib/apk/db/installed
// npm: /package-lock.json
```

### **4. CVE Database Access**

**Current**: Grype queries its own DB  
**Custom**: Read Trivy DB directly (BoltDB)

```go
import "github.com/boltdb/bolt"

db, _ := bolt.Open("trivy.db", 0600, &bolt.Options{ReadOnly: true})
// Query vulnerability bucket
```

---

## 🎯 ADJUSTED RECOMMENDATIONS

### **For KSAM Project**

```
RECOMMENDATION: HYBRID APPROACH ✅
═══════════════════════════════════════════════════════════════

Phase 1 (Immediate): Keep Syft/Grype Working
├─ Fix Docker build issues
├─ Deploy current implementation
└─ Get CVE scanning operational

Phase 2 (Parallel - Week 1-6): Build Custom
├─ Develop custom components
├─ Test thoroughly
└─ Compare with Syft/Grype results

Phase 3 (Week 7-8): Switch Over
├─ Enable custom pipeline
├─ Monitor closely
└─ Remove Syft/Grype when stable

BENEFITS:
✅ Lower risk (proven solution first)
✅ Faster initial delivery
✅ Time to build custom properly
✅ Can compare results
✅ Fallback available
```

### **Alternative: Direct Custom Implementation**

If zero dependency is absolute requirement:

```
Week 1-2: Custom Extractor (dpkg, apk, npm)
Week 3: Custom Normalizer
Week 4-5: Custom CVE Matcher + Version Comparator
Week 6: Integration & Testing
Week 7-8: Production Deployment

RISK: Higher (no fallback)
TIMELINE: 6-8 weeks
EFFORT: 3-4 developers
```

---

## 📊 COMPARISON MATRIX

| Aspect | Current (Syft/Grype) | Custom (Zero Dep) | Recommendation |
|--------|---------------------|-------------------|----------------|
| **External Tools** | 2 binaries | 0 ✅ | Custom |
| **Development Time** | 3-4 weeks | 6-8 weeks | Syft/Grype first |
| **Control** | Limited | Full ✅ | Custom |
| **Risk** | Low | Medium | Hybrid |
| **Maintenance** | Low | High | Consider |
| **Performance** | Optimized | Can optimize ✅ | Custom |
| **Independence** | Partial | Complete ✅ | Custom |

---

## ✅ FINAL RECOMMENDATION

### **For KSAM: Hybrid Approach**

```
IMMEDIATE (This Week):
1. Fix Docker build (Syft/Grype)
2. Deploy current implementation
3. Get CVE scanning working

PARALLEL (Next 6-8 Weeks):
1. Build custom extractor
2. Build custom normalizer
3. Build custom CVE matcher
4. Test thoroughly
5. Compare with Syft/Grype

SWITCH (Week 7-8):
1. Enable custom pipeline
2. Monitor results
3. Remove Syft/Grype when confident
```

**Why Hybrid?**
- ✅ Lower risk (proven solution first)
- ✅ Faster time-to-value
- ✅ Can compare and validate
- ✅ Fallback available
- ✅ Achieves zero dependency goal

---

**Status**: Ready to begin custom implementation  
**Next Step**: Start with Custom SBOM Extractor (Phase 1)


