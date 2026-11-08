package auth

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"image/png"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	totpIssuer     = "Bappelitbangda Sulsel"
	totpPeriod     = 30 // seconds
	totpSecretSize = 20 // bytes
	totpQRSize     = 256
)

// TotpKey is a new TOTP enrollment: the base32 secret, the otpauth URI and a PNG QR code of the URI.
type TotpKey struct {
	Secret string
	URI    string
	PNG    []byte
}

var totpValidateOpts = totp.ValidateOpts{
	Period:    totpPeriod,
	Digits:    otp.DigitsSix,
	Algorithm: otp.AlgorithmSHA1,
}

// NewTotpKey creates a TOTP secret for username (SHA-1, 6 digits, 30-second period).
func NewTotpKey(username string) (TotpKey, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: username,
		Period:      totpPeriod,
		SecretSize:  totpSecretSize,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return TotpKey{}, fmt.Errorf("auth: generate totp key: %w", err)
	}
	img, err := key.Image(totpQRSize, totpQRSize)
	if err != nil {
		return TotpKey{}, fmt.Errorf("auth: render totp qr code: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return TotpKey{}, fmt.Errorf("auth: encode totp qr code: %w", err)
	}
	return TotpKey{Secret: key.Secret(), URI: key.URL(), PNG: buf.Bytes()}, nil
}

// ValidTotpCode reports whether code is exactly six ASCII digits.
func ValidTotpCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for i := range len(code) {
		if code[i] < '0' || code[i] > '9' {
			return false
		}
	}
	return true
}

// MatchTotp checks a 6-digit code against the secret for the steps now-1, now and now+1. It
// returns the largest matching 30-second step, and ok is false when the code does not match.
// pquerna/otp only answers true or false, so the steps are computed here to learn which matched.
func MatchTotp(secret, code string, now time.Time) (step int64, ok bool) {
	if !ValidTotpCode(code) {
		return 0, false
	}
	current := now.Unix() / totpPeriod
	for s := current + 1; s >= current-1; s-- {
		want, err := totp.GenerateCodeCustom(secret, time.Unix(s*totpPeriod, 0), totpValidateOpts)
		if err != nil {
			return 0, false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}
