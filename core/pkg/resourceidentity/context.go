package resourceidentity

import "context"

type clusterContextKey struct{}

// WithClusterID attaches a previously validated cluster ownership key to a
// request/background context. Callers must validate the value before calling
// this helper; consumers treat it as a trusted ownership boundary.
func WithClusterID(ctx context.Context, clusterID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, clusterContextKey{}, clusterID)
}

// ClusterIDFromContext returns a trusted cluster ownership key when one has
// been attached by an authentication/ownership boundary.
func ClusterIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	clusterID, ok := ctx.Value(clusterContextKey{}).(string)
	return clusterID, ok && clusterID != ""
}
