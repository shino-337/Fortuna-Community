package extractor

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	rpmdb "github.com/anchore/go-rpmdb/pkg"
)

// Default path inside the image for a line-based RPM inventory (Sprint C1 / Tier 0 MVP).
// Format: see parseRPMPackagesList.
const defaultRPMInventoryPath = "/var/lib/fortuna/rpm-packages.list"

// RpmParser extracts RPM packages from a Fortuna inventory manifest embedded in the image.
// Full Berkeley DB /var/lib/rpm/Packages parsing is out of scope; build pipelines or base
// images can materialize the inventory file for deterministic SBOM on RPM-based images.
type RpmParser struct{}

// NewRpmParser creates a new rpm parser
func NewRpmParser() *RpmParser {
	return &RpmParser{}
}

// RPM manifest paths for Mariner / Azure Linux distroless (Trivy parity).
var rpmManifestPaths = []string{
	"/var/lib/rpmmanifest/container-manifest-2",
	"/var/lib/rpmmanifest/container-manifest-1",
}

// Parse reads rpm-packages.list (or FORTUNA_RPM_INVENTORY_PATH) and returns packages with stable PURLs.
// Fallback chain: inventory file → rpmmanifest (Mariner distroless) → rpmdb (sqlite/bdb).
func (p *RpmParser) Parse(fs *Filesystem) ([]Package, error) {
	path := strings.TrimSpace(os.Getenv("FORTUNA_RPM_INVENTORY_PATH"))
	if path == "" {
		path = defaultRPMInventoryPath
	}
	if !fs.FileExists(path) {
		// Try Mariner/Azure Linux distroless manifest (rpmqa-format output).
		if pkgs := p.parseRPMManifest(fs); len(pkgs) > 0 {
			return pkgs, nil
		}
		// A4: fallback to parsing rpmdb when Fortuna inventory is missing.
		pkgs, err := p.parseRPMDBFromFileSystem(fs)
		if err != nil {
			return nil, nil
		}
		return pkgs, nil
	}
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, err
	}
	distro := rpmDistroFromOSRelease(fs)
	pkgs, err := parseRPMPackagesList(string(data), distro)
	if err != nil {
		return nil, err
	}
	return pkgs, nil
}

func (p *RpmParser) parseRPMDBFromFileSystem(fs *Filesystem) ([]Package, error) {
	// Candidate paths for modern sqlite-backed rpmdb, NDB and legacy BerkeleyDB "Packages".
	candidates := []string{
		"/var/lib/rpm/rpmdb.sqlite",
		"/usr/lib/sysimage/rpm/rpmdb.sqlite",
		"/usr/lib/sysimage/rpm/rpmdb.sqlite3",
		"/usr/lib/sysimage/rpm/Packages.db",
		"/var/lib/rpm/Packages",
		"/usr/lib/sysimage/rpm/Packages",
	}

	distro := rpmDistroFromOSRelease(fs)

	for _, cand := range candidates {
		if !fs.FileExists(cand) {
			continue
		}
		data, err := fs.ReadFile(cand)
		if err != nil {
			continue
		}

		tmp, err := os.CreateTemp("", "fortuna-rpmdb-*.sqlite")
		if err != nil {
			continue
		}
		tmpPath := tmp.Name()
		_ = tmp.Close()

		f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			_ = os.Remove(tmpPath)
			continue
		}
		if _, err := io.Copy(f, bytes.NewReader(data)); err != nil {
			_ = f.Close()
			_ = os.Remove(tmpPath)
			continue
		}
		_ = f.Close()

		db, err := rpmdb.Open(tmpPath)
		if err != nil {
			_ = os.Remove(tmpPath)
			continue
		}

		pkgsInfo, err := db.ListPackages()
		if err != nil {
			db.Close()
			_ = os.Remove(tmpPath)
			continue
		}

		out := make([]Package, 0, len(pkgsInfo))
		seen := make(map[string]struct{})
		for _, info := range pkgsInfo {
			if info == nil {
				continue
			}
			// Field names differ across rpmdb versions; keep best-effort by using likely getters/fields.
			name := strings.TrimSpace(info.Name)
			arch := strings.TrimSpace(info.Arch)
			ver := strings.TrimSpace(info.Version)
			rel := strings.TrimSpace(info.Release)
			epoch := ""
			if info.Epoch != nil && *info.Epoch != 0 {
				epoch = strconv.Itoa(*info.Epoch)
			}

			if name == "" || ver == "" {
				continue
			}

			evr := ver
			if rel != "" {
				evr = ver + "-" + rel
			}
			if epoch != "" {
				evr = epoch + ":" + evr
			}

			purl := rpmPURL(distro, name, evr, arch)
			key := name + "\x00" + arch + "\x00" + evr
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, Package{
				Name:       name,
				Version:    evr,
				Type:       "rpm",
				Arch:       arch,
				PURL:       purl,
				Source:     "rpmdb-fallback",
				Confidence: "high",
			})
		}
		db.Close()
		_ = os.Remove(tmpPath)
		return out, nil
	}

	// No rpmdb candidate found in filesystem.
	return nil, nil
}

// parseRPMManifest reads rpmmanifest/container-manifest-2 (CBL-Mariner / Azure Linux distroless).
// Format: one package per line as "name-version-release.arch" (output of rpm -qa).
func (p *RpmParser) parseRPMManifest(fs *Filesystem) []Package {
	var data []byte
	for _, mp := range rpmManifestPaths {
		if d, err := fs.ReadFile(mp); err == nil {
			data = d
			break
		}
	}
	if data == nil {
		return nil
	}

	distro := rpmDistroFromOSRelease(fs)
	seen := make(map[string]struct{})
	var out []Package

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Format: name-version-release.arch  OR  name-epoch:version-release.arch
		// Split from the right: arch is after the last '.', release before that after last '-', etc.
		dotIdx := strings.LastIndex(line, ".")
		if dotIdx < 0 {
			continue
		}
		arch := line[dotIdx+1:]
		rest := line[:dotIdx] // name-version-release OR name-epoch:version-release

		relIdx := strings.LastIndex(rest, "-")
		if relIdx < 0 {
			continue
		}
		release := rest[relIdx+1:]
		rest = rest[:relIdx] // name-version OR name-epoch:version

		verIdx := strings.LastIndex(rest, "-")
		if verIdx < 0 {
			continue
		}
		name := rest[:verIdx]
		version := rest[verIdx+1:]

		// Handle epoch prefix in version (e.g., "1:2.36")
		epoch := ""
		if colonIdx := strings.Index(version, ":"); colonIdx > 0 {
			epoch = version[:colonIdx]
			version = version[colonIdx+1:]
		}

		if name == "" || version == "" {
			continue
		}

		evr := version
		if release != "" {
			evr = version + "-" + release
		}
		if epoch != "" && epoch != "0" {
			evr = epoch + ":" + evr
		}

		key := name + "\x00" + arch + "\x00" + evr
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		out = append(out, Package{
			Name:       name,
			Version:    evr,
			Type:       "rpm",
			Arch:       arch,
			PURL:       rpmPURL(distro, name, evr, arch),
			Source:     "rpmmanifest",
			Confidence: "high",
		})
	}
	return out
}

func rpmDistroFromOSRelease(fs *Filesystem) string {
	b, err := fs.ReadFile("/etc/os-release")
	if err != nil {
		return "redhat"
	}
	id := ""
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
			break
		}
	}
	if id == "" {
		return "redhat"
	}
	// PURL namespace: keep short lowercase token
	return strings.ReplaceAll(strings.ToLower(id), " ", "-")
}

// parseRPMPackagesList parses lines:
//   - name<TAB>version<TAB>release<TAB>arch
//   - name|version|release|arch
//   - name<TAB>epoch<TAB>version<TAB>release<TAB>arch
// Lines starting with # or empty are skipped.
func parseRPMPackagesList(content, distro string) ([]Package, error) {
	seen := make(map[string]struct{})
	var out []Package
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fields []string
		if strings.Contains(line, "\t") {
			fields = strings.Split(line, "\t")
		} else if strings.Contains(line, "|") {
			fields = strings.Split(line, "|")
		} else {
			return nil, fmt.Errorf("rpm inventory: unsupported line (need TAB or | separators): %q", line)
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		var name, epoch, version, release, arch string
		switch len(fields) {
		case 4:
			name, version, release, arch = fields[0], fields[1], fields[2], fields[3]
		case 5:
			name, epoch, version, release, arch = fields[0], fields[1], fields[2], fields[3], fields[4]
		default:
			return nil, fmt.Errorf("rpm inventory: want 4 or 5 fields, got %d in %q", len(fields), line)
		}
		if name == "" || version == "" {
			return nil, fmt.Errorf("rpm inventory: empty name or version in %q", line)
		}
		evr := version
		if release != "" {
			evr = version + "-" + release
		}
		if epoch != "" && epoch != "0" {
			evr = epoch + ":" + evr
		}
		key := name + "\x00" + arch + "\x00" + evr
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		purl := rpmPURL(distro, name, evr, arch)
		out = append(out, Package{
			Name:       name,
			Version:    evr,
			Type:       "rpm",
			Arch:       arch,
			PURL:       purl,
			Source:     "rpm-inventory",
			Confidence: "high",
		})
	}
	return out, nil
}

func rpmPURL(distro, name, evr, arch string) string {
	if distro == "" {
		distro = "redhat"
	}
	// Minimal escaping for PURL path/name segments (avoid raw @ in name).
	safeName := strings.ReplaceAll(name, "/", "%2F")
	safeName = strings.ReplaceAll(safeName, "@", "%40")
	safeEvr := strings.ReplaceAll(evr, "@", "%40")
	s := fmt.Sprintf("pkg:rpm/%s/%s@%s", distro, safeName, safeEvr)
	if arch != "" {
		s += "?arch=" + url.QueryEscape(arch)
	}
	return s
}
