# Bug Fix History

**Last Updated:** 2025-12-27  
**Status:** Historical Reference

---

## Overview

This document archives significant bug fixes applied to Fortuna K8s Management Platform. For current system behavior, refer to component-specific documentation.

---

## Fix Categories

### Agent Fixes

#### Agent Pod Watcher Fix
**Date:** 2025-12  
**Issue:** Agent not detecting pods correctly  
**Resolution:** Fixed pod watcher event handling  
**Status:** ✅ Resolved

### Core Fixes

#### Async Queue Fix
**Date:** 2025-12  
**Issue:** Async queue processing issues  
**Resolution:** Fixed queue worker implementation  
**Status:** ✅ Resolved

#### Migration Fix
**Date:** 2025-12  
**Issue:** Migration execution problems  
**Resolution:** Fixed migration runner and error handling  
**Status:** ✅ Resolved

### SBOM Fixes

#### SBOM Extractor Fix
**Date:** 2025-12  
**Issue:** SBOM extraction failures  
**Resolution:** Fixed image access and layer extraction  
**Status:** ✅ Resolved

#### SBOM JSONB Fix
**Date:** 2025-12  
**Issue:** JSONB field issues in SBOM storage  
**Resolution:** Fixed JSONB schema and queries  
**Status:** ✅ Resolved

### Parser Fixes

#### OS-Aware Parser Fix
**Date:** 2025-12  
**Issue:** Parser not handling different OS types correctly  
**Resolution:** Fixed OS detection and parsing logic  
**Status:** ✅ Resolved

### SQL Fixes

#### SQL Errors Fix
**Date:** 2025-12  
**Issue:** SQL query errors in various components  
**Resolution:** Fixed SQL syntax and parameter binding  
**Status:** ✅ Resolved

### NATS Fixes

#### NATS Subject Fix
**Date:** 2025-12  
**Issue:** NATS subject naming inconsistencies  
**Resolution:** Standardized subject naming convention  
**Status:** ✅ Resolved

---

## Fix Summary

All listed fixes have been:
- ✅ Applied to codebase
- ✅ Tested and verified
- ✅ Deployed to production
- ✅ Documented in git commits

---

## Notes

- This is a historical reference document
- For current system behavior, see component-specific documentation
- All fixes are tracked in git history
- Detailed fix information available in git commits

---

**For current system documentation, see:**
- [Agent Documentation](../../03-components/agent/README.md)
- [Core Documentation](../../03-components/core/README.md)
- [SBOM Documentation](../../03-components/sbom/README.md)


