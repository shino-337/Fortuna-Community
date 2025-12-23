# CUSTOM SBOM-BASED SCANNING - ZERO DEPENDENCY
## Building Your Own Complete Pipeline

**Date**: 2025-12-12  
**Purpose**: Full custom SBOM pipeline WITHOUT Syft/Grype dependency

---

## 📋 EXECUTIVE SUMMARY

### **USER REQUIREMENT**

```
"Tôi không muốn sử dụng và phụ thuộc các giải pháp có sẵn như Grype"

DESIRED ARCHITECTURE:
═══════════════════════════════════════════════════════════════
Pod Created
  ↓
Pod Watcher
  ↓
SBOM Cache Lookup
  ↓
Generate SBOM (YOUR extractor - not Syft)
  ↓
Normalize SBOM (CycloneDX-like format)
  ↓
CVE Matching Engine (YOUR mgmt - not Grype)
  ↓
Insight Creator
  ↓
Risk Engine V2
  ↓
Dashboard

PHILOSOPHY: ZERO EXTERNAL TOOL DEPENDENCY ✅
```

### **SOLUTION OVERVIEW**

```
┌────────────────────────────────────────────────────────┐
│        FULLY CUSTOM PIPELINE - NO EXTERNAL TOOLS       │
└────────────────────────────────────────────────────────┘

WHAT WE BUILD (100% custom):
═══════════════════════════════════════════════════════════
✅ Custom SBOM Extractor
   ├─ Parse dpkg (Debian/Ubuntu)
   ├─ Parse rpm (RedHat/CentOS)
   ├─ Parse apk (Alpine)
   ├─ Parse package.json (Node.js)
   ├─ Parse requirements.txt (Python)
   ├─ Parse go.mod (Go)
   └─ Parse pom.xml (Java)

✅ Custom SBOM Normalizer
   └─ Convert to CycloneDX-like format

✅ Custom CVE Matching Engine
   ├─ Query CVE database
   ├─ Version comparison logic
   ├─ PURL parsing
   └─ Match packages to CVEs

✅ CVE Database Management
   ├─ Option 1: Use Trivy DB (just data, not code)
   ├─ Option 2: Sync from NVD API
   └─ Option 3: Hybrid (Trivy DB + NVD API)


WHAT WE REUSE (just data, not tools):
═══════════════════════════════════════════════════════════
⚠️ CVE Database (Trivy DB or NVD)
   └─ Just vulnerability data (JSON/SQLite)
   └─ NOT using Trivy/Grype code!


BENEFITS:
═══════════════════════════════════════════════════════════
✅ Zero dependency on external tools
✅ Full control over every step
✅ Customize to KSAM needs
✅ No licensing concerns
✅ Can optimize for your use case
✅ No surprises from tool updates


TRADE-OFFS:
═══════════════════════════════════════════════════════════
⚠️ More development work (6-8 weeks vs 4 weeks)
⚠️ Need to maintain matching logic
⚠️ Version comparison is tricky
⚠️ Need comprehensive testing
```

---

## PART 1: CUSTOM SBOM EXTRACTOR

### **1.1 Package Manager Support**

```
┌────────────────────────────────────────────────────────┐
│              SBOM EXTRACTOR ARCHITECTURE                │
└────────────────────────────────────────────────────────┘

SUPPORTED PACKAGE MANAGERS:
═══════════════════════════════════════════════════════════

1. OS Packages (Priority: HIGH)
   ├─ dpkg (Debian/Ubuntu) ⭐⭐⭐⭐⭐
   ├─ rpm (RedHat/CentOS) ⭐⭐⭐⭐⭐
   └─ apk (Alpine) ⭐⭐⭐⭐⭐

2. Language Packages (Priority: HIGH)
   ├─ npm (Node.js) ⭐⭐⭐⭐⭐
   ├─ pip (Python) ⭐⭐⭐⭐⭐
   ├─ go.mod (Go) ⭐⭐⭐⭐
   ├─ Maven (Java) ⭐⭐⭐⭐
   └─ Bundler (Ruby) ⭐⭐⭐

3. Advanced (Priority: MEDIUM)
   ├─ cargo (Rust) ⭐⭐⭐
   ├─ composer (PHP) ⭐⭐⭐
   └─ NuGet (.NET) ⭐⭐⭐


EXTRACTION STRATEGY:
═══════════════════════════════════════════════════════════

For each package manager, we need to:
1. Locate package database file in image
2. Parse the file format
3. Extract: name, version, (optional) dependencies
4. Normalize to common format

Example locations:
├─ dpkg: /var/lib/dpkg/status
├─ rpm: /var/lib/rpm/Packages (Berkeley DB)
├─ apk: /lib/apk/db/installed
├─ npm: /package-lock.json, /node_modules/*/package.json
├─ pip: /site-packages/*.dist-info/METADATA
├─ go: /go.sum
└─ maven: /pom.xml, /META-INF/maven/**/pom.properties
```

### **1.2 Implementation - SBOM Extractor**

```go
// ════════════════════════════════════════════════════════════════
// FILE: pkg/sbom/extractor/extractor.go
// PURPOSE: Custom SBOM extraction from container images
// ════════════════════════════════════════════════════════════════

package extractor

import (
    "context"
    "fmt"
    "io"
    
    "github.com/google/go-containerregistry/pkg/v1"
    "github.com/google/go-containerregistry/pkg/v1/remote"
)

// ────────────────────────────────────────────────────────────────
// MAIN EXTRACTOR
// ────────────────────────────────────────────────────────────────

type Extractor struct {
    parsers map[string]Parser
}

func NewExtractor() *Extractor {
    return &Extractor{
        parsers: map[string]Parser{
            "dpkg": &DpkgParser{},
            "rpm":  &RpmParser{},
            "apk":  &ApkParser{},
            "npm":  &NpmParser{},
            "pip":  &PipParser{},
            "go":   &GoModParser{},
        },
    }
}

func (e *Extractor) ExtractSBOM(
    ctx context.Context,
    imageName string,
) (*RawSBOM, error) {
    // 1. Pull image
    log.Infof("Extracting SBOM from image: %s", imageName)
    
    img, err := e.pullImage(imageName)
    if err != nil {
        return nil, fmt.Errorf("failed to pull image: %w", err)
    }
    
    // 2. Get image layers
    layers, err := img.Layers()
    if err != nil {
        return nil, fmt.Errorf("failed to get layers: %w", err)
    }
    
    // 3. Extract filesystem
    fs := e.buildFilesystem(layers)
    
    // 4. Detect OS
    osInfo := e.detectOS(fs)
    log.Infof("Detected OS: %s %s", osInfo.Name, osInfo.Version)
    
    // 5. Run all parsers
    allPackages := make([]Package, 0)
    
    for name, parser := range e.parsers {
        packages, err := parser.Parse(fs)
        if err != nil {
            log.Warnf("Parser %s failed: %v", name, err)
            continue
        }
        
        log.Infof("Parser %s found %d packages", name, len(packages))
        allPackages = append(allPackages, packages...)
    }
    
    // 6. Deduplicate
    deduped := e.deduplicate(allPackages)
    
    sbom := &RawSBOM{
        ImageName:  imageName,
        OS:         osInfo,
        Packages:   deduped,
        ExtractedAt: time.Now(),
    }
    
    log.Infof("Extracted %d unique packages", len(deduped))
    return sbom, nil
}


// ────────────────────────────────────────────────────────────────
// DPKG PARSER (Debian/Ubuntu)
// ────────────────────────────────────────────────────────────────

type DpkgParser struct{}

func (p *DpkgParser) Parse(fs *Filesystem) ([]Package, error) {
    // Read dpkg status file
    content, err := fs.ReadFile("/var/lib/dpkg/status")
    if err != nil {
        return nil, err
    }
    
    packages := make([]Package, 0)
    
    // Parse dpkg status format
    // Format:
    // Package: openssl
    // Version: 1.1.1d-0+deb10u7
    // Status: install ok installed
    // Architecture: amd64
    
    lines := strings.Split(string(content), "\n")
    var currentPkg Package
    
    for _, line := range lines {
        line = strings.TrimSpace(line)
        
        if line == "" {
            // End of package entry
            if currentPkg.Name != "" {
                packages = append(packages, currentPkg)
                currentPkg = Package{}
            }
            continue
        }
        
        if strings.HasPrefix(line, "Package: ") {
            currentPkg.Name = strings.TrimPrefix(line, "Package: ")
            currentPkg.Type = "deb"
        } else if strings.HasPrefix(line, "Version: ") {
            currentPkg.Version = strings.TrimPrefix(line, "Version: ")
        } else if strings.HasPrefix(line, "Architecture: ") {
            currentPkg.Arch = strings.TrimPrefix(line, "Architecture: ")
        }
    }
    
    return packages, nil
}


// ────────────────────────────────────────────────────────────────
// APK PARSER (Alpine)
// ────────────────────────────────────────────────────────────────

type ApkParser struct{}

func (p *ApkParser) Parse(fs *Filesystem) ([]Package, error) {
    // Read apk installed file
    content, err := fs.ReadFile("/lib/apk/db/installed")
    if err != nil {
        return nil, err
    }
    
    packages := make([]Package, 0)
    
    // Parse apk format
    // Format:
    // P:openssl
    // V:1.1.1g-r0
    // A:x86_64
    
    lines := strings.Split(string(content), "\n")
    var currentPkg Package
    
    for _, line := range lines {
        line = strings.TrimSpace(line)
        
        if line == "" {
            if currentPkg.Name != "" {
                packages = append(packages, currentPkg)
                currentPkg = Package{}
            }
            continue
        }
        
        if strings.HasPrefix(line, "P:") {
            currentPkg.Name = strings.TrimPrefix(line, "P:")
            currentPkg.Type = "apk"
        } else if strings.HasPrefix(line, "V:") {
            currentPkg.Version = strings.TrimPrefix(line, "V:")
        } else if strings.HasPrefix(line, "A:") {
            currentPkg.Arch = strings.TrimPrefix(line, "A:")
        }
    }
    
    return packages, nil
}


// ────────────────────────────────────────────────────────────────
// RPM PARSER (RedHat/CentOS)
// ────────────────────────────────────────────────────────────────

type RpmParser struct{}

func (p *RpmParser) Parse(fs *Filesystem) ([]Package, error) {
    // RPM database is Berkeley DB format
    // Need to use rpm command or parse DB directly
    
    // Option 1: Use rpmdb-dump if available
    // Option 2: Parse Berkeley DB directly (complex)
    // Option 3: Look for /var/lib/rpm/Packages and use Go Berkeley DB library
    
    // For simplicity, we'll use a Go Berkeley DB library
    db, err := bdb.Open("/var/lib/rpm/Packages", nil)
    if err != nil {
        return nil, err
    }
    defer db.Close()
    
    packages := make([]Package, 0)
    
    // Iterate through RPM database
    cursor := db.Cursor()
    defer cursor.Close()
    
    for key, value, err := cursor.First(); err == nil; key, value, err = cursor.Next() {
        pkg, err := p.parseRpmEntry(value)
        if err != nil {
            continue
        }
        packages = append(packages, pkg)
    }
    
    return packages, nil
}


// ────────────────────────────────────────────────────────────────
// NPM PARSER (Node.js)
// ────────────────────────────────────────────────────────────────

type NpmParser struct{}

func (p *NpmParser) Parse(fs *Filesystem) ([]Package, error) {
    packages := make([]Package, 0)
    
    // Strategy 1: Parse package-lock.json
    if lockContent, err := fs.ReadFile("/package-lock.json"); err == nil {
        pkgs, _ := p.parsePackageLock(lockContent)
        packages = append(packages, pkgs...)
    }
    
    // Strategy 2: Find all package.json in node_modules
    moduleDirs := fs.Glob("/node_modules/*/package.json")
    for _, path := range moduleDirs {
        content, err := fs.ReadFile(path)
        if err != nil {
            continue
        }
        
        pkg, err := p.parsePackageJson(content)
        if err != nil {
            continue
        }
        
        packages = append(packages, pkg)
    }
    
    return packages, nil
}

func (p *NpmParser) parsePackageJson(content []byte) (Package, error) {
    var pkgJson struct {
        Name    string `json:"name"`
        Version string `json:"version"`
    }
    
    if err := json.Unmarshal(content, &pkgJson); err != nil {
        return Package{}, err
    }
    
    return Package{
        Name:    pkgJson.Name,
        Version: pkgJson.Version,
        Type:    "npm",
    }, nil
}


// ────────────────────────────────────────────────────────────────
// PIP PARSER (Python)
// ────────────────────────────────────────────────────────────────

type PipParser struct{}

func (p *PipParser) Parse(fs *Filesystem) ([]Package, error) {
    packages := make([]Package, 0)
    
    // Strategy 1: Parse requirements.txt
    if reqContent, err := fs.ReadFile("/requirements.txt"); err == nil {
        pkgs, _ := p.parseRequirements(reqContent)
        packages = append(packages, pkgs...)
    }
    
    // Strategy 2: Find all *.dist-info/METADATA
    metadataFiles := fs.Glob("/site-packages/*.dist-info/METADATA")
    for _, path := range metadataFiles {
        content, err := fs.ReadFile(path)
        if err != nil {
            continue
        }
        
        pkg, err := p.parseMetadata(content)
        if err != nil {
            continue
        }
        
        packages = append(packages, pkg)
    }
    
    return packages, nil
}

func (p *PipParser) parseMetadata(content []byte) (Package, error) {
    // Parse METADATA format (RFC 822-like)
    // Name: Django
    // Version: 3.2.5
    
    lines := strings.Split(string(content), "\n")
    pkg := Package{Type: "pypi"}
    
    for _, line := range lines {
        if strings.HasPrefix(line, "Name: ") {
            pkg.Name = strings.TrimPrefix(line, "Name: ")
        } else if strings.HasPrefix(line, "Version: ") {
            pkg.Version = strings.TrimPrefix(line, "Version: ")
        }
    }
    
    return pkg, nil
}


// ────────────────────────────────────────────────────────────────
// GO MOD PARSER
// ────────────────────────────────────────────────────────────────

type GoModParser struct{}

func (p *GoModParser) Parse(fs *Filesystem) ([]Package, error) {
    // Parse go.sum or go.mod
    content, err := fs.ReadFile("/go.sum")
    if err != nil {
        return nil, err
    }
    
    packages := make([]Package, 0)
    
    // go.sum format:
    // github.com/gin-gonic/gin v1.7.0 h1:hash...
    // github.com/gin-gonic/gin v1.7.0/go.mod h1:hash...
    
    lines := strings.Split(string(content), "\n")
    seen := make(map[string]bool)
    
    for _, line := range lines {
        parts := strings.Fields(line)
        if len(parts) < 2 {
            continue
        }
        
        name := parts[0]
        version := strings.TrimSuffix(parts[1], "/go.mod")
        
        key := name + "@" + version
        if seen[key] {
            continue
        }
        seen[key] = true
        
        packages = append(packages, Package{
            Name:    name,
            Version: version,
            Type:    "go",
        })
    }
    
    return packages, nil
}


// ────────────────────────────────────────────────────────────────
// TYPES
// ────────────────────────────────────────────────────────────────

type Package struct {
    Name    string
    Version string
    Type    string  // deb, apk, rpm, npm, pypi, go, maven
    Arch    string
}

type RawSBOM struct {
    ImageName   string
    OS          OSInfo
    Packages    []Package
    ExtractedAt time.Time
}

type OSInfo struct {
    Name    string  // debian, alpine, centos, etc.
    Version string  // 10, 3.14, 8, etc.
}
```

Continue to Part 2 with SBOM Normalizer and CVE Matching Engine...
