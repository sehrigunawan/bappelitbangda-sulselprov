package server_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/server"
)

func loginQueryString(t testing.TB, env *testEnv, query string, args ...any) string {
	t.Helper()
	var s string
	if err := env.db.Super.QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		t.Fatalf("loginQueryString %q: %v", query, err)
	}
	return s
}

// A6 + RF5: Correct login for humas_editor without TOTP gives 200 authenticated.
// Set-Cookie is HttpOnly, SameSite=Strict, and Secure when configured.
// A second login with the same cookie gets a different token, and the first token gives 401.
// Logging in as Andi.admin for seeded andi.admin gives 200 (username case-insensitivity).
func TestLoginSetsSessionCookie(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("andi.admin", []authz.Role{authz.HumasEditor}, seed{})

	c := newClient(env)
	rec := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "andi.admin",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}

	state := decode[api.SessionState](t, rec)
	if state.Stage != api.Authenticated {
		t.Fatalf("stage = %q, want %q", state.Stage, api.Authenticated)
	}

	cookieName := sessionCookieName(env.cfg)
	var sessionCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == cookieName {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("cookie %q not found in response", cookieName)
	}
	if !sessionCookie.HttpOnly {
		t.Errorf("cookie HttpOnly = false, want true")
	}
	if sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie SameSite = %v, want SameSiteStrictMode", sessionCookie.SameSite)
	}
	if !sessionCookie.Secure {
		t.Errorf("cookie Secure = false, want true")
	}
	if sessionCookie.Path != "/" {
		t.Errorf("cookie Path = %q, want \"/\"", sessionCookie.Path)
	}

	firstToken := sessionCookie.Value

	// A second login sending the first cookie gets a different token
	rec2 := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "andi.admin",
		"password": testPassword,
	})
	if rec2.Code != http.StatusOK {
		t.Fatalf("second login status = %d, want %d", rec2.Code, http.StatusOK)
	}
	if c.cookie == nil || c.cookie.Value == firstToken {
		t.Fatalf("second login token = %v, expected different from first token %q", c.cookie, firstToken)
	}

	// The first token then gives 401
	oldClient := newClient(env)
	oldClient.cookie = &http.Cookie{ //nolint:gosec // request cookie in a test
		Name:  cookieName,
		Value: firstToken,
	}
	recOld := oldClient.get("/api/v1/admin/auth/session")
	if recOld.Code != http.StatusUnauthorized {
		t.Fatalf("first token after second login: got status %d, want %d", recOld.Code, http.StatusUnauthorized)
	}

	// Login as Andi.admin for seeded andi.admin gives 200, and verifies throttle key is lowercased (RF5)
	cCapital := newClient(env)
	recCapitalFail := cCapital.post("/api/v1/admin/auth/session", map[string]string{
		"username": "Andi.admin",
		"password": "wrong-password",
	})
	if recCapitalFail.Code != http.StatusUnauthorized {
		t.Fatalf("login as Andi.admin wrong pw status = %d, want %d", recCapitalFail.Code, http.StatusUnauthorized)
	}
	if count := queryInt(t, env, "SELECT count(*) FROM login_throttle WHERE scope = 'account' AND key = 'andi.admin'"); count != 1 {
		t.Fatalf("throttle account key 'andi.admin' count = %d, want 1", count)
	}
	if count := queryInt(t, env, "SELECT count(*) FROM login_throttle WHERE scope = 'account' AND key = 'Andi.admin'"); count != 0 {
		t.Fatalf("throttle account key 'Andi.admin' count = %d, want 0", count)
	}

	recCapital := cCapital.post("/api/v1/admin/auth/session", map[string]string{
		"username": "Andi.admin",
		"password": testPassword,
	})
	if recCapital.Code != http.StatusOK {
		t.Fatalf("login as Andi.admin status = %d, want %d", recCapital.Code, http.StatusOK)
	}
	stateCapital := decode[api.SessionState](t, recCapital)
	if stateCapital.Stage != api.Authenticated {
		t.Fatalf("stage = %q, want %q", stateCapital.Stage, api.Authenticated)
	}
	if count := queryInt(t, env, "SELECT count(*) FROM login_throttle WHERE scope = 'account' AND key = 'andi.admin'"); count != 0 {
		t.Fatalf("throttle account key 'andi.admin' after success count = %d, want 0", count)
	}
}

// A7: Wrong password and unknown user both give 401 with byte-identical bodies.
func TestLoginFailureBodiesIdentical(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("andi.editor", []authz.Role{authz.HumasEditor}, seed{})

	c1 := newClient(env)
	recWrongPw := c1.post("/api/v1/admin/auth/session", map[string]string{
		"username": "andi.editor",
		"password": "wrong-password-1234",
	})
	if recWrongPw.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want %d", recWrongPw.Code, http.StatusUnauthorized)
	}

	c2 := newClient(env)
	recUnknownUser := c2.post("/api/v1/admin/auth/session", map[string]string{
		"username": "unknown.user",
		"password": testPassword,
	})
	if recUnknownUser.Code != http.StatusUnauthorized {
		t.Fatalf("unknown user status = %d, want %d", recUnknownUser.Code, http.StatusUnauthorized)
	}

	wantBody := "{\"detail\":\"Nama pengguna atau kata sandi salah.\",\"status\":401,\"title\":\"Unauthorized\"}\n"
	if !bytes.Equal(recWrongPw.Body.Bytes(), recUnknownUser.Body.Bytes()) {
		t.Fatalf("bodies differ:\nwrong pw:     %q\nunknown user: %q", recWrongPw.Body.String(), recUnknownUser.Body.String())
	}
	if recWrongPw.Body.String() != wantBody {
		t.Fatalf("body = %q, want %q", recWrongPw.Body.String(), wantBody)
	}
}

// A12: A disabled user gets 401 at login; an existing session gets 401 on the next request.
func TestDisabledUser(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("disabled.user", []authz.Role{authz.HumasEditor}, seed{disabled: true})

	c := newClient(env)
	recDisabled := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "disabled.user",
		"password": testPassword,
	})
	if recDisabled.Code != http.StatusUnauthorized {
		t.Fatalf("disabled user status = %d, want %d", recDisabled.Code, http.StatusUnauthorized)
	}
	wantBody := "{\"detail\":\"Nama pengguna atau kata sandi salah.\",\"status\":401,\"title\":\"Unauthorized\"}\n"
	if recDisabled.Body.String() != wantBody {
		t.Fatalf("disabled body = %q, want %q", recDisabled.Body.String(), wantBody)
	}

	// Active user logs in, then is disabled in DB
	env.seedUser("active.user", []authz.Role{authz.HumasEditor}, seed{})
	cActive := newClient(env)
	recLogin := cActive.post("/api/v1/admin/auth/session", map[string]string{
		"username": "active.user",
		"password": testPassword,
	})
	if recLogin.Code != http.StatusOK {
		t.Fatalf("active login status = %d, want %d", recLogin.Code, http.StatusOK)
	}

	// Disable user in DB via Super
	exec(t, env, "UPDATE users SET disabled_at = now() WHERE username = $1", "active.user")

	recAfterDisable := cActive.get("/api/v1/admin/auth/session")
	if recAfterDisable.Code != http.StatusUnauthorized {
		t.Fatalf("session after disable status = %d, want %d", recAfterDisable.Code, http.StatusUnauthorized)
	}
	assertJSONEqual(t, recAfterDisable.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)
}

// A13 + RF2: Stage order and permissions:[] on pending stages.
// TOTP super_admin gives totp_required with "permissions":[];
// new ppid_officer with mustChange gives password_change_required,
// then PUT password gives GET session totp_enrollment_required;
// GET users while pending gives 401.
func TestLoginStageOrder(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	// Case 1: TOTP super_admin
	env.seedUser("totp.admin", []authz.Role{authz.SuperAdmin}, seed{totp: true})
	c1 := newClient(env)
	rec1 := c1.post("/api/v1/admin/auth/session", map[string]string{
		"username": "totp.admin",
		"password": testPassword,
	})
	if rec1.Code != http.StatusOK {
		t.Fatalf("totp.admin login status = %d, want %d", rec1.Code, http.StatusOK)
	}

	state1 := decode[api.SessionState](t, rec1)
	if state1.Stage != api.TotpRequired {
		t.Fatalf("stage = %q, want %q", state1.Stage, api.TotpRequired)
	}
	if state1.Permissions == nil || len(state1.Permissions) != 0 {
		t.Fatalf("permissions = %v, want non-nil empty slice", state1.Permissions)
	}
	if !strings.Contains(rec1.Body.String(), `"permissions":[]`) {
		t.Fatalf("response body does not contain literal \"permissions\":[]: %s", rec1.Body.String())
	}

	// GET users while pending totp_required gives 401
	recUsers1 := c1.get("/api/v1/admin/auth/users")
	if recUsers1.Code != http.StatusUnauthorized {
		t.Fatalf("GET users while pending status = %d, want %d", recUsers1.Code, http.StatusUnauthorized)
	}

	// Case 2: new ppid_officer with mustChange
	env.seedUser("new.ppid", []authz.Role{authz.PpidOfficer}, seed{mustChange: true})
	c2 := newClient(env)
	rec2 := c2.post("/api/v1/admin/auth/session", map[string]string{
		"username": "new.ppid",
		"password": testPassword,
	})
	if rec2.Code != http.StatusOK {
		t.Fatalf("new.ppid login status = %d, want %d", rec2.Code, http.StatusOK)
	}

	state2 := decode[api.SessionState](t, rec2)
	if state2.Stage != api.PasswordChangeRequired {
		t.Fatalf("stage = %q, want %q", state2.Stage, api.PasswordChangeRequired)
	}
	if state2.Permissions == nil || len(state2.Permissions) != 0 {
		t.Fatalf("permissions = %v, want non-nil empty slice", state2.Permissions)
	}
	if !strings.Contains(rec2.Body.String(), `"permissions":[]`) {
		t.Fatalf("response body does not contain literal \"permissions\":[]: %s", rec2.Body.String())
	}

	// GET users while pending password_change_required gives 401
	recUsers2 := c2.get("/api/v1/admin/auth/users")
	if recUsers2.Code != http.StatusUnauthorized {
		t.Fatalf("GET users while pending status = %d, want %d", recUsers2.Code, http.StatusUnauthorized)
	}

	// PUT password
	recPw := c2.put("/api/v1/admin/auth/password", map[string]string{
		"currentPassword": testPassword,
		"newPassword":     "password-baru-ppid-01",
	})
	if recPw.Code != http.StatusNoContent {
		t.Fatalf("PUT password status = %d, want %d", recPw.Code, http.StatusNoContent)
	}

	// GET session now gives totp_enrollment_required (ppid_officer without TOTP)
	recGet := c2.get("/api/v1/admin/auth/session")
	if recGet.Code != http.StatusOK {
		t.Fatalf("GET session status = %d, want %d", recGet.Code, http.StatusOK)
	}
	stateGet := decode[api.SessionState](t, recGet)
	if stateGet.Stage != api.TotpEnrollmentRequired {
		t.Fatalf("stage = %q, want %q", stateGet.Stage, api.TotpEnrollmentRequired)
	}
	if stateGet.Permissions == nil || len(stateGet.Permissions) != 0 {
		t.Fatalf("permissions = %v, want non-nil empty slice", stateGet.Permissions)
	}
	if !strings.Contains(recGet.Body.String(), `"permissions":[]`) {
		t.Fatalf("response body does not contain literal \"permissions\":[]: %s", recGet.Body.String())
	}
}

// A14: A pending stage older than 10 min gives 401.
func TestPendingStageExpires(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("totp.user", []authz.Role{authz.SuperAdmin}, seed{totp: true})

	c := newClient(env)
	recLogin := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "totp.user",
		"password": testPassword,
	})
	if recLogin.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", recLogin.Code, http.StatusOK)
	}

	// Advance clock beyond PendingTTL (10m)
	env.clock.Advance(10*time.Minute + time.Second)

	recGet := c.get("/api/v1/admin/auth/session")
	if recGet.Code != http.StatusUnauthorized {
		t.Fatalf("expired pending session status = %d, want %d", recGet.Code, http.StatusUnauthorized)
	}
	assertJSONEqual(t, recGet.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)
}

// A20: Stored password hash has prefix $argon2id$v=19$m=19456,t=2,p=1$.
// A user seeded with a t=1 hash logs in 200 and the stored hash is updated to t=2.
func TestPasswordHashParameters(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	// Build a valid argon2id hash with t=1
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	key := argon2.IDKey([]byte(testPassword), salt, 1, 19456, 1, 32)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Key := base64.RawStdEncoding.EncodeToString(key)
	phcT1 := fmt.Sprintf("$argon2id$v=19$m=19456,t=1,p=1$%s$%s", b64Salt, b64Key)

	env.seedUser("rehash.user", []authz.Role{authz.HumasEditor}, seed{})
	exec(t, env, "UPDATE users SET password_hash = $1 WHERE username = $2", phcT1, "rehash.user")

	c := newClient(env)
	recLogin := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "rehash.user",
		"password": testPassword,
	})
	if recLogin.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", recLogin.Code, http.StatusOK)
	}

	wantPrefix := "$argon2id$v=19$m=19456,t=2,p=1$"
	storedHash := loginQueryString(t, env, "SELECT password_hash FROM users WHERE username = $1", "rehash.user")
	if !strings.HasPrefix(storedHash, wantPrefix) {
		t.Fatalf("rehashed password_hash = %q, want prefix %q", storedHash, wantPrefix)
	}

	// users.password_hash after PUT password also has prefix $argon2id$v=19$m=19456,t=2,p=1$
	recPut := c.put("/api/v1/admin/auth/password", map[string]string{
		"currentPassword": testPassword,
		"newPassword":     "password-baru-panjang-01",
	})
	if recPut.Code != http.StatusNoContent {
		t.Fatalf("PUT password status = %d, want %d", recPut.Code, http.StatusNoContent)
	}

	newHash := loginQueryString(t, env, "SELECT password_hash FROM users WHERE username = $1", "rehash.user")
	if !strings.HasPrefix(newHash, wantPrefix) {
		t.Fatalf("new password_hash = %q, want prefix %q", newHash, wantPrefix)
	}
}

// A44: createSession with unparsable body gives 400.
// A username not matching the pattern gives the identical 401 and counts against the IP key only.
func TestLoginMalformedInput(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	c := newClient(env)

	// Body "{" gives 400
	recBadJSON := c.do(http.MethodPost, "/api/v1/admin/auth/session", "{")
	if recBadJSON.Code != http.StatusBadRequest {
		t.Fatalf("bad JSON status = %d, want %d", recBadJSON.Code, http.StatusBadRequest)
	}

	// Username "Bad User!" gives identical 401
	recBadUsername := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "Bad User!",
		"password": testPassword,
	})
	if recBadUsername.Code != http.StatusUnauthorized {
		t.Fatalf("bad username status = %d, want %d", recBadUsername.Code, http.StatusUnauthorized)
	}
	wantBody := "{\"detail\":\"Nama pengguna atau kata sandi salah.\",\"status\":401,\"title\":\"Unauthorized\"}\n"
	if recBadUsername.Body.String() != wantBody {
		t.Fatalf("bad username body = %q, want %q", recBadUsername.Body.String(), wantBody)
	}

	// login_throttle has ('ip', '192.0.2.1') failures 1 and no account row
	ipFailures := queryInt(t, env, "SELECT failures FROM login_throttle WHERE scope = 'ip' AND key = '192.0.2.1'")
	if ipFailures != 1 {
		t.Fatalf("ip failures = %d, want 1", ipFailures)
	}
	accountRows := queryInt(t, env, "SELECT count(*) FROM login_throttle WHERE scope = 'account'")
	if accountRows != 0 {
		t.Fatalf("account throttle rows = %d, want 0", accountRows)
	}
}

// A42: AUTH_COOKIE_SECURE=false yields cookie bpl_session without Secure;
// true yields __Host-bpl_session with Secure.
func TestCookieNameFollowsSecureFlag(t *testing.T) {
	t.Parallel()

	// Case 1: CookieSecure = false
	envInsecure := newEnv(t, func(cfg *server.Config) {
		cfg.CookieSecure = false
	})
	envInsecure.seedUser("andi.insecure", []authz.Role{authz.HumasEditor}, seed{})
	cInsecure := newClient(envInsecure)
	recInsecure := cInsecure.post("/api/v1/admin/auth/session", map[string]string{
		"username": "andi.insecure",
		"password": testPassword,
	})
	if recInsecure.Code != http.StatusOK {
		t.Fatalf("insecure login status = %d, want %d", recInsecure.Code, http.StatusOK)
	}

	var foundInsecure, foundHostInsecure bool
	for _, cookie := range recInsecure.Result().Cookies() {
		if cookie.Name == "bpl_session" {
			foundInsecure = true
			if cookie.Secure {
				t.Errorf("bpl_session cookie Secure = true, want false")
			}
		}
		if cookie.Name == "__Host-bpl_session" {
			foundHostInsecure = true
		}
	}
	if !foundInsecure {
		t.Errorf("cookie bpl_session not found when CookieSecure=false")
	}
	if foundHostInsecure {
		t.Errorf("__Host-bpl_session found when CookieSecure=false, want only bpl_session")
	}

	// Case 2: CookieSecure = true
	envSecure := newEnv(t, func(cfg *server.Config) {
		cfg.CookieSecure = true
	})
	envSecure.seedUser("andi.secure", []authz.Role{authz.HumasEditor}, seed{})
	cSecure := newClient(envSecure)
	recSecure := cSecure.post("/api/v1/admin/auth/session", map[string]string{
		"username": "andi.secure",
		"password": testPassword,
	})
	if recSecure.Code != http.StatusOK {
		t.Fatalf("secure login status = %d, want %d", recSecure.Code, http.StatusOK)
	}

	var foundSecure, foundPlainSecure bool
	for _, cookie := range recSecure.Result().Cookies() {
		if cookie.Name == "__Host-bpl_session" {
			foundSecure = true
			if !cookie.Secure {
				t.Errorf("__Host-bpl_session cookie Secure = false, want true")
			}
		}
		if cookie.Name == "bpl_session" {
			foundPlainSecure = true
		}
	}
	if !foundSecure {
		t.Errorf("cookie __Host-bpl_session not found when CookieSecure=true")
	}
	if foundPlainSecure {
		t.Errorf("bpl_session found when CookieSecure=true, want only __Host-bpl_session")
	}
}
