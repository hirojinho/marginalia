# Roadmap

What's worth building next, in three buckets. Items move down the file as they ship — when something ships it leaves this file and lands in [`CHANGELOG.md`](CHANGELOG.md). Things that won't be done at all live in the "Won't do" section so the reasoning isn't lost.

Last reviewed: 2026-05-10 (post-audit).

## Now

- **Restic restore drill.** Backups taken nightly but never restored. 15-min drill: pull the latest snapshot to a scratch dir on the VPS, diff against live `~/stack/study-app/`. Untested backups aren't backups. Operational task, not code.

## Next

Cheap, small, ready when there's an excuse to pull them in.

- **Make file tools VaultRoot-relative.** `toolReadFile` and `toolListFiles` currently take absolute paths verbatim (no `VaultPath()` join), while `toolSaveNote` joins `p.Path` with `VaultRoot`. Inconsistent footgun — and forces `CLAUDE.local.md` / `memory/study-context.md` to hardcode the live filesystem path (broken once on the Phase B move 2026-05-09; broken again any future move). Fix: make all four tools accept paths relative to vault root, update tool descriptions in `GetTools()`, rewrite the system-prompt docs to use relative paths (`memory/courses/…`, `data/plans/…`).
- **Inline reasoning stream.** Thinking tokens currently land in a single collapsible block at the top of the assistant message regardless of when the model emitted them. Render them in event order instead: each reasoning span shows up inline between the answer tokens that surround it (answer → thinking → answer …). Backend already emits `event: reasoning` and `event: token` separately; the work is in `static/chat.js`'s render loop, which routes all `reasoning` events into one container.
- **Surface tool calls in the chat UI.** When the LLM invokes a tool (`toolReadFile`, `toolRAGSearch`, `toolPDFExtract`, …), show what was called and a summary of the result. Today it's invisible to the user — answers reference things with no trail. Needs a new SSE event from `/chat` for tool start/end (plus a render path in `chat.js`). Decide whether to fold it into the same inline timeline as reasoning.
- **Pomodoro timer.** 25-min focus / 5-min break, ambient — no chat integration in v1, just a corner widget. Decide later whether it logs anything to plans.
- **Courses-drawer UX review.** Current courses/sessions management feels poor. Brainstorm pass first — list what's clumsy, what's missing, what should disappear — before touching code. Likely splits into 2–3 small follow-up items.
- **Phase 2.6 — migration system.** Inline migrations in `agent/db.go` cover the current schema. The first time the schema needs a non-trivial change, replace them with a numbered-migration runner (something `golang-migrate`-shaped or a tiny in-tree version).
- **Cloudflare Access on top of bearer auth.** Optional second auth layer at the CF edge. Belt-and-suspenders — only worth it if you want zero unauthenticated traffic ever reaching the app.

## Later

Bigger things, not blocking, would need their own design pass.

- **"Fast study" mode.** Undefined — placeholder for a low-friction path to short, lookup-style study moments (no orientation, no full session lifecycle?). Brainstorm before prioritising; capture the trigger first, then the shape.
- **Review the app ↔ Claw agent relationship.** Revisit how `claw-study` (this app) and Claw (the Telegram bot) divide responsibilities. Tier A `claw-study-read` skill is shipped; Tiers B/C/D from the original plan are speculative. Question is whether the Tier model is still right or whether the boundary should be redrawn. Output is likely an ADR (or one that supersedes Tier B/C/D below).
- **Tier B `claw-study-notes` skill.** Mutating skill for Claw — fleeting notes, plan toggles, memory edits.
- **Tier C `claw-study-api` skill.** HTTP API client for things the filesystem doesn't expose well (RAG search, chat).
- **Tier D `claw-study-deploy` skill.** Build / scp / systemctl / git ops on the repo.

## Won't do

Decisions deliberately ruled out. Listed so they're not relitigated by accident.

- **Service / repository layer split.** See [ADR 0002](docs/adr/0002-no-service-repository-layer.md).
- **Docker + Compose for the deploy.** See [ADR 0003](docs/adr/0003-no-docker-portability-first.md).
- **Stack rewrite (Go → anything else).** See [ADR 0001](docs/adr/0001-stay-with-go.md).
- **Frontend framework (HTMX / React / Svelte).** See [ADR 0004](docs/adr/0004-vanilla-js-frontend.md).
- **PR-based workflow.** See [ADR 0005](docs/adr/0005-push-to-main-no-prs.md).
