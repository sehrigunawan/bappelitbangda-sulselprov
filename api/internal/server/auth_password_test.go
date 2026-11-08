package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
)

// A19: Password policy rejects 11 characters, 129 characters, a listed common password and a password containing the username (400). It accepts a 12-character Unicode password
func TestPasswordPolicy(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("andi.admin", []authz.Role{authz.SuperAdmin}, seed{totp: false, mustChange: true})

	c := newClient(env)
	rec := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "andi.admin",
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}
	state := decode[api.SessionState](t, rec)
	if state.Stage != api.PasswordChangeRequired {
		t.Fatalf("stage = %q, want %q", state.Stage, api.PasswordChangeRequired)
	}

	// 1. 11 characters -> 400 "Minimal 12 karakter."
	rec = c.put("/api/v1/admin/auth/password", map[string]string{
		"currentPassword": testPassword,
		"newPassword":     "short-pass!",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("11 chars status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Bad Request","status":400,"detail":"Minimal 12 karakter."}`)

	// 2. 129 characters -> 400 "Maksimal 128 karakter."
	rec = c.put("/api/v1/admin/auth/password", map[string]string{
		"currentPassword": testPassword,
		"newPassword":     strings.Repeat("x", 129),
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("129 chars status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Bad Request","status":400,"detail":"Maksimal 128 karakter."}`)

	// 3. Listed common password q1w2e3r4t5y6 -> 400 "Kata sandi terlalu umum."
	rec = c.put("/api/v1/admin/auth/password", map[string]string{
		"currentPassword": testPassword,
		"newPassword":     "q1w2e3r4t5y6",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("common password status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Bad Request","status":400,"detail":"Kata sandi terlalu umum."}`)

	// 4. Equal to or containing the username xx-andi.admin-xx -> 400 "Kata sandi tidak boleh memuat nama pengguna."
	rec = c.put("/api/v1/admin/auth/password", map[string]string{ //nolint:gosec // synthetic test value
		"currentPassword": testPassword,
		"newPassword":     "xx-andi.admin-xx",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("contains username status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Bad Request","status":400,"detail":"Kata sandi tidak boleh memuat nama pengguna."}`)

	// 5. 12-character Unicode password ŝekretoj-ĉiu (12 code points) -> 204
	rec = c.put("/api/v1/admin/auth/password", map[string]string{ //nolint:gosec // synthetic test value
		"currentPassword": testPassword,
		"newPassword":     "ŝekretoj-ĉiu",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("12 code points Unicode password status = %d, want %d, body = %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

// A38: updatePassword: a wrong current password gives 400 Kata sandi saat ini salah. and counts toward the throttle; success ends the user's other sessions, keeps this one with a renewed token and the new epoch, and re-resolves the stage
func TestUpdatePassword(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	id := env.seedUser("pw.update", []authz.Role{authz.HumasEditor}, seed{totp: false, mustChange: false})

	// Client A logs in
	cA := newClient(env)
	recA := cA.post("/api/v1/admin/auth/session", map[string]string{
		"username": "pw.update",
		"password": testPassword,
	})
	if recA.Code != http.StatusOK {
		t.Fatalf("cA login status = %d, want %d", recA.Code, http.StatusOK)
	}
	tokenA := cA.cookie.Value

	// Client B logs in (concurrent session)
	cB := newClient(env)
	recB := cB.post("/api/v1/admin/auth/session", map[string]string{
		"username": "pw.update",
		"password": testPassword,
	})
	if recB.Code != http.StatusOK {
		t.Fatalf("cB login status = %d, want %d", recB.Code, http.StatusOK)
	}

	// Wrong current password -> 400 "Kata sandi saat ini salah.", failures 1 in account throttle
	recWrong := cA.put("/api/v1/admin/auth/password", map[string]string{
		"currentPassword": "wrong-password-here",
		"newPassword":     "new-strong-password-01",
	})
	if recWrong.Code != http.StatusBadRequest {
		t.Fatalf("wrong current password status = %d, want %d", recWrong.Code, http.StatusBadRequest)
	}
	assertJSONEqual(t, recWrong.Body.Bytes(), `{"title":"Bad Request","status":400,"detail":"Kata sandi saat ini salah."}`)

	failures := queryInt(t, env.db.Super, "SELECT failures FROM login_throttle WHERE scope='account' AND key='pw.update'")
	if failures != 1 {
		t.Fatalf("expected account throttle failures = 1, got %d", failures)
	}

	epochBefore := queryInt(t, env.db.Super, "SELECT session_epoch FROM users WHERE id = $1", id)

	// Successful password update -> 204
	recSuccess := cA.put("/api/v1/admin/auth/password", map[string]string{
		"currentPassword": testPassword,
		"newPassword":     "new-strong-password-01",
	})
	if recSuccess.Code != http.StatusNoContent {
		t.Fatalf("password update status = %d, want %d, body = %s", recSuccess.Code, http.StatusNoContent, recSuccess.Body.String())
	}

	// session_epoch incremented by 1 in database
	epochAfter := queryInt(t, env.db.Super, "SELECT session_epoch FROM users WHERE id = $1", id)
	if epochAfter != epochBefore+1 {
		t.Fatalf("session_epoch = %d, want %d", epochAfter, epochBefore+1)
	}

	// Client A's session token is renewed, and client A can still access session (200)
	if cA.cookie == nil || cA.cookie.Value == tokenA {
		t.Fatalf("expected renewed session token on client A")
	}
	recA2 := cA.get("/api/v1/admin/auth/session")
	if recA2.Code != http.StatusOK {
		t.Fatalf("client A session status = %d, want %d", recA2.Code, http.StatusOK)
	}

	// Client B's session is now 401 (invalidated by epoch change)
	recB2 := cB.get("/api/v1/admin/auth/session")
	if recB2.Code != http.StatusUnauthorized {
		t.Fatalf("client B prior session status = %d, want %d", recB2.Code, http.StatusUnauthorized)
	}
	assertJSONEqual(t, recB2.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)
}
