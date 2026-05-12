# Roadmap

What's worth building next, in three buckets. Items move down the file as they ship — when something ships it leaves this file and lands in [`CHANGELOG.md`](CHANGELOG.md). Things that won't be done at all live in the "Won't do" section so the reasoning isn't lost.

Last reviewed: 2026-05-12 (restic drill + Pi lock TTL).

## Now

Nothing urgent.

## Next

Cheap, small, ready when there's an excuse to pull them in.

- **Persist reasoning across reloads.** Thinking tokens (SSE `reasoning` events) are rendered during a live turn but not saved — `/chat-v2` only persists the answer text. On page reload, thinking blocks disappear. Fix: store the reasoning content alongside the assistant message (separate DB column or a `messages.reasoning` field); populate the thinking block when loading message history in `static/sessions.js`.
- **Session topic: Pi-quality rename.** Auto-rename currently truncates the first user message (≤60 chars). The Pi agent's AGENTS.md still instructs it to call `claw-cli session topic` with a smarter title; the server-side truncation is the guaranteed fallback. Consider whether to drop the AGENTS.md instruction (simplify) or let Pi override (keep quality upside).
- **Observability & user metrics.** Add structured event logging (chat-turn started/ended, tool invoked, plan toggled, PDF opened, session created, latency, model + token usage). Storage: SQLite table with 90-day retention. Surface as a `/debug/metrics` view (auth-gated). Goal: make per-user behavior observable so the app can be tailored — which courses are hot, where users get stuck, which tools are rarely useful.
- **Courses-drawer UX review.** Current courses/sessions management feels poor. Brainstorm pass first — list what's clumsy, what's missing, what should disappear — before touching code. Likely splits into 2–3 small follow-up items.
- **Phase 2.6 — migration system.** Inline migrations in `agent/db.go` cover the current schema. The first time the schema needs a non-trivial change, replace them with a numbered-migration runner (something `golang-migrate`-shaped or a tiny in-tree version).
- **Cloudflare Access on top of bearer auth.** Optional second auth layer at the CF edge. Belt-and-suspenders — only worth it if you want zero unauthenticated traffic ever reaching the app.

## Later

Bigger things, not blocking, would need their own design pass.

- **Agent-emitted HTML snippets / interactive templates.** Let the LLM return inline UI fragments — flashcards, quizzes, diagrams, expandable comparison tables, code playgrounds — alongside prose. Needs a sanctioned subset of HTML (sanitized; restricted attributes; no scripts), a new content type in the SSE stream, and a renderer that can hydrate `data-action` hooks into the existing event delegator. Touches model trust boundary: prompt-inject defenses required.
- **Agent-generated files & downloads (especially PDFs).** Tool that lets the agent build a file (PDF summary, LaTeX export, JSON plan dump, flashcard deck) and surface it to the user as a download link in the chat. Server-side generation (likely `gofpdf` or `chromedp` headless render); files saved to a sandboxed dir under `data/agent-out/`; one-time signed URLs to avoid leaking. Decide auth/cleanup policy.
- **Memory system — brainstorm needed.** Today the agent has zero accumulated taste between sessions; every chat starts amnesiac. Decide the shape: per-user feedback memories (style/density/voice rules), conversation-summary memory (what was discussed last week), semantic memory of artifacts produced, project memory (what's in flight), or some union. Likely depends on the app↔Claude-runtime decision (replace vs complement). Brainstorm before designing storage, retrieval, or write-paths.
- **Knowledge base — brainstorm needed.** RAG corpus today is course markdowns the maintainer pre-converts. Real knowledge lives in formal `.tex` notes, fleeting `.md` notes, raw PDFs, and the agent's own past answers — none of which the corpus contains. Question is whether the KB should auto-ingest the user's actual work tree, what the indexing primitive is, how fleeting-vs-formal notes get weighted, and whether each user gets their own KB. Brainstorm before any indexer work.
- **Coding surface — clarify shape, then design.** User asked for "some system to add coding" — could mean (a) the agent can write and execute code in a sandbox (DSA practice, runnable examples, scratch experiments), (b) a coding-practice mode for study sessions (drills, problem flow), or (c) both. Resolve scope before designing.
- **Agent-administered test questions / active recall.** Tool that generates and grades test questions from the corpus + plan progress. Connects to `study-methods/active-recall.md` and `study-methods/spaced-repetition.md`. Needs: question generation (LLM call), answer comparison (semantic, not exact), score persistence, scheduling (which question to ask next, when). Likely couples with the HTML-snippet item above so questions render as proper UI, not free-text. Big enough to want its own design doc before any code.
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
