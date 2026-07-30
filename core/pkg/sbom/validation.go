package sbom

import (
	"fmt"
	"strings"
)

// ValidationResult holds the outcome of ValidateSBOM.
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// SBOMInput is the minimal contract that ValidateSBOM checks.
// Populate it from the gRPC request fields before calling validation.
type SBOMInput struct {
	// ImageRef is the container image reference (e.g. "docker.io/library/nginx:latest").
	ImageRef string
	// ImageDigest is the content-addressable digest (e.g. "sha256:abc...").
	ImageDigest string
	// Components holds per-package data from the agent.
	Components []SBOMComponentInput
}

// SBOMComponentInput carries the per-component fields that require validation.
type SBOMComponentInput struct {
	// PURL is the Package URL for the component. May be empty — only validated when present.
	PURL string
}

// ValidateSBOM validates the structural and semantic integrity of an incoming SBOM.
//
// Rules:
//  1. ImageRef must not be empty.
//  2. ImageDigest must not be empty.
//  3. Every non-empty PURL must start with "pkg:" (minimal Package-URL scheme check).
//
// Note: zero components is intentionally allowed because agents send empty-package SBOMs
// to report scan failures (pull_error / extraction_error).  The handler marks these as
// status="failed" which is a legitimate, well-defined state.
func ValidateSBOM(input SBOMInput) ValidationResult {
	var errs []string

	if strings.TrimSpace(input.ImageRef) == "" {
		errs = append(errs, "image reference is required")
	}
	if strings.TrimSpace(input.ImageDigest) == "" {
		errs = append(errs, "image digest is required")
	}

	for i, c := range input.Components {
		purl := strings.TrimSpace(c.PURL)
		if purl == "" {
			continue
		}
		if err := validatePURL(purl); err != nil {
			errs = append(errs, fmt.Sprintf("component[%d] invalid PURL %q: %s", i, purl, err.Error()))
		}
	}

	return ValidationResult{
		Valid:  len(errs) == 0,
		Errors: errs,
	}
}

// validatePURL performs a minimal Package-URL format check.
// A valid PURL must begin with "pkg:" followed by a non-empty type segment.
func validatePURL(purl string) error {
	if !strings.HasPrefix(purl, "pkg:") {
		return fmt.Errorf("must start with 'pkg:'")
	}
	rest := purl[len("pkg:"):]
	if rest == "" || rest == "/" {
		return fmt.Errorf("type segment missing after 'pkg:'")
	}
	// Type is everything before the first '/'
	typeEnd := strings.IndexByte(rest, '/')
	var purlType string
	if typeEnd < 0 {
		purlType = rest
	} else {
		purlType = rest[:typeEnd]
	}
	if strings.TrimSpace(purlType) == "" {
		return fmt.Errorf("PURL type segment must not be empty")
	}
	return nil
}
