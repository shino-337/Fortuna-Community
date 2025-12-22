//go:build legacy_trivy

package scanner

import "strings"

// parseImageRefFromString parses image reference into name and tag
func parseImageRefFromString(imageRef string) (string, string) {
	// Remove registry if present
	parts := strings.Split(imageRef, "/")
	imagePart := parts[len(parts)-1]

	// Split name and tag
	if strings.Contains(imagePart, ":") {
		parts := strings.Split(imagePart, ":")
		return parts[0], parts[1]
	}
	return imagePart, "latest"
}


