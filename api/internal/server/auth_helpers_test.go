package server_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/auth"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/dbtest"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/keyring"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/server"
)

const testPassword = "rahasia-panjang-01" //nolint:gosec // synthetic test value

// testPasswordHash computes the argon2id hash of testPassword once.
var testPasswordHash = sync.OnceValue(func() string {
	h := auth.NewHasher()
	hash, err := h.Hash(context.Background(), testPassword)
	if err != nil {
		panic("auth.NewHasher().Hash failed: " + err.Error())
	}
	return hash
})

// testClock provides a controllable clock for tests, initialized to 2026-10-15T06:00:00Z.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func newTestClock() *testClock {
	start := time.Date(2026, 10, 15, 6, 0, 0, 0, time.UTC)
	return &testClock{t: start}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// safeBuffer is a thread-safe bytes.Buffer for slog JSON output.
type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func (s *safeBuffer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.b.Reset()
}

// testEnv encapsulates the test database, keyring, fake clock, handler, and state.
type testEnv struct {
	t       testing.TB
	db      *dbtest.DB
	keys    *keyring.Keyring
	clock   *testClock
	h       http.Handler
	cfg     server.Config
	secrets map[string]string // username -> TOTP base32 secret
	logBuf  *safeBuffer
}

// logs returns the captured JSON log lines.
func (e *testEnv) logs() string {
	return e.logBuf.String()
}

// newEnv creates a fresh test environment with an isolated cloned database and test keyring.
func newEnv(t testing.TB, opts ...func(*server.Config)) *testEnv {
	t.Helper()
	db := dbtest.New(t)

	rawKey := make([]byte, 32)
	if _, err := rand.Read(rawKey); err != nil {
		t.Fatalf("newEnv rand.Read: %v", err)
	}
	kekContent := fmt.Sprintf("test-1 %s\n", base64.StdEncoding.EncodeToString(rawKey))
	kekPath := filepath.Join(t.TempDir(), "kek")
	if err := os.WriteFile(kekPath, []byte(kekContent), 0600); err != nil {
		t.Fatalf("newEnv write kek file: %v", err)
	}

	ctx := context.Background()
	k, err := keyring.Load(ctx, db.App, kekPath, "test-1")
	if err != nil {
		t.Fatalf("newEnv keyring.Load: %v", err)
	}

	cfg := server.DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	clock := newTestClock()
	logBuf := &safeBuffer{}
	logger := slog.New(slog.NewJSONHandler(logBuf, nil))

	deps := server.Deps{
		Pool:    db.App,
		Keyring: k,
		Config:  cfg,
		Now:     clock.Now,
	}
	h := server.NewHandler(logger, deps)

	return &testEnv{
		t:       t,
		db:      db,
		keys:    k,
		clock:   clock,
		h:       h,
		cfg:     cfg,
		secrets: make(map[string]string),
		logBuf:  logBuf,
	}
}

// seed describes flags for seedUser.
type seed struct {
	totp       bool
	mustChange bool
	disabled   bool
}

// seedUser inserts an admin user and roles directly into the superuser pool.
func (e *testEnv) seedUser(username string, roles []authz.Role, o seed) int64 {
	e.t.Helper()
	var disabledAt *time.Time
	if o.disabled {
		t := e.clock.Now()
		disabledAt = &t
	}

	var id int64
	err := e.db.Super.QueryRow(
		context.Background(),
		`INSERT INTO users (username, full_name, password_hash, password_change_required, disabled_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		username,
		"User "+username,
		testPasswordHash(),
		o.mustChange,
		disabledAt,
	).Scan(&id)
	if err != nil {
		e.t.Fatalf("seedUser insert user %q: %v", username, err)
	}

	if o.totp {
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      "Bappelitbangda Sulsel",
			AccountName: username,
			Period:      30,
			SecretSize:  20,
			Digits:      otp.DigitsSix,
			Algorithm:   otp.AlgorithmSHA1,
		})
		if err != nil {
			e.t.Fatalf("seedUser totp.Generate: %v", err)
		}
		secret := key.Secret()
		aad := []byte(fmt.Sprintf("users.totp_secret:%d", id))
		sealed, err := e.keys.Seal([]byte(secret), aad)
		if err != nil {
			e.t.Fatalf("seedUser keys.Seal: %v", err)
		}
		now := e.clock.Now()
		_, err = e.db.Super.Exec(
			context.Background(),
			`UPDATE users SET totp_secret = $1, totp_confirmed_at = $2 WHERE id = $3`,
			sealed,
			now,
			id,
		)
		if err != nil {
			e.t.Fatalf("seedUser update totp: %v", err)
		}
		e.secrets[username] = secret
	}

	for _, role := range roles {
		_, err = e.db.Super.Exec(
			context.Background(),
			`INSERT INTO user_roles (user_id, role) VALUES ($1, $2)`,
			id,
			string(role),
		)
		if err != nil {
			e.t.Fatalf("seedUser insert user_role %q: %v", role, err)
		}
	}
	return id
}

// code generates a valid 6-digit TOTP code for username at the given timestamp.
func (e *testEnv) code(username string, at time.Time) string {
	e.t.Helper()
	secret, ok := e.secrets[username]
	if !ok {
		e.t.Fatalf("code: no TOTP secret recorded for user %q", username)
	}
	code, err := totp.GenerateCodeCustom(secret, at, totp.ValidateOpts{
		Period:    30,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		e.t.Fatalf("code: GenerateCodeCustom: %v", err)
	}
	return code
}

// client represents an HTTP test client with cookie tracking and remote address emulation.
type client struct {
	*testEnv
	env    *testEnv
	cookie *http.Cookie
	header http.Header
	remote string
}

// newClient constructs a client pointing at env.
func newClient(env *testEnv) *client {
	return &client{
		testEnv: env,
		env:     env,
		header:  make(http.Header),
		remote:  "192.0.2.1:1234",
	}
}

// do sends a request through the test handler, maintaining session cookies.
func (c *client) do(method, path string, body any) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	hasBody := false
	if body != nil {
		hasBody = true
		switch v := body.(type) {
		case io.Reader:
			bodyReader = v
		case []byte:
			bodyReader = bytes.NewReader(v)
		case string:
			bodyReader = strings.NewReader(v)
		default:
			data, err := json.Marshal(v)
			if err != nil {
				c.t.Fatalf("client.do: marshal body: %v", err)
			}
			bodyReader = bytes.NewReader(data)
		}
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if hasBody && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.remote != "" {
		req.RemoteAddr = c.remote
	} else {
		req.RemoteAddr = "192.0.2.1:1234"
	}

	for k, vv := range c.header {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}

	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}

	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)

	cookieName := sessionCookieName(c.cfg)
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == cookieName || cookie.Name == "__Host-bpl_session" || cookie.Name == "bpl_session" {
			if cookie.MaxAge < 0 || (!cookie.Expires.IsZero() && cookie.Expires.Before(time.Now())) {
				c.cookie = nil
			} else {
				c.cookie = cookie
			}
		}
	}

	return rec
}

// get issues a GET request.
func (c *client) get(path string) *httptest.ResponseRecorder {
	return c.do(http.MethodGet, path, nil)
}

// post issues a POST request with JSON body.
func (c *client) post(path string, body any) *httptest.ResponseRecorder {
	return c.do(http.MethodPost, path, body)
}

// put issues a PUT request with JSON body.
func (c *client) put(path string, body any) *httptest.ResponseRecorder {
	return c.do(http.MethodPut, path, body)
}

// delete issues a DELETE request.
func (c *client) delete(path string) *httptest.ResponseRecorder {
	return c.do(http.MethodDelete, path, nil)
}

// login logs in with testPassword, and verifies TOTP if stage is totp_required.
func login(c *client, username string) *httptest.ResponseRecorder {
	rec := c.do(http.MethodPost, "/api/v1/admin/auth/session", map[string]string{
		"username": username,
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		return rec
	}
	var state struct {
		Stage string `json:"stage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err == nil {
		if state.Stage == "totp_required" {
			code := c.env.code(username, c.env.clock.Now())
			rec = c.do(http.MethodPost, "/api/v1/admin/auth/session/totp", map[string]string{
				"code": code,
			})
		}
	}
	return rec
}

// stepUp advances the clock by 30 seconds and submits a fresh TOTP code to /session/step-up.
func stepUp(c *client, username string) *httptest.ResponseRecorder {
	c.env.clock.Advance(30 * time.Second)
	code := c.env.code(username, c.env.clock.Now())
	return c.do(http.MethodPost, "/api/v1/admin/auth/session/step-up", map[string]string{
		"code": code,
	})
}

// sessionCookieName returns the session cookie name based on CookieSecure.
func sessionCookieName(cfg server.Config) string {
	if cfg.CookieSecure {
		return "__Host-bpl_session"
	}
	return "bpl_session"
}

// assertJSONEqual compares JSON documents ignoring key order (generated struct orders keys alphabetically).
func assertJSONEqual(t testing.TB, got []byte, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("response is not JSON: %v: %s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("bad expectation: %v", err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Fatalf("body = %s, want %s", strings.TrimSpace(string(got)), strings.TrimSpace(want))
	}
}

// decode unmarshals JSON response body into T.
func decode[T any](t testing.TB, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %T: %v (body: %s)", v, err, rec.Body.String())
	}
	return v
}

// exec executes raw SQL against a pool, dbtest.DB, or transaction.
func exec(t testing.TB, target any, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	var err error
	switch v := target.(type) {
	case *pgxpool.Pool:
		_, err = v.Exec(ctx, sql, args...)
	case *dbtest.DB:
		_, err = v.Super.Exec(ctx, sql, args...)
	case *testEnv:
		_, err = v.db.Super.Exec(ctx, sql, args...)
	case pgx.Tx:
		_, err = v.Exec(ctx, sql, args...)
	default:
		t.Fatalf("exec: unsupported target type %T", target)
	}
	if err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

// queryInt queries an integer scalar against a pool, dbtest.DB, or transaction.
func queryInt(t testing.TB, target any, sql string, args ...any) int64 {
	t.Helper()
	ctx := context.Background()
	var row pgx.Row
	switch v := target.(type) {
	case *pgxpool.Pool:
		row = v.QueryRow(ctx, sql, args...)
	case *dbtest.DB:
		row = v.Super.QueryRow(ctx, sql, args...)
	case *testEnv:
		row = v.db.Super.QueryRow(ctx, sql, args...)
	case pgx.Tx:
		row = v.QueryRow(ctx, sql, args...)
	default:
		t.Fatalf("queryInt: unsupported target type %T", target)
	}
	var n int64
	if err := row.Scan(&n); err != nil {
		t.Fatalf("queryInt %q: %v", sql, err)
	}
	return n
}
