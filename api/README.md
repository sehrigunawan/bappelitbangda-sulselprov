# api

Go backend (chi, pgx, sqlc, goose). Handlers implement the interface generated from `../openapi.yaml`.

## Run

```sh
make db-up db-init db-migrate      # from the repo root
cd api && go run ./cmd/api         # listens on :8080 (API_PORT)
curl localhost:8080/api/v1/health
```

## Environment

| Variable | Purpose | Default |
|---|---|---|
| `API_PORT` | HTTP port | `8080` |

Database settings are added with the first feature that uses the database. See `../.env.example`.

## Layout

- `cmd/api` entry point; `internal/server` handlers; `internal/api` and `internal/db/dbgen` are **generated**.
- `db/migrations` goose migrations (never edit one that is on `development`), `db/queries` sqlc queries, `db/tests` SQL tests, `db/init` dev roles.
- Dev tools (goose, sqlc, oapi-codegen, govulncheck, tbls, gitleaks) are pinned in `../tools/go.mod`; use `make`.

## Test

```sh
make test         # fast, no database
make test-full    # includes tests that use PostgreSQL (needs make db-up)
make db-test      # SQL tests in db/tests
```

Design and decisions: see ../docs/
