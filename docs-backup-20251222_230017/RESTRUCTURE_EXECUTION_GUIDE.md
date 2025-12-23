# Documentation Restructure - Execution Guide

**Date**: December 22, 2024  
**Status**: ✅ Ready to Execute  
**Estimated Time**: 4 hours (Phase 1-3), 2 weeks (Phase 4-6)

---

## 📚 Quick Links

- **Full Plan**: [`DOCUMENTATION_RESTRUCTURE_PLAN.md`](./DOCUMENTATION_RESTRUCTURE_PLAN.md) - Complete strategy
- **Summary**: [`DOCUMENTATION_RESTRUCTURE_SUMMARY.md`](./DOCUMENTATION_RESTRUCTURE_SUMMARY.md) - Quick reference
- **This Guide**: Step-by-step execution instructions

---

## 🎯 What This Guide Covers

This guide provides **step-by-step instructions** to execute the documentation restructure.

**You will**:
1. Understand what will happen
2. Create a backup
3. Execute Phase 1 (automated)
4. Review results
5. Get next steps

---

## ⚡ Quick Start (TL;DR)

```bash
# 1. Backup
cp -r docs docs-backup-$(date +%Y%m%d_%H%M%S)

# 2. Execute Phase 1
bash scripts/restructure-docs-phase1.sh

# 3. Review
git status
git diff docs/09-archive/

# 4. Commit
git add docs/
git commit -m "docs: Phase 1 - Archive outdated documents and create new structure"
```

---

## 📋 Phase 1: Cleanup & Archive (Automated)

**Duration**: 5 minutes  
**Script**: `scripts/restructure-docs-phase1.sh`  
**Files Affected**: ~50 files moved to archive

### What Phase 1 Does

✅ Creates new folder structure (9 sections)  
✅ Archives 50+ outdated files to `09-archive/`  
✅ Removes 2 duplicate folders  
✅ Creates archive index with README  
✅ Preserves all content (nothing deleted)  

### Files to be Archived

**Agent Documents** (3 files):
- `AGENT_DEPLOYMENT_COMPLETE.md`
- `AGENT_STATUS_FINAL.md`
- `REFACTORING_PLAN_AGENT_BASED.md`

**Implementation Plans** (22 files):
- `development/setup/*` (all implementation plans)

**SBOM History** (8 files):
- `components/sbom/CUSTOM_SBOM_ANALYSIS_*.md`
- `components/sbom/SBOM_BASED_SCANNING_*.md`
- `components/sbom/SBOM_FIX_*.md`

**CVE Reports** (10 files):
- `development/CVE_BULK_LOADING_STRATEGY.md`
- `development/CVE_DATA_OPTIMIZATION.md`
- `development/CVE_OPTIMIZATION_*.md`
- `development/BULK_LOADER_*.md`
- `development/cve-optimization/`

**Organization Docs** (4 files):
- `CLEANUP_COMPLETE.md`
- `DOCUMENT_ORGANIZATION_PLAN.md`
- `FINAL_STRUCTURE_COMPLETE.md`
- `ARCHITECTURE_DIAGRAMS_COMPARISON.md`

**Duplicates** (2 folders):
- `02-architecture/` (duplicate of `architecture/`)
- `06-development/` (duplicate of `development/`)

### Step-by-Step Execution

#### Step 1: Backup (CRITICAL!)

```bash
# Create timestamped backup
cp -r docs docs-backup-$(date +%Y%m%d_%H%M%S)

# Verify backup exists
ls -la | grep docs-backup

# Expected output:
# drwxr-xr-x  docs-backup-20241222_143000
```

**Why?** Safety net if you need to rollback.

#### Step 2: Review Script (Optional)

```bash
# Read the script to understand what it does
cat scripts/restructure-docs-phase1.sh

# Key actions:
# - mkdir -p docs/01-getting-started/ ... (create new folders)
# - mv docs/AGENT_*.md docs/09-archive/agent/ (move files)
# - rm -rf docs/02-architecture/ (remove duplicates)
```

#### Step 3: Execute Phase 1

```bash
# Make script executable
chmod +x scripts/restructure-docs-phase1.sh

# Run the script
bash scripts/restructure-docs-phase1.sh
```

**Expected Output**:
```
================================================
  Fortuna Documentation Restructure - Phase 1
================================================

Step 1: Creating new folder structure...
✓ New structure created

Step 2: Archiving agent-related documents...
✓ Archived AGENT_DEPLOYMENT_COMPLETE.md
✓ Archived AGENT_STATUS_FINAL.md
✓ Archived REFACTORING_PLAN_AGENT_BASED.md
Archiving implementation plans...
✓ Archived development/setup/* (22 files)
Archiving SBOM development history...
✓ Archived SBOM development history (8 files)
Archiving CVE optimization reports...
✓ Archived CVE optimization reports (~10 files)
Archiving organization documents...
✓ Archived organization documents (4 files)

Step 3: Creating archive index...
✓ Created archive/README.md

Step 4: Removing duplicate files...
✓ Removed duplicate 02-architecture/
✓ Removed duplicate 06-development/

================================================
  Phase 1 Complete!
================================================

Summary:
  ✓ Created new folder structure (9 sections)
  ✓ Archived agent documents (3 files)
  ✓ Archived implementation plans (22+ files)
  ✓ Archived SBOM history (8 files)
  ✓ Archived CVE reports (~10 files)
  ✓ Archived organization docs (4 files)
  ✓ Created archive index
  ✓ Removed duplicate folders (2 folders)

Total files archived: ~50 files
New archive location: docs/09-archive/
```

#### Step 4: Verify Results

```bash
# Check new structure created
ls -la docs/

# Expected folders:
# 01-getting-started/
# 02-architecture/
# 03-components/
# 04-development/
# 05-operations/
# 06-reference/
# 07-guides/
# 08-tutorials/
# 09-archive/

# Check archive contents
ls -la docs/09-archive/

# Expected folders:
# agent/
# sbom/
# cve/
# implementation/
# organization/
# cleanup/
# README.md

# Check archive README
cat docs/09-archive/README.md

# Verify files moved correctly
ls docs/09-archive/agent/
# Expected:
# AGENT_DEPLOYMENT_COMPLETE.md
# AGENT_STATUS_FINAL.md
# REFACTORING_PLAN_AGENT_BASED.md
```

#### Step 5: Review Git Changes

```bash
# See what changed
git status

# Expected output:
# Changes not staged for commit:
#   renamed:    docs/AGENT_DEPLOYMENT_COMPLETE.md -> docs/09-archive/agent/AGENT_DEPLOYMENT_COMPLETE.md
#   renamed:    docs/AGENT_STATUS_FINAL.md -> docs/09-archive/agent/AGENT_STATUS_FINAL.md
#   ...
#   new file:   docs/09-archive/README.md
#   ...

# Review specific changes
git diff --stat docs/

# See file moves
git diff --name-status docs/
```

#### Step 6: Commit Changes

```bash
# Stage all changes
git add docs/

# Commit with descriptive message
git commit -m "docs: Phase 1 - Archive outdated documents and create new structure

- Created 9-section documentation structure
- Archived 50+ outdated documents to 09-archive/
- Removed duplicate folders (02-architecture, 06-development)
- Added archive README for reference
- Preserved all historical content

Next: Phase 2 - Move documents to new locations"

# Push to remote (if ready)
git push origin main
```

---

## ✅ Verification Checklist

After Phase 1, verify:

- [ ] New folders exist: `01-getting-started/` through `08-tutorials/`
- [ ] Archive folder exists: `09-archive/`
- [ ] Archive has subfolders: `agent/`, `sbom/`, `cve/`, etc.
- [ ] Archive README exists and is readable
- [ ] Agent files moved to `09-archive/agent/`
- [ ] Implementation plans moved to `09-archive/implementation/`
- [ ] SBOM history moved to `09-archive/sbom/`
- [ ] CVE reports moved to `09-archive/cve/`
- [ ] Duplicate folders removed: `02-architecture/`, `06-development/`
- [ ] Git shows ~50 file moves (not deletions)
- [ ] Backup folder exists: `docs-backup-YYYYMMDD_HHMMSS/`

---

## 🚫 Troubleshooting

### Issue: Script fails with "No such file or directory"

**Cause**: File already moved or doesn't exist  
**Solution**: This is OK - script continues with other files

### Issue: Git shows deleted files instead of moves

**Cause**: Git threshold for rename detection  
**Solution**: 
```bash
# Tell Git to detect moves better
git add -A docs/
git status -M90%  # Detect moves with 90% similarity
```

### Issue: Accidentally deleted backup

**Cause**: Removed backup folder  
**Solution**: 
```bash
# Restore from Git
git restore docs/
# Re-run Phase 1 after creating new backup
```

### Issue: Want to undo Phase 1

**Solution**:
```bash
# Restore from backup
rm -rf docs/
mv docs-backup-YYYYMMDD_HHMMSS docs

# Or restore from Git
git restore docs/
```

---

## 📊 Expected Results After Phase 1

### File Count
- **Before**: 129 files
- **After Phase 1**: 129 files (same, just reorganized)
- **In Archive**: ~50 files
- **In Active Docs**: ~79 files

### Folder Structure
```
docs/
├── 01-getting-started/ (empty, ready for Phase 2)
├── 02-architecture/ (empty, ready for Phase 2)
├── 03-components/ (existing files preserved)
│   ├── agent/
│   ├── cve-scanner/
│   ├── sbom/ (8 files removed to archive)
│   └── INDEX.md
├── 04-development/ (22 files removed to archive)
├── 05-operations/ (empty, ready for Phase 2)
├── 06-reference/ (empty, ready for Phase 2)
├── 07-guides/ (empty, ready for Phase 2)
├── 08-tutorials/ (empty, ready for Phase 2)
├── 09-archive/ (NEW - 50+ archived files)
│   ├── README.md (NEW)
│   ├── agent/
│   ├── sbom/
│   ├── cve/
│   ├── implementation/
│   ├── organization/
│   └── cleanup/
└── (existing root files preserved)
```

---

## ➡️ Next Steps

After completing Phase 1, you're ready for:

### Phase 2: Restructure (Next Step)

**What**: Move documents to new locations  
**When**: After Phase 1 committed  
**Script**: `scripts/restructure-docs-phase2.sh` (to be created)  
**Duration**: ~30 minutes  

**Actions**:
- Move getting-started docs to `01-getting-started/`
- Move architecture docs to `02-architecture/`
- Move component docs to `03-components/*/`
- Rename files for consistency

### Phase 3: Update Content

**What**: Update all content to Core-Only architecture  
**When**: After Phase 2 committed  
**Duration**: ~2 hours  

**Actions**:
- Global replace: KSAM → Fortuna
- Update architecture diagrams (remove Agent)
- Update README.md
- Update ARCHITECTURE.md
- Update START_HERE.md

### Phase 4: Consolidate

**What**: Merge duplicate/overlapping documents  
**When**: After Phase 3 committed  
**Duration**: ~4 hours  

**Actions**:
- Consolidate CVE docs (10 → 1)
- Consolidate SBOM docs (17 → 2)
- Consolidate testing docs (3 → 1)

### Phase 5-6: Create New Docs

**What**: Write missing essential guides  
**When**: After Phase 4 committed  
**Duration**: ~1 week  

**Actions**:
- Write tutorials (5 new)
- Write guides (5 new)
- Write reference docs (3 new)

---

## 📞 Need Help?

### Common Questions

**Q: Can I skip Phase 1 and do it manually?**  
A: Yes, but script is safer and faster. Manual steps available in plan document.

**Q: What if I find an issue after Phase 1?**  
A: Restore from backup or Git, fix script, re-run.

**Q: Can I modify Phase 1 before running?**  
A: Yes, edit `scripts/restructure-docs-phase1.sh` as needed.

**Q: Will this break my current documentation?**  
A: No, Phase 1 only archives. Phase 3 will update links properly.

**Q: How long until all phases complete?**  
A: Phase 1-3 can be done in one day. Phase 4-6 will take 1-2 weeks.

---

## 🎉 Success Criteria

Phase 1 is successful when:

✅ All 9 new folders created  
✅ ~50 files archived with no deletions  
✅ Archive README created  
✅ Duplicate folders removed  
✅ Git history preserved (files moved, not deleted)  
✅ Backup created and verified  
✅ Changes committed to Git  

---

## 📄 Additional Resources

- **Full Plan**: [`docs/DOCUMENTATION_RESTRUCTURE_PLAN.md`](./DOCUMENTATION_RESTRUCTURE_PLAN.md)
- **Summary**: [`docs/DOCUMENTATION_RESTRUCTURE_SUMMARY.md`](./DOCUMENTATION_RESTRUCTURE_SUMMARY.md)
- **Archive Index**: [`docs/09-archive/README.md`](./09-archive/README.md) (after Phase 1)

---

## 🚀 Ready to Begin?

**Checklist**:
- [ ] Read this guide
- [ ] Read summary document
- [ ] Understand Phase 1 scope
- [ ] Ready to create backup
- [ ] Ready to execute script
- [ ] Ready to review results
- [ ] Ready to commit changes

**Execute**:
```bash
# Step 1: Backup
cp -r docs docs-backup-$(date +%Y%m%d_%H%M%S)

# Step 2: Execute
bash scripts/restructure-docs-phase1.sh

# Step 3: Review & Commit
git status
git add docs/
git commit -m "docs: Phase 1 - Archive outdated documents"
```

---

**Good luck! 🎉**

Remember: You have a backup, and everything is in Git. You can always undo and try again.

---

**Last Updated**: December 22, 2024  
**Phase**: 1 of 6  
**Status**: Ready for execution


