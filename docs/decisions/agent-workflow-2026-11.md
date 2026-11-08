# Agent workflow decisions, November 2026

Why the module loop, the `development` branch and the vendored skills look the way they do. Condensed from five research passes by Gemini deep-search subagents plus a conflict audit of the superpowers skills. Claims are marked **verified** (a person or the agent read the primary source or ran it) or **agent-reported** (a research subagent's summary, not re-checked). Decisions rest on verified claims; agent-reported ones are context.

## Findings

| Finding | Status | Source |
|---|---|---|
| Agent-inner-loop TDD (red/green per test inside the agent) showed no quality or mutation gain at 3 to 8.5 times the tokens. Up-front design plus test-first helped, and a red test only proves something when someone checks why it is red. | verified | [Böckeler, TDD in the agent loop](https://martinfowler.com/articles/exploring-gen-ai/tdd-in-the-agent-loop.html) |
| Subagents help for read-only work and for isolated lanes with a fixed contract between them. Parallel agents editing coupled code fail. | agent-reported, consistent with the two cited posts | Anthropic multi-agent research post; [Cognition, Don't build multi-agents](https://cognition.ai/blog/dont-build-multi-agents) |
| The Agent Skills spec defines only the skill directory (`SKILL.md`, `name` equal to the directory, `description` up to 1024 characters, body under 500 lines, optional `scripts/ references/ assets/`). It defines no repository path. | verified | [agentskills.io/specification](https://agentskills.io/specification) |
| `.agents/skills/` is the path most tools share (Codex, Gemini CLI, Copilot, Cursor, OpenCode, Antigravity). Claude Code reads only `.claude/skills/` but accepts symlinked skill folders. | vendor docs read; per-tool behaviour only partly re-run (see below) | [Claude Code skills](https://code.claude.com/docs/en/skills), [memory](https://code.claude.com/docs/en/memory), vendor docs listed in the harness matrix |
| Subagent definitions, hooks, MCP servers and permissions have no shared format; each tool has its own directory. | agent-reported, vendor docs | harness matrix report |
| Specs are committed, living documents separate from change plans (OpenAI harness engineering: `docs/` with product specs, design docs and exec plans; OpenSpec: current truth in `openspec/specs/`, per-change proposals in `openspec/changes/`). | agent-reported | harness and spec reports |
| Adding any operation to `openapi.yaml` breaks the build (`var _ api.StrictServerInterface = (*Server)(nil)`), so a contract PR must carry 501 stubs. | verified (code) | `api/internal/server/server.go` |
| The generated strict server's default error handlers write `text/plain` and leak `err.Error()` on 500; `NewStrictHandlerWithOptions` replaces them. | verified (code, tests in `api/internal/server/errors_test.go`) | `api/internal/api/api.gen.go` |
| vacuum lints OpenAPI with Spectral-compatible rulesets; oasdiff reports breaking changes and can fail on a level. | verified (both run in this repo; probes: a removed example fails `make contract-lint`, a renamed path shows in `make contract-diff`) | `.vacuum.yaml`, `Makefile` |
| `@vitest/browser-playwright` pins the exact vitest version; with vitest 4.1.x it must be 4.1.x. | verified (npm metadata) | `web/package.json` |
| Vendoring skills beats auto-enabling a marketplace plugin: the repo controls the text and the skill matches the repo's rules. | judgement | this document |

## Decisions

1. **Branches.** One integration branch `development` (the GitHub default branch) plus `main` (production). Feature PRs squash into `development`; `development` reaches `main` only through a human-opened promotion PR (merge commit), enforced by the `promotion-source` CI check and two rulesets. Each developer cuts short-lived branches freely; no open-PR caps.
2. **Module loop.** Spec (`docs/modules/<module>.md`), then a contract PR (spec, `openapi.yaml`, generated code, 501 stubs), then BE and FE lanes in parallel. Contract gates: vacuum lint (blocking), oasdiff breaking-change report (non-blocking).
3. **TDD.** Test-first in one batch, a human red checkpoint, then implementation, then a mutation check. No per-test micro red/green inside the agent.
4. **FE tests.** Svelte component tests (vitest browser mode with Playwright Chromium) plus SSR tests with `svelte/server` for the no-JavaScript requirement (NFR-04).
5. **Skills.** Repo-authored `module-contract` and `module-tdd`, plus an adapted subset of obra/superpowers 7.0.1 (MIT), in `.agents/skills/` with `.claude/skills/` symlinks, checked by `make skills-check`. Rules live in `AGENTS.md` with nested `api/AGENTS.md` and `web/AGENTS.md`; permissions stay Claude-only and CI is the cross-tool enforcement.

## Rejected options

| Option | Reason |
|---|---|
| Per-test TDD inside the agent loop | Cost 3 to 8.5 times the tokens with no measured quality gain (Böckeler). A human checking why tests are red is the part that works. |
| Prism mock server | Not installed, adds a process. FE lanes test against fixtures typed from the contract `example`s. |
| Blocking oasdiff | Breaking changes are sometimes intended before go-live. The report goes to the PR summary and a human decides. |
| Auto-enabling the superpowers plugin | Its skills assume `main`, `.worktrees/`, local merges and `docs/superpowers/`, which contradict `AGENTS.md`. Vendored copies are adapted and reviewed. |
| OpenAPI 3.1 | PRD Appendix D fixes 3.0.3; tooling in the chain (oapi-codegen) is most solid on 3.0. |
| `uuidv7()` | PostgreSQL 18 only; CI tests 16 and 18 until OQ-04 decides the server version. |
| SvelteKit remote functions | Would bypass the contract-first OpenAPI chain. |
| testcontainers | The throwaway-database script (`scripts/db-throwaway.sh`) already isolates tests and needs no Docker-in-Docker. |
| Spec Kit, OpenSpec, Kiro | Each adds a CLI and its own artifact tree on top of the PRD (OpenSpec also collects telemetry by default). The module spec template covers PRD Appendix D. |
| Non-skill agent config in `.agents/` | `.agents/` is a skills location in practice; subagents, hooks and permissions have no shared format. |
| Committing lane plans | Plans are short-lived; the living truth is the module spec. Plans go in git-ignored `.plans/` and their task list is pasted into the PR body. |

## Trial later (researched, not requested)

Svelte MCP server and `llms.txt`, Squawk (migration linting), `sqlc vet`.

## Not verified

- Gemini CLI, Copilot and Cursor are not installed on our machines; `.gemini/settings.json` and their skills discovery are untested here.
- Per-tool discovery of `.agents/skills` and `.claude/skills` was smoke-tested only for the CLIs installed (see the PR description for the pass/fail table).
