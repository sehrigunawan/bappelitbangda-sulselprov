---
name: module-contract
description: Use when starting a new module, adding or changing an API endpoint, or editing openapi.yaml.
---

# Module contract PR

The contract PR is the first PR of a module. It carries the module spec, `openapi.yaml`, the generated code and 501 stubs. It contains no handler logic, no migration and no web page. After it merges into `development`, the BE and FE lanes start (`module-tdd`).

`AGENTS.md` outranks this skill. In particular: changing the contract beyond the task, adding a table or a column, and anything in auth, sessions, KTP encryption or keys needs the human first. Security-lane modules: the human drafts the spec; you only ask questions.

## 1. Spec

1. Branch `feat/<issue-id>-<module>-contract` from fresh `origin/development`, in a worktree `../wt/<issue-id>` (`using-git-worktrees`).
2. Run `brainstorming` with the human to settle scope. Meanwhile dispatch read-only subagents in parallel (one message) for:
   - the PRD rows for the module's FR/NFR ids (`docs/prd/PRD.md`, `docs/prd/traceability.md`);
   - the Appendix A inventory rows (old site data) for the module;
   - existing code, queries and components to reuse.
3. Create `docs/modules/<module>.md` from `docs/modules/_template.md` and add its line to `docs/modules/README.md`. The API section lists operationIds and links to `openapi.yaml`; it never copies schemas. The test list is complete before the lanes start. Requirement ids are exactly as in the PRD; an open question that blocks a choice is written "depends on OQ-nn" and the choice waits.
4. A human approves the spec before you edit `openapi.yaml`.

## 2. Contract conventions

- Paths: public `/api/v1/<module>`; admin `/api/v1/admin/<module>` (admin endpoints wait for the FR-33 auth module).
- The generated server binds query and path parameters with `github.com/oapi-codegen/runtime`. The first operation with a parameter (the first list endpoint) makes `api/internal/api` import it, and `api/go.mod` does not have it yet: ask the human before `go get`, note the dependency and its reason in the PR. The generated code binds and type-checks parameters (a malformed `?page=abc` becomes 400 `Problem` through `NewHandler`'s `ErrorHandlerFunc`; put that case in the spec test list) but does not enforce `minimum`, `maximum` or `default` ranges (a pilot `?page=0` reached the handler); the BE lane validates them and answers 400 `Problem`, so put those cases in the spec test list too.
- `operationId` is `list|get|create|update|delete<Thing>`. One tag per module, equal to the module slug; declare it in the top-level `tags:` list.
- Every operation has `summary`, `description`, an `example` on each response body, and `default: $ref '#/components/responses/Problem'`. A missing resource is 404 with `components/responses/Problem`. Every schema and property has a `description` and an `example` (`make contract-lint` enforces examples).
- List endpoints use `components/parameters/Page` (`page`, integer >= 1, default 1) and `PageSize` (`pageSize`, 1 to 50, default 10) and return `<Thing>Page` with `items`, `page`, `pageSize`, `total`, all required. Add `Page` and `PageSize` to `components.parameters` with the first list endpoint. Numbered pagination serves FR-10 and FR-15.
- Dates are `format: date-time`, UTC.
- OpenAPI stays 3.0.3 (PRD Appendix D).
- Do not hand-edit generated code (`api/internal/api`, `web/src/lib/api`); `make gen` produces it.

## 3. Stubs

Create `api/internal/server/<module>.go`: one method per new operation on `Server`, returning `nil, ErrNotImplemented`. The generated strict interface will not compile without them, and `internal/server/errors.go` turns the sentinel into a 501 `application/problem+json`.

```go
func (Server) ListThings(_ context.Context, _ api.ListThingsRequestObject) (api.ListThingsResponseObject, error) {
	return nil, ErrNotImplemented
}
```

## 4. Gates

Run, in this order, and read the output:

1. `make gen`
2. `make contract-lint` (blocking; fix the contract, never weaken `.vacuum.yaml`)
3. `make verify` (paste the tail)
4. `make contract-diff BASE=origin/development` (report only). A breaking change on an existing operation (renamed field, changed status code) needs a note in the PR and the FE developer's approval.

## 5. PR

Open the PR into `development` with the repo template: the spec link, `Closes #<issue-id>`, the breaking-change note if any. It contains the spec, `openapi.yaml`, the generated files and the stubs, nothing else. Before opening, run the reviewer subagent from `requesting-code-review`; it never replaces the human, who reviews the same day. Finish with `finishing-a-development-branch`. Never merge.

## Stop and ask

- A choice that depends on an open question (OQ-nn).
- A change that touches auth, sessions, KTP encryption or keys.
- A breaking change to an operation another lane already uses.
