package extractor

import "testing"

func TestMavenParser_Parse(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/app/pom.xml"] = []byte(`<?xml version="1.0"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <groupId>com.example</groupId>
  <artifactId>demo</artifactId>
  <version>1.2.3</version>
</project>`)

	p := NewMavenParser()
	pkgs, err := p.Parse(fs)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("len=%d", len(pkgs))
	}
	if pkgs[0].Name != "com.example:demo" || pkgs[0].Version != "1.2.3" || pkgs[0].Type != "maven" {
		t.Fatalf("%+v", pkgs[0])
	}
}
