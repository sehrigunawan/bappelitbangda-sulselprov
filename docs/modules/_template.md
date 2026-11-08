# Module: <module slug>

<!-- Copy to docs/modules/<module>.md. One to two pages (PRD Appendix D). Living document: any PR that changes the module's behaviour updates it in the same PR. Link to openapi.yaml, never copy schemas. -->

- **Requirements:** FR-nn, NFR-nn (ids exactly as in `docs/prd/PRD.md`)
- **Depends on open questions:** OQ-nn (or "none")
- **Security lane:** yes / no (auth, sessions, RBAC, TOTP, KTP encryption, keys: the human drafts, the other reads every line)

## Data

Tables, columns, constraints and indexes the module needs. Foreign keys get an index; say which tables the app role must not edit or delete.

| Table | Columns | Constraints and indexes |
|---|---|---|
| | | |

## API

Operations live in `openapi.yaml` (tag = module slug). This table only names them.

| operationId | Method and path | Auth | Responses |
|---|---|---|---|
| `list<Thing>` | `GET /api/v1/<module>` | public | 200 `<Thing>Page`, default Problem |

## Interface

Routes, components, and the Indonesian labels (NFR-16). Content pages must render without JavaScript (NFR-04).

| Route | Component | Notes |
|---|---|---|
| | | |

## Test list

Written before the BE and FE lanes start. Each lane PR fills the "Test name" column for its rows.

| Id | Behaviour | Layer | Test name |
|---|---|---|---|
| T1 | | `db` / `api` / `web-load` / `web-component` / `web-ssr` | |

## Hard-to-reverse decisions

Proposed rows for PRD Appendix D (decision | choice | reason | requirements). Humans edit the PRD master; do not edit `docs/prd/PRD.md` here.

| Decision | Choice | Reason | Requirements |
|---|---|---|---|
| | | | |
