// Package server holds the HTTP handlers. They implement the interface generated from openapi.yaml
// (internal/api), so the contract and the code cannot drift silently.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/auth"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/db/dbgen"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/keyring"
)

// adminPrefix is the path prefix of every operation that needs sessions.
const adminPrefix = "/api/v1/admin/"

// Session cookie names: the __Host- prefix needs the Secure attribute, so plain-http development
// uses the bare name.
const (
	secureCookieName = "__Host-bpl_session"
	devCookieName    = "bpl_session"
)

// Config holds the auth settings. DefaultConfig gives the values of docs/modules/auth.md.
type Config struct {
	// CookieSecure marks the session cookie Secure and selects the __Host- prefix.
	CookieSecure bool
	// SessionIdleTimeout ends a session after this long without a request.
	SessionIdleTimeout time.Duration
	// SessionLifetime is the absolute session lifetime, not extended by renewals.
	SessionLifetime time.Duration
	// PendingTTL bounds how long a pending (not yet authenticated) stage lasts.
	PendingTTL time.Duration
	// StepUpTTL is how long a TOTP re-verification stays valid.
	StepUpTTL time.Duration
	// ThrottleFailures is the number of failures that starts a block.
	ThrottleFailures int
	// ThrottleBaseDelay is the first block duration.
	ThrottleBaseDelay time.Duration
	// ThrottleMaxDelay caps the block duration.
	ThrottleMaxDelay time.Duration
	// TrustProxyHeader makes the client IP come from X-Real-IP instead of the connection.
	TrustProxyHeader bool
}

// DefaultConfig returns the auth settings of docs/modules/auth.md.
func DefaultConfig() Config {
	return Config{
		CookieSecure:       true,
		SessionIdleTimeout: 30 * time.Minute,
		SessionLifetime:    12 * time.Hour,
		PendingTTL:         10 * time.Minute,
		StepUpTTL:          10 * time.Minute,
		ThrottleFailures:   5,
		ThrottleBaseDelay:  15 * time.Minute,
		ThrottleMaxDelay:   24 * time.Hour,
		TrustProxyHeader:   false,
	}
}

// Deps are the collaborators of the handlers. Pool and Keyring are nil when no database is configured.
type Deps struct {
	// Pool is the database pool, used as role app.
	Pool *pgxpool.Pool
	// Keyring seals TOTP secrets. It is required when Pool is set.
	Keyring *keyring.Keyring
	// Config holds the auth settings.
	Config Config
	// Now returns the current time. Tests replace it with a fake clock.
	Now func() time.Time
}

// Server implements api.StrictServerInterface.
type Server struct {
	deps   Deps
	logger *slog.Logger
	// The fields below are nil when Deps.Pool is nil.
	q      *dbgen.Queries
	sm     *scs.SessionManager
	hasher *auth.Hasher
	cop    *http.CrossOriginProtection
}

var _ api.StrictServerInterface = (*Server)(nil)

// GetHealth reports liveness. It deliberately does not touch the database.
func (*Server) GetHealth(_ context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{Status: api.Ok}, nil
}

// now returns the injected clock, which drives throttling, pending TTL and step-up.
func (s *Server) now() time.Time { return s.deps.Now() }

// NewHandler builds the router with the generated routes and base middleware. Errors returned by
// handlers are mapped to problem+json (errors.go) and logged to logger.
func NewHandler(logger *slog.Logger, deps Deps) http.Handler {
	return newHandler(logger, deps, operationRules)
}

// newHandler is NewHandler with the operation rules injected, so tests can drop a rule.
func newHandler(logger *slog.Logger, deps Deps, rules map[string]rule) http.Handler {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Pool != nil && deps.Keyring == nil {
		// Construction-time programming error, not a request-time failure.
		panic("server: Deps.Keyring is required when Deps.Pool is set")
	}
	s := &Server{deps: deps, logger: logger}
	if deps.Pool != nil {
		s.q = dbgen.New(deps.Pool)
		s.hasher = auth.NewHasher()
		s.sm = newSessionManager(logger, deps)
		s.cop = http.NewCrossOriginProtection()
		s.cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeProblem(w, http.StatusForbidden, detailCrossOrigin)
		}))
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(s.adminOnly)
	strict := api.NewStrictHandlerWithOptions(s, []api.StrictMiddlewareFunc{s.authorize(rules)}, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  requestErrorHandler,
		ResponseErrorHandlerFunc: responseErrorHandler(logger),
	})
	// ErrorHandlerFunc handles query/path parameter binding errors, which the strict options above do not see.
	return api.HandlerWithOptions(strict, api.ChiServerOptions{BaseRouter: r, ErrorHandlerFunc: requestErrorHandler})
}

// newSessionManager configures scs with the pgxstore session table.
func newSessionManager(logger *slog.Logger, deps Deps) *scs.SessionManager {
	cfg := deps.Config
	sm := scs.New()
	sm.Store = pgxstore.New(deps.Pool)
	sm.IdleTimeout = cfg.SessionIdleTimeout
	sm.Lifetime = cfg.SessionLifetime
	sm.Cookie.Name = devCookieName
	if cfg.CookieSecure {
		sm.Cookie.Name = secureCookieName
	}
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = http.SameSiteStrictMode
	sm.Cookie.Path = "/"
	sm.Cookie.Secure = cfg.CookieSecure
	sm.Cookie.Persist = true
	sm.ErrorFunc = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.ErrorContext(r.Context(), "session store error", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeProblem(w, http.StatusInternalServerError, "")
	}
	return sm
}

// adminOnly wraps the operations under /api/v1/admin/ with, outermost first: no-store, cross-origin
// protection, the session loader and loadPrincipal. Other paths (health) never see sessions, so
// they stay free of database access even when a cookie is sent. Without a pool there are no
// sessions, so the admin operations do not exist.
func (s *Server) adminOnly(next http.Handler) http.Handler {
	if s.sm == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, adminPrefix) {
				writeProblem(w, http.StatusNotFound, "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	admin := noStore(s.cop.Handler(s.sm.LoadAndSave(s.loadPrincipal(next))))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, adminPrefix) {
			admin.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// noStore keeps admin responses out of every cache.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
