package server_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/server"
)

type authSessionQueryCounter struct {
	count atomic.Int64
}

func (q *authSessionQueryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	q.count.Add(1)
	return ctx
}

func (q *authSessionQueryCounter) TraceQueryEnd(_ context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
}

// A22, A48: session idle timeout, absolute lifetime, and step-up keeping deadline
func TestSessionTimeouts(t *testing.T) {
	t.Parallel()
	env := newEnv(t, func(c *server.Config) {
		c.SessionIdleTimeout = 1 * time.Second
		c.SessionLifetime = 2 * time.Second
	})

	// Part 1 (A22): Idle timeout. 1s idle timeout, sleep 1.5s -> 401.
	env.seedUser("idle.user", []authz.Role{authz.HumasEditor}, seed{})
	cIdle := newClient(env)
	if rec := login(cIdle, "idle.user"); rec.Code != http.StatusOK {
		t.Fatalf("login idle.user status = %d, want 200", rec.Code)
	}
	time.Sleep(1500 * time.Millisecond)
	recIdle := cIdle.get("/api/v1/admin/auth/session")
	if recIdle.Code != http.StatusUnauthorized {
		t.Fatalf("idle timeout status = %d, want 401", recIdle.Code)
	}
	assertJSONEqual(t, recIdle.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)

	// Part 2 (A22): Absolute lifetime with continuous activity every 0.5s -> at 2.5s gives 401.
	env.seedUser("life.user", []authz.Role{authz.HumasEditor}, seed{})
	cLife := newClient(env)
	if rec := login(cLife, "life.user"); rec.Code != http.StatusOK {
		t.Fatalf("login life.user status = %d, want 200", rec.Code)
	}
	startLife := time.Now()
	for time.Since(startLife) < 2000*time.Millisecond {
		time.Sleep(500 * time.Millisecond)
		if time.Since(startLife) < 2000*time.Millisecond {
			if rec := cLife.get("/api/v1/admin/auth/session"); rec.Code != http.StatusOK {
				t.Fatalf("life intermediate status at %v = %d, want 200", time.Since(startLife), rec.Code)
			}
		}
	}
	if remain := 2500*time.Millisecond - time.Since(startLife); remain > 0 {
		time.Sleep(remain)
	}
	recLife := cLife.get("/api/v1/admin/auth/session")
	if recLife.Code != http.StatusUnauthorized {
		t.Fatalf("lifetime status = %d, want 401", recLife.Code)
	}
	assertJSONEqual(t, recLife.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)

	// Part 3 (A48): Step-up does not extend absolute lifetime.
	// A TOTP user with lifetime 2s logs in, steps up at ~1s, and GET at 2.5s gives 401.
	// The idle timeout of 1s above would end this session at the 1s step-up, so it uses its own handler.
	envStep := newEnv(t, func(c *server.Config) { c.SessionLifetime = 2 * time.Second })
	envStep.seedUser("step.user", []authz.Role{authz.SuperAdmin}, seed{totp: true})
	cStep := newClient(envStep)
	if rec := login(cStep, "step.user"); rec.Code != http.StatusOK {
		t.Fatalf("login step.user status = %d, want 200", rec.Code)
	}
	startStep := time.Now()
	time.Sleep(1000 * time.Millisecond)
	if rec := stepUp(cStep, "step.user"); rec.Code != http.StatusOK {
		t.Fatalf("stepUp status = %d, want 200", rec.Code)
	}
	time.Sleep(500 * time.Millisecond)
	if rec := cStep.get("/api/v1/admin/auth/session"); rec.Code != http.StatusOK {
		t.Fatalf("intermediate step get status = %d, want 200", rec.Code)
	}
	if remain := 2500*time.Millisecond - time.Since(startStep); remain > 0 {
		time.Sleep(remain)
	}
	recStepAfter := cStep.get("/api/v1/admin/auth/session")
	if recStepAfter.Code != http.StatusUnauthorized {
		t.Fatalf("step-up absolute lifetime status = %d, want 401", recStepAfter.Code)
	}
	assertJSONEqual(t, recStepAfter.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)
}

// A23: logout, then the old cookie gives 401
func TestLogout(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	env.seedUser("logout.user", []authz.Role{authz.HumasEditor}, seed{})
	c := newClient(env)
	if rec := login(c, "logout.user"); rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", rec.Code)
	}

	oldCookie := c.cookie
	if oldCookie == nil {
		t.Fatal("session cookie is nil after login")
	}

	recDel := c.delete("/api/v1/admin/auth/session")
	if recDel.Code != http.StatusNoContent {
		t.Fatalf("delete session status = %d, want 204", recDel.Code)
	}

	cOld := newClient(env)
	cOld.cookie = oldCookie
	recGet := cOld.get("/api/v1/admin/auth/session")
	if recGet.Code != http.StatusUnauthorized {
		t.Fatalf("old cookie get session status = %d, want 401", recGet.Code)
	}
	assertJSONEqual(t, recGet.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Sesi tidak valid atau telah berakhir."}`)
}

// A24: a POST with Origin: https://evil.example gives a 403 Problem; a same-origin POST passes
func TestCrossOriginRejected(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	env.seedUser("andi.admin", []authz.Role{authz.SuperAdmin}, seed{totp: true})

	// 1. Cross-origin POST is rejected with 403 Problem
	cEvil := newClient(env)
	cEvil.header.Set("Origin", "https://evil.example")
	rec := cEvil.post("/api/v1/admin/auth/session", map[string]string{
		"username": "andi.admin",
		"password": testPassword,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d, want 403", rec.Code)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Forbidden","status":403,"detail":"Permintaan lintas situs ditolak."}`)

	// 2. Same-origin POST passes through
	cSame := newClient(env)
	cSame.header.Set("Sec-Fetch-Site", "same-origin")
	recSame := cSame.post("/api/v1/admin/auth/session", map[string]string{
		"username": "andi.admin",
		"password": testPassword,
	})
	if recSame.Code == http.StatusForbidden {
		t.Fatalf("same-origin was forbidden with 403: %s", recSame.Body.String())
	}
	if recSame.Code != http.StatusOK {
		t.Fatalf("same-origin status = %d, want 200", recSame.Code)
	}
}

// A45: a cross-origin POST without a session gives 403, not 401 (middleware order)
func TestCrossOriginBeforeAuth(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	c := newClient(env)
	c.header.Set("Origin", "https://evil.example")
	rec := c.post("/api/v1/admin/auth/users", map[string]any{
		"username": "test.user",
		"fullName": "Test User",
		"roles":    []string{string(authz.HumasEditor)},
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin before auth status = %d, want 403", rec.Code)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Forbidden","status":403,"detail":"Permintaan lintas situs ditolak."}`)
}

// A25: admin responses carry Cache-Control: no-store; absent on /api/v1/health
func TestAdminNoStore(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	env.seedUser("nostore.user", []authz.Role{authz.HumasEditor}, seed{})

	// 1. Anonymous 401 admin response has Cache-Control: no-store
	cAnon := newClient(env)
	rec401 := cAnon.get("/api/v1/admin/auth/session")
	if rec401.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous get status = %d, want 401", rec401.Code)
	}
	if cc := rec401.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("401 Cache-Control = %q, want no-store", cc)
	}

	// 2. Authenticated 200 admin response has Cache-Control: no-store
	cAuth := newClient(env)
	recLogin := login(cAuth, "nostore.user")
	if recLogin.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", recLogin.Code)
	}
	if cc := recLogin.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("login Cache-Control = %q, want no-store", cc)
	}
	rec200 := cAuth.get("/api/v1/admin/auth/session")
	if rec200.Code != http.StatusOK {
		t.Fatalf("authenticated get session status = %d, want 200", rec200.Code)
	}
	if cc := rec200.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("200 Cache-Control = %q, want no-store", cc)
	}

	// 3. /api/v1/health does not have no-store
	recHealth := cAnon.get("/api/v1/health")
	if recHealth.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", recHealth.Code)
	}
	if cc := recHealth.Header().Get("Cache-Control"); strings.Contains(cc, "no-store") {
		t.Fatalf("health Cache-Control contains no-store: %q", cc)
	}
}

// A46: GET /api/v1/health with an invalid or foreign session cookie gives 200 and makes no session query
func TestHealthSkipsSessions(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	poolCfg := env.db.AppConfig(t)
	var counter authSessionQueryCounter
	poolCfg.ConnConfig.Tracer = &counter

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig: %v", err)
	}
	t.Cleanup(pool.Close)

	logger := slog.New(slog.DiscardHandler)
	deps := server.Deps{
		Pool:    pool,
		Keyring: env.keys,
		Config:  env.cfg,
		Now:     env.clock.Now,
	}
	h := server.NewHandler(logger, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	cookieName := sessionCookieName(env.cfg)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "bogus-session-token"}) //nolint:gosec // request cookie in a test
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", rec.Code)
	}
	if gotQueries := counter.count.Load(); gotQueries != 0 {
		t.Fatalf("query count = %d, want 0", gotQueries)
	}
}

// A34: captured logs of a failed login and a TOTP failure contain no password, code or session token
func TestAuthLogsHaveNoSecrets(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	env.seedUser("totp.admin", []authz.Role{authz.SuperAdmin}, seed{totp: true})

	// 1. Failed login with wrong password
	const wrongPassword = "kata-sandi-salah-99" //nolint:gosec // synthetic test value
	c := newClient(env)
	recLoginFail := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "totp.admin",
		"password": wrongPassword,
	})
	if recLoginFail.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want 401", recLoginFail.Code)
	}

	// 2. Successful login with testPassword to reach totp_required
	recLoginOk := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "totp.admin",
		"password": testPassword,
	})
	if recLoginOk.Code != http.StatusOK {
		t.Fatalf("correct login status = %d, want 200", recLoginOk.Code)
	}
	var sessionToken string
	if c.cookie != nil {
		sessionToken = c.cookie.Value
	}

	// 3. Failed TOTP verification with wrong code 000000 (or 999999 if valid code happens to be 000000)
	wrongCode := "000000"
	if env.code("totp.admin", env.clock.Now()) == wrongCode {
		wrongCode = "999999"
	}
	recTotpFail := c.post("/api/v1/admin/auth/session/totp", map[string]string{
		"code": wrongCode,
	})
	if recTotpFail.Code != http.StatusUnauthorized {
		t.Fatalf("wrong totp status = %d, want 401", recTotpFail.Code)
	}

	logs := env.logs()
	if !strings.Contains(logs, "auth.login.failed") {
		t.Fatalf("logs do not contain 'auth.login.failed': %s", logs)
	}
	if !strings.Contains(logs, "auth.totp.failed") {
		t.Fatalf("logs do not contain 'auth.totp.failed': %s", logs)
	}
	if strings.Contains(logs, wrongPassword) {
		t.Fatalf("logs leak wrong password %q", wrongPassword)
	}
	if strings.Contains(logs, testPassword) {
		t.Fatalf("logs leak correct password %q", testPassword)
	}
	if strings.Contains(logs, wrongCode) {
		t.Fatalf("logs leak wrong TOTP code %q", wrongCode)
	}
	if sessionToken != "" && strings.Contains(logs, sessionToken) {
		t.Fatalf("logs leak session token %q", sessionToken)
	}
}
