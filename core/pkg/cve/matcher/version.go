package matcher

// ResolverVersion is the semantic version of resolver/normalization/trust logic used for matching.
// Bump when matching behavior changes so incidents can distinguish data vs logic regressions.
// v1.3: keep gobinary-main for CVE matching, PURL rewrite generic→golang for K8s control plane.
const ResolverVersion = "v1.3"
