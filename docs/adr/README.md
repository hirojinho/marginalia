# Architecture Decision Records

Numbered, append-only. Once written, an ADR is not edited — to change a decision, write a new ADR that supersedes it and update the older one's status to `Superseded by NNNN`.

| # | Title | Status | Date |
|---|---|---|---|
| [0001](0001-stay-with-go.md) | Stay with Go; restructure instead of rewrite | Accepted | 2026-05-08 |
| [0002](0002-no-service-repository-layer.md) | No service/repository layer split | Accepted | 2026-05-08 |
| [0003](0003-no-docker-portability-first.md) | No Docker; ship a static binary under systemd | Accepted | 2026-05-10 |
| [0004](0004-vanilla-js-frontend.md) | Vanilla JS frontend, no Node build step | Accepted | 2026-05-10 |
| [0005](0005-push-to-main-no-prs.md) | Push to main directly, no PRs | Accepted | 2026-05-08 |

New ADR? Copy [`template.md`](template.md), pick the next number, link it from the table above.
