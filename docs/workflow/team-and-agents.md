# Team and agent workflow

How the two of us work in this repo, with and without AI coding agents. Agent rules live in `AGENTS.md`; this file holds only what people need beyond them. Commands: `make` (see `AGENTS.md`). CI: `.github/workflows/`. Branch protection: `.github/rulesets/`. Release and deploy: `docs/handover/runbook.md`.

## Principles

1. **Review attention is the bottleneck, not code generation.** Keep PRs small. There is no cap on open branches or PRs; the other person's review time is the limit.
2. **Contract first.** `openapi.yaml` and the migrations are the fence; CI proves nothing drifted.
3. **`make verify-full` is the definition of done.** CI is the gate; prompts and rules files are reminders. A rule that must never break belongs in CI or a deny rule.
4. **The security lane is never delegated.** Auth, sessions, KTP encryption and key handling are read line by line by the person who did not write them.
5. **One source of truth per fact.** Link, do not copy.
6. **Built for handover.** Diskominfo staff who did not write the code must be able to run, patch and restore it.

## Roles

| Area | Writes | Reviews before merge |
|---|---|---|
| `api/` (handlers, queries, migrations) | BE | FE |
| `web/` | FE | BE |
| `openapi.yaml` | whoever holds the open contract task | the other, same day |
| Auth, sessions, RBAC, TOTP, KTP encryption, keys | BE drafts | both read every line |
| Makefile, CI, `AGENTS.md`, `.claude/`, `.agents/`, handover docs | shared, by PR | the other |

`CODEOWNERS` lists every developer on `*`, so the other person is always the required reviewer. The rulesets on `development` and `main` require that review (code-owner review, and the last pusher cannot approve). It has one handle today; add the second developer's handle on the same line when they join, then raise required approvals to 1 (`scripts/github-apply-settings.sh --approvals 1`).

## Data and secrets

- Agents work against a local dev database with synthetic data. No production credentials, real KTP numbers, real `.env` or VPS access in any agent session.
- `.env` is git-ignored; `.env.example` holds fake values. Production `.env`, the database password and the KEK live only on the VPS; the KEK is held by two named people, one a Diskominfo officer, and is never in the repo or in backups. Roles are in `docs/handover/OWNERS.md`.
- If a secret is ever committed or pasted into an agent session, treat it as leaked: rotate first, clean up after.
- Deny rules reduce accidents but are not a security boundary for shell commands; use a dev container or the sandbox where possible.
- Whether source code may go to an external AI service is an open question for Diskominfo (see `docs/handover/known-gaps.md`). Until answered, no real data reaches an agent.

## Module loop

A module is built contract first, then tests first. Skills `module-contract` and `module-tdd` (`.agents/skills/`) carry the detail; this table is the process.

| Phase | Session and subagents | Isolation | Human gate |
|---|---|---|---|
| Spec: `docs/modules/<module>.md` from `docs/modules/_template.md` | interactive session; read-only scout subagents for PRD rows, Appendix A and reusable code | main checkout | the other developer approves the spec |
| Contract PR: spec, `openapi.yaml`, generated code, 501 stubs | same session; a reviewer subagent before the PR opens | `../wt/<contract-issue>` | review the same day; merged into `development` |
| BE lane and FE lane, in parallel, started after the contract merged | one session each; scout and implementer subagents inside a lane only on disjoint files | `../wt/<be-issue>` and `../wt/<fe-issue>` | plan read, then the red checkpoint, then PR review |
| Security-lane modules | the agent drafts only; no subagent-driven execution | worktree | both developers read every line |

Inside a lane:

1. **Plan.** The agent writes `.plans/<issue-id>-<lane>.md` (git-ignored) from the module spec's test list; the human reads it; implementation starts in a fresh session in the same worktree.
2. **Tests first, in one batch.** The agent writes all failing tests for the lane's rows of the spec test list, with empty stubs so they fail on assertions, runs the focused target (`make test-api RUN=... PKG=...`, `make db-test`, `make test-web FILE=...`) and pastes the output.
3. **Red checkpoint.** The human checks that every test fails for the right reason (for example `status = 501, want 200`). Only then does implementation start. A red test proves something only when someone checks why it is red; per-test red/green inside the agent loop was rejected (`docs/decisions/agent-workflow-2026-11.md`).
4. **Implement, verify.** `make verify` (and `make verify-full` for SQL or migrations) until it exits 0. Read edited tests for weakened assertions.
5. **PR** from the template: spec link, red output, the plan's task list under "## Plan", "what I do not yet understand about this diff". A reviewer subagent may run first and never replaces the human.
6. **Review** by the other person, **merge** by a human into `development` (squash). CI reruns on `development`. Remove the branch and worktree.

A contract defect found in a lane stops the lane: report it, fix it in a new contract PR, rebase the lane. FE lanes test against fixtures typed from the contract `example`s; there is no mock server.

Other tasks (`fix`, `chore`, `docs`): issue, branch, plan, `make verify`, PR, review, merge as above, without the contract and red-checkpoint steps unless behaviour changes (then the failing test comes first and its output goes in the PR).

## Agent harnesses

Rules are written once and read by whichever agent tool a developer uses.

- `AGENTS.md` at the root is canonical. Nested `api/AGENTS.md` and `web/AGENTS.md` hold the subsystem rules (read natively by Codex, Copilot, Cursor and OpenCode; the root file tells every other tool to read them). Claude Code reads `AGENTS.md` through `CLAUDE.md`, which only imports it, and loads the nested files when it opens a file there. Gemini CLI is pointed at `AGENTS.md` by `.gemini/settings.json` (not tested here; Gemini CLI is not installed on our machines).
- omp task subagents drop files named `AGENTS.md` but keep `@`-imported text, so `CLAUDE.md` importing `AGENTS.md` is what carries the rules into them. Keep that file.
- Skills live in `.agents/skills/<name>/`, the path most tools scan. Claude Code reads only `.claude/skills/`, so each skill has a relative symlink there. `make skills-check` fails on a missing link, a name that differs from the directory, a description over 1024 characters or a `SKILL.md` over 500 lines. Some tools read both directories and may list a skill twice; the entries point to the same file.
- `.agents/` contains only `skills/`. Subagent definitions, hooks, MCP servers and permissions have no shared format and are not committed per tool. Lane roles are described in the skills and run through each tool's own subagent mechanism. Do not use Claude Code's `isolation: worktree` subagents for lanes: their worktree path differs from `../wt/`.
- `.claude/settings.json` deny rules are the only committed permission config and apply to Claude Code only. They reduce accidents, not risk. The enforcement for every tool is CI: `gen-check`, migration immutability, `contract-lint`, `skills-check`, and the review rules.
- Vendored skills are adapted from obra/superpowers (MIT); see `.agents/skills/README.md`. A globally installed superpowers plugin can list its own copies of the same skills (omp showed both, the plugin's under a `superpowers/` prefix). Disable a globally installed superpowers plugin while working in this repo; the repo copies are adapted to its rules.

## Branches and parallel work

1. Every branch starts from an issue. Name: `feat/<issue-id>-<module>-contract`, `feat/<issue-id>-<module>-be` (BE owns migrations, queries, handlers) and `feat/<issue-id>-<module>-fe` for module work; `fix|chore|docs/<issue-id>-<slug>` otherwise. No issue, no branch: open a one-line issue. Each developer cuts their own short-lived branches from `origin/development`; there are no per-person or per-lane long-lived branches. The only fixed order is technical: a module's BE and FE branches start after its contract PR is merged into `development`.
2. Create from fresh `development`: `git fetch origin && git switch -c <branch> origin/development`. If a second session runs at the same time, use a worktree instead: `git worktree add ../wt/<issue-id> -b <branch> origin/development`. A worktree isolates files only, so run the dev stack (ports, dev database) from one checkout at a time. `make verify` and `make db-test` use throwaway databases and are safe in parallel.
3. One task, one branch, one PR, one worktree. When the PR merges: `git worktree remove` and `git branch -d`.
4. Every branch opens its PR into `development`. The PR title becomes the squash commit and the release note, so it must name the change. The PR body has `Closes #<issue-id>` (it fires on merge into `development`, the default branch). Contract, BE and FE changes go in separate PRs (branches `-contract`, `-be`, `-fe`); migrations ride in the BE PR. Merging needs one approval from the other developer; the author never self-merges without it.
5. Assign the issue to yourself before branching, so the other person sees who works on it.
6. No caps on open branches or PRs. Migrations: `make db-new` numbers sequentially, so two open BE branches can produce the same number. After rebasing onto `origin/development`, if your migration's number already exists there, rename your file to the next free number (allowed: it is not yet on `development`); CI's goose run fails on duplicates.
7. Avoid stacked PRs. Contract first already lets BE and FE work in parallel. If a stack is unavoidable, keep it to depth 2, never merge branches into each other (the promotion merge commit on `main` is created by GitHub, not by an agent), and after the parent is squash-merged run `git rebase --onto origin/development <parent>` on the child. Agents never force-push.

Traceability chain: PRD id, issue, branch name, PR (`Closes #n`), squash commit on `development`, promotion PR, release note.

## Promotion: `development` to `main`

`development` is the integration branch; `main` is production. A maintainer opens a PR `development` → `main` when `development` is green and a release is planned. It is merged with a merge commit only (squash would make the branches diverge), after the same review as any PR. The `promotion-source` CI check fails any PR into `main` whose head is not this repository's `development`. Tags and releases stay on `main` (`docs/handover/runbook.md`). There is no hotfix bypass: a fix goes through `development` like everything else. Agents never open or merge promotion PRs.

## Contract and database

Rules are in `AGENTS.md`, `api/AGENTS.md` and `web/AGENTS.md`. Also: breaking API changes (renamed field, changed status code) need a note in the PR and the FE reviewer's approval; risky schema changes use expand and contract across two releases. Contract PRs are checked by `make contract-lint` (blocking) and `make contract-diff` (report only).

## Security lane checklist (the PR template ticks these)

- An agent may draft; the author explains every line and the other person reads every line.
- Design is fixed by PRD Appendix D: `scs` + `pgxstore` sessions, argon2id with at most 2 concurrent hashes (re-check OWASP parameters), `Can(user, perm)` deny by default, progressive delay per account and IP and never a permanent lock (FR-33), TOTP for Super Admin and Petugas PPID, KTP step-up and audit, KEK outside repo and backups, key id inside the ciphertext.
- Encryption uses stdlib AES-GCM with envelope encryption. No custom crypto; nonce and key handling are never left to an agent unreviewed.
- Tests exercise the real auth path plus negative authorization cases; no test-only bypass.
- Before go-live: OWASP ASVS level 2 for the admin panel, authentication and the PPID module (NFR-05), level 1 elsewhere; `govulncheck` clean; an independent review or a network-restricted admin panel.

## Repository settings (a human applies them; agents never change settings)

`.github/rulesets/main.json`, `.github/rulesets/development.json` and `scripts/github-apply-settings.sh` (dry run first, then `--apply`) are the source of truth for branch and tag protection, the default branch (`development`) and merge methods. Beyond them:

- Own the repository through an organization with at least two owners and required two-factor authentication. The repository is currently public under a personal account (see `docs/decisions/prd-v1.1-reconciliation.md` item 6). Move it to an organization or a Diskominfo Git host before handover.
- No outside collaborators with write access. No forking of private repositories. Wiki, Projects and Discussions off.
- Actions: read-only default token, third-party actions pinned by full commit SHA, "allow Actions to create and approve pull requests" off, no secrets other than `GITHUB_TOKEN`, no webhooks or integrations unless added in a PR that says why.
- Secret scanning with push protection if the plan offers it; otherwise `gitleaks` (already in `make`).
- Once a week a maintainer runs `git clone --mirror` to a location Diskominfo or the VPS owner controls, so the code does not live only in one organization.
- With two organization owners, an owner can still edit the rules. The ruleset prevents accidents and unreviewed pushes, not a deliberate owner. An emergency fix uses the same path (PR, green CI, review); if a rule truly blocks it, change the rule deliberately, note it in `docs/handover/deploy-log.md`, and restore it right after.
- If the host or plan cannot enforce a control, write it in `docs/handover/OWNERS.md` and enforce what is possible elsewhere.
- A red `development` or `main` is the top priority. A flaky test is fixed or quarantined with an issue within a day.

## Rhythm

- Daily, 10 minutes: who holds the contract today, open tasks, blockers.
- Friday, 30 minutes: explain-back. Each of us walks the other through a module they did not write. This is how we avoid code nobody understands at handover. Update the runbook and `known-gaps.md`.
- Weekly: close stale branches and worktrees. Once a week, write one small core piece by hand.

## Handover checklist

- One-command local run; READMEs accurate; a Diskominfo staff member cold-starts it from the README on a clean machine (NFR-14).
- Runbook complete: deploy, rollback, backup and restore, key rotation, add an admin, TOTP break-glass.
- Restore drill done by Diskominfo staff, including decrypting a test KTP with the key.
- Dependencies are explained by `api/go.mod`, `web/package.json` and the SBOM; add the reason in the PR that adds each one.
- `docs/handover/OWNERS.md` names the system owner and the patch owners (application, OS/VPS).
- Known gaps listed honestly.
