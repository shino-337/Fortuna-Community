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

// ExtractTar extracts a tar archive into the filesystem
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

		// Only extract regular files (skip directories, symlinks, etc.)
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Read file content
		content := make([]byte, header.Size)
		if _, err := io.ReadFull(tr, content); err != nil {
			continue // Skip files that can't be read
		}

		// Normalize path
		path := filepath.Clean(header.Name)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
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

// Glob finds files matching a pattern
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

// FileExists checks if a file exists
func (fs *Filesystem) FileExists(path string) bool {
	path = filepath.Clean(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	_, exists := fs.files[path]
	return exists
}

