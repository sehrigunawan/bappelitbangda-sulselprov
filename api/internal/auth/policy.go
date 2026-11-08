package auth

import (
	_ "embed"
	"strings"
	"sync"
	"unicode/utf8"
)

// Password length bounds in Unicode code points (docs/modules/auth.md "Passwords").
const (
	minPasswordLen = 12
	maxPasswordLen = 128
)

// Detail texts of the password policy; the client shows them as they are (NFR-16).
const (
	detailTooShort  = "Minimal 12 karakter."
	detailTooLong   = "Maksimal 128 karakter."
	detailHasName   = "Kata sandi tidak boleh memuat nama pengguna."
	detailTooCommon = "Kata sandi terlalu umum."
)

//go:embed common-passwords.txt
var commonPasswordsFile string

// commonPasswords is the lowercased set of the embedded list, built once on first use.
var commonPasswords = sync.OnceValue(func() map[string]struct{} {
	set := make(map[string]struct{}, 1300)
	for _, line := range strings.Split(commonPasswordsFile, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "# ") {
			continue
		}
		set[strings.ToLower(line)] = struct{}{}
	}
	return set
})

// CheckPassword validates a new password for username. It returns "" when the password is
// acceptable, or the Indonesian detail text the client shows otherwise.
func CheckPassword(username, password string) string {
	n := utf8.RuneCountInString(password)
	switch {
	case n < minPasswordLen:
		return detailTooShort
	case n > maxPasswordLen:
		return detailTooLong
	}
	lower := strings.ToLower(password)
	if username != "" && strings.Contains(lower, strings.ToLower(username)) {
		return detailHasName
	}
	if _, found := commonPasswords()[lower]; found {
		return detailTooCommon
	}
	return ""
}
