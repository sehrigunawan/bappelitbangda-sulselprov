package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/auth"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/db/dbgen"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/keyring"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/server"
)

// requireEnv returns a mandatory environment variable or an error naming it.
func requireEnv(getenv func(string) string, name string) (string, error) {
	v := getenv(name)
	if v == "" {
		return "", fmt.Errorf("%s is not set", name)
	}
	return v, nil
}

func openPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}
	return pool, nil
}

// configFromEnv reads the AUTH_* variables into a server.Config. Unset variables keep their defaults;
// an invalid value fails startup with an error naming the variable.
func configFromEnv(getenv func(string) string) (server.Config, error) {
	cfg := server.DefaultConfig()
	durations := []struct {
		name string
		dst  *time.Duration
	}{
		{"AUTH_SESSION_IDLE_TIMEOUT", &cfg.SessionIdleTimeout},
		{"AUTH_SESSION_LIFETIME", &cfg.SessionLifetime},
		{"AUTH_PENDING_TTL", &cfg.PendingTTL},
		{"AUTH_STEP_UP_TTL", &cfg.StepUpTTL},
		{"AUTH_THROTTLE_BASE_DELAY", &cfg.ThrottleBaseDelay},
		{"AUTH_THROTTLE_MAX_DELAY", &cfg.ThrottleMaxDelay},
	}
	for _, d := range durations {
		v := getenv(d.name)
		if v == "" {
			continue
		}
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return server.Config{}, fmt.Errorf("%s: invalid value %q", d.name, v)
		}
		if parsed <= 0 {
			return server.Config{}, fmt.Errorf("%s: must be positive", d.name)
		}
		*d.dst = parsed
	}

	if v := getenv("AUTH_THROTTLE_FAILURES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return server.Config{}, fmt.Errorf("AUTH_THROTTLE_FAILURES: invalid value %q", v)
		}
		if n < 1 {
			return server.Config{}, fmt.Errorf("AUTH_THROTTLE_FAILURES: must be positive")
		}
		cfg.ThrottleFailures = n
	}

	bools := []struct {
		name string
		dst  *bool
	}{
		{"AUTH_COOKIE_SECURE", &cfg.CookieSecure},
		{"AUTH_TRUST_PROXY_HEADER", &cfg.TrustProxyHeader},
	}
	for _, b := range bools {
		v := getenv(b.name)
		if v == "" {
			continue
		}
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return server.Config{}, fmt.Errorf("%s: invalid value %q", b.name, v)
		}
		*b.dst = parsed
	}
	return cfg, nil
}

// newFlagSet returns a flag set that reports errors through its return value only.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// adminCommand implements "api admin create|reset-password|reset-totp". It needs only DATABASE_URL.
func adminCommand(ctx context.Context, args []string, stdout io.Writer, getenv func(string) string) error {
	if len(args) == 0 {
		return errors.New("admin: missing subcommand (want create, reset-password or reset-totp)")
	}
	name := args[0]
	fs := newFlagSet("admin " + name)
	username := fs.String("username", "", "username")
	fullName := fs.String("full-name", "", "full name")
	if err := fs.Parse(args[1:]); err != nil {
		return fmt.Errorf("admin %s: %w", name, err)
	}
	switch name {
	case "create", "reset-password", "reset-totp":
	default:
		return fmt.Errorf("admin: unknown subcommand %q (want create, reset-password or reset-totp)", name)
	}
	if name == "create" && (*username == "" || *fullName == "") {
		return errors.New("admin create: --username and --full-name are required")
	}
	if name != "create" && *username == "" {
		return fmt.Errorf("admin %s: --username is required", name)
	}

	dbURL, err := requireEnv(getenv, "DATABASE_URL")
	if err != nil {
		return err
	}
	pool, err := openPool(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch name {
	case "create":
		if !auth.ValidUsername(*username) {
			return errors.New("admin create: --username must be 3 to 32 characters of a-z, 0-9, '.', '_' and '-'")
		}
		if !auth.ValidFullName(*fullName) {
			return errors.New("admin create: --full-name must be 1 to 100 characters")
		}
		user, temp, err := auth.CreateUser(ctx, pool, auth.NewHasher(), *username, *fullName, []authz.Role{authz.SuperAdmin})
		if errors.Is(err, auth.ErrUsernameTaken) {
			return fmt.Errorf("username %q is taken", *username)
		}
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "created super_admin %s (id %d)\ntemporary password: %s\n", user.Username, user.ID, temp)
		return err
	case "reset-password":
		user, err := userByName(ctx, pool, *username)
		if err != nil {
			return err
		}
		temp, err := auth.ResetPassword(ctx, pool, auth.NewHasher(), user.ID)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "temporary password: %s\n", temp)
		return err
	default: // reset-totp
		user, err := userByName(ctx, pool, *username)
		if err != nil {
			return err
		}
		if err := auth.ResetTotp(ctx, pool, user.ID); err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "TOTP reset for %s\n", user.Username)
		return err
	}
}

func userByName(ctx context.Context, pool *pgxpool.Pool, username string) (dbgen.User, error) {
	user, err := dbgen.New(pool).GetUserByUsername(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.User{}, fmt.Errorf("unknown user %q", username)
	}
	if err != nil {
		return dbgen.User{}, fmt.Errorf("look up user: %w", err)
	}
	return user, nil
}

// keysCommand implements "api keys rotate-dek|rewrap". It needs DATABASE_URL, KEK_FILE and KEK_ACTIVE_ID.
func keysCommand(ctx context.Context, args []string, stdout io.Writer, getenv func(string) string) error {
	if len(args) == 0 {
		return errors.New("keys: missing subcommand (want rotate-dek or rewrap)")
	}
	name := args[0]
	if name != "rotate-dek" && name != "rewrap" {
		return fmt.Errorf("keys: unknown subcommand %q (want rotate-dek or rewrap)", name)
	}
	if err := newFlagSet("keys " + name).Parse(args[1:]); err != nil {
		return fmt.Errorf("keys %s: %w", name, err)
	}

	dbURL, err := requireEnv(getenv, "DATABASE_URL")
	if err != nil {
		return err
	}
	kekFile, err := requireEnv(getenv, "KEK_FILE")
	if err != nil {
		return err
	}
	kekActive, err := requireEnv(getenv, "KEK_ACTIVE_ID")
	if err != nil {
		return err
	}
	pool, err := openPool(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	keys, err := keyring.Load(ctx, pool, kekFile, kekActive)
	if err != nil {
		return err
	}

	if name == "rotate-dek" {
		id, err := keys.RotateDEK(ctx)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "new data key %d wrapped with %s\n", id, kekActive)
		return err
	}
	n, err := keys.Rewrap(ctx)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "rewrapped %d data keys with %s\n", n, kekActive)
	return err
}
