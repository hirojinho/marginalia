# Roadmap

What's worth building next, in three buckets. Items move down the file as they ship — when something ships it leaves this file and lands in [`CHANGELOG.md`](CHANGELOG.md). Things that won't be done at all live in the "Won't do" section so the reasoning isn't lost.

Last reviewed: 2026-05-19 (chat-v2 reliability bundle + pedagogy rules + Pi context overhaul).

## Now

Nothing urgent.

## Next

Cheap, small, ready when there's an excuse to pull them in.

- **Persist reasoning across reloads.** Thinking tokens (SSE `reasoning` events) are rendered during a live turn but not saved — `/chat-v2` only persists the answer text. On page reload, thinking blocks disappear. Fix: store the reasoning content alongside the assistant message (separate DB column or a `messages.reasoning` field); populate the thinking block when loading message history in `static/sessions.js`.
- **Session topic: Pi-quality rename.** Auto-rename currently truncates the first user message (≤60 chars). The Pi agent's AGENTS.md still instructs it to call `claw-cli session topic` with a smarter title; the server-side truncation is the guaranteed fallback. Consider whether to drop the AGENTS.md instruction (simplify) or let Pi override (keep quality upside).
- **Courses-drawer UX review.** Current courses/sessions management feels poor. Brainstorm pass first — list what's clumsy, what's missing, what should disappear — before touching code. Likely splits into 2–3 small follow-up items.
- **Phase 2.6 — migration system.** Inline migrations in `agent/db.go` cover the current schema. The first time the schema needs a non-trivial change, replace them with a numbered-migration runner (something `golang-migrate`-shaped or a tiny in-tree version).
- **Cloudflare Access on top of bearer auth.** Optional second auth layer at the CF edge. Belt-and-suspenders — only worth it if you want zero unauthenticated traffic ever reaching the app.

### Pedagogy backlog (graded by evidence strength)

Today's session baked the front-half of a learning loop (encoding scaffolds — see pedagogy rules in `CLAUDE.local.md` and `agent/sandbox.go`'s AGENTS.md template). The back half — retrieval, spacing, mastery — is missing. Items below are ranked by evidence weight; **R1 + R2 carry roughly half the total evidence weight of all retrieval-side interventions** (Roediger & Karpicke 2006; Dunlosky et al. 2013).

- **R1 — Retrieval-practice loop.** Highest single ROI. New `retrieval_queue` table (`concept_id`, `due_at`, `last_confidence`); `claw-cli retrieve due` subcommand surfaces what's due; AGENTS.md hook makes Pi open every session with a retrieval round on the user's recent material. Currently approximated by Rule 6 (prompt-only); the full version persists, schedules, and re-surfaces. Spec ~3 tables + 1 subcommand + 1 skill change. Evidence: Roediger & Karpicke (2006), Karpicke & Blunt (2011, *Science*), Dunlosky et al. (2013) — practice testing rated **high utility** in the 10-technique meta-review.
- **R2 — Expanding-interval spaced review (SM-2).** Extends R1: after each retrieval, schedule next review on an SM-2-style schedule (confidence high → interval ×2.5; low → reset to 1 d). Cepeda et al. (2008, *Psychological Science*). Small marginal cost once R1 lands.
- **R3 — Mastery gates.** Uses R1's retrieval results as the gating signal. A cluster is "done" only when retrieval confidence ≥0.7 across ≥3 probes spread over ≥1 week; otherwise the cluster's tasks re-queue. Bloom (1968, mastery learning). Couples cleanly with R1.
- **R4 — Confidence trajectory persistence.** Save the Rule-3 confidence values (`confidence_log(timestamp, concept, value)`) so we can prioritize the retrieval queue and visualize learning curves over time. Lightweight (single table). Dashboard view is a separate item under Later.
- **R5 — Worked-example → completion pairs.** For technical content with computation (ETA probability, FTA cut-sets, risk matrices), define a new task type: `worked-example` (study) → `completion` (fill blanked steps). Pi grades against a rubric prompt. Sweller & Cooper (1985); Renkl (2014, *example-based learning*). Bigger lift — needs a new task kind, completion-grading rubric, and frontend rendering.
- **R6 — Interleaving by default in plan structure.** `course-study-path` skill rule: after every 3–4 new-content tasks, insert a `revisit` task that surfaces an earlier phase's content via retrieval (couples with R1). Rohrer & Taylor (2007); Carvalho & Goldstone (2014). Skill-level change once R1 ships.
- **R7 — Interest-log activation (full version).** Today shipped a lightweight prompt-only surfacing rule (Rule 6 / Q6). Full version: time-based queue (`interests` table with `surfaced_at`, `closed`, `pursued`), weekly resurfacing of oldest entries with explicit close-or-pursue action, decay if ignored. Berlyne (1960, curiosity as the engine of self-directed learning).
- **R8 — Bloom-level enforcement at phase boundaries.** Q3 shipped per-task Bloom tags. Full version: a phase cannot be marked complete unless every Bloom level (Apply / Analyze / Evaluate / Create) has been touched at least once. Compile-time check in the plan builder. Anderson & Krathwohl (2001).

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
- **R-Later — Pedagogy dashboard.** Once R4 (confidence trajectories) ships, surface a per-concept learning-curve view in the UI: confidence vs. time, retrieval success rate, items overdue for review. Backs the Bjork "desirable difficulties" framing (Bjork & Bjork 2011) — makes the fluency illusion visible.
- **R-Later — Term-budget self-enforcement.** Rule 8 is currently advisory (Pi may still violate it). A stronger version has Pi self-count new terms per turn and refuse to advance until the user demonstrates the previous batch. Sweller (1988) but instrumented. Lower ROI than R1–R4; nice-to-have polish.
- **R-Later — Deliberate-practice tracks.** When a concept's retrieval confidence stays low across multiple probes, branch the user into an isolated practice loop with targeted exercises and immediate feedback. Ericsson (1993). Depends on R1+R4 being in place.

## Won't do

Decisions deliberately ruled out. Listed so they're not relitigated by accident.

- **Service / repository layer split.** See [ADR 0002](docs/adr/0002-no-service-repository-layer.md).
- **Docker + Compose for the deploy.** See [ADR 0003](docs/adr/0003-no-docker-portability-first.md).
- **Stack rewrite (Go → anything else).** See [ADR 0001](docs/adr/0001-stay-with-go.md).
- **Frontend framework (HTMX / React / Svelte).** See [ADR 0004](docs/adr/0004-vanilla-js-frontend.md).
- **PR-based workflow.** See [ADR 0005](docs/adr/0005-push-to-main-no-prs.md).
