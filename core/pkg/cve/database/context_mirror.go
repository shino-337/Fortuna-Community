package database

import (
	"context"
	"strings"
)

// freezeMirrorKey is a private context key type (per mirror name).
type freezeMirrorKey struct {
	name string
}

// WithFrozenMirrorVersion attaches a frozen mirror version string for the given mirror name (e.g. "osv").
// getMirrorVersion / cache suffix use this value when present so one matcher run does not drift if mirror_state bumps mid-run.
func WithFrozenMirrorVersion(ctx context.Context, mirrorName, version string) context.Context {
	n := strings.ToLower(strings.TrimSpace(mirrorName))
	v := strings.TrimSpace(version)
	if n == "" || v == "" {
		return ctx
	}
	return context.WithValue(ctx, freezeMirrorKey{name: n}, v)
}

func frozenMirrorVersionFromContext(ctx context.Context, mirrorName string) string {
	n := strings.ToLower(strings.TrimSpace(mirrorName))
	if n == "" {
		return ""
	}
	v, _ := ctx.Value(freezeMirrorKey{name: n}).(string)
	return strings.TrimSpace(v)
}
