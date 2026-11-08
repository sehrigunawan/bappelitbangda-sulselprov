// Package dbtest gives each DB-backed test its own database, cloned from the
// migrated template that scripts/db-throwaway.sh creates (TEST_TEMPLATE_DB).
package dbtest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is one cloned database. App connects as role app, the same role the API
// uses in production, so missing grants fail here instead of in deploy.
type DB struct {
	Name   string
	AppURL string        // postgres URL as role app (fake dev password "app", api/db/init/00_roles_and_database.sql)
	App    *pgxpool.Pool // role app: what the API uses
	Super  *pgxpool.Pool // superuser: seeding and inspection only
}

// cloneLockKey serialises CREATE DATABASE ... TEMPLATE across packages, because
// Postgres refuses to copy a template while another session is using it.
const cloneLockKey = 4242

// cloneAttempts and cloneRetryDelay bound the retry when the template is busy.
const (
	cloneAttempts   = 5
	cloneRetryDelay = 200 * time.Millisecond
)

var counter atomic.Int64

// New clones the template into a fresh database that is dropped when t ends.
func New(t testing.TB) *DB {
	t.Helper()
	if testing.Short() {
		t.Skip("needs PostgreSQL; skipped under -short")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	tpl := os.Getenv("TEST_TEMPLATE_DB")
	if dsn == "" || tpl == "" {
		t.Skip("TEST_TEMPLATE_DB not set; run make test-full")
	}

	base, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("dbtest: parse TEST_DATABASE_URL: %v", err)
	}
	adminURL := withDatabase(base, "postgres", nil)

	clone := fmt.Sprintf("%s_c%d_%d", tpl, os.Getpid(), counter.Add(1))
	createClone(t, adminURL, clone, tpl)

	appURL := withDatabase(base, clone, url.UserPassword("app", "app"))
	superURL := withDatabase(base, clone, base.User)

	ctx := context.Background()
	app, err := pgxpool.New(ctx, appURL.String())
	if err != nil {
		dropClone(t, adminURL, clone)
		t.Fatalf("dbtest: connect app pool: %v", err)
	}
	super, err := pgxpool.New(ctx, superURL.String())
	if err != nil {
		app.Close()
		dropClone(t, adminURL, clone)
		t.Fatalf("dbtest: connect superuser pool: %v", err)
	}

	t.Cleanup(func() {
		app.Close()
		super.Close()
		dropClone(t, adminURL, clone)
	})

	return &DB{Name: clone, AppURL: appURL.String(), App: app, Super: super}
}

// AppConfig returns a fresh parsed pool config for the app role, so a test can
// attach a tracer before it creates its own pool.
func (d *DB) AppConfig(t testing.TB) *pgxpool.Config {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(d.AppURL)
	if err != nil {
		t.Fatalf("dbtest: parse app config: %v", err)
	}
	return cfg
}

// withDatabase copies base, replaces the path with the database name and
// optionally replaces the userinfo.
func withDatabase(base *url.URL, name string, user *url.Userinfo) *url.URL {
	u := *base
	u.Path = "/" + name
	u.RawPath = ""
	if user != nil {
		u.User = user
	}
	return &u
}

func createClone(t testing.TB, adminURL *url.URL, clone, tpl string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminURL.String())
	if err != nil {
		t.Fatalf("dbtest: connect admin: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", cloneLockKey); err != nil {
		t.Fatalf("dbtest: take clone lock: %v", err)
	}
	defer func() {
		if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", cloneLockKey); err != nil {
			t.Errorf("dbtest: release clone lock: %v", err)
		}
	}()

	stmt := fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s",
		pgx.Identifier{clone}.Sanitize(), pgx.Identifier{tpl}.Sanitize())

	for attempt := 1; ; attempt++ {
		_, err = conn.Exec(ctx, stmt)
		if err == nil {
			return
		}
		if !strings.Contains(err.Error(), "being accessed by other users") || attempt >= cloneAttempts {
			t.Fatalf("dbtest: create clone %s: %v", clone, err)
		}
		time.Sleep(cloneRetryDelay)
	}
}

func dropClone(t testing.TB, adminURL *url.URL, clone string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminURL.String())
	if err != nil {
		t.Errorf("dbtest: connect admin for drop: %v", err)
		return
	}
	defer func() { _ = conn.Close(ctx) }()

	stmt := fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", pgx.Identifier{clone}.Sanitize())
	if _, err := conn.Exec(ctx, stmt); err != nil {
		t.Errorf("dbtest: drop clone %s: %v", clone, err)
	}
}
