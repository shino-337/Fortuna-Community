# Fortuna Scripts - Organized Structure

**Last Updated**: December 22, 2024  
**Total Scripts**: 147 scripts organized into 7 categories

---

## 📚 Quick Navigation

| Category | Scripts | Purpose |
|----------|---------|---------|
| **[deployment/](./deployment/)** | 7 | Deploy, rebuild, and manage Fortuna components |
| **[testing/](./testing/)** | 85 | E2E, integration, performance, and verification tests |
| **[database/](./database/)** | 11 | Database setup, migrations, and maintenance |
| **[monitoring/](./monitoring/)** | 6 | Monitor Fortuna components and pipelines |
| **[setup/](./setup/)** | 8 | Initial setup, minikube, and environment configuration |
| **[development/](./development/)** | 10 | Development tools, debugging, and certificate generation |
| **[utils/](./utils/)** | 20 | Utilities, cleanup, and helper scripts |

---

## 🚀 Quick Start

### First Time Setup
```bash
# 1. Start minikube
./setup/start_minikube.sh

# 2. Deploy Fortuna
./deployment/deploy.sh

# 3. Setup database
./database/setup_database.sh

# 4. Verify deployment
./setup/verify_deployment.sh
```

### Daily Development
```bash
# Start port forwards
./setup/start_portforwards.sh

# Monitor system
./monitoring/monitor_fortuna.sh

# Run tests
./testing/run_all_tests.sh
```

### Rebuild & Deploy
```bash
# Clean rebuild and deploy
./deployment/clean-rebuild-deploy.sh

# Or just rebuild
./deployment/rebuild_and_deploy.sh
```

---

## 📁 Directory Structure

```
scripts/
├── README.md                    ← This file
│
├── deployment/                  ← 7 scripts
│   ├── README.md
│   ├── deploy.sh
│   ├── deploy_full.sh
│   ├── rebuild_and_deploy.sh
│   └── ...
│
├── testing/                     ← 85 scripts
│   ├── README.md
│   ├── e2e/                     ← End-to-end tests (19 scripts)
│   ├── integration/             ← Integration tests (8 scripts)
│   ├── performance/             ← Performance tests (4 scripts)
│   ├── verification/            ← Verification scripts (7 scripts)
│   └── ... (component tests, dashboard, mTLS, etc.)
│
├── database/                    ← 11 scripts
│   ├── README.md
│   ├── setup_database.sh
│   ├── clear_database.sh
│   └── ... (migrations, sync)
│
├── monitoring/                  ← 6 scripts
│   ├── README.md
│   ├── monitor_fortuna.sh
│   ├── monitor-pipeline.sh
│   └── ...
│
├── setup/                       ← 8 scripts
│   ├── README.md
│   ├── start_minikube.sh
│   ├── quick_start_minikube.sh
│   └── ...
│
├── development/                 ← 10 scripts
│   ├── README.md
│   ├── generate_certs.sh
│   ├── debug_routes.sh
│   └── ...
│
├── utils/                       ← 20 scripts
│   ├── README.md
│   ├── publish_*.sh (NATS utilities)
│   ├── cleanup_*.sh
│   └── ...
│
├── archive/                     ← Deprecated/old scripts
│   ├── deprecated/
│   └── test_results/
│
└── Documentation Scripts (root)
    ├── commit-docs-restructure.sh
    ├── show-docs-tree.sh
    ├── Restructure-Docs-Phase1.ps1
    └── restructure-docs-phase2.sh
```

---

## 🎯 Common Tasks

### Deployment
```bash
# Initial deployment
cd scripts/deployment
./deploy.sh

# Full deployment with all components
./deploy_full.sh

# Rebuild after code changes
./rebuild_and_deploy.sh
```

### Testing
```bash
# Run all tests
cd scripts/testing
./run_all_tests.sh

# E2E tests
cd e2e
./test_e2e_comprehensive.sh

# Verify specific functionality
cd ../verification
./verify_insights_complete.sh
```

### Database Management
```bash
cd scripts/database

# Setup database
./setup_database.sh

# Clear database (destructive!)
./clear_database.sh

# Run migrations
./run-migration-020.sh
```

### Monitoring
```bash
cd scripts/monitoring

# Monitor entire system
./monitor_fortuna.sh

# Monitor SBOM pipeline
./monitor-pipeline.sh

# List insights
./list_insights.sh
```

---

## 🔧 Development Workflow

### 1. Make Code Changes
```bash
# Edit code in core/, agent/, or dashboard/
```

### 2. Rebuild & Deploy
```bash
cd scripts/deployment
./rebuild_and_deploy.sh
```

### 3. Run Tests
```bash
cd scripts/testing
./run_validation_tests.sh
```

### 4. Monitor & Verify
```bash
cd scripts/monitoring
./monitor_fortuna.sh
```

---

## 📊 Testing Categories

### E2E Tests (`testing/e2e/`)
Full end-to-end tests covering entire pipelines:
- Pod → SBOM → CVE → Insights flow
- Complete system tests
- MVP1/MVP2 comprehensive tests

**Example:**
```bash
cd scripts/testing/e2e
./test_e2e_comprehensive.sh
```

### Integration Tests (`testing/integration/`)
Tests for component integration:
- API integration
- Data flow between components
- Migration tests

**Example:**
```bash
cd scripts/testing/integration
./test_all_apis.sh
```

### Performance Tests (`testing/performance/`)
Performance and load testing:
- Worker backpressure
- SBOM generation performance
- System benchmarks

**Example:**
```bash
cd scripts/testing/performance
./test_performance.sh
```

### Verification (`testing/verification/`)
Verification and validation scripts:
- Verify insights generation
- Dashboard data verification
- Pipeline verification

**Example:**
```bash
cd scripts/testing/verification
./verify_insights_complete.sh
```

---

## 🛠️ Utility Scripts

### NATS Publishing (`utils/publish_*.sh`)
Publish test messages to NATS for testing:
```bash
cd scripts/utils
./publish_test_messages_natsbox.sh
./publish_vulnerable_pod_data.sh
```

### Database Queries (`utils/query_*.sh`, `utils/view_*.sh`)
Query database and view data:
```bash
cd scripts/utils
./query_policy_db.sh
./view_policy_templates.sh
```

### Cleanup (`utils/cleanup_*.sh`)
Cleanup scripts:
```bash
cd scripts/utils
./cleanup_duplicate_pods.sh
./clear_images.sh
```

---

## 📖 Documentation Scripts

Scripts for documentation management (in scripts root):

```bash
# Show documentation structure
./show-docs-tree.sh

# Commit documentation changes
./commit-docs-restructure.sh

# Reorganize scripts (this script!)
./reorganize-scripts.sh
```

---

## 🗺️ Script Naming Conventions

### Prefixes
- `deploy_*.sh` - Deployment scripts
- `test_*.sh` - Test scripts
- `monitor_*.sh` - Monitoring scripts
- `verify_*.sh` - Verification scripts
- `setup_*.sh` - Setup scripts
- `run_*.sh` - Script runners
- `publish_*.sh` - NATS publishing utilities
- `query_*.sh`, `view_*.sh` - Database query utilities
- `cleanup_*.sh`, `clear_*.sh` - Cleanup utilities

### Suffixes
- `*_e2e.sh` - End-to-end tests
- `*_comprehensive.sh` - Comprehensive test suites
- `*_complete.sh` - Complete workflows
- `*_k8s.sh` - Kubernetes-specific scripts

---

## 🔍 Finding Scripts

### By Purpose
```bash
# Deployment
ls -la deployment/

# Testing
ls -la testing/
ls -la testing/e2e/

# Database
ls -la database/

# Monitoring
ls -la monitoring/
```

### By Name
```bash
# Find script by name
find scripts/ -name "*webhook*"
find scripts/ -name "*sbom*"
find scripts/ -name "*cve*"
```

### By Content
```bash
# Search script content
grep -r "kubectl apply" scripts/deployment/
grep -r "psql" scripts/database/
```

---

## 📝 Script Documentation

Each category has its own README with:
- **Purpose**: What the scripts in this category do
- **Usage**: How to use common scripts
- **Examples**: Real-world usage examples
- **Troubleshooting**: Common issues and solutions

See individual README files:
- [deployment/README.md](./deployment/README.md)
- [testing/README.md](./testing/README.md)
- [database/README.md](./database/README.md)
- [monitoring/README.md](./monitoring/README.md)
- [setup/README.md](./setup/README.md)
- [development/README.md](./development/README.md)
- [utils/README.md](./utils/README.md)

---

## 🚨 Important Notes

### Destructive Scripts
These scripts modify or delete data. Use with caution!
- `database/clear_database.sh` - Drops all database tables
- `database/clear_database_k8s.sh` - Clears K8s-related data
- `utils/cleanup_*.sh` - Cleanup scripts
- `utils/clear_images.sh` - Deletes Docker images

**Always backup before running destructive scripts!**

### Production Usage
Most scripts are designed for **development and testing**. For production:
- Use Helm charts (see `helm/ksam/`)
- Follow production deployment guide (see `docs/05-operations/`)
- Review security considerations

---

## 🔄 Migration from Old Structure

If you have scripts pointing to old locations:

### Old → New Mapping
```
scripts/deploy.sh              → scripts/deployment/deploy.sh
scripts/test_e2e_full.sh       → scripts/testing/e2e/test_e2e_full.sh
scripts/monitor_fortuna.sh     → scripts/monitoring/monitor_fortuna.sh
scripts/setup_database.sh      → scripts/database/setup_database.sh
scripts/start_minikube.sh      → scripts/setup/start_minikube.sh
scripts/generate_certs.sh      → scripts/development/generate_certs.sh
scripts/publish_messages.sh    → scripts/utils/publish_messages_direct.sh
```

### Update Your Scripts
```bash
# Old
./scripts/deploy.sh

# New
./scripts/deployment/deploy.sh

# Or use absolute paths
cd scripts
./deployment/deploy.sh
```

---

## 📊 Statistics

### By Category
- **Testing**: 85 scripts (58%)
- **Utils**: 20 scripts (14%)
- **Database**: 11 scripts (7%)
- **Development**: 10 scripts (7%)
- **Setup**: 8 scripts (5%)
- **Deployment**: 7 scripts (5%)
- **Monitoring**: 6 scripts (4%)

### By Type
- **Bash scripts**: ~140 files (.sh)
- **Python scripts**: 1 file (.py)
- **Go scripts**: 1 file (.go)
- **SQL scripts**: 1 file (.sql)
- **PowerShell**: 1 file (.ps1)

### Total
- **147 scripts** organized
- **7 categories** created
- **8 README files** (this + 7 category READMEs)

---

## 🤝 Contributing

### Adding New Scripts

1. **Choose correct category**:
   - Deployment? → `deployment/`
   - Testing? → `testing/` (or subdirectory)
   - Database? → `database/`
   - etc.

2. **Follow naming conventions**:
   - Use descriptive names
   - Follow prefix/suffix patterns
   - Use `.sh` extension for bash scripts

3. **Add documentation**:
   - Add header comment explaining purpose
   - Update category README if needed
   - Add usage examples

4. **Make executable**:
   ```bash
   chmod +x scripts/category/your_script.sh
   ```

### Script Template
```bash
#!/bin/bash
# Script: your_script.sh
# Purpose: Brief description of what this script does
# Usage: ./your_script.sh [options]
# Example: ./your_script.sh --namespace fortuna

set -e  # Exit on error

# Your code here
```

---

## 📚 Related Documentation

- **[Fortuna Documentation](../docs/README.md)** - Complete documentation
- **[Development Guide](../docs/04-development/README.md)** - Development setup
- **[Testing Guide](../docs/04-development/testing/README.md)** - Testing strategies
- **[Operations Guide](../docs/05-operations/README.md)** - Production operations

---

## 🆘 Troubleshooting

### Script Not Found
```bash
# Make sure you're in the right directory
cd scripts/deployment
./deploy.sh

# Or use absolute path from project root
./scripts/deployment/deploy.sh
```

### Permission Denied
```bash
# Make script executable
chmod +x scripts/deployment/deploy.sh
```

### Script Fails
```bash
# Check prerequisites
./setup/verify-dependencies.sh

# Check deployment
./setup/verify_deployment.sh

# View logs
./monitoring/monitor_fortuna.sh
```

---

## 🎉 Quick Win Commands

### "Just make it work!"
```bash
cd scripts
./setup/quick_start_minikube.sh
./deployment/deploy_full.sh
./testing/e2e/test_e2e_comprehensive.sh
```

### "Show me everything is working"
```bash
cd scripts
./monitoring/monitor_fortuna.sh
./testing/verification/verify_insights_complete.sh
./monitoring/list_insights.sh
```

### "Start fresh"
```bash
cd scripts
./database/clear_database.sh
./deployment/clean-rebuild-deploy.sh
./testing/e2e/full_e2e_test.sh
```

---

**Ready to use Fortuna scripts!** 🚀

For detailed usage of each category, see individual README files in subdirectories.

---

*Fortuna K8s Management Platform - Scripts v2.0*
