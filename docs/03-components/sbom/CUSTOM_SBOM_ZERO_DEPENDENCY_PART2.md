# CUSTOM SBOM-BASED SCANNING - PART 2
## SBOM Normalizer & CVE Matching Engine

**Continuation from Part 1**

---

## PART 2: SBOM NORMALIZER

### **2.1 CycloneDX-like Format**

```go
// ════════════════════════════════════════════════════════════════
// FILE: pkg/sbom/normalizer/normalizer.go
// PURPOSE: Normalize raw SBOM to CycloneDX-like format
// ════════════════════════════════════════════════════════════════

package normalizer

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
)

// ────────────────────────────────────────────────────────────────
// CYCLONEDX-LIKE FORMAT
// ────────────────────────────────────────────────────────────────

type NormalizedSBOM struct {
    BOMFormat   string      `json:"bomFormat"`
    SpecVersion string      `json:"specVersion"`
    Version     int         `json:"version"`
    Metadata    Metadata    `json:"metadata"`
    Components  []Component `json:"components"`
}

type Metadata struct {
    Timestamp string    `json:"timestamp"`
    Component Component `json:"component"`
}

type Component struct {
    Type    string `json:"type"`
    Name    string `json:"name"`
    Version string `json:"version"`
    PURL    string `json:"purl"`  // Package URL (universal identifier)
    Hashes  []Hash `json:"hashes,omitempty"`
}

type Hash struct {
    Alg     string `json:"alg"`
    Content string `json:"content"`
}


// ────────────────────────────────────────────────────────────────
// NORMALIZER
// ────────────────────────────────────────────────────────────────

type Normalizer struct{}

func NewNormalizer() *Normalizer {
    return &Normalizer{}
}

func (n *Normalizer) Normalize(raw *RawSBOM) (*NormalizedSBOM, error) {
    log.Infof("Normalizing SBOM for image: %s", raw.ImageName)
    
    // Convert packages to components
    components := make([]Component, 0)
    
    for _, pkg := range raw.Packages {
        component := Component{
            Type:    "library",
            Name:    pkg.Name,
            Version: pkg.Version,
            PURL:    n.generatePURL(pkg, raw.OS),
        }
        
        components = append(components, component)
    }
    
    sbom := &NormalizedSBOM{
        BOMFormat:   "CycloneDX",
        SpecVersion: "1.4",
        Version:     1,
        Metadata: Metadata{
            Timestamp: raw.ExtractedAt.Format(time.RFC3339),
            Component: Component{
                Type:    "container",
                Name:    raw.ImageName,
                Version: "latest",  // Could extract from tag
            },
        },
        Components: components,
    }
    
    log.Infof("Normalized %d components", len(components))
    return sbom, nil
}


// ────────────────────────────────────────────────────────────────
// PURL GENERATION (Package URL)
// ────────────────────────────────────────────────────────────────

func (n *Normalizer) generatePURL(pkg Package, os OSInfo) string {
    // PURL spec: pkg:<type>/<namespace>/<name>@<version>
    // Examples:
    // - pkg:deb/debian/openssl@1.1.1d
    // - pkg:npm/lodash@4.17.21
    // - pkg:pypi/django@3.2.5
    // - pkg:golang/github.com/gin-gonic/gin@v1.7.0
    
    switch pkg.Type {
    case "deb":
        // pkg:deb/debian/openssl@1.1.1d
        return fmt.Sprintf("pkg:deb/%s/%s@%s", os.Name, pkg.Name, pkg.Version)
        
    case "apk":
        // pkg:apk/alpine/openssl@1.1.1g-r0
        return fmt.Sprintf("pkg:apk/alpine/%s@%s", pkg.Name, pkg.Version)
        
    case "rpm":
        // pkg:rpm/centos/openssl@1.1.1k-5.el8
        return fmt.Sprintf("pkg:rpm/%s/%s@%s", os.Name, pkg.Name, pkg.Version)
        
    case "npm":
        // pkg:npm/lodash@4.17.21
        return fmt.Sprintf("pkg:npm/%s@%s", pkg.Name, pkg.Version)
        
    case "pypi":
        // pkg:pypi/django@3.2.5
        return fmt.Sprintf("pkg:pypi/%s@%s", pkg.Name, pkg.Version)
        
    case "go":
        // pkg:golang/github.com/gin-gonic/gin@v1.7.0
        return fmt.Sprintf("pkg:golang/%s@%s", pkg.Name, pkg.Version)
        
    case "maven":
        // pkg:maven/org.springframework/spring-core@5.3.9
        // Note: Maven needs groupId:artifactId format
        return fmt.Sprintf("pkg:maven/%s@%s", pkg.Name, pkg.Version)
        
    default:
        // Generic format
        return fmt.Sprintf("pkg:generic/%s@%s", pkg.Name, pkg.Version)
    }
}
```

---

## PART 3: CUSTOM CVE MATCHING ENGINE

### **3.1 Architecture**

```
┌────────────────────────────────────────────────────────┐
│         CUSTOM CVE MATCHING ENGINE                     │
└────────────────────────────────────────────────────────┘

COMPONENTS:
═══════════════════════════════════════════════════════════

1. CVE Database Manager
   ├─ Option A: Use Trivy DB (just data)
   ├─ Option B: Sync from NVD API
   └─ Option C: Hybrid (recommended)

2. PURL Parser
   └─ Parse Package URL to extract ecosystem/name/version

3. Version Comparator
   ├─ Debian version comparison
   ├─ RPM version comparison
   ├─ Semantic versioning (npm, pip, etc.)
   └─ Go version comparison

4. CVE Matcher
   ├─ Query database by package ecosystem + name
   ├─ Apply version constraints
   ├─ Filter by severity
   └─ Return matched CVEs

5. Enrichment Engine
   ├─ Add CVSS scores
   ├─ Add exploit availability
   └─ Add fix versions


MATCHING FLOW:
═══════════════════════════════════════════════════════════

Input: SBOM with packages
  ↓
For each package:
  1. Parse PURL → extract ecosystem, name, version
  2. Query CVE DB → get applicable CVEs for package
  3. For each CVE:
     - Check version constraint (e.g., "< 1.1.1l")
     - Is installed version vulnerable?
     - If yes → Add to matches
  4. Enrich matches (CVSS, exploits, fixes)
  5. Filter by severity (CRITICAL + HIGH)
  ↓
Output: List of CVE matches
```

### **3.2 Implementation - CVE Database Manager**

```go
// ════════════════════════════════════════════════════════════════
// FILE: pkg/cve/database/manager.go
// PURPOSE: CVE database management (Trivy DB + NVD API)
// ════════════════════════════════════════════════════════════════

package database

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    
    "github.com/boltdb/bolt"
)

// ────────────────────────────────────────────────────────────────
// DATABASE MANAGER
// ────────────────────────────────────────────────────────────────

type Manager struct {
    trivyDB *TrivyDBReader
    nvdAPI  *NVDAPIClient
    cache   *CVECache
}

func NewManager(trivyDBPath string) (*Manager, error) {
    // Option 1: Use Trivy DB (just read data, not use Trivy code)
    trivyDB, err := NewTrivyDBReader(trivyDBPath)
    if err != nil {
        return nil, err
    }
    
    // Option 2: NVD API client (for updates)
    nvdAPI := NewNVDAPIClient()
    
    // Cache for fast lookups
    cache := NewCVECache()
    
    return &Manager{
        trivyDB: trivyDB,
        nvdAPI:  nvdAPI,
        cache:   cache,
    }, nil
}

func (m *Manager) GetVulnerabilitiesForPackage(
    ecosystem string,
    name string,
    version string,
) ([]CVE, error) {
    // 1. Check cache first
    cacheKey := fmt.Sprintf("%s:%s:%s", ecosystem, name, version)
    if cached, ok := m.cache.Get(cacheKey); ok {
        return cached, nil
    }
    
    // 2. Query Trivy DB (primary source)
    cves, err := m.trivyDB.Query(ecosystem, name, version)
    if err != nil {
        log.Warnf("Trivy DB query failed: %v", err)
    }
    
    // 3. Fallback to NVD API if Trivy DB has no data
    if len(cves) == 0 {
        cves, err = m.nvdAPI.Query(ecosystem, name, version)
        if err != nil {
            log.Warnf("NVD API query failed: %v", err)
        }
    }
    
    // 4. Cache result
    m.cache.Set(cacheKey, cves)
    
    return cves, nil
}


// ────────────────────────────────────────────────────────────────
// TRIVY DB READER (Just data access, not using Trivy code!)
// ────────────────────────────────────────────────────────────────

type TrivyDBReader struct {
    db *bolt.DB
}

func NewTrivyDBReader(dbPath string) (*TrivyDBReader, error) {
    // Open Trivy DB (BoltDB format)
    db, err := bolt.Open(dbPath, 0600, &bolt.Options{ReadOnly: true})
    if err != nil {
        return nil, err
    }
    
    return &TrivyDBReader{db: db}, nil
}

func (t *TrivyDBReader) Query(
    ecosystem string,
    name string,
    version string,
) ([]CVE, error) {
    cves := make([]CVE, 0)
    
    err := t.db.View(func(tx *bolt.Tx) error {
        // Get vulnerability bucket
        bucket := tx.Bucket([]byte("vulnerability"))
        if bucket == nil {
            return fmt.Errorf("vulnerability bucket not found")
        }
        
        // Construct advisory key
        // Format: "ecosystem:distro:package:version"
        // Example: "debian:10:openssl:1.1.1d"
        key := t.buildAdvisoryKey(ecosystem, name, version)
        
        // Get advisory data
        advisoryBucket := tx.Bucket([]byte("advisory"))
        if advisoryBucket == nil {
            return nil
        }
        
        advisoryData := advisoryBucket.Get([]byte(key))
        if advisoryData == nil {
            return nil  // No vulnerabilities found
        }
        
        // Parse advisory (list of CVE IDs)
        var cveIDs []string
        if err := json.Unmarshal(advisoryData, &cveIDs); err != nil {
            return err
        }
        
        // Get details for each CVE
        for _, cveID := range cveIDs {
            vulnData := bucket.Get([]byte(cveID))
            if vulnData == nil {
                continue
            }
            
            var cve CVE
            if err := json.Unmarshal(vulnData, &cve); err != nil {
                continue
            }
            
            cves = append(cves, cve)
        }
        
        return nil
    })
    
    return cves, err
}

func (t *TrivyDBReader) buildAdvisoryKey(
    ecosystem string,
    name string,
    version string,
) string {
    // Trivy DB key format varies by ecosystem
    switch ecosystem {
    case "debian":
        return fmt.Sprintf("debian:%s:%s", name, version)
    case "alpine":
        return fmt.Sprintf("alpine:%s:%s", name, version)
    case "redhat":
        return fmt.Sprintf("redhat:%s:%s", name, version)
    default:
        return fmt.Sprintf("%s:%s:%s", ecosystem, name, version)
    }
}


// ────────────────────────────────────────────────────────────────
// NVD API CLIENT (Fallback source)
// ────────────────────────────────────────────────────────────────

type NVDAPIClient struct {
    apiKey  string
    baseURL string
    client  *http.Client
}

func NewNVDAPIClient() *NVDAPIClient {
    return &NVDAPIClient{
        baseURL: "https://services.nvd.nist.gov/rest/json/cves/2.0",
        client:  &http.Client{Timeout: 30 * time.Second},
    }
}

func (n *NVDAPIClient) Query(
    ecosystem string,
    name string,
    version string,
) ([]CVE, error) {
    // Query NVD API
    // https://services.nvd.nist.gov/rest/json/cves/2.0?keywordSearch=openssl
    
    url := fmt.Sprintf("%s?keywordSearch=%s&resultsPerPage=100", n.baseURL, name)
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    if n.apiKey != "" {
        req.Header.Set("apiKey", n.apiKey)
    }
    
    resp, err := n.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // Parse response
    var nvdResp NVDResponse
    if err := json.NewDecoder(resp.Body).Decode(&nvdResp); err != nil {
        return nil, err
    }
    
    // Convert to our CVE format
    cves := make([]CVE, 0)
    for _, item := range nvdResp.Vulnerabilities {
        cve := n.convertNVDtoCVE(item)
        cves = append(cves, cve)
    }
    
    return cves, nil
}


// ────────────────────────────────────────────────────────────────
// TYPES
// ────────────────────────────────────────────────────────────────

type CVE struct {
    ID           string   `json:"id"`
    Description  string   `json:"description"`
    Severity     string   `json:"severity"`
    CVSSScore    float64  `json:"cvss_score"`
    AffectedPkgs []string `json:"affected_packages"`
    FixedVersion string   `json:"fixed_version"`
    Published    time.Time `json:"published"`
    Modified     time.Time `json:"modified"`
}
```

### **3.3 Version Comparator (Critical Component)**

```go
// ════════════════════════════════════════════════════════════════
// FILE: pkg/cve/matcher/version_comparator.go
// PURPOSE: Version comparison logic for different ecosystems
// ════════════════════════════════════════════════════════════════

package matcher

import (
    "strconv"
    "strings"
    
    "github.com/hashicorp/go-version"
)

// ────────────────────────────────────────────────────────────────
// VERSION COMPARATOR
// ────────────────────────────────────────────────────────────────

type VersionComparator struct{}

func NewVersionComparator() *VersionComparator {
    return &VersionComparator{}
}

func (vc *VersionComparator) IsVulnerable(
    installedVersion string,
    constraint string,
    ecosystem string,
) (bool, error) {
    // Parse constraint: "< 1.1.1l", ">= 2.0.0, < 3.0.0", etc.
    
    switch ecosystem {
    case "deb", "debian", "ubuntu":
        return vc.compareDebianVersion(installedVersion, constraint)
    case "rpm", "redhat", "centos":
        return vc.compareRPMVersion(installedVersion, constraint)
    case "apk", "alpine":
        return vc.compareAlpineVersion(installedVersion, constraint)
    case "npm", "pypi", "go":
        return vc.compareSemver(installedVersion, constraint)
    default:
        // Fallback to semantic versioning
        return vc.compareSemver(installedVersion, constraint)
    }
}


// ────────────────────────────────────────────────────────────────
// DEBIAN VERSION COMPARISON
// ────────────────────────────────────────────────────────────────

func (vc *VersionComparator) compareDebianVersion(
    installed string,
    constraint string,
) (bool, error) {
    // Debian version format: [epoch:]upstream_version[-debian_revision]
    // Example: 1:1.1.1d-0+deb10u7
    
    // Parse constraint operator
    op, targetVersion := vc.parseConstraint(constraint)
    
    // Compare versions using Debian's dpkg --compare-versions logic
    result := vc.dpkgCompareVersions(installed, targetVersion)
    
    // Apply operator
    switch op {
    case "<":
        return result < 0, nil
    case "<=":
        return result <= 0, nil
    case ">":
        return result > 0, nil
    case ">=":
        return result >= 0, nil
    case "==":
        return result == 0, nil
    default:
        return false, fmt.Errorf("unknown operator: %s", op)
    }
}

func (vc *VersionComparator) dpkgCompareVersions(v1, v2 string) int {
    // Simplified Debian version comparison
    // Full algorithm: https://www.debian.org/doc/debian-policy/ch-controlfields.html#version
    
    // 1. Split into epoch:version-revision
    epoch1, ver1, rev1 := vc.splitDebianVersion(v1)
    epoch2, ver2, rev2 := vc.splitDebianVersion(v2)
    
    // 2. Compare epoch
    if epoch1 != epoch2 {
        return epoch1 - epoch2
    }
    
    // 3. Compare upstream version
    cmp := vc.compareDebianVersionPart(ver1, ver2)
    if cmp != 0 {
        return cmp
    }
    
    // 4. Compare revision
    return vc.compareDebianVersionPart(rev1, rev2)
}

func (vc *VersionComparator) splitDebianVersion(v string) (int, string, string) {
    // Split "1:1.1.1d-0+deb10u7" → epoch=1, version=1.1.1d, revision=0+deb10u7
    
    epoch := 0
    version := v
    revision := ""
    
    // Extract epoch
    if idx := strings.Index(v, ":"); idx != -1 {
        epoch, _ = strconv.Atoi(v[:idx])
        version = v[idx+1:]
    }
    
    // Extract revision
    if idx := strings.LastIndex(version, "-"); idx != -1 {
        revision = version[idx+1:]
        version = version[:idx]
    }
    
    return epoch, version, revision
}

func (vc *VersionComparator) compareDebianVersionPart(v1, v2 string) int {
    // Debian version comparison is complex:
    // - Letters < numbers
    // - Compare character by character
    // - Treat non-alphanumeric as less than anything
    
    // Simplified implementation (for production, use dpkg library or algorithm)
    if v1 < v2 {
        return -1
    } else if v1 > v2 {
        return 1
    }
    return 0
}


// ────────────────────────────────────────────────────────────────
// SEMANTIC VERSIONING (npm, pip, go, etc.)
// ────────────────────────────────────────────────────────────────

func (vc *VersionComparator) compareSemver(
    installed string,
    constraint string,
) (bool, error) {
    // Use hashicorp/go-version for semantic versioning
    // Handles: 1.2.3, 1.2.3-alpha, 1.2.3+build, etc.
    
    v1, err := version.NewVersion(installed)
    if err != nil {
        return false, err
    }
    
    // Parse constraint: "< 1.2.3", ">= 1.0.0, < 2.0.0"
    constraints, err := version.NewConstraint(constraint)
    if err != nil {
        return false, err
    }
    
    // Check if installed version satisfies constraint
    // Returns true if MATCHES constraint (i.e., vulnerable!)
    return constraints.Check(v1), nil
}


// ────────────────────────────────────────────────────────────────
// ALPINE VERSION COMPARISON
// ────────────────────────────────────────────────────────────────

func (vc *VersionComparator) compareAlpineVersion(
    installed string,
    constraint string,
) (bool, error) {
    // Alpine version format: 1.1.1g-r0
    // -rN is Alpine release number
    
    // Strip Alpine release suffix
    installed = strings.TrimSuffix(installed, strings.Split(installed, "-r")[len(strings.Split(installed, "-r"))-1])
    
    // Use semantic versioning for comparison
    return vc.compareSemver(installed, constraint)
}


// ────────────────────────────────────────────────────────────────
// RPM VERSION COMPARISON
// ────────────────────────────────────────────────────────────────

func (vc *VersionComparator) compareRPMVersion(
    installed string,
    constraint string,
) (bool, error) {
    // RPM version format: [epoch:]version-release
    // Example: 1:1.1.1k-5.el8
    
    // Similar to Debian but simpler
    // For production, use RPM library or implement full algorithm
    
    return vc.compareSemver(installed, constraint)
}


// ────────────────────────────────────────────────────────────────
// HELPER FUNCTIONS
// ────────────────────────────────────────────────────────────────

func (vc *VersionComparator) parseConstraint(constraint string) (string, string) {
    // Parse "<version", "<=1.2.3", etc.
    constraint = strings.TrimSpace(constraint)
    
    if strings.HasPrefix(constraint, "<=") {
        return "<=", strings.TrimSpace(constraint[2:])
    } else if strings.HasPrefix(constraint, ">=") {
        return ">=", strings.TrimSpace(constraint[2:])
    } else if strings.HasPrefix(constraint, "<") {
        return "<", strings.TrimSpace(constraint[1:])
    } else if strings.HasPrefix(constraint, ">") {
        return ">", strings.TrimSpace(constraint[1:])
    } else if strings.HasPrefix(constraint, "==") {
        return "==", strings.TrimSpace(constraint[2:])
    }
    
    // Default: exact match
    return "==", constraint
}
```

Continue to Part 3 with CVE Matcher implementation...
