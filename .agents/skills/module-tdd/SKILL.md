---
name: module-tdd
description: Use when implementing a module's backend or frontend lane after its contract PR is merged into development.
---

# Module lane: tests first, human red checkpoint, then implementation

A module has two lanes after its contract PR merged into `development`: BE (`feat/<issue-id>-<module>-be`: migrations, queries, handlers) and FE (`feat/<issue-id>-<module>-fe`). Each lane is one session, one worktree, one PR. They run in parallel because the contract fixes the interface between them.

Read `api/AGENTS.md` or `web/AGENTS.md` first. `AGENTS.md` outranks this skill.

## Lane setup

1. `using-git-worktrees`: `git worktree add ../wt/<issue-id> -b feat/<issue-id>-<module>-<be|fe> origin/development`.
2. `writing-plans`: write `.plans/<issue-id>-<lane>.md` (git-ignored) from the module spec's test list. The human reads the plan.
3. Execute in a fresh session in the same worktree: `subagent-driven-development`, or `executing-plans` when the harness has no subagents. Parallel agents only on disjoint files.
4. NEVER edit `openapi.yaml` in a lane. A contract defect is a stop condition.

## BE lane

Tests first, in one batch, for the spec rows with layer `db` or `api` (`test-driven-development`):

- SQL `DO $$ ... ASSERT` blocks in `api/db/tests/{constraints,queries,privileges}.sql`, following `api/db/tests/queries.sql`.
- `api/internal/server/<module>_test.go`: table-driven httptest through `server.NewHandler(...)` against `TEST_DATABASE_URL`, skipping under `testing.Short()` (pattern: `server_test.go`, `internal/db/db_test.go`). Seed rows with SQL; literal expected status and body.
- The first DB-backed module adds `q *dbgen.Queries` to `Server` and changes `NewHandler(logger *slog.Logger, pool *pgxpool.Pool)`; `cmd/api/main.go` opens the pool from `DATABASE_URL` (`.env.example`) and both callers are updated. Ask the human first if that adds a dependency.

Red checkpoint:

1. `make test-api RUN='Test<Module>' PKG=./internal/server/...` must fail with `status = 501, want ...` (the stubs).
2. `make db-test` must fail with `relation "<table>" does not exist`.
3. Paste both outputs and STOP. The human confirms every failure is for the right reason. Do not continue on your own.

Implement, only after the human confirms:

1. `make db-new name=...` (Up and a working Down, `COMMENT ON TABLE`, an index on each foreign key, grants), `api/db/queries/<module>.sql`, `make db-generate`.
2. Replace the stubs with handlers that use the generated queries only.
3. Green: the focused targets, then `make verify-full` (needs `make db-up`) and `make db-docs`.
4. Mutation check from `test-driven-development`.

## FE lane

1. Create empty `src/routes/<path>/+page.svelte` and `+page.ts` returning `{}`.
2. Tests first, in one batch, for the rows with layer `web-load`, `web-component` and `web-ssr`:
   - load tests in `src/routes/<path>/page.spec.ts` with an injected fetch (pattern: `src/lib/http/client.spec.ts`) and fixtures typed `components['schemas']['X']`, copied from the contract `example`s and covering the full structure;
   - component tests in `*.svelte.spec.ts` (vitest browser mode);
   - the no-JS test with `render` from `svelte/server` in a plain `*.spec.ts` (NFR-04).
3. Red checkpoint: `make test-web FILE=src/routes/<path>`; paste the output and STOP for the human.
4. Implement. Interface text is Indonesian (NFR-16). Run `make verify`.

## Finish

1. `verification-before-completion`: `make verify` (and `make verify-full` if SQL or migrations changed); paste the tail.
2. `requesting-code-review`: the reviewer subagent checks the diff against `docs/modules/<module>.md`, `AGENTS.md` and the nested `AGENTS.md`. It never replaces the human.
3. `finishing-a-development-branch`: the PR goes into `development` with the template, the pasted red output, the plan's task list under "## Plan", and `Closes #<issue-id>`.
4. Spec upkeep: fill the "Test name" column of this lane's rows in `docs/modules/<module>.md` in the same PR. Any PR that changes module behaviour updates the spec in that PR.

## Stop conditions

Stop and report to the human, do not work around:

- a contract defect (wrong status code, missing field, unclear example);
- a security-lane task (auth, sessions, RBAC, TOTP, KTP encryption, keys): draft only, no subagent-driven execution;
- a choice that depends on an open question: write "depends on OQ-nn";
- a test you want to weaken or delete: say so instead.
