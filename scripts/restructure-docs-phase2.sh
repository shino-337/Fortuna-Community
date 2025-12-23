#!/bin/bash
# Documentation Restructure - Phase 2: Move and Organize All Documents
# Fortuna K8s Management Platform

set -e

echo "================================================"
echo "  Fortuna Documentation Restructure - Phase 2"
echo "================================================"
echo ""

BASE_DIR="docs"
cd "$(dirname "$0")/.."

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Step 1: Move getting-started documents
echo -e "${GREEN}Step 1: Moving getting-started documents...${NC}"

if [ -d "$BASE_DIR/getting-started" ]; then
    # Move all files from getting-started to 01-getting-started
    for file in "$BASE_DIR/getting-started"/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            mv "$file" "$BASE_DIR/01-getting-started/$filename"
            echo "  ✓ Moved $filename"
        fi
    done
    rmdir "$BASE_DIR/getting-started" 2>/dev/null || true
    echo -e "${GREEN}✓ Getting started documents moved (12 files)${NC}"
else
    echo -e "${YELLOW}⚠ getting-started/ not found${NC}"
fi

# Step 2: Move architecture documents
echo ""
echo -e "${GREEN}Step 2: Moving architecture documents...${NC}"

if [ -d "$BASE_DIR/architecture" ]; then
    # Move architecture files
    for file in "$BASE_DIR/architecture"/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            mv "$file" "$BASE_DIR/02-architecture/$filename"
            echo "  ✓ Moved $filename"
        elif [ -d "$file" ]; then
            dirname=$(basename "$file")
            mv "$file" "$BASE_DIR/02-architecture/$dirname"
            echo "  ✓ Moved $dirname/"
        fi
    done
    rmdir "$BASE_DIR/architecture" 2>/dev/null || true
fi

# Move root architecture documents
if [ -f "$BASE_DIR/ARCHITECTURE.md" ]; then
    mv "$BASE_DIR/ARCHITECTURE.md" "$BASE_DIR/02-architecture/ARCHITECTURE_OLD.md"
    echo "  ✓ Moved ARCHITECTURE.md (renamed to ARCHITECTURE_OLD.md)"
fi

if [ -f "$BASE_DIR/ARCHITECTURE_ANALYSIS.md" ]; then
    mv "$BASE_DIR/ARCHITECTURE_ANALYSIS.md" "$BASE_DIR/02-architecture/CORE_ONLY_ANALYSIS.md"
    echo "  ✓ Moved ARCHITECTURE_ANALYSIS.md (renamed to CORE_ONLY_ANALYSIS.md)"
fi

echo -e "${GREEN}✓ Architecture documents moved${NC}"

# Step 3: Move components documents
echo ""
echo -e "${GREEN}Step 3: Moving components documents...${NC}"

if [ -d "$BASE_DIR/components" ]; then
    # Move component subdirectories
    for dir in "$BASE_DIR/components"/*; do
        if [ -d "$dir" ]; then
            dirname=$(basename "$dir")
            # Skip if already exists in target
            if [ ! -d "$BASE_DIR/03-components/$dirname" ]; then
                mv "$dir" "$BASE_DIR/03-components/$dirname"
                echo "  ✓ Moved components/$dirname/"
            else
                # Merge contents
                for file in "$dir"/*; do
                    if [ -f "$file" ]; then
                        filename=$(basename "$file")
                        mv "$file" "$BASE_DIR/03-components/$dirname/$filename"
                        echo "  ✓ Merged $dirname/$filename"
                    fi
                done
                rmdir "$dir" 2>/dev/null || true
            fi
        fi
    done
    
    # Move INDEX.md if exists
    if [ -f "$BASE_DIR/components/INDEX.md" ]; then
        mv "$BASE_DIR/components/INDEX.md" "$BASE_DIR/03-components/INDEX.md"
        echo "  ✓ Moved components/INDEX.md"
    fi
    
    rmdir "$BASE_DIR/components" 2>/dev/null || true
    echo -e "${GREEN}✓ Components documents moved${NC}"
fi

# Step 4: Move development documents
echo ""
echo -e "${GREEN}Step 4: Moving development documents...${NC}"

if [ -d "$BASE_DIR/development" ]; then
    for item in "$BASE_DIR/development"/*; do
        if [ -f "$item" ]; then
            filename=$(basename "$item")
            mv "$item" "$BASE_DIR/04-development/$filename"
            echo "  ✓ Moved $filename"
        elif [ -d "$item" ]; then
            dirname=$(basename "$item")
            mv "$item" "$BASE_DIR/04-development/$dirname"
            echo "  ✓ Moved $dirname/"
        fi
    done
    rmdir "$BASE_DIR/development" 2>/dev/null || true
    echo -e "${GREEN}✓ Development documents moved (16 files + 2 subdirs)${NC}"
fi

# Step 5: Move operations documents
echo ""
echo -e "${GREEN}Step 5: Moving operations documents...${NC}"

if [ -d "$BASE_DIR/operations" ]; then
    # operations/performance/ already exists, check content
    for item in "$BASE_DIR/operations"/*; do
        if [ -d "$item" ]; then
            dirname=$(basename "$item")
            # Merge if exists, otherwise move
            if [ -d "$BASE_DIR/05-operations/$dirname" ]; then
                for file in "$item"/*; do
                    if [ -f "$file" ]; then
                        filename=$(basename "$file")
                        mv "$file" "$BASE_DIR/05-operations/$dirname/$filename"
                        echo "  ✓ Merged $dirname/$filename"
                    fi
                done
                rmdir "$item" 2>/dev/null || true
            else
                mv "$item" "$BASE_DIR/05-operations/$dirname"
                echo "  ✓ Moved $dirname/"
            fi
        fi
    done
    rmdir "$BASE_DIR/operations" 2>/dev/null || true
    echo -e "${GREEN}✓ Operations documents moved${NC}"
fi

# Step 6: Move migration documents to 06-reference
echo ""
echo -e "${GREEN}Step 6: Moving migration documents to reference...${NC}"

if [ -d "$BASE_DIR/migration" ]; then
    mkdir -p "$BASE_DIR/06-reference/migration"
    for file in "$BASE_DIR/migration"/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            mv "$file" "$BASE_DIR/06-reference/migration/$filename"
            echo "  ✓ Moved migration/$filename"
        fi
    done
    rmdir "$BASE_DIR/migration" 2>/dev/null || true
    echo -e "${GREEN}✓ Migration documents moved (6 files)${NC}"
fi

# Step 7: Move reference documents
echo ""
echo -e "${GREEN}Step 7: Moving reference documents...${NC}"

if [ -f "$BASE_DIR/SECURITY.md" ]; then
    mv "$BASE_DIR/SECURITY.md" "$BASE_DIR/06-reference/SECURITY.md"
    echo "  ✓ Moved SECURITY.md"
fi

echo -e "${GREEN}✓ Reference documents organized${NC}"

# Step 8: Archive old structure folder
echo ""
echo -e "${GREEN}Step 8: Archiving old structure...${NC}"

if [ -d "$BASE_DIR/archive" ]; then
    # Move entire archive folder to 09-archive/old-archive
    if [ ! -d "$BASE_DIR/09-archive/old-archive" ]; then
        mkdir -p "$BASE_DIR/09-archive/old-archive"
    fi
    
    for item in "$BASE_DIR/archive"/*; do
        if [ -f "$item" ] || [ -d "$item" ]; then
            itemname=$(basename "$item")
            mv "$item" "$BASE_DIR/09-archive/old-archive/$itemname"
            echo "  ✓ Archived $itemname"
        fi
    done
    
    rmdir "$BASE_DIR/archive" 2>/dev/null || true
    echo -e "${GREEN}✓ Old archive moved (9 files)${NC}"
fi

# Step 9: Handle 06-development duplicate
echo ""
echo -e "${GREEN}Step 9: Cleaning up duplicates...${NC}"

if [ -d "$BASE_DIR/06-development" ]; then
    # Move any files to 04-development
    for file in "$BASE_DIR/06-development"/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            # Check if already exists in 04-development
            if [ ! -f "$BASE_DIR/04-development/$filename" ]; then
                mv "$file" "$BASE_DIR/04-development/$filename"
                echo "  ✓ Moved duplicate $filename to 04-development"
            else
                echo "  ⚠ Skipped duplicate $filename (already exists)"
            fi
        fi
    done
    rmdir "$BASE_DIR/06-development" 2>/dev/null || true
    echo -e "${GREEN}✓ Removed duplicate 06-development folder${NC}"
fi

# Step 10: Create section README files
echo ""
echo -e "${GREEN}Step 10: Creating section README files...${NC}"

# 01-getting-started README
cat > "$BASE_DIR/01-getting-started/README.md" << 'EOF'
# Getting Started with Fortuna

Quick start guides and tutorials for deploying and using Fortuna.

## 📚 Documents

### Quick Start
- **[QUICKSTART.md](./QUICKSTART.md)** - 10-minute deployment guide
- **[MINIKUBE_SETUP.md](./MINIKUBE_SETUP.md)** - Local development setup

### Deployment Guides
- **[SBOM_DEPLOYMENT_GUIDE.md](./SBOM_DEPLOYMENT_GUIDE.md)** - SBOM feature setup
- **[WEBHOOK_DEPLOYMENT_INSTRUCTIONS.md](./WEBHOOK_DEPLOYMENT_INSTRUCTIONS.md)** - Admission webhook
- **[DASHBOARD_ACCESS_GUIDE.md](./DASHBOARD_ACCESS_GUIDE.md)** - Web UI access

### Implementation Guides
- **[CVE_MASTER_IMPLEMENTATION_GUIDE.md](./CVE_MASTER_IMPLEMENTATION_GUIDE.md)** - CVE scanning setup
- **[ADMISSION_METRICS_SETUP_GUIDE.md](./ADMISSION_METRICS_SETUP_GUIDE.md)** - Metrics configuration
- **[INSIGHTS_MANAGEMENT_GUIDE.md](./INSIGHTS_MANAGEMENT_GUIDE.md)** - Managing security insights

### Setup & Configuration
- **[DATABASE_SETUP_RESULTS.md](./DATABASE_SETUP_RESULTS.md)** - Database setup verification
- **[MIGRATION_ERROR_WORKAROUND.md](./MIGRATION_ERROR_WORKAROUND.md)** - Common fixes

### Security
- **[SECURITY_GUIDE.md](./SECURITY_GUIDE.md)** - Security best practices

---

**Next**: [Architecture Overview](../02-architecture/)
EOF
echo "  ✓ Created 01-getting-started/README.md"

# 02-architecture README (update existing if needed)
cat > "$BASE_DIR/02-architecture/README.md" << 'EOF'
# Architecture Documentation

Comprehensive documentation of Fortuna's architecture, design decisions, and system components.

## 📚 Core Documents

### Primary Architecture
- **[README.md](./README.md)** - Complete architecture overview (17KB)
- **[CORE_ONLY_ANALYSIS.md](./CORE_ONLY_ANALYSIS.md)** - Core-Only architecture analysis
- **[KSAM_ADR_FULL.md](./KSAM_ADR_FULL.md)** - Architecture Decision Records

### Historical
- **[ARCHITECTURE_OLD.md](./ARCHITECTURE_OLD.md)** - Previous architecture (for reference)

### Index
- **[INDEX.md](./INDEX.md)** - Architecture document index

### Changelog
- **[changelog/](./changelog/)** - Architecture evolution history

---

## 🎯 Current Architecture: Core-Only

**Important**: Fortuna currently uses a **Core-Only Architecture**.

```
┌─────────────────────────────────────────────────────────┐
│                Kubernetes Cluster                        │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  Fortuna Core (Deployment)                              │
│       ├─→ K8s API Client (collect resources)           │
│       ├─→ SBOM Worker (extract packages)                │
│       ├─→ CVE Matcher (match vulnerabilities)          │
│       ├─→ Risk Engine (calculate scores)                │
│       ├─→ Insight Manager (create findings)             │
│       └─→ REST API (serve dashboard)                    │
│                                                          │
│  PostgreSQL + NATS JetStream                            │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

**Key Points**:
- ✅ **Core handles all processing**: Collection, SBOM, CVE, Insights
- ❌ **Agent is disabled**: Not needed for single-cluster deployments
- ✅ **Simpler deployment**: Single pod vs N pods (agent per node)
- ✅ **Feature complete**: All capabilities available

---

**Next**: [Components](../03-components/)
EOF
echo "  ✓ Updated 02-architecture/README.md"

# 04-development README
cat > "$BASE_DIR/04-development/README.md" << 'EOF'
# Development Documentation

Developer guides, API documentation, and testing resources.

## 📚 Documents

### CVE & SBOM Development
- **[CVE_LOADING_GUIDE.md](./CVE_LOADING_GUIDE.md)** - Load CVE data into PostgreSQL
- **[CVE_LOADER_USAGE.md](./CVE_LOADER_USAGE.md)** - Using the CVE loader tool
- **[CVE_LOADER_TESTING_GUIDE.md](./CVE_LOADER_TESTING_GUIDE.md)** - Testing CVE loading
- **[CVE_UPDATE_MECHANISM.md](./CVE_UPDATE_MECHANISM.md)** - CVE update automation
- **[CVE_DATABASE_STATUS.md](./CVE_DATABASE_STATUS.md)** - Current database state
- **[CVE_DATA_SOURCE_STRATEGY.md](./CVE_DATA_SOURCE_STRATEGY.md)** - Data sources & strategy
- **[OSV_PARSER_ANALYSIS.md](./OSV_PARSER_ANALYSIS.md)** - OSV JSON parsing

### API Documentation
- **[API_VERIFICATION_RESULTS.md](./API_VERIFICATION_RESULTS.md)** - API endpoint testing
- **[INSIGHTS_API_CURL_EXAMPLES.md](./INSIGHTS_API_CURL_EXAMPLES.md)** - API usage examples

### Testing
- **[E2E_CVE_INSIGHTS_TEST.md](./E2E_CVE_INSIGHTS_TEST.md)** - End-to-end CVE testing
- **[E2E_OPTIMIZATION_VERIFICATION_REPORT.md](./E2E_OPTIMIZATION_VERIFICATION_REPORT.md)** - Performance verification
- **[E2E_TEST_ANALYSIS_20251217.md](./E2E_TEST_ANALYSIS_20251217.md)** - Test analysis
- **[testing/](./testing/)** - Test suites and reports

### Architecture Review
- **[AGENT_ARCHITECTURE_REVIEW.md](./AGENT_ARCHITECTURE_REVIEW.md)** - Agent design review (historical)

### Optimization
- **[INSIGHT_MANAGER_OPTIMIZATION.md](./INSIGHT_MANAGER_OPTIMIZATION.md)** - Insight manager optimization

### Database Migrations
- **[migrations/](./migrations/)** - Database migration guides

### Version Information
- **[GO_VERSION_VERIFICATION.md](./GO_VERSION_VERIFICATION.md)** - Go version requirements

---

**Next**: [Operations](../05-operations/)
EOF
echo "  ✓ Created 04-development/README.md"

# 05-operations README
cat > "$BASE_DIR/05-operations/README.md" << 'EOF'
# Operations Documentation

Deployment, monitoring, and operational guides for Fortuna.

## 📚 Documents

### Performance
- **[performance/BENCHMARKS.md](./performance/BENCHMARKS.md)** - Performance benchmarks

### Deployment
- **[deployment/](./deployment/)** - Deployment guides (if exists)

### Monitoring
- **[monitoring/](./monitoring/)** - Monitoring & alerting setup

---

## 🚀 Quick Operations Guide

### Check System Health
```bash
kubectl get pods -n fortuna
kubectl logs -n fortuna -l app=fortuna-core --tail=50
```

### Monitor CVE Database
```bash
# Connect to PostgreSQL
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it $POSTGRES_POD -n fortuna -- psql -U postgres -d fortuna

# Check database status
SELECT COUNT(*) FROM cves;
SELECT COUNT(*) FROM insights WHERE status = 'active';
SELECT COUNT(*) FROM sboms;
```

### Performance Monitoring
- SBOM generation: ~2-5 seconds per image
- CVE matching: <1 second per SBOM
- API response: <50ms (p99)

---

**Next**: [Reference](../06-reference/)
EOF
echo "  ✓ Created 05-operations/README.md"

# 06-reference README
cat > "$BASE_DIR/06-reference/README.md" << 'EOF'
# Reference Documentation

API references, security policies, and migration guides.

## 📚 Documents

### Security
- **[SECURITY.md](./SECURITY.md)** - Security policies and best practices

### Migration
- **[migration/](./migration/)** - KSAM to Fortuna migration guides
  - **[MIGRATION_GUIDE_KSAM_TO_FORTUNA.md](./migration/MIGRATION_GUIDE_KSAM_TO_FORTUNA.md)** - Migration overview
  - **[MIGRATION_EXECUTION_REPORT.md](./migration/MIGRATION_EXECUTION_REPORT.md)** - Execution report
  - **[MIGRATION_COMPLETE.md](./migration/MIGRATION_COMPLETE.md)** - Completion status
  - **[DATABASE_DEEP_ANALYSIS_REPORT.md](./migration/DATABASE_DEEP_ANALYSIS_REPORT.md)** - Database analysis
  - **[FILE_MIGRATION_MAP.md](./migration/FILE_MIGRATION_MAP.md)** - File mapping
  - **[FULL_PROJECT_REVIEW_SUMMARY.md](./migration/FULL_PROJECT_REVIEW_SUMMARY.md)** - Review summary

---

**Next**: [Guides](../07-guides/)
EOF
echo "  ✓ Created 06-reference/README.md"

# 07-guides README
cat > "$BASE_DIR/07-guides/README.md" << 'EOF'
# Guides

Detailed how-to guides for specific tasks and features.

## 📚 Coming Soon

- Policy writing guide
- Custom SBOM extractor development
- Attack path analysis guide
- Multi-cluster setup guide

---

**Next**: [Tutorials](../08-tutorials/)
EOF
echo "  ✓ Created 07-guides/README.md"

# 08-tutorials README
cat > "$BASE_DIR/08-tutorials/README.md" << 'EOF'
# Tutorials

Step-by-step tutorials for learning Fortuna.

## 📚 Coming Soon

- Tutorial 1: Deploy and scan your first app
- Tutorial 2: Write a custom policy
- Tutorial 3: Analyze attack paths
- Tutorial 4: Set up monitoring and alerts

---

**Next**: [Archive](../09-archive/)
EOF
echo "  ✓ Created 08-tutorials/README.md"

echo -e "${GREEN}✓ All section READMEs created${NC}"

# Step 11: Generate final summary
echo ""
echo "================================================"
echo -e "${GREEN}  Phase 2 Complete!${NC}"
echo "================================================"
echo ""
echo -e "${CYAN}Summary:${NC}"
echo "  ✓ Moved getting-started documents (12 files) → 01-getting-started/"
echo "  ✓ Moved architecture documents (4+ files) → 02-architecture/"
echo "  ✓ Moved components documents → 03-components/"
echo "  ✓ Moved development documents (16+ files) → 04-development/"
echo "  ✓ Moved operations documents → 05-operations/"
echo "  ✓ Moved migration documents (6 files) → 06-reference/migration/"
echo "  ✓ Moved reference documents (SECURITY.md) → 06-reference/"
echo "  ✓ Archived old structure (9 files) → 09-archive/old-archive/"
echo "  ✓ Created section READMEs (8 files)"
echo "  ✓ Removed duplicate folders"
echo ""
echo -e "${CYAN}New Structure:${NC}"
echo "  docs/"
echo "    ├── README.md                 (main index - kept)"
echo "    ├── START_HERE.md             (entry point - kept)"
echo "    ├── 01-getting-started/       (12 documents + README)"
echo "    ├── 02-architecture/          (5+ documents + README)"
echo "    ├── 03-components/            (7 subdirectories)"
echo "    ├── 04-development/           (16+ documents + README)"
echo "    ├── 05-operations/            (subdirectories + README)"
echo "    ├── 06-reference/             (SECURITY + migration + README)"
echo "    ├── 07-guides/                (README - empty)"
echo "    ├── 08-tutorials/             (README - empty)"
echo "    └── 09-archive/               (50+ archived documents)"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "  1. Review changes: git status"
echo "  2. Test documentation: cat docs/README.md"
echo "  3. Commit: git add docs/ && git commit -m 'docs: Phase 2 - Restructure complete'"
echo ""

