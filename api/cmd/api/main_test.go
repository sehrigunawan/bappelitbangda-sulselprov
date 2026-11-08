package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/dbtest"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/keyring"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/server"
)

// A36: CLI admin subcommands: create super_admin, reset-password, and reset-totp break-glass.
func TestAdminCLI(t *testing.T) {
	t.Parallel()
	db := dbtest.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rawURL := os.Getenv("TEST_DATABASE_URL")
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	parsed.Path = "/" + db.Name
	superuserURL := parsed.String()

	envMap := map[string]string{
		"DATABASE_URL": superuserURL,
		"API_PORT":     "-1",
	}
	getenv := func(key string) string { return envMap[key] }

	// 1. admin create creates super_admin with password_change_required and prints temporary password
	var out bytes.Buffer
	err = run(ctx, []string{"admin", "create", "--username", "andi.admin", "--full-name", "Andi Pratama"}, &out, getenv)
	if err != nil {
		t.Fatalf("run admin create: %v", err)
	}

	reCreate := regexp.MustCompile(`^created super_admin andi\.admin \(id (\d+)\)\ntemporary password: ([0-9a-hjkmnp-tv-z]{16})\n$`)
	matches := reCreate.FindStringSubmatch(out.String())
	if len(matches) < 3 {
		t.Fatalf("output = %q, want created super_admin format", out.String())
	}

	var mustChange bool
	var role string
	err = db.Super.QueryRow(ctx, "SELECT password_change_required FROM users WHERE username = $1", "andi.admin").Scan(&mustChange)
	if err != nil {
		t.Fatalf("query user: %v", err)
	}
	if !mustChange {
		t.Fatal("password_change_required = false, want true")
	}
	err = db.Super.QueryRow(ctx, "SELECT role FROM user_roles WHERE user_id = (SELECT id FROM users WHERE username = $1)", "andi.admin").Scan(&role)
	if err != nil {
		t.Fatalf("query user role: %v", err)
	}
	if role != "super_admin" {
		t.Fatalf("role = %q, want super_admin", role)
	}

	// 2. Duplicate admin create fails with error containing "taken"
	out.Reset()
	err = run(ctx, []string{"admin", "create", "--username", "andi.admin", "--full-name", "Andi Pratama"}, &out, getenv)
	if err == nil {
		t.Fatal("duplicate admin create succeeded, want error")
	}
	if !strings.Contains(err.Error(), "taken") {
		t.Fatalf("error %q does not contain 'taken'", err.Error())
	}

	// 3. admin reset-password generates new temporary password and bumps session_epoch
	out.Reset()
	err = run(ctx, []string{"admin", "reset-password", "--username", "andi.admin"}, &out, getenv)
	if err != nil {
		t.Fatalf("run admin reset-password: %v", err)
	}
	reReset := regexp.MustCompile(`^temporary password: ([0-9a-hjkmnp-tv-z]{16})\n$`)
	if !reReset.MatchString(out.String()) {
		t.Fatalf("reset-password output = %q, want temporary password format", out.String())
	}
	var epoch int64
	err = db.Super.QueryRow(ctx, "SELECT session_epoch, password_change_required FROM users WHERE username = $1", "andi.admin").Scan(&epoch, &mustChange)
	if err != nil {
		t.Fatalf("query user after reset-password: %v", err)
	}
	if !mustChange {
		t.Fatal("password_change_required = false after reset-password")
	}
	if epoch < 1 {
		t.Fatalf("session_epoch = %d, want >= 1", epoch)
	}

	// 4. admin reset-totp clears totp fields, deletes recovery codes, and bumps session_epoch
	var totpUserID int64
	err = db.Super.QueryRow(ctx, "INSERT INTO users (username, full_name, password_hash, totp_secret, totp_confirmed_at, totp_last_step, session_epoch) VALUES ($1, $2, $3, $4, now(), 12345, 1) RETURNING id", "totp.user", "TOTP User", "hash", []byte("secret")).Scan(&totpUserID)
	if err != nil {
		t.Fatalf("seed totp user: %v", err)
	}
	_, err = db.Super.Exec(ctx, "INSERT INTO user_roles (user_id, role) VALUES ($1, 'super_admin')", totpUserID)
	if err != nil {
		t.Fatalf("seed user role: %v", err)
	}
	_, err = db.Super.Exec(ctx, "INSERT INTO recovery_codes (user_id, code_hash) VALUES ($1, $2)", totpUserID, "codehash")
	if err != nil {
		t.Fatalf("seed recovery code: %v", err)
	}

	out.Reset()
	err = run(ctx, []string{"admin", "reset-totp", "--username", "totp.user"}, &out, getenv)
	if err != nil {
		t.Fatalf("run admin reset-totp: %v", err)
	}
	wantResetTotp := "TOTP reset for totp.user\n"
	if out.String() != wantResetTotp {
		t.Fatalf("reset-totp output = %q, want %q", out.String(), wantResetTotp)
	}

	var secret []byte
	var confirmedAt *time.Time
	var lastStep *int64
	var newEpoch int64
	err = db.Super.QueryRow(ctx, "SELECT totp_secret, totp_confirmed_at, totp_last_step, session_epoch FROM users WHERE id = $1", totpUserID).Scan(&secret, &confirmedAt, &lastStep, &newEpoch)
	if err != nil {
		t.Fatalf("query user after reset-totp: %v", err)
	}
	if secret != nil {
		t.Fatalf("totp_secret = %v, want nil", secret)
	}
	if confirmedAt != nil {
		t.Fatalf("totp_confirmed_at = %v, want nil", confirmedAt)
	}
	if lastStep != nil {
		t.Fatalf("totp_last_step = %v, want nil", lastStep)
	}
	if newEpoch != 2 {
		t.Fatalf("session_epoch = %d, want 2", newEpoch)
	}
	var codeCount int
	err = db.Super.QueryRow(ctx, "SELECT count(*) FROM recovery_codes WHERE user_id = $1", totpUserID).Scan(&codeCount)
	if err != nil {
		t.Fatalf("query recovery codes: %v", err)
	}
	if codeCount != 0 {
		t.Fatalf("recovery_codes count = %d, want 0", codeCount)
	}

	// 5. Unknown user returns error
	out.Reset()
	err = run(ctx, []string{"admin", "reset-totp", "--username", "nonexistent.user"}, &out, getenv)
	if err == nil {
		t.Fatal("reset-totp for unknown user succeeded, want error")
	}
	out.Reset()
	err = run(ctx, []string{"admin", "reset-password", "--username", "nonexistent.user"}, &out, getenv)
	if err == nil {
		t.Fatal("reset-password for unknown user succeeded, want error")
	}
}

// A35: CLI keys subcommands: rotate-dek inserts new DEK, rewrap updates DEKs with active KEK.
func TestKeysCLI(t *testing.T) {
	t.Parallel()
	db := dbtest.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rawURL := os.Getenv("TEST_DATABASE_URL")
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	parsed.Path = "/" + db.Name
	superuserURL := parsed.String()

	kek1 := make([]byte, 32)
	if _, err := rand.Read(kek1); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	kekFile := filepath.Join(t.TempDir(), "kek")
	if err := os.WriteFile(kekFile, []byte(fmt.Sprintf("k1 %s\n", base64.StdEncoding.EncodeToString(kek1))), 0600); err != nil {
		t.Fatalf("write kek file: %v", err)
	}

	envMap := map[string]string{
		"DATABASE_URL":  superuserURL,
		"KEK_FILE":      kekFile,
		"KEK_ACTIVE_ID": "k1",
		"API_PORT":      "-1",
	}
	getenv := func(key string) string { return envMap[key] }

	// Initialize keyring and seal data with DEK 1
	kr1, err := keyring.Load(ctx, db.App, kekFile, "k1")
	if err != nil {
		t.Fatalf("initial keyring.Load: %v", err)
	}
	pt := []byte("sensitive-keys-cli-data")
	aad := []byte("test.column:1")
	ct, err := kr1.Seal(pt, aad)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// 1. keys rotate-dek creates new DEK wrapped with k1
	var out bytes.Buffer
	err = run(ctx, []string{"keys", "rotate-dek"}, &out, getenv)
	if err != nil {
		t.Fatalf("run keys rotate-dek: %v", err)
	}
	wantRotate := "new data key 2 wrapped with k1\n"
	if out.String() != wantRotate {
		t.Fatalf("rotate-dek output = %q, want %q", out.String(), wantRotate)
	}

	// Keyring loaded fresh still opens earlier ciphertext
	krFresh, err := keyring.Load(ctx, db.App, kekFile, "k1")
	if err != nil {
		t.Fatalf("fresh keyring.Load: %v", err)
	}
	got, err := krFresh.Open(ct, aad)
	if err != nil {
		t.Fatalf("Open after rotate-dek: %v", err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatalf("Open got %q, want %q", got, pt)
	}

	// 2. keys rewrap with k2 rewraps all data keys
	kek2 := make([]byte, 32)
	if _, err := rand.Read(kek2); err != nil {
		t.Fatalf("rand.Read kek2: %v", err)
	}
	kekContent := fmt.Sprintf("k1 %s\nk2 %s\n", base64.StdEncoding.EncodeToString(kek1), base64.StdEncoding.EncodeToString(kek2))
	if err := os.WriteFile(kekFile, []byte(kekContent), 0600); err != nil {
		t.Fatalf("rewrite kek file: %v", err)
	}
	envMap["KEK_ACTIVE_ID"] = "k2"

	out.Reset()
	err = run(ctx, []string{"keys", "rewrap"}, &out, getenv)
	if err != nil {
		t.Fatalf("run keys rewrap: %v", err)
	}
	wantRewrap := "rewrapped 2 data keys with k2\n"
	if out.String() != wantRewrap {
		t.Fatalf("rewrap output = %q, want %q", out.String(), wantRewrap)
	}

	// 3. keyring.Load with a file holding only k2 succeeds and opens earlier ciphertext
	kekFileK2 := filepath.Join(t.TempDir(), "kek_only_k2")
	if err := os.WriteFile(kekFileK2, []byte(fmt.Sprintf("k2 %s\n", base64.StdEncoding.EncodeToString(kek2))), 0600); err != nil {
		t.Fatalf("write kek file k2: %v", err)
	}
	krK2, err := keyring.Load(ctx, db.App, kekFileK2, "k2")
	if err != nil {
		t.Fatalf("Load with only k2: %v", err)
	}
	gotAfterRewrap, err := krK2.Open(ct, aad)
	if err != nil {
		t.Fatalf("Open after rewrap: %v", err)
	}
	if !bytes.Equal(gotAfterRewrap, pt) {
		t.Fatalf("Open got %q, want %q", gotAfterRewrap, pt)
	}
}

// A43: configFromEnv applies defaults when unset, validates values, and parses overrides.
func TestConfigFromEnv(t *testing.T) {
	t.Run("empty env uses defaults", func(t *testing.T) {
		cfg, err := configFromEnv(func(string) string { return "" })
		if err != nil {
			t.Fatalf("configFromEnv(empty): %v", err)
		}
		want := server.DefaultConfig()
		if !reflect.DeepEqual(cfg, want) {
			t.Fatalf("configFromEnv(empty) = %+v, want %+v", cfg, want)
		}
	})

	t.Run("invalid values fail naming variable", func(t *testing.T) {
		cases := []struct {
			name    string
			env     map[string]string
			wantVar string
		}{
			{"invalid pending ttl", map[string]string{"AUTH_PENDING_TTL": "abc"}, "AUTH_PENDING_TTL"},
			{"invalid throttle failures", map[string]string{"AUTH_THROTTLE_FAILURES": "x"}, "AUTH_THROTTLE_FAILURES"},
			{"invalid cookie secure", map[string]string{"AUTH_COOKIE_SECURE": "maybe"}, "AUTH_COOKIE_SECURE"},
			{"zero step up ttl", map[string]string{"AUTH_STEP_UP_TTL": "0s"}, "AUTH_STEP_UP_TTL"},
			{"zero throttle failures", map[string]string{"AUTH_THROTTLE_FAILURES": "0"}, "AUTH_THROTTLE_FAILURES"},
			{"negative idle timeout", map[string]string{"AUTH_SESSION_IDLE_TIMEOUT": "-1m"}, "AUTH_SESSION_IDLE_TIMEOUT"},
			{"negative lifetime", map[string]string{"AUTH_SESSION_LIFETIME": "-12h"}, "AUTH_SESSION_LIFETIME"},
			{"invalid base delay", map[string]string{"AUTH_THROTTLE_BASE_DELAY": "invalid"}, "AUTH_THROTTLE_BASE_DELAY"},
			{"zero max delay", map[string]string{"AUTH_THROTTLE_MAX_DELAY": "0s"}, "AUTH_THROTTLE_MAX_DELAY"},
			{"invalid trust proxy header", map[string]string{"AUTH_TRUST_PROXY_HEADER": "123"}, "AUTH_TRUST_PROXY_HEADER"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := configFromEnv(func(k string) string { return tc.env[k] })
				if err == nil {
					t.Fatalf("%s: expected error, got nil", tc.name)
				}
				if !strings.Contains(err.Error(), tc.wantVar) {
					t.Fatalf("%s: error %q does not contain variable name %q", tc.name, err.Error(), tc.wantVar)
				}
			})
		}
	})

	t.Run("valid overrides parse", func(t *testing.T) {
		env := map[string]string{
			"AUTH_COOKIE_SECURE":        "false",
			"AUTH_SESSION_IDLE_TIMEOUT": "15m",
			"AUTH_SESSION_LIFETIME":     "6h",
			"AUTH_PENDING_TTL":          "5m",
			"AUTH_STEP_UP_TTL":          "5m",
			"AUTH_THROTTLE_FAILURES":    "10",
			"AUTH_THROTTLE_BASE_DELAY":  "5m",
			"AUTH_THROTTLE_MAX_DELAY":   "12h",
			"AUTH_TRUST_PROXY_HEADER":   "true",
		}
		cfg, err := configFromEnv(func(k string) string { return env[k] })
		if err != nil {
			t.Fatalf("configFromEnv(valid): %v", err)
		}
		want := server.Config{
			CookieSecure:       false,
			SessionIdleTimeout: 15 * time.Minute,
			SessionLifetime:    6 * time.Hour,
			PendingTTL:         5 * time.Minute,
			StepUpTTL:          5 * time.Minute,
			ThrottleFailures:   10,
			ThrottleBaseDelay:  5 * time.Minute,
			ThrottleMaxDelay:   12 * time.Hour,
			TrustProxyHeader:   true,
		}
		if !reflect.DeepEqual(cfg, want) {
			t.Fatalf("configFromEnv(valid) = %+v, want %+v", cfg, want)
		}
	})
}
