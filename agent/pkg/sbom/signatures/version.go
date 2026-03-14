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
)

// DistrolessJSON is the schema of distroless.json (Finding #8.3 / B1).
type DistrolessJSON struct {
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Binaries    []string `json:"binaries"`
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
