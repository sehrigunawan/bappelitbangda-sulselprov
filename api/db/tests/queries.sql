-- One case per query in api/db/queries/. Keep in step with the sqlc queries.
-- Ping
DO $$
BEGIN
  ASSERT (SELECT 1::int AS ok) = 1, 'Ping must return 1';
END
$$;

-- Auth queries (D5). Each block truncates the auth tables, seeds its own rows and asserts the returned or affected values.
-- GetUserByID
DO $$
DECLARE v_id bigint; v_name text;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.one', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  SELECT username INTO v_name FROM users WHERE id = v_id;
  ASSERT v_name = 'q.one', 'GetUserByID must return the seeded user by id';
END
$$;

-- GetUserByUsername
DO $$
DECLARE v_id bigint; v_found int;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.two', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  SELECT count(*) INTO v_found FROM users WHERE username = 'q.two';
  ASSERT v_found = 1, 'GetUserByUsername must find the seeded user';
  SELECT count(*) INTO v_found FROM users WHERE username = 'q.missing';
  ASSERT v_found = 0, 'GetUserByUsername must return no rows for an unknown username';
END
$$;

-- ListUserRoles
DO $$
DECLARE v_id bigint; v_roles text;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.three', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  INSERT INTO user_roles (user_id, role) VALUES (v_id, 'humas_editor'), (v_id, 'document_operator');
  SELECT string_agg(role, ',' ORDER BY role) INTO v_roles FROM (
    SELECT role FROM user_roles WHERE user_id = v_id ORDER BY role) AS r;
  ASSERT v_roles = 'document_operator,humas_editor', 'ListUserRoles must return the roles in alphabetical order';
END
$$;

-- CountUsers
DO $$
DECLARE v_n bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.a1', 'Pengguna Uji', 'hash-uji'), ('q.a2', 'Pengguna Uji', 'hash-uji'), ('q.a3', 'Pengguna Uji', 'hash-uji');
  SELECT count(*) INTO v_n FROM users;
  ASSERT v_n = 3, 'CountUsers must count every user';
END
$$;

-- ListUsersPage
DO $$
DECLARE v_id bigint; v_names text; v_rc int;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.pa', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.pb', 'Pengguna Uji', 'hash-uji');
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.pc', 'Pengguna Uji', 'hash-uji');
  INSERT INTO user_roles (user_id, role) VALUES (v_id, 'leadership');
  SELECT string_agg(username, ',' ORDER BY username) INTO v_names FROM (
    SELECT u.username FROM users u ORDER BY u.username LIMIT 2 OFFSET 1) AS p;
  ASSERT v_names = 'q.pb,q.pc', 'ListUsersPage must page by username order';
  SELECT cardinality(coalesce((SELECT array_agg(r.role ORDER BY r.role) FROM user_roles r WHERE r.user_id = u.id), '{}')::text[])
    INTO v_rc FROM users u WHERE u.username = 'q.pb';
  ASSERT v_rc = 0, 'ListUsersPage must give an empty roles array, not NULL, for a user without roles';
  SELECT count(*) INTO v_rc FROM (SELECT u.id FROM users u ORDER BY u.username LIMIT 10 OFFSET 10) AS p;
  ASSERT v_rc = 0, 'ListUsersPage past the end must return no rows';
END
$$;

-- InsertUser
DO $$
DECLARE v_pcr boolean; v_ep bigint; v_id bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.ins', 'Pengguna Uji', 'hash-uji')
    RETURNING id, password_change_required, session_epoch INTO v_id, v_pcr, v_ep;
  ASSERT v_id IS NOT NULL AND v_pcr = true AND v_ep = 0, 'InsertUser must default password_change_required true and session_epoch 0';
END
$$;

-- InsertUserRole and DeleteUserRoles
DO $$
DECLARE v_id bigint; v_n bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.role', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  INSERT INTO user_roles (user_id, role) VALUES (v_id, 'ppid_officer');
  SELECT count(*) INTO v_n FROM user_roles WHERE user_id = v_id;
  ASSERT v_n = 1, 'InsertUserRole must add one role row';
  DELETE FROM user_roles WHERE user_id = v_id;
  SELECT count(*) INTO v_n FROM user_roles WHERE user_id = v_id;
  ASSERT v_n = 0, 'DeleteUserRoles must remove every role of the user';
END
$$;

-- UpdateUserProfile: deactivate an active user (epoch +1, disabled_at set)
DO $$
DECLARE v_id bigint; v_dis timestamptz; v_ep bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.up1', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  UPDATE users SET full_name = 'Pengguna Uji', disabled_at = CASE WHEN false THEN NULL ELSE coalesce(disabled_at, timestamptz '2026-10-15 06:00:00+00') END,
    session_epoch = session_epoch + CASE WHEN NOT false AND disabled_at IS NULL THEN 1 ELSE 0 END, updated_at = now()
    WHERE id = v_id RETURNING disabled_at, session_epoch INTO v_dis, v_ep;
  ASSERT v_dis = timestamptz '2026-10-15 06:00:00+00' AND v_ep = 1, 'UpdateUserProfile deactivate must set disabled_at and bump session_epoch';
END
$$;

-- UpdateUserProfile: deactivating an already disabled user keeps disabled_at and does not bump the epoch again
DO $$
DECLARE v_id bigint; v_dis timestamptz; v_ep bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash, disabled_at, session_epoch)
    VALUES ('q.up2', 'Pengguna Uji', 'hash-uji', timestamptz '2026-10-01 00:00:00+00', 1) RETURNING id INTO v_id;
  UPDATE users SET full_name = 'Pengguna Uji', disabled_at = CASE WHEN false THEN NULL ELSE coalesce(disabled_at, timestamptz '2026-10-15 06:00:00+00') END,
    session_epoch = session_epoch + CASE WHEN NOT false AND disabled_at IS NULL THEN 1 ELSE 0 END, updated_at = now()
    WHERE id = v_id RETURNING disabled_at, session_epoch INTO v_dis, v_ep;
  ASSERT v_dis = timestamptz '2026-10-01 00:00:00+00' AND v_ep = 1, 'UpdateUserProfile must keep the original disabled_at and the epoch when already disabled';
END
$$;

-- UpdateUserProfile: reactivate clears disabled_at and leaves the epoch unchanged
DO $$
DECLARE v_id bigint; v_dis timestamptz; v_ep bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash, disabled_at, session_epoch)
    VALUES ('q.up3', 'Pengguna Uji', 'hash-uji', timestamptz '2026-10-01 00:00:00+00', 1) RETURNING id INTO v_id;
  UPDATE users SET full_name = 'Pengguna Uji', disabled_at = CASE WHEN true THEN NULL ELSE coalesce(disabled_at, timestamptz '2026-10-15 06:00:00+00') END,
    session_epoch = session_epoch + CASE WHEN NOT true AND disabled_at IS NULL THEN 1 ELSE 0 END, updated_at = now()
    WHERE id = v_id RETURNING disabled_at, session_epoch INTO v_dis, v_ep;
  ASSERT v_dis IS NULL AND v_ep = 1, 'UpdateUserProfile reactivate must clear disabled_at and keep the epoch';
END
$$;

-- LockUser
DO $$
DECLARE v_id bigint; v_got bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.lock', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  SELECT id INTO v_got FROM users WHERE id = v_id FOR UPDATE;
  ASSERT v_got = v_id, 'LockUser must lock and return the user row';
END
$$;

-- LockActiveSuperAdmins: only active super admins are returned
DO $$
DECLARE v_sa bigint; v_dis bigint; v_n int;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.sa1', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_sa;
  INSERT INTO user_roles (user_id, role) VALUES (v_sa, 'super_admin');
  INSERT INTO users (username, full_name, password_hash, disabled_at) VALUES ('q.sa2', 'Pengguna Uji', 'hash-uji', now()) RETURNING id INTO v_dis;
  INSERT INTO user_roles (user_id, role) VALUES (v_dis, 'super_admin');
  SELECT count(*) INTO v_n FROM (
    SELECT u.id FROM users u JOIN user_roles r ON r.user_id = u.id
    WHERE r.role = 'super_admin' AND u.disabled_at IS NULL ORDER BY u.id FOR UPDATE OF u) AS l;
  ASSERT v_n = 1, 'LockActiveSuperAdmins must return only active super admins';
END
$$;

-- CountActiveSuperAdmins
DO $$
DECLARE v_sa bigint; v_n bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.csa', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_sa;
  INSERT INTO user_roles (user_id, role) VALUES (v_sa, 'super_admin');
  SELECT count(*) INTO v_n FROM users u JOIN user_roles r ON r.user_id = u.id WHERE r.role = 'super_admin' AND u.disabled_at IS NULL;
  ASSERT v_n = 1, 'CountActiveSuperAdmins must count active super admins';
END
$$;

-- UpdatePasswordHash
DO $$
DECLARE v_id bigint; v_h text;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.hash', 'Pengguna Uji', 'hash-lama') RETURNING id INTO v_id;
  UPDATE users SET password_hash = 'hash-baru', updated_at = now() WHERE id = v_id;
  SELECT password_hash INTO v_h FROM users WHERE id = v_id;
  ASSERT v_h = 'hash-baru', 'UpdatePasswordHash must store the new hash';
END
$$;

-- ChangeOwnPassword: clears password_change_required and bumps the epoch
DO $$
DECLARE v_id bigint; v_ep bigint; v_pcr boolean;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.cop', 'Pengguna Uji', 'hash-lama') RETURNING id INTO v_id;
  UPDATE users SET password_hash = 'hash-baru', password_change_required = false, session_epoch = session_epoch + 1, updated_at = now()
    WHERE id = v_id RETURNING session_epoch, password_change_required INTO v_ep, v_pcr;
  ASSERT v_ep = 1 AND v_pcr = false, 'ChangeOwnPassword must bump session_epoch to 1 and clear password_change_required';
END
$$;

-- ResetPassword: sets password_change_required and bumps the epoch
DO $$
DECLARE v_id bigint; v_ep bigint; v_pcr boolean; v_ret bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash, password_change_required) VALUES ('q.rst', 'Pengguna Uji', 'hash-lama', false) RETURNING id INTO v_id;
  UPDATE users SET password_hash = 'hash-baru', password_change_required = true, session_epoch = session_epoch + 1, updated_at = now()
    WHERE id = v_id RETURNING id, session_epoch, password_change_required INTO v_ret, v_ep, v_pcr;
  ASSERT v_ret = v_id AND v_ep = 1 AND v_pcr = true, 'ResetPassword must return the id, bump the epoch and require a password change';
END
$$;

-- SetLastLogin
DO $$
DECLARE v_id bigint; v_at timestamptz;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.ll', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  UPDATE users SET last_login_at = timestamptz '2026-10-15 06:00:00+00', updated_at = now() WHERE id = v_id;
  SELECT last_login_at INTO v_at FROM users WHERE id = v_id;
  ASSERT v_at = timestamptz '2026-10-15 06:00:00+00', 'SetLastLogin must store the login time';
END
$$;

-- SetPendingTotp: sets a secret only while TOTP is not confirmed
DO $$
DECLARE v_id bigint; v_got bigint; v_last bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.pt1', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  UPDATE users SET totp_secret = '\x01'::bytea, totp_confirmed_at = NULL, totp_last_step = NULL, updated_at = now()
    WHERE id = v_id AND totp_confirmed_at IS NULL RETURNING id INTO v_got;
  ASSERT v_got = v_id, 'SetPendingTotp must set the secret on an unconfirmed user';
  INSERT INTO users (username, full_name, password_hash, totp_secret, totp_confirmed_at, totp_last_step)
    VALUES ('q.pt2', 'Pengguna Uji', 'hash-uji', '\x02'::bytea, now(), 5) RETURNING id INTO v_id;
  UPDATE users SET totp_secret = '\x03'::bytea, totp_confirmed_at = NULL, totp_last_step = NULL, updated_at = now()
    WHERE id = v_id AND totp_confirmed_at IS NULL RETURNING id INTO v_got;
  ASSERT v_got IS NULL, 'SetPendingTotp must not change a confirmed TOTP enrollment';
  SELECT totp_last_step INTO v_last FROM users WHERE id = v_id;
  ASSERT v_last = 5, 'SetPendingTotp must leave the step of a confirmed enrollment untouched';
END
$$;

-- ConfirmTotp: confirms only a pending secret, with a step above the last one
DO $$
DECLARE v_id bigint; v_got bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash, totp_secret) VALUES ('q.ct1', 'Pengguna Uji', 'hash-uji', '\x01'::bytea) RETURNING id INTO v_id;
  UPDATE users SET totp_confirmed_at = timestamptz '2026-10-15 06:00:00+00', totp_last_step = 100, updated_at = now()
    WHERE id = v_id AND totp_secret IS NOT NULL AND totp_confirmed_at IS NULL AND (totp_last_step IS NULL OR totp_last_step < 100) RETURNING id INTO v_got;
  ASSERT v_got = v_id, 'ConfirmTotp must confirm a pending secret';
  UPDATE users SET totp_confirmed_at = timestamptz '2026-10-15 06:00:00+00', totp_last_step = 101, updated_at = now()
    WHERE id = v_id AND totp_secret IS NOT NULL AND totp_confirmed_at IS NULL AND (totp_last_step IS NULL OR totp_last_step < 101) RETURNING id INTO v_got;
  ASSERT v_got IS NULL, 'ConfirmTotp must not confirm twice';
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.ct2', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  UPDATE users SET totp_confirmed_at = now(), totp_last_step = 1, updated_at = now()
    WHERE id = v_id AND totp_secret IS NOT NULL AND totp_confirmed_at IS NULL AND (totp_last_step IS NULL OR totp_last_step < 1) RETURNING id INTO v_got;
  ASSERT v_got IS NULL, 'ConfirmTotp must not confirm a user without a secret';
END
$$;

-- ClaimTotpStep: a step can be used once, and only for a confirmed enrollment
DO $$
DECLARE v_id bigint; v_got bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash, totp_secret, totp_confirmed_at, totp_last_step)
    VALUES ('q.cs1', 'Pengguna Uji', 'hash-uji', '\x01'::bytea, now(), 100) RETURNING id INTO v_id;
  UPDATE users SET totp_last_step = 100, updated_at = now()
    WHERE id = v_id AND totp_confirmed_at IS NOT NULL AND (totp_last_step IS NULL OR totp_last_step < 100) RETURNING id INTO v_got;
  ASSERT v_got IS NULL, 'ClaimTotpStep must reject a replayed step';
  UPDATE users SET totp_last_step = 101, updated_at = now()
    WHERE id = v_id AND totp_confirmed_at IS NOT NULL AND (totp_last_step IS NULL OR totp_last_step < 101) RETURNING id INTO v_got;
  ASSERT v_got = v_id, 'ClaimTotpStep must accept a later step';
  UPDATE users SET totp_last_step = 101, updated_at = now()
    WHERE id = v_id AND totp_confirmed_at IS NOT NULL AND (totp_last_step IS NULL OR totp_last_step < 101) RETURNING id INTO v_got;
  ASSERT v_got IS NULL, 'ClaimTotpStep must reject the same step twice';
  INSERT INTO users (username, full_name, password_hash, totp_secret) VALUES ('q.cs2', 'Pengguna Uji', 'hash-uji', '\x01'::bytea) RETURNING id INTO v_id;
  UPDATE users SET totp_last_step = 1, updated_at = now()
    WHERE id = v_id AND totp_confirmed_at IS NOT NULL AND (totp_last_step IS NULL OR totp_last_step < 1) RETURNING id INTO v_got;
  ASSERT v_got IS NULL, 'ClaimTotpStep must require a confirmed enrollment';
END
$$;

-- ResetTotp: clears the TOTP columns and bumps the epoch
DO $$
DECLARE v_id bigint; v_ep bigint; v_sec bytea; v_conf timestamptz; v_last bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash, totp_secret, totp_confirmed_at, totp_last_step)
    VALUES ('q.rt', 'Pengguna Uji', 'hash-uji', '\x01'::bytea, now(), 7) RETURNING id INTO v_id;
  UPDATE users SET totp_secret = NULL, totp_confirmed_at = NULL, totp_last_step = NULL, session_epoch = session_epoch + 1, updated_at = now()
    WHERE id = v_id RETURNING session_epoch, totp_secret, totp_confirmed_at, totp_last_step INTO v_ep, v_sec, v_conf, v_last;
  ASSERT v_ep = 1 AND v_sec IS NULL AND v_conf IS NULL AND v_last IS NULL, 'ResetTotp must clear the TOTP columns and bump the epoch';
END
$$;

-- DeleteRecoveryCodes
DO $$
DECLARE v_id bigint; v_n bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.dr', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  INSERT INTO recovery_codes (user_id, code_hash) VALUES (v_id, 'h1'), (v_id, 'h2');
  DELETE FROM recovery_codes WHERE user_id = v_id;
  SELECT count(*) INTO v_n FROM recovery_codes WHERE user_id = v_id;
  ASSERT v_n = 0, 'DeleteRecoveryCodes must remove all codes of the user';
END
$$;

-- InsertRecoveryCode
DO $$
DECLARE v_id bigint; v_n bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.irc', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  INSERT INTO recovery_codes (user_id, code_hash) VALUES (v_id, 'h1');
  SELECT count(*) INTO v_n FROM recovery_codes WHERE user_id = v_id AND used_at IS NULL;
  ASSERT v_n = 1, 'InsertRecoveryCode must insert an unused code';
END
$$;

-- ListUnusedRecoveryCodes: unused codes only, ordered by id
DO $$
DECLARE v_id bigint; v_hashes text;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.lu', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  INSERT INTO recovery_codes (user_id, code_hash, used_at) VALUES (v_id, 'h1', NULL), (v_id, 'h2', now()), (v_id, 'h3', NULL);
  SELECT string_agg(code_hash, ',' ORDER BY id) INTO v_hashes FROM (
    SELECT id, code_hash FROM recovery_codes WHERE user_id = v_id AND used_at IS NULL ORDER BY id) AS c;
  ASSERT v_hashes = 'h1,h3', 'ListUnusedRecoveryCodes must return unused codes in id order';
END
$$;

-- UseRecoveryCode: single use
DO $$
DECLARE v_id bigint; v_rc1 int; v_rc2 int;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO users (username, full_name, password_hash) VALUES ('q.uc', 'Pengguna Uji', 'hash-uji') RETURNING id INTO v_id;
  INSERT INTO recovery_codes (user_id, code_hash) VALUES (v_id, 'h1');
  UPDATE recovery_codes SET used_at = timestamptz '2026-10-15 06:00:00+00' WHERE id = (SELECT min(id) FROM recovery_codes WHERE user_id = v_id) AND used_at IS NULL;
  GET DIAGNOSTICS v_rc1 = ROW_COUNT;
  ASSERT v_rc1 = 1, 'UseRecoveryCode must mark an unused code used (1 row)';
  UPDATE recovery_codes SET used_at = timestamptz '2026-10-15 06:00:00+00' WHERE id = (SELECT min(id) FROM recovery_codes WHERE user_id = v_id) AND used_at IS NULL;
  GET DIAGNOSTICS v_rc2 = ROW_COUNT;
  ASSERT v_rc2 = 0, 'UseRecoveryCode must not reuse a code (0 rows)';
END
$$;

-- GetThrottleBlock: the latest unexpired block across the account and ip keys
DO $$
DECLARE v_b timestamptz;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO login_throttle (scope, key, failures, blocked_until, last_failure_at)
    VALUES ('account', 'q.acc', 5, timestamptz '2026-10-15 06:15:00+00', timestamptz '2026-10-15 06:00:00+00'),
           ('ip', '192.0.2.1', 5, timestamptz '2026-10-15 06:20:00+00', timestamptz '2026-10-15 06:00:00+00');
  SELECT max(blocked_until)::timestamptz INTO v_b FROM login_throttle
    WHERE ((scope = 'account' AND key = 'q.acc') OR (scope = 'ip' AND key = '192.0.2.1')) AND blocked_until > timestamptz '2026-10-15 06:05:00+00';
  ASSERT v_b = timestamptz '2026-10-15 06:20:00+00', 'GetThrottleBlock must return the latest unexpired block';
  SELECT max(blocked_until)::timestamptz INTO v_b FROM login_throttle
    WHERE ((scope = 'account' AND key = 'q.acc') OR (scope = 'ip' AND key = '192.0.2.1')) AND blocked_until > timestamptz '2026-10-15 07:00:00+00';
  ASSERT v_b IS NULL, 'GetThrottleBlock must return NULL when every block has expired';
END
$$;

-- RecordThrottleFailure: counts up, resets only after decay, keeps counting under an unexpired block
DO $$
DECLARE v_f int;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO login_throttle (scope, key, failures, last_failure_at)
    VALUES ('ip', 'q.ip1', 1, timestamptz '2026-10-15 06:00:00+00')
    ON CONFLICT (scope, key) DO UPDATE SET failures = CASE WHEN login_throttle.last_failure_at < timestamptz '2026-10-15 05:50:00+00'
      AND (login_throttle.blocked_until IS NULL OR login_throttle.blocked_until <= timestamptz '2026-10-15 06:00:00+00') THEN 1 ELSE login_throttle.failures + 1 END,
      last_failure_at = timestamptz '2026-10-15 06:00:00+00'
    RETURNING failures INTO v_f;
  ASSERT v_f = 1, 'RecordThrottleFailure must insert with failures 1';
  INSERT INTO login_throttle (scope, key, failures, last_failure_at)
    VALUES ('ip', 'q.ip1', 1, timestamptz '2026-10-15 06:00:30+00')
    ON CONFLICT (scope, key) DO UPDATE SET failures = CASE WHEN login_throttle.last_failure_at < timestamptz '2026-10-15 05:50:00+00'
      AND (login_throttle.blocked_until IS NULL OR login_throttle.blocked_until <= timestamptz '2026-10-15 06:00:30+00') THEN 1 ELSE login_throttle.failures + 1 END,
      last_failure_at = timestamptz '2026-10-15 06:00:30+00'
    RETURNING failures INTO v_f;
  ASSERT v_f = 2, 'RecordThrottleFailure must count a second failure inside the window';
  UPDATE login_throttle SET last_failure_at = timestamptz '2026-10-15 05:00:00+00' WHERE scope = 'ip' AND key = 'q.ip1';
  INSERT INTO login_throttle (scope, key, failures, last_failure_at)
    VALUES ('ip', 'q.ip1', 1, timestamptz '2026-10-15 06:30:00+00')
    ON CONFLICT (scope, key) DO UPDATE SET failures = CASE WHEN login_throttle.last_failure_at < timestamptz '2026-10-15 05:50:00+00'
      AND (login_throttle.blocked_until IS NULL OR login_throttle.blocked_until <= timestamptz '2026-10-15 06:30:00+00') THEN 1 ELSE login_throttle.failures + 1 END,
      last_failure_at = timestamptz '2026-10-15 06:30:00+00'
    RETURNING failures INTO v_f;
  ASSERT v_f = 1, 'RecordThrottleFailure must reset to 1 after the decay window';
  UPDATE login_throttle SET blocked_until = timestamptz '2026-10-15 07:00:00+00' WHERE scope = 'ip' AND key = 'q.ip1';
  UPDATE login_throttle SET last_failure_at = timestamptz '2026-10-15 05:00:00+00' WHERE scope = 'ip' AND key = 'q.ip1';
  INSERT INTO login_throttle (scope, key, failures, last_failure_at)
    VALUES ('ip', 'q.ip1', 1, timestamptz '2026-10-15 06:40:00+00')
    ON CONFLICT (scope, key) DO UPDATE SET failures = CASE WHEN login_throttle.last_failure_at < timestamptz '2026-10-15 05:50:00+00'
      AND (login_throttle.blocked_until IS NULL OR login_throttle.blocked_until <= timestamptz '2026-10-15 06:40:00+00') THEN 1 ELSE login_throttle.failures + 1 END,
      last_failure_at = timestamptz '2026-10-15 06:40:00+00'
    RETURNING failures INTO v_f;
  ASSERT v_f = 2, 'RecordThrottleFailure must keep counting while a block is unexpired';
END
$$;

-- SetThrottleBlock
DO $$
DECLARE v_b timestamptz;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO login_throttle (scope, key, failures, last_failure_at) VALUES ('account', 'q.sb', 5, timestamptz '2026-10-15 06:00:00+00');
  UPDATE login_throttle SET blocked_until = timestamptz '2026-10-15 06:15:00+00' WHERE scope = 'account' AND key = 'q.sb';
  SELECT blocked_until INTO v_b FROM login_throttle WHERE scope = 'account' AND key = 'q.sb';
  ASSERT v_b = timestamptz '2026-10-15 06:15:00+00', 'SetThrottleBlock must store blocked_until';
END
$$;

-- DeleteAccountThrottle: removes only the account row
DO $$
DECLARE v_n bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO login_throttle (scope, key, failures, last_failure_at) VALUES ('account', 'q.da', 2, now()), ('ip', 'q.da', 2, now());
  DELETE FROM login_throttle WHERE scope = 'account' AND key = 'q.da';
  SELECT count(*) INTO v_n FROM login_throttle;
  ASSERT v_n = 1, 'DeleteAccountThrottle must remove only the account row';
END
$$;

-- ListDataKeys: ordered by id
DO $$
DECLARE v_ids text;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO data_keys (kek_id, wrapped_key) VALUES ('k1', '\x01'::bytea), ('k1', '\x02'::bytea);
  SELECT string_agg(id::text, ',' ORDER BY id) INTO v_ids FROM (SELECT id FROM data_keys ORDER BY id) AS d;
  ASSERT v_ids = '1,2', 'ListDataKeys must return keys in id order';
END
$$;

-- CountDataKeys
DO $$
DECLARE v_n bigint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO data_keys (kek_id, wrapped_key) VALUES ('k1', '\x01'::bytea);
  SELECT count(*) INTO v_n FROM data_keys;
  ASSERT v_n = 1, 'CountDataKeys must count the keys';
END
$$;

-- InsertDataKey: returns the new id
DO $$
DECLARE v_id smallint;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO data_keys (kek_id, wrapped_key) VALUES ('k1', '\x01'::bytea) RETURNING id INTO v_id;
  ASSERT v_id = 1, 'InsertDataKey must return the new id';
END
$$;

-- UpdateDataKeyWrap: rewraps the key under another KEK
DO $$
DECLARE v_kek text; v_w bytea;
BEGIN
  TRUNCATE users, user_roles, recovery_codes, login_throttle, data_keys RESTART IDENTITY CASCADE;
  INSERT INTO data_keys (kek_id, wrapped_key) VALUES ('k1', '\x01'::bytea);
  UPDATE data_keys SET kek_id = 'k2', wrapped_key = '\x02'::bytea WHERE id = 1;
  SELECT kek_id, wrapped_key INTO v_kek, v_w FROM data_keys WHERE id = 1;
  ASSERT v_kek = 'k2' AND v_w = '\x02'::bytea, 'UpdateDataKeyWrap must store the new KEK id and wrapped key';
END
$$;

-- LockDataKeys: takes the advisory lock for the first-DEK creation
DO $$
DECLARE v_n bigint;
BEGIN
  PERFORM pg_advisory_xact_lock(1801812338);
  SELECT count(*) INTO v_n FROM pg_locks
    WHERE locktype = 'advisory' AND objid = 1801812338 AND objsubid = 1 AND granted AND pid = pg_backend_pid();
  ASSERT v_n = 1, 'LockDataKeys must hold the advisory lock 1801812338 for the transaction';
END
$$;
