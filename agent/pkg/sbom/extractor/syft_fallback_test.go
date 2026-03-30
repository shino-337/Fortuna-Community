package extractor

import "testing"

func TestShouldInvokeSyft_ZeroFortunaDistroless(t *testing.T) {
	e := &Extractor{
		syftEnabled:             true,
		syftAdapter:             &SyftAdapter{},
		syftMinPackageThreshold: 20,
	}

	if !e.shouldInvokeSyft(0, "distroless") {
		t.Fatalf("shouldInvokeSyft(os=distroless, fortuna=0) = false, want true")
	}
}

func TestShouldInvokeSyft_ZeroFortunaDebian(t *testing.T) {
	e := &Extractor{
		syftEnabled:             true,
		syftAdapter:             &SyftAdapter{},
		syftMinPackageThreshold: 20,
	}

	if e.shouldInvokeSyft(0, "debian") {
		t.Fatalf("shouldInvokeSyft(os=debian, fortuna=0) = true, want false")
	}
}

func TestShouldInvokeSyft_DistrolessSmallFortuna(t *testing.T) {
	e := &Extractor{
		syftEnabled:             true,
		syftAdapter:             &SyftAdapter{},
		syftMinPackageThreshold: 20,
	}

	if !e.shouldInvokeSyft(10, "distroless") {
		t.Fatalf("shouldInvokeSyft(os=distroless, fortuna=10) = false, want true")
	}
	if e.shouldInvokeSyft(25, "distroless") {
		t.Fatalf("shouldInvokeSyft(os=distroless, fortuna=25) = true, want false")
	}
}

func TestMergeSyftPackages_PrefersFortunaByPURL(t *testing.T) {
	e := &Extractor{}

	fortuna := []Package{
		{
			Name:       "openssl",
			Version:    "3.0.8-1",
			Type:       "deb",
			PURL:       "pkg:deb/debian/openssl@3.0.8-1",
			Source:     "parsers",
			Confidence: "high",
		},
	}
	syft := []Package{
		{
			Name:       "openssl",
			Version:    "3.0.8-1",
			Type:       "deb",
			PURL:       "pkg:deb/debian/openssl@3.0.8-1",
			Source:     "syft-adapter",
			Confidence: "high",
		},
		{
			Name:       "ca-certificates",
			Version:    "20230311",
			Type:       "deb",
			PURL:       "pkg:deb/debian/ca-certificates@20230311",
			Source:     "syft-adapter",
			Confidence: "high",
		},
	}

	merged := e.mergeSyftPackages(fortuna, syft)
	if len(merged) != 2 {
		t.Fatalf("mergeSyftPackages() len = %d, want 2", len(merged))
	}
	if merged[0].Source != "parsers" || merged[0].Name != "openssl" {
		t.Fatalf("expected fortuna openssl first; got %+v", merged[0])
	}
}

