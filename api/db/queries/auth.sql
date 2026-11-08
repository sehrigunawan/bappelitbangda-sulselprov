-- auth module queries (docs/modules/auth.md). Every UPDATE on users sets updated_at.
-- Time values that drive throttling and TOTP come from the caller (Deps.Now), not from now().

-- ---- users ----

-- name: GetUserByID :one
SELECT * FROM users WHERE id = @id;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = @username;

-- name: ListUserRoles :many
SELECT role FROM user_roles WHERE user_id = @user_id ORDER BY role;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: ListUsersPage :many
SELECT u.id, u.username, u.full_name, u.disabled_at, u.totp_confirmed_at, u.last_login_at, u.created_at,
       coalesce((SELECT array_agg(r.role ORDER BY r.role) FROM user_roles r WHERE r.user_id = u.id), '{}')::text[] AS roles
FROM users u
ORDER BY u.username
LIMIT @page_limit OFFSET @page_offset;

-- name: InsertUser :one
INSERT INTO users (username, full_name, password_hash)
VALUES (@username, @full_name, @password_hash)
RETURNING *;

-- name: InsertUserRole :exec
INSERT INTO user_roles (user_id, role) VALUES (@user_id, @role);

-- name: DeleteUserRoles :exec
DELETE FROM user_roles WHERE user_id = @user_id;

-- Deactivating a user ends their sessions (session_epoch + 1), once; reactivating clears disabled_at.
-- name: UpdateUserProfile :one
UPDATE users SET
    full_name     = @full_name,
    disabled_at   = CASE WHEN @active::boolean THEN NULL ELSE coalesce(disabled_at, now()) END,
    session_epoch = session_epoch + CASE WHEN NOT @active::boolean AND disabled_at IS NULL THEN 1 ELSE 0 END,
    updated_at    = now()
WHERE id = @id
RETURNING *;

-- name: LockUser :one
SELECT id FROM users WHERE id = @id FOR UPDATE;

-- name: LockActiveSuperAdmins :many
SELECT u.id FROM users u
JOIN user_roles r ON r.user_id = u.id
WHERE r.role = 'super_admin' AND u.disabled_at IS NULL
ORDER BY u.id
FOR UPDATE OF u;

-- name: CountActiveSuperAdmins :one
SELECT count(*) FROM users u
JOIN user_roles r ON r.user_id = u.id
WHERE r.role = 'super_admin' AND u.disabled_at IS NULL;

-- name: UpdatePasswordHash :exec
UPDATE users SET password_hash = @password_hash, updated_at = now() WHERE id = @id;

-- name: ChangeOwnPassword :one
UPDATE users SET
    password_hash            = @password_hash,
    password_change_required = false,
    session_epoch            = session_epoch + 1,
    updated_at               = now()
WHERE id = @id
RETURNING session_epoch;

-- name: ResetPassword :one
UPDATE users SET
    password_hash            = @password_hash,
    password_change_required = true,
    session_epoch            = session_epoch + 1,
    updated_at               = now()
WHERE id = @id
RETURNING id;

-- name: SetLastLogin :exec
UPDATE users SET last_login_at = @at, updated_at = now() WHERE id = @id;

-- A repeated enrollment before confirmation overwrites the pending secret; a confirmed one is never replaced.
-- name: SetPendingTotp :one
UPDATE users SET
    totp_secret       = @totp_secret,
    totp_confirmed_at = NULL,
    totp_last_step    = NULL,
    updated_at        = now()
WHERE id = @id AND totp_confirmed_at IS NULL
RETURNING id;

-- name: ConfirmTotp :one
UPDATE users SET
    totp_confirmed_at = @at,
    totp_last_step    = @step,
    updated_at        = now()
WHERE id = @id
  AND totp_secret IS NOT NULL
  AND totp_confirmed_at IS NULL
  AND (totp_last_step IS NULL OR totp_last_step < @step)
RETURNING id;

-- Accepts a TOTP time step once: two concurrent requests with the same code cannot both pass.
-- name: ClaimTotpStep :one
UPDATE users SET totp_last_step = @step, updated_at = now()
WHERE id = @id
  AND totp_confirmed_at IS NOT NULL
  AND (totp_last_step IS NULL OR totp_last_step < @step)
RETURNING id;

-- name: ResetTotp :one
UPDATE users SET
    totp_secret       = NULL,
    totp_confirmed_at = NULL,
    totp_last_step    = NULL,
    session_epoch     = session_epoch + 1,
    updated_at        = now()
WHERE id = @id
RETURNING id;

-- ---- recovery codes ----

-- name: DeleteRecoveryCodes :exec
DELETE FROM recovery_codes WHERE user_id = @user_id;

-- name: InsertRecoveryCode :exec
INSERT INTO recovery_codes (user_id, code_hash) VALUES (@user_id, @code_hash);

-- name: ListUnusedRecoveryCodes :many
SELECT id, code_hash FROM recovery_codes WHERE user_id = @user_id AND used_at IS NULL ORDER BY id;

-- name: UseRecoveryCode :execrows
UPDATE recovery_codes SET used_at = @at WHERE id = @id AND used_at IS NULL;

-- ---- login throttle ----

-- The latest block deadline of the two keys that is still in the future (NULL when not blocked).
-- name: GetThrottleBlock :one
SELECT max(blocked_until)::timestamptz AS blocked_until
FROM login_throttle
WHERE ((scope = 'account' AND key = @account_key::text) OR (scope = 'ip' AND key = @ip_key::text))
  AND blocked_until > @now::timestamptz;

-- The counter restarts at 1 when the previous failure is older than decay_before and the key is not blocked.
-- name: RecordThrottleFailure :one
INSERT INTO login_throttle (scope, key, failures, last_failure_at)
VALUES (@scope::text, @key::text, 1, @now::timestamptz)
ON CONFLICT (scope, key) DO UPDATE SET
    failures = CASE
        WHEN login_throttle.last_failure_at < @decay_before::timestamptz
         AND (login_throttle.blocked_until IS NULL OR login_throttle.blocked_until <= @now::timestamptz)
        THEN 1
        ELSE login_throttle.failures + 1
    END,
    last_failure_at = @now::timestamptz
RETURNING failures;

-- name: SetThrottleBlock :exec
UPDATE login_throttle SET blocked_until = @blocked_until WHERE scope = @scope AND key = @key;

-- name: DeleteAccountThrottle :exec
DELETE FROM login_throttle WHERE scope = 'account' AND key = @key;

-- ---- data keys ----

-- name: ListDataKeys :many
SELECT id, kek_id, wrapped_key FROM data_keys ORDER BY id;

-- name: CountDataKeys :one
SELECT count(*) FROM data_keys;

-- name: InsertDataKey :one
INSERT INTO data_keys (kek_id, wrapped_key) VALUES (@kek_id, @wrapped_key) RETURNING id;

-- name: UpdateDataKeyWrap :exec
UPDATE data_keys SET kek_id = @kek_id, wrapped_key = @wrapped_key WHERE id = @id;

-- Serialises the creation of the first data key between the server and the CLI (held until the transaction ends).
-- name: LockDataKeys :exec
SELECT pg_advisory_xact_lock(1801812338);
