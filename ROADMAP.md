# Roadmap

What's worth building next, in three buckets. Items move down the file as they ship — when something ships it leaves this file and lands in [`CHANGELOG.md`](CHANGELOG.md). Things that won't be done at all live in the "Won't do" section so the reasoning isn't lost.

Last reviewed: 2026-05-10 (post-audit).

## Now

Latent code-quality items surfaced 2026-05-10 audit (post stability-plan close):

- **Split `agent/tools.go`.** 970 LOC, 11 tool methods on `App` plus dispatch + helpers + system-prompt generation in one file. New tool = hunt through god-file. Mechanical refactor into `agent/tools/` (one file per tool) or at least split by concern. No behavior change.
- **Split `static/app.js` into ES modules.** 1,318 LOC single file — bigger than the original `index.html` was before Phase 4. Modern browsers do native `import`; no bundler needed. Natural splits: `apiFetch.js`, `errorBanner.js`, `chat.js`, `sessions.js`, `plan.js`, `pdf.js`.
- **Raise `agent/` test coverage on `tools.go`.** Real coverage as of 2026-05-10: 17.6% agent, 36.8% handler, 0% root, 0% convert. Tools — JSON parsing of LLM args, file I/O, dispatch — are the highest-failure-rate code path and barely covered. Target ≥50% on `agent/`.
- **Install `staticcheck` in the dev workflow.** Stability plan 3.6 prescribed `go vet` + `staticcheck` + `golangci-lint`; only `go vet` runs today.
- **Restic restore drill.** Backups taken nightly but never restored. 15-min drill: pull last snapshot to a scratch dir, diff against live `~/stack/study-app/`. Untested backups aren't backups.

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
