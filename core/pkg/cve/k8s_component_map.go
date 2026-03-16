package cve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// K8sComponentMapping represents a single component → module prefix mapping entry.
type K8sComponentMapping struct {
	Ecosystem    string `yaml:"ecosystem"`
	ModulePrefix string `yaml:"module_prefix"`
}

type k8sComponentConfig struct {
	Components map[string]K8sComponentMapping `yaml:"components"`
}

var (
	k8sComponentMap     map[string]K8sComponentMapping
	k8sComponentMapOnce sync.Once
	k8sComponentMapErr  error
)

// LoadK8sComponentMap lazily loads the Kubernetes component → module prefix mapping from YAML.
// It is safe for concurrent use.
func LoadK8sComponentMap() (map[string]K8sComponentMapping, error) {
	k8sComponentMapOnce.Do(func() {
		pathEnv := strings.TrimSpace(os.Getenv("FORTUNA_K8S_COMPONENT_MAP_PATH"))
		candidates := []string{}
		if pathEnv != "" {
			candidates = append(candidates, pathEnv)
		}
		// Reasonable defaults for core binary and tests:
		// - running from repo root: core/pkg/cve/k8s_component_map.yaml
		// - running tests from core/: pkg/cve/k8s_component_map.yaml
		// - running tests from pkg/cve/matcher: ../k8s_component_map.yaml
		candidates = append(candidates,
			filepath.Join("core", "pkg", "cve", "k8s_component_map.yaml"),
			filepath.Join("pkg", "cve", "k8s_component_map.yaml"),
			"k8s_component_map.yaml",
			filepath.Join("..", "k8s_component_map.yaml"),
		)

		var data []byte
		var err error
		var usedPath string
		for _, p := range candidates {
			if strings.TrimSpace(p) == "" {
				continue
			}
			if _, statErr := os.Stat(p); statErr != nil {
				continue
			}
			data, err = os.ReadFile(p)
			if err != nil {
				continue
			}
			usedPath = p
			break
		}
		if data == nil || usedPath == "" {
			k8sComponentMapErr = fmt.Errorf("failed to read k8s component map from any candidate path (FORTUNA_K8S_COMPONENT_MAP_PATH, core/pkg/cve/k8s_component_map.yaml, pkg/cve/k8s_component_map.yaml, ../k8s_component_map.yaml)")
			return
		}

		var cfg k8sComponentConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			k8sComponentMapErr = fmt.Errorf("failed to parse k8s component map YAML from %s: %w", usedPath, err)
			return
		}

		res := make(map[string]K8sComponentMapping, len(cfg.Components))
		for name, m := range cfg.Components {
			n := strings.TrimSpace(strings.ToLower(name))
			if n == "" {
				continue
			}
			res[n] = K8sComponentMapping{
				Ecosystem:    strings.TrimSpace(strings.ToLower(m.Ecosystem)),
				ModulePrefix: strings.TrimSpace(m.ModulePrefix),
			}
		}

		k8sComponentMap = res
	})

	return k8sComponentMap, k8sComponentMapErr
}

