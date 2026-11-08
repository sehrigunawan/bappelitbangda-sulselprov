package server_test

import (
	"bytes"
	"context"
	"encoding/base32"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
)

// A15: TOTP verify: valid gives authenticated with a renewed token; wrong gives 401; replay of the same code gives 401; the 5th wrong code gives 429
func TestTotpVerification(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("totp.verify", []authz.Role{authz.SuperAdmin}, seed{totp: true})

	// 1. Valid TOTP verification: returns authenticated with renewed session token.
	cA := newClient(env)
	recA := cA.post("/api/v1/admin/auth/session", map[string]string{
		"username": "totp.verify",
		"password": testPassword,
	})
	if recA.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", recA.Code, http.StatusOK)
	}
	pendingToken := cA.cookie.Value

	validCode := env.code("totp.verify", env.clock.Now())
	recVerify := cA.post("/api/v1/admin/auth/session/totp", map[string]string{
		"code": validCode,
	})
	if recVerify.Code != http.StatusOK {
		t.Fatalf("totp verification status = %d, want %d", recVerify.Code, http.StatusOK)
	}
	state := decode[api.SessionState](t, recVerify)
	if state.Stage != api.Authenticated {
		t.Fatalf("stage = %q, want %q", state.Stage, api.Authenticated)
	}
	if cA.cookie == nil || cA.cookie.Value == pendingToken {
		t.Fatalf("expected renewed cookie token after TOTP verification")
	}

	// 2. Wrong code gives 401.
	cB := newClient(env)
	recB := cB.post("/api/v1/admin/auth/session", map[string]string{
		"username": "totp.verify",
		"password": testPassword,
	})
	if recB.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", recB.Code, http.StatusOK)
	}
	wrongCode := "000000"
	if wrongCode == env.code("totp.verify", env.clock.Now()) {
		wrongCode = "999999"
	}
	recWrong := cB.post("/api/v1/admin/auth/session/totp", map[string]string{
		"code": wrongCode,
	})
	if recWrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong code status = %d, want %d", recWrong.Code, http.StatusUnauthorized)
	}
	assertJSONEqual(t, recWrong.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)

	// 3. Replay of the same code again in a new login gives 401.
	cC := newClient(env)
	recC := cC.post("/api/v1/admin/auth/session", map[string]string{
		"username": "totp.verify",
		"password": testPassword,
	})
	if recC.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", recC.Code, http.StatusOK)
	}
	recReplay := cC.post("/api/v1/admin/auth/session/totp", map[string]string{
		"code": validCode,
	})
	if recReplay.Code != http.StatusUnauthorized {
		t.Fatalf("replayed code status = %d, want %d", recReplay.Code, http.StatusUnauthorized)
	}
	assertJSONEqual(t, recReplay.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)

	// 4. 5 wrong codes -> the 5th gives 429 with Retry-After 900.
	env.seedUser("totp.throttle", []authz.Role{authz.SuperAdmin}, seed{totp: true})
	cD := newClient(env)
	// The earlier parts already counted failures against 192.0.2.1; this part needs its own IP.
	cD.remote = "192.0.2.50:1234"
	recD := cD.post("/api/v1/admin/auth/session", map[string]string{
		"username": "totp.throttle",
		"password": testPassword,
	})
	if recD.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", recD.Code, http.StatusOK)
	}
	for i := 1; i <= 4; i++ {
		recFail := cD.post("/api/v1/admin/auth/session/totp", map[string]string{
			"code": "000000",
		})
		if recFail.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want %d", i, recFail.Code, http.StatusUnauthorized)
		}
		assertJSONEqual(t, recFail.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)
	}
	rec5 := cD.post("/api/v1/admin/auth/session/totp", map[string]string{
		"code": "000000",
	})
	if rec5.Code != http.StatusTooManyRequests {
		t.Fatalf("5th attempt status = %d, want %d", rec5.Code, http.StatusTooManyRequests)
	}
	assertJSONEqual(t, rec5.Body.Bytes(), `{"title":"Too Many Requests","status":429}`)
	if retryAfter := rec5.Header().Get("Retry-After"); retryAfter != "900" {
		t.Fatalf("Retry-After = %q, want %q", retryAfter, "900")
	}
}

// A16, RF4: A recovery code works once; its reuse gives 401. Normalization handles spaces, uppercase, and 0->o, 1->l.
func TestRecoveryCodeSingleUse(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("sari.ppid", []authz.Role{authz.PpidOfficer}, seed{totp: false, mustChange: false})

	// Enroll through the API to get recovery codes.
	c := newClient(env)
	rec := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "sari.ppid",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}
	rec = c.post("/api/v1/admin/auth/totp", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("start enrollment status = %d, want %d", rec.Code, http.StatusOK)
	}
	enroll := decode[api.TotpEnrollment](t, rec)
	env.secrets["sari.ppid"] = enroll.Secret

	code := env.code("sari.ppid", env.clock.Now())
	rec = c.put("/api/v1/admin/auth/totp", map[string]string{
		"code": code,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm enrollment status = %d, want %d", rec.Code, http.StatusOK)
	}
	recoveryCodes := decode[api.RecoveryCodes](t, rec)
	if len(recoveryCodes.Codes) != 10 {
		t.Fatalf("got %d recovery codes, want 10", len(recoveryCodes.Codes))
	}
	firstCode := recoveryCodes.Codes[0]

	// RF4: submit the first recovery code uppercased, with a space instead of '-',
	// and with 0->o / 1->l substitutions where the code contains those digits.
	transformed := strings.ReplaceAll(firstCode, "-", " ")
	transformed = strings.ToUpper(transformed)
	transformed = strings.ReplaceAll(transformed, "0", "o")
	transformed = strings.ReplaceAll(transformed, "1", "l")

	// Log in again on a new client: user now has TOTP, so stage is totp_required.
	c2 := newClient(env)
	rec = c2.post("/api/v1/admin/auth/session", map[string]string{
		"username": "sari.ppid",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}
	state := decode[api.SessionState](t, rec)
	if state.Stage != api.TotpRequired {
		t.Fatalf("stage = %q, want %q", state.Stage, api.TotpRequired)
	}

	// Verify using the transformed recoveryCode -> authenticated.
	rec = c2.post("/api/v1/admin/auth/session/totp", map[string]string{
		"recoveryCode": transformed,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify recoveryCode status = %d, want %d", rec.Code, http.StatusOK)
	}
	state = decode[api.SessionState](t, rec)
	if state.Stage != api.Authenticated {
		t.Fatalf("stage = %q, want %q", state.Stage, api.Authenticated)
	}

	// In a later login, reuse of the same recovery code gives 401.
	c3 := newClient(env)
	rec = c3.post("/api/v1/admin/auth/session", map[string]string{
		"username": "sari.ppid",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}
	rec = c3.post("/api/v1/admin/auth/session/totp", map[string]string{
		"recoveryCode": firstCode,
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("reused recoveryCode status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)

	// Using the transformed string also gives 401.
	rec = c3.post("/api/v1/admin/auth/session/totp", map[string]string{
		"recoveryCode": transformed,
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("reused transformed recoveryCode status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)
}

// A17: Enrollment returns secret, URI and PNG. Confirming returns 10 codes in xxxxx-xxxxx format. users.totp_secret starts with 0x01 and is not the base32 secret. It opens with the right AAD and fails with another user's AAD
func TestTotpEnrollment(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	id := env.seedUser("sari.ppid", []authz.Role{authz.PpidOfficer}, seed{totp: false, mustChange: false})

	c := newClient(env)
	rec := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "sari.ppid",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}

	// POST totp -> secret is base32, otpauthUri starts with otpauth://totp/, qrCodePng starts with \x89PNG
	rec = c.post("/api/v1/admin/auth/totp", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("start enrollment status = %d, want %d", rec.Code, http.StatusOK)
	}
	enroll := decode[api.TotpEnrollment](t, rec)

	secretClean := strings.TrimRight(enroll.Secret, "=")
	if _, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secretClean); err != nil {
		if _, err = base32.StdEncoding.DecodeString(enroll.Secret); err != nil {
			t.Fatalf("secret %q is not valid base32: %v", enroll.Secret, err)
		}
	}
	if !strings.HasPrefix(enroll.OtpauthUri, "otpauth://totp/") {
		t.Fatalf("otpauthUri = %q, want prefix %q", enroll.OtpauthUri, "otpauth://totp/")
	}
	if !bytes.HasPrefix(enroll.QrCodePng, []byte("\x89PNG")) {
		t.Fatalf("qrCodePng does not start with PNG header")
	}

	// PUT totp with valid code -> 10 codes matching ^[0-9a-hjkmnp-tv-z]{5}-[0-9a-hjkmnp-tv-z]{5}$
	env.secrets["sari.ppid"] = enroll.Secret
	code := env.code("sari.ppid", env.clock.Now())
	rec = c.put("/api/v1/admin/auth/totp", map[string]string{
		"code": code,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm enrollment status = %d, want %d", rec.Code, http.StatusOK)
	}
	rc := decode[api.RecoveryCodes](t, rec)
	if len(rc.Codes) != 10 {
		t.Fatalf("got %d recovery codes, want 10", len(rc.Codes))
	}
	re := regexp.MustCompile(`^[0-9a-hjkmnp-tv-z]{5}-[0-9a-hjkmnp-tv-z]{5}$`)
	for _, rcCode := range rc.Codes {
		if !re.MatchString(rcCode) {
			t.Fatalf("recovery code %q does not match required format", rcCode)
		}
	}

	// DB totp_secret[0] == 0x01 and is not equal to []byte(secret).
	// keys.Open(ct, "users.totp_secret:<id>") == secret; Open with another id's AAD errors.
	var ct []byte
	err := env.db.Super.QueryRow(context.Background(), "SELECT totp_secret FROM users WHERE id = $1", id).Scan(&ct)
	if err != nil {
		t.Fatalf("query totp_secret: %v", err)
	}
	if len(ct) == 0 {
		t.Fatalf("totp_secret is empty")
	}
	if ct[0] != 0x01 {
		t.Fatalf("totp_secret[0] = 0x%02x, want 0x01", ct[0])
	}
	if bytes.Equal(ct, []byte(enroll.Secret)) {
		t.Fatalf("totp_secret in database must be ciphertext, not plaintext")
	}

	aad := []byte(fmt.Sprintf("users.totp_secret:%d", id))
	pt, err := env.keys.Open(ct, aad)
	if err != nil {
		t.Fatalf("keys.Open with valid AAD failed: %v", err)
	}
	if string(pt) != enroll.Secret {
		t.Fatalf("keys.Open decrypted %q, want %q", string(pt), enroll.Secret)
	}

	wrongAad := []byte(fmt.Sprintf("users.totp_secret:%d", id+999))
	if _, err := env.keys.Open(ct, wrongAad); err == nil {
		t.Fatalf("expected keys.Open with wrong AAD to error, got nil")
	}
}

// A18: createRecoveryCodes invalidates the old codes
func TestRecoveryCodesRegenerate(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("sari.ppid", []authz.Role{authz.PpidOfficer}, seed{totp: false, mustChange: false})

	// Enroll via API to get initial recovery codes.
	c := newClient(env)
	rec := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "sari.ppid",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}
	rec = c.post("/api/v1/admin/auth/totp", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("start enrollment status = %d, want %d", rec.Code, http.StatusOK)
	}
	enroll := decode[api.TotpEnrollment](t, rec)
	env.secrets["sari.ppid"] = enroll.Secret

	code := env.code("sari.ppid", env.clock.Now())
	rec = c.put("/api/v1/admin/auth/totp", map[string]string{
		"code": code,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm enrollment status = %d, want %d", rec.Code, http.StatusOK)
	}
	oldRC := decode[api.RecoveryCodes](t, rec)
	if len(oldRC.Codes) != 10 {
		t.Fatalf("got %d old recovery codes, want 10", len(oldRC.Codes))
	}

	// Step-up, then POST /totp/recovery-codes -> 200 with 10 new codes.
	rec = stepUp(c, "sari.ppid")
	if rec.Code != http.StatusOK {
		t.Fatalf("stepUp status = %d, want %d", rec.Code, http.StatusOK)
	}
	rec = c.post("/api/v1/admin/auth/totp/recovery-codes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("regenerate recovery codes status = %d, want %d", rec.Code, http.StatusOK)
	}
	newRC := decode[api.RecoveryCodes](t, rec)
	if len(newRC.Codes) != 10 {
		t.Fatalf("got %d new recovery codes, want 10", len(newRC.Codes))
	}

	// An old code at the next login gives 401.
	c2 := newClient(env)
	rec = c2.post("/api/v1/admin/auth/session", map[string]string{
		"username": "sari.ppid",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}
	rec = c2.post("/api/v1/admin/auth/session/totp", map[string]string{
		"recoveryCode": oldRC.Codes[0],
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old recovery code status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)

	// A new code at the next login gives 200.
	c3 := newClient(env)
	rec = c3.post("/api/v1/admin/auth/session", map[string]string{
		"username": "sari.ppid",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}
	rec = c3.post("/api/v1/admin/auth/session/totp", map[string]string{
		"recoveryCode": newRC.Codes[0],
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("new recovery code status = %d, want %d", rec.Code, http.StatusOK)
	}
	state := decode[api.SessionState](t, rec)
	if state.Stage != api.Authenticated {
		t.Fatalf("stage = %q, want %q", state.Stage, api.Authenticated)
	}
}

// A39: createTotpEnrollment after confirmation gives 409; updateTotpEnrollment with a wrong code gives 400, with no pending enrollment gives 409; a repeated createTotpEnrollment before confirmation replaces the pending secret
func TestTotpEnrollmentConflicts(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("user.conflicts", []authz.Role{authz.PpidOfficer}, seed{totp: false, mustChange: false})

	c := newClient(env)
	rec := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "user.conflicts",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}

	// PUT totp without pending enrollment -> 409
	rec = c.put("/api/v1/admin/auth/totp", map[string]string{
		"code": "123456",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("PUT without pending status = %d, want %d", rec.Code, http.StatusConflict)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Conflict","status":409}`)

	// First POST totp -> 200
	rec = c.post("/api/v1/admin/auth/totp", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("first POST totp status = %d, want %d", rec.Code, http.StatusOK)
	}
	enroll1 := decode[api.TotpEnrollment](t, rec)

	// Second POST totp before confirm replaces the secret -> 200
	rec = c.post("/api/v1/admin/auth/totp", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("second POST totp status = %d, want %d", rec.Code, http.StatusOK)
	}
	enroll2 := decode[api.TotpEnrollment](t, rec)
	if enroll1.Secret == enroll2.Secret {
		t.Fatalf("expected different secret on repeated enrollment")
	}

	// Code from the first secret -> 400 "Kode tidak valid."
	code1, err := totp.GenerateCodeCustom(enroll1.Secret, env.clock.Now(), totp.ValidateOpts{
		Period:    30,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("GenerateCodeCustom: %v", err)
	}
	rec = c.put("/api/v1/admin/auth/totp", map[string]string{
		"code": code1,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT with old secret code status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Bad Request","status":400,"detail":"Kode tidak valid."}`)

	// PUT with a wrong code -> 400 "Kode tidak valid."
	wrongCode := "000000"
	code2, err := totp.GenerateCodeCustom(enroll2.Secret, env.clock.Now(), totp.ValidateOpts{
		Period:    30,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("GenerateCodeCustom: %v", err)
	}
	if wrongCode == code2 {
		wrongCode = "999999"
	}
	rec = c.put("/api/v1/admin/auth/totp", map[string]string{
		"code": wrongCode,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT with wrong code status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Bad Request","status":400,"detail":"Kode tidak valid."}`)

	// Confirm with the second secret's code -> 200
	rec = c.put("/api/v1/admin/auth/totp", map[string]string{
		"code": code2,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm with second secret status = %d, want %d", rec.Code, http.StatusOK)
	}

	// POST totp after confirm -> 409
	rec = c.post("/api/v1/admin/auth/totp", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("POST totp after confirm status = %d, want %d", rec.Code, http.StatusConflict)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Conflict","status":409}`)
}

// A27: createStepUp for a user without TOTP gives 403
func TestStepUpWithoutTotp(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("humas.editor", []authz.Role{authz.HumasEditor}, seed{totp: false, mustChange: false})

	c := newClient(env)
	rec := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "humas.editor",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}
	state := decode[api.SessionState](t, rec)
	if state.Stage != api.Authenticated {
		t.Fatalf("stage = %q, want %q", state.Stage, api.Authenticated)
	}

	rec = c.post("/api/v1/admin/auth/session/step-up", map[string]string{
		"code": "123456",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("stepUp status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Forbidden","status":403,"detail":"Akses ditolak."}`)
}

// A40: A wrong step-up code gives 401 and counts toward the throttle; a correct code used at login cannot be reused for step-up in the same 30 s step; non-6-digit code gives 400
func TestStepUpCodeChecks(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("stepup.checks", []authz.Role{authz.SuperAdmin}, seed{totp: true, mustChange: false})

	c := newClient(env)
	rec := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "stepup.checks",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}

	loginCode := env.code("stepup.checks", env.clock.Now())
	rec = c.post("/api/v1/admin/auth/session/totp", map[string]string{
		"code": loginCode,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("totp verify status = %d, want %d", rec.Code, http.StatusOK)
	}

	// 1. code: "12" -> 400 (not counted toward throttle)
	rec = c.post("/api/v1/admin/auth/session/step-up", map[string]string{
		"code": "12",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short code status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Bad Request","status":400}`)
	failures := queryInt(t, env.db.Super, "SELECT coalesce((SELECT failures FROM login_throttle WHERE scope='account' AND key='stepup.checks'), 0)")
	if failures != 0 {
		t.Fatalf("expected 0 throttle failures after 400, got %d", failures)
	}

	// 2. The login code reused immediately (in same 30s step) -> 401
	rec = c.post("/api/v1/admin/auth/session/step-up", map[string]string{
		"code": loginCode,
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("reused login code status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)
	failures = queryInt(t, env.db.Super, "SELECT failures FROM login_throttle WHERE scope='account' AND key='stepup.checks'")
	if failures != 1 {
		t.Fatalf("expected account throttle failures = 1 after the reused code, got %d", failures)
	}

	// 3. Wrong code -> 401 and counted (the account row failures 2: the reused code above counted too)
	env.clock.Advance(30 * time.Second)
	wrongCode := "000000"
	if wrongCode == env.code("stepup.checks", env.clock.Now()) {
		wrongCode = "999999"
	}
	rec = c.post("/api/v1/admin/auth/session/step-up", map[string]string{
		"code": wrongCode,
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong step-up code status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)
	failures = queryInt(t, env.db.Super, "SELECT failures FROM login_throttle WHERE scope='account' AND key='stepup.checks'")
	if failures != 2 {
		t.Fatalf("expected account throttle failures = 2, got %d", failures)
	}
}

// A26: createPasswordReset, createRecoveryCodes and deleteUserTotp without step-up give 403 with the step-up detail. After createStepUp, createPasswordReset gives 200 and the target's old session gives 401 (epoch). After 10 min it gives 403 again
func TestStepUpGate(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	saID := env.seedUser("sa.gate", []authz.Role{authz.SuperAdmin}, seed{totp: true, mustChange: false})
	_ = saID
	targetID := env.seedUser("target.gate", []authz.Role{authz.HumasEditor}, seed{totp: false, mustChange: false})

	// Target logs in
	cTarget := newClient(env)
	rec := cTarget.post("/api/v1/admin/auth/session", map[string]string{
		"username": "target.gate",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("target login status = %d, want %d", rec.Code, http.StatusOK)
	}
	rec = cTarget.get("/api/v1/admin/auth/session")
	if rec.Code != http.StatusOK {
		t.Fatalf("target get session status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Super admin logs in
	cSA := newClient(env)
	rec = login(cSA, "sa.gate")
	if rec.Code != http.StatusOK {
		t.Fatalf("SA login status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Without step-up:
	// 1. createPasswordReset -> 403
	rec = cSA.post(fmt.Sprintf("/api/v1/admin/auth/users/%d/password-reset", targetID), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("createPasswordReset without step-up status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Forbidden","status":403,"detail":"Verifikasi ulang dengan kode autentikator diperlukan."}`)

	// 2. createRecoveryCodes -> 403
	rec = cSA.post("/api/v1/admin/auth/totp/recovery-codes", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("createRecoveryCodes without step-up status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Forbidden","status":403,"detail":"Verifikasi ulang dengan kode autentikator diperlukan."}`)

	// 3. deleteUserTotp -> 403
	rec = cSA.delete(fmt.Sprintf("/api/v1/admin/auth/users/%d/totp", targetID))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("deleteUserTotp without step-up status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Forbidden","status":403,"detail":"Verifikasi ulang dengan kode autentikator diperlukan."}`)

	// Open step-up window
	rec = stepUp(cSA, "sa.gate")
	if rec.Code != http.StatusOK {
		t.Fatalf("stepUp status = %d, want %d", rec.Code, http.StatusOK)
	}

	// After step-up: createPasswordReset -> 200
	rec = cSA.post(fmt.Sprintf("/api/v1/admin/auth/users/%d/password-reset", targetID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("createPasswordReset with step-up status = %d, want %d", rec.Code, http.StatusOK)
	}
	tmp := decode[api.TemporaryPassword](t, rec)
	if tmp.TemporaryPassword == "" {
		t.Fatalf("expected non-empty temporaryPassword")
	}

	// Target's prior session gives 401 (session_epoch incremented)
	rec = cTarget.get("/api/v1/admin/auth/session")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("target prior session status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)

	// Advance clock past step-up TTL (10m + 1s) -> 403 again
	env.clock.Advance(10*time.Minute + time.Second)
	rec = cSA.post("/api/v1/admin/auth/totp/recovery-codes", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("createRecoveryCodes after step-up expired status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Forbidden","status":403,"detail":"Verifikasi ulang dengan kode autentikator diperlukan."}`)
}
