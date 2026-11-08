# Go backend rules
- Handlers implement the generated strict-server interface (`api.StrictServerInterface`); never hand-edit generated code.
- Database access only through sqlc-generated queries in `api/db/queries/`.
- Authorization goes through `Can(user, perm)`, deny by default. No ad-hoc role checks.
- Sessions use scs + pgxstore. Passwords use argon2id with at most 2 concurrent hash operations.
- KTP numbers and TOTP secrets are ciphertext with a key id; never log them; never choose nonce or key handling yourself; draft only, a human explains every line.
- Return errors with context; no panics in handlers; no global state.
- Errors: expected failures (404, 409, validation) are typed response objects from the generated interface, with the `Problem` body. A returned `error` is a bug: `internal/server/errors.go` logs it with the request id and answers a generic 500. Contract-PR stubs return `ErrNotImplemented` (501).
- DB tests call dbtest.New(t) (one database cloned from the migrated template, role app plus a superuser pool) and skip under testing.Short().
