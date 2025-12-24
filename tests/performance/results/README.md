# Performance Test Results

This directory contains results from performance benchmark tests.

## File Naming Convention

- `sbom_performance_YYYYMMDD_HHMMSS.txt` - SBOM processing performance
- `cve_matching_performance_YYYYMMDD_HHMMSS.txt` - CVE matching performance
- `insight_generation_performance_YYYYMMDD_HHMMSS.txt` - Insight generation performance

## Metrics Collected

### SBOM Processing
- Average extraction time
- Min/Max extraction times
- Component count per SBOM
- Processing rate (SBOMs/second)

### CVE Matching
- Average matching time
- Min/Max matching times
- Matches per SBOM
- Query performance

### Insight Generation
- Average generation time
- Min/Max generation times
- Insights per CVE match
- Batch processing efficiency

## Performance Targets

- SBOM Processing: < 30 seconds for typical image
- CVE Matching: < 5 seconds for 200 packages
- Insight Generation: < 2 seconds for 100 CVEs
- Total E2E Time: < 60 seconds

## Analyzing Results

1. Compare results across test runs
2. Identify performance regressions
3. Track improvements over time
4. Correlate with code changes

