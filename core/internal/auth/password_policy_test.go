package auth

import "testing"

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string // empty = ok
	}{
		{"ok", "Fortuna_DevOnly_P@ssw0rd", ""},
		{"short", "Ab1!", ErrPasswordTooShort.Error()},
		{"no upper", "fortuna_devonly_p@ssw0rd", ErrPasswordMissingUpper.Error()},
		{"no lower", "FORTUNA_DEVONLY_P@SSW0RD", ErrPasswordMissingLower.Error()},
		{"no digit", "Fortuna_DevOnly_P@ssword", ErrPasswordMissingDigit.Error()},
		{"no special", "FortunaDevOnlyPassw0rd", ErrPasswordMissingSpecial.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.in)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.want {
				t.Fatalf("want %q got %v", tt.want, err)
			}
		})
	}
}
