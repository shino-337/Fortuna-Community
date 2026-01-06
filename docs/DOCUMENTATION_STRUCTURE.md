# Documentation Structure

**Last Updated**: 2026-01-05

## Root Level Files

Main entry points and essential guides:

- `README.md` - Main documentation entry point
- `PRODUCTION_DEPLOYMENT.md` - Complete production deployment guide
- `DEPLOYMENT.md` - Quick deployment reference
- `MIGRATIONS.md` - Database migrations guide
- `BUILD_GUIDE.md` - Build instructions
- `ENVIRONMENT_PREPARATION.md` - Environment setup guide
- `ARCHITECTURE.md` - System architecture documentation
- `DATABASE_SCHEMA_UPDATED.md` - Database schema reference
- `API_REFERENCE.md` - REST API documentation
- `PRODUCTION_READINESS_SUMMARY.md` - Production readiness summary
- `End-to-end-testcase-verify-05012026.md` - E2E test specification

## Directory Structure

### 01-getting-started/
Getting started guides and quick start documentation.

### 02-architecture/
Detailed architecture documentation, ADRs, and design decisions.

### 03-components/
Component-specific documentation (Agent, Core, SBOM, CVE Scanner).

### 04-development/
Development guides, implementation details, and development workflows.

### 05-operations/
Operations guides, troubleshooting (current issues only), and operational procedures.

### 06-reference/
Reference materials, ADRs, migration history, changelog, and technical specifications.

### 07-guides/
How-to guides and step-by-step instructions.

### 08-tutorials/
Tutorials and walkthroughs.

### test-results/
Test execution results and reports.

## File Organization Rules

1. **Root Level**: Only essential, frequently accessed files
2. **Subdirectories**: Organized by purpose and audience
3. **Troubleshooting**: Only current, relevant troubleshooting guides
4. **Old Issues**: Removed or archived in reference/history
5. **Duplicates**: Consolidated into single source of truth

## Maintenance

- Remove outdated troubleshooting/bug/debug files regularly
- Keep root level minimal and focused
- Move implementation details to appropriate subdirectories
- Archive historical documents to 06-reference/history/
