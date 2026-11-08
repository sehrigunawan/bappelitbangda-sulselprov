// User administration handlers of the auth module (docs/modules/auth.md). Authorization (permission
// users.manage, step-up where required) is decided by the authorize middleware before these run.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/auth"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/db/dbgen"
)

// Paging bounds of listUsers.
const (
	defaultPage     = 1
	defaultPageSize = 10
	maxPageSize     = 50
	maxPage         = 1 << 20 // keeps (page-1)*pageSize inside int32
)

// Errors that end the updateUser transaction with a typed response.
var (
	errUserNotFound   = errors.New("user not found")
	errLastSuperAdmin = errors.New("last active super admin")
)

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

// toAPIUser maps user columns to the API model. Roles is never nil.
func toAPIUser(id int64, username, fullName string, roles []string, disabledAt, totpConfirmedAt, lastLoginAt, createdAt pgtype.Timestamptz) api.User {
	apiRoles := make([]api.Role, len(roles))
	for i, r := range roles {
		apiRoles[i] = api.Role(r)
	}
	return api.User{
		Active:       !disabledAt.Valid,
		CreatedAt:    createdAt.Time.UTC(),
		FullName:     fullName,
		Id:           id,
		LastLoginAt:  timePtr(lastLoginAt),
		Roles:        apiRoles,
		TotpEnrolled: totpConfirmedAt.Valid,
		Username:     username,
	}
}

func userFromRow(u dbgen.User, roles []authz.Role) api.User {
	names := make([]string, len(roles))
	for i, r := range roles {
		names[i] = string(r)
	}
	return toAPIUser(u.ID, u.Username, u.FullName, names, u.DisabledAt, u.TotpConfirmedAt, u.LastLoginAt, u.CreatedAt)
}

// validRoles checks the roles of a request body: at least one, each known, none repeated.
func validRoles(in []api.Role) ([]authz.Role, bool) {
	if len(in) == 0 {
		return nil, false
	}
	out := make([]authz.Role, 0, len(in))
	for _, r := range in {
		if !r.Valid() || slices.Contains(out, authz.Role(r)) {
			return nil, false
		}
		out = append(out, authz.Role(r))
	}
	return out, true
}

func invalidUserID() api.InvalidUserIdApplicationProblemPlusJSONResponse {
	return api.InvalidUserIdApplicationProblemPlusJSONResponse(problem(400, ""))
}

func userNotFound() api.UserNotFoundApplicationProblemPlusJSONResponse {
	return api.UserNotFoundApplicationProblemPlusJSONResponse(problem(404, ""))
}

func ownAccount() api.OwnAccountApplicationProblemPlusJSONResponse {
	return api.OwnAccountApplicationProblemPlusJSONResponse(problem(409, ""))
}

// ListUsers returns one page of users, ordered by username.
func (s *Server) ListUsers(ctx context.Context, request api.ListUsersRequestObject) (api.ListUsersResponseObject, error) {
	page, pageSize := defaultPage, defaultPageSize
	if request.Params.Page != nil {
		page = *request.Params.Page
	}
	if request.Params.PageSize != nil {
		pageSize = *request.Params.PageSize
	}
	if page < 1 || pageSize < 1 || pageSize > maxPageSize {
		return api.ListUsers400ApplicationProblemPlusJSONResponse(problem(400, "")), nil
	}

	total, err := s.q.CountUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	// A page beyond maxPage is past the end of any real table: answer an empty page without
	// building an offset that could overflow int32. Below maxPage, both values stay far below that limit.
	var rows []dbgen.ListUsersPageRow
	if page <= maxPage {
		rows, err = s.q.ListUsersPage(ctx, dbgen.ListUsersPageParams{PageLimit: int32(pageSize), PageOffset: int32((page - 1) * pageSize)}) //nolint:gosec // bounded above
		if err != nil {
			return nil, fmt.Errorf("list users: %w", err)
		}
	}
	items := make([]api.User, len(rows))
	for i, row := range rows {
		items[i] = toAPIUser(row.ID, row.Username, row.FullName, row.Roles, row.DisabledAt, row.TotpConfirmedAt, row.LastLoginAt, row.CreatedAt)
	}
	return api.ListUsers200JSONResponse{Items: items, Page: page, PageSize: pageSize, Total: int(total)}, nil
}

// CreateUser creates a user with a temporary password.
func (s *Server) CreateUser(ctx context.Context, request api.CreateUserRequestObject) (api.CreateUserResponseObject, error) {
	badRequest := api.CreateUser400ApplicationProblemPlusJSONResponse(problem(400, ""))
	if request.Body == nil {
		return badRequest, nil
	}
	roles, ok := validRoles(request.Body.Roles)
	if !ok || !auth.ValidUsername(request.Body.Username) || !auth.ValidFullName(request.Body.FullName) {
		return badRequest, nil
	}

	user, temp, err := auth.CreateUser(ctx, s.deps.Pool, s.hasher, request.Body.Username, request.Body.FullName, roles)
	if errors.Is(err, auth.ErrUsernameTaken) {
		return api.CreateUser409ApplicationProblemPlusJSONResponse(problem(409, detailUsernameTaken)), nil
	}
	if err != nil {
		return nil, err
	}
	stored, err := s.userRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	s.logAdminAction(ctx, "auth.user.created", user.Username, user.ID)
	return api.CreateUser201JSONResponse{User: userFromRow(user, stored), TemporaryPassword: temp}, nil
}

// GetUser returns one user.
func (s *Server) GetUser(ctx context.Context, request api.GetUserRequestObject) (api.GetUserResponseObject, error) {
	if request.UserId < 1 {
		return api.GetUser400ApplicationProblemPlusJSONResponse{InvalidUserIdApplicationProblemPlusJSONResponse: invalidUserID()}, nil
	}
	user, err := s.q.GetUserByID(ctx, request.UserId)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.GetUser404ApplicationProblemPlusJSONResponse{UserNotFoundApplicationProblemPlusJSONResponse: userNotFound()}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	roles, err := s.userRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return api.GetUser200JSONResponse(userFromRow(user, roles)), nil
}

// UpdateUser changes the name, roles and active flag of a user.
func (s *Server) UpdateUser(ctx context.Context, request api.UpdateUserRequestObject) (api.UpdateUserResponseObject, error) {
	if request.UserId < 1 {
		return api.UpdateUser400ApplicationProblemPlusJSONResponse(problem(400, "")), nil
	}
	badRequest := api.UpdateUser400ApplicationProblemPlusJSONResponse(problem(400, ""))
	if request.Body == nil {
		return badRequest, nil
	}
	roles, ok := validRoles(request.Body.Roles)
	if !ok || !auth.ValidFullName(request.Body.FullName) {
		return badRequest, nil
	}
	conflict := api.UpdateUser409ApplicationProblemPlusJSONResponse(problem(409, ""))
	self := authFrom(ctx).user.ID == request.UserId
	if self && (!request.Body.Active || !slices.Contains(roles, authz.SuperAdmin)) {
		return conflict, nil
	}

	var updated dbgen.User
	err := pgx.BeginTxFunc(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		// Lock every active super admin, then the target, so that two concurrent demotions
		// serialize and cannot both see "another super admin remains".
		if _, err := q.LockActiveSuperAdmins(ctx); err != nil {
			return err
		}
		if _, err := q.LockUser(ctx, request.UserId); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errUserNotFound
			}
			return err
		}
		target, err := q.GetUserByID(ctx, request.UserId)
		if err != nil {
			return err
		}
		current, err := q.ListUserRoles(ctx, request.UserId)
		if err != nil {
			return err
		}
		activeSuperAdmin := !target.DisabledAt.Valid && slices.Contains(current, string(authz.SuperAdmin))
		staysSuperAdmin := request.Body.Active && slices.Contains(roles, authz.SuperAdmin)
		if activeSuperAdmin && !staysSuperAdmin {
			n, err := q.CountActiveSuperAdmins(ctx)
			if err != nil {
				return err
			}
			if n <= 1 {
				return errLastSuperAdmin
			}
		}

		updated, err = q.UpdateUserProfile(ctx, dbgen.UpdateUserProfileParams{FullName: request.Body.FullName, Active: request.Body.Active, ID: request.UserId})
		if err != nil {
			return err
		}
		if err := q.DeleteUserRoles(ctx, request.UserId); err != nil {
			return err
		}
		for _, r := range roles {
			if err := q.InsertUserRole(ctx, dbgen.InsertUserRoleParams{UserID: request.UserId, Role: string(r)}); err != nil {
				return err
			}
		}
		return nil
	})
	switch {
	case errors.Is(err, errUserNotFound):
		return api.UpdateUser404ApplicationProblemPlusJSONResponse{UserNotFoundApplicationProblemPlusJSONResponse: userNotFound()}, nil
	case errors.Is(err, errLastSuperAdmin):
		return conflict, nil
	case err != nil:
		return nil, fmt.Errorf("update user: %w", err)
	}

	stored, err := s.userRoles(ctx, updated.ID)
	if err != nil {
		return nil, err
	}
	s.logAdminAction(ctx, "auth.user.updated", updated.Username, updated.ID)
	return api.UpdateUser200JSONResponse(userFromRow(updated, stored)), nil
}

// CreatePasswordReset gives a user a new temporary password and ends their sessions.
func (s *Server) CreatePasswordReset(ctx context.Context, request api.CreatePasswordResetRequestObject) (api.CreatePasswordResetResponseObject, error) {
	if request.UserId < 1 {
		return api.CreatePasswordReset400ApplicationProblemPlusJSONResponse{InvalidUserIdApplicationProblemPlusJSONResponse: invalidUserID()}, nil
	}
	if request.UserId == authFrom(ctx).user.ID {
		return api.CreatePasswordReset409ApplicationProblemPlusJSONResponse{OwnAccountApplicationProblemPlusJSONResponse: ownAccount()}, nil
	}
	temp, err := auth.ResetPassword(ctx, s.deps.Pool, s.hasher, request.UserId)
	if errors.Is(err, auth.ErrUserNotFound) {
		return api.CreatePasswordReset404ApplicationProblemPlusJSONResponse{UserNotFoundApplicationProblemPlusJSONResponse: userNotFound()}, nil
	}
	if err != nil {
		return nil, err
	}
	s.logAdminAction(ctx, "auth.password.reset", "", request.UserId)
	return api.CreatePasswordReset200JSONResponse{TemporaryPassword: temp}, nil
}

// DeleteUserTotp removes the TOTP enrollment of a user and ends their sessions.
func (s *Server) DeleteUserTotp(ctx context.Context, request api.DeleteUserTotpRequestObject) (api.DeleteUserTotpResponseObject, error) {
	if request.UserId < 1 {
		return api.DeleteUserTotp400ApplicationProblemPlusJSONResponse{InvalidUserIdApplicationProblemPlusJSONResponse: invalidUserID()}, nil
	}
	if request.UserId == authFrom(ctx).user.ID {
		return api.DeleteUserTotp409ApplicationProblemPlusJSONResponse{OwnAccountApplicationProblemPlusJSONResponse: ownAccount()}, nil
	}
	err := auth.ResetTotp(ctx, s.deps.Pool, request.UserId)
	if errors.Is(err, auth.ErrUserNotFound) {
		return api.DeleteUserTotp404ApplicationProblemPlusJSONResponse{UserNotFoundApplicationProblemPlusJSONResponse: userNotFound()}, nil
	}
	if err != nil {
		return nil, err
	}
	s.logAdminAction(ctx, "auth.totp.reset", "", request.UserId)
	return api.DeleteUserTotp204Response{}, nil
}

// logAdminAction logs a user-administration event. The target is the affected user; the actor is
// the signed-in administrator.
func (s *Server) logAdminAction(ctx context.Context, event, targetUsername string, targetID int64) {
	actor := authFrom(ctx).user
	s.logAuth(ctx, slog.LevelInfo, event, targetUsername, targetID, "actor_id", actor.ID, "actor", actor.Username)
}
