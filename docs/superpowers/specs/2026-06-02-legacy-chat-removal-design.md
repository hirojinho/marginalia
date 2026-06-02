# S3 — Remove the Legacy `/chat` Path — Design

**Date:** 2026-06-02
**Status:** Approved, pending implementation plan
**Part of:** the 4-spec pedagogy-consolidation series (S1 ✅, S2 ✅, **S3**, S4). Independent of S1/S2 but cleaner after them (the surviving prompt source — `agent/sandbox.go` — is now settled).

## Problem

The app has two chat paths: legacy `/chat` (direct Go LLM call + Go tool-call loop, `agent.LoadSystemPrompt` + `toolsAndRulesPrompt` + `CLAUDE.local.md`) and Pi `/chat-v2` (subprocess, bash-only, `AGENTS.md` from `sandbox.go`). `/chat-v2` is the only path the UI uses (`/api/runtime` returns `mode=pi`). The legacy path is dead weight: it was where R4/R1 confidence persistence originally lived as Go tools Pi can't call (the root cause S1 fixed). Keeping it means every pedagogy change must be mirrored in two prompt sources (`sandbox.go` vs `agent.go`+`CLAUDE.local.md`) — the divergence S1/S2 deliberately deferred to here.

## Goal

Delete the legacy `/chat` path and everything used only by it; hardwire the UI to `/chat-v2`. The app keeps exactly one chat path. No behavior change for the user.

## Non-goals

- No change to the Pi `/chat-v2` path, `pi_runner.go`, `sandbox.go`, or `claw-cli`.
- Not deleting shared infrastructure (see KEEP list).
- Not removing the `CLAUDE.local.md` *file* from disk (only its use as a system-prompt source); the repo copy can stay as documentation.

## What to DELETE (verified used only by the legacy path)

**Routes (`handler/handler.go`):** `/chat`, `/api/runtime`, `/debug/tools`.

**Handlers:**
- `handleChat` (`handler/sessions.go:291`) and its helper call to `GetSessionSystemPrompt(...LoadSystemPrompt())`.
- `handleRuntime` + `runtimeResponse` — delete the whole `handler/runtime.go`.
- `toolsHandler` (`handler/debug.go`) — the `/debug/tools` handler (consumes `GetTools`).

**`agent/llm.go`** (the tool-loop half — keep the summary/title half):
- DELETE: `ProcessWithTools` (only `handleChat` calls it), `CallLLM` (only `ProcessWithTools`), `ParseStream` (only `ProcessWithTools`), and `jsonEscape` if it becomes unused after those go.
- KEEP: `LLMClient`, `NewLLMClient`, `CallLLMNonStreaming`, `cleanTitle`, `GenerateTitle` (used by `chat_v2.go:124`), `GenerateSummary` (used by `sessions.go` summary path), `Message`.

**`agent/tools.go`:**
- DELETE: `GetTools`, `ExecuteTool`, the `ToolDef` / `ToolFunc` / `ToolCall` types (used only by `GetTools`/`CallLLM`/`ParseStream`/`ExecuteTool`), and `ToolCreateCourse` (the LLM-tool wrapper; `CreateCourse` itself is kept and called directly by `handler/courses.go` + `claw-cli`).
- KEEP: `CourseName`, `AppCourseName` (verify callers; keep if used outside the deleted set — they are general helpers).

**`agent/agent.go`:** DELETE `LoadSystemPrompt` and the `toolsAndRulesPrompt` const. If the file is then empty (or only has the package clause), delete the file.

**`agent/tools_skill.go`:** DELETE `GetSessionSystemPrompt` (only `handleChat` calls it). Keep `ToolStudySkill` (claw-cli uses it) and the rest of the file.

**Legacy Go tool wrappers (whole files, each holds only the wrapper):**
- `agent/tools_confidence.go` (`ToolLogConfidence`) — claw-cli's `confidence log` calls `App.LogConfidence` directly (S1), not this.
- `agent/tools_knowledge.go` (`ToolCreateKnowledge`) — claw-cli's `knowledge create` calls `App.CreateKnowledgeComponent` directly.
  (Confirm each file contains ONLY the wrapper before deleting the file; if it has kept helpers, delete just the function.)

**Frontend (`static/app.js`):** `loadRuntimeEndpoint()` fetches `/api/runtime` to choose `/chat` vs `/chat-v2`. Replace its use with a hardwired `/chat-v2` (delete the function and the `/api/runtime` fetch; the chat endpoint is now constant).

**Tests referencing the deleted cluster** (update or delete the specific tests, not whole unrelated files):
- `agent/agent_test.go` — tests of `LoadSystemPrompt` / `toolsAndRulesPrompt`.
- `agent/tools_dispatch_test.go` — tests of `GetTools` / `ExecuteTool` dispatch (likely the whole file).
- `agent/db_test.go` — whichever single test references a deleted symbol (grep to find it; keep the rest of the file, incl. the S2 `TestHasConfidenceAtLeast`).
- Any `handler/*_test.go` for `handleChat` / runtime / `/debug/tools` (grep to confirm and remove).

## What to KEEP (shared — must still compile & pass)

- The entire Pi path: `handler/chat_v2.go`, `agent/pi_runner.go`, `agent/sandbox.go`.
- `LLMClient` + `GenerateSummary` + `GenerateTitle` + `CallLLMNonStreaming` (summaries/titles, both historically and on `/chat-v2`).
- All `Tool*` methods claw-cli invokes: `ToolUpdatePlan`, `ToolRewritePlan`, `ToolRAGSearch`, `ToolPDFExtract`, `ToolStudySkill`, `ToolSaveNote`.
- `App.CreateCourse`, `App.LogConfidence`, `App.CreateKnowledgeComponent`, `App.HasConfidenceAtLeast`, the whole mastery-gate/plan/confidence surface from S1/S2.
- `main.go`'s `NewLLMClient` wiring (still needed for summaries/titles) and the `Handler.LLM` field.

## Method (compiler + grep guided)

Go does not flag unused package-level funcs, so deletion is deliberate: remove the entry points (routes/handlers/`ProcessWithTools`), then remove each now-orphaned symbol, grepping to confirm no remaining non-test references before each deletion, and removing the tests that covered deleted code. The build + full test suite + a new guard test are the safety net.

## Testing

- **Guard test (new):** a handler test asserting `GET/POST /chat` returns 404 (route removed) and that `/chat-v2` is still registered (e.g. it does NOT 404 — a method/era-appropriate assertion mirroring existing handler tests). Confirms the removal without breaking the live path.
- Remove tests of deleted symbols; keep all others.
- `/opt/homebrew/bin/go vet ./... && go test ./... && go build . && go build -o /tmp/cc ./claw-cli` all green.
- Grep gate: after the change, `grep -rn "handleChat\|ProcessWithTools\|GetTools\|ExecuteTool\|LoadSystemPrompt\|toolsAndRulesPrompt\|GetSessionSystemPrompt\|ToolLogConfidence\|ToolCreateKnowledge\|/api/runtime\|/debug/tools" --include=*.go --include=*.js .` (excluding worktrees/specs/plans) returns nothing in live code.

Manual acceptance: deploy both binaries; confirm the app still loads and a `/chat-v2` study turn works (SSE streams, plan/PDF tools work); confirm `GET /chat` now 404s; confirm session summaries still generate.

## Deploy

Standard flow: cross-compile `study-app` (+ `claw-cli` if it changed — it should not in S3), scp, restart. The frontend (`static/app.js`) is embedded in the server binary, so the hardwired endpoint ships with the rebuild.
