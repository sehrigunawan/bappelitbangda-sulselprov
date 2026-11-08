# Documentation index

Generated schema documentation lives in [`schema/`](schema/) (made by `make db-docs`, never hand-edited). Add a line here when you add a doc, and delete the doc when nobody needs it.

| Document | Path | Purpose |
|---|---|---|
| PRD v1.1 (English copy, unsigned) | [`prd/PRD.md`](prd/PRD.md) | Requirements baseline. Humans edit the Google Doc, never this file. |
| PRD source, hash, update procedure | [`prd/README.md`](prd/README.md) | How the copy is kept in sync with the master. |
| Requirement traceability (generated) | [`prd/traceability.md`](prd/traceability.md) | Each FR/NFR id with phase and priority. |
| Team and agent workflow | [`workflow/team-and-agents.md`](workflow/team-and-agents.md) | Roles, module loop, agent harnesses, branches and promotion, security lane. |
| Module roadmap | [`workflow/roadmap.md`](workflow/roadmap.md) | Module dependency graph, build waves and open-question gates. |
| Module specs | [`modules/README.md`](modules/README.md) | One living spec per module, from `_template.md`; status index. |
| Agent workflow decisions | [`decisions/agent-workflow-2026-11.md`](decisions/agent-workflow-2026-11.md) | Research synthesis, decisions taken and rejected options. |
| PRD v1.1 reconciliation | [`decisions/prd-v1.1-reconciliation.md`](decisions/prd-v1.1-reconciliation.md) | Open mismatches and decisions that need a human. |
| Runbook | [`handover/runbook.md`](handover/runbook.md) | Release, deploy, rollback, backup and restore. |
| Owners | [`handover/OWNERS.md`](handover/OWNERS.md) | Who holds what, by role. |
| Deploy log | [`handover/deploy-log.md`](handover/deploy-log.md) | One line per deploy. |
| Known gaps | [`handover/known-gaps.md`](handover/known-gaps.md) | What is not done, with the blocking OQ. |
