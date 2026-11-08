package server_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/server"
)

// A4: listUsers: anonymous gives 401, humas_editor gives 403, super_admin gives 200
func TestListUsersAuthorization(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	// 1. Anonymous request gives 401 with standard Unauthorized body.
	cAnon := newClient(env)
	rec := cAnon.get("/api/v1/admin/auth/users")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous ListUsers status = %d, want 401", rec.Code)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)

	// 2. Authenticated humas_editor gives 403 Akses ditolak.
	env.seedUser("andi.humas", []authz.Role{authz.HumasEditor}, seed{})
	cHumas := newClient(env)
	if recLogin := login(cHumas, "andi.humas"); recLogin.Code != http.StatusOK {
		t.Fatalf("login andi.humas status = %d, want 200", recLogin.Code)
	}
	rec = cHumas.get("/api/v1/admin/auth/users")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("humas_editor ListUsers status = %d, want 403", rec.Code)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Forbidden","status":403,"detail":"Akses ditolak."}`)

	// 3. Authenticated super_admin (with TOTP verified) gives 200.
	env.seedUser("siti.admin", []authz.Role{authz.SuperAdmin}, seed{totp: true})
	cAdmin := newClient(env)
	if recLogin := login(cAdmin, "siti.admin"); recLogin.Code != http.StatusOK {
		t.Fatalf("login siti.admin status = %d, want 200", recLogin.Code)
	}
	rec = cAdmin.get("/api/v1/admin/auth/users")
	if rec.Code != http.StatusOK {
		t.Fatalf("super_admin ListUsers status = %d, want 200", rec.Code)
	}
}

// A5: an operation absent from operationRules gives 403 (the test injects a rule map)
func TestUnlistedOperationForbidden(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	env.seedUser("siti.admin", []authz.Role{authz.SuperAdmin}, seed{totp: true})
	c := newClient(env)
	if recLogin := login(c, "siti.admin"); recLogin.Code != http.StatusOK {
		t.Fatalf("login siti.admin status = %d, want 200", recLogin.Code)
	}

	logger := slog.New(slog.DiscardHandler)
	deps := server.Deps{
		Pool:    env.db.App,
		Keyring: env.keys,
		Config:  env.cfg,
		Now:     env.clock.Now,
	}
	hWithout := server.NewHandlerWithoutRule(logger, deps, "ListUsers")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/users", nil)
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	hWithout.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Forbidden","status":403,"detail":"Akses ditolak."}`)
}

// A37: wrong stage: no session gives 401; a pending session on a permission operation gives 401;
// an authenticated session on createTotpVerification, and password_change_required on createTotpEnrollment, give 403
func TestStageRules(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	// 1. Anonymous GET session gives 401.
	cAnon := newClient(env)
	rec := cAnon.get("/api/v1/admin/auth/session")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous GET session status = %d, want 401", rec.Code)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)

	// 2. Pending totp_required session on ListUsers gives 401.
	env.seedUser("totp.pending", []authz.Role{authz.SuperAdmin}, seed{totp: true})
	cTotp := newClient(env)
	recPost := cTotp.post("/api/v1/admin/auth/session", map[string]string{
		"username": "totp.pending",
		"password": testPassword,
	})
	if recPost.Code != http.StatusOK {
		t.Fatalf("login credentials status = %d, want 200", recPost.Code)
	}
	state := decode[api.SessionState](t, recPost)
	if state.Stage != api.TotpRequired {
		t.Fatalf("stage = %q, want %q", state.Stage, api.TotpRequired)
	}
	rec = cTotp.get("/api/v1/admin/auth/users")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("totp_required on ListUsers status = %d, want 401", rec.Code)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)

	// 3. Authenticated session on POST /session/totp gives 403.
	env.seedUser("auth.user", []authz.Role{authz.HumasEditor}, seed{})
	cAuth := newClient(env)
	if recLogin := login(cAuth, "auth.user"); recLogin.Code != http.StatusOK {
		t.Fatalf("login auth.user status = %d, want 200", recLogin.Code)
	}
	rec = cAuth.post("/api/v1/admin/auth/session/totp", map[string]string{
		"code": "123456",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("authenticated POST /session/totp status = %d, want 403", rec.Code)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Forbidden","status":403,"detail":"Akses ditolak."}`)

	// 4. password_change_required session on POST /totp gives 403.
	env.seedUser("pw.pending", []authz.Role{authz.PpidOfficer}, seed{mustChange: true})
	cPw := newClient(env)
	recPw := cPw.post("/api/v1/admin/auth/session", map[string]string{
		"username": "pw.pending",
		"password": testPassword,
	})
	if recPw.Code != http.StatusOK {
		t.Fatalf("login pw.pending status = %d, want 200", recPw.Code)
	}
	statePw := decode[api.SessionState](t, recPw)
	if statePw.Stage != api.PasswordChangeRequired {
		t.Fatalf("stage = %q, want %q", statePw.Stage, api.PasswordChangeRequired)
	}
	rec = cPw.post("/api/v1/admin/auth/totp", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("password_change_required POST /totp status = %d, want 403", rec.Code)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Forbidden","status":403,"detail":"Akses ditolak."}`)
}

// A33: SessionState.permissions of a two-role user equals Effective
func TestSessionPermissionsUnion(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	env.seedUser("combo.user", []authz.Role{authz.HumasEditor, authz.DocumentOperator}, seed{})
	c := newClient(env)
	rec := login(c, "combo.user")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", rec.Code)
	}

	wantPerms := []api.Permission{
		{Area: api.Publications, Level: api.Manage},
		{Area: api.Pages, Level: api.Manage},
		{Area: api.Documents, Level: api.Manage},
		{Area: api.SiteContent, Level: api.Manage},
		{Area: api.Media, Level: api.Manage},
	}

	state := decode[api.SessionState](t, rec)
	if state.Stage != api.Authenticated {
		t.Fatalf("stage = %q, want %q", state.Stage, api.Authenticated)
	}
	if !reflect.DeepEqual(state.Permissions, wantPerms) {
		t.Fatalf("login session permissions = %+v, want %+v", state.Permissions, wantPerms)
	}

	recGet := c.get("/api/v1/admin/auth/session")
	if recGet.Code != http.StatusOK {
		t.Fatalf("get session status = %d, want 200", recGet.Code)
	}
	stateGet := decode[api.SessionState](t, recGet)
	if !reflect.DeepEqual(stateGet.Permissions, wantPerms) {
		t.Fatalf("get session permissions = %+v, want %+v", stateGet.Permissions, wantPerms)
	}
}
