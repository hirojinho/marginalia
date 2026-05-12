# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Dates are commit dates on `main` (no version tags — solo project, push-to-main per [ADR 0005](docs/adr/0005-push-to-main-no-prs.md)).

## [Unreleased]

Nothing pending.

## [2026-05-12]

### Added
- **Pi lock TTL.** `AcquirePiLock` now stores a `time.Time` acquisition timestamp instead of `struct{}`. Locks older than `piTurnTimeout + 30s` (5m30s) are treated as stale and replaced via a CAS loop. Prevents sessions from staying stuck at 409 after a Pi crash or mid-turn context cancellation.

### Ops
- **Restic restore drill.** Restored snapshot `56e2e5dd` (2026-05-12 03:00 UTC) to a scratch dir and diffed against live `~/stack/study-app/`. Diff was clean: new agent sessions, one new PDF, in-flight WAL files, and a skill update from this session. Backup chain verified.

## [2026-05-11]

### Added
- **Pi agent runtime.** Each chat session now runs through `@earendil-works/pi-coding-agent` (Pi), a subprocess-based agent with a bash tool surface. `/chat-v2` endpoint replaces `/chat` for sessions in Pi mode. Pi is spawned per-turn via `agent.RunPi`; SSE events (`token`, `reasoning`, `tool_start`, `tool_end`, `skill_start`, `compaction`, `model_change`, `done`, `error`) stream directly to the browser.
- **Per-session Pi sandbox.** `SandboxManager` creates `data/agent-sessions/<id>/` with a generated `AGENTS.md` (via `claw-cli memory load --session --course`), a `notes/` scratch dir, and a symlink to `data/agent-out/`. Sandboxes are swept after 7 days of inactivity.
- **`claw-cli session topic` subcommand.** Sets a descriptive topic on any session by ID. Accepts `--session-id` and `--topic` flags; reads/writes study.db via `App.UpdateSessionTopic`.
- **Session topic auto-rename on first turn.** `handleChatV2` detects the first message in any "General" session and immediately derives a title by truncating the user message (≤60 runes, word-boundary). Sends a `session_topic` SSE event so the sidebar and header pill update without a page reload. Pi can still override the topic during its run.
- **Inline reasoning stream.** Thinking tokens now render inline between the answer segments that surround them rather than in a single pre-allocated block at the top. Each reasoning span is a `<details class="thinking-inline">` collapsed by default; answer tokens go into `<div class="answer-segment">` divs. Frontend tracks `currentSegmentType` across both event types.
- **Tool calls visible in chat UI.** `tool_start` and `tool_end` SSE events render as collapsible tool panels inside the answer timeline, showing the tool name, input summary, output summary, and success/failure indicator.
- **Session topic inline rename.** Pencil icon next to each session name in the sidebar; click converts the label to an `<input>`, `Enter` or blur commits via `PATCH /api/sessions?id=`. Topic is updated in-memory and the header pill refreshes.
- **Pomodoro timer in header.** `#pomodoro-btn` in the header actions area shows a 25:00 countdown. Click starts/pauses, double-click resets. Switches to 5-min break after each focus block. `font-variant-numeric: tabular-nums` prevents width jitter.

### Changed
- `Cache-Control: no-store` set on all JS, CSS, and HTML static asset responses to prevent Cloudflare edge caching stale embedded files.
- File tools (`read_file`, `list_files`) now accept vault-relative paths (joined to `VaultRoot`) in addition to absolute paths. Tool descriptions updated to document relative-path support.

### Fixed
- DDIA `agent_memory` project rows on VPS had stale laptop-only paths; updated to use `claw-cli plan show/toggle --course ddia` and correct vault-relative interests path.
- `TestHandleSessionsMethodNotAllowed` used `PATCH` which is now a valid method; updated to use `PUT`.

## [2026-05-10]

### Added
- Pre-push dev loop now includes `staticcheck ./...` alongside `go vet` and `go test`. Install once with `go install honnef.co/go/tools/cmd/staticcheck@latest`. README documents the checklist.
- Test coverage on `agent/` raised from 17.6% to 47.4%, focused on the `tools_*.go` surface (`toolUpdatePlan`, `toolReadFile`/`toolListFiles`/`toolSaveNote`, `ExecuteTool` dispatch, `toolStudySkill` skill branches, `toolRAGSearch` argument paths, `reserveWebFetchSlot` rate-limit, `LoadSystemPrompt`, `ChunkFile`). 9 new test files in `agent/`. No production-code edits.
- Documentation reorganization: root specs moved into `docs/specs/` (active) and `docs/specs/archive/` (historical phase plans). New `docs/adr/` with five ADRs (0001 stay-with-Go, 0002 no-service-repo-split, 0003 no-Docker, 0004 vanilla-JS, 0005 push-to-main). Top-level `CHANGELOG.md` and `ROADMAP.md` added; root `README.md` points at the new tree.
- Global `window.onerror` + `unhandledrejection` handlers with a fixed-position recovery banner.
- `apiFetch` wrapper: up to 3 attempts on network errors and 5xx for idempotent GETs, exponential backoff with jitter (200/400/800ms +0–100ms). Non-GET methods pass through with one attempt to avoid duplicating writes.
- Input validation on chat (≤4000 chars), session topic (≤200 chars), PDF (type + ≤50 MiB), surfaced via the error banner.
- Loading state on session create button.

### Changed
- `agent/tools.go` split from one 970-line god-file into per-concern files: `tools.go` (manifest + dispatch), `tools_file.go`, `tools_rag.go`, `tools_plan.go`, `tools_pdf.go`, `tools_web.go`, `tools_skill.go`. No behavior change.
- `static/app.js` split from one 1,318-line file into native ES modules: `app.js` (entry + `data-action` dispatcher), `apiFetch.js`, `errorBanner.js`, `dom.js`, `marked.js` (shim), `chat.js`, `sessions.js`, `plan.js`, `pdf.js`. Loaded as `<script type="module">`; no bundler. No behavior change.
- `static/index.html` split into `static/style.css` (828 lines) and `static/app.js` (1,203 lines, `defer`-loaded). `index.html` shrank from 2,159 to 127 lines.
- `marked.min.js` v15.0.12 bundled locally — last CDN dependency removed.
- Inline `onclick` replaced with `data-action` event delegation across `index.html` and templates.
- `agent/db.go` `getMetaInt` now uses `strconv.ParseInt` instead of `fmt.Sscanf`.

### Fixed
- `pdfPanel` scope bug in the `pdf-btn` click handler.
- `Content-Type` header set correctly for `.css` and `.js` static assets.

## [2026-05-09]

### Added
- HTTP auth middleware (`handler/auth.go`): bearer header or HttpOnly cookie, `subtle.ConstantTimeCompare`, SameSite=Strict. `AUTH_TOKEN` env gates everything except `/login`. Empty token = warn-and-allow for local dev. 11 unit tests in `handler/auth_test.go`.
- Top-level `README.md`.

## [2026-05-08]

### Added
- `agent.App` struct holding DB connection, config, chat mutex, atomic active session id. Eliminates package-level globals.
- `handler/` package — HTTP handlers split by domain (sessions, plan, pdf, static).
- `slog`-based structured logging across the codebase.
- Unit tests for pure functions in `agent/` and `handler/` (chunker, `parsePageSelection`, embedding serde, `pickPages`, `CosineSimilarity`, `embedText`, `keywordSearch`, `NeedsReindex`, db lifecycle, sessions, plan HTTP, debug health, `toggleTaskAt`).
- `claw-study-read` skill spec + SKILL.md (Tier A of claw-task-execute decomposition).
- SQLite pragmas: `journal_mode=WAL`, `busy_timeout=5000`, `foreign_keys=ON`, `synchronous=NORMAL`, `cache_size=-2000`.
- HTTP timeouts: Read 30s, Write 5min (covers streaming chat), Idle 2min. Graceful shutdown via SIGINT/SIGTERM with 15s grace.
- `http.MaxBytesReader` for PDF uploads (50 MiB cap). Chat message cap (4000 bytes).

### Changed
- Go bumped to 1.24.1 (required by `github.com/ledongthuc/pdf`).
- `VaultPath` fix: `VAULT_ROOT` is the study-app directory itself, not its parent. `/workspace/study-app/...` hardcoded paths replaced with `VaultPath(...)` calls.
- Hardened silent error paths in `agent/*` and `handler/pdf.go`: `DB.Exec` errors checked, `defer rows.Close()` added, `os.WriteFile`/`MkdirAll` errors checked, `context.Context` threaded through all LLM calls.
- Personal notes (`memory/courses/*`, `memory/thesis/*`, `memory/courses-agent` symlink) added to `.gitignore` — sensitive, not source.

### Initial commit (`edbb0de`)
Imported the prior in-tree study app as the seed of this repo: HTTP server with embedded SPA, SQLite-backed sessions/messages/plans, RAG over a local corpus, PDF viewer, function-calling LLM tools.
