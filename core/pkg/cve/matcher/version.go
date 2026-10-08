package matcher

// ResolverVersion is the semantic version of resolver/normalization/trust logic used for matching.
// Bump when matching behavior changes so incidents can distinguish data vs logic regressions.
// v1.3: keep gobinary-main for CVE matching, PURL rewrite generic→golang for K8s control plane.
// v1.4: distro packages matched by purl upstream (source) package and version; spec PURL
// decoding; maven/npm/gem/composer/cargo queried in their own ecosystems.
// v1.5: distro advisory ranges scoped to the component's release; open (unfixed) ranges and
// explicit OSV versions matched; withdrawn advisories dropped.
const ResolverVersion = "v1.6"
