package signatures

import (
	"embed"
	"encoding/json"
	"sync"
)

//go:embed distroless.json
var distrolessFS embed.FS

const defaultVersion = "1"

var (
	versionOnce sync.Once
	version     string
	distOnce    sync.Once
	distData    *DistrolessJSON
)

// DistrolessJSON is the schema of distroless.json (Finding #8.3 / B1).
type DistrolessJSON struct {
	Version     string                         `json:"version"`
	Description string                         `json:"description"`
	Binaries    map[string]DistrolessSignature `json:"binaries"`
}

// DistrolessSignature stores PURL and confidence for a known binary.
type DistrolessSignature struct {
	PURL       string `json:"purl"`
	Confidence string `json:"confidence"`
	VersionFromTag bool              `json:"versionFromTag"` // use image tag if present
	DigestMap      map[string]string `json:"digestMap"`      // digest -> version override
	LabelKeys      []string          `json:"labelKeys"`      // OCI label keys to try (e.g., org.opencontainers.image.version)
}

// Version returns the signature DB version for cache invalidation (B3).
// Reads from embedded distroless.json; falls back to defaultVersion on error.
func Version() string {
	versionOnce.Do(func() {
		data, err := distrolessFS.ReadFile("distroless.json")
		if err != nil {
			version = defaultVersion
			return
		}
		var d DistrolessJSON
		if err := json.Unmarshal(data, &d); err != nil {
			version = defaultVersion
			return
		}
		if d.Version != "" {
			version = d.Version
		} else {
			version = defaultVersion
		}
	})
	return version
}

// LoadDistroless parses distroless.json and returns the data (cached).
func LoadDistroless() *DistrolessJSON {
	distOnce.Do(func() {
		data, err := distrolessFS.ReadFile("distroless.json")
		if err != nil {
			distData = &DistrolessJSON{Version: defaultVersion, Binaries: map[string]DistrolessSignature{}}
			return
		}
		var d DistrolessJSON
		if err := json.Unmarshal(data, &d); err != nil {
			distData = &DistrolessJSON{Version: defaultVersion, Binaries: map[string]DistrolessSignature{}}
			return
		}
		if d.Binaries == nil {
			d.Binaries = map[string]DistrolessSignature{}
		}
		distData = &d
	})
	return distData
}
