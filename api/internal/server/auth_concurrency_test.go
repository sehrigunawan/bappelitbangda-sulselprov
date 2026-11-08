package server_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
)

// A16 (concurrency): two sessions that present the same recovery code at the same time cannot both
// succeed. Both requests read the code as unused; the guarded UPDATE lets exactly one use it.
func TestRecoveryCodeConcurrentUse(t *testing.T) {
	t.Parallel()
	env := newEnv(t)
	env.seedUser("conc.user", []authz.Role{authz.SuperAdmin}, seed{totp: true})

	owner := newClient(env)
	if rec := login(owner, "conc.user"); rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec := stepUp(owner, "conc.user"); rec.Code != http.StatusOK {
		t.Fatalf("step-up status = %d, want %d", rec.Code, http.StatusOK)
	}
	rec := owner.post("/api/v1/admin/auth/totp/recovery-codes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("regenerate status = %d, want %d", rec.Code, http.StatusOK)
	}
	code := decode[api.RecoveryCodes](t, rec).Codes[0]

	// Two fresh logins, both waiting at the totp_required stage.
	clients := []*client{newClient(env), newClient(env)}
	for i, c := range clients {
		c.remote = []string{"192.0.2.10:1234", "192.0.2.11:1234"}[i] // separate IPs: no shared throttle counter
		if rec := c.post("/api/v1/admin/auth/session", map[string]string{"username": "conc.user", "password": testPassword}); rec.Code != http.StatusOK {
			t.Fatalf("login %d status = %d, want %d", i, rec.Code, http.StatusOK)
		}
	}

	statuses := make([]int, len(clients))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, c := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			statuses[i] = c.post("/api/v1/admin/auth/session/totp", map[string]string{"recoveryCode": code}).Code
		}()
	}
	close(start)
	wg.Wait()

	ok, denied := 0, 0
	for _, s := range statuses {
		switch s {
		case http.StatusOK:
			ok++
		case http.StatusUnauthorized:
			denied++
		}
	}
	if ok != 1 || denied != 1 {
		t.Fatalf("statuses = %v, want exactly one 200 and one 401", statuses)
	}
}
