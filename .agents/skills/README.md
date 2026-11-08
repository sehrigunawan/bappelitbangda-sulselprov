# Agent skills

Canonical location of this repository's agent skills (Agent Skills format: `<name>/SKILL.md`, see [agentskills.io/specification](https://agentskills.io/specification)). Codex, Gemini CLI, Copilot, Cursor, OpenCode and Antigravity scan this directory. Claude Code scans only `.claude/skills/`, where each skill is a relative symlink `.claude/skills/<name> -> ../../.agents/skills/<name>`. `make skills-check` (part of `make verify`) enforces the layout, the name, the description length (1024) and the `SKILL.md` size (500 lines).

`AGENTS.md` outranks every skill. This directory holds only skills; other agent configuration is not shared across tools (`docs/workflow/team-and-agents.md`, Agent harnesses). Skill-private scripts live in `<name>/scripts/`; anything a human or CI also runs belongs in `scripts/` behind a `make` target.

Adding a skill: create `.agents/skills/<name>/SKILL.md`, then `ln -s ../../.agents/skills/<name> .claude/skills/<name>`, then `make skills-check`. Follow `writing-skills` (baseline without the skill first, then with it).

Skills copied from obra/superpowers 7.0.1 are MIT licensed, see [`LICENSE-superpowers`](LICENSE-superpowers). They are adapted copies: they follow this repository's rules (`make` targets, `development` branch, `../wt/<issue-id>` worktrees), not upstream's. Skipped: `using-superpowers` (replaced by the pointer in `AGENTS.md`) and `diagnosing-superpowers` (files issues upstream over the network). If a globally installed superpowers plugin is also active, disable it while working here.

| Skill | Origin | Adaptation |
|---|---|---|
| `module-contract` | repo | Spec, contract conventions, 501 stubs, gates, contract PR. |
| `module-tdd` | repo | BE and FE lanes: tests in one batch, human red checkpoint, implementation. |
| `brainstorming` | superpowers 7.0.1 | Visual companion removed. Design doc is `docs/modules/<module>.md`. Security-lane topics: the human drafts. |
| `writing-plans` | superpowers 7.0.1 | Plan is `.plans/<issue-id>-<lane>.md` (git-ignored, short-lived); task list pasted in the PR body. |
| `executing-plans` | superpowers 7.0.1 | `make` targets. Stops at the red checkpoint, security-lane tasks and contract defects. |
| `subagent-driven-development` | superpowers 7.0.1 | `make` targets; same three stop points; base branch `origin/development`. |
| `test-driven-development` | superpowers 7.0.1 | Iron Law kept. Batch protocol: tests in one batch, empty stubs, focused run, human red checkpoint, implement, mutation check. |
| `systematic-debugging` | superpowers 7.0.1 | Pressure tests, `find-polluter.sh` and creation log dropped; `make` targets. |
| `verification-before-completion` | superpowers 7.0.1 | The command is `make verify` (`make verify-full` for SQL or migrations); paste the tail. |
| `requesting-code-review` | superpowers 7.0.1 | Reviewer subagent runs before the PR opens, checks the diff against the module spec and `AGENTS.md`, never replaces the human. |
| `receiving-code-review` | superpowers 7.0.1 | Security-lane feedback: the human implements or reviews every line. |
| `using-git-worktrees` | superpowers 7.0.1 | Worktrees at `../wt/<issue-id>` from `origin/development`; one dev stack at a time. |
| `finishing-a-development-branch` | superpowers 7.0.1 | Push and open a PR into `development`, keep, or discard. Never merges locally. |
| `dispatching-parallel-agents` | superpowers 7.0.1 | Parallel agents in one branch only on disjoint files; cross-PR parallelism uses separate worktrees. |
| `writing-skills` | superpowers 7.0.1 | Skill location is `.agents/skills/`. |
