package extractor

import "testing"

func TestParseGemfileLockSpecs(t *testing.T) {
	data := `GEM
  remote: https://rubygems.org/
  specs:
    rake (13.0.6)
    minitest (5.14.4)

PLATFORMS
  ruby

DEPENDENCIES
  rake

BUNDLED WITH
   2.4.22
`
	pkgs := parseGemfileLockSpecs([]byte(data))
	if len(pkgs) < 2 {
		t.Fatalf("expected >=2 gems, got %d %+v", len(pkgs), pkgs)
	}
}

func TestParseGemfileLockSpecs_MultipleGems(t *testing.T) {
	data := `GEM
  remote: https://rubygems.org/
  specs:
    a (1.0.0)
    b (2.0.0)

PLATFORMS
  ruby
`
	pkgs := parseGemfileLockSpecs([]byte(data))
	if len(pkgs) != 2 {
		t.Fatalf("len=%d %+v", len(pkgs), pkgs)
	}
}

func TestParseGemfileLockSpecs_NoSpecs(t *testing.T) {
	if n := len(parseGemfileLockSpecs([]byte("GEM\n"))); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestRubyGemsParser_Parse(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/app/Gemfile.lock"] = []byte(`GEM
  remote: https://rubygems.org/
  specs:
    rack (2.2.4)
`)

	p := NewRubyGemsParser()
	pkgs, err := p.Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || pkgs[0].Name != "rack" {
		t.Fatalf("%+v", pkgs)
	}
}
