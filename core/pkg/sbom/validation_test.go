package sbom

import (
	"strings"
	"testing"
)

func TestValidateSBOM_Valid(t *testing.T) {
	input := SBOMInput{
		ImageRef:    "docker.io/library/nginx:latest",
		ImageDigest: "sha256:abc123def456",
		Components: []SBOMComponentInput{
			{PURL: "pkg:deb/debian/libc6@2.31-13"},
		},
	}
	result := ValidateSBOM(input)
	if !result.Valid {
		t.Errorf("expected valid SBOM, got errors: %v", result.Errors)
	}
}

func TestValidateSBOM_MissingImageRef(t *testing.T) {
	input := SBOMInput{
		ImageRef:    "",
		ImageDigest: "sha256:abc123",
		Components:  []SBOMComponentInput{{PURL: "pkg:npm/lodash@4.17.21"}},
	}
	result := ValidateSBOM(input)
	if result.Valid {
		t.Fatal("expected validation failure for missing image ref")
	}
	if !containsError(result.Errors, "image reference is required") {
		t.Errorf("expected 'image reference is required' error, got: %v", result.Errors)
	}
}

func TestValidateSBOM_MissingDigest(t *testing.T) {
	input := SBOMInput{
		ImageRef:    "nginx:latest",
		ImageDigest: "",
		Components:  []SBOMComponentInput{{PURL: "pkg:npm/lodash@4.17.21"}},
	}
	result := ValidateSBOM(input)
	if result.Valid {
		t.Fatal("expected validation failure for missing digest")
	}
	if !containsError(result.Errors, "image digest is required") {
		t.Errorf("expected 'image digest is required' error, got: %v", result.Errors)
	}
}

func TestValidateSBOM_ZeroComponents_Allowed(t *testing.T) {
	// Zero components is a valid state: agents send empty-package SBOMs for failed scans.
	input := SBOMInput{
		ImageRef:    "nginx:latest",
		ImageDigest: "sha256:abc123",
		Components:  nil,
	}
	result := ValidateSBOM(input)
	if !result.Valid {
		t.Errorf("zero components should be allowed (failed scan), got errors: %v", result.Errors)
	}
}

func TestValidateSBOM_MalformedPURL(t *testing.T) {
	input := SBOMInput{
		ImageRef:    "nginx:latest",
		ImageDigest: "sha256:abc123",
		Components: []SBOMComponentInput{
			{PURL: "not-a-purl"},
		},
	}
	result := ValidateSBOM(input)
	if result.Valid {
		t.Fatal("expected validation failure for malformed PURL")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "invalid PURL") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'invalid PURL' error, got: %v", result.Errors)
	}
}

func TestValidateSBOM_EmptyPURLIsAllowed(t *testing.T) {
	input := SBOMInput{
		ImageRef:    "nginx:latest",
		ImageDigest: "sha256:abc123",
		Components: []SBOMComponentInput{
			{PURL: ""},
		},
	}
	result := ValidateSBOM(input)
	if !result.Valid {
		t.Errorf("empty PURL should be allowed, got errors: %v", result.Errors)
	}
}

func TestValidateSBOM_MultipleErrors(t *testing.T) {
	input := SBOMInput{
		ImageRef:    "",
		ImageDigest: "",
		Components:  nil,
	}
	result := ValidateSBOM(input)
	if result.Valid {
		t.Fatal("expected validation failure")
	}
	// Must report errors for both missing image ref and missing digest
	if len(result.Errors) < 2 {
		t.Errorf("expected at least 2 errors (imageRef + digest), got %d: %v", len(result.Errors), result.Errors)
	}
}

// containsError returns true if any error string equals the expected message.
func containsError(errs []string, expected string) bool {
	for _, e := range errs {
		if e == expected {
			return true
		}
	}
	return false
}
