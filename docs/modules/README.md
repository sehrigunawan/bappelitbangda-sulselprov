# Module specs

One living spec per module, from [`_template.md`](_template.md) (PRD Appendix D asks for 1 to 2 pages before a module is built). A module's contract PR adds its spec and flips its status below. Build order, dependencies and open-question gates: [`../workflow/roadmap.md`](../workflow/roadmap.md). The `module-contract` and `module-tdd` skills (`.agents/skills/`) describe the loop.

Status: `not started` | `contract` | `in progress` | `done`. Requirements marked P2 or P3 extend the module's spec in a later contract PR.

| Module | Wave | Spec | Requirements | Security lane | Status |
|---|---|---|---|---|---|
| `auth` | 1 | [auth.md](auth.md) | FR-33, NFR-05, NFR-17 | yes | done |
| `media` | 1 | (not written) | FR-38, NFR-06 | no | not started |
| `site` | 2 | (not written) | FR-02, FR-03, FR-35, FR-37, FR-44; P3: FR-32 | no | not started |
| `applications` | 2 | (not written) | FR-28, FR-37 | no | not started |
| `staff` | 2 | (not written) | FR-07, FR-37; P2: FR-08 | no | not started |
| `pages` | 3 | (not written) | FR-05, FR-35; P2: FR-06, FR-09 | no | not started |
| `publications` | 3 | (not written) | FR-10, FR-11, FR-13, FR-34; P2: FR-12 | no | not started |
| `documents` | 3 | (not written) | FR-15, FR-19, FR-36; P2: FR-16, FR-17, FR-18 | no | not started |
| `contact` | 4 | (not written) | FR-29, NFR-07 | no | not started |
| `surveys` | 4 | (not written) | FR-25, FR-26, FR-27, FR-37 | no | not started |
| `home` | 4 | (not written) | FR-01, FR-37; P2: FR-45; P3: FR-30, FR-31 | no | not started |
| `ppid` | 5 | (not written) | FR-20, FR-21, FR-22, FR-24, NFR-08; P2: FR-23 | yes | not started |
| `seo` | 6 | (not written) | FR-41, FR-42 | no | not started |
| `search` | 7 | (not written) | FR-04, FR-43 | no | not started |
| `audit` | 7 | (not written) | FR-39 | no | not started |
| `gallery` | 8 | (not written) | FR-14 | no | not started |

FR-40 (leadership summary) is deferred (`Won't`) and has no module.
