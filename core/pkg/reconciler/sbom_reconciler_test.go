package reconciler

import "testing"

func TestOrphanGracePeriod_DefaultAndParse(t *testing.T) {
	t.Setenv("FORTUNA_SBOM_ORPHAN_GRACE_PERIOD", "")
	if got := orphanGracePeriod(); got != defaultOrphanGracePeriod {
		t.Fatalf("default grace period=%v want=%v", got, defaultOrphanGracePeriod)
	}

	t.Setenv("FORTUNA_SBOM_ORPHAN_GRACE_PERIOD", "45m")
	if got := orphanGracePeriod().Minutes(); got != 45 {
		t.Fatalf("parsed grace period minutes=%v want=45", got)
	}

	t.Setenv("FORTUNA_SBOM_ORPHAN_GRACE_PERIOD", "12")
	if got := orphanGracePeriod().Minutes(); got != 12 {
		t.Fatalf("numeric grace period minutes=%v want=12", got)
	}
}
