package extractor

import (
)

// RpmParser parses RedHat/CentOS packages
// Note: RPM database is Berkeley DB format, complex to parse
// For now, this is a placeholder that can be enhanced later
type RpmParser struct{}

// NewRpmParser creates a new rpm parser
func NewRpmParser() *RpmParser {
	return &RpmParser{}
}

// Parse parses rpm packages
// TODO: Implement full RPM database parsing
// For now, returns empty (can be enhanced with Berkeley DB library)
func (p *RpmParser) Parse(fs *Filesystem) ([]Package, error) {
	// RPM database is at /var/lib/rpm/Packages (Berkeley DB format)
	// This requires a Berkeley DB library or using rpm command
	// For initial implementation, we'll skip this
	// Can be enhanced later with github.com/etcd-io/bbolt or similar

	// Check if RPM database exists
	if !fs.FileExists("/var/lib/rpm/Packages") {
		return nil, nil // Not an RPM-based image
	}

	// TODO: Implement RPM database parsing
	// For now, return empty (will be implemented in Phase 2)
	return []Package{}, nil
}

