package auth

import (
	"crypto/rand"
	"fmt"
	"strings"
	"unicode/utf8"
)

// alphabet is the lowercase Crockford base32 alphabet. 32 symbols divide 256, so mapping a random
// byte with b&31 is unbiased.
const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

const (
	temporaryPasswordLen = 16 // 80 bits
	recoveryCodeLen      = 10 // 50 bits
	usernameMin          = 3
	usernameMax          = 32
	fullNameMax          = 100
)

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: read random bytes: %w", err)
	}
	for i, v := range b {
		b[i] = alphabet[v&31]
	}
	return string(b), nil
}

// TemporaryPassword returns a random 16-character password for a first sign-in or a reset.
func TemporaryPassword() (string, error) {
	return randomString(temporaryPasswordLen)
}

// NewRecoveryCode returns a random raw recovery code of 10 characters.
func NewRecoveryCode() (string, error) {
	return randomString(recoveryCodeLen)
}

// FormatRecoveryCode returns the display form of a raw recovery code, for example "abcde-fghjk".
func FormatRecoveryCode(raw string) string {
	half := len(raw) / 2
	return raw[:half] + "-" + raw[half:]
}

// NormalizeRecoveryCode returns the canonical form of user input for a recovery code: separators
// removed, lowercased, and the look-alike letters o, i and l mapped to 0 and 1.
func NormalizeRecoveryCode(in string) string {
	in = strings.ToLower(in)
	var b strings.Builder
	b.Grow(len(in))
	for _, r := range in {
		switch r {
		case '-', ' ':
		case 'o':
			b.WriteByte('0')
		case 'i', 'l':
			b.WriteByte('1')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ValidRecoveryCode reports whether s, already normalized, has the shape of a raw recovery code.
func ValidRecoveryCode(s string) bool {
	if len(s) != recoveryCodeLen {
		return false
	}
	for i := range len(s) {
		if strings.IndexByte(alphabet, s[i]) < 0 {
			return false
		}
	}
	return true
}

// ValidUsername reports whether s matches the username format of docs/modules/auth.md
// (^[a-z0-9._-]{3,32}$).
func ValidUsername(s string) bool {
	if len(s) < usernameMin || len(s) > usernameMax {
		return false
	}
	for i := range len(s) {
		c := s[i]
		ok := c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-'
		if !ok {
			return false
		}
	}
	return true
}

// ValidFullName reports whether s is a full name of 1 to 100 characters.
func ValidFullName(s string) bool {
	n := utf8.RuneCountInString(s)
	return n >= 1 && n <= fullNameMax
}
