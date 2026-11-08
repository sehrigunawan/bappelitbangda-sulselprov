package server_test

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
)

// A28: updateUser gives 409 for self-demotion, self-disable and the last active Super Admin;
// createPasswordReset and deleteUserTotp on the caller's own id give 409
func TestUpdateUserConflicts(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	sa1ID := env.seedUser("sa.one", []authz.Role{authz.SuperAdmin}, seed{totp: true})
	sa2ID := env.seedUser("sa.two", []authz.Role{authz.SuperAdmin}, seed{totp: true})

	c1 := newClient(env)
	if rec := login(c1, "sa.one"); rec.Code != http.StatusOK {
		t.Fatalf("login sa.one: %d", rec.Code)
	}

	c2 := newClient(env)
	if rec := login(c2, "sa.two"); rec.Code != http.StatusOK {
		t.Fatalf("login sa.two: %d", rec.Code)
	}

	// 1. Self-demotion gives 409
	rec := c1.put(fmt.Sprintf("/api/v1/admin/auth/users/%d", sa1ID), map[string]any{
		"fullName": "SA One",
		"roles":    []string{string(authz.HumasEditor)},
		"active":   true,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("self-demote status = %d, want 409", rec.Code)
	}

	// 2. Self-disable gives 409
	rec = c1.put(fmt.Sprintf("/api/v1/admin/auth/users/%d", sa1ID), map[string]any{
		"fullName": "SA One",
		"roles":    []string{string(authz.SuperAdmin)},
		"active":   false,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("self-disable status = %d, want 409", rec.Code)
	}

	// 3. createPasswordReset and deleteUserTotp on own id with step-up give 409
	if rec := stepUp(c1, "sa.one"); rec.Code != http.StatusOK {
		t.Fatalf("stepUp sa.one: %d", rec.Code)
	}
	rec = c1.post(fmt.Sprintf("/api/v1/admin/auth/users/%d/password-reset", sa1ID), nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("own password-reset status = %d, want 409", rec.Code)
	}
	rec = c1.delete(fmt.Sprintf("/api/v1/admin/auth/users/%d/totp", sa1ID))
	if rec.Code != http.StatusConflict {
		t.Fatalf("own delete totp status = %d, want 409", rec.Code)
	}

	// 4. Two SAs concurrently demoting each other: statuses are one 200 plus one of {409, 403},
	// and DB has exactly 1 active super_admin.
	type demoteResult struct {
		code int
	}
	ch1 := make(chan demoteResult, 1)
	ch2 := make(chan demoteResult, 1)
	var startWg sync.WaitGroup
	startWg.Add(1)

	go func() {
		startWg.Wait()
		r := c1.put(fmt.Sprintf("/api/v1/admin/auth/users/%d", sa2ID), map[string]any{
			"fullName": "SA Two",
			"roles":    []string{string(authz.HumasEditor)},
			"active":   true,
		})
		ch1 <- demoteResult{code: r.Code}
	}()

	go func() {
		startWg.Wait()
		r := c2.put(fmt.Sprintf("/api/v1/admin/auth/users/%d", sa1ID), map[string]any{
			"fullName": "SA One",
			"roles":    []string{string(authz.HumasEditor)},
			"active":   true,
		})
		ch2 <- demoteResult{code: r.Code}
	}()

	startWg.Done()
	res1 := <-ch1
	res2 := <-ch2

	codes := []int{res1.code, res2.code}
	has200 := codes[0] == http.StatusOK || codes[1] == http.StatusOK
	hasConflictOrForbidden := (codes[0] == http.StatusConflict || codes[0] == http.StatusForbidden) ||
		(codes[1] == http.StatusConflict || codes[1] == http.StatusForbidden)

	if !has200 || !hasConflictOrForbidden {
		t.Fatalf("concurrent demotion statuses = %v, want one 200 and one of {409, 403}", codes)
	}

	activeSAs := queryInt(t, env.db.Super, `
		SELECT count(DISTINCT u.id)
		FROM users u
		JOIN user_roles r ON u.id = r.user_id
		WHERE r.role = 'super_admin' AND u.disabled_at IS NULL
	`)
	if activeSAs != 1 {
		t.Fatalf("active super_admins in DB = %d, want 1", activeSAs)
	}
}

// A29: createUser gives 409 for a duplicate, 400 for an unknown role, 400 for empty roles.
// 201 returns a temporary password that works, and the new user is in password_change_required
func TestCreateUser(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	env.seedUser("sa.creator", []authz.Role{authz.SuperAdmin}, seed{totp: true})
	c := newClient(env)
	if rec := login(c, "sa.creator"); rec.Code != http.StatusOK {
		t.Fatalf("login sa.creator: %d", rec.Code)
	}

	// 1. Unknown role "root" -> 400
	rec := c.post("/api/v1/admin/auth/users", map[string]any{
		"username": "user.root",
		"fullName": "User Root",
		"roles":    []string{"root"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown role status = %d, want 400", rec.Code)
	}

	// 2. Empty roles [] -> 400
	rec = c.post("/api/v1/admin/auth/users", map[string]any{
		"username": "user.empty",
		"fullName": "User Empty",
		"roles":    []string{},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty roles status = %d, want 400", rec.Code)
	}

	// 3. Repeated role -> 400
	rec = c.post("/api/v1/admin/auth/users", map[string]any{
		"username": "user.repeated",
		"fullName": "User Repeated",
		"roles":    []string{string(authz.HumasEditor), string(authz.HumasEditor)},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("repeated role status = %d, want 400", rec.Code)
	}

	// 4. Valid user creation -> 201
	rec = c.post("/api/v1/admin/auth/users", map[string]any{
		"username": "andi.operator",
		"fullName": "Andi Operator",
		"roles":    []string{string(authz.DocumentOperator)},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, want 201", rec.Code)
	}

	created := decode[api.UserCreated](t, rec)
	matched, _ := regexp.MatchString("^[0-9a-hjkmnp-tv-z]{16}$", created.TemporaryPassword)
	if !matched {
		t.Fatalf("temporaryPassword %q does not match alphabet/length", created.TemporaryPassword)
	}
	if created.User.Username != "andi.operator" {
		t.Fatalf("username = %q, want andi.operator", created.User.Username)
	}

	// 5. Duplicate username -> 409 Nama pengguna sudah dipakai.
	recDup := c.post("/api/v1/admin/auth/users", map[string]any{
		"username": "andi.operator",
		"fullName": "Another User",
		"roles":    []string{string(authz.HumasEditor)},
	})
	if recDup.Code != http.StatusConflict {
		t.Fatalf("duplicate username status = %d, want 409", recDup.Code)
	}
	assertJSONEqual(t, recDup.Body.Bytes(), `{"title":"Conflict","status":409,"detail":"Nama pengguna sudah dipakai."}`)

	// 6. Logging in with temporary password lands in password_change_required
	cNew := newClient(env)
	recLoginNew := cNew.post("/api/v1/admin/auth/session", map[string]string{
		"username": "andi.operator",
		"password": created.TemporaryPassword,
	})
	if recLoginNew.Code != http.StatusOK {
		t.Fatalf("new user login status = %d, want 200", recLoginNew.Code)
	}
	state := decode[api.SessionState](t, recLoginNew)
	if state.Stage != api.PasswordChangeRequired {
		t.Fatalf("stage = %q, want %q", state.Stage, api.PasswordChangeRequired)
	}
}

// A30: listUsers gives 400 for page=0, page=abc and pageSize=51. Defaults are page 1, size 10, and total is correct
// RF2: items is [] (never null) when a page has no users
func TestListUsersPaging(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	env.seedUser("user.01", []authz.Role{authz.SuperAdmin}, seed{totp: true})
	for i := 2; i <= 12; i++ {
		env.seedUser(fmt.Sprintf("user.%02d", i), []authz.Role{authz.HumasEditor}, seed{})
	}

	c := newClient(env)
	if rec := login(c, "user.01"); rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", rec.Code)
	}

	// 1. Default paging: page 1, pageSize 10, total 12, usernames ascending
	rec := c.get("/api/v1/admin/auth/users")
	if rec.Code != http.StatusOK {
		t.Fatalf("default ListUsers status = %d, want 200", rec.Code)
	}
	p1 := decode[api.UserPage](t, rec)
	if p1.Page != 1 {
		t.Fatalf("page = %d, want 1", p1.Page)
	}
	if p1.PageSize != 10 {
		t.Fatalf("pageSize = %d, want 10", p1.PageSize)
	}
	if p1.Total != 12 {
		t.Fatalf("total = %d, want 12", p1.Total)
	}
	if len(p1.Items) != 10 {
		t.Fatalf("len(items) = %d, want 10", len(p1.Items))
	}
	for i := range p1.Items {
		want := fmt.Sprintf("user.%02d", i+1)
		if p1.Items[i].Username != want {
			t.Fatalf("item[%d].Username = %q, want %q", i, p1.Items[i].Username, want)
		}
	}

	// 2. page=2 gives 2 items
	rec = c.get("/api/v1/admin/auth/users?page=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("page=2 status = %d, want 200", rec.Code)
	}
	p2 := decode[api.UserPage](t, rec)
	if p2.Page != 2 {
		t.Fatalf("page = %d, want 2", p2.Page)
	}
	if len(p2.Items) != 2 {
		t.Fatalf("page=2 len(items) = %d, want 2", len(p2.Items))
	}
	if p2.Items[0].Username != "user.11" || p2.Items[1].Username != "user.12" {
		t.Fatalf("page=2 usernames = %q, %q; want user.11, user.12", p2.Items[0].Username, p2.Items[1].Username)
	}

	// 3. page=99 gives "items":[]
	rec = c.get("/api/v1/admin/auth/users?page=99")
	if rec.Code != http.StatusOK {
		t.Fatalf("page=99 status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("page=99 body does not contain '\"items\":[]': %s", rec.Body.String())
	}
	p99 := decode[api.UserPage](t, rec)
	if p99.Items == nil || len(p99.Items) != 0 {
		t.Fatalf("page=99 items = %v, want empty non-nil slice", p99.Items)
	}

	// A page number far past any real table is still an empty page, not an error.
	rec = c.get("/api/v1/admin/auth/users?page=2000000000")
	if rec.Code != http.StatusOK {
		t.Fatalf("page=2000000000 status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("page=2000000000 body does not contain '\"items\":[]': %s", rec.Body.String())
	}

	// 4. Invalid query parameters give 400
	invalidParams := []string{
		"/api/v1/admin/auth/users?page=0",
		"/api/v1/admin/auth/users?page=abc",
		"/api/v1/admin/auth/users?pageSize=51",
		"/api/v1/admin/auth/users?pageSize=0",
	}
	for _, url := range invalidParams {
		rec = c.get(url)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("get %s status = %d, want 400", url, rec.Code)
		}
	}
}

// A31: getUser with an unknown id gives 404
func TestGetUserNotFound(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	env.seedUser("sa.admin", []authz.Role{authz.SuperAdmin}, seed{totp: true})
	c := newClient(env)
	if rec := login(c, "sa.admin"); rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", rec.Code)
	}

	rec := c.get("/api/v1/admin/auth/users/999999")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("getUser status = %d, want 404", rec.Code)
	}
	assertJSONEqual(t, rec.Body.Bytes(), `{"title":"Not Found","status":404}`)
}

// A32: after deleteUserTotp, a mandatory-role user's next login reaches totp_enrollment_required
func TestDeleteUserTotpForcesEnrollment(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	env.seedUser("sa.admin", []authz.Role{authz.SuperAdmin}, seed{totp: true})
	targetID := env.seedUser("target.ppid", []authz.Role{authz.PpidOfficer}, seed{totp: true})

	cSA := newClient(env)
	if rec := login(cSA, "sa.admin"); rec.Code != http.StatusOK {
		t.Fatalf("sa login: %d", rec.Code)
	}
	if rec := stepUp(cSA, "sa.admin"); rec.Code != http.StatusOK {
		t.Fatalf("stepUp: %d", rec.Code)
	}

	recDel := cSA.delete(fmt.Sprintf("/api/v1/admin/auth/users/%d/totp", targetID))
	if recDel.Code != http.StatusNoContent {
		t.Fatalf("delete totp status = %d, want 204", recDel.Code)
	}

	cTarget := newClient(env)
	recLogin := cTarget.post("/api/v1/admin/auth/session", map[string]string{
		"username": "target.ppid",
		"password": testPassword,
	})
	if recLogin.Code != http.StatusOK {
		t.Fatalf("target login status = %d, want 200", recLogin.Code)
	}
	state := decode[api.SessionState](t, recLogin)
	if state.Stage != api.TotpEnrollmentRequired {
		t.Fatalf("stage = %q, want %q", state.Stage, api.TotpEnrollmentRequired)
	}
}

// A47: getUser, updateUser, createPasswordReset and deleteUserTotp give 400 for userId abc and 0
func TestUserIdValidation(t *testing.T) {
	t.Parallel()
	env := newEnv(t)

	env.seedUser("sa.admin", []authz.Role{authz.SuperAdmin}, seed{totp: true})
	c := newClient(env)
	if rec := login(c, "sa.admin"); rec.Code != http.StatusOK {
		t.Fatalf("sa login: %d", rec.Code)
	}
	if rec := stepUp(c, "sa.admin"); rec.Code != http.StatusOK {
		t.Fatalf("stepUp: %d", rec.Code)
	}

	for _, badID := range []string{"abc", "0"} {
		// 1. GET /api/v1/admin/auth/users/{userId}
		rec := c.get(fmt.Sprintf("/api/v1/admin/auth/users/%s", badID))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("GET /users/%s status = %d, want 400", badID, rec.Code)
		}

		// 2. PUT /api/v1/admin/auth/users/{userId}
		rec = c.put(fmt.Sprintf("/api/v1/admin/auth/users/%s", badID), map[string]any{
			"fullName": "Valid Name",
			"roles":    []string{string(authz.HumasEditor)},
			"active":   true,
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("PUT /users/%s status = %d, want 400", badID, rec.Code)
		}

		// 3. POST /api/v1/admin/auth/users/{userId}/password-reset
		rec = c.post(fmt.Sprintf("/api/v1/admin/auth/users/%s/password-reset", badID), nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("POST /users/%s/password-reset status = %d, want 400", badID, rec.Code)
		}

		// 4. DELETE /api/v1/admin/auth/users/{userId}/totp
		rec = c.delete(fmt.Sprintf("/api/v1/admin/auth/users/%s/totp", badID))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("DELETE /users/%s/totp status = %d, want 400", badID, rec.Code)
		}
	}
}
