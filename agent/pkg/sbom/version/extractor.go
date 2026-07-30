package version

import (
	"bytes"
	"debug/buildinfo"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// Extractor extracts best-effort version information from binaries.
// It is designed to be used when package-manager metadata is missing (e.g., distroless).
type Extractor struct {
	logger *log.Logger
}

func NewExtractor(logger *log.Logger) *Extractor {
	return &Extractor{logger: logger}
}

// ExtractFromBinary returns (version, source, confidence).
// Priority:
//  1) Go buildinfo (high confidence for Go-based control-plane components)
//  2) Embedded string heuristics (medium confidence; best-effort)
func (e *Extractor) ExtractFromBinary(binaryContent []byte, componentName string) (string, string, string) {
	if len(binaryContent) < 1024 {
		return "", "", ""
	}

	// 1) Go buildinfo
	if v, src, conf := e.extractGoBuildInfo(binaryContent); v != "" {
		return v, src, conf
	}

	// 2) String heuristics (limited best-effort)
	if componentName == "" {
		componentName = "version"
	}
	if v, src, conf := e.extractFromComponentStrings(binaryContent, componentName); v != "" {
		return v, src, conf
	}

	return "", "", ""
}

func (e *Extractor) extractGoBuildInfo(binaryContent []byte) (string, string, string) {
	r := bytes.NewReader(binaryContent)
	bi, err := buildinfo.Read(r)
	if err != nil || bi == nil {
		return "", "", ""
	}
	ver := strings.TrimSpace(bi.Main.Version)
	if ver == "" || ver == "(devel)" {
		return "", "", ""
	}
	if e.logger != nil {
		e.logger.Printf("[VERSION] extracted via go-buildinfo: %s", ver)
	}
	return ver, "go-buildinfo", "high"
}

func (e *Extractor) extractFromComponentStrings(binaryContent []byte, componentName string) (string, string, string) {
	// Avoid converting the full binary to string if it's huge.
	// We still use small windows because version strings are typically stored in read-only sections.
	const windowBytes = 4 << 20 // 4 MiB
	b := binaryContent
	var windows [][]byte
	if len(b) <= 2*windowBytes {
		windows = [][]byte{b}
	} else {
		windows = [][]byte{b[:windowBytes], b[len(b)-windowBytes:]}
	}

	escaped := regexp.QuoteMeta(componentName)
	// Capture:
	//   <componentName> ... v?1.2.3[-rc1|+build]?
	verRe := regexp.MustCompile(`(?i)` + escaped + `[^0-9]{0,40}v?([0-9]+(?:\.[0-9]+)+(?:[-+][0-9A-Za-z\.\_]+)?)`)

	for _, w := range windows {
		m := verRe.FindSubmatch(w)
		if len(m) == 2 {
			return string(m[1]), fmt.Sprintf("strings:%s", componentName), "medium"
		}
		// Fallback: componentName missing but version string present; only accept if it looks like semver.
		anyVerRe := regexp.MustCompile(`(?i)\bv?([0-9]+(?:\.[0-9]+)+(?:[-+][0-9A-Za-z\.\_]+)?)\b`)
		m2 := anyVerRe.FindSubmatch(w)
		if len(m2) == 2 {
			return string(m2[1]), "strings:generic", "low"
		}
	}
	return "", "", ""
}

