# ADR-001: Version Comparison Strategy

**Status**: Accepted  
**Date**: 2025-12-28  
**Deciders**: Architecture Team  
**Context**: CVE matching requires accurate version comparison across multiple package ecosystems (Debian, Alpine, RPM, npm, etc.)

---

## Context

The KSAM platform needs to match installed package versions against CVE vulnerability ranges to determine if a resource is vulnerable. Version comparison is critical for:

1. **CVE Matching**: Determining if an installed package version falls within a vulnerable range
2. **Insight Generation**: Creating accurate security insights based on version comparisons
3. **Multi-Ecosystem Support**: Handling different version formats (Debian, Alpine, RPM, npm, Go, etc.)

### Problem

Different package ecosystems use different version formats:
- **Debian**: `2.12.7+dfsg+really2.9.14-2.1+deb13u2` (complex with repackaging markers)
- **Alpine**: `1.1.1g-r0` (simple with release suffix)
- **RPM**: `1:1.1.1k-5.el8` (epoch:version-release)
- **npm**: `1.2.3` (semantic versioning)
- **Go**: `v1.2.3` (semantic versioning with prefix)

Manual implementation of version comparison algorithms is:
- Error-prone
- Difficult to maintain
- May not match official ecosystem behavior
- Requires extensive testing

---

## Decision

We will use **ecosystem-specific comparator libraries** instead of implementing version comparison algorithms ourselves.

### Key Principles

1. **No Self-Implementation**: Do not implement version comparison algorithms from scratch
2. **Ecosystem-Specific Comparators**: Use dedicated libraries for each ecosystem
3. **OSV Range Model**: Follow OSV.dev range model for vulnerability ranges
4. **No Implicit Semver Fallback**: Explicitly handle each ecosystem, no silent fallback to semantic versioning

---

## Architecture Decision

### 1. Use Proven Libraries

For each ecosystem, use well-tested, maintained libraries:

- **Debian/Ubuntu**: `github.com/knqyf263/go-deb-version`
  - Implements full Debian version comparison algorithm
  - Matches `dpkg --compare-versions` behavior
  - Handles complex formats (epoch, repackaging markers, etc.)

- **Alpine**: Custom implementation (simple format) or library if available
  - Format: `version-rN`
  - Can strip `-rN` suffix and use semantic versioning

- **RPM/RedHat/CentOS**: Use library or implement based on RPM spec
  - Format: `[epoch:]version-release`
  - Consider: `github.com/knqyf263/go-rpm-version` (if available)

- **npm/PyPI/Go**: `github.com/hashicorp/go-version`
  - Semantic versioning (SemVer)
  - Well-established library

### 2. Ecosystem-Specific Comparator Pattern

```go
type VersionComparator struct {
    debianComparator *debversion.Version
    // ... other ecosystem comparators
}

func (vc *VersionComparator) IsVulnerable(
    installedVersion string,
    constraint string,
    ecosystem string,
) (bool, error) {
    switch ecosystem {
    case "deb", "debian", "ubuntu":
        return vc.compareDebianVersion(installedVersion, constraint)
    case "apk", "alpine":
        return vc.compareAlpineVersion(installedVersion, constraint)
    case "rpm", "redhat", "centos":
        return vc.compareRPMVersion(installedVersion, constraint)
    case "npm", "pypi", "go":
        return vc.compareSemver(installedVersion, constraint)
    default:
        return false, fmt.Errorf("unsupported ecosystem: %s", ecosystem)
    }
}
```

### 3. OSV Range Model

Follow OSV.dev range model for vulnerability constraints:
- `version_start_including`: `>= 1.0.0`
- `version_start_excluding`: `> 1.0.0`
- `version_end_including`: `<= 2.0.0`
- `version_end_excluding`: `< 2.0.0`
- `fixed_version`: Exact version where vulnerability is fixed

Convert OSV ranges to comparator constraints:
```go
// Example: version_start_including="1.0.0", version_end_excluding="2.0.0"
constraint := ">= 1.0.0, < 2.0.0"
```

### 4. No Implicit Semver Fallback

**Explicit Error Handling**:
- If ecosystem is unknown → return error (don't silently fallback)
- If version parsing fails → return error with context
- Log all version comparison failures for debugging

**Before (Bad)**:
```go
default:
    // Fallback to semantic versioning
    return vc.compareSemver(installedVersion, constraint)
```

**After (Good)**:
```go
default:
    return false, fmt.Errorf("unsupported ecosystem: %s", ecosystem)
```

---

## Consequences

### Positive

1. **Accuracy**: Version comparisons match official ecosystem behavior
2. **Maintainability**: Less code to maintain, libraries handle edge cases
3. **Reliability**: Well-tested libraries reduce bugs
4. **Consistency**: Matches behavior of other security tools (Trivy, etc.)

### Negative

1. **Dependencies**: Additional Go dependencies
2. **Library Updates**: Need to track and update library versions
3. **Ecosystem Coverage**: May need to add libraries as new ecosystems are supported

### Risks

1. **Library Maintenance**: If library becomes unmaintained, may need to switch
2. **Performance**: Library overhead (minimal, but exists)
3. **Compatibility**: Library version compatibility with Go version

### Mitigation

1. **Dependency Management**: Use Go modules with version pinning
2. **Testing**: Comprehensive test suite for version comparisons
3. **Monitoring**: Log version comparison failures for analysis
4. **Fallback Strategy**: Document fallback approach if library fails

---

## Implementation

### Phase 1: Debian Support (Current)
- Add `github.com/knqyf263/go-deb-version`
- Update `compareDebianVersion` to use library
- Remove manual implementation

### Phase 2: Other Ecosystems
- Evaluate libraries for RPM, Alpine
- Update comparators as needed
- Add comprehensive tests

### Phase 3: Monitoring & Optimization
- Add metrics for version comparison performance
- Monitor failure rates
- Optimize as needed

---

## References

- [OSV.dev Range Model](https://ossf.github.io/osv-schema/#affectedranges-field)
- [Debian Version Format](https://www.debian.org/doc/debian-policy/ch-controlfields.html#version)
- [go-deb-version Library](https://github.com/knqyf263/go-deb-version)
- [dpkg --compare-versions](https://manpages.debian.org/testing/dpkg-dev/dpkg.1.en.html)

---

## Status

- ✅ **Accepted**: 2025-12-28
- ✅ **Implemented**: Phase 1 (Debian) - In Progress
- ⏳ **Pending**: Phase 2 (Other Ecosystems)

---

**Last Updated**: 2025-12-28

