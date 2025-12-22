# MASTER IMPLEMENTATION GUIDE: CVE Detection Integration

**Mục tiêu**: Tích hợp CVE scanning hoàn chỉnh vào KSAM  
**Timeline**: 20 ngày (4 tuần)  
**Effort**: 1 engineer full-time

---

## 📋 MỤC LỤC

1. [Tổng Quan](#1-tổng-quan)
2. [Checklist Tổng Thể](#2-checklist-tổng-thể)
3. [Database Changes](#3-database-changes)
4. [Code Components](#4-code-components)
5. [Deployments](#5-deployments)
6. [Integration Points](#6-integration-points)
7. [Testing Plan](#7-testing-plan)
8. [Timeline Chi Tiết](#8-timeline-chi-tiết)

---

## 1. TỔNG QUAN

### **1.1 Current State vs Target State**

```
HIỆN TẠI (AS-IS):
═══════════════════════════════════════════════════════════
✅ Policy Engine       → Detects misconfigurations
✅ Correlation Engine  → Detects attack patterns
✅ Risk Scorer         → Calculates risk scores
✅ PostgreSQL          → Stores insights + scores

❌ KHÔNG CÓ CVE scanning
❌ KHÔNG BIẾT images có vulnerabilities
❌ KHÔNG THỂ detect CVE-2021-23017

MỤC TIÊU (TO-BE):
═══════════════════════════════════════════════════════════
✅ CVE Database        → Stores 200K+ CVEs
✅ Image Scanner       → Scans all container images
✅ Auto Detection      → Scans pods automatically
✅ CVE Insights        → Creates insights for CVEs
✅ CVE Risk Scoring    → Scores CVE risks
✅ Dashboard           → Shows CVE findings

✅ CÓ THỂ detect nginx:1.19.0 has CVE-2021-23017
✅ CÓ THỂ calculate risk score = 85.5 (P1)
✅ CÓ THỂ recommend: "Upgrade to 1.20.1"
```

### **1.2 Architecture Overview**

```
┌─────────────────────────────────────────────────────────┐
│  NEW SYSTEM ARCHITECTURE                                 │
└─────────────────────────────────────────────────────────┘

External → KSAM New Components → Database → Existing KSAM
  │              │                  │              │
  │              │                  │              │
NVD API      CVE Updater       New Tables    Insight Mgr
Trivy DB     Image Scanner     (4 tables)    Risk Scorer
             Pod Watcher                      Dashboard
             CVE Processor
```

---

## 2. CHECKLIST TỔNG THỂ

### **2.1 Implementation Checklist**

```
☐ PHASE 1: DATABASE (2 ngày)
  ☐ Tạo 4 tables mới
  ☐ Tạo 3 views
  ☐ Tạo indexes
  ☐ Test migrations
  
☐ PHASE 2: CODE (8 ngày)
  ☐ CVE Updater (2 ngày)
  ☐ Image Scanner (2 ngày)  
  ☐ Pod Watcher (2 ngày)
  ☐ CVE Processor (2 ngày)
  
☐ PHASE 3: DEPLOYMENT (3 ngày)
  ☐ Deploy Trivy Server
  ☐ Deploy KSAM Scanner
  ☐ Configure secrets
  
☐ PHASE 4: INTEGRATION (2 ngày)
  ☐ Modify Risk Scorer
  ☐ Add API endpoints
  ☐ Update Dashboard
  
☐ PHASE 5: TESTING (3 ngày)
  ☐ Unit tests
  ☐ Integration tests
  ☐ E2E tests
  
☐ PHASE 6: DEPLOYMENT (2 ngày)
  ☐ Deploy to staging
  ☐ Deploy to production
  ☐ Monitor & tune
```

---

## 3. DATABASE CHANGES

### **3.1 Cần Tạo Bao Nhiêu Tables?**

**Trả lời: CẦN 4 TABLES MỚI**

```
1. cves                      → Store CVE data (200K+ rows)
2. package_vulnerabilities   → Link CVEs to packages (500K+ rows)
3. image_scan_results        → Store scan results (1K+ rows)
4. pod_image_scans           → Map pods to scans (10K+ rows)
```

### **3.2 Chi Tiết Từng Table**

#### **TABLE 1: cves**

**Purpose**: Lưu trữ thông tin CVE từ NVD

**Columns** (17 columns):
```sql
cve_id               VARCHAR(20)    PRIMARY KEY    -- CVE-2021-23017
cvss_score           DECIMAL(3,1)                  -- 8.1
cvss_vector          TEXT                          -- CVSS:3.1/AV:N/AC:H/...
cvss_version         VARCHAR(10)                   -- 3.1
severity             VARCHAR(20)    NOT NULL       -- CRITICAL/HIGH/MEDIUM/LOW
title                TEXT                          -- Short title
description          TEXT                          -- Full description
published_date       TIMESTAMP                     -- When published
last_modified_date   TIMESTAMP                     -- Last update
exploit_available    BOOLEAN        DEFAULT FALSE  -- Has public exploit?
exploit_maturity     VARCHAR(20)                   -- poc/functional/high
exploit_sources      TEXT[]                        -- [metasploit, exploit-db]
references           JSONB                         -- URLs, references
cwe_ids              TEXT[]                        -- [CWE-787]
source               VARCHAR(50)    NOT NULL       -- nvd/trivy/github
source_url           TEXT                          -- Link to source
created_at           TIMESTAMP      AUTO           -- When inserted
updated_at           TIMESTAMP      AUTO           -- Last update
```

**Indexes** (6 indexes):
```sql
PRIMARY KEY: cve_id
INDEX: severity
INDEX: cvss_score DESC
INDEX: published_date DESC
INDEX: exploit_available (WHERE TRUE)
INDEX: description (Full-text search)
```

**Sample Data**:
```sql
INSERT INTO cves VALUES (
  'CVE-2021-23017',
  8.1,
  'CVSS:3.1/AV:N/AC:H/PR:N/UI:N/S:U/C:H/I:H/A:H',
  '3.1',
  'HIGH',
  'nginx DNS resolver off-by-one heap write',
  'A security issue in nginx resolver...',
  '2021-06-01',
  '2021-06-15',
  TRUE,
  'functional',
  ARRAY['metasploit'],
  '{"vendor": "https://nginx.org/..."}',
  ARRAY['CWE-787'],
  'nvd',
  'https://nvd.nist.gov/vuln/detail/CVE-2021-23017',
  NOW(),
  NOW()
);
```

---

#### **TABLE 2: package_vulnerabilities**

**Purpose**: Link CVEs với packages và version ranges

**Columns** (15 columns):
```sql
id                        SERIAL       PRIMARY KEY
cve_id                    VARCHAR(20)  REFERENCES cves(cve_id)
package_name              VARCHAR(255) NOT NULL      -- nginx
package_type              VARCHAR(50)                -- deb/rpm
ecosystem                 VARCHAR(50)                -- debian/alpine
affected_range            TEXT                       -- >=0.6.18,<1.20.1
version_start_including   VARCHAR(50)                -- 0.6.18
version_start_excluding   VARCHAR(50)
version_end_including     VARCHAR(50)
version_end_excluding     VARCHAR(50)                -- 1.20.1 (most common)
fixed_version             VARCHAR(50)                -- 1.20.1
fixed_in_versions         TEXT[]                     -- [1.20.1, 1.21.0]
vendor                    VARCHAR(100)               -- f5
product                   VARCHAR(100)               -- nginx
created_at                TIMESTAMP    AUTO
updated_at                TIMESTAMP    AUTO
```

**Indexes** (4 indexes):
```sql
PRIMARY KEY: id
INDEX: cve_id
INDEX: package_name
INDEX: (package_name, ecosystem)
UNIQUE: (cve_id, package_name, ecosystem, version_end_excluding)
```

**Sample Data**:
```sql
INSERT INTO package_vulnerabilities VALUES (
  1,
  'CVE-2021-23017',
  'nginx',
  'deb',
  'debian',
  '>=0.6.18,<1.20.1',
  '0.6.18',
  NULL,
  NULL,
  '1.20.1',
  '1.20.1',
  ARRAY['1.20.1'],
  'f5',
  'nginx',
  NOW(),
  NOW()
);
```

---

#### **TABLE 3: image_scan_results**

**Purpose**: Lưu kết quả scan từ Trivy

**Columns** (24 columns):
```sql
id                      SERIAL       PRIMARY KEY
image_name              VARCHAR(255) NOT NULL        -- nginx
image_tag               VARCHAR(50)  NOT NULL        -- 1.19.0
image_digest            VARCHAR(71)                  -- sha256:abc123...
registry                VARCHAR(255)                 -- docker.io
full_image_ref          TEXT                         -- full ref
scanned_at              TIMESTAMP    AUTO
scanner_name            VARCHAR(50)  DEFAULT 'trivy'
scanner_version         VARCHAR(50)                  -- trivy-v0.45.0
scan_duration_seconds   DECIMAL(10,2)                -- 120.5
os_family               VARCHAR(50)                  -- debian
os_name                 VARCHAR(100)                 -- Debian GNU/Linux
os_version              VARCHAR(50)                  -- 10 (buster)
total_vulnerabilities   INT          DEFAULT 0       -- 245
critical_count          INT          DEFAULT 0       -- 2
high_count              INT          DEFAULT 0       -- 19
medium_count            INT          DEFAULT 0       -- 46
low_count               INT          DEFAULT 0       -- 178
unknown_count           INT          DEFAULT 0       -- 0
vulnerabilities         JSONB                        -- Full JSON array
packages                JSONB                        -- All packages
status                  VARCHAR(20)  DEFAULT 'in_progress'
error_message           TEXT
cache_key               VARCHAR(100)
expires_at              TIMESTAMP                    -- Cache expiry
created_at              TIMESTAMP    AUTO
updated_at              TIMESTAMP    AUTO
```

**Indexes** (7 indexes):
```sql
PRIMARY KEY: id
INDEX: (image_name, image_tag)
INDEX: image_digest
INDEX: scanned_at DESC
INDEX: status
INDEX: critical_count (WHERE > 0)
INDEX: vulnerabilities (GIN index for JSON)
UNIQUE: (image_name, image_tag, image_digest)
```

**Sample Data**:
```sql
INSERT INTO image_scan_results VALUES (
  1,
  'nginx',
  '1.19.0',
  'sha256:abc123...',
  'docker.io',
  'docker.io/library/nginx:1.19.0@sha256:abc123...',
  NOW(),
  'trivy',
  'trivy-v0.45.0',
  120.5,
  'debian',
  'Debian GNU/Linux',
  '10',
  245,
  2,
  19,
  46,
  178,
  0,
  '[{"cve_id": "CVE-2021-23017", ...}]',
  '[{"name": "nginx", "version": "1.19.0", ...}]',
  'completed',
  NULL,
  'nginx:1.19.0',
  NOW() + INTERVAL '24 hours',
  NOW(),
  NOW()
);
```

---

#### **TABLE 4: pod_image_scans**

**Purpose**: Map pods với scan results

**Columns** (17 columns):
```sql
id                SERIAL       PRIMARY KEY
pod_uid           VARCHAR(255) NOT NULL        -- abc-123-def-456
pod_name          VARCHAR(255) NOT NULL        -- nginx-deployment-abc
pod_namespace     VARCHAR(255) NOT NULL        -- production
cluster_id        VARCHAR(255) NOT NULL        -- prod-cluster-1
container_name    VARCHAR(255) NOT NULL        -- nginx
container_image   TEXT         NOT NULL        -- nginx:1.19.0
image_name        VARCHAR(255)                 -- nginx
image_tag         VARCHAR(50)                  -- 1.19.0
image_registry    VARCHAR(255)                 -- docker.io
scan_result_id    INT          REFERENCES image_scan_results(id)
pod_phase         VARCHAR(20)                  -- Running
pod_created_at    TIMESTAMP
pod_deleted_at    TIMESTAMP
created_at        TIMESTAMP    AUTO
updated_at        TIMESTAMP    AUTO
```

**Indexes** (4 indexes):
```sql
PRIMARY KEY: id
INDEX: pod_uid
INDEX: (cluster_id, pod_namespace)
INDEX: scan_result_id
UNIQUE: (pod_uid, container_name)
```

**Sample Data**:
```sql
INSERT INTO pod_image_scans VALUES (
  1,
  'abc-123-def-456',
  'nginx-deployment-abc',
  'production',
  'prod-cluster-1',
  'nginx',
  'nginx:1.19.0',
  'nginx',
  '1.19.0',
  'docker.io',
  1,
  'Running',
  NOW() - INTERVAL '2 hours',
  NULL,
  NOW(),
  NOW()
);
```

---

### **3.3 Migration Script**

**File**: `KSAM/core/migrations/20250112_001_add_cve_tables.sql`

**Chạy migration**:
```bash
# Step 1: Backup database
pg_dump -h localhost -U ksam ksam > backup_before_cve.sql

# Step 2: Run migration
psql -h localhost -U ksam -d ksam -f migrations/20250112_001_add_cve_tables.sql

# Step 3: Verify
psql -h localhost -U ksam -d ksam -c "
SELECT 
  schemaname, 
  tablename, 
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public' 
  AND tablename IN ('cves', 'package_vulnerabilities', 'image_scan_results', 'pod_image_scans')
ORDER BY tablename;
"

# Expected output:
#  schemaname |        tablename         | size
# ────────────┼─────────────────────────┼──────
#  public     | cves                     | 8 kB
#  public     | image_scan_results       | 8 kB
#  public     | package_vulnerabilities  | 8 kB
#  public     | pod_image_scans          | 8 kB
```

---

## 4. CODE COMPONENTS

### **4.1 Tổng Quan Components**

**CẦN TẠO 4 GO FILES MỚI**:

```
KSAM/core/pkg/scanner/
├── cve_updater.go       (~300 LOC) → Sync CVEs from NVD
├── image_scanner.go     (~400 LOC) → Scan images with Trivy
├── pod_watcher.go       (~200 LOC) → Watch pod events
└── cve_processor.go     (~300 LOC) → Create insights

Total: ~1200 LOC
```

### **4.2 Component 1: CVE Updater**

**File**: `KSAM/core/pkg/scanner/cve_updater.go`

**Chức năng**:
- Fetch CVEs từ NVD API (daily)
- Parse và store vào database
- Update existing CVEs

**Struct**:
```go
type CVEUpdater struct {
    db          *gorm.DB
    nvdAPIKey   string
    nvdBaseURL  string
    httpClient  *http.Client
    logger      *logrus.Logger
}
```

**Main Functions**:
```go
// Start runs daily CVE updates
func (u *CVEUpdater) Start(ctx context.Context) error

// UpdateCVEDatabase fetches latest CVEs
func (u *CVEUpdater) UpdateCVEDatabase(ctx context.Context) error

// fetchCVEsFromNVD calls NVD API
func (u *CVEUpdater) fetchCVEsFromNVD(startDate, endDate time.Time) ([]NVDVulnerability, error)

// parseCVE converts NVD format to our model
func (u *CVEUpdater) parseCVE(nvdVuln *NVDVulnerability) (*models.CVE, []*models.PackageVulnerability, error)

// upsertCVE saves or updates CVE
func (u *CVEUpdater) upsertCVE(cve *models.CVE, pkgVulns []*models.PackageVulnerability) error
```

**Key Logic**:
```go
func (u *CVEUpdater) UpdateCVEDatabase(ctx context.Context) error {
    // 1. Get last update timestamp
    var lastUpdate time.Time
    u.db.Raw("SELECT MAX(published_date) FROM cves").Scan(&lastUpdate)
    
    if lastUpdate.IsZero() {
        lastUpdate = time.Now().AddDate(0, -1, 0) // Last 30 days
    }
    
    // 2. Fetch CVEs from NVD
    cves, err := u.fetchCVEsFromNVD(lastUpdate, time.Now())
    if err != nil {
        return err
    }
    
    // 3. Process each CVE
    for _, nvdCVE := range cves {
        cve, pkgVulns, err := u.parseCVE(&nvdCVE)
        if err != nil {
            u.logger.Errorf("Failed to parse CVE %s: %v", nvdCVE.ID, err)
            continue
        }
        
        // 4. Upsert to database
        if err := u.upsertCVE(cve, pkgVulns); err != nil {
            u.logger.Errorf("Failed to upsert CVE %s: %v", cve.CVEID, err)
            continue
        }
    }
    
    u.logger.Infof("Updated %d CVEs", len(cves))
    return nil
}
```

**Configuration**:
```go
type CVEUpdaterConfig struct {
    NVDAPIKey      string        `yaml:"nvd_api_key"`
    UpdateInterval time.Duration `yaml:"update_interval"` // 24h
    BatchSize      int           `yaml:"batch_size"`      // 100
}
```

---

### **4.3 Component 2: Image Scanner**

**File**: `KSAM/core/pkg/scanner/image_scanner.go`

**Chức năng**:
- Scan container images với Trivy
- Cache scan results (24h)
- Store results vào database

**Struct**:
```go
type ImageScanner struct {
    db           *gorm.DB
    trivyURL     string
    httpClient   *http.Client
    cacheTTL     time.Duration
    logger       *logrus.Logger
}
```

**Main Functions**:
```go
// ScanImage scans a container image
func (s *ImageScanner) ScanImage(ctx context.Context, imageRef string) (*models.ImageScanResult, error)

// checkCache checks for recent scan
func (s *ImageScanner) checkCache(imageName, imageTag string) (*models.ImageScanResult, error)

// trivyScan calls Trivy server
func (s *ImageScanner) trivyScan(ctx context.Context, imageRef string) (*TrivyResult, error)

// saveScanResult stores result in database
func (s *ImageScanner) saveScanResult(result *models.ImageScanResult) error
```

**Key Logic**:
```go
func (s *ImageScanner) ScanImage(ctx context.Context, imageRef string) (*models.ImageScanResult, error) {
    start := time.Now()
    
    // 1. Parse image reference
    imageName, imageTag := parseImageRef(imageRef)
    
    // 2. Check cache (< 24h old)
    cached, err := s.checkCache(imageName, imageTag)
    if err == nil && cached != nil {
        s.logger.Infof("Using cached scan for %s:%s", imageName, imageTag)
        return cached, nil
    }
    
    // 3. Perform scan via Trivy
    trivyResult, err := s.trivyScan(ctx, imageRef)
    if err != nil {
        return nil, fmt.Errorf("trivy scan failed: %w", err)
    }
    
    // 4. Convert to our model
    scanResult := &models.ImageScanResult{
        ImageName:      imageName,
        ImageTag:       imageTag,
        ImageDigest:    trivyResult.ArtifactName,
        ScannedAt:      time.Now(),
        ScannerVersion: trivyResult.Metadata.Version,
        ScanDurationSeconds: time.Since(start).Seconds(),
        Status:         "completed",
    }
    
    // 5. Process vulnerabilities
    for _, result := range trivyResult.Results {
        for _, vuln := range result.Vulnerabilities {
            // Count by severity
            switch vuln.Severity {
            case "CRITICAL":
                scanResult.CriticalCount++
            case "HIGH":
                scanResult.HighCount++
            case "MEDIUM":
                scanResult.MediumCount++
            case "LOW":
                scanResult.LowCount++
            }
        }
    }
    scanResult.TotalVulnerabilities = scanResult.CriticalCount + scanResult.HighCount + 
                                     scanResult.MediumCount + scanResult.LowCount
    
    // 6. Store vulnerabilities as JSON
    vulnsJSON, _ := json.Marshal(trivyResult.Results[0].Vulnerabilities)
    pkgsJSON, _ := json.Marshal(trivyResult.Results[0].Packages)
    scanResult.Vulnerabilities = vulnsJSON
    scanResult.Packages = pkgsJSON
    
    // 7. Save to database
    if err := s.saveScanResult(scanResult); err != nil {
        return nil, fmt.Errorf("failed to save scan result: %w", err)
    }
    
    s.logger.Infof("Scanned %s:%s - found %d vulnerabilities (Critical: %d, High: %d)",
        imageName, imageTag, scanResult.TotalVulnerabilities, 
        scanResult.CriticalCount, scanResult.HighCount)
    
    return scanResult, nil
}
```

**Configuration**:
```go
type ImageScannerConfig struct {
    TrivyServerURL string        `yaml:"trivy_server_url"` // http://trivy:8080
    CacheTTL       time.Duration `yaml:"cache_ttl"`        // 24h
    Timeout        time.Duration `yaml:"timeout"`          // 5m
}
```

---

### **4.4 Component 3: Pod Watcher**

**File**: `KSAM/core/pkg/scanner/pod_watcher.go`

**Chức năng**:
- Watch Kubernetes pod events
- Extract container images
- Trigger scans

**Struct**:
```go
type PodWatcher struct {
    db          *gorm.DB
    k8sClient   *kubernetes.Clientset
    scanner     *ImageScanner
    processor   *CVEProcessor
    clusterID   string
    logger      *logrus.Logger
}
```

**Main Functions**:
```go
// Start watches for pod events
func (w *PodWatcher) Start(ctx context.Context) error

// handlePodEvent processes pod create/update
func (w *PodWatcher) handlePodEvent(event watch.Event) error

// scanPod scans all containers in a pod
func (w *PodWatcher) scanPod(ctx context.Context, pod *corev1.Pod) error

// linkPodToScan creates pod_image_scans entry
func (w *PodWatcher) linkPodToScan(pod *corev1.Pod, container corev1.Container, scanID int) error
```

**Key Logic**:
```go
func (w *PodWatcher) Start(ctx context.Context) error {
    w.logger.Info("Starting pod watcher...")
    
    // Watch all namespaces
    watcher, err := w.k8sClient.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{})
    if err != nil {
        return fmt.Errorf("failed to create watcher: %w", err)
    }
    defer watcher.Stop()
    
    for {
        select {
        case event := <-watcher.ResultChan():
            if event.Type == watch.Added || event.Type == watch.Modified {
                if err := w.handlePodEvent(event); err != nil {
                    w.logger.Errorf("Failed to handle pod event: %v", err)
                }
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (w *PodWatcher) scanPod(ctx context.Context, pod *corev1.Pod) error {
    w.logger.Infof("Scanning pod %s/%s", pod.Namespace, pod.Name)
    
    // Scan each container
    for _, container := range pod.Spec.Containers {
        // 1. Scan image
        scanResult, err := w.scanner.ScanImage(ctx, container.Image)
        if err != nil {
            w.logger.Errorf("Failed to scan %s: %v", container.Image, err)
            continue
        }
        
        // 2. Link pod to scan
        if err := w.linkPodToScan(pod, container, scanResult.ID); err != nil {
            w.logger.Errorf("Failed to link pod to scan: %v", err)
            continue
        }
        
        // 3. Process vulnerabilities (create insights)
        if scanResult.CriticalCount > 0 || scanResult.HighCount > 0 {
            if err := w.processor.ProcessScanResult(ctx, pod, container.Name, scanResult); err != nil {
                w.logger.Errorf("Failed to process scan result: %v", err)
            }
        }
    }
    
    return nil
}
```

---

### **4.5 Component 4: CVE Processor**

**File**: `KSAM/core/pkg/scanner/cve_processor.go`

**Chức năng**:
- Filter vulnerabilities (CRITICAL + HIGH only)
- Create insights cho mỗi CVE
- Enrich với CVE database info

**Struct**:
```go
type CVEProcessor struct {
    db              *gorm.DB
    insightManager  *riskengine.InsightManager
    severityFilter  []string // ["CRITICAL", "HIGH"]
    logger          *logrus.Logger
}
```

**Main Functions**:
```go
// ProcessScanResult creates insights from scan
func (p *CVEProcessor) ProcessScanResult(ctx context.Context, pod *corev1.Pod, containerName string, scanResult *models.ImageScanResult) error

// filterVulnerabilities filters by severity
func (p *CVEProcessor) filterVulnerabilities(vulns []Vulnerability) []Vulnerability

// enrichCVE adds database info to CVE
func (p *CVEProcessor) enrichCVE(cveID string) (*models.CVE, error)

// createInsight creates insight for CVE
func (p *CVEProcessor) createInsight(pod *corev1.Pod, containerName string, vuln Vulnerability, cveInfo *models.CVE) error
```

**Key Logic**:
```go
func (p *CVEProcessor) ProcessScanResult(
    ctx context.Context,
    pod *corev1.Pod,
    containerName string,
    scanResult *models.ImageScanResult,
) error {
    // 1. Parse vulnerabilities from JSON
    var vulns []Vulnerability
    json.Unmarshal(scanResult.Vulnerabilities, &vulns)
    
    // 2. Filter by severity (CRITICAL + HIGH only)
    filtered := p.filterVulnerabilities(vulns)
    
    p.logger.Infof("Processing %d vulnerabilities for pod %s/%s", 
        len(filtered), pod.Namespace, pod.Name)
    
    // 3. Create insight for each CVE
    for _, vuln := range filtered {
        // Enrich with database info
        cveInfo, err := p.enrichCVE(vuln.VulnerabilityID)
        if err != nil {
            p.logger.Warnf("Failed to enrich CVE %s: %v", vuln.VulnerabilityID, err)
        }
        
        // Create insight
        if err := p.createInsight(pod, containerName, vuln, cveInfo); err != nil {
            p.logger.Errorf("Failed to create insight for %s: %v", vuln.VulnerabilityID, err)
            continue
        }
        
        p.logger.Infof("Created insight for %s in pod %s/%s",
            vuln.VulnerabilityID, pod.Namespace, pod.Name)
    }
    
    return nil
}

func (p *CVEProcessor) createInsight(
    pod *corev1.Pod,
    containerName string,
    vuln Vulnerability,
    cveInfo *models.CVE,
) error {
    // Build description
    description := fmt.Sprintf(`Pod '%s' in namespace '%s' is running container '%s' with vulnerable image.

Vulnerability Details:
• CVE ID: %s
• Severity: %s (CVSS %.1f)
• Package: %s
• Installed Version: %s
• Fixed Version: %s

Description: %s

Recommendation: Update the image to use version %s or later.`,
        pod.Name,
        pod.Namespace,
        containerName,
        vuln.VulnerabilityID,
        vuln.Severity,
        vuln.CVSSScore,
        vuln.PkgName,
        vuln.InstalledVersion,
        vuln.FixedVersion,
        vuln.Description,
        vuln.FixedVersion,
    )
    
    // Add exploit warning if available
    if cveInfo != nil && cveInfo.ExploitAvailable {
        description += fmt.Sprintf(`

⚠️  WARNING: Public exploit is available!
• Exploit Maturity: %s
• Sources: %s

This vulnerability is being actively exploited. Immediate patching is recommended.`,
            cveInfo.ExploitMaturity,
            strings.Join(cveInfo.ExploitSources, ", "),
        )
    }
    
    // Create insight
    insight := &models.Insight{
        Type:        "vulnerability",
        Severity:    mapSeverity(vuln.Severity),
        Title:       fmt.Sprintf("Container has vulnerable image: %s", vuln.VulnerabilityID),
        Description: description,
        AffectedResources: []models.Resource{{
            Type:      "Pod",
            UID:       string(pod.UID),
            Name:      pod.Name,
            Namespace: pod.Namespace,
        }},
        Source: "cve-scanner",
        Status: "active",
        
        // CVE-specific fields
        CVEID:       vuln.VulnerabilityID,
        CVSSScore:   &vuln.CVSSScore,
        PackageName: vuln.PkgName,
        FixedVersion: vuln.FixedVersion,
        
        // Additional details
        Details: map[string]interface{}{
            "container_name":    containerName,
            "image":             fmt.Sprintf("%s:%s", scanResult.ImageName, scanResult.ImageTag),
            "installed_version": vuln.InstalledVersion,
            "references":        vuln.References,
        },
    }
    
    // Save insight (this triggers risk scoring automatically!)
    return p.insightManager.CreateInsight(insight)
}
```

---

## 5. DEPLOYMENTS

### **5.1 Cần Deploy Gì?**

**CẦN 2 DEPLOYMENTS MỚI**:

```
1. Trivy Server (third-party)
   └─ Aqua Security's container scanner
   
2. KSAM Scanner (new service)
   └─ Our CVE integration service
```

### **5.2 Deployment 1: Trivy Server**

**File**: `KSAM/deployments/trivy-server.yaml`

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ksam-system

---
# PersistentVolumeClaim for cache
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: trivy-cache
  namespace: ksam-system
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi

---
# Trivy Server Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: trivy-server
  namespace: ksam-system
spec:
  replicas: 2
  selector:
    matchLabels:
      app: trivy-server
  template:
    metadata:
      labels:
        app: trivy-server
    spec:
      containers:
      - name: trivy
        image: aquasec/trivy:0.45.0
        args:
          - server
          - --listen=0.0.0.0:8080
          - --cache-dir=/root/.cache
        ports:
        - containerPort: 8080
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 2000m
            memory: 2Gi
        volumeMounts:
        - name: cache
          mountPath: /root/.cache
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
      volumes:
      - name: cache
        persistentVolumeClaim:
          claimName: trivy-cache

---
# Service
apiVersion: v1
kind: Service
metadata:
  name: trivy-server
  namespace: ksam-system
spec:
  selector:
    app: trivy-server
  ports:
  - port: 8080
    targetPort: 8080
```

**Deploy commands**:
```bash
# Step 1: Create namespace
kubectl create namespace ksam-system

# Step 2: Deploy Trivy
kubectl apply -f deployments/trivy-server.yaml

# Step 3: Wait for ready
kubectl wait --for=condition=available deployment/trivy-server -n ksam-system --timeout=300s

# Step 4: Test
kubectl run test-trivy --rm -i --restart=Never --image=curlimages/curl -- \
  curl http://trivy-server.ksam-system:8080/healthz

# Expected output: "OK"
```

---

### **5.3 Deployment 2: KSAM Scanner**

**File**: `KSAM/deployments/ksam-scanner.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ksam-scanner-config
  namespace: ksam-system
data:
  config.yaml: |
    trivy_server_url: http://trivy-server.ksam-system:8080
    cluster_id: prod-cluster-1
    database:
      host: postgres.ksam-system
      port: 5432
      name: ksam
      user: ksam
    cve_updater:
      enabled: true
      update_interval: 24h
    image_scanner:
      cache_ttl: 24h
      timeout: 5m
    pod_watcher:
      enabled: true
    cve_processor:
      severity_filter: [CRITICAL, HIGH]

---
apiVersion: v1
kind: Secret
metadata:
  name: ksam-scanner-secrets
  namespace: ksam-system
type: Opaque
stringData:
  nvd-api-key: "YOUR_NVD_API_KEY_HERE"
  db-password: "YOUR_DB_PASSWORD_HERE"

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ksam-scanner
  namespace: ksam-system
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ksam-scanner
  template:
    metadata:
      labels:
        app: ksam-scanner
    spec:
      serviceAccountName: ksam-scanner
      containers:
      - name: scanner
        image: ksam/scanner:latest
        env:
        - name: CONFIG_PATH
          value: /etc/ksam/config.yaml
        - name: NVD_API_KEY
          valueFrom:
            secretKeyRef:
              name: ksam-scanner-secrets
              key: nvd-api-key
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: ksam-scanner-secrets
              key: db-password
        resources:
          requests:
            cpu: 250m
            memory: 256Mi
          limits:
            cpu: 1000m
            memory: 1Gi
        volumeMounts:
        - name: config
          mountPath: /etc/ksam
      volumes:
      - name: config
        configMap:
          name: ksam-scanner-config

---
# ServiceAccount with RBAC
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ksam-scanner
  namespace: ksam-system

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ksam-scanner
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ksam-scanner
subjects:
- kind: ServiceAccount
  name: ksam-scanner
  namespace: ksam-system
roleRef:
  kind: ClusterRole
  name: ksam-scanner
  apiGroup: rbac.authorization.k8s.io
```

**Deploy commands**:
```bash
# Step 1: Get NVD API key
# Register at https://nvd.nist.gov/developers/request-an-api-key

# Step 2: Create secrets
kubectl create secret generic ksam-scanner-secrets \
  --from-literal=nvd-api-key=YOUR_NVD_API_KEY \
  --from-literal=db-password=YOUR_DB_PASSWORD \
  -n ksam-system

# Step 3: Deploy scanner
kubectl apply -f deployments/ksam-scanner.yaml

# Step 4: Wait for ready
kubectl wait --for=condition=available deployment/ksam-scanner -n ksam-system --timeout=300s

# Step 5: Check logs
kubectl logs -n ksam-system deployment/ksam-scanner --tail=50

# Expected output:
# INFO Starting CVE updater...
# INFO Starting pod watcher...
# INFO CVE updater: fetched 150 CVEs
# INFO Pod watcher: watching for events
```

---

## 6. INTEGRATION POINTS

### **6.1 Modify Risk Scorer**

**File**: `KSAM/core/pkg/risk/scorer.go`

**Changes**:
```go
// Add CVE-specific scoring logic
func (s *Scorer) CalculateScore(resourceUID string) (*RiskScore, error) {
    // ... existing code ...
    
    // NEW: Handle CVE insights differently
    for _, insight := range insights {
        if insight.Type == "vulnerability" && insight.CVEID != "" {
            // CVE-specific scoring
            s.appl yCVEScoring(insight, &score)
        } else {
            // Existing scoring logic
            s.applyStandardScoring(insight, &score)
        }
    }
    
    // ... rest of code ...
}

func (s *Scorer) applyCVEScoring(insight *Insight, score *RiskScore) {
    // Use CVSS score directly
    if insight.CVSSScore != nil {
        score.BaseScore = *insight.CVSSScore * 10 // Scale to 0-100
    }
    
    // Boost if exploit available
    var cve models.CVE
    s.db.Where("cve_id = ?", insight.CVEID).First(&cve)
    if cve.ExploitAvailable {
        score.BaseScore *= 1.2 // 20% boost
    }
    
    // ... rest of CVE scoring logic ...
}
```

### **6.2 Add API Endpoints**

**File**: `KSAM/core/internal/server/cve_handlers.go` (NEW FILE)

```go
package server

// GET /api/v1/cves
func (h *CVEHandler) listCVEs(c *gin.Context) {
    // List CVEs with filters
}

// GET /api/v1/cves/:cve_id
func (h *CVEHandler) getCVE(c *gin.Context) {
    // Get single CVE detail
}

// GET /api/v1/images/scan-results
func (h *CVEHandler) listScanResults(c *gin.Context) {
    // List scan results
}

// GET /api/v1/pods/:pod_uid/vulnerabilities
func (h *CVEHandler) getPodVulnerabilities(c *gin.Context) {
    // Get vulnerabilities for specific pod
}

// POST /api/v1/images/scan
func (h *CVEHandler) triggerScan(c *gin.Context) {
    // Manually trigger image scan
}
```

### **6.3 Update Dashboard**

**Changes needed**:
```
Frontend (React):
├─ Add CVE tab
├─ Add vulnerability charts
├─ Add scan results view
└─ Add CVE details modal

API calls:
├─ GET /api/v1/cves (list CVEs)
├─ GET /api/v1/pods/:id/vulnerabilities
└─ GET /api/v1/images/scan-results
```

---

## 7. TESTING PLAN

### **7.1 Unit Tests**

```bash
# Test CVE Updater
go test -v ./pkg/scanner -run TestCVEUpdater

# Test Image Scanner
go test -v ./pkg/scanner -run TestImageScanner

# Test Pod Watcher
go test -v ./pkg/scanner -run TestPodWatcher

# Test CVE Processor
go test -v ./pkg/scanner -run TestCVEProcessor
```

### **7.2 Integration Tests**

```bash
# End-to-end test
./scripts/test-cve-integration.sh

# Script content:
# 1. Create test pod with nginx:1.19.0
# 2. Wait for scan
# 3. Verify insights created
# 4. Verify risk scores calculated
# 5. Cleanup
```

### **7.3 Manual Testing**

```bash
# Test 1: Create vulnerable pod
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-nginx
spec:
  containers:
  - name: nginx
    image: nginx:1.19.0
EOF

# Wait 2 minutes for scan
sleep 120

# Check scan result
kubectl logs -n ksam-system deployment/ksam-scanner | grep "nginx:1.19.0"

# Check insights
curl http://ksam-api/api/v1/insights?source=cve-scanner

# Expected: 21 insights for CVE-2021-23017 and others

# Check risk score
curl http://ksam-api/api/v1/risks?resource_uid=$(kubectl get pod test-nginx -o jsonpath='{.metadata.uid}')

# Expected: TotalScore ~85-100, Priority P0 or P1

# Cleanup
kubectl delete pod test-nginx
```

---

## 8. TIMELINE CHI TIẾT

### **Week 1 (5 days)**

```
Day 1: Database Setup
├─ 09:00-12:00: Write migration script
├─ 13:00-15:00: Test migration locally
├─ 15:00-17:00: Document schema
└─ End of day: Migration ready ✅

Day 2: CVE Updater
├─ 09:00-12:00: Implement CVE Updater
├─ 13:00-15:00: Test NVD API integration
├─ 15:00-17:00: Unit tests
└─ End of day: CVE Updater working ✅

Day 3: Image Scanner (Part 1)
├─ 09:00-12:00: Implement core scanning logic
├─ 13:00-15:00: Trivy API integration
├─ 15:00-17:00: Cache implementation
└─ End of day: Basic scanner working ✅

Day 4: Image Scanner (Part 2)
├─ 09:00-12:00: Result parsing
├─ 13:00-15:00: Database storage
├─ 15:00-17:00: Unit tests
└─ End of day: Scanner complete ✅

Day 5: Pod Watcher
├─ 09:00-12:00: Kubernetes watch implementation
├─ 13:00-15:00: Event handling
├─ 15:00-17:00: Integration with scanner
└─ End of day: Pod watcher working ✅
```

### **Week 2 (5 days)**

```
Day 6: CVE Processor
├─ 09:00-12:00: Vulnerability filtering
├─ 13:00-15:00: Insight creation
├─ 15:00-17:00: Enrichment logic
└─ End of day: Processor complete ✅

Day 7: Trivy Deployment
├─ 09:00-12:00: Deploy Trivy to staging
├─ 13:00-15:00: Test Trivy API
├─ 15:00-17:00: Performance tuning
└─ End of day: Trivy deployed ✅

Day 8: Scanner Deployment
├─ 09:00-12:00: Build Docker image
├─ 13:00-15:00: Deploy to staging
├─ 15:00-17:00: Verify logs and functionality
└─ End of day: Scanner deployed ✅

Day 9: Integration (Part 1)
├─ 09:00-12:00: Modify Risk Scorer
├─ 13:00-15:00: Add API endpoints
├─ 15:00-17:00: Unit tests
└─ End of day: Backend integration done ✅

Day 10: Integration (Part 2)
├─ 09:00-12:00: Frontend changes
├─ 13:00-15:00: Dashboard updates
├─ 15:00-17:00: UI testing
└─ End of day: Full integration complete ✅
```

### **Week 3 (5 days)**

```
Day 11-13: Testing
├─ Unit tests
├─ Integration tests
├─ End-to-end tests
└─ Bug fixes

Day 14-15: Documentation
├─ User guide
├─ API documentation
├─ Runbook
└─ Architecture docs
```

### **Week 4 (5 days)**

```
Day 16-17: Staging Deployment
├─ Deploy to staging
├─ Monitor for issues
├─ Performance tuning
└─ Security review

Day 18-19: Production Deployment
├─ Deploy to production
├─ Gradual rollout
├─ Monitor metrics
└─ On-call support

Day 20: Wrap-up
├─ Final documentation
├─ Knowledge transfer
├─ Retrospective
└─ Project complete ✅
```

---

## 📝 FINAL CHECKLIST

```
☐ Phase 1: Database
  ☐ 4 tables created
  ☐ Indexes added
  ☐ Views created
  ☐ Migration tested
  
☐ Phase 2: Code
  ☐ CVE Updater implemented
  ☐ Image Scanner implemented
  ☐ Pod Watcher implemented
  ☐ CVE Processor implemented
  ☐ All unit tests pass
  
☐ Phase 3: Deployment
  ☐ Trivy Server deployed
  ☐ KSAM Scanner deployed
  ☐ Secrets configured
  ☐ RBAC configured
  
☐ Phase 4: Integration
  ☐ Risk Scorer modified
  ☐ API endpoints added
  ☐ Dashboard updated
  
☐ Phase 5: Testing
  ☐ Unit tests: 100+ tests
  ☐ Integration tests pass
  ☐ E2E tests pass
  ☐ Manual testing complete
  
☐ Phase 6: Production
  ☐ Deployed to staging
  ☐ Deployed to production
  ☐ Monitoring configured
  ☐ Documentation complete
```

---

**Status**: ✅ **MASTER PLAN COMPLETE**  
**Timeline**: 20 days (4 weeks)  
**Effort**: 1 engineer full-time  
**Result**: Full CVE detection + risk scoring trong KSAM
