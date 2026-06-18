# Architecture Decision Records

Numbered, append-only. Once written, an ADR is not edited — to change a decision, write a new ADR that supersedes it and update the older one's status to `Superseded by NNNN`.

| # | Title | Status | Date |
|---|---|---|---|
| [0001](0001-stay-with-go.md) | Stay with Go; restructure instead of rewrite | Accepted | 2026-05-08 |
| [0002](0002-no-service-repository-layer.md) | No service/repository layer split | Accepted | 2026-05-08 |
| [0003](0003-no-docker-portability-first.md) | No Docker; ship a static binary under systemd | Accepted | 2026-05-10 |
| [0004](0004-vanilla-js-frontend.md) | Vanilla JS frontend, no Node build step | Accepted | 2026-05-10 |
| [0005](0005-push-to-main-no-prs.md) | Push to main directly, no PRs | Accepted | 2026-05-08 |
| [0006](0006-embed-pi-as-agent-runtime.md) | Embed Pi as the agent runtime; expose domain ops via Go `claw-cli` | Accepted | 2026-05-10 |
| [0007](0007-knowledge-component-as-atomic-note.md) | Knowledge Component as a content-bearing atomic note | Accepted | 2026-05-27 |
| [0008](0008-sidebar-course-first-launcher.md) | Session sidebar is a course-first launcher, not a navigator | Superseded by 0011 | 2026-05-29 |
| [0009](0009-session-single-task-spaced-unit.md) | Study session is a single-task spaced unit; tutor stops rather than chains | Accepted | 2026-05-29 |
| [0010](0010-steering-via-settings-ui.md) | Steering is edited via a settings UI to source-of-truth stores | Accepted | 2026-05-29 |
| [0011](0011-plan-is-navigation-spine.md) | The Plan is the navigation spine; a Session is a Task's workspace | Accepted | 2026-05-29 |
| [0012](0012-segmented-active-reading.md) | Segmented active reading with a position-aware tutor | Accepted | 2026-05-29 |
| [0014](0014-phase3-task-anchored-sessions-data-model.md) | Phase 3 data model: task-anchored Sessions, Scratch, clean-break migration | Accepted | 2026-05-30 |

> 0013 (`feat/wander`) is intentionally absent here — it lives on the Wander branch; reconcile this table on merge.

New ADR? Copy [`template.md`](template.md), pick the next number, link it from the table above.
