// Session, TOTP and password handlers of the auth module (docs/modules/auth.md).
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/auth"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/db/dbgen"
)

// Input bounds that the generated server does not enforce.
const (
	maxUsernameInput     = 64
	maxPasswordInput     = 128
	maxRecoveryCodeInput = 32
)

// unauthorized is the 401 body of every operation except createSession.
func unauthorized() api.UnauthorizedApplicationProblemPlusJSONResponse {
	return api.UnauthorizedApplicationProblemPlusJSONResponse(problem(401, detailUnauthorized))
}

// forbidden is the 403 body for a stage or permission failure that a handler detects itself.
func forbidden() api.ForbiddenApplicationProblemPlusJSONResponse {
	return api.ForbiddenApplicationProblemPlusJSONResponse(problem(403, detailForbidden))
}

// logAuth writes one auth event. It never receives passwords, codes, keys or session tokens.
func (s *Server) logAuth(ctx context.Context, level slog.Level, event, username string, userID int64, attrs ...any) {
	ip := ""
	if a := authFrom(ctx); a != nil {
		ip = a.clientIP
	}
	base := []any{"client_ip", ip, "request_id", middleware.GetReqID(ctx)}
	if username != "" {
		base = append(base, "username", username)
	}
	if userID != 0 {
		base = append(base, "user_id", userID)
	}
	s.logger.Log(ctx, level, event, append(base, attrs...)...)
}

// completeLogin runs once per sign-in, when the session first reaches the authenticated stage.
func (s *Server) completeLogin(ctx context.Context, u dbgen.User) error {
	if err := s.throttleClearAccount(ctx, u.Username); err != nil {
		return err
	}
	if err := s.q.SetLastLogin(ctx, dbgen.SetLastLoginParams{At: timestamptz(s.now()), ID: u.ID}); err != nil {
		return fmt.Errorf("set last login: %w", err)
	}
	return nil
}

// totpAAD binds a sealed TOTP secret to its column and row.
func totpAAD(userID int64) []byte {
	return fmt.Appendf(nil, "users.totp_secret:%d", userID)
}

// CreateSession starts a sign-in with username and password.
func (s *Server) CreateSession(ctx context.Context, request api.CreateSessionRequestObject) (api.CreateSessionResponseObject, error) {
	if request.Body == nil {
		return api.CreateSession400ApplicationProblemPlusJSONResponse(problem(400, "")), nil
	}
	ip := authFrom(ctx).clientIP
	username := strings.ToLower(request.Body.Username)
	password := request.Body.Password

	// A malformed username names no account: it counts against the IP key only and skips the lookup.
	wellFormed := utf8.RuneCountInString(request.Body.Username) <= maxUsernameInput && auth.ValidUsername(username)
	keys := throttleKeys{ip: ip}
	if wellFormed {
		keys.account = username
	}

	retry, blocked, err := s.throttleBlocked(ctx, keys)
	if err != nil {
		return nil, err
	}
	if blocked {
		s.logAuth(ctx, slog.LevelWarn, "auth.login.throttled", keys.account, 0, "retry_after", retry)
		return api.CreateSession429ApplicationProblemPlusJSONResponse{TooManyRequestsApplicationProblemPlusJSONResponse: tooMany(retry)}, nil
	}

	var user dbgen.User
	found := false
	if wellFormed {
		user, err = s.q.GetUserByUsername(ctx, username)
		switch {
		case err == nil:
			found = true
		case !errors.Is(err, pgx.ErrNoRows):
			return nil, fmt.Errorf("look up user: %w", err)
		}
	}

	var ok, rehash bool
	switch {
	case utf8.RuneCountInString(password) > maxPasswordInput:
		// Counted failure without hashing: no legitimate password is this long.
	case !found:
		// Same cost as a real check, so timing does not reveal whether the account exists.
		if err := s.hasher.VerifyDummy(ctx, password); err != nil {
			return nil, err
		}
	default:
		ok, rehash, err = s.hasher.Verify(ctx, user.PasswordHash, password)
		if err != nil {
			return nil, err
		}
		// A disabled user gets the same answer as a wrong password, after the same work.
		ok = ok && !user.DisabledAt.Valid
	}

	if !ok {
		retry, blocked, err := s.throttleFailure(ctx, keys)
		if err != nil {
			return nil, err
		}
		s.logAuth(ctx, slog.LevelWarn, "auth.login.failed", keys.account, user.ID)
		if blocked {
			return api.CreateSession429ApplicationProblemPlusJSONResponse{TooManyRequestsApplicationProblemPlusJSONResponse: tooMany(retry)}, nil
		}
		return api.CreateSession401ApplicationProblemPlusJSONResponse(problem(401, detailBadCredentials)), nil
	}

	if rehash {
		newHash, err := s.hasher.Hash(ctx, password)
		if err != nil {
			return nil, err
		}
		if err := s.q.UpdatePasswordHash(ctx, dbgen.UpdatePasswordHashParams{PasswordHash: newHash, ID: user.ID}); err != nil {
			return nil, fmt.Errorf("rehash password: %w", err)
		}
	}
	roles, err := s.userRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// A new session for every sign-in: the old one, if any, is destroyed first and the token is
	// new, so a fixated token never becomes an authenticated one.
	if s.sm.Token(ctx) != "" {
		if err := s.sm.Destroy(ctx); err != nil {
			return nil, fmt.Errorf("destroy previous session: %w", err)
		}
	}
	if err := s.sm.RenewToken(ctx); err != nil {
		return nil, fmt.Errorf("renew session token: %w", err)
	}
	s.sm.Put(ctx, keyUserID, user.ID)
	s.sm.Put(ctx, keyEpoch, int64(user.SessionEpoch))
	s.sm.Remove(ctx, keyStepUpAt)
	stage := resolveStage(user, roles, false)
	s.setStage(ctx, stage)
	if stage == api.Authenticated {
		if err := s.completeLogin(ctx, user); err != nil {
			return nil, err
		}
	}
	s.logAuth(ctx, slog.LevelInfo, "auth.login.succeeded", user.Username, user.ID, "stage", string(stage))
	return api.CreateSession200JSONResponse{Body: s.sessionState(user, roles, stage, time.Time{})}, nil
}

// GetSession returns the state of the current session.
func (s *Server) GetSession(ctx context.Context, _ api.GetSessionRequestObject) (api.GetSessionResponseObject, error) {
	a := authFrom(ctx)
	return api.GetSession200JSONResponse(s.sessionState(a.user, a.roles, a.stage, a.stepUpAt)), nil
}

// DeleteSession ends the current session.
func (s *Server) DeleteSession(ctx context.Context, _ api.DeleteSessionRequestObject) (api.DeleteSessionResponseObject, error) {
	a := authFrom(ctx)
	if err := s.sm.Destroy(ctx); err != nil {
		return nil, fmt.Errorf("destroy session: %w", err)
	}
	s.logAuth(ctx, slog.LevelInfo, "auth.logout", a.user.Username, a.user.ID)
	return api.DeleteSession204Response{}, nil
}

// CreateTotpVerification completes sign-in with an authenticator code or a recovery code.
func (s *Server) CreateTotpVerification(ctx context.Context, request api.CreateTotpVerificationRequestObject) (api.CreateTotpVerificationResponseObject, error) {
	a := authFrom(ctx)
	body := request.Body
	if body == nil || (body.Code == nil) == (body.RecoveryCode == nil) {
		return api.CreateTotpVerification400ApplicationProblemPlusJSONResponse(problem(400, "")), nil
	}
	keys := sessionThrottleKeys(a)
	retry, blocked, err := s.throttleBlocked(ctx, keys)
	if err != nil {
		return nil, err
	}
	if blocked {
		return api.CreateTotpVerification429ApplicationProblemPlusJSONResponse{TooManyRequestsApplicationProblemPlusJSONResponse: tooMany(retry)}, nil
	}

	var ok bool
	if body.Code != nil {
		ok, err = s.claimTotpCode(ctx, a.user, *body.Code)
	} else {
		ok, err = s.useRecoveryCode(ctx, a.user.ID, *body.RecoveryCode)
	}
	if err != nil {
		return nil, err
	}
	if !ok {
		retry, blocked, err := s.throttleFailure(ctx, keys)
		if err != nil {
			return nil, err
		}
		s.logAuth(ctx, slog.LevelWarn, "auth.totp.failed", a.user.Username, a.user.ID)
		if blocked {
			return api.CreateTotpVerification429ApplicationProblemPlusJSONResponse{TooManyRequestsApplicationProblemPlusJSONResponse: tooMany(retry)}, nil
		}
		return api.CreateTotpVerification401ApplicationProblemPlusJSONResponse{UnauthorizedApplicationProblemPlusJSONResponse: unauthorized()}, nil
	}

	stage := resolveStage(a.user, a.roles, true)
	if err := s.renewKeepingDeadline(ctx); err != nil {
		return nil, fmt.Errorf("renew session token: %w", err)
	}
	s.setStage(ctx, stage)
	if stage == api.Authenticated {
		if err := s.completeLogin(ctx, a.user); err != nil {
			return nil, err
		}
	}
	return api.CreateTotpVerification200JSONResponse{Body: s.sessionState(a.user, a.roles, stage, a.stepUpAt)}, nil
}

// claimTotpCode checks a code against the user's confirmed secret and consumes its time step, so the
// same code cannot be used twice.
func (s *Server) claimTotpCode(ctx context.Context, u dbgen.User, code string) (bool, error) {
	secret, err := s.keyringOpenSecret(u)
	if err != nil {
		return false, err
	}
	step, ok := auth.MatchTotp(secret, code, s.now())
	if !ok {
		return false, nil
	}
	_, err = s.q.ClaimTotpStep(ctx, dbgen.ClaimTotpStepParams{Step: pgtype.Int8{Int64: step, Valid: true}, ID: u.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // replay: the step was used already
	}
	if err != nil {
		return false, fmt.Errorf("claim totp step: %w", err)
	}
	return true, nil
}

// keyringOpenSecret decrypts the user's TOTP secret.
func (s *Server) keyringOpenSecret(u dbgen.User) (string, error) {
	plain, err := s.deps.Keyring.Open(u.TotpSecret, totpAAD(u.ID))
	if err != nil {
		return "", fmt.Errorf("open totp secret of user %d: %w", u.ID, err)
	}
	return string(plain), nil
}

// useRecoveryCode checks the input against the unused recovery codes and consumes the match.
func (s *Server) useRecoveryCode(ctx context.Context, userID int64, input string) (bool, error) {
	if utf8.RuneCountInString(input) > maxRecoveryCodeInput {
		return false, nil
	}
	code := auth.NormalizeRecoveryCode(input)
	if !auth.ValidRecoveryCode(code) {
		return false, nil
	}
	stored, err := s.q.ListUnusedRecoveryCodes(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("list recovery codes: %w", err)
	}
	for _, rc := range stored {
		match, _, err := s.hasher.Verify(ctx, rc.CodeHash, code)
		if err != nil {
			return false, err
		}
		if !match {
			continue
		}
		n, err := s.q.UseRecoveryCode(ctx, dbgen.UseRecoveryCodeParams{At: timestamptz(s.now()), ID: rc.ID})
		if err != nil {
			return false, fmt.Errorf("use recovery code: %w", err)
		}
		return n == 1, nil // 0 means a concurrent request used it first
	}
	return false, nil
}

// CreateStepUp re-verifies the session with an authenticator code.
func (s *Server) CreateStepUp(ctx context.Context, request api.CreateStepUpRequestObject) (api.CreateStepUpResponseObject, error) {
	a := authFrom(ctx)
	if request.Body == nil || !auth.ValidTotpCode(request.Body.Code) {
		return api.CreateStepUp400ApplicationProblemPlusJSONResponse(problem(400, "")), nil
	}
	if !a.user.TotpConfirmedAt.Valid {
		return api.CreateStepUp403ApplicationProblemPlusJSONResponse{ForbiddenApplicationProblemPlusJSONResponse: forbidden()}, nil
	}
	keys := sessionThrottleKeys(a)
	retry, blocked, err := s.throttleBlocked(ctx, keys)
	if err != nil {
		return nil, err
	}
	if blocked {
		return api.CreateStepUp429ApplicationProblemPlusJSONResponse{TooManyRequestsApplicationProblemPlusJSONResponse: tooMany(retry)}, nil
	}

	ok, err := s.claimTotpCode(ctx, a.user, request.Body.Code)
	if err != nil {
		return nil, err
	}
	if !ok {
		retry, blocked, err := s.throttleFailure(ctx, keys)
		if err != nil {
			return nil, err
		}
		s.logAuth(ctx, slog.LevelWarn, "auth.stepup.failed", a.user.Username, a.user.ID)
		if blocked {
			return api.CreateStepUp429ApplicationProblemPlusJSONResponse{TooManyRequestsApplicationProblemPlusJSONResponse: tooMany(retry)}, nil
		}
		return api.CreateStepUp401ApplicationProblemPlusJSONResponse{UnauthorizedApplicationProblemPlusJSONResponse: unauthorized()}, nil
	}

	now := s.now()
	s.sm.Put(ctx, keyStepUpAt, now.UnixNano())
	if err := s.renewKeepingDeadline(ctx); err != nil {
		return nil, fmt.Errorf("renew session token: %w", err)
	}
	s.logAuth(ctx, slog.LevelInfo, "auth.stepup.succeeded", a.user.Username, a.user.ID)
	return api.CreateStepUp200JSONResponse{ExpiresAt: now.Add(s.deps.Config.StepUpTTL).UTC()}, nil
}

// UpdatePassword changes the signed-in user's password.
func (s *Server) UpdatePassword(ctx context.Context, request api.UpdatePasswordRequestObject) (api.UpdatePasswordResponseObject, error) {
	a := authFrom(ctx)
	if request.Body == nil {
		return api.UpdatePassword400ApplicationProblemPlusJSONResponse(problem(400, "")), nil
	}
	keys := sessionThrottleKeys(a)
	retry, blocked, err := s.throttleBlocked(ctx, keys)
	if err != nil {
		return nil, err
	}
	if blocked {
		return api.UpdatePassword429ApplicationProblemPlusJSONResponse{TooManyRequestsApplicationProblemPlusJSONResponse: tooMany(retry)}, nil
	}

	var current bool
	if utf8.RuneCountInString(request.Body.CurrentPassword) <= maxPasswordInput {
		if current, _, err = s.hasher.Verify(ctx, a.user.PasswordHash, request.Body.CurrentPassword); err != nil {
			return nil, err
		}
	}
	if !current {
		retry, blocked, err := s.throttleFailure(ctx, keys)
		if err != nil {
			return nil, err
		}
		if blocked {
			return api.UpdatePassword429ApplicationProblemPlusJSONResponse{TooManyRequestsApplicationProblemPlusJSONResponse: tooMany(retry)}, nil
		}
		return api.UpdatePassword400ApplicationProblemPlusJSONResponse(problem(400, detailWrongCurrentPassword)), nil
	}
	if detail := auth.CheckPassword(a.user.Username, request.Body.NewPassword); detail != "" {
		return api.UpdatePassword400ApplicationProblemPlusJSONResponse(problem(400, detail)), nil
	}

	hash, err := s.hasher.Hash(ctx, request.Body.NewPassword)
	if err != nil {
		return nil, err
	}
	epoch, err := s.q.ChangeOwnPassword(ctx, dbgen.ChangeOwnPasswordParams{PasswordHash: hash, ID: a.user.ID})
	if err != nil {
		return nil, fmt.Errorf("change password: %w", err)
	}

	// This session survives with the new epoch; every other session of the user ends.
	updated := a.user
	updated.PasswordChangeRequired = false
	stage := resolveStage(updated, a.roles, true)
	s.sm.Put(ctx, keyEpoch, int64(epoch))
	s.sm.Remove(ctx, keyStepUpAt)
	s.setStage(ctx, stage)
	if err := s.renewKeepingDeadline(ctx); err != nil {
		return nil, fmt.Errorf("renew session token: %w", err)
	}
	if a.stage != api.Authenticated && stage == api.Authenticated {
		if err := s.completeLogin(ctx, updated); err != nil {
			return nil, err
		}
	}
	s.logAuth(ctx, slog.LevelInfo, "auth.password.changed", a.user.Username, a.user.ID)
	return api.UpdatePassword204Response{}, nil
}

// CreateTotpEnrollment starts TOTP enrollment and returns the secret and QR code.
func (s *Server) CreateTotpEnrollment(ctx context.Context, _ api.CreateTotpEnrollmentRequestObject) (api.CreateTotpEnrollmentResponseObject, error) {
	a := authFrom(ctx)
	conflict := api.CreateTotpEnrollment409ApplicationProblemPlusJSONResponse(problem(409, ""))
	if a.user.TotpConfirmedAt.Valid {
		return conflict, nil
	}
	key, err := auth.NewTotpKey(a.user.Username)
	if err != nil {
		return nil, err
	}
	sealed, err := s.deps.Keyring.Seal([]byte(key.Secret), totpAAD(a.user.ID))
	if err != nil {
		return nil, fmt.Errorf("seal totp secret: %w", err)
	}
	// A repeated enrollment before confirmation replaces the pending secret.
	if _, err := s.q.SetPendingTotp(ctx, dbgen.SetPendingTotpParams{TotpSecret: sealed, ID: a.user.ID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return conflict, nil // confirmed by a concurrent request
		}
		return nil, fmt.Errorf("store pending totp secret: %w", err)
	}
	return api.CreateTotpEnrollment200JSONResponse{Secret: key.Secret, OtpauthUri: key.URI, QrCodePng: key.PNG}, nil
}

// UpdateTotpEnrollment confirms TOTP enrollment and returns the recovery codes.
func (s *Server) UpdateTotpEnrollment(ctx context.Context, request api.UpdateTotpEnrollmentRequestObject) (api.UpdateTotpEnrollmentResponseObject, error) {
	a := authFrom(ctx)
	if request.Body == nil {
		return api.UpdateTotpEnrollment400ApplicationProblemPlusJSONResponse(problem(400, "")), nil
	}
	if a.user.TotpConfirmedAt.Valid || len(a.user.TotpSecret) == 0 {
		return api.UpdateTotpEnrollment409ApplicationProblemPlusJSONResponse(problem(409, "")), nil
	}
	invalid := api.UpdateTotpEnrollment400ApplicationProblemPlusJSONResponse(problem(400, detailInvalidCode))
	secret, err := s.keyringOpenSecret(a.user)
	if err != nil {
		return nil, err
	}
	step, ok := auth.MatchTotp(secret, request.Body.Code, s.now())
	if !ok {
		return invalid, nil
	}

	var codes []string
	err = pgx.BeginTxFunc(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		_, err := q.ConfirmTotp(ctx, dbgen.ConfirmTotpParams{At: timestamptz(s.now()), Step: pgtype.Int8{Int64: step, Valid: true}, ID: a.user.ID})
		if err != nil {
			return err
		}
		codes, err = auth.ReplaceRecoveryCodes(ctx, q, s.hasher, a.user.ID)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return invalid, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm totp enrollment: %w", err)
	}

	if a.stage == api.TotpEnrollmentRequired {
		updated := a.user
		updated.TotpConfirmedAt = timestamptz(s.now())
		stage := resolveStage(updated, a.roles, true)
		s.setStage(ctx, stage)
		if err := s.renewKeepingDeadline(ctx); err != nil {
			return nil, fmt.Errorf("renew session token: %w", err)
		}
		if stage == api.Authenticated {
			if err := s.completeLogin(ctx, updated); err != nil {
				return nil, err
			}
		}
	}
	s.logAuth(ctx, slog.LevelInfo, "auth.totp.enrolled", a.user.Username, a.user.ID)
	return api.UpdateTotpEnrollment200JSONResponse{Codes: codes}, nil
}

// CreateRecoveryCodes replaces the recovery codes of the signed-in user.
func (s *Server) CreateRecoveryCodes(ctx context.Context, _ api.CreateRecoveryCodesRequestObject) (api.CreateRecoveryCodesResponseObject, error) {
	a := authFrom(ctx)
	var codes []string
	err := pgx.BeginTxFunc(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		codes, err = auth.ReplaceRecoveryCodes(ctx, s.q.WithTx(tx), s.hasher, a.user.ID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("replace recovery codes: %w", err)
	}
	s.logAuth(ctx, slog.LevelInfo, "auth.recovery_codes.regenerated", a.user.Username, a.user.ID)
	return api.CreateRecoveryCodes200JSONResponse{Codes: codes}, nil
}
