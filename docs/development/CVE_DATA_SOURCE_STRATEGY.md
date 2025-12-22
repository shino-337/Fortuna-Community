# CVE Data Source Strategy - Optimal Approaches

**Date**: 2024-12-20  
**Status**: 📋 Analysis & Recommendations

---

## Executive Summary

### Current State
- ✅ CVE data stored in `/cve-data/all` (74,561 JSON files)
- ✅ Incremental updates supported
- ⚠️ Manual sync required
- ❌ No auto-sync from NVD/OSV.dev

### Goal
- **Built-in CVE data** hoặc **Cloud storage** với **auto-sync** từ NVD/OSV.dev
- **Optimal approach** cho production deployment

---

## Phương Án So Sánh

### Option 1: Built-in CVE Data (Embedded)

**Approach**: Embed CVE data vào Docker image hoặc binary

**Implementation**:
```dockerfile
# Dockerfile
FROM golang:1.24 AS builder
WORKDIR /app
COPY cve-data/ /app/cve-data/
COPY . .
RUN go build -o /app/bin/core ./cmd/main.go

FROM alpine:3.20
COPY --from=builder /app/bin/core /usr/local/bin/
COPY --from=builder /app/cve-data /cve-data
ENTRYPOINT ["/usr/local/bin/core"]
```

**Pros**:
- ✅ **Self-contained**: Không cần external storage
- ✅ **Fast access**: Local filesystem, no network
- ✅ **Offline capable**: Works without internet
- ✅ **Simple deployment**: Single image

**Cons**:
- ❌ **Large image size**: ~500MB-1GB (74k files)
- ❌ **Slow builds**: Copy 74k files mỗi lần build
- ❌ **Update requires rebuild**: Phải rebuild image để update CVE
- ❌ **Version coupling**: CVE data tied to application version

**Use Case**: 
- Small deployments
- Air-gapped environments
- Development/testing

---

### Option 2: Cloud Storage (S3/GCS/Azure Blob)

**Approach**: Store CVE data trong cloud storage, download on startup

**Implementation**:
```go
// On startup
func InitCVEData() error {
    // Download from S3
    s3Client := s3.New(session.New())
    err := downloadFromS3("ksam-cve-data", "all/", "/cve-data/all/")
    return err
}
```

**Pros**:
- ✅ **Centralized**: Single source of truth
- ✅ **Easy updates**: Update S3, all instances get new data
- ✅ **Small image**: Image chỉ chứa application code
- ✅ **Version independent**: CVE data separate from app version
- ✅ **Cost effective**: S3 storage rất rẻ (~$0.023/GB/month)

**Cons**:
- ⚠️ **Network dependency**: Cần internet để download
- ⚠️ **Startup delay**: First download có thể mất 1-2 phút
- ⚠️ **Storage cost**: ~500MB-1GB storage

**Use Case**:
- Production deployments
- Multi-region deployments
- Frequent CVE updates

---

### Option 3: Hybrid (Built-in + Cloud Sync)

**Approach**: Built-in baseline + incremental sync từ cloud

**Implementation**:
```go
// Built-in: Latest snapshot (updated weekly)
// Cloud: Daily incremental updates
func InitCVEData() error {
    // 1. Use built-in baseline (fast)
    baselinePath := "/cve-data/baseline"
    
    // 2. Download incremental updates from cloud
    updates := downloadIncrementalUpdates()
    
    // 3. Merge updates
    mergeUpdates(baselinePath, updates)
}
```

**Pros**:
- ✅ **Fast startup**: Baseline available immediately
- ✅ **Small updates**: Chỉ download changes
- ✅ **Offline capable**: Works với baseline nếu cloud unavailable
- ✅ **Best of both**: Combines benefits

**Cons**:
- ⚠️ **Complexity**: Cần merge logic
- ⚠️ **Baseline maintenance**: Phải update baseline định kỳ

**Use Case**:
- Production deployments
- High availability requirements
- Balanced approach

---

### Option 4: Auto-Sync từ NVD/OSV.dev (Recommended)

**Approach**: Tự động download và sync từ NVD/OSV.dev APIs

**Implementation**:
```go
// Background worker
func CVESyncWorker() {
    ticker := time.NewTicker(6 * time.Hour)
    for {
        <-ticker.C
        
        // 1. Fetch new CVEs from OSV.dev
        newCVEs := fetchFromOSV()
        
        // 2. Download JSON files
        downloadCVEFiles(newCVEs)
        
        // 3. Process incremental update
        incrementalUpdate()
    }
}
```

**Pros**:
- ✅ **Always up-to-date**: Real-time CVE data
- ✅ **No manual sync**: Fully automated
- ✅ **Source of truth**: Direct from authoritative sources
- ✅ **Flexible**: Có thể sync từ multiple sources

**Cons**:
- ⚠️ **API rate limits**: NVD có rate limits
- ⚠️ **Network dependency**: Cần internet
- ⚠️ **Complexity**: Cần handle API errors, retries

**Use Case**:
- Production deployments
- Real-time CVE detection
- Multi-source CVE data

---

## Recommended Solution: Hybrid Auto-Sync

### Architecture

```
┌─────────────────────────────────────────┐
│  CVE Data Sources                       │
├─────────────────────────────────────────┤
│  • OSV.dev API (primary)                │
│  • NVD API (secondary/fallback)         │
│  • GitHub OSV Database (backup)         │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│  CVE Sync Service                        │
│  (Background Worker)                     │
├─────────────────────────────────────────┤
│  1. Fetch new/updated CVEs              │
│  2. Download JSON files                  │
│  3. Store in local/cloud storage         │
│  4. Trigger incremental update           │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│  Storage Layer                          │
├─────────────────────────────────────────┤
│  Option A: Local filesystem             │
│    /cve-data/all/*.json                 │
│                                          │
│  Option B: Cloud storage (S3/GCS)       │
│    s3://ksam-cve-data/all/*.json        │
│                                          │
│  Option C: Database (PostgreSQL)         │
│    Already implemented ✅                │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│  CVE Loader                             │
│  (Incremental Update)                   │
├─────────────────────────────────────────┤
│  • Detect new/changed files              │
│  • Bulk load to PostgreSQL              │
│  • Update metadata                      │
└─────────────────────────────────────────┘
```

---

## Implementation Plan

### Phase 1: Auto-Sync Service

**File**: `KSAM/core/pkg/cve/sync/sync_service.go` (new)

```go
package sync

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "time"
    
    "gorm.io/gorm"
)

// SyncService manages automatic CVE data synchronization
type SyncService struct {
    db            *gorm.DB
    sourceDir     string
    osvAPIBaseURL string
    nvdAPIBaseURL string
    httpClient    *http.Client
    logger        *log.Logger
}

// NewSyncService creates a new CVE sync service
func NewSyncService(db *gorm.DB, sourceDir string) *SyncService {
    return &SyncService{
        db:            db,
        sourceDir:     sourceDir,
        osvAPIBaseURL: "https://osv.dev/api/v1",
        nvdAPIBaseURL: "https://services.nvd.nist.gov/rest/json/cves/2.0",
        httpClient: &http.Client{
            Timeout: 30 * time.Minute,
        },
        logger: log.New(log.Writer(), "[CVESync] ", log.LstdFlags),
    }
}

// Start starts the background sync worker
func (s *SyncService) Start(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    // Initial sync on startup
    s.logger.Println("Starting initial CVE sync...")
    if err := s.Sync(ctx); err != nil {
        s.logger.Printf("Initial sync failed: %v", err)
    }
    
    // Periodic sync
    for {
        select {
        case <-ticker.C:
            s.logger.Println("Starting periodic CVE sync...")
            if err := s.Sync(ctx); err != nil {
                s.logger.Printf("Sync failed: %v", err)
            }
        case <-ctx.Done():
            s.logger.Println("Stopping CVE sync service...")
            return
        }
    }
}

// Sync performs a full sync from OSV.dev
func (s *SyncService) Sync(ctx context.Context) error {
    // Step 1: Get list of all CVEs from OSV.dev
    cveList, err := s.fetchCVEList(ctx)
    if err != nil {
        return fmt.Errorf("failed to fetch CVE list: %w", err)
    }
    
    s.logger.Printf("Found %d CVEs to sync", len(cveList))
    
    // Step 2: Download each CVE JSON file
    downloaded := 0
    for _, cveID := range cveList {
        if err := s.downloadCVE(ctx, cveID); err != nil {
            s.logger.Printf("Failed to download %s: %v", cveID, err)
            continue
        }
        downloaded++
        
        // Rate limiting: 10 requests/second
        time.Sleep(100 * time.Millisecond)
    }
    
    s.logger.Printf("Downloaded %d CVE files", downloaded)
    
    // Step 3: Trigger incremental update
    // This will use existing incremental tracker
    tracker := loader.NewIncrementalTracker(s.db, s.sourceDir, false)
    files, err := tracker.GetFilesToProcess(ctx)
    if err != nil {
        return fmt.Errorf("failed to get files to process: %w", err)
    }
    
    if len(files) > 0 {
        bulkLoader := loader.NewBulkLoader(s.db, 20, 500, 1000)
        if err := bulkLoader.LoadFiles(ctx, files); err != nil {
            return fmt.Errorf("bulk load failed: %w", err)
        }
        
        tracker.MarkProcessingComplete(ctx, files)
        s.logger.Printf("Processed %d new/changed CVE files", len(files))
    }
    
    return nil
}

// fetchCVEList gets list of all CVE IDs from OSV.dev
func (s *SyncService) fetchCVEList(ctx context.Context) ([]string, error) {
    // Option 1: Use OSV.dev query API
    // GET /v1/query
    // This returns all vulnerabilities
    
    // Option 2: Use GitHub OSV database
    // https://github.com/google/osv.dev/tree/main/vuln
    // This is more reliable for bulk download
    
    // For now, use GitHub API to list all CVE files
    url := "https://api.github.com/repos/google/osv.dev/contents/vuln"
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    resp, err := s.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var files []struct {
        Name string `json:"name"`
        Type string `json:"type"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
        return nil, err
    }
    
    var cveList []string
    for _, file := range files {
        if file.Type == "file" && filepath.Ext(file.Name) == ".json" {
            cveList = append(cveList, file.Name[:len(file.Name)-5]) // Remove .json
        }
    }
    
    return cveList, nil
}

// downloadCVE downloads a single CVE JSON file
func (s *SyncService) downloadCVE(ctx context.Context, cveID string) error {
    // Download from GitHub raw content
    url := fmt.Sprintf("https://raw.githubusercontent.com/google/osv.dev/main/vuln/%s.json", cveID)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return err
    }
    
    resp, err := s.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
    }
    
    // Save to local filesystem
    filePath := filepath.Join(s.sourceDir, fmt.Sprintf("%s.json", cveID))
    file, err := os.Create(filePath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    if _, err := io.Copy(file, resp.Body); err != nil {
        return err
    }
    
    return nil
}
```

---

### Phase 2: Cloud Storage Integration (Optional)

**File**: `KSAM/core/pkg/cve/storage/cloud_storage.go` (new)

```go
package storage

import (
    "context"
    "fmt"
    "io"
    "path/filepath"
    
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
)

// CloudStorage manages CVE data in cloud storage
type CloudStorage struct {
    s3Client *s3.S3
    bucket   string
    prefix   string
}

// NewCloudStorage creates a new cloud storage manager
func NewCloudStorage(bucket, prefix string) (*CloudStorage, error) {
    sess, err := session.NewSession(&aws.Config{
        Region: aws.String("us-east-1"), // Or from config
    })
    if err != nil {
        return nil, err
    }
    
    return &CloudStorage{
        s3Client: s3.New(sess),
        bucket:   bucket,
        prefix:   prefix,
    }, nil
}

// DownloadAll downloads all CVE files from S3
func (cs *CloudStorage) DownloadAll(ctx context.Context, localDir string) error {
    // List all objects in bucket/prefix
    listInput := &s3.ListObjectsV2Input{
        Bucket: aws.String(cs.bucket),
        Prefix: aws.String(cs.prefix),
    }
    
    err := cs.s3Client.ListObjectsV2PagesWithContext(ctx, listInput, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
        for _, obj := range page.Contents {
            key := *obj.Key
            localPath := filepath.Join(localDir, filepath.Base(key))
            
            // Download object
            if err := cs.downloadObject(ctx, key, localPath); err != nil {
                log.Printf("Failed to download %s: %v", key, err)
                continue
            }
        }
        return true
    })
    
    return err
}

// Upload uploads a CVE file to S3
func (cs *CloudStorage) Upload(ctx context.Context, localPath, cveID string) error {
    file, err := os.Open(localPath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    key := fmt.Sprintf("%s/%s.json", cs.prefix, cveID)
    
    _, err = cs.s3Client.PutObjectWithContext(ctx, &s3.PutObjectInput{
        Bucket: aws.String(cs.bucket),
        Key:    aws.String(key),
        Body:   file,
    })
    
    return err
}

func (cs *CloudStorage) downloadObject(ctx context.Context, key, localPath string) error {
    result, err := cs.s3Client.GetObjectWithContext(ctx, &s3.GetObjectInput{
        Bucket: aws.String(cs.bucket),
        Key:    aws.String(key),
    })
    if err != nil {
        return err
    }
    defer result.Body.Close()
    
    file, err := os.Create(localPath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    _, err = io.Copy(file, result.Body)
    return err
}
```

---

### Phase 3: Integration vào Core Service

**File**: `KSAM/core/cmd/main.go` (modify)

```go
// Add CVE sync service
if os.Getenv("KSAM_CVE_AUTO_SYNC") == "true" {
    syncInterval := 6 * time.Hour // Default: every 6 hours
    if intervalStr := os.Getenv("KSAM_CVE_SYNC_INTERVAL"); intervalStr != "" {
        if parsed, err := time.ParseDuration(intervalStr); err == nil {
            syncInterval = parsed
        }
    }
    
    syncService := sync.NewSyncService(db, "/cve-data/all")
    go syncService.Start(ctx, syncInterval)
    log.Printf("✅ CVE auto-sync enabled (interval: %v)", syncInterval)
}
```

---

## Deployment Options

### Option A: Local Filesystem (Current)

**Kubernetes Deployment**:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: cve-data-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 2Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ksam-core
spec:
  template:
    spec:
      containers:
      - name: core
        volumeMounts:
        - name: cve-data
          mountPath: /cve-data
      volumes:
      - name: cve-data
        persistentVolumeClaim:
          claimName: cve-data-pvc
```

**Pros**: Simple, no external dependencies  
**Cons**: Requires PVC, not shared across pods

---

### Option B: Cloud Storage (S3/GCS)

**Kubernetes Deployment**:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aws-credentials
type: Opaque
data:
  access-key-id: <base64>
  secret-access-key: <base64>
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ksam-core
spec:
  template:
    spec:
      containers:
      - name: core
        env:
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: access-key-id
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: secret-access-key
        - name: KSAM_CVE_STORAGE_TYPE
          value: "s3"
        - name: KSAM_CVE_S3_BUCKET
          value: "ksam-cve-data"
```

**Pros**: Shared storage, easy updates  
**Cons**: Requires cloud credentials, network dependency

---

### Option C: Init Container + Cloud Storage

**Kubernetes Deployment**:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ksam-core
spec:
  template:
    spec:
      initContainers:
      - name: download-cve-data
        image: awscli:latest
        command:
        - sh
        - -c
        - |
          aws s3 sync s3://ksam-cve-data/all /cve-data/all
        volumeMounts:
        - name: cve-data
          mountPath: /cve-data
      containers:
      - name: core
        volumeMounts:
        - name: cve-data
          mountPath: /cve-data
      volumes:
      - name: cve-data
        emptyDir: {}
```

**Pros**: Fast startup (parallel download), no PVC needed  
**Cons**: Download on every pod restart

---

## Recommended Approach: Hybrid Auto-Sync

### Architecture

```
┌─────────────────────────────────────────┐
│  CVE Sync Service (Background)         │
│  • Fetches from OSV.dev GitHub          │
│  • Downloads new/updated CVEs           │
│  • Stores in local filesystem           │
│  • Triggers incremental update          │
│  • Runs every 6 hours                   │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│  Local Storage                          │
│  /cve-data/all/*.json                   │
│  (PersistentVolume or emptyDir)         │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│  Incremental Tracker                    │
│  • Detects new/changed files             │
│  • Processes only changes                │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│  PostgreSQL Database                     │
│  • cves table                            │
│  • package_vulnerabilities table          │
│  • cve_file_metadata table               │
└─────────────────────────────────────────┘
```

### Benefits

1. ✅ **Auto-updates**: Không cần manual intervention
2. ✅ **Efficient**: Chỉ download changes
3. ✅ **Fast**: Incremental updates (4-9 seconds)
4. ✅ **Reliable**: Local storage + database backup
5. ✅ **Scalable**: Works với multiple pods

---

## Implementation Priority

### Phase 1: Auto-Sync Service (High Priority)
- ✅ Implement `sync_service.go`
- ✅ Integrate vào `main.go`
- ✅ Test với OSV.dev GitHub API
- **Time**: 2-3 days

### Phase 2: Cloud Storage (Optional)
- ⚠️ Implement S3/GCS integration
- ⚠️ Add init container support
- **Time**: 1-2 days

### Phase 3: NVD API Integration (Future)
- ⚠️ Add NVD API as secondary source
- ⚠️ Handle rate limits
- **Time**: 2-3 days

---

## Cost Analysis

### Option 1: Local Storage (PVC)
- **Storage**: 2GB PVC = ~$0.20/month (AWS EBS)
- **Network**: $0 (local)
- **Total**: ~$0.20/month

### Option 2: S3 Storage
- **Storage**: 1GB S3 = ~$0.023/month
- **Requests**: 100k requests = ~$0.005/month
- **Data Transfer**: $0 (same region)
- **Total**: ~$0.03/month

### Option 3: Built-in Image
- **Image Size**: +500MB = ~$0.01/month (ECR storage)
- **Build Time**: +2-3 minutes
- **Total**: ~$0.01/month + build overhead

**Winner**: S3 Storage (cheapest, most flexible)

---

## Summary

### Recommended Solution

**Hybrid Auto-Sync với Local Storage**:
1. ✅ **Auto-sync service** từ OSV.dev GitHub
2. ✅ **Local filesystem** storage (PVC hoặc emptyDir)
3. ✅ **Incremental updates** (existing implementation)
4. ✅ **Background worker** chạy mỗi 6 giờ

**Why This Approach**:
- ✅ Fully automated (no manual intervention)
- ✅ Fast updates (4-9 seconds)
- ✅ Cost effective (~$0.20/month)
- ✅ Reliable (local storage + database)
- ✅ Scalable (works với multiple pods)

**Next Steps**:
1. Implement `sync_service.go`
2. Add to `main.go` với environment variable
3. Test với OSV.dev GitHub API
4. Deploy và monitor

---

**Last Updated**: 2024-12-20  
**Status**: 📋 Ready for Implementation

