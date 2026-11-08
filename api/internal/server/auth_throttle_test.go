package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/server"
)

// A8: Failures 1 to 4 answer 401; the 5th answers 429 with Retry-After: 900
// and body {"title":"Too Many Requests","status":429};
// a 6th attempt with the correct password also gives 429;
// after the clock advances 15 min the correct password gives 200.
func TestThrottleBlocksAfterFiveFailures(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("target.user", []authz.Role{authz.HumasEditor}, seed{})
	c := newClient(env)

	// Failures 1-4 answer 401
	for i := 1; i <= 4; i++ {
		rec := c.post("/api/v1/admin/auth/session", map[string]string{
			"username": "target.user",
			"password": "wrong-password",
		})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", i, rec.Code, http.StatusUnauthorized)
		}
		assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Unauthorized","status":401,"detail":"Nama pengguna atau kata sandi salah."}`)
	}

	// 5th failure answers 429 with Retry-After: 900
	rec5 := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "target.user",
		"password": "wrong-password",
	})
	if rec5.Code != http.StatusTooManyRequests {
		t.Fatalf("5th attempt status = %d, want %d", rec5.Code, http.StatusTooManyRequests)
	}
	if got := rec5.Header().Get("Retry-After"); got != "900" {
		t.Fatalf("5th attempt Retry-After = %q, want %q", got, "900")
	}
	assertJSONEqual(t, rec5.Body.Bytes(), `{"title":"Too Many Requests","status":429}`)

	// 6th attempt with correct password also answers 429
	rec6 := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "target.user",
		"password": testPassword,
	})
	if rec6.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt while blocked status = %d, want %d", rec6.Code, http.StatusTooManyRequests)
	}
	assertJSONEqual(t, rec6.Body.Bytes(), `{"title":"Too Many Requests","status":429}`)

	// After clock advances 15 min, correct password gives 200
	env.clock.Advance(15 * time.Minute)
	rec7 := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "target.user",
		"password": testPassword,
	})
	if rec7.Code != http.StatusOK {
		t.Fatalf("login after unblock status = %d, want %d", rec7.Code, http.StatusOK)
	}
}

// A9: After the block expires, one more failure answers 429 with Retry-After: 1800;
// repeating the cycle ends at 86400 (24h) and never more;
// then Advance(24h+1s) decays the counter and a failure gives 401 with failures=1 in the DB.
func TestThrottleBackoffAndDecay(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("backoff.user", []authz.Role{authz.HumasEditor}, seed{})
	c := newClient(env)

	// Trigger initial 5 failures -> 429 (blocked for 15m)
	for i := 1; i <= 5; i++ {
		rec := c.post("/api/v1/admin/auth/session", map[string]string{
			"username": "backoff.user",
			"password": "wrong-password",
		})
		if i < 5 && rec.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d status = %d, want 401", i, rec.Code)
		} else if i == 5 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("failure 5 status = %d, want 429", rec.Code)
		}
	}

	// Advance 15m: first block expires
	env.clock.Advance(15 * time.Minute)

	// One more failure answers 429 with Retry-After: 1800
	rec6 := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "backoff.user",
		"password": "wrong-password",
	})
	if rec6.Code != http.StatusTooManyRequests {
		t.Fatalf("failure 6 status = %d, want 429", rec6.Code)
	}
	if got := rec6.Header().Get("Retry-After"); got != "1800" {
		t.Fatalf("failure 6 Retry-After = %q, want 1800", got)
	}

	// Repeating the cycle: doubling delay up to 24h (86400s) cap
	steps := []struct {
		advance   time.Duration
		wantRetry string
	}{
		{1800 * time.Second, "3600"},
		{3600 * time.Second, "7200"},
		{7200 * time.Second, "14400"},
		{14400 * time.Second, "28800"},
		{28800 * time.Second, "57600"},
		{57600 * time.Second, "86400"},
		{86400 * time.Second, "86400"},
	}
	for i, step := range steps {
		env.clock.Advance(step.advance)
		rec := c.post("/api/v1/admin/auth/session", map[string]string{
			"username": "backoff.user",
			"password": "wrong-password",
		})
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("step %d status = %d, want 429", i, rec.Code)
		}
		if got := rec.Header().Get("Retry-After"); got != step.wantRetry {
			t.Fatalf("step %d Retry-After = %q, want %q", i, got, step.wantRetry)
		}
	}

	// After the 24h block expires, wait > 24h (decay period)
	env.clock.Advance(24*time.Hour + time.Second)

	// Counter has decayed: failure gives 401 instead of 429, and DB failures restarts at 1
	recDecay := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "backoff.user",
		"password": "wrong-password",
	})
	if recDecay.Code != http.StatusUnauthorized {
		t.Fatalf("after decay status = %d, want 401", recDecay.Code)
	}
	dbFailures := queryInt(t, env, "SELECT failures FROM login_throttle WHERE scope = 'account' AND key = 'backoff.user'")
	if dbFailures != 1 {
		t.Fatalf("after decay DB failures = %d, want 1", dbFailures)
	}
}

// A10: 5 unknown usernames fail from one IP; a 6th, different username gives 429.
func TestThrottlePerIP(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	c := newClient(env)

	usernames := []string{
		"unknown.user.alpha",
		"unknown.user.bravo",
		"unknown.user.charlie",
		"unknown.user.delta",
		"unknown.user.echo",
	}
	for i, u := range usernames {
		rec := c.post("/api/v1/admin/auth/session", map[string]string{
			"username": u,
			"password": "wrong-password",
		})
		// The 5th failure itself sets the block, so it answers 429 (docs/modules/auth.md, Login throttle).
		want := http.StatusUnauthorized
		if i == len(usernames)-1 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Fatalf("user %d (%s) status = %d, want %d", i, u, rec.Code, want)
		}
	}

	// 6th different unknown username from same IP answers 429
	rec6 := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "unknown.user.foxtrot",
		"password": "wrong-password",
	})
	if rec6.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt status = %d, want 429", rec6.Code)
	}
	if got := rec6.Header().Get("Retry-After"); got != "900" {
		t.Fatalf("6th attempt Retry-After = %q, want 900", got)
	}
	assertJSONEqual(t, rec6.Body.Bytes(), `{"title":"Too Many Requests","status":429}`)
}

// A11: 2 failures, then success deletes the account throttle row and keeps the IP row with failures 2.
func TestThrottleSuccessClearsAccountOnly(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("clear.user", []authz.Role{authz.HumasEditor}, seed{})
	c := newClient(env)

	for i := 1; i <= 2; i++ {
		rec := c.post("/api/v1/admin/auth/session", map[string]string{
			"username": "clear.user",
			"password": "wrong-password",
		})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d status = %d, want 401", i, rec.Code)
		}
	}

	// Verify both account and IP have failures = 2 before success
	accBefore := queryInt(t, env, "SELECT failures FROM login_throttle WHERE scope = 'account' AND key = 'clear.user'")
	if accBefore != 2 {
		t.Fatalf("before success account failures = %d, want 2", accBefore)
	}
	ipBefore := queryInt(t, env, "SELECT failures FROM login_throttle WHERE scope = 'ip' AND key = '192.0.2.1'")
	if ipBefore != 2 {
		t.Fatalf("before success ip failures = %d, want 2", ipBefore)
	}

	// Successful login
	recSuccess := c.post("/api/v1/admin/auth/session", map[string]string{
		"username": "clear.user",
		"password": testPassword,
	})
	if recSuccess.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", recSuccess.Code)
	}

	// Account row is deleted
	accAfter := queryInt(t, env, "SELECT count(*) FROM login_throttle WHERE scope = 'account' AND key = 'clear.user'")
	if accAfter != 0 {
		t.Fatalf("after success account rows = %d, want 0", accAfter)
	}

	// IP row is preserved with failures = 2
	ipAfter := queryInt(t, env, "SELECT failures FROM login_throttle WHERE scope = 'ip' AND key = '192.0.2.1'")
	if ipAfter != 2 {
		t.Fatalf("after success ip failures = %d, want 2", ipAfter)
	}
}

// A41: TrustProxyHeader=true with X-Real-IP: 203.0.113.9 sets IP key to 203.0.113.9;
// false uses the remote host 192.0.2.1 and ignores X-Real-IP.
func TestClientIPSource(t *testing.T) {
	t.Parallel()

	// Case 1: TrustProxyHeader = true
	envTrust := newEnv(t, func(cfg *server.Config) {
		cfg.TrustProxyHeader = true
	})
	envTrust.seedUser("proxy.user", []authz.Role{authz.HumasEditor}, seed{})
	c1 := newClient(envTrust)
	c1.header.Set("X-Real-IP", "203.0.113.9")
	rec1 := c1.post("/api/v1/admin/auth/session", map[string]string{
		"username": "proxy.user",
		"password": "wrong-password",
	})
	if rec1.Code != http.StatusUnauthorized {
		t.Fatalf("case 1 status = %d, want 401", rec1.Code)
	}

	countProxy := queryInt(t, envTrust, "SELECT count(*) FROM login_throttle WHERE scope = 'ip' AND key = '203.0.113.9'")
	if countProxy != 1 {
		t.Fatalf("case 1 ip 203.0.113.9 count = %d, want 1", countProxy)
	}
	countRemote1 := queryInt(t, envTrust, "SELECT count(*) FROM login_throttle WHERE scope = 'ip' AND key = '192.0.2.1'")
	if countRemote1 != 0 {
		t.Fatalf("case 1 ip 192.0.2.1 count = %d, want 0", countRemote1)
	}

	// Case 2: TrustProxyHeader = false (default)
	envDefault := newEnv(t, func(cfg *server.Config) {
		cfg.TrustProxyHeader = false
	})
	envDefault.seedUser("noproxy.user", []authz.Role{authz.HumasEditor}, seed{})
	c2 := newClient(envDefault)
	c2.header.Set("X-Real-IP", "203.0.113.9")
	rec2 := c2.post("/api/v1/admin/auth/session", map[string]string{
		"username": "noproxy.user",
		"password": "wrong-password",
	})
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("case 2 status = %d, want 401", rec2.Code)
	}

	countRemote2 := queryInt(t, envDefault, "SELECT count(*) FROM login_throttle WHERE scope = 'ip' AND key = '192.0.2.1'")
	if countRemote2 != 1 {
		t.Fatalf("case 2 ip 192.0.2.1 count = %d, want 1", countRemote2)
	}
	countIgnoredProxy := queryInt(t, envDefault, "SELECT count(*) FROM login_throttle WHERE scope = 'ip' AND key = '203.0.113.9'")
	if countIgnoredProxy != 0 {
		t.Fatalf("case 2 ip 203.0.113.9 count = %d, want 0", countIgnoredProxy)
	}
}
