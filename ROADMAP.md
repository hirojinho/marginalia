# Roadmap

What's worth building next, in three buckets. Items move down the file as they ship — when something ships it leaves this file and lands in [`CHANGELOG.md`](CHANGELOG.md). Things that won't be done at all live in the "Won't do" section so the reasoning isn't lost.

Last reviewed: 2026-05-10.

## Now

Nothing in flight. The May 2026 stability plan closed on 2026-05-10.

## Next

Cheap, small, ready when there's an excuse to pull them in.

- **Phase 2.6 — migration system.** Inline migrations in `agent/db.go` cover the current schema. The first time the schema needs a non-trivial change, replace them with a numbered-migration runner (something `golang-migrate`-shaped or a tiny in-tree version).
- **Cloudflare Access on top of bearer auth.** Optional second auth layer at the CF edge. Belt-and-suspenders — only worth it if you want zero unauthenticated traffic ever reaching the app.

## Later

Bigger things, not blocking, would need their own design pass.

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
