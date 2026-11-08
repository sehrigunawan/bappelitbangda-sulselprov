# Module: auth

> **Draft, security lane.** Written by an agent from the PRD and repo rules. The author explains every line; the other developer reads every line (`docs/workflow/team-and-agents.md`, security lane checklist). Items marked **[review]** are security parameters a human must confirm.

Living document: any PR that changes the module's behaviour updates it in the same PR. This spec is longer than the 1 to 2 pages of the template (PRD Appendix D) because the roadmap assigns sessions, RBAC, TOTP, login throttling and envelope encryption to this one module. The schema list in section "API" moves into `openapi.yaml` in phase 2 of the contract PR and is replaced there by a link.

- **Requirements:** FR-33, NFR-05, NFR-17. The module also provides the step-up primitive that `ppid` uses for NFR-08 (NFR-08 itself stays in `ppid`).
- **Depends on open questions:**
  - OQ-10: no approval permission; the Humas Editor publishes directly. An approval step would add a level to the permission model in a later contract PR.
  - OQ-11: no email, so no self-service password reset. An admin resets passwords.
  - OQ-16: blocks the auth merge and production keys. Drafted with the PRD defaults (AES-256-GCM, argon2id, TOTP).
  - OQ-17: the dev key comes from a file and protects test data only; the two production key holders are not decided.
- **Security lane:** yes. In the BE lane an agent may draft code and tests, but the author explains every line and rewrites nothing they cannot explain; the keyring (nonce and key handling), TOTP, sessions and password code are read line by line by the other developer (`api/AGENTS.md`).

## Roles and permissions

Roles are a fixed enum stored as text. Slug and Indonesian label (NFR-16):

| Slug | Label |
|---|---|
| `super_admin` | Super Admin |
| `humas_editor` | Editor Humas |
| `document_operator` | Operator Dokumen |
| `ppid_officer` | Petugas PPID |
| `leadership` | Pimpinan |

Areas (9 slugs, one per row of PRD section 3.4):

| Slug | Requirement | Covers |
|---|---|---|
| `publications` | FR-34 | Publications |
| `pages` | FR-35 | Pages and menus |
| `documents` | FR-36 | Documents and categories |
| `site_content` | FR-37 | Sliders, applications, staff, surveys, holidays, settings |
| `media` | FR-38 | Media library |
| `ppid` | FR-24 | PPID queue and applicant data |
| `users` | FR-33 | Users and roles |
| `activity_log` | FR-39 | Activity log |
| `statistics` | FR-32 | Statistics and summaries (FR-40 is deferred) |

Levels: `view` < `manage`. `manage` implies `view`; this is the PRD's "M overrides V".

Matrix, copied from PRD section 3.4 (`docs/prd/PRD.md` lines 164-173). `manage` = M, `view` = V, `—` = no access:

| Area | Requirement | Super Admin | Humas Editor | Document Operator | PPID Officer | Leadership |
|---|---|---|---|---|---|---|
| `publications` | FR-34 | manage | manage | — | — | — |
| `pages` | FR-35 | manage | manage | — | — | — |
| `documents` | FR-36 | manage | — | manage | view | — |
| `site_content` | FR-37 | manage | manage | — | — | — |
| `media` | FR-38 | manage | manage | manage | manage | — |
| `ppid` | FR-24 | manage | — | — | manage | — |
| `users` | FR-33 | manage | — | — | — | — |
| `activity_log` | FR-39 | view | — | — | — | — |
| `statistics` | FR-32, FR-40 (deferred) | view | — | — | view | view |

Notes:
- The PRD calls the matrix an "initial proposal". Holidays sit in `site_content` per FR-37; who enters holidays is still open (`docs/decisions/prd-v1.1-reconciliation.md` item 12), and `ppid_officer` gets no holiday rights until that is answered.
- Role changes take effect on the user's next request, because `loadPrincipal` reads the roles on every request.

Go API, new package `api/internal/authz` (nothing like it exists today):
- `type Role string`, `type Area string`, `type Level uint8` with constants `None`, `View`, `Manage`.
- `type Permission struct{ Area Area; Level Level }`.
- `type Principal struct{ UserID int64; Roles []Role }`.
- `func Can(p Principal, perm Permission) bool`: true if any role's matrix level for `perm.Area` is `>= perm.Level` and `perm.Level > None`. An unknown role or area is denied.
- `func Effective(p Principal) []Permission`: the union, one entry per area at its highest level, areas in the order of the table above. It feeds `SessionState.permissions`.
- The matrix is an unexported Go map. Changing it needs a PR.

Enforcement is deny by default through one strict middleware `authorize` in `api/internal/server` (`StrictMiddlewareFunc`, `api/internal/api/api.gen.go:274`):
- It holds `operationRules map[string]rule`, keyed by the Go operation name the middleware receives (for example `"ListUsers"`, `api.gen.go:320`), where `rule{public bool; stages []SessionStage; perm *authz.Permission; stepUp bool}`.
- An operation missing from the map is answered 403 (`Akses ditolak.`).
- Handlers never check roles; they read the principal with `principalFrom(ctx)`.
- A rule without `perm` and without `public` means "any session at one of `stages`". A rule with `perm` requires stage `authenticated` and `authz.Can`.

## Data

One migration in the BE lane. Every table gets `COMMENT ON TABLE` and a grant to the `app` role. Timestamps are `timestamptz` (UTC). Foreign keys have an index.

| Table | Columns | Constraints, indexes, `app` grants |
|---|---|---|
| `users` | `id bigint generated always as identity`, `username text`, `full_name text`, `password_hash text`, `password_change_required boolean default true`, `disabled_at timestamptz null`, `session_epoch integer default 0`, `totp_secret bytea null` (ciphertext), `totp_confirmed_at timestamptz null`, `totp_last_step bigint null`, `last_login_at timestamptz null`, `created_at`, `updated_at` (both `default now()`) | PK `id`; `UNIQUE(username)`; `CHECK (username ~ '^[a-z0-9._-]{3,32}$')`; `CHECK (char_length(full_name) BETWEEN 1 AND 100)`; `CHECK (totp_confirmed_at IS NULL OR totp_secret IS NOT NULL)`. Grants: SELECT, INSERT, UPDATE; **no DELETE** (deactivate via `disabled_at`). |
| `user_roles` | `user_id bigint REFERENCES users ON DELETE CASCADE`, `role text` | PK `(user_id, role)`, which also serves as the FK index; `CHECK (role IN ('super_admin','humas_editor','document_operator','ppid_officer','leadership'))`. Grants: SELECT, INSERT, DELETE. |
| `recovery_codes` | `id bigint generated always as identity`, `user_id bigint REFERENCES users ON DELETE CASCADE`, `code_hash text`, `used_at timestamptz null`, `created_at` | PK `id`; index `recovery_codes_user_id_idx` on `user_id`. Grants: SELECT, INSERT, UPDATE(`used_at`), DELETE (regeneration replaces the set). |
| `login_throttle` | `scope text`, `key text`, `failures integer`, `blocked_until timestamptz null`, `last_failure_at timestamptz` | PK `(scope, key)`; `CHECK (scope IN ('account','ip'))`. Grants: SELECT, INSERT, UPDATE, DELETE. No purge job: the admin is network-allowlisted and the row count stays small. |
| `sessions` | `token text PRIMARY KEY`, `data bytea NOT NULL`, `expiry timestamptz NOT NULL` | Index `sessions_expiry_idx ON sessions (expiry)`. This is the `pgxstore` schema, checked against the pgxstore README (which also requires SELECT, INSERT, UPDATE, DELETE for the app role; that is the grant here). Re-check if the pgxstore version changes. |
| `data_keys` | `id smallint generated always as identity`, `kek_id text`, `wrapped_key bytea`, `created_at` | PK `id`. The active DEK is `max(id)`. Grants: SELECT, INSERT, UPDATE(`kek_id`, `wrapped_key`) for KEK rewrap; **no DELETE**. |

`api/db/tests/privileges.sql` cases: `app` has no DELETE on `users` and none on `data_keys`; `app` cannot UPDATE `data_keys.id` or `data_keys.created_at`.

## Mechanisms

Every bullet tagged **[review]** is for the human reviewer.

- **Sessions. [review]** `scs` with `pgxstore`; `LoadAndSave` wraps the `/api/v1/admin/` subrouter only, so `GetHealth` and every other public route stay DB-free even when a cookie is sent. **[agent call]**
  - Middleware order on `/api/v1/admin/`, outermost first: `no-store` header, `CrossOriginProtection`, scs `LoadAndSave`, `loadPrincipal`, strict `authorize`. A cross-origin POST is therefore rejected with 403 before any session or authorization work. **[agent call]**
  - Idle timeout 30 min (NFR-05, `AUTH_SESSION_IDLE_TIMEOUT`); absolute lifetime 12 h (ASVS L2 re-authentication, `AUTH_SESSION_LIFETIME`).
  - Cookie name `__Host-bpl_session` when `AUTH_COOKIE_SECURE=true`, `bpl_session` otherwise (dev over http).
  - Cookie flags: `HttpOnly`, `SameSite=Strict`, `Path=/`, no `Domain`; `Secure` follows `AUTH_COOKIE_SECURE`.
  - `RenewToken` on every stage change, on step-up and on password change.
  - Session keys: `user_id`, `stage`, `stage_at`, `session_epoch`, `step_up_at`.
  - A chi middleware `loadPrincipal` reads the user and roles on every request. If the user is disabled, the stored epoch differs from `users.session_epoch`, or a pending stage is older than `AUTH_PENDING_TTL`, it calls `Destroy` and the request continues as anonymous.
  - Clocks: throttle, pending TTL and step-up read `Deps.Now`. scs idle timeout and absolute lifetime (and the `pgxstore` expiry query) use the real clock, so their tests use short real durations. **[agent call]**
- **Stages.** Enum `SessionStage`: `password_change_required`, `totp_required`, `totp_enrollment_required`, `authenticated`.
  - After a correct password, resolve in this order: TOTP confirmed, then `totp_required`; else `password_change_required` if the flag is set; else, if a role is `super_admin` or `ppid_officer` and TOTP is not confirmed, `totp_enrollment_required`; else `authenticated`.
  - After a stage is completed (TOTP verified, password changed, enrollment confirmed) the same resolution runs again from the remaining conditions, so a TOTP user with a temporary password goes `totp_required`, then `password_change_required`, then `authenticated`.
  - Any user may enroll TOTP. Once enrolled, TOTP is always required at login.
  - A pending session is valid for `AUTH_PENDING_TTL` (default 10 min), measured from `stage_at`, which is set at login and again at every stage change. Each pending stage therefore gets its own 10 minutes; there are at most three. **[agent call]**
  - Only `authenticated` produces a `Principal`.
- **Login throttle (FR-33). [review]**
  - Keys: `('account', lower(username))` and `('ip', client IP)`. The account key is recorded whether or not the user exists, but only for usernames that match the username pattern; other values (including over-long ones) count against the IP key only. Rows for invented usernames are not purged: accepted risk, the admin is network-allowlisted. **[review]**
  - Before any hash work, if either key has `blocked_until > now`, answer 429 with `Retry-After` set to the remaining whole seconds, rounded up.
  - On each failure, increment `failures` for both keys. If the key's `last_failure_at` is older than `AUTH_THROTTLE_MAX_DELAY` and it is not blocked, restart `failures` at 1 first (the counter decays; one typo a month never escalates). **[agent call]**
  - Counted failures: wrong password at login, wrong TOTP or recovery code at `createTotpVerification`, wrong step-up code at `createStepUp`, wrong current password at `updatePassword`. The last two use the session user's account key and the client IP. **[agent call]**
  - When `failures >= AUTH_THROTTLE_FAILURES` (5), set `blocked_until = now + min(AUTH_THROTTLE_BASE_DELAY * 2^(failures - threshold), AUTH_THROTTLE_MAX_DELAY)`. That gives 15 min, then 30, then 60, capped at 24 h. Never a permanent lock. The failing request that sets `blocked_until` itself answers 429 with `Retry-After`; failures below the threshold answer 401 (or 400 for `updatePassword`).
  - A successful full login (stage `authenticated`) deletes the account row only. The IP row is kept so one valid account cannot reset an attacker's IP counter; it decays by the rule above. **[review]**
  - The client IP is `X-Real-IP` only when `AUTH_TRUST_PROXY_HEADER=true` (nginx sets it); otherwise the host part of `RemoteAddr`.
  - Throttle queries take `now` from the injected clock `Deps.Now`, so tests can advance time.
  - An unknown username runs argon2id against a fixed dummy hash, so timing matches a real user.
- **Passwords. [review]**
  - argon2id via `golang.org/x/crypto/argon2`: m=19456 KiB, t=2, p=1, 16-byte salt, 32-byte key, PHC string `$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>` (OWASP minimum; re-check at review).
  - Rehash on successful login when the stored parameters differ.
  - A `Hasher` type limits concurrency with a buffered channel of capacity 2 and acquires it with the request context.
  - Policy (ASVS 2.1): 12 to 128 characters counted as Unicode code points (no normalization), no composition rules; not equal to the username and not containing it (case-insensitive); not in the embedded common-password list `api/internal/auth/common-passwords.txt`, compared case-insensitively. A policy failure answers 400 whose `detail` is exactly one of the labels `Minimal 12 karakter.`, `Kata sandi terlalu umum.`, `Kata sandi tidak boleh memuat nama pengguna.` (and `Maksimal 128 karakter.`); the FE shows `detail`. **[agent call]**
  - Common-password list source: SecLists `Passwords/Common-Credentials/10k-most-common.txt` (MIT; the path exists in the SecLists repo), filtered to entries of 12 or more characters, with a license line in the file header. The filtered list is short; the BE lane may switch to a larger file in the same directory (for example `100k-most-used-passwords-NCSC.txt`) if fewer than 100 entries remain. **[agent call]**
  - Temporary passwords: 16 characters drawn with `crypto/rand` from the lowercase Crockford base32 alphabet `0123456789abcdefghjkmnpqrstvwxyz` (80 bits), shown once; they set `password_change_required`.
- **TOTP and recovery codes. [review]**
  - `github.com/pquerna/otp`: RFC 6238, SHA-1, 6 digits, 30 s period, skew +-1 step, issuer `Bappelitbangda Sulsel`, account name = username, 20-byte secret.
  - Replay protection: accept a code only if its time step is greater than `totp_last_step`, then store the step (update with `WHERE totp_last_step IS NULL OR totp_last_step < $step` so two concurrent requests cannot both pass).
  - The secret is stored with `Keyring.Seal(secret, aad="users.totp_secret:"+id)`. A pending enrollment writes `totp_secret` with `totp_confirmed_at = NULL`; confirming sets `totp_confirmed_at` and returns 10 recovery codes.
  - Recovery code: 10 characters from the same lowercase Crockford alphabet (50 bits), shown as `xxxxx-xxxxx`. Input is normalized before hashing and comparing: strip `-` and spaces, lowercase, map `o` to `0` and `i`/`l` to `1`. **[agent call]**
  - Codes are stored as argon2id hashes via `Hasher`. Verification checks each unused code and sets `used_at`. Up to 10 hashes per attempt through the 2-slot `Hasher` is accepted at this scale; whether a fast keyed hash is better for 50-bit codes is a human call. **[review]** A TOTP code and a recovery code count equally as one failure for throttling.
  - `pquerna/otp` validation returns a boolean, so the implementation checks the steps `now-1`, `now`, `now+1` itself (`totp.GenerateCodeCustom`, constant-time compare) to learn the matched step. Login, step-up and enrollment confirmation all check and store `totp_last_step`; a code used at login therefore cannot be reused for step-up within the same 30 s window, and the FE asks for a fresh code. **[agent call]**
- **Step-up.**
  - `POST /session/step-up` takes a TOTP code only (recovery codes are not accepted) and sets `step_up_at`.
  - A `stepUp: true` rule requires `now - step_up_at <= AUTH_STEP_UP_TTL` (default 10 min); otherwise 403 with detail `Verifikasi ulang dengan kode autentikator diperlukan.` The session's `stepUpExpiresAt` is `step_up_at + AUTH_STEP_UP_TTL`, and `null` once past. A successful password change clears `step_up_at`.
  - `ppid` reuses the same rule flag for personal-data access.
  - A user without confirmed TOTP gets 403 from step-up.
- **CSRF. [review]** `http.NewCrossOriginProtection()` (stdlib, uses `Sec-Fetch-Site`, else compares the `Origin` and `Host` hostnames, allows safe methods) wraps `/api/v1/admin/`. Its deny handler writes a 403 Problem with detail `Permintaan lintas situs ditolak.` This works together with `SameSite=Strict`. No CSRF token and no trusted origins: nginx forwards `Host` (`proxy_set_header Host $host`) and the dev server proxies `/api` (`web/vite.config.ts`), so browsers are same-origin in both.
- **No caching.** Responses under `/api/v1/admin/` get `Cache-Control: no-store`.
- **Envelope encryption (NFR-17). [review]** New package `api/internal/keyring`. Nonce and key handling are never left to an agent unreviewed.
  - KEK file at `KEK_FILE`: one key per line, `<kek-id> <base64 std of 32 bytes>`; `#` starts a comment.
  - `KEK_ACTIVE_ID` selects the wrapping KEK. All KEKs in the file can unwrap.
  - DEK: 32 random bytes. `wrapped_key = AES-256-GCM(KEK, random 12-byte nonce, DEK, aad="data_keys:"+kek_id)`, stored as `nonce||ct`; the AAD binds the wrapped key to the KEK id stored beside it, and rewrap changes both together. **[agent call]**
  - Field ciphertext: `0x01 || uint16 big-endian data_key_id || 12-byte random nonce || GCM ciphertext+tag`. The caller AAD binds the value to its column and row.
  - API:
    - `func Load(ctx context.Context, q *dbgen.Queries, kekFile, activeID string) (*Keyring, error)`; `q` is built on the `DATABASE_URL` pool.
    - `func (k *Keyring) Seal(plaintext, aad []byte) ([]byte, error)`
    - `func (k *Keyring) Open(ciphertext, aad []byte) ([]byte, error)`
  - Startup fails with an error naming the missing `kek_id` (never key bytes) when a `data_keys` row cannot be unwrapped, or when `KEK_ACTIVE_ID` is not in the KEK file.
  - If `data_keys` is empty, `Load` creates the first DEK inside a transaction that holds a `pg_advisory_xact_lock` and re-checks emptiness, so the server and the CLI cannot both create one. **[agent call]**
  - `Open` with an unknown DEK id reloads `data_keys` once. `Seal` uses the DEK that was active at load, so a DEK rotation applies to new writes after the next restart. KEK rewrap needs no restart.
  - Never log plaintext, key bytes, TOTP codes, passwords, recovery codes or session tokens.
- **Logging.** slog events `auth.login.succeeded`, `auth.login.failed`, `auth.login.throttled`, `auth.logout`, `auth.totp.failed`, `auth.totp.enrolled`, `auth.recovery_codes.regenerated`, `auth.stepup.succeeded`, `auth.stepup.failed`, `auth.password.changed`, `auth.user.created`, `auth.user.updated`, `auth.password.reset`, `auth.totp.reset`. Fields: `username`, `user_id`, `client_ip`, `request_id`. The FR-39 activity log belongs to the `audit` module; nothing here writes an audit table.
- **Edge rules.**
  - `createSession` while any session exists destroys it first, then starts a new one.
  - A successful `updatePassword` increments `session_epoch` (other sessions end), stores the new epoch in the current session, re-resolves the stage (it may become `totp_enrollment_required`), clears `step_up_at` and renews the token, keeping that session.
  - A repeated `createTotpEnrollment` before confirmation overwrites the pending secret.
  - `username` is immutable; `UserUpdate` has no username field.
  - `listUsers` orders by `username` ascending.
  - The last-active-Super-Admin check in `updateUser` runs in one transaction that locks the active `super_admin` rows (`SELECT ... FOR UPDATE`), so two concurrent demotions cannot both pass.
  - Resetting a password or TOTP, and disabling a user, bumps the target's `session_epoch`. `createPasswordReset` and `deleteUserTotp` on the caller's own id answer 409 (use `updatePassword` and the CLI), so an admin cannot log themselves out by accident.
  - A role change does not bump `session_epoch` (roles are read on every request).
  - Every `UPDATE users` sets `updated_at = now()` in the query.
- **Authorization failures.** `authorize` and `loadPrincipal` return sentinel errors (`ErrUnauthenticated`, `ErrForbidden`, `ErrStepUpRequired`, a throttle error carrying the retry seconds) that `responseErrorHandler` in `api/internal/server/errors.go` maps to Problems, like `ErrNotImplemented` today; they are expected failures, not bugs, so they are not logged at error level. **[agent call]**
  - No session, or a pending session facing a rule that needs `authenticated` or a permission: 401. A session at a stage the rule does not list (for example an authenticated user calling `createTotpVerification`, or `password_change_required` calling `createTotpEnrollment`): 403. **[agent call]**
- **Not in this module.** NFR-05's CSP, HSTS and TLS settings belong to nginx and the Wave 6 MVP gates (`docs/handover/known-gaps.md`). The enrollment QR is a `data:` image, so the CSP for `/admin` must allow `img-src data:`.

**Column nullability.** Columns not marked `null` in the Data table are `NOT NULL`. Defaults: `failures` 0, `created_at` `now()` on every table that has it, `last_failure_at` set by the query.

## Configuration and CLI

Env vars, read in `api/cmd/api/main.go` with `os.Getenv` (the existing pattern):
- Required: `DATABASE_URL`, `KEK_FILE` (production `/run/secrets/kek`), `KEK_ACTIVE_ID`.
- Defaults: `AUTH_COOKIE_SECURE=true`, `AUTH_SESSION_IDLE_TIMEOUT=30m`, `AUTH_SESSION_LIFETIME=12h`, `AUTH_PENDING_TTL=10m`, `AUTH_STEP_UP_TTL=10m`, `AUTH_THROTTLE_FAILURES=5`, `AUTH_THROTTLE_BASE_DELAY=15m`, `AUTH_THROTTLE_MAX_DELAY=24h`, `AUTH_TRUST_PROXY_HEADER=false`.
- Durations use `time.ParseDuration`; booleans and integers use `strconv`. An invalid value fails startup. `main.go` parses these into a plain `server.Config` struct (one field per variable) through a helper `configFromEnv(getenv func(string) string) (server.Config, error)`; `server.DefaultConfig()` returns the defaults for tests. **[agent call]**

The BE lane:
- adds these to `.env.example` (fake values, `AUTH_COOKIE_SECURE=false`) and to `deploy/env.example` (`AUTH_TRUST_PROXY_HEADER=true`, `KEK_FILE=/run/secrets/kek`, `KEK_ACTIVE_ID=kek-1`);
- removes the "Not read by the application yet" comment in `deploy/docker-compose.yml` on the `kek` secret.

Server wiring: `server.NewHandler(logger *slog.Logger, deps Deps) http.Handler` with `type Deps struct{ Pool *pgxpool.Pool; Keyring *keyring.Keyring; Config Config; Now func() time.Time }`. Update every caller (`api/cmd/api/main.go`, `api/internal/server/*_test.go`). `GetHealth` stays DB-free. Non-DB tests pass `Deps{Config: DefaultConfig(), Now: time.Now}` with a nil `Pool`; the admin subrouter is mounted only when `Pool` is non-nil. Sessions always use `pgxstore` (no in-memory store), so session tests need `TEST_DATABASE_URL`. Test keyrings use a random KEK written to `t.TempDir()`. **[agent call]**

CLI: subcommands of the same binary, dispatched on `os.Args[1]`; no args means serve. The dispatcher is `run(ctx, args []string, stdout io.Writer, getenv func(string) string) error`, which tests call directly (A36). Each command prints results to stdout and exits non-zero on error (duplicate username, unknown user, missing env var: one-line message, no stack). `admin *` needs only `DATABASE_URL`; `keys *` also needs `KEK_FILE` and `KEK_ACTIVE_ID`. In development run them from `api/` with `go run ./cmd/api admin ...`; there is no `make` target because the commands are for operators, not the dev loop. The dev KEK is a single line `dev-1 <base64 of 32 random bytes>` in the git-ignored `secrets/` directory, generated with `head -c 32 /dev/urandom | base64`. **[agent call]**
- `api admin create --username U --full-name "N"`: creates a `super_admin` with `password_change_required` and prints the temporary password once.
- `api admin reset-password --username U`: new temporary password; bumps `session_epoch`.
- `api admin reset-totp --username U`: TOTP break-glass. Clears the secret, `totp_confirmed_at`, `totp_last_step` and the recovery codes; bumps `session_epoch`.
- `api keys rotate-dek`: inserts a new DEK wrapped with the active KEK.
- `api keys rewrap`: rewraps every `data_keys` row with `KEK_ACTIVE_ID`.

The BE lane fills the runbook sections "Add an admin", "TOTP break-glass" and "Key rotation" (`docs/handover/runbook.md`, all `TODO` today) with these commands, run as `docker compose exec api /api ...`. The binary path `/api` is confirmed in `api/Dockerfile` (`ENTRYPOINT ["/api"]`).

## API

Operations live in `openapi.yaml` (tag `auth`) once phase 2 lands. This table only names them. All paths are under `/api/v1/admin/auth`. Every operation has `default: Problem`. The Auth column uses: `public`; `any session` (including pending stages); a stage name; `perm <area>.<level>`; `+step-up`. The 401 Problem detail is `Sesi tidak valid atau telah berakhir.`; the 403 detail is `Akses ditolak.`

Contract conventions **[agent call]**:
- 401, 403 and 429 are listed explicitly on every operation that can return them, as `$ref` to new shared `components/responses` `Unauthorized`, `Forbidden` and `TooManyRequests` (the last carries the `RetryAfter` header). The server produces them from sentinel errors in `authorize`, not from handler return values (see "Authorization failures").
- `components/securitySchemes.cookieAuth`: `apiKey` in a cookie named `__Host-bpl_session` (the production name; the description states the dev name `bpl_session`). Every operation has `security: [{cookieAuth: []}]` except `createSession`, which has `security: []`. `createSession` and `createTotpVerification` document the `Set-Cookie` response header.
- `createSession` and `getSession` 200 carry one named example per `SessionStage` value, so the FE fixtures cover all four stages.
- The Problem schema has only `title`, `status`, `detail`. This module does not add fields: the FE tells errors apart by status plus the exact Indonesian `detail` text listed here. Whether Problem should gain a machine-readable `code` is a human decision for the contract review.

| operationId | Method and path | Auth | Responses |
|---|---|---|---|
| `createSession` | `POST /session` | public | 200 `SessionState`; 400; 401 (`Nama pengguna atau kata sandi salah.`, identical for an unknown user, a wrong password or a disabled user); 429 + `Retry-After` |
| `getSession` | `GET /session` | any session | 200 `SessionState`; 401 |
| `deleteSession` | `DELETE /session` | any session | 204; 401 |
| `createTotpVerification` | `POST /session/totp` | stage `totp_required` | 200 `SessionState`; 400 (neither or both of `code` and `recoveryCode`); 401 (wrong or replayed code); 429 |
| `createStepUp` | `POST /session/step-up` | `authenticated` | 200 `StepUp`; 400; 401; 403 (no TOTP enrolled); 429 |
| `updatePassword` | `PUT /password` | `authenticated` or `password_change_required` | 204; 400 (policy, or wrong current password with detail `Kata sandi saat ini salah.`); 401; 429 |
| `createTotpEnrollment` | `POST /totp` | `authenticated` or `totp_enrollment_required` | 200 `TotpEnrollment`; 409 already confirmed |
| `updateTotpEnrollment` | `PUT /totp` | `authenticated` or `totp_enrollment_required` | 200 `RecoveryCodes`; 400 wrong code; 409 no pending enrollment |
| `createRecoveryCodes` | `POST /totp/recovery-codes` | `authenticated` +step-up | 200 `RecoveryCodes`; 403 |
| `listUsers` | `GET /users?page&pageSize` | perm `users.manage` | 200 `UserPage`; 400 (bad or out-of-range `page`/`pageSize`) |
| `createUser` | `POST /users` | perm `users.manage` | 201 `UserCreated`; 400; 409 duplicate username |
| `getUser` | `GET /users/{userId}` | perm `users.manage` | 200 `User`; 404 |
| `updateUser` | `PUT /users/{userId}` | perm `users.manage` | 200 `User`; 400; 404; 409 (removing own `super_admin`, disabling self, or leaving zero active Super Admins) |
| `createPasswordReset` | `POST /users/{userId}/password-reset` | perm `users.manage` +step-up | 200 `TemporaryPassword`; 403; 404; 409 (own id) |
| `deleteUserTotp` | `DELETE /users/{userId}/totp` | perm `users.manage` +step-up | 204; 403; 404; 409 (own id) |

The contract PR adds `components/parameters/Page` and `PageSize` (per `module-contract` section 2), `components/headers/RetryAfter` (integer seconds) and the three shared responses above. `listUsers` defaults: page 1, pageSize 10, maximum pageSize 50. `userId` is `int64`, minimum 1.

Schemas for the contract PR (moved into `openapi.yaml` and replaced by a link in that PR):
- `Role` enum (5 slugs); `PermissionArea` enum (9); `PermissionLevel` enum `[view, manage]`; `SessionStage` enum (4).
- `Permission {area, level}`.
- `Credentials {username, password}`.
- `SessionState {stage, user: SessionUser, permissions: Permission[], totpEnrolled: boolean, stepUpExpiresAt: date-time or null}`. `permissions` is empty unless the stage is `authenticated`. `SessionUser {id, username, fullName, roles}`.
- `TotpVerification {code?: 6-digit string, recoveryCode?: string}`: two optional fields (no `oneOf`, to keep the generated Go and TypeScript types plain); the handler answers 400 unless exactly one is present. `TotpCode {code}` is used by `createStepUp` and `updateTotpEnrollment`. `createRecoveryCodes` takes no request body: the step-up rule is its check. **[agent call]**
- `TotpEnrollment {secret: base32, otpauthUri, qrCodePng: base64 PNG}`; `RecoveryCodes {codes: string[10]}`.
- `PasswordChange {currentPassword, newPassword}`; `StepUp {expiresAt}`.
- `User {id, username, fullName, roles, active, totpEnrolled, lastLoginAt (date-time or null), createdAt}`.
- `UserPage {items, page, pageSize, total}`.
- `UserCreate {username, fullName, roles (min 1, unique)}`; `UserUpdate {fullName, roles (min 1), active}`.
- `UserCreated {user, temporaryPassword}`; `TemporaryPassword {temporaryPassword}`.
- Field constraints (the generated server does not enforce ranges, so handlers re-check them): `username` pattern `^[a-z0-9._-]{3,32}$`; `password` and `newPassword` 12 to 128 characters; `code` pattern `^[0-9]{6}$`; `recoveryCode` max 32 characters; `fullName` 1 to 100 characters; `roles` `minItems: 1`, `uniqueItems: true`, items `Role`; `qrCodePng` is `format: byte`. `createSession` takes `Credentials` with `password` max 128 and any `username` string up to 64 characters: a malformed login answers 401, not 400, so it reveals nothing; 400 is only for an unparsable body. **[agent call]**

## Interface

The admin is an SPA: `web/src/routes/admin/+layout.ts` exports `ssr = false`. NFR-04 does not apply to the admin, so there are no `web-ssr` rows. Dates use a new `web/src/lib/format/date.ts` `formatDateTimeWita(iso)`: `Intl.DateTimeFormat` with locale `id-ID` and time zone `Asia/Makassar`, assembled from `formatToParts` as weekday, day, month, year, time with `.` separator, then the literal suffix `WITA` (the ICU zone name may render as `GMT+8`, so it is never taken from `Intl`). Example: "Kamis, 15 Oktober 2026 14.05 WITA". No such helper exists today.

| Route | Purpose |
|---|---|
| `admin/(masuk)/masuk` | Login. An already `authenticated` session redirects to `/admin`. `?kembali=` (path and query kept) is accepted only if it equals `/admin` or starts with `/admin/` and not with `/admin/masuk`; otherwise `/admin`. `?alasan=sesi-berakhir` shows the expiry notice. |
| `admin/(masuk)/masuk/verifikasi` | TOTP or recovery code (stage `totp_required`). |
| `admin/(masuk)/masuk/kata-sandi` | Forced password change. |
| `admin/(masuk)/masuk/autentikator` | Forced enrollment: QR `<img src="data:image/png;base64,...">`, manual key, then the recovery codes once. |
| `admin/(panel)/+layout.ts` | `getSession` guard. 401 goes to `/admin/masuk?kembali=<path>`. A pending stage goes to its stage page. |
| `admin/(panel)/+layout.svelte` | Nav filtered by `permissions`, the user's name, `Keluar`. |
| `admin/(panel)` | `Beranda` placeholder. |
| `admin/(panel)/akun` | Own account: change password, enroll TOTP, regenerate recovery codes (step-up). |
| `admin/(panel)/pengguna`, `pengguna/baru`, `pengguna/[id]` | User list (paged with `?halaman=`; 10 per page; a simple previous/next control, no shared pager exists yet), create (shows the temporary password once), edit roles and active flag, reset password and reset TOTP (both via step-up). The `pengguna*` loads check `permissions` for `users.manage` and otherwise render `Anda tidak memiliki akses ke halaman ini.` |

Each `(masuk)` stage page's `load` calls `getSession` and redirects when the stage does not match. A thin wrapper around the typed client in `web/src/lib/admin/` handles session loss: a 401 from any admin call goes to `/admin/masuk?alasan=sesi-berakhir`, except calls made by the pages that treat 401 as a normal answer (`createSession`, `createTotpVerification`, `updatePassword` and `deleteSession` handle it inline; `deleteSession` 401 counts as a successful logout). After an inline 401 from `createStepUp` the dialog calls `getSession` once: if that is 401 too the session is gone and the wrapper redirects, otherwise the code was wrong and `Kode tidak valid.` is shown. **[agent call]** A 429 anywhere shows the throttle label with `{n}` from `Retry-After`. A 403 whose `detail` is the step-up text reopens `StepUpDialog`; other 403s show `Anda tidak memiliki akses ke halaman ini.`

Components:
- `StepUpDialog.svelte` in `web/src/lib/admin/`: a callback prop `onverified`. It opens before a step-up operation when `stepUpExpiresAt` is null or in the past (the server returns `null` once expired; clock skew is handled by the 403 path above). `ppid` reuses it.
- `RecoveryCodesPanel.svelte`.
- `RoleBadge.svelte`.

Labels (NFR-16), verbatim:
- Login:
  - `Masuk Panel Admin`, `Nama pengguna`, `Kata sandi`, `Masuk`
  - `Nama pengguna atau kata sandi salah.`
  - `Terlalu banyak percobaan gagal. Coba lagi dalam {n} menit.` (n = ceil(Retry-After / 60))
  - `Sesi Anda berakhir. Silakan masuk kembali.`
- Verification: `Verifikasi Dua Langkah`, `Kode autentikator (6 digit)`, `Gunakan kode pemulihan`, `Kode pemulihan`, `Verifikasi`, `Kode tidak valid.`
- Password: `Ganti Kata Sandi`, `Kata sandi saat ini`, `Kata sandi baru`, `Ulangi kata sandi baru`, `Minimal 12 karakter.`, `Kata sandi terlalu umum.`, `Kata sandi tidak boleh memuat nama pengguna.`, `Kata sandi baru tidak cocok.`
- Enrollment:
  - `Aktifkan Autentikator`
  - `Pindai kode QR dengan aplikasi autentikator, lalu masukkan kode 6 digit.`
  - `Kunci manual`
  - `Kode Pemulihan`
  - `Simpan kode ini di tempat aman. Setiap kode hanya dapat dipakai sekali dan tidak akan ditampilkan lagi.`
  - `Saya sudah menyimpan kode ini`
- Shell: `Beranda`, `Akun Saya`, `Pengguna`, `Keluar`, `Anda tidak memiliki akses ke halaman ini.` The nav shows `Beranda` and `Akun Saya` to every session and `Pengguna` only with `users.manage`; other modules add their own entries.
- Account: `Kata sandi berhasil diganti.`, `Autentikator berhasil diaktifkan.`, `Buat Ulang Kode Pemulihan`, `Aktifkan Autentikator` (button)
- Generic: `Batal`, `Memuat...`, `Terjadi kesalahan. Coba lagi.`, `Belum ada pengguna.`, `Sebelumnya`, `Berikutnya`
- Server `detail` texts shown as is: `Nama pengguna sudah dipakai.` (409 `createUser`), `Kata sandi saat ini salah.`, the three password-policy texts and `Maksimal 128 karakter.`, `Kode tidak valid.` (400 `updateTotpEnrollment`)
- Users:
  - `Daftar Pengguna`, `Tambah Pengguna`, `Nama lengkap`, `Peran`, `Status`, `Aktif`, `Nonaktif`, `Autentikator`, `Terdaftar`, `Belum terdaftar`, `Terakhir masuk`, `Simpan`
  - `Atur Ulang Kata Sandi`, `Atur Ulang Autentikator`
  - `Kata sandi sementara: {pw}. Berikan langsung kepada pengguna; kata sandi ini tidak akan ditampilkan lagi.`
- Step-up: `Konfirmasi Identitas`, `Masukkan kode autentikator untuk melanjutkan.`
- Roles: the 5 labels from "Roles and permissions".

## Test list

Written before the BE and FE lanes start. Each lane PR fills the "Test name" column for its rows. Go tests that need Postgres run through `NewHandler` with httptest on `TEST_DATABASE_URL` and skip under `-short`; rows marked (unit) run without a database.

| Id | Behaviour | Layer | Test name |
|---|---|---|---|
| D1 | `users` rejects username `Ab` and a duplicate username | `db` | |
| D2 | `user_roles` rejects an unknown role and a duplicate `(user_id, role)` | `db` | |
| D3 | `users` rejects `totp_confirmed_at` without `totp_secret` | `db` | |
| D4 | `app` has no DELETE on `users` or `data_keys`, and cannot UPDATE `data_keys.id` or `created_at` (`privileges.sql`) | `db` | |
| D5 | One `queries.sql` case per new query (the BE lane lists them) | `db` | |
| A1 | `Can` matches all 45 cells of the matrix (unit) | `api` | |
| A2 | Union for 3 role pairs: `humas_editor`+`document_operator`; `document_operator`+`ppid_officer` (documents = manage, so M overrides V); `ppid_officer`+`leadership` (unit) | `api` | |
| A3 | Every `StrictServerInterface` method (reflection) has an `operationRules` entry, and `GetHealth` is public (unit) | `api` | |
| A4 | `listUsers`: anonymous gives 401, `humas_editor` gives 403, `super_admin` gives 200 | `api` | |
| A5 | An operation absent from `operationRules` gives 403 (the test injects a rule map) | `api` | |
| A6 | Correct login for `humas_editor` without TOTP gives 200 `authenticated`. Set-Cookie is HttpOnly, SameSite=Strict, and Secure when configured. The token differs from the pre-login one | `api` | |
| A7 | Wrong password and unknown username give byte-identical 401 bodies | `api` | |
| A8 | The 5th wrong password answers 429 with `Retry-After: 900` (failures 1 to 4 answer 401); a 6th attempt with the correct password also gives 429. After the clock advances 15 min the correct password gives 200 | `api` | |
| A9 | After a block expires, one more failure answers 429 with `Retry-After: 1800`; the delay is capped at 24 h; a failure more than 24 h after the previous one restarts the count at 1 | `api` | |
| A10 | 5 failures across 5 usernames from one IP give 429 for a 6th username | `api` | |
| A11 | Success deletes the account throttle row and keeps the IP row | `api` | |
| A12 | A disabled user gets 401 at login; an existing session of a disabled user gets 401 on the next request | `api` | |
| A13 | Stage order: a TOTP user gets `totp_required`; a new `ppid_officer` gets `password_change_required` then `totp_enrollment_required`. Admin operations give 401 while pending | `api` | |
| A14 | A pending stage older than 10 min gives 401 | `api` | |
| A15 | TOTP verify: valid gives `authenticated` with a renewed token; wrong gives 401; replay of the same code gives 401; the 5th wrong code gives 429 | `api` | |
| A16 | A recovery code works once; its reuse gives 401 | `api` | |
| A17 | Enrollment returns secret, URI and PNG. Confirming returns 10 codes in `xxxxx-xxxxx` format. `users.totp_secret` starts with `0x01` and is not the base32 secret. It opens with the right AAD and fails with another user's AAD | `api` | |
| A18 | `createRecoveryCodes` invalidates the old codes | `api` | |
| A19 | Password policy rejects 11 characters, 129 characters, a listed common password and a password containing the username (400). It accepts a 12-character Unicode password | `api` | |
| A20 | The stored hash has prefix `$argon2id$v=19$m=19456,t=2,p=1$`. A hash with older parameters is rehashed on login | `api` | |
| A21 | `Hasher` never runs more than 2 hashes concurrently (5 goroutines, instrumented hook) (unit) | `api` | |
| A22 | Idle timeout: with `AUTH_SESSION_IDLE_TIMEOUT=1s`, a request after 1.5 s gives 401. Absolute lifetime: with `AUTH_SESSION_LIFETIME=2s` and activity every 0.5 s, the request after 2.5 s gives 401 (real clock) | `api` | |
| A23 | Logout, then the old cookie gives 401 | `api` | |
| A24 | A POST with `Origin: https://evil.example` gives a 403 Problem; a same-origin POST passes | `api` | |
| A25 | Admin responses carry `Cache-Control: no-store` | `api` | |
| A26 | `createPasswordReset`, `createRecoveryCodes` and `deleteUserTotp` without step-up give 403 with the step-up detail. After `createStepUp`, `createPasswordReset` gives 200 and the target's old session gives 401 (epoch). After 10 min it gives 403 again | `api` | |
| A27 | `createStepUp` for a user without TOTP gives 403 | `api` | |
| A28 | `updateUser` gives 409 for self-demotion, self-disable and the last active Super Admin; `createPasswordReset` and `deleteUserTotp` on the caller's own id give 409 | `api` | |
| A29 | `createUser` gives 409 for a duplicate, 400 for an unknown role, 400 for empty roles. 201 returns a temporary password that works, and the new user is in `password_change_required` | `api` | |
| A30 | `listUsers` gives 400 for `page=0`, `page=abc` and `pageSize=51`. Defaults are page 1, size 10, and `total` is correct | `api` | |
| A31 | `getUser` with an unknown id gives 404 | `api` | |
| A32 | After `deleteUserTotp`, a mandatory-role user's next login reaches `totp_enrollment_required` | `api` | |
| A33 | `SessionState.permissions` of a two-role user equals `Effective` | `api` | |
| A34 | Captured logs of a failed login and a TOTP failure contain no password, code or session token | `api` | |
| A35 | Keyring: `Load` fails naming the kek id when the KEK is missing. `rotate-dek` then `Open` of old ciphertext works. `rewrap` then `Load` with only the new KEK works | `api` | |
| A36 | CLI `admin create` creates a `super_admin` with `password_change_required`; `admin reset-totp` clears TOTP and the recovery codes | `api` | |
| A37 | Wrong stage: no session gives 401; a pending session on a permission operation gives 401; an `authenticated` session on `createTotpVerification`, and `password_change_required` on `createTotpEnrollment`, give 403 | `api` | |
| A38 | `updatePassword`: a wrong current password gives 400 `Kata sandi saat ini salah.` and counts toward the throttle; success ends the user's other sessions, keeps this one with a renewed token and the new epoch, and re-resolves the stage | `api` | |
| A39 | `createTotpEnrollment` after confirmation gives 409; `updateTotpEnrollment` with a wrong code gives 400, with no pending enrollment gives 409; a repeated `createTotpEnrollment` before confirmation replaces the pending secret | `api` | |
| A40 | A wrong step-up code gives 401 and counts toward the throttle; a correct code used at login cannot be reused for step-up in the same 30 s step | `api` | |
| A41 | `X-Real-IP` sets the throttle IP only when `AUTH_TRUST_PROXY_HEADER=true`; otherwise `RemoteAddr` is used and the header is ignored | `api` | |
| A42 | `AUTH_COOKIE_SECURE=false` yields cookie `bpl_session` without `Secure`; `true` yields `__Host-bpl_session` with `Secure` | `api` | |
| A43 | `configFromEnv` applies the defaults of "Configuration and CLI" and fails on an invalid duration, integer or boolean (unit) | `api` | |
| A44 | `createSession` with an unparsable body gives 400; a username that does not match the pattern gives the same 401 as a wrong password and counts against the IP key only | `api` | |
| A45 | A cross-origin POST without a session gives 403, not 401 (middleware order) | `api` | |
| A46 | `GET /api/v1/health` with an invalid or foreign session cookie gives 200 and makes no session query | `api` | |
| W1 | The `(panel)` guard maps 401 to a redirect to `/admin/masuk?kembali=...`, and each pending stage to its page (injected fetch, fixtures typed from the contract examples) | `web-load` | |
| W2 | A stage page with a mismatching stage redirects | `web-load` | |
| W3 | `kembali` values `//evil`, `https://x` and `/publik` resolve to `/admin` | `web-load` | |
| W4 | The login form shows the 401 and 429 messages (minutes from `Retry-After`) and navigates on 200 | `web-component` | |
| W5 | Verification toggles to the recovery-code input | `web-component` | |
| W6 | Enrollment renders the QR and manual key, then the recovery codes; continuing requires `Saya sudah menyimpan kode ini` | `web-component` | |
| W7 | The nav shows `Beranda` and `Akun Saya` to every session and `Pengguna` only with `users.manage`; `/admin/pengguna` without it shows `Anda tidak memiliki akses ke halaman ini.` | `web-component` | |
| W8 | The user list shows Indonesian role labels and pages | `web-component` | |
| W9 | Create user shows the temporary password once | `web-component` | |
| W10 | `StepUpDialog` opens before a reset when `stepUpExpiresAt` is null, calls `onverified` after success, and reopens on a 403 with the step-up detail | `web-component` | |
| W11 | A 401 from a panel call redirects with `alasan=sesi-berakhir`; a 401 from `createSession`, `createTotpVerification` or `updatePassword` shows its inline message instead; a 401 from `deleteSession` counts as a successful logout | `web-component` | |
| W12 | `formatDateTimeWita` formats the instant 2026-10-15T06:05:00Z as `Kamis, 15 Oktober 2026 14.05 WITA` (pure function, plain `*.spec.ts`) | `web-load` | |
| W13 | The `akun` page changes the password, enrolls TOTP (QR, confirm, recovery codes once) and regenerates recovery codes through the step-up dialog | `web-component` | |
| W14 | `pengguna/[id]` saves roles and the active flag, and resets password and TOTP through the step-up dialog; the temporary password is shown once | `web-component` | |
| W15 | `Keluar` calls `deleteSession` and goes to `/admin/masuk`; after login a valid `kembali` is honoured | `web-component` | |

### ASVS level 2 checklist map

Chapter numbers from ASVS 4.0.3; confirm each against the standard during review.

| ASVS area | Test ids |
|---|---|
| V2.1 password security | A19, A20 |
| V2.2 anti-automation | A8, A9, A10, A38, A40 |
| V2.5 credential recovery | A16, A26, A36 |
| V2.8 one-time verifiers | A15, A17 |
| V3.2 session binding | A6, A15 |
| V3.3 logout and timeout | A22, A23 |
| V3.4 cookie attributes | A6, A42 |
| V4.1 deny by default | A3, A5, A37 |
| V4.2 CSRF | A24, A45 |
| V6.2 algorithms | A17, A35 |
| V7.1 no secrets in logs | A34 |
| V8.2 no-store | A25 |

## Hard-to-reverse decisions

Proposed rows for PRD Appendix D (decision | choice | reason | requirements). Humans edit the PRD master; do not edit `docs/prd/PRD.md` here.

| Decision | Choice | Reason | Requirements |
|---|---|---|---|
| Ids | `bigint identity` | PostgreSQL 16 and 18 are both supported; `uuidv7` was rejected | NFR-14 |
| Login identifier | Username only, no email | OQ-11; about a dozen accounts | FR-33 |
| RBAC storage | Matrix in Go, roles as checked text | The 45-cell test is auditable in a PR | FR-33 |
| Ciphertext v1 | Version, DEK id, nonce, ct+tag; row-bound AAD; DEKs wrapped by a file KEK | Key id inside the ciphertext allows rotation | NFR-17 |
| Session cookie and CSRF | `__Host-` cookie, SameSite=Strict, stdlib `CrossOriginProtection` | ASVS L2 without token plumbing | NFR-05 |
| Login throttle | Per account and per IP, doubling from 15 min, 24 h cap, no lock | Progressive delay without a denial-of-service lock-out | FR-33, NFR-05 |
| Step-up | TOTP only, 10 min | Gate for personal-data access via `ppid` | NFR-08 |
| Password hash | argon2id m=19456, t=2, p=1, rehash on login | OWASP minimum, tunable later without migration | NFR-17 |

## Dependencies needing approval

Each needs human sign-off (`AGENTS.md`: ask before adding a dependency):
- `github.com/oapi-codegen/runtime`: contract PR; path and query binding for `getUser` and `listUsers`.
- `github.com/alexedwards/scs/v2` and `github.com/alexedwards/scs/pgxstore`: BE.
- `golang.org/x/crypto`: BE, argon2.
- `github.com/pquerna/otp`: BE, TOTP and QR PNG.
- The FE adds none (the QR comes from the server).
