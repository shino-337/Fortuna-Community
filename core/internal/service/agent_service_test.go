package service

import "testing"

func TestEqualJSONIgnoresObjectKeyOrder(t *testing.T) {
	a := `{"name":"admin","rules":[{"verbs":["get","list"],"resources":["pods"]}]}`
	b := `{"rules":[{"resources":["pods"],"verbs":["get","list"]}],"name":"admin"}`

	if !equalJSON(a, b) {
		t.Fatalf("expected semantically equal JSON to match")
	}
}

func TestEqualJSONDetectsDifferentArrays(t *testing.T) {
	a := `{"subjects":["a","b"]}`
	b := `{"subjects":["b","a"]}`

	if equalJSON(a, b) {
		t.Fatalf("expected array order changes to differ")
	}
}
