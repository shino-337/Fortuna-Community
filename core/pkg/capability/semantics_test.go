package capability

import "testing"

func TestValidateCapabilityClass(t *testing.T) {
	if err := ValidateCapabilityClass("effective"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCapabilityClass(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCapabilityClass("bogus"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateProgressionState(t *testing.T) {
	if err := ValidateProgressionState("exploited"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProgressionState(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProgressionState("maybe"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateClassAndState(t *testing.T) {
	if err := ValidateClassAndState("observed", "detected"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateClassAndState("invalid", "detected"); err == nil {
		t.Fatal("expected error on class")
	}
	if err := ValidateClassAndState("effective", "nope"); err == nil {
		t.Fatal("expected error on state")
	}
}
