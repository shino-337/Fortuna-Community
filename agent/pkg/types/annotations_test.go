package types

import "testing"

func TestSafeAnnotationsDropsManifestAndHidesSecrets(t *testing.T) {
	in := map[string]string{
		"kubectl.kubernetes.io/last-applied-configuration": `{"spec":{"containers":[{"env":[{"name":"DB_PASSWORD","value":"hunter2"}]}]}}`,
		"deployment.kubernetes.io/revision":                "3",
		"example.com/webhook":                              "https://svc:hunter2@hooks.internal/x",
	}
	out := SafeAnnotations(in)
	if _, ok := out["kubectl.kubernetes.io/last-applied-configuration"]; ok {
		t.Fatal("last-applied-configuration must not leave the node")
	}
	if out["deployment.kubernetes.io/revision"] != "3" {
		t.Fatalf("other annotations must be kept: %v", out)
	}
	if out["example.com/webhook"] != "https://svc:[REDACTED]@hooks.internal/x" {
		t.Fatalf("credential in annotation value must be hidden: %q", out["example.com/webhook"])
	}
	if SafeAnnotations(nil) != nil {
		t.Fatal("nil stays nil")
	}
}
