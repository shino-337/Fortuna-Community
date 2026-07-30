package capability

import (
	"fmt"
	"strings"
)

// Normative capability classes (ADR-001). Empty class is treated as "effective" at persistence layer default.
const (
	ClassDeclared  = "declared"
	ClassObserved  = "observed"
	ClassEffective = "effective"
)

var (
	validClasses = map[string]struct{}{
		ClassDeclared: {}, ClassObserved: {}, ClassEffective: {},
	}
	validStates = map[string]struct{}{
		string(StateDetected): {}, string(StateConfirmed): {}, string(StateExploited): {}, string(StateChained): {},
	}
)

// ValidateCapabilityClass returns an error if v is non-empty and not a known class.
func ValidateCapabilityClass(v string) error {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return nil
	}
	if _, ok := validClasses[v]; !ok {
		return fmt.Errorf("capability: invalid capability_class %q (expected declared|observed|effective)", v)
	}
	return nil
}

// ValidateProgressionState returns an error if v is non-empty and not a known progression state.
func ValidateProgressionState(v string) error {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return nil
	}
	if _, ok := validStates[v]; !ok {
		return fmt.Errorf("capability: invalid state %q (expected detected|confirmed|exploited|chained)", v)
	}
	return nil
}

// ValidateClassAndState checks both dimensions independently (ADR-001).
func ValidateClassAndState(capabilityClass, state string) error {
	if err := ValidateCapabilityClass(capabilityClass); err != nil {
		return err
	}
	return ValidateProgressionState(state)
}
