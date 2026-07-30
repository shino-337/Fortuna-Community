package contextkeys

import "context"

// sbomMutationKeyType is a typed context key to avoid collisions.
type sbomMutationKeyType struct{}

var sbomMutationKey = sbomMutationKeyType{}

// WithSBOMMutationAllowed returns a derived context that explicitly allows
// SBOM mutations for the current call chain.
func WithSBOMMutationAllowed(ctx context.Context) context.Context {
	return context.WithValue(ctx, sbomMutationKey, true)
}

// IsSBOMMutationAllowed reports whether SBOM mutations are allowed in this context.
// Default is deny (false) when the flag is not present or invalid.
func IsSBOMMutationAllowed(ctx context.Context) bool {
	v := ctx.Value(sbomMutationKey)
	allowed, _ := v.(bool)
	return allowed
}

