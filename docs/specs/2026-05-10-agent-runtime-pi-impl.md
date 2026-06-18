# Implementation Plan — Agent Runtime (Pi)

> **Status:** Plan (2026-05-10). References: [spec](./agent-runtime-pi.md), [ADR 0006](../adr/0006-embed-pi-as-agent-runtime.md). When all phases land and the legacy `/chat` is removed, move this file to `docs/specs/archive/`.

## Phase ordering

Each phase ships a verifiable artifact and de-risks the next. Phases are sized to be reviewable in one sitting (about half a day each). Order minimizes blocking — earliest phases produce things later phases need, but each phase's work is self-contained.

## Phase 0 — Pi installed, smoke-tested with OpenCode

**Scope.** Install `@earendil-works/pi-coding-agent` on the VPS. Configure OpenCode Go authentication. Run `pi --mode rpc` from a shell, send a `prompt` command via JSONL on stdin, capture the event stream from stdout. Confirm `deepseek-v4-pro` works for tool-call. Probe `kimi-k2.6` and `glm-5.1`. Measure cold-start latency for each.

**Deliverables.** A short notes file (`docs/specs/proposals/pi-rpc-handshake-notes.md` or appended to this plan) recording: install path, model latencies, sample event stream, observed quirks per model.

**Verification.** Three model probes return well-formed events and produce a `tool_call` for a simple "list files in /tmp" prompt. Latencies recorded.

## Phase 1 — `claw-cli` skeleton + memory subcommands

**Scope.** New Go binary at `cmd/claw-cli/`. Subcommands: `memory load`, `memory save`, `memory search`. Schema migration adding the `agent_memory` table. Loader assembles `AGENTS.md` content from user profile + course context + active feedback memories + recent activity + skill list (skill list parsed from frontmatter of each file under `skills/`).

**Deliverables.** `claw-cli` binary built and deployed. Migration applied. Seed script that imports the relevant memories from `~/.claude/projects/<project-slug>/` into `agent_memory` (user profile, feedback files, course-scoped memories, interests).

**Verification.** `claw-cli memory load --session <fake> --course ce297 --user eduardo` produces a non-trivial, well-formed `AGENTS.md` of at most 3 KB. New unit tests on the loader cover: empty memory, course-only filtering, recency cutoff, skill-list assembly.

## Phase 2 — `claw-cli` domain subcommands

**Scope.** Add the remaining subcommands: `rag search`, `plan show`, `plan toggle`, `course interests`, `note save`, `pdf extract`, `web fetch`, `skill dispatch`. Each is a thin Go wrapper over the existing function in `agent/tools_*.go`.

**Deliverables.** Full `claw-cli` surface as documented in the spec. Each subcommand has a `--help` and emits valid JSON on success, structured errors on failure.

**Verification.** End-to-end from a shell: `claw-cli rag search "STAMP" --course ce297 --top-k 3` returns valid JSON; `claw-cli plan show --course ddia` shows the plan. Unit tests cover argument parsing, JSON output shape, error paths.

## Phase 3 — Skill ports

**Scope.** Port the six study-core skills from `~/.claude/skills/` into `skills/` at the repo root. Per skill: rewrite Claude-Code-specific tool references to Pi-equivalents (`/skill:name`, `claw-cli`, `bash`), strip protocol affordances that don't translate, validate via `pi --skills-dir=./skills` that the skill loads and lists.

**Deliverables.** Six SKILL.md files under `skills/`, checked in. A short porting-notes appendix capturing what changed per skill (and why) — useful when porting career skills later.

**Verification.** For each skill: `pi --skills-dir=./skills /skills` lists it; `/skill:study-notes` (etc.) invokes it without runtime errors and produces a coherent first response.

## Phase 4 — Per-session sandbox

**Scope.** Add a `agent.SandboxManager` to `claw-study`. On the first `/chat-v2` turn for a session, create `data/agent-sessions/<id>/`, generate `AGENTS.md` via `claw-cli memory load`, set up the `out` symlink, create the `notes/` subdir. Reuse on subsequent turns. Delete on session deletion. Add a sweep job for stale sandboxes (older than N idle days).

**Deliverables.** Sandbox manager Go code with unit tests. Wired into a stub `/chat-v2` handler that creates the sandbox and returns the path (no Pi yet — this phase ends before spawning).

**Verification.** `curl -X POST /chat-v2 -d '{"session_id":1,"message":"hi"}'` creates the sandbox; second call reuses it; deleting the session removes the dir. Sweep job marks and deletes a sandbox aged past the threshold in tests.

## Phase 5 — `/chat-v2` proxy + SSE translation

**Scope.** `/chat-v2` spawns `pi --mode rpc`, sends the user message as a JSONL `prompt` command on stdin, streams Pi's stdout events back to the browser as SSE in the new vocabulary. Implement the event translator (`token`, `reasoning`, `tool_start`, `tool_end`, `skill_start`, `compaction`, `model_change`, `done`, `error`). Implement Pi process lifecycle: spawn, 60 s timeout, deferred kill + waitpid on handler exit.

**Deliverables.** Handler at `handler/chat_v2.go`. SSE translator with unit tests. Pi process management isolated in `agent/pi_runner.go` (or similar) for replaceability.

**Verification.** Browser hits `/chat-v2` with `AGENT_RUNTIME=pi`, gets a streamed answer with at least one tool call (e.g. RAG search returning corpus snippets). Manual test: ask a CE-297 question, observe correct skill auto-invocation and grounded response.

## Phase 6 — Browser UI for new event types

**Scope.** Extend `static/chat.js` and add a new `static/chat-events.js` module. Render paths for `tool_start` / `tool_end` (collapsible panel, name + summary), `skill_start` (chip header above the assistant turn), `compaction` (toast or inline notice), `model_change` (footer update). Inline reasoning falls out of event ordering — Pi emits `text_delta` and `thinking` deltas in source order.

**Deliverables.** New `chat-events.js` module. Updates to `chat.js`, `app.js` (event dispatcher), and `style.css`. Browser smoke test passes against a live `/chat-v2`.

**Verification.** Send a prompt that triggers a tool call: panel renders, output truncates appropriately. Send a prompt that triggers a skill (`/skill:resource-orientation`): chip appears. Hard-refresh and re-test (cache-bust).

## Phase 7 — Rollout flag + production cutover

**Scope.** Wire the `AGENT_RUNTIME=pi|legacy` env var. New `/api/runtime` endpoint returns the active mode. Browser picks the endpoint at boot. Document operational details (model selection, sandbox sweep, log paths) in `claw_study_service.md` (auto-memory). Run `pi` mode for two weeks. If stable, delete the legacy `/chat`, remove the flag, rename `/chat-v2` to `/chat`.

**Deliverables.** Flag-routed handler resolution. Updated runbook memory. Cutover commit deletes `~150` LOC of legacy chat code.

**Verification.** Flag flips work both ways (legacy keeps responding while flag is `legacy`; pi mode returns SSE in the new vocabulary while flag is `pi`). Two-week observation window without P0 regressions before cutover.

## Risks and mitigations

| Risk | Likelihood | Mitigation |
|---|---|---|
| Pi cold-start latency unbearable | medium | Phase 0 measures it; if greater than 10 s consistently, design Phase 5 around a small process pool early |
| OpenCode model mis-formats `claw-cli` invocations | medium | Phase 0 probes each model on a fixed CLI grammar; if persistent, restrict v1 to `deepseek-v4-pro` only or fall back to Path C (TypeScript extensions for typed tools) |
| A skill won't translate cleanly | medium | Phase 3 tackles ports first — bail and re-scope if a skill needs Claude-Code-only affordances |
| `/chat-v2` SSE format breaks the browser | low | Feature-flag means `/chat` stays available for fallback; new vocabulary is additive |
| Memory loader produces too-long `AGENTS.md` | low | Cap at about 3 KB; loader prefers freshest, most-scoped memories |
| Sandbox dir grows without bound | low | Phase 4 includes a sweep job; tested in unit tests before production |
| OpenCode catalog renames again | low | Already handled this once today (2026-05-10 model rename); env-var-driven model selection means a sed-replace fixes it |
