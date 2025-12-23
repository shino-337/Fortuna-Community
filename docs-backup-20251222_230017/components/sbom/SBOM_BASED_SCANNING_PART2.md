# SBOM-BASED SCANNING - PART 2
## Architecture Designs & Implementation Guide

**Continuation from Part 1**

---

## PART 4: ARCHITECTURE DESIGNS

### **4.1 Architecture Option 1: Full SBOM Pipeline** ⭐⭐⭐⭐⭐ **RECOMMENDED**

```
┌─────────────────────────────────────────────────────────────────┐
│              FULL SBOM PIPELINE ARCHITECTURE                     │
└─────────────────────────────────────────────────────────────────┘

COMPONENTS:
══════════════════════════════════════════════════════════════════

┌────────────────────────────────────────────────────────────────┐
│ Kubernetes Cluster                                              │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ 1. Pod Watcher                                            │ │
│  │    Watch pods → Extract image name → Trigger pipeline    │ │
│  └────────────────────┬─────────────────────────────────────┘ │
│                       │                                         │
│                       ▼                                         │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ 2. SBOM Cache Lookup                                      │ │
│  │    Query: sboms table                                     │ │
│  │    Key: image_digest (SHA256)                            │ │
│  │    ├─ Hit? → Use cached SBOM ✅                          │ │
│  │    └─ Miss? → Generate new SBOM                          │ │
│  └────────────────────┬─────────────────────────────────────┘ │
│                       │                                         │
│           ┌───────────┴───────────┐                            │
│           │                       │                            │
│      [Cache Hit]            [Cache Miss]                       │
│           │                       │                            │
│           │                       ▼                            │
│           │  ┌──────────────────────────────────────────────┐ │
│           │  │ 3. SBOM Generator (Syft)                      │ │
│           │  │    ├─ Pull image manifest                     │ │
│           │  │    ├─ Analyze package managers               │ │
│           │  │    ├─ Generate CycloneDX JSON               │ │
│           │  │    └─ Time: 5-10s                            │ │
│           │  └────────────────┬─────────────────────────────┘ │
│           │                   │                                │
│           │                   ▼                                │
│           │  ┌──────────────────────────────────────────────┐ │
│           │  │ 4. SBOM Storage                               │ │
│           │  │    ├─ Save to database (sboms table)         │ │
│           │  │    ├─ Index by: image_digest                 │ │
│           │  │    └─ Store: CycloneDX JSON                  │ │
│           │  └────────────────┬─────────────────────────────┘ │
│           │                   │                                │
│           └───────────────────┤                                │
│                               ▼                                │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ 5. SBOM Parser                                            │ │
│  │    ├─ Parse CycloneDX JSON                               │ │
│  │    ├─ Extract components (packages)                      │ │
│  │    └─ Prepare for matching                               │ │
│  └────────────────────┬─────────────────────────────────────┘ │
│                       │                                         │
│                       ▼                                         │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ 6. CVE Matching Engine (Grype)                           │ │
│  │    ├─ For each package in SBOM:                          │ │
│  │    │  ├─ Query Grype DB                                  │ │
│  │    │  ├─ Match: purl + version                           │ │
│  │    │  └─ Get applicable CVEs                             │ │
│  │    └─ Time: <1s (fast DB queries)                       │ │
│  └────────────────────┬─────────────────────────────────────┘ │
│                       │                                         │
│                       ▼                                         │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ 7. CVE Enrichment                                         │ │
│  │    ├─ Add CVSS scores                                    │ │
│  │    ├─ Add exploit info                                   │ │
│  │    ├─ Add fix versions                                   │ │
│  │    └─ Filter by severity (CRITICAL+HIGH)                │ │
│  └────────────────────┬─────────────────────────────────────┘ │
│                       │                                         │
│                       ▼                                         │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ 8. Insight Creator                                        │ │
│  │    ├─ For each CVE, create Insight                       │ │
│  │    ├─ Save to insights table                             │ │
│  │    └─ Link to pod/resource                               │ │
│  └────────────────────┬─────────────────────────────────────┘ │
│                       │                                         │
│                       ▼                                         │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ 9. Risk Engine (V2)                                       │ │
│  │    ├─ Calculate CVE score                                │ │
│  │    ├─ Calculate Policy score                             │ │
│  │    ├─ Apply context multiplier                           │ │
│  │    └─ Save to risk_scores table                          │ │
│  └────────────────────┬─────────────────────────────────────┘ │
│                       │                                         │
│                       ▼                                         │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ 10. Dashboard                                             │ │
│  │     ├─ Display risk scores                               │ │
│  │     ├─ Show CVE details                                  │ │
│  │     └─ Remediation recommendations                       │ │
│  └──────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────┘


TIMING BREAKDOWN:
══════════════════════════════════════════════════════════════════
Step 1: Pod Watcher              0.1s
Step 2: Cache Lookup             0.1s
Step 3: SBOM Generation          5-10s (first time) / 0s (cached)
Step 4: SBOM Storage             0.2s
Step 5: SBOM Parsing             0.5s
Step 6: CVE Matching             1s
Step 7: CVE Enrichment           0.5s
Step 8: Insight Creation         1s
Step 9: Risk Scoring             2s
───────────────────────────────────────
TOTAL (first time):              10-15s ✅
TOTAL (cached SBOM):             5s ✅

vs Current Trivy-based:          34s
Speedup: 2.3-6.8x faster!


RESOURCE USAGE:
══════════════════════════════════════════════════════════════════
SBOM Generator (Syft):
├─ CPU: 200-500m
├─ Memory: 256-512Mi
└─ Replicas: 2-4 (autoscale)

SBOM Parser:
├─ CPU: 100m
├─ Memory: 256Mi
└─ Part of Risk Engine

CVE Matching Engine (Grype):
├─ CPU: 200m
├─ Memory: 512Mi
├─ Grype DB: 1Gi (PVC)
└─ Replicas: 2-4 (autoscale)

Total: ~1 CPU, 1.5Gi RAM (vs 2.5 CPU, 3.5Gi with Trivy!)
Savings: 60% CPU, 57% RAM ✅
```

### **4.2 Database Schema**

```sql
-- ════════════════════════════════════════════════════════════════
-- SBOM-BASED SCHEMA
-- ════════════════════════════════════════════════════════════════

-- Table 1: SBOMs (persistent cache)
-- ────────────────────────────────────────────────────────────────
CREATE TABLE sboms (
    id SERIAL PRIMARY KEY,
    
    -- Image identification
    image_name VARCHAR(255) NOT NULL,
    image_tag VARCHAR(255) NOT NULL,
    image_digest VARCHAR(255) NOT NULL UNIQUE,  -- SHA256, immutable!
    
    -- SBOM content
    sbom_format VARCHAR(50) NOT NULL DEFAULT 'cyclonedx-json',
    sbom_content JSONB NOT NULL,  -- CycloneDX JSON
    
    -- Component summary
    component_count INTEGER NOT NULL,
    os_packages INTEGER,
    language_packages INTEGER,
    
    -- Generation metadata
    generator VARCHAR(100) DEFAULT 'syft',
    generator_version VARCHAR(50),
    generated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    -- Caching
    last_used_at TIMESTAMP NOT NULL DEFAULT NOW(),
    use_count INTEGER DEFAULT 1,
    
    -- Indexing
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_sboms_image_digest ON sboms(image_digest);
CREATE INDEX idx_sboms_image_name_tag ON sboms(image_name, image_tag);
CREATE INDEX idx_sboms_last_used ON sboms(last_used_at);


-- Table 2: SBOM Components (extracted for fast querying)
-- ────────────────────────────────────────────────────────────────
CREATE TABLE sbom_components (
    id SERIAL PRIMARY KEY,
    sbom_id INTEGER NOT NULL REFERENCES sboms(id) ON DELETE CASCADE,
    
    -- Component identification
    component_type VARCHAR(50) NOT NULL,  -- library, application, os
    component_name VARCHAR(255) NOT NULL,
    component_version VARCHAR(255) NOT NULL,
    purl VARCHAR(512),  -- Package URL (standard)
    
    -- Additional info
    licenses JSONB,
    supplier VARCHAR(255),
    
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_sbom_components_sbom_id ON sbom_components(sbom_id);
CREATE INDEX idx_sbom_components_purl ON sbom_components(purl);
CREATE INDEX idx_sbom_components_name_version 
    ON sbom_components(component_name, component_version);


-- Table 3: CVE Matches (cached matches)
-- ────────────────────────────────────────────────────────────────
CREATE TABLE cve_matches (
    id SERIAL PRIMARY KEY,
    sbom_id INTEGER NOT NULL REFERENCES sboms(id) ON DELETE CASCADE,
    component_id INTEGER NOT NULL REFERENCES sbom_components(id) ON DELETE CASCADE,
    
    -- CVE information
    cve_id VARCHAR(20) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    cvss_score DECIMAL(3,1),
    
    -- Fix information
    fixed_version VARCHAR(255),
    
    -- Metadata
    matched_at TIMESTAMP NOT NULL DEFAULT NOW(),
    matcher VARCHAR(50) DEFAULT 'grype',
    
    -- Freshness tracking
    db_version VARCHAR(50),  -- Grype DB version used
    needs_recheck BOOLEAN DEFAULT FALSE
);

-- Indexes
CREATE INDEX idx_cve_matches_sbom_id ON cve_matches(sbom_id);
CREATE INDEX idx_cve_matches_cve_id ON cve_matches(cve_id);
CREATE INDEX idx_cve_matches_severity ON cve_matches(severity);
CREATE INDEX idx_cve_matches_needs_recheck ON cve_matches(needs_recheck);


-- Table 4: Modified insights table (add SBOM reference)
-- ────────────────────────────────────────────────────────────────
ALTER TABLE insights ADD COLUMN sbom_id INTEGER REFERENCES sboms(id);
ALTER TABLE insights ADD COLUMN cve_match_id INTEGER REFERENCES cve_matches(id);

CREATE INDEX idx_insights_sbom_id ON insights(sbom_id);


-- Example Queries:
-- ────────────────────────────────────────────────────────────────

-- Get SBOM for image
SELECT sbom_content 
FROM sboms 
WHERE image_digest = 'sha256:abc123...';

-- Get all vulnerabilities for an SBOM
SELECT cm.cve_id, cm.severity, cm.cvss_score, 
       sc.component_name, sc.component_version
FROM cve_matches cm
JOIN sbom_components sc ON cm.component_id = sc.id
WHERE cm.sbom_id = 123
  AND cm.severity IN ('CRITICAL', 'HIGH')
ORDER BY cm.cvss_score DESC;

-- Find images affected by specific CVE
SELECT DISTINCT s.image_name, s.image_tag
FROM sboms s
JOIN cve_matches cm ON s.id = cm.sbom_id
WHERE cm.cve_id = 'CVE-2021-44228';

-- Re-match outdated SBOMs (when CVE DB updates)
SELECT sbom_id, COUNT(*) as match_count
FROM cve_matches
WHERE needs_recheck = TRUE
GROUP BY sbom_id;
```

### **4.3 Implementation Code**

```go
// ════════════════════════════════════════════════════════════════
// FILE: pkg/sbom/generator.go
// PURPOSE: SBOM generation using Syft
// ════════════════════════════════════════════════════════════════

package sbom

import (
    "context"
    "encoding/json"
    
    "github.com/anchore/syft/syft"
    "github.com/anchore/syft/syft/pkg/cataloger"
    "github.com/anchore/syft/syft/source"
)

type Generator struct {
    cache *SBOMCache
}

func NewGenerator(cache *SBOMCache) *Generator {
    return &Generator{cache: cache}
}

func (g *Generator) GetOrGenerateSBOM(
    ctx context.Context,
    imageName string,
) (*SBOM, error) {
    // 1. Try cache first
    digest, err := g.getImageDigest(imageName)
    if err != nil {
        return nil, err
    }
    
    cachedSBOM, err := g.cache.Get(digest)
    if err == nil {
        log.Infof("SBOM cache hit for %s", imageName)
        g.cache.UpdateLastUsed(digest)
        return cachedSBOM, nil
    }
    
    log.Infof("SBOM cache miss, generating for %s", imageName)
    
    // 2. Generate new SBOM with Syft
    sbom, err := g.generateSBOM(ctx, imageName)
    if err != nil {
        return nil, err
    }
    
    // 3. Cache for future use
    if err := g.cache.Save(digest, sbom); err != nil {
        log.Warnf("Failed to cache SBOM: %v", err)
    }
    
    return sbom, nil
}

func (g *Generator) generateSBOM(
    ctx context.Context,
    imageName string,
) (*SBOM, error) {
    // Configure Syft
    src, cleanup, err := source.New(
        imageName,
        nil,  // registry options
        []string{"all-layers"},
    )
    if err != nil {
        return nil, err
    }
    defer cleanup()
    
    // Run Syft cataloger
    catalog, relationships, distro, err := syft.CatalogPackages(
        src,
        cataloger.DefaultConfig(),
    )
    if err != nil {
        return nil, err
    }
    
    // Convert to CycloneDX
    cyclonedx := g.convertToCycloneDX(catalog, relationships, distro)
    
    sbom := &SBOM{
        ImageName:      imageName,
        Format:         "cyclonedx-json",
        Content:        cyclonedx,
        ComponentCount: len(catalog.Artifacts()),
        GeneratedAt:    time.Now(),
    }
    
    return sbom, nil
}


// ════════════════════════════════════════════════════════════════
// FILE: pkg/sbom/matcher.go
// PURPOSE: CVE matching using Grype
// ════════════════════════════════════════════════════════════════

package sbom

import (
    "context"
    
    "github.com/anchore/grype/grype"
    "github.com/anchore/grype/grype/matcher"
    "github.com/anchore/grype/grype/store"
)

type CVEMatcher struct {
    db     *store.Store
    config matcher.Config
}

func NewCVEMatcher(dbPath string) (*CVEMatcher, error) {
    // Load Grype vulnerability database
    db, err := store.NewFromDir(dbPath, true)
    if err != nil {
        return nil, err
    }
    
    return &CVEMatcher{
        db: db,
        config: matcher.Config{
            Matchers: matcher.GetDefaultMatchers(),
        },
    }, nil
}

func (cm *CVEMatcher) FindVulnerabilities(
    ctx context.Context,
    sbom *SBOM,
) ([]Vulnerability, error) {
    // Parse SBOM
    packages, err := cm.extractPackages(sbom)
    if err != nil {
        return nil, err
    }
    
    log.Infof("Matching %d packages against CVE database", len(packages))
    
    // Match against Grype DB
    matches := matcher.FindMatches(
        cm.db,
        packages,
        cm.config,
    )
    
    log.Infof("Found %d vulnerability matches", len(matches.Sorted()))
    
    // Convert to KSAM vulnerability format
    vulnerabilities := make([]Vulnerability, 0)
    for _, match := range matches.Sorted() {
        vuln := Vulnerability{
            CVEID:            match.Vulnerability.ID,
            PackageName:      match.Package.Name,
            PackageVersion:   match.Package.Version,
            Severity:         match.Vulnerability.Severity,
            CVSSScore:        match.Vulnerability.CVSS.BaseScore,
            FixedVersion:     match.Vulnerability.Fix.Versions[0],
            Description:      match.Vulnerability.Description,
        }
        
        vulnerabilities = append(vulnerabilities, vuln)
    }
    
    return vulnerabilities, nil
}


// ════════════════════════════════════════════════════════════════
// FILE: pkg/sbom/pipeline.go
// PURPOSE: Main SBOM processing pipeline
// ════════════════════════════════════════════════════════════════

package sbom

import (
    "context"
)

type Pipeline struct {
    generator     *Generator
    matcher       *CVEMatcher
    insightMgr    *InsightManager
    riskEngine    *RiskEngine
}

func NewPipeline(
    generator *Generator,
    matcher *CVEMatcher,
    insightMgr *InsightManager,
    riskEngine *RiskEngine,
) *Pipeline {
    return &Pipeline{
        generator:  generator,
        matcher:    matcher,
        insightMgr: insightMgr,
        riskEngine: riskEngine,
    }
}

func (p *Pipeline) ProcessImage(
    ctx context.Context,
    imageName string,
    resourceUID string,
) error {
    log.Infof("Processing image: %s", imageName)
    
    // Step 1: Get or generate SBOM (5-10s first time, <1s cached)
    sbom, err := p.generator.GetOrGenerateSBOM(ctx, imageName)
    if err != nil {
        return fmt.Errorf("SBOM generation failed: %w", err)
    }
    
    log.Infof("SBOM contains %d components", sbom.ComponentCount)
    
    // Step 2: Match CVEs (<1s)
    vulnerabilities, err := p.matcher.FindVulnerabilities(ctx, sbom)
    if err != nil {
        return fmt.Errorf("CVE matching failed: %w", err)
    }
    
    log.Infof("Found %d vulnerabilities", len(vulnerabilities))
    
    // Step 3: Filter by severity
    filtered := p.filterBySeverity(vulnerabilities, []string{"CRITICAL", "HIGH"})
    
    log.Infof("After filtering: %d critical/high vulnerabilities", len(filtered))
    
    // Step 4: Create insights
    for _, vuln := range filtered {
        insight := &Insight{
            Type:             "vulnerability",
            ResourceUID:      resourceUID,
            Severity:         vuln.Severity,
            CVEID:            vuln.CVEID,
            PackageName:      vuln.PackageName,
            InstalledVersion: vuln.PackageVersion,
            FixedVersion:     vuln.FixedVersion,
            CVSSScore:        vuln.CVSSScore,
        }
        
        if err := p.insightMgr.CreateInsight(ctx, insight); err != nil {
            log.Errorf("Failed to create insight: %v", err)
        }
    }
    
    // Step 5: Trigger risk scoring (async)
    go p.riskEngine.CalculateScore(resourceUID)
    
    log.Infof("Pipeline completed for %s", imageName)
    return nil
}

func (p *Pipeline) filterBySeverity(
    vulns []Vulnerability,
    severities []string,
) []Vulnerability {
    filtered := make([]Vulnerability, 0)
    for _, vuln := range vulns {
        for _, sev := range severities {
            if vuln.Severity == sev {
                filtered = append(filtered, vuln)
                break
            }
        }
    }
    return filtered
}
```

Continue to Part 5 with deployment and migration...
