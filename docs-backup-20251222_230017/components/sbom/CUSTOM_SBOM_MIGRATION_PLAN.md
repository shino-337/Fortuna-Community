# Custom SBOM Migration Plan
## From Syft/Grype to Zero-Dependency Custom Implementation

**Date**: 2025-12-12  
**Status**: Planning Phase

---

## 📋 CURRENT STATE ANALYSIS

### **Current Implementation (Syft/Grype-based)**

```
Current Architecture:
═══════════════════════════════════════════════════════════════

Pod Created
  ↓
RiskWorker detects Pod
  ↓
SBOM Pipeline:
  1. SBOM Generator (pkg/sbom/generator.go)
     └─ Calls Syft CLI: syft image -o cyclonedx-json
  2. SBOM Matcher (pkg/sbom/matcher.go)
     └─ Calls Grype CLI: grype sbom:sbom.json -o json
  3. Create Insights
  4. Risk Scoring

Dependencies:
├─ Syft binary (external tool)
├─ Grype binary (external tool)
└─ Docker image includes both
```

### **Target Implementation (Custom Zero-Dependency)**

```
Target Architecture:
═══════════════════════════════════════════════════════════════

Pod Created
  ↓
RiskWorker detects Pod
  ↓
Custom SBOM Pipeline:
  1. Custom SBOM Extractor (pkg/sbom/extractor/)
     └─ YOUR code: Parse dpkg/rpm/apk/npm/pip/go
  2. Custom SBOM Normalizer (pkg/sbom/normalizer/)
     └─ YOUR code: Convert to CycloneDX format
  3. Custom CVE Matcher (pkg/cve/matcher/)
     └─ YOUR code: Query Trivy DB + version comparison
  4. Create Insights
  5. Risk Scoring

Dependencies:
├─ Trivy DB (just data, ~500MB)
├─ Standard Go libraries only
└─ NO external tools!
```

---

## 🔄 MIGRATION STRATEGY

### **Option 1: Big Bang Migration** ⚠️ Risky

```
Week 1-2: Build all custom components
Week 3: Test thoroughly
Week 4: Switch over (remove Syft/Grype)
```

**Pros**: Clean cut, no dual maintenance  
**Cons**: High risk, no fallback

### **Option 2: Gradual Migration** ✅ **RECOMMENDED**

```
Phase 1 (Week 1-2): Build Custom Extractor
├─ Implement dpkg/rpm/apk/npm/pip/go parsers
├─ Test with real images
└─ Keep Syft as fallback

Phase 2 (Week 3): Build Custom Normalizer
├─ CycloneDX conversion
├─ PURL generation
└─ Test with extractor

Phase 3 (Week 4-5): Build Custom CVE Matcher
├─ Trivy DB reader
├─ Version comparator (CRITICAL!)
└─ CVE matching logic

Phase 4 (Week 6): Integration & Testing
├─ End-to-end pipeline
├─ Performance testing
└─ Bug fixes

Phase 5 (Week 7-8): Production Deployment
├─ Gradual rollout
├─ Monitor vs Syft/Grype
└─ Remove Syft/Grype when confident
```

**Pros**: Lower risk, can compare results, fallback available  
**Cons**: Dual maintenance during transition

---

## 🛠️ IMPLEMENTATION PLAN

### **Phase 1: Custom SBOM Extractor**

**Location**: `pkg/sbom/extractor/`

**Components**:
1. `extractor.go` - Main orchestrator
2. `parsers/dpkg.go` - Debian/Ubuntu parser
3. `parsers/rpm.go` - RedHat/CentOS parser
4. `parsers/apk.go` - Alpine parser
5. `parsers/npm.go` - Node.js parser
6. `parsers/pip.go` - Python parser
7. `parsers/gomod.go` - Go parser
8. `filesystem.go` - Image filesystem access

**Dependencies**:
- `github.com/google/go-containerregistry` - Image pulling
- Standard Go libraries

**Timeline**: 10-12 days

### **Phase 2: Custom SBOM Normalizer**

**Location**: `pkg/sbom/normalizer/`

**Components**:
1. `normalizer.go` - Main normalizer
2. `cyclonedx.go` - CycloneDX format conversion
3. `purl.go` - PURL generation

**Timeline**: 3-4 days

### **Phase 3: Custom CVE Database Manager**

**Location**: `pkg/cve/database/`

**Components**:
1. `manager.go` - Main database manager
2. `trivy_db.go` - Trivy DB reader (just data!)
3. `nvd_api.go` - NVD API client (fallback)

**Dependencies**:
- `github.com/boltdb/bolt` - Read Trivy DB (BoltDB format)
- Standard Go libraries

**Timeline**: 3-4 days

### **Phase 4: Custom Version Comparator** ⚠️ **CRITICAL**

**Location**: `pkg/cve/matcher/`

**Components**:
1. `version_comparator.go` - Main comparator
2. `debian.go` - Debian version comparison
3. `rpm.go` - RPM version comparison
4. `semver.go` - Semantic versioning

**Dependencies**:
- `github.com/hashicorp/go-version` - Semver support
- Standard Go libraries

**Timeline**: 7-10 days (most complex!)

### **Phase 5: Custom CVE Matcher**

**Location**: `pkg/cve/matcher/`

**Components**:
1. `matcher.go` - Main matcher
2. `purl_parser.go` - PURL parsing
3. `enricher.go` - CVE enrichment

**Timeline**: 5-7 days

### **Phase 6: Pipeline Integration**

**Location**: `pkg/sbom/pipeline.go` (update existing)

**Changes**:
- Replace `Generator` (Syft) with `Extractor` (custom)
- Replace `Matcher` (Grype) with custom `CVEMatcher`
- Keep same interface for compatibility

**Timeline**: 3-4 days

---

## 📊 COMPARISON: Current vs Custom

| Aspect | Current (Syft/Grype) | Custom (Zero Dep) |
|--------|---------------------|-------------------|
| **External Tools** | 2 (Syft + Grype) | 0 ✅ |
| **Code Control** | Partial | Full ✅ |
| **Customization** | Limited | Unlimited ✅ |
| **Development Time** | 3-4 weeks | 6-8 weeks |
| **Maintenance** | Low (tools maintained) | High (you maintain) |
| **Bug Risk** | Low (battle-tested) | Medium (version comparison) |
| **Performance** | Optimized | Can optimize for KSAM ✅ |
| **Independence** | Partial | Complete ✅ |

---

## 🎯 RECOMMENDED APPROACH

### **Hybrid Strategy** (Best of Both Worlds)

```
Phase 1: Build Custom Components (6-8 weeks)
├─ Develop all custom components
├─ Test thoroughly
└─ Keep Syft/Grype as fallback

Phase 2: Parallel Running (2-4 weeks)
├─ Run both pipelines
├─ Compare results
├─ Fix custom implementation bugs
└─ Build confidence

Phase 3: Switch Over (1-2 weeks)
├─ Enable custom pipeline
├─ Monitor closely
└─ Remove Syft/Grype when stable

Phase 4: Optimization (Ongoing)
├─ Performance tuning
├─ Add more package managers
└─ Enhance version comparison
```

---

## 📝 IMPLEMENTATION CHECKLIST

### **Week 1-2: SBOM Extractor**
- [ ] Create `pkg/sbom/extractor/` package
- [ ] Implement image pulling (go-containerregistry)
- [ ] Implement filesystem access
- [ ] Implement dpkg parser
- [ ] Implement apk parser
- [ ] Implement rpm parser
- [ ] Implement npm parser
- [ ] Implement pip parser
- [ ] Implement go.mod parser
- [ ] Add deduplication logic
- [ ] Unit tests for each parser
- [ ] Integration tests with real images

### **Week 3: SBOM Normalizer**
- [ ] Create `pkg/sbom/normalizer/` package
- [ ] Implement CycloneDX format
- [ ] Implement PURL generation
- [ ] Add validation
- [ ] Unit tests

### **Week 4-5: CVE Database & Version Comparator**
- [ ] Create `pkg/cve/database/` package
- [ ] Implement Trivy DB reader
- [ ] Implement NVD API client
- [ ] Create `pkg/cve/matcher/` package
- [ ] Implement Debian version comparison
- [ ] Implement RPM version comparison
- [ ] Implement semantic versioning
- [ ] Comprehensive testing (CRITICAL!)

### **Week 6: CVE Matcher**
- [ ] Implement PURL parser
- [ ] Implement CVE matching logic
- [ ] Implement enrichment
- [ ] Add filtering
- [ ] Integration tests

### **Week 7-8: Integration & Deployment**
- [ ] Update `pkg/sbom/pipeline.go`
- [ ] Update `pkg/worker/risk_worker.go`
- [ ] End-to-end testing
- [ ] Performance benchmarking
- [ ] Bug fixes
- [ ] Production deployment
- [ ] Monitoring setup

---

## ⚠️ RISKS & MITIGATION

### **Risk 1: Version Comparison Bugs**

**Impact**: HIGH - Wrong CVE matches = false positives/negatives

**Mitigation**:
- Extensive test suite with known CVEs
- Compare results with Grype during parallel running
- Code review by security expert
- Gradual rollout with monitoring

### **Risk 2: Missing Package Managers**

**Impact**: MEDIUM - Some packages not detected

**Mitigation**:
- Start with most common (dpkg, apk, rpm, npm, pip)
- Add more as needed
- Log missing packages for analysis

### **Risk 3: Performance Issues**

**Impact**: MEDIUM - Slower than Syft/Grype

**Mitigation**:
- Benchmark early and often
- Optimize hot paths
- Use caching aggressively
- Parallel processing where possible

### **Risk 4: Timeline Overrun**

**Impact**: MEDIUM - Delays other features

**Mitigation**:
- Keep Syft/Grype as fallback
- Prioritize critical components first
- Incremental delivery (extractor → normalizer → matcher)

---

## 🚀 NEXT STEPS

1. **Review & Approve Plan** - Confirm approach and timeline
2. **Allocate Resources** - 3-4 developers + 1 security expert
3. **Start Phase 1** - Begin with SBOM Extractor
4. **Set Up Testing** - Create test image library
5. **Parallel Development** - Keep Syft/Grype working during transition

---

**Status**: Ready to begin implementation  
**Estimated Completion**: 6-8 weeks from start


