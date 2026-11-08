// Contract-PR stubs for the auth module (docs/modules/auth.md). Each answers 501 through
// ErrNotImplemented until the BE lane implements it.
package server

import (
	"context"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
)

func (Server) CreateSession(_ context.Context, _ api.CreateSessionRequestObject) (api.CreateSessionResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) GetSession(_ context.Context, _ api.GetSessionRequestObject) (api.GetSessionResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) DeleteSession(_ context.Context, _ api.DeleteSessionRequestObject) (api.DeleteSessionResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) CreateTotpVerification(_ context.Context, _ api.CreateTotpVerificationRequestObject) (api.CreateTotpVerificationResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) CreateStepUp(_ context.Context, _ api.CreateStepUpRequestObject) (api.CreateStepUpResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) UpdatePassword(_ context.Context, _ api.UpdatePasswordRequestObject) (api.UpdatePasswordResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) CreateTotpEnrollment(_ context.Context, _ api.CreateTotpEnrollmentRequestObject) (api.CreateTotpEnrollmentResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) UpdateTotpEnrollment(_ context.Context, _ api.UpdateTotpEnrollmentRequestObject) (api.UpdateTotpEnrollmentResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) CreateRecoveryCodes(_ context.Context, _ api.CreateRecoveryCodesRequestObject) (api.CreateRecoveryCodesResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) ListUsers(_ context.Context, _ api.ListUsersRequestObject) (api.ListUsersResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) CreateUser(_ context.Context, _ api.CreateUserRequestObject) (api.CreateUserResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) GetUser(_ context.Context, _ api.GetUserRequestObject) (api.GetUserResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) UpdateUser(_ context.Context, _ api.UpdateUserRequestObject) (api.UpdateUserResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) CreatePasswordReset(_ context.Context, _ api.CreatePasswordResetRequestObject) (api.CreatePasswordResetResponseObject, error) {
	return nil, ErrNotImplemented
}

func (Server) DeleteUserTotp(_ context.Context, _ api.DeleteUserTotpRequestObject) (api.DeleteUserTotpResponseObject, error) {
	return nil, ErrNotImplemented
}
