// Package scanner contains legacy Trivy-based scanning code.
//
// The active production CVE/SBOM flow is the custom SBOM pipeline under `pkg/sbom`
// and event-driven workers under `pkg/worker`.
//
// This package is intentionally kept buildable (as an empty stub) to avoid breaking
// `go test ./...` while the legacy implementation is gated behind build tags.
package scanner


