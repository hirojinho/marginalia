# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
marginalia does not use version tags (solo project, push-to-main).

## [Unreleased] — 2026-06-18

### Added
- **OSS distribution (Stage 1):** MIT license, README with screenshot and 5-minute run recipes (OpenAI + Ollama), bundled sample corpus, `.env.example`. Repo renamed from claw-study to marginalia.
- **Public/private repo split:** `marginalia` (public, MIT) and `marginalia-personal` (private, live instance). Open-core topology.

## May–June 2026

### Added
- **R9 — Practice testing (retrieval probes feed SM-2).** The agent generates questions from Knowledge Components, grades answers, and feeds results into the SM-2 spaced-repetition scheduler — replacing self-reported confidence with actual retrieval performance. Roediger & Karpicke (2006).
- **R6 — Plan-spine interleaving.** Spaced-retrieval revisit tasks can now be woven into a study plan, inserting a recall task after every N new-content tasks. Rohrer & Taylor (2007); Kornell & Bjork (2008).
- **R8 — Bloom-level enforcement at phase boundaries.** Plan phases can't complete on recall-only tasks — higher Bloom levels (analyze, evaluate, create) must be covered before a phase closes.
- **R2 — SM-2 expanding-interval spaced review.** Replaced crude fixed bands with a real SM-2 scheduler: ease factor, interval tracking, grade-driven resets below 3. Cepeda et al. (2008).
- **R1 — Retrieval-practice loop.** New `retrieval_queue` table; session-open recall of due Knowledge Components with scored grading. The highest-ROI pedagogy item.
- **R4 — Confidence trajectory persistence.** Rule-3 confidence values are now persisted to a `confidence_log` table, forming the substrate for the retrieval queue and learning-curve views.
- **KC capture box.** Frontend surface for the learner to author Knowledge Components (the generative capture act per ADR 0007) with a real UI, not just the agent tool.
- **Knowledge Component entity.** First-class `knowledge_components` table + DB methods + CLI subcommands. Learner-authored atomic notes below the task level.
- **Agent `knowledge_create` capture tool.** The tutor can capture a Knowledge Component mid-session with title, body, and source-task provenance.
- **PDF/slide access for the agent.** The Pi agent can now discover and extract PDF pages via `claw-cli pdf list` + `pdf current` — no more reconstructing content from training memory.
- **Plan-interleaving API.** `POST /api/plan/interleave` + `claw-cli plan interleave --course --cadence` for structural spaced retrieval.
- **8 pedagogical rules baked into both code paths.** Constructivist + cognitive-load scaffolds: no lecturing, prior-knowledge calibration, confidence checks, Bloom progression, session-open retrieval, pre-Read prediction, term budget (max 3 new terms/turn), session-close free recall, and elaborative interrogation.
- **AGENTS.md template overhaul.** Pi now sees course memory, session block, skill index, JSON-only plan tools, and pedagogy rules in order.
- **Pi agent runtime.** Each chat session runs through `@earendil-works/pi-coding-agent` (Pi), a subprocess-based agent with bash tool surface. SSE events (`token`, `reasoning`, `tool_start`, `tool_end`, `done`, `error`) stream directly to the browser.
- **Phase 3 — task-anchored sessions.** Sessions are 1:1 with plan tasks; the Plan is the navigation spine. Detached sessions (orphaned by plan restructures) are survivable.
- **Scratch space.** Ad-hoc chats not tied to any task — one-off questions, paper checks, exploratory threads.
- **Authoring surface.** Generative course/plan creation with the agent: conversational course building and plan editing.
- **Steering UI.** Declarative settings form for course config, plan toggles, and preferences — owned by the learner directly.
- **Session topic auto-rename.** First-turn messages derive a session title automatically; inline rename via the sidebar.
- **Inline reasoning stream.** Thinking tokens render inline between answer segments as collapsible `<details>` blocks.
- **Tool calls visible in chat.** `tool_start`/`tool_end` events render as collapsible panels showing tool name, input, output, and status.
- **Pomodoro timer.** 25-minute countdown in the header with tabular-nums for no-width-jitter.
- **Sidebar: course-first launcher.** Courses are the top-level navigation; sessions are reached through their plan tasks.
- **JSON-only plan enforcement.** `data/plans/<course>.json` is the single source of truth; markdown plans retired.
- **SSE keepalive.** 15-second comment frames prevent Cloudflare tunnel reaping during slow LLM waits.
- **Graceful shutdown, timeouts, input validation.** HTTP server with ReadTimeout/WriteTimeout/IdleTimeout; SIGTERM handler with 15s drain; 50MB PDF cap; 4000-char message cap.
- **Structured error responses.** JSON error objects on all failure paths; `context.Context` cancellation through to the LLM.
- **Frontend modularization.** 2,160-line `index.html` split into ~13 vanilla JS modules with retry-with-backoff and error banner.
- **golangci-lint + pre-commit hook.** `errcheck`, `govet`, `staticcheck`, `gocyclo`, `gochecknoglobals`, `forbidigo` enforced on staged files.

### Changed
- Per-turn timeout 5 min → 10 min to accommodate course-planning turns.
- `streamPiTurn` is ctx-aware — returns immediately on client disconnect.
- Pedagogy block in agent templates lost legacy dual-plan references in favor of JSON-only.

### Fixed
- AGENTS.md was always falling back to placeholder when `CLAW_CLI_PATH` was unset.
- Pi was reading the wrong plan (two plan stores had silently diverged).

## May 2026

### Added
- **Pi lock TTL.** Stale locks (older than turn timeout + 30s) are treated as expired and replaced via CAS loop.
- **Session topic inline rename.** Pencil icon → inline input in the sidebar.
- **Per-session Pi sandbox.** Isolated working directory with generated AGENTS.md, notes scratch dir, and agent-out symlink. Swept after 7 days.
- **Course CRUD.** Courses are first-class entities with settings (`mastery_threshold`, `interleaving` toggle).
- **Plan creation and editing.** `claw-cli plan create` / `plan status` / `plan toggle` for agent-driven plan management.
- **Stable task UUIDs.** Every plan task carries an immutable UUIDv4, enabling reliable foreign-key tracking across sessions.
- **RAG (retrieval-augmented generation).** Corpus indexing on startup (`data/corpus/*.md` → vector store); semantic search via `/api/corpus/search`.
- **PDF viewer with SyncTeX.** Embedded PDF.js viewer; page tracking; LaTeX SyncTeX integration via vimtex.
- **Restic backups.** Nightly snapshots of the vault directory.
- **`marginalia-read` skill (Tier A).** Read-only Claw skill for querying plan progress, sessions, PDF state, and corpus from Telegram.
