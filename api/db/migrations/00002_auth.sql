-- +goose Up
-- auth module (docs/modules/auth.md, "Data"): users, roles, recovery codes, login throttle,
-- sessions (the pgxstore schema) and the envelope-encryption data keys.

CREATE TABLE users (
    id                       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username                 text        NOT NULL,
    full_name                text        NOT NULL,
    password_hash            text        NOT NULL,
    password_change_required boolean     NOT NULL DEFAULT true,
    disabled_at              timestamptz,
    session_epoch            integer     NOT NULL DEFAULT 0,
    totp_secret              bytea,
    totp_confirmed_at        timestamptz,
    totp_last_step           bigint,
    last_login_at            timestamptz,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_username_unique UNIQUE (username),
    CONSTRAINT users_username_format CHECK (username ~ '^[a-z0-9._-]{3,32}$'),
    CONSTRAINT users_full_name_length CHECK (char_length(full_name) BETWEEN 1 AND 100),
    CONSTRAINT users_totp_confirmed_needs_secret CHECK (totp_confirmed_at IS NULL OR totp_secret IS NOT NULL)
);
COMMENT ON TABLE users IS 'Admin panel accounts. Never deleted; deactivated through disabled_at. totp_secret is ciphertext (keyring, version byte 0x01).';

CREATE TABLE user_roles (
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role    text   NOT NULL,
    PRIMARY KEY (user_id, role),
    CONSTRAINT user_roles_role_known CHECK (role IN ('super_admin', 'humas_editor', 'document_operator', 'ppid_officer', 'leadership'))
);
COMMENT ON TABLE user_roles IS 'Roles of a user (fixed set, see internal/authz). The primary key also serves as the foreign key index.';

CREATE TABLE recovery_codes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code_hash  text        NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX recovery_codes_user_id_idx ON recovery_codes (user_id);
COMMENT ON TABLE recovery_codes IS 'One-time TOTP recovery codes, stored as argon2id hashes. Regeneration replaces the whole set.';

CREATE TABLE login_throttle (
    scope           text        NOT NULL,
    key             text        NOT NULL,
    failures        integer     NOT NULL DEFAULT 0,
    blocked_until   timestamptz,
    last_failure_at timestamptz NOT NULL,
    PRIMARY KEY (scope, key),
    CONSTRAINT login_throttle_scope_known CHECK (scope IN ('account', 'ip'))
);
COMMENT ON TABLE login_throttle IS 'Failed-login counters per account (lowercased username) and per client IP, with the block deadline.';

CREATE TABLE sessions (
    token  text        PRIMARY KEY,
    data   bytea       NOT NULL,
    expiry timestamptz NOT NULL
);
CREATE INDEX sessions_expiry_idx ON sessions (expiry);
COMMENT ON TABLE sessions IS 'Server-side sessions (alexedwards/scs pgxstore schema). The token is the cookie value.';

CREATE TABLE data_keys (
    id          smallint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kek_id      text        NOT NULL,
    wrapped_key bytea       NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE data_keys IS 'Envelope encryption data keys (DEK), wrapped with a key encryption key from the KEK file. The active DEK is the one with the highest id.';

-- Runtime role app (created outside migrations, see the runbook "Database runtime role").
GRANT SELECT, INSERT, UPDATE ON users TO app;
GRANT SELECT, INSERT, DELETE ON user_roles TO app;
GRANT SELECT, INSERT, DELETE ON recovery_codes TO app;
GRANT UPDATE (used_at) ON recovery_codes TO app;
GRANT SELECT, INSERT, UPDATE, DELETE ON login_throttle TO app;
GRANT SELECT, INSERT, UPDATE, DELETE ON sessions TO app;
GRANT SELECT, INSERT ON data_keys TO app;
GRANT UPDATE (kek_id, wrapped_key) ON data_keys TO app;

-- +goose Down
DROP TABLE data_keys, sessions, login_throttle, recovery_codes, user_roles, users;
