package version

import "testing"

func TestNormalizeDebVersionStripsSuffixes(t *testing.T) {
	in := "2:3.0.8-1~deb12u2+deb12u3"
	got := NormalizeVersionForPURL("pkg:deb/debian/openssl@"+in, "deb", in)
	// Expected: remove epoch and distro/build suffixes.
	want := "3.0.8-1"
	if got != want {
		t.Fatalf("NormalizeVersionForPURL() = %q, want %q", got, want)
	}
}

func TestNormalizeGoVersionStripsLeadingV(t *testing.T) {
	in := "v1.10.1"
	got := NormalizeVersionForPURL("pkg:golang/github.com/coredns/coredns@"+in, "generic", in)
	want := "1.10.1"
	if got != want {
		t.Fatalf("NormalizeVersionForPURL() = %q, want %q", got, want)
	}
}

func TestUpdatePURLVersionPreservesQuery(t *testing.T) {
	purl := "pkg:deb/debian/openssl@3.0.8-1~deb12u2?arch=amd64"
	got := UpdatePURLVersion(purl, "3.0.8-1")
	want := "pkg:deb/debian/openssl@3.0.8-1?arch=amd64"
	if got != want {
		t.Fatalf("UpdatePURLVersion() = %q, want %q", got, want)
	}
}

