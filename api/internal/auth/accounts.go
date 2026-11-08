package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/authz"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/db/dbgen"
)

// recoveryCodeCount is the number of recovery codes issued per set.
const recoveryCodeCount = 10

// pgUniqueViolation is the SQLSTATE of a unique constraint violation.
const pgUniqueViolation = "23505"

// ErrUsernameTaken is returned when a user with the requested username already exists.
var ErrUsernameTaken = errors.New("username taken")

// ErrUserNotFound is returned when the target user does not exist.
var ErrUserNotFound = errors.New("user not found")

// CreateUser inserts an active user with a temporary password and the given roles. It returns the
// new user and the temporary password, which is shown once.
func CreateUser(ctx context.Context, pool *pgxpool.Pool, h *Hasher, username, fullName string, roles []authz.Role) (dbgen.User, string, error) {
	temp, err := TemporaryPassword()
	if err != nil {
		return dbgen.User{}, "", err
	}
	hash, err := h.Hash(ctx, temp)
	if err != nil {
		return dbgen.User{}, "", err
	}

	var user dbgen.User
	err = pgx.BeginTxFunc(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		var err error
		user, err = q.InsertUser(ctx, dbgen.InsertUserParams{Username: username, FullName: fullName, PasswordHash: hash})
		if err != nil {
			return err
		}
		for _, r := range roles {
			if err := q.InsertUserRole(ctx, dbgen.InsertUserRoleParams{UserID: user.ID, Role: string(r)}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return dbgen.User{}, "", ErrUsernameTaken
		}
		return dbgen.User{}, "", fmt.Errorf("auth: create user: %w", err)
	}
	return user, temp, nil
}

// ResetPassword replaces the password of userID with a temporary one and ends the user's sessions.
// It returns the temporary password.
func ResetPassword(ctx context.Context, pool *pgxpool.Pool, h *Hasher, userID int64) (string, error) {
	temp, err := TemporaryPassword()
	if err != nil {
		return "", err
	}
	hash, err := h.Hash(ctx, temp)
	if err != nil {
		return "", err
	}
	if _, err := dbgen.New(pool).ResetPassword(ctx, dbgen.ResetPasswordParams{PasswordHash: hash, ID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrUserNotFound
		}
		return "", fmt.Errorf("auth: reset password: %w", err)
	}
	return temp, nil
}

// ResetTotp removes the TOTP enrollment and the recovery codes of userID and ends the user's sessions.
func ResetTotp(ctx context.Context, pool *pgxpool.Pool, userID int64) error {
	err := pgx.BeginTxFunc(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if _, err := q.ResetTotp(ctx, userID); err != nil {
			return err
		}
		return q.DeleteRecoveryCodes(ctx, userID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("auth: reset totp: %w", err)
	}
	return nil
}

// ReplaceRecoveryCodes deletes the recovery codes of userID and stores 10 new ones. q must be bound
// to the caller's transaction. It returns the formatted codes, shown to the user once.
func ReplaceRecoveryCodes(ctx context.Context, q *dbgen.Queries, h *Hasher, userID int64) ([]string, error) {
	if err := q.DeleteRecoveryCodes(ctx, userID); err != nil {
		return nil, fmt.Errorf("auth: delete recovery codes: %w", err)
	}
	codes := make([]string, 0, recoveryCodeCount)
	for range recoveryCodeCount {
		raw, err := NewRecoveryCode()
		if err != nil {
			return nil, err
		}
		hash, err := h.Hash(ctx, raw)
		if err != nil {
			return nil, err
		}
		if err := q.InsertRecoveryCode(ctx, dbgen.InsertRecoveryCodeParams{UserID: userID, CodeHash: hash}); err != nil {
			return nil, fmt.Errorf("auth: insert recovery code: %w", err)
		}
		codes = append(codes, FormatRecoveryCode(raw))
	}
	return codes, nil
}
