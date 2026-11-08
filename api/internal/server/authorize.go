package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/db/dbgen"
)

// rule describes who may call one operation. An operation with no rule is denied.
type rule struct {
	// public operations skip the session and permission checks.
	public bool
	// stages lists the session stages that may call the operation.
	stages []api.SessionStage
	// perm is the permission required in the authenticated stage, if any. It replaces stages.
	perm *authz.Permission
	// stepUp requires a TOTP re-verification within Config.StepUpTTL.
	stepUp bool
}

var (
	permUsersManage = authz.Permission{Area: authz.Users, Level: authz.Manage}

	allStages = []api.SessionStage{api.Authenticated, api.PasswordChangeRequired, api.TotpRequired, api.TotpEnrollmentRequired}
)

// operationRules maps each generated operation id to its rule (docs/modules/auth.md, "API").
// An operation that is not listed is answered 403.
var operationRules = map[string]rule{
	"GetHealth":     {public: true},
	"CreateSession": {public: true},
	"GetSession":    {stages: allStages},
	"DeleteSession": {stages: allStages},

	"CreateTotpVerification": {stages: []api.SessionStage{api.TotpRequired}},
	"CreateStepUp":           {stages: []api.SessionStage{api.Authenticated}},
	"UpdatePassword":         {stages: []api.SessionStage{api.Authenticated, api.PasswordChangeRequired}},
	"CreateTotpEnrollment":   {stages: []api.SessionStage{api.Authenticated, api.TotpEnrollmentRequired}},
	"UpdateTotpEnrollment":   {stages: []api.SessionStage{api.Authenticated, api.TotpEnrollmentRequired}},
	"CreateRecoveryCodes":    {stages: []api.SessionStage{api.Authenticated}, stepUp: true},

	"ListUsers":           {perm: &permUsersManage},
	"CreateUser":          {perm: &permUsersManage},
	"GetUser":             {perm: &permUsersManage},
	"UpdateUser":          {perm: &permUsersManage},
	"CreatePasswordReset": {perm: &permUsersManage, stepUp: true},
	"DeleteUserTotp":      {perm: &permUsersManage, stepUp: true},
}

// Session keys. Values are int64 or string so that gob needs no type registration.
const (
	keyUserID   = "user_id"
	keyStage    = "stage"
	keyStageAt  = "stage_at" // Unix nanoseconds of the last stage change
	keyEpoch    = "session_epoch"
	keyStepUpAt = "step_up_at" // Unix nanoseconds of the last step-up
)

// requestAuth is what loadPrincipal learned about the request. stage is empty for an anonymous request.
type requestAuth struct {
	clientIP string
	user     dbgen.User
	roles    []authz.Role
	stage    api.SessionStage
	stepUpAt time.Time
}

type authKey struct{}

func authFrom(ctx context.Context) *requestAuth {
	a, _ := ctx.Value(authKey{}).(*requestAuth)
	return a
}

// authorize is the single place that decides who may call an operation (deny by default).
func (s *Server) authorize(rules map[string]rule) api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		rl, known := rules[operationID]
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			if !known {
				return nil, ErrForbidden
			}
			if rl.public {
				return next(ctx, w, r, request)
			}
			a := authFrom(ctx)
			if a == nil || a.stage == "" {
				return nil, ErrUnauthenticated
			}
			if err := s.checkStage(a, rl); err != nil {
				return nil, err
			}
			if rl.stepUp && (a.stepUpAt.IsZero() || s.now().Sub(a.stepUpAt) > s.deps.Config.StepUpTTL) {
				return nil, ErrStepUpRequired
			}
			return next(ctx, w, r, request)
		}
	}
}

// checkStage applies the stage and permission part of a rule.
func (s *Server) checkStage(a *requestAuth, rl rule) error {
	if rl.perm != nil {
		if a.stage != api.Authenticated {
			return ErrUnauthenticated
		}
		if !authz.Can(authz.Principal{UserID: a.user.ID, Roles: a.roles}, *rl.perm) {
			return ErrForbidden
		}
		return nil
	}
	if slices.Contains(rl.stages, a.stage) {
		return nil
	}
	// An operation that needs a fully signed-in user answers a pending session like no session.
	if len(rl.stages) == 1 && rl.stages[0] == api.Authenticated {
		return ErrUnauthenticated
	}
	return ErrForbidden
}

// loadPrincipal reads the user and roles on every request, so a role change or a disabled account
// applies on the next request. A session that is no longer valid is destroyed and the request
// continues as anonymous.
func (s *Server) loadPrincipal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		a := &requestAuth{clientIP: s.clientIP(r)}
		if err := s.fillAuth(ctx, a); err != nil {
			s.logger.ErrorContext(ctx, "load principal", "error", err, "request_id", middleware.GetReqID(ctx))
			writeProblem(w, http.StatusInternalServerError, "")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, authKey{}, a)))
	})
}

func (s *Server) fillAuth(ctx context.Context, a *requestAuth) error {
	uid := s.sm.GetInt64(ctx, keyUserID)
	if uid == 0 {
		return nil
	}
	user, err := s.q.GetUserByID(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.sm.Destroy(ctx)
	}
	if err != nil {
		return err
	}

	stage := api.SessionStage(s.sm.GetString(ctx, keyStage))
	stageAt := time.Unix(0, s.sm.GetInt64(ctx, keyStageAt))
	switch {
	case user.DisabledAt.Valid,
		s.sm.GetInt64(ctx, keyEpoch) != int64(user.SessionEpoch),
		!stage.Valid(),
		stage != api.Authenticated && s.now().Sub(stageAt) > s.deps.Config.PendingTTL:
		return s.sm.Destroy(ctx)
	}

	roles, err := s.userRoles(ctx, user.ID)
	if err != nil {
		return err
	}
	a.user, a.roles, a.stage = user, roles, stage
	if at := s.sm.GetInt64(ctx, keyStepUpAt); at != 0 {
		a.stepUpAt = time.Unix(0, at)
	}
	return nil
}

// userRoles reads the roles of a user, alphabetically.
func (s *Server) userRoles(ctx context.Context, userID int64) ([]authz.Role, error) {
	names, err := s.q.ListUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles := make([]authz.Role, len(names))
	for i, n := range names {
		roles[i] = authz.Role(n)
	}
	return roles, nil
}

// clientIP is X-Real-IP (set by nginx) only when AUTH_TRUST_PROXY_HEADER is on and the value is an
// IP address; otherwise the host part of the connection address.
func (s *Server) clientIP(r *http.Request) string {
	if s.deps.Config.TrustProxyHeader {
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// resolveStage decides the stage after a password check or after a stage was completed
// (docs/modules/auth.md, "Stages"). totpPassed is true once the session has verified TOTP.
func resolveStage(u dbgen.User, roles []authz.Role, totpPassed bool) api.SessionStage {
	switch {
	case u.TotpConfirmedAt.Valid && !totpPassed:
		return api.TotpRequired
	case u.PasswordChangeRequired:
		return api.PasswordChangeRequired
	case !u.TotpConfirmedAt.Valid && slices.ContainsFunc(roles, mandatoryTotp):
		return api.TotpEnrollmentRequired
	default:
		return api.Authenticated
	}
}

// mandatoryTotp reports whether a role must have TOTP enrolled.
func mandatoryTotp(r authz.Role) bool {
	return r == authz.SuperAdmin || r == authz.PpidOfficer
}

// setStage records the stage and when it began, which starts the pending TTL.
func (s *Server) setStage(ctx context.Context, stage api.SessionStage) {
	s.sm.Put(ctx, keyStage, string(stage))
	s.sm.Put(ctx, keyStageAt, s.now().UnixNano())
}

// renewKeepingDeadline rotates the session token without extending the absolute lifetime.
// scs.RenewToken resets the deadline to now+Lifetime, so only createSession may call it plainly.
func (s *Server) renewKeepingDeadline(ctx context.Context) error {
	deadline := s.sm.Deadline(ctx)
	if err := s.sm.RenewToken(ctx); err != nil {
		return err
	}
	s.sm.SetDeadline(ctx, deadline)
	return nil
}

// sessionState builds the SessionState of a user at a stage. Permissions are only granted at the
// authenticated stage; slices are never nil so they encode as [] and not null.
func (s *Server) sessionState(u dbgen.User, roles []authz.Role, stage api.SessionStage, stepUpAt time.Time) api.SessionState {
	perms := []api.Permission{}
	if stage == api.Authenticated {
		for _, p := range authz.Effective(authz.Principal{UserID: u.ID, Roles: roles}) {
			perms = append(perms, api.Permission{Area: api.PermissionArea(p.Area), Level: api.PermissionLevel(p.Level.String())})
		}
	}
	apiRoles := make([]api.Role, len(roles))
	for i, r := range roles {
		apiRoles[i] = api.Role(r)
	}
	var expires *time.Time
	if !stepUpAt.IsZero() {
		if end := stepUpAt.Add(s.deps.Config.StepUpTTL); end.After(s.now()) {
			end = end.UTC()
			expires = &end
		}
	}
	return api.SessionState{
		Permissions:     perms,
		Stage:           stage,
		StepUpExpiresAt: expires,
		TotpEnrolled:    u.TotpConfirmedAt.Valid,
		User:            api.SessionUser{FullName: u.FullName, Id: u.ID, Roles: apiRoles, Username: u.Username},
	}
}
