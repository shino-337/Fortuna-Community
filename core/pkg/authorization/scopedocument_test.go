package authorization

import "testing"

func TestScopeDocumentValidate(t *testing.T) {
	d := ScopeDocument{Labels: map[string]string{"a": "b"}}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	big := ScopeDocument{Namespaces: make([]string, 600)}
	if err := big.Validate(); err == nil {
		t.Fatal("expected error")
	}
}
