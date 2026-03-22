//go:build legacy_trivy

package scanner

import "strings"

// parseImageRefFromString parses image reference into name and tag
func parseImageRefFromString(imageRef string) (string, string) {
	imageRef = strings.TrimSpace(imageRef)
	if imageRef == "" {
		return "", "latest"
	}
	// Digest reference: repo/name@sha256:...
	if at := strings.LastIndex(imageRef, "@"); at > 0 && at < len(imageRef)-1 {
		return imageRef[:at], imageRef[at+1:]
	}
	// Tag reference: only treat ":" as tag separator if it appears after the last "/"
	// so registry ports (e.g. registry:5000/repo/image:tag) are handled correctly.
	lastSlash := strings.LastIndex(imageRef, "/")
	lastColon := strings.LastIndex(imageRef, ":")
	if lastColon > lastSlash && lastColon < len(imageRef)-1 {
		return imageRef[:lastColon], imageRef[lastColon+1:]
	}
	if lastColon > lastSlash && lastColon == len(imageRef)-1 {
		return strings.TrimSuffix(imageRef, ":"), "latest"
	}
	return imageRef, "latest"
}


