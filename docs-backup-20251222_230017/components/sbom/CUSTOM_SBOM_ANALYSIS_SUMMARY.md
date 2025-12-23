# Custom SBOM Analysis & Adjustment Summary

**Date**: 2025-12-12  
**Status**: Analysis Complete - Implementation Started

---

## 📊 PHÂN TÍCH HIỆN TRẠNG

### **Current Implementation (Syft/Grype-based)**

```
Architecture:
Pod → RiskWorker → SBOM Pipeline
                  ├─ Generator (Syft CLI)
                  ├─ Matcher (Grype CLI)
                  └─ Insights

Issues:
❌ External tool dependency (Syft + Grype)
❌ Docker build network issues
❌ Limited control over extraction/matching
❌ Black box behavior
```

### **Target Implementation (Custom Zero-Dependency)**

```
Architecture:
Pod → RiskWorker → Custom SBOM Pipeline
                  ├─ Custom Extractor (YOUR code)
                  ├─ Custom Normalizer (YOUR code)
                  ├─ Custom CVE Matcher (YOUR code)
                  └─ Insights

Benefits:
✅ Zero external tool dependency
✅ Full control over every step
✅ Customize to KSAM needs
✅ No network issues during build
```

---

## 🎯 ĐIỀU CHỈNH GIẢI PHÁP

### **1. Migration Strategy: Hybrid Approach** ✅

```
Phase 1 (Week 1-2): Build Custom Extractor
├─ Implement all parsers
├─ Test with real images
└─ Keep Syft as fallback

Phase 2 (Week 3): Build Custom Normalizer
├─ CycloneDX conversion
└─ PURL generation

Phase 3 (Week 4-5): Build Custom CVE Matcher
├─ Trivy DB reader (just data!)
├─ Version comparator (CRITICAL!)
└─ CVE matching logic

Phase 4 (Week 6): Integration
├─ Update pipeline.go
├─ Add feature flags
└─ Parallel running

Phase 5 (Week 7-8): Production
├─ Gradual rollout
├─ Remove Syft/Grype
└─ Monitor
```

### **2. Key Design Decisions**

1. **Keep Same Interface**
   - `GetOrGenerateSBOM()` → Drop-in replacement
   - `MatchSBOM()` → Drop-in replacement
   - Allows gradual migration

2. **Feature Flags**
   ```go
   KSAM_SBOM_USE_CUSTOM_EXTRACTOR=true/false
   KSAM_SBOM_USE_CUSTOM_MATCHER=true/false
   ```

3. **Trivy DB as Data Source**
   - Use Trivy DB (BoltDB) for CVE data
   - Just read data, NOT use Trivy code
   - Acceptable compromise for zero tool dependency

4. **Priority: Most Common First**
   - dpkg (Debian/Ubuntu) - HIGH
   - apk (Alpine) - HIGH
   - npm (Node.js) - HIGH
   - pip (Python) - MEDIUM
   - go.mod (Go) - MEDIUM
   - rpm (RedHat) - LOW (complex, can add later)

---

## ✅ IMPLEMENTATION PROGRESS

### **Completed Components**

1. **Custom SBOM Extractor** ✅
   - ✅ Core extractor (`pkg/sbom/extractor/extractor.go`)
   - ✅ Filesystem (`pkg/sbom/extractor/filesystem.go`)
   - ✅ DpkgParser (`parsers/dpkg.go`)
   - ✅ ApkParser (`parsers/apk.go`)
   - ✅ NpmParser (`parsers/npm.go`)
   - ✅ PipParser (`parsers/pip.go`)
   - ✅ GoModParser (`parsers/gomod.go`)
   - ⚠️ RpmParser (placeholder - needs Berkeley DB)

2. **Custom SBOM Normalizer** ✅
   - ✅ Normalizer (`pkg/sbom/normalizer/normalizer.go`)
   - ✅ CycloneDX format
   - ✅ PURL generation

### **Pending Components**

3. **Custom CVE Database Manager** ⏳
   - [ ] Trivy DB reader
   - [ ] NVD API client
   - [ ] Database manager

4. **Custom Version Comparator** ⏳
   - [ ] Debian version comparison
   - [ ] RPM version comparison
   - [ ] Semantic versioning

5. **Custom CVE Matcher** ⏳
   - [ ] PURL parser
   - [ ] Matching logic
   - [ ] Enrichment

6. **Pipeline Integration** ⏳
   - [ ] Update pipeline.go
   - [ ] Feature flags
   - [ ] Testing

---

## 📋 ADJUSTED RECOMMENDATIONS

### **For KSAM Project**

```
IMMEDIATE (This Week):
1. Fix Docker build (Syft/Grype) - Get current solution working
2. Deploy current implementation
3. Get CVE scanning operational

PARALLEL (Next 6-8 Weeks):
1. Build custom extractor ✅ (DONE)
2. Build custom normalizer ✅ (DONE)
3. Build custom CVE matcher (Week 4-5)
4. Test thoroughly
5. Compare with Syft/Grype

SWITCH (Week 7-8):
1. Enable custom pipeline
2. Monitor results
3. Remove Syft/Grype when confident
```

### **Why Hybrid?**

- ✅ **Lower Risk**: Proven solution first
- ✅ **Faster Delivery**: Get CVE scanning working now
- ✅ **Validation**: Compare custom vs Syft/Grype
- ✅ **Fallback**: Available if custom has issues
- ✅ **Achieves Goal**: Zero dependency eventually

---

## 🔧 TECHNICAL ADJUSTMENTS

### **1. Dependency Management**

**Added**:
- `github.com/google/go-containerregistry` - Image pulling

**Will Add**:
- `github.com/boltdb/bolt` - Trivy DB reader
- `github.com/hashicorp/go-version` - Semver support

**Removed** (eventually):
- Syft binary
- Grype binary

### **2. Interface Compatibility**

```go
// Same interface for drop-in replacement
type SBOMGenerator interface {
    GetOrGenerateSBOM(ctx context.Context, imageRef string) (*models.SBOM, error)
}

type CVEMatcher interface {
    MatchSBOM(ctx context.Context, sbom *models.SBOM) ([]*models.CVEMatch, error)
}
```

### **3. Feature Flags**

```go
// In pipeline.go
useCustomExtractor := os.Getenv("KSAM_SBOM_USE_CUSTOM_EXTRACTOR") == "true"
useCustomMatcher := os.Getenv("KSAM_SBOM_USE_CUSTOM_MATCHER") == "true"

if useCustomExtractor {
    extractor = custom.NewExtractor()
} else {
    extractor = sbom.NewGenerator(db, syftPath)
}
```

---

## 📊 COMPARISON: Current vs Custom

| Aspect | Current | Custom | Status |
|--------|---------|--------|--------|
| **External Tools** | 2 (Syft+Grype) | 0 ✅ | Target |
| **Code Control** | Partial | Full ✅ | Target |
| **Development** | 3-4 weeks | 6-8 weeks | In Progress |
| **Extractor** | Syft CLI | Custom ✅ | Done |
| **Normalizer** | Syft output | Custom ✅ | Done |
| **CVE Matcher** | Grype CLI | Custom | Pending |
| **Version Compare** | Grype | Custom | Pending |

---

## ⚠️ RISKS & MITIGATION

### **Risk 1: Version Comparison Bugs**

**Impact**: HIGH  
**Mitigation**:
- Extensive test suite
- Compare with Grype during parallel running
- Code review by security expert

### **Risk 2: Missing Packages**

**Impact**: MEDIUM  
**Mitigation**:
- Start with most common parsers
- Log missing packages
- Add more parsers as needed

### **Risk 3: Performance**

**Impact**: MEDIUM  
**Mitigation**:
- Benchmark early
- Optimize hot paths
- Use caching

---

## 🚀 NEXT STEPS

1. **Complete CVE Database Manager** (Week 4-5)
   - Implement Trivy DB reader
   - Implement NVD API client

2. **Implement Version Comparator** (Week 4-5)
   - Debian version logic (CRITICAL!)
   - Semantic versioning

3. **Build CVE Matcher** (Week 5-6)
   - PURL parsing
   - Matching logic

4. **Integrate Pipeline** (Week 6)
   - Update pipeline.go
   - Add feature flags

5. **Testing & Deployment** (Week 7-8)
   - End-to-end testing
   - Production deployment

---

**Status**: Phase 1 Complete (30%), Phase 2 Starting  
**Timeline**: 6-8 weeks total  
**Risk**: Medium (version comparison is complex)


