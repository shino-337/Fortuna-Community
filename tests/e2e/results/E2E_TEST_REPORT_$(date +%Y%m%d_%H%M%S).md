# End-to-End Test Report

**Date**: $(date)  
**Test Pod**: $(cat /tmp/test-pod-name.txt 2>/dev/null || echo "N/A")  
**Pod UID**: $(cat /tmp/pod-uid.txt 2>/dev/null || echo "N/A")

---

## Test Overview

This report documents a complete end-to-end test from pod creation to insight generation.

---

## Test Steps

### 1. Pod Creation
- **Pod Name**: $(cat /tmp/test-pod-name.txt 2>/dev/null || echo "N/A")
- **Image**: $(cat /tmp/image-name.txt 2>/dev/null || echo "N/A")
- **Namespace**: $(cat /tmp/pod-namespace.txt 2>/dev/null || echo "N/A")
- **Status**: Created and Running

### 2. Agent Processing
- Agent detected pod
- SBOM extraction initiated
- SBOM sent to Core

### 3. Database Verification
- SBOM stored in database
- Components extracted
- CVE matches created
- Insights generated

### 4. API Verification
- Insights accessible via API
- Data matches database

---

## Results

*Results will be populated during test execution*

---

**Report Generated**: $(date)

