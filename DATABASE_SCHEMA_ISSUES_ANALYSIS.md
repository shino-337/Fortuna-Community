# Phân Tích & Đánh Giá Database Schema Issues

**Ngày phân tích:** 2024-12-23  
**Codebase:** Fortuna Core & Agent (Agent-Based Architecture)

---

## 📊 Tổng Quan Đánh Giá

| Issue # | Mức Độ | Trạng Thái | Ghi Chú |
|---------|--------|------------|---------|
| 1. Schema Mismatch | 🔴 CRITICAL | ✅ **ĐÃ FIX** | User đã fix component_id → package_name |
| 2. Insight Deduplication | 🟡 MEDIUM | ⚠️ **PARTIALLY FIXED** | Vẫn sequential nhưng đã optimize query |
| 3. N+1 Query Pattern | 🔴 CRITICAL | ❌ **CHƯA FIX** | Vẫn còn N+1 trong cve_matcher_worker |
| 4. Connection Pool Monitoring | 🟡 MEDIUM | ❌ **CHƯA FIX** | Không có metrics |
| 5. NATS Retention | 🟡 MEDIUM | ⚠️ **VẪN CÒN** | Vẫn 1 hour cho ksam-raw/normalized |
| 6. Missing Batch Operations | 🔴 CRITICAL | ❌ **CHƯA FIX** | Vẫn query CVE per-package |
| 7. Data Synchronization | 🟡 MEDIUM | ❌ **CHƯA FIX** | Không có reconciliation loop |

---

## 🔍 Chi Tiết Từng Issue

### ✅ Issue #1: Database Index Mismatch - **ĐÃ FIX**

**Báo cáo gốc:**
- Location: `core/pkg/worker/cve_matcher_worker.go:146`
- Bug: Sử dụng `component_id` nhưng schema dùng `package_name`

**Thực tế code hiện tại:**
```go
// core/pkg/worker/cve_matcher_worker.go:146
Columns: []clause.Column{{Name: "sbom_id"}, {Name: "package_name"}, {Name: "cve_id"}},
```

**✅ Đánh giá:** 
- **ĐÃ FIX** - User đã sửa đúng từ `component_id` → `package_name`
- Code hiện tại khớp với schema mới (Agent-Based Architecture)
- Schema model (`core/pkg/models/sbom.go:86`) sử dụng `PackageName` ✅

**⚠️ Lưu ý Migration:**
- Migration `023_fix_sbom_cve_indexes.go:77` vẫn tạo index với `component_id`:
  ```sql
  CREATE UNIQUE INDEX idx_cve_matches_unique_sbom_component_cve
    ON cve_matches(sbom_id, component_id, cve_id)
  ```
- **CẦN UPDATE MIGRATION** để khớp với code:
  ```sql
  CREATE UNIQUE INDEX idx_cve_matches_unique_sbom_package_cve
    ON cve_matches(sbom_id, package_name, cve_id)
    WHERE deleted_at IS NULL;
  ```

---

### ⚠️ Issue #2: Inefficient Insight Deduplication - **PARTIALLY FIXED**

**Báo cáo gốc:**
- Location: `core/pkg/riskengine/insight_manager.go:36-38`
- Vấn đề: Multiple sequential queries, no batching

**Thực tế code hiện tại:**
```go
// core/pkg/riskengine/insight_manager.go:36-37
query := tx.Where("insight_type = ? AND resource_uid = ? AND cve_id = ? AND (status = ? OR status IS NULL) AND deleted_at IS NULL",
    "vulnerability", insight.ResourceUID, insight.CVEID, "active")
```

**✅ Cải thiện:**
- Đã loại bỏ JSONB parsing (`AffectedResources`) → dùng direct fields (`ResourceUID`, `CVEID`)
- Query đơn giản hơn, sử dụng composite index hiệu quả hơn

**❌ Vẫn còn vấn đề:**
```go
// core/pkg/riskengine/insight_manager.go:201-209
func (m *InsightManager) BatchCreateOrUpdateInsights(insights []*models.Insight) error {
    return m.db.Transaction(func(tx *gorm.DB) error {
        for _, insight := range insights {  // ⚠️ Vẫn loop sequential
            if err := m.createOrUpdateInsightTx(tx, insight); err != nil {
                return err
            }
        }
        return nil
    })
}
```

**Đánh giá:**
- ✅ Query đã được optimize (không còn JSONB parsing)
- ❌ Vẫn sequential processing (N queries cho N insights)
- 💡 **Khuyến nghị:** Implement PostgreSQL UPSERT batch như báo cáo đề xuất

---

### ❌ Issue #3: N+1 Query Pattern - **CHƯA FIX**

**Báo cáo gốc:**
- Location: `core/pkg/worker/cve_matcher_worker.go:86-108`
- Vấn đề: 200 queries cho 100 CVE matches

**Thực tế code hiện tại:**
```go
// core/pkg/worker/cve_matcher_worker.go:86-108
for _, m := range matches {
    // Query 1: Load persisted match
    var persisted models.CVEMatch
    if err := w.db.WithContext(ctx).
        Where("sbom_id = ? AND package_name = ? AND cve_id = ? AND deleted_at IS NULL",
            m.SBOMID, m.PackageName, m.CVEID).
        First(&persisted).Error; err != nil {
        continue
    }

    // Query 2: Load component
    var component models.SBOMComponent
    if err := w.db.WithContext(ctx).
        Where("sbom_id = ? AND component_name = ? AND deleted_at IS NULL", m.SBOMID, m.PackageName).
        First(&component).Error; err != nil {
        continue
    }
    // ... process insight
}
```

**❌ Đánh giá:**
- **VẪN CÒN N+1** - Mỗi match = 2 queries (persisted + component)
- Cho 100 matches = 200 queries
- **Impact:** High latency, database load cao

**💡 Giải pháp đề xuất:**
```go
// Load ALL matches và components trong 2 queries
var persistedMatches []models.CVEMatch
w.db.WithContext(ctx).
    Where("sbom_id = ? AND deleted_at IS NULL", sbomModel.ID).
    Find(&persistedMatches)

var components []models.SBOMComponent
w.db.WithContext(ctx).
    Where("sbom_id = ? AND deleted_at IS NULL", sbomModel.ID).
    Find(&components)

// Build lookup maps
matchMap := make(map[string]*models.CVEMatch)
for i := range persistedMatches {
    key := fmt.Sprintf("%s:%s", persistedMatches[i].PackageName, persistedMatches[i].CVEID)
    matchMap[key] = &persistedMatches[i]
}

componentMap := make(map[string]*models.SBOMComponent)
for i := range components {
    componentMap[components[i].ComponentName] = &components[i]
}

// Process in memory
for _, m := range matches {
    key := fmt.Sprintf("%s:%s", m.PackageName, m.CVEID)
    persisted := matchMap[key]
    component := componentMap[m.PackageName]
    // ... process
}
```

---

### ❌ Issue #4: Missing Connection Pool Monitoring - **CHƯA FIX**

**Báo cáo gốc:**
- Location: `core/internal/storage/storage.go:34-37`
- Vấn đề: Không có metrics, risk of exhaustion

**Thực tế code hiện tại:**
```go
// core/internal/storage/storage.go:34-37
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(100)
sqlDB.SetConnMaxLifetime(time.Hour)
sqlDB.SetConnMaxIdleTime(10 * time.Minute)
```

**❌ Đánh giá:**
- Không có metrics collection
- Không có monitoring/alerting
- MaxOpenConns(100) có thể không đủ với 5 workers × 5 concurrency = 25 concurrent ops

**💡 Khuyến nghị:**
- Thêm Prometheus metrics cho connection pool stats
- Alert khi `WaitCount > 0` (connection starvation)
- Consider auto-scaling connection pool dựa trên load

---

### ⚠️ Issue #5: NATS Stream Retention - **VẪN CÒN**

**Báo cáo gốc:**
- Location: `core/pkg/messaging/nats_client.go:89-90`
- Vấn đề: 1 hour retention quá ngắn

**Thực tế code hiện tại:**
```go
// core/pkg/messaging/nats_client.go:89-90
if stream.name == "ksam-raw" || stream.name == "ksam-normalized" {
    maxAge = 1 * time.Hour // Layer 4: 1 hour for pod-related streams
}
```

**⚠️ Đánh giá:**
- Vẫn còn 1 hour retention cho `ksam-raw` và `ksam-normalized`
- Risk: Nếu workers backlog > 1 hour → messages bị discard
- **Trade-off:** 1 hour giúp cleanup ghost pods nhanh, nhưng risk mất data

**💡 Khuyến nghị:**
- Tăng lên 24 hours cho safety
- Hoặc dùng `WorkQueuePolicy` với consumer-based cleanup
- Monitor consumer lag để detect backlog

---

### ❌ Issue #6: Missing Batch Operations - **CHƯA FIX**

**Báo cáo gốc:**
- Location: `core/pkg/cve/matcher/matcher.go:54-117`
- Vấn đề: Query CVEs individually per component (N queries)

**Thực tế code hiện tại:**
```go
// core/pkg/cve/matcher/matcher.go:54-117
for _, component := range components {
    // ...
    // 2. Query CVE database
    cves, err := m.dbManager.GetVulnerabilitiesForPackage(
        ctx,
        queryEcosystem,
        purl.Name,
        component.ComponentVersion,
    )
    // ...
}
```

**❌ Đánh giá:**
- Vẫn query CVE per-package (N queries cho N packages)
- Cho 200-package SBOM = 200+ CVE queries
- **Impact:** 15-20s → có thể giảm xuống 2-3s với batch

**💡 Giải pháp đề xuất:**
```go
// Collect all packages first
packagesByEcosystem := make(map[string][]string)
for _, component := range components {
    purl, _ := ParsePURL(component.PURL)
    ecosystem := normalizeQueryEcosystem(purl)
    packagesByEcosystem[ecosystem] = append(packagesByEcosystem[ecosystem], purl.Name)
}

// Batch query per ecosystem
for ecosystem, packages := range packagesByEcosystem {
    cves, err := m.dbManager.GetVulnerabilitiesForPackages(ctx, ecosystem, packages)
    // Process in bulk
}
```

**Cần implement:**
```go
// core/pkg/cve/database/manager.go
func (m *Manager) GetVulnerabilitiesForPackages(
    ctx context.Context,
    ecosystem string,
    packageNames []string,
) ([]VulnerabilityData, error) {
    // Single query with IN clause
    var vulns []PackageVulnerability
    err := m.db.WithContext(ctx).
        Where("ecosystem = ? AND package_name IN ?", ecosystem, packageNames).
        Find(&vulns).Error
    // ...
}
```

---

### ❌ Issue #7: Data Synchronization - **CHƯA FIX**

**Báo cáo gốc:**
- Vấn đề: Không có reconciliation loop khi Core/Agent restart

**Thực tế code hiện tại:**
- Không có reconciliation mechanism
- Relies purely on pod watch events
- Risk: Data drift khi miss events during downtime

**❌ Đánh giá:**
- **CHƯA IMPLEMENT** - Không có reconciliation loop
- **Impact:** Data inconsistency giữa K8s cluster và database

**💡 Khuyến nghị:**
- Implement periodic reconciliation (mỗi 1 hour)
- Compare running pods vs SBOMs in database
- Trigger SBOM extraction cho missing pods
- Mark orphaned SBOMs (pods đã deleted)

---

## 📈 Priority Ranking (Updated)

### 🔴 Priority 1: CRITICAL - Fix ngay

1. **Issue #3: N+1 Query Pattern** ⏱️ 2 hours
   - Impact: 20x performance improvement
   - Effort: Medium
   - **ROI: Very High**

2. **Issue #6: Missing Batch Operations** ⏱️ 3 hours
   - Impact: 6-8x faster CVE matching
   - Effort: Medium
   - **ROI: Very High**

3. **Update Migration 023** ⏱️ 30 min
   - Fix index name từ `component_id` → `package_name`
   - **ROI: High (prevent future bugs)**

### 🟡 Priority 2: MEDIUM - Fix trong sprint

4. **Issue #2: Insight Deduplication** ⏱️ 4 hours
   - Implement PostgreSQL UPSERT batch
   - **ROI: High**

5. **Issue #4: Connection Pool Monitoring** ⏱️ 2 hours
   - Add Prometheus metrics
   - **ROI: Medium (observability)**

6. **Issue #5: NATS Retention** ⏱️ 1 hour
   - Tăng retention lên 24h
   - **ROI: Medium (safety)**

### 🟢 Priority 3: LOW - Backlog

7. **Issue #7: Data Synchronization** ⏱️ 4 hours
   - Implement reconciliation loop
   - **ROI: Medium (data consistency)**

---

## 🎯 Quick Wins Summary

| Task | Time | Impact | Status |
|------|------|--------|--------|
| Fix N+1 in CVE Matcher | 2h | 🔥 Very High | ❌ TODO |
| Implement Batch CVE Lookup | 3h | 🔥 Very High | ❌ TODO |
| Update Migration 023 | 30m | ⚠️ High | ❌ TODO |
| Add Connection Pool Metrics | 2h | 📊 Medium | ❌ TODO |
| Increase NATS Retention | 1h | 🛡️ Medium | ❌ TODO |

**Total Quick Wins:** ~8.5 hours → **Expected 20-30x performance improvement**

---

## ✅ Kết Luận

1. **Issue #1 (Schema Mismatch):** ✅ **ĐÃ FIX** - Code đã đúng, nhưng cần update migration
2. **Issue #2 (Insight Deduplication):** ⚠️ **PARTIALLY FIXED** - Query optimized nhưng vẫn sequential
3. **Issues #3, #4, #5, #6, #7:** ❌ **CHƯA FIX** - Vẫn còn các vấn đề performance và reliability

**Khuyến nghị:**
- **Immediate:** Fix N+1 queries (Issue #3) và batch CVE lookup (Issue #6) → 20-30x improvement
- **Short-term:** Add monitoring và increase retention
- **Long-term:** Implement reconciliation loop

