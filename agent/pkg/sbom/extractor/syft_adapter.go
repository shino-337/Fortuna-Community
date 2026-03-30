package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// SyftAdapter is a fallback SBOM package inventory extractor for minimal/distroless images.
// It runs Syft as a comprehensive discovery layer and converts packages into Fortuna format.
type SyftAdapter struct {
	logger  *log.Logger
	timeout time.Duration
	maxPkgs int
	syftBin string
}

func NewSyftAdapter(logger *log.Logger, timeout time.Duration, maxPkgs int) *SyftAdapter {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	if maxPkgs <= 0 {
		maxPkgs = 1000
	}

	syftBin := strings.TrimSpace(os.Getenv("SBOM_SYFT_BIN"))
	if syftBin == "" {
		syftBin = "syft"
	}
	return &SyftAdapter{
		logger:  logger,
		timeout: timeout,
		maxPkgs: maxPkgs,
		syftBin: syftBin,
	}
}

func (s *SyftAdapter) DiscoverPackages(ctx context.Context, imageRef string) ([]Package, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	if s.logger != nil {
		s.logger.Printf("[SyftAdapter] DiscoverPackages: imageRef=%s timeout=%s", imageRef, s.timeout)
	}

	if _, err := exec.LookPath(s.syftBin); err != nil {
		// Syft binary not present => treat as "no fallback" (best-effort).
		if s.logger != nil {
			s.logger.Printf("[SyftAdapter] syft not found (bin=%q); skipping Syft fallback", s.syftBin)
		}
		return nil, nil
	}

	// Syft JSON output typically includes an "artifacts" array with fields:
	// name, version, type, purl (best-effort).
	cmd := exec.CommandContext(ctx, s.syftBin, "packages", imageRef, "-o", "json")
	b, err := cmd.CombinedOutput()
	if err != nil {
		// If the command was killed due to timeout, prefer a clean error message.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("syft discovery timeout after %s", s.timeout)
		}
		return nil, fmt.Errorf("syft discovery failed: %w (output=%q)", err, strings.TrimSpace(string(b)))
	}

	var parsed syftPackagesJSON
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, fmt.Errorf("syft json parse failed: %w", err)
	}
	if len(parsed.Artifacts) == 0 {
		return nil, nil
	}

	out := make([]Package, 0, minInt(s.maxPkgs, len(parsed.Artifacts)))
	seen := make(map[string]bool, len(parsed.Artifacts))

	for _, a := range parsed.Artifacts {
		name := strings.TrimSpace(a.Name)
		version := strings.TrimSpace(a.Version)
		if name == "" {
			continue
		}
		if version == "" {
			version = "unknown"
		}

		fortType := normalizeSyftPackageType(a.Type)
		purl := strings.TrimSpace(a.PURL)
		if purl == "" {
			purl = generateFortunaPURL(fortType, name, version)
		}

		key := purl
		if key == "" {
			key = fortType + "|" + name + "@" + version
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if seen[key] {
			continue
		}
		seen[key] = true

		out = append(out, Package{
			Name:       name,
			Version:    version,
			Type:       fortType,
			PURL:       purl,
			Source:     "syft-adapter",
			Confidence: "high",
		})
		if len(out) >= s.maxPkgs {
			break
		}
	}

	if len(out) == 0 {
		return nil, nil
	}
	if s.logger != nil {
		s.logger.Printf("[SyftAdapter] DiscoverPackages: found %d packages", len(out))
	}
	return out, nil
}

type syftPackagesJSON struct {
	Artifacts []syftArtifactJSON `json:"artifacts"`
}

type syftArtifactJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"`
	PURL    string `json:"purl"`
}

func normalizeSyftPackageType(syftType string) string {
	// Fortuna ecosystems use: deb, rpm, apk, npm, pypi, go, gem, maven, cargo, generic.
	switch strings.ToLower(strings.TrimSpace(syftType)) {
	case "deb":
		return "deb"
	case "rpm":
		return "rpm"
	case "apk":
		return "apk"
	case "npm":
		return "npm"
	case "python", "pypi":
		return "pypi"
	case "go-module", "golang", "go":
		return "go"
	case "gem":
		return "gem"
	case "java-archive", "maven":
		return "maven"
	case "rust-crate", "cargo":
		return "cargo"
	case "binary":
		return "generic"
	default:
		return "generic"
	}
}

func generateFortunaPURL(fortType, name, version string) string {
	switch fortType {
	case "deb":
		return fmt.Sprintf("pkg:deb/debian/%s@%s", name, version)
	case "apk":
		return fmt.Sprintf("pkg:apk/alpine/%s@%s", name, version)
	case "rpm":
		return fmt.Sprintf("pkg:rpm/rhel/%s@%s", name, version)
	case "npm":
		return fmt.Sprintf("pkg:npm/%s@%s", name, version)
	case "pypi":
		return fmt.Sprintf("pkg:pypi/%s@%s", name, version)
	case "gem":
		return fmt.Sprintf("pkg:gem/%s@%s", name, version)
	case "go":
		return fmt.Sprintf("pkg:go/%s@%s", name, version)
	default:
		return fmt.Sprintf("pkg:generic/%s@%s", name, version)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

