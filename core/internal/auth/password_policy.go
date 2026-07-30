package auth

import (
	"errors"
	"unicode"
)

// MinPasswordLength is the minimum length for user-chosen and bootstrap admin passwords
// when policy is enforced.
const MinPasswordLength = 12

var (
	// ErrPasswordTooShort is returned when the password is shorter than MinPasswordLength.
	ErrPasswordTooShort = errors.New("password must be at least 12 characters")
	// ErrPasswordMissingUpper is returned when no ASCII uppercase letter is present.
	ErrPasswordMissingUpper = errors.New("password must contain at least one uppercase letter")
	// ErrPasswordMissingLower is returned when no ASCII lowercase letter is present.
	ErrPasswordMissingLower = errors.New("password must contain at least one lowercase letter")
	// ErrPasswordMissingDigit is returned when no ASCII digit is present.
	ErrPasswordMissingDigit = errors.New("password must contain at least one number")
	// ErrPasswordMissingSpecial is returned when no special character is present.
	ErrPasswordMissingSpecial = errors.New("password must contain at least one special character (!@#$%^&*()_+-=[]{}|;:,.<>?)")
)

// ValidatePasswordStrength enforces length and complexity for interactive accounts and bootstrap.
func ValidatePasswordStrength(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r) && r <= unicode.MaxASCII:
			hasUpper = true
		case unicode.IsLower(r) && r <= unicode.MaxASCII:
			hasLower = true
		case unicode.IsDigit(r) && r <= unicode.MaxASCII:
			hasDigit = true
		case isPasswordSpecial(r):
			hasSpecial = true
		}
	}
	if !hasUpper {
		return ErrPasswordMissingUpper
	}
	if !hasLower {
		return ErrPasswordMissingLower
	}
	if !hasDigit {
		return ErrPasswordMissingDigit
	}
	if !hasSpecial {
		return ErrPasswordMissingSpecial
	}
	return nil
}

func isPasswordSpecial(r rune) bool {
	switch r {
	case '!', '@', '#', '$', '%', '^', '&', '*', '(', ')', '_', '+', '-', '=', '[', ']', '{', '}', '|', ';', ':', ',', '.', '<', '>', '?':
		return true
	default:
		return false
	}
}
