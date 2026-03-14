package extractor

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
)

// Filesystem represents a virtual filesystem extracted from image layers
type Filesystem struct {
	files map[string][]byte
	logger *log.Logger
}

// NewFilesystem creates a new filesystem
func NewFilesystem() *Filesystem {
	return &Filesystem{
		files: make(map[string][]byte),
		logger: log.New(log.Writer(), "[Filesystem] ", log.LstdFlags),
	}
}

// ExtractTar extracts a tar archive into the filesystem (OCI overlay semantics).
// Handles whiteout (.wh.filename, .wh..wh..opq) so layer N can remove files from layer N-1.
func (fs *Filesystem) ExtractTar(ctx context.Context, r io.Reader) error {
	tr := tar.NewReader(r)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Normalize path once (used for whiteout and for storing)
		path := filepath.Clean(header.Name)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		dir := filepath.Dir(path)
		base := filepath.Base(path)

		// OCI whiteout: .wh.<name> means remove <name> from lower layers
		if strings.HasPrefix(base, ".wh.") {
			if base == ".wh..wh..opq" {
				// Opaque dir: remove all files under dir from lower layers
				prefix := dir + "/"
				for p := range fs.files {
					if strings.HasPrefix(p, prefix) || p == dir {
						delete(fs.files, p)
					}
				}
			} else {
				// Remove single file/dir: target = dir + name without .wh.
				target := filepath.Join(dir, strings.TrimPrefix(base, ".wh."))
				if !strings.HasPrefix(target, "/") {
					target = "/" + target
				}
				delete(fs.files, target)
			}
			// Consume body so next header is valid
			if header.Size > 0 {
				_, _ = io.CopyN(io.Discard, tr, header.Size)
			}
			continue
		}

		// Only store regular files (symlinks/dirs skipped for SBOM file-level scan)
		if header.Typeflag != tar.TypeReg {
			if header.Size > 0 {
				_, _ = io.CopyN(io.Discard, tr, header.Size)
			}
			continue
		}

		// Read file content
		content := make([]byte, header.Size)
		if _, err := io.ReadFull(tr, content); err != nil {
			continue
		}

		fs.files[path] = content
	}

	return nil
}

// ReadFile reads a file from the filesystem
func (fs *Filesystem) ReadFile(path string) ([]byte, error) {
	// Normalize path
	path = filepath.Clean(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	content, ok := fs.files[path]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	return content, nil
}

// Glob finds files matching a pattern (single * per component; no **).
func (fs *Filesystem) Glob(pattern string) []string {
	matches := make([]string, 0)
	for path := range fs.files {
		matched, err := filepath.Match(pattern, path)
		if err != nil {
			continue
		}
		if matched {
			matches = append(matches, path)
		}
	}
	return matches
}

// FindPathsBySuffix returns all stored paths ending with suffix (e.g. "package-lock.json").
// Used by npm parser to discover lock files anywhere in the image.
func (fs *Filesystem) FindPathsBySuffix(suffix string) []string {
	out := make([]string, 0)
	for path := range fs.files {
		if strings.HasSuffix(path, suffix) {
			out = append(out, path)
		}
	}
	return out
}

// FindPathsContaining returns all stored paths that contain sub and end with end (e.g. "node_modules", "package.json").
// Used by npm parser to discover node_modules/*/package.json anywhere in the image.
func (fs *Filesystem) FindPathsContaining(sub, end string) []string {
	out := make([]string, 0)
	for path := range fs.files {
		if strings.Contains(path, sub) && strings.HasSuffix(path, end) {
			out = append(out, path)
		}
	}
	return out
}

// FileExists checks if a file exists
func (fs *Filesystem) FileExists(path string) bool {
	path = filepath.Clean(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	_, exists := fs.files[path]
	return exists
}

// PathsUnder returns all stored paths that have the given prefix (e.g. "/bin/").
// Used by distroless parser to discover binaries under /bin, /usr/bin, /usr/lib.
func (fs *Filesystem) PathsUnder(prefix string) []string {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}
	out := make([]string, 0)
	for path := range fs.files {
		if strings.HasPrefix(path, prefix) {
			out = append(out, path)
		}
	}
	return out
}
