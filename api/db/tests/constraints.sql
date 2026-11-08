-- Database rule tests: each rule gets a check that FAILS when the rule is broken.
-- Run by `make db-test` against a freshly migrated throwaway database (ON_ERROR_STOP).
DO $$
BEGIN
  ASSERT (SELECT count(*) FROM pg_extension WHERE extname IN ('pg_trgm', 'unaccent')) = 2,
    'extensions pg_trgm and unaccent must exist (PRD Appendix D: PostgreSQL text search)';
END
$$;

-- D1: users.username must match the format check, and must be unique.
DO $$
DECLARE
  caught text := '';
BEGIN
  BEGIN
    INSERT INTO users (username, full_name, password_hash) VALUES ('Ab', 'Pengguna Uji', 'hash-uji');
  EXCEPTION WHEN check_violation THEN
    caught := 'check_violation';
  END;
  ASSERT caught = 'check_violation', 'D1: username Ab must be rejected by users_username_format';

  INSERT INTO users (username, full_name, password_hash) VALUES ('d1.dupe', 'Pengguna Uji', 'hash-uji');
  caught := '';
  BEGIN
    INSERT INTO users (username, full_name, password_hash) VALUES ('d1.dupe', 'Pengguna Uji', 'hash-uji');
  EXCEPTION WHEN unique_violation THEN
    caught := 'unique_violation';
  END;
  ASSERT caught = 'unique_violation', 'D1: a duplicate username must be rejected by the unique constraint';
  DELETE FROM users WHERE username = 'd1.dupe';
END
$$;

-- D2: user_roles.role must be a known role, and (user_id, role) must be unique.
DO $$
DECLARE
  uid bigint;
  caught text := '';
BEGIN
  INSERT INTO users (username, full_name, password_hash) VALUES ('d2.roles', 'Pengguna Uji', 'hash-uji') RETURNING id INTO uid;
  BEGIN
    INSERT INTO user_roles (user_id, role) VALUES (uid, 'root');
  EXCEPTION WHEN check_violation THEN
    caught := 'check_violation';
  END;
  ASSERT caught = 'check_violation', 'D2: role root must be rejected by user_roles_role_known';

  INSERT INTO user_roles (user_id, role) VALUES (uid, 'humas_editor');
  caught := '';
  BEGIN
    INSERT INTO user_roles (user_id, role) VALUES (uid, 'humas_editor');
  EXCEPTION WHEN unique_violation THEN
    caught := 'unique_violation';
  END;
  ASSERT caught = 'unique_violation', 'D2: a duplicate (user_id, role) must be rejected by the primary key';
  DELETE FROM users WHERE id = uid;
END
$$;

-- D3: a confirmed TOTP enrollment must have a secret.
DO $$
DECLARE
  caught text := '';
BEGIN
  BEGIN
    INSERT INTO users (username, full_name, password_hash, totp_secret, totp_confirmed_at)
    VALUES ('d3.totp', 'Pengguna Uji', 'hash-uji', NULL, now());
  EXCEPTION WHEN check_violation THEN
    caught := 'check_violation';
  END;
  ASSERT caught = 'check_violation', 'D3: totp_confirmed_at without totp_secret must be rejected by users_totp_confirmed_needs_secret';
END
$$;
