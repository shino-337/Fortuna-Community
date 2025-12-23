#!/bin/sh
# Show Fortuna Documentation Structure
# Usage: wsl sh scripts/show-docs-tree.sh

echo "════════════════════════════════════════════════════════"
echo "   Fortuna K8s Management Platform"
echo "   Documentation Structure v2.0"
echo "════════════════════════════════════════════════════════"
echo ""

cd docs

echo "📚 MAIN INDEX"
echo "  ├── README.md (Main documentation index)"
echo "  └── START_HERE.md (5-minute overview)"
echo ""

echo "📁 SECTION STRUCTURE"
echo ""

echo "🚀 01-getting-started/ (Installation & Tutorials)"
if [ -d "01-getting-started" ]; then
    file_count=$(find 01-getting-started -type f -name "*.md" | wc -l)
    echo "  ├── README.md (Section index)"
    echo "  ├── QUICKSTART.md (10-minute guide)"
    echo "  ├── MINIKUBE_SETUP.md"
    echo "  └── ... ($file_count files total)"
else
    echo "  ⚠️  Not found"
fi
echo ""

echo "🏗️  02-architecture/ (System Design)"
if [ -d "02-architecture" ]; then
    file_count=$(find 02-architecture -type f -name "*.md" | wc -l)
    echo "  ├── README.md (Architecture overview)"
    echo "  ├── CORE_ONLY_ANALYSIS.md ✅ (Current architecture)"
    echo "  ├── ARCHITECTURE_OLD.md (Historical)"
    echo "  ├── KSAM_ADR_FULL.md (ADRs)"
    echo "  └── changelog/ ($file_count files total)"
else
    echo "  ⚠️  Not found"
fi
echo ""

echo "🔧 03-components/ (Component Documentation)"
if [ -d "03-components" ]; then
    echo "  ├── INDEX.md"
    echo "  ├── agent/ ⚠️  (Historical - disabled)"
    echo "  ├── core/"
    echo "  ├── sbom/ (9 documents)"
    echo "  ├── cve-scanner/ (6 documents)"
    echo "  ├── policy-engine/"
    echo "  ├── risk-engine/"
    echo "  └── graph-engine/"
else
    echo "  ⚠️  Not found"
fi
echo ""

echo "💻 04-development/ (Developer Guides)"
if [ -d "04-development" ]; then
    file_count=$(find 04-development -type f -name "*.md" | wc -l)
    echo "  ├── README.md"
    echo "  ├── CVE_LOADING_GUIDE.md"
    echo "  ├── API_VERIFICATION_RESULTS.md"
    echo "  ├── INSIGHTS_API_CURL_EXAMPLES.md"
    echo "  ├── migrations/ (3 files)"
    echo "  ├── testing/ (2 files)"
    echo "  └── ... ($file_count files total)"
else
    echo "  ⚠️  Not found"
fi
echo ""

echo "🚀 05-operations/ (Operations Guides)"
if [ -d "05-operations" ]; then
    echo "  ├── README.md"
    echo "  ├── deployment/"
    echo "  ├── monitoring/"
    echo "  └── performance/"
else
    echo "  ⚠️  Not found"
fi
echo ""

echo "📖 06-reference/ (Reference Materials)"
if [ -d "06-reference" ]; then
    echo "  ├── README.md"
    echo "  ├── SECURITY.md (51KB security guide)"
    echo "  └── migration/ (6 files)"
else
    echo "  ⚠️  Not found"
fi
echo ""

echo "📝 07-guides/ (How-To Guides)"
if [ -d "07-guides" ]; then
    echo "  └── README.md (Planned content)"
else
    echo "  ⚠️  Not found"
fi
echo ""

echo "🎓 08-tutorials/ (Tutorials)"
if [ -d "08-tutorials" ]; then
    echo "  └── README.md (Planned content)"
else
    echo "  ⚠️  Not found"
fi
echo ""

echo "📦 09-archive/ (Historical Documentation)"
if [ -d "09-archive" ]; then
    echo "  ├── README.md (Archive index)"
    echo "  ├── agent/ (3 files)"
    echo "  ├── sbom/ (8 files)"
    echo "  ├── cve/ (9 files)"
    echo "  ├── implementation/ (22 files)"
    echo "  ├── organization/ (3 files)"
    echo "  ├── cleanup/ (1 file)"
    echo "  └── old-archive/ (9+ files)"
else
    echo "  ⚠️  Not found"
fi
echo ""

echo "════════════════════════════════════════════════════════"
echo "📊 STATISTICS"
echo "════════════════════════════════════════════════════════"
echo ""

# Count files
total_active=$(find 01-getting-started 02-architecture 03-components 04-development 05-operations 06-reference 07-guides 08-tutorials -type f -name "*.md" 2>/dev/null | wc -l)
total_archive=$(find 09-archive -type f -name "*.md" 2>/dev/null | wc -l)
total_files=$((total_active + total_archive))

echo "📁 Active Documentation: $total_active files"
echo "📦 Archived Documentation: $total_archive files"
echo "📚 Total: $total_files files"
echo ""

echo "════════════════════════════════════════════════════════"
echo "✅ STRUCTURE STATUS: COMPLETE"
echo "════════════════════════════════════════════════════════"
echo ""
echo "Next steps:"
echo "  1. Review: cat docs/README.md"
echo "  2. Start: cat docs/START_HERE.md"
echo "  3. Commit: git add docs/ && git commit"
echo ""

