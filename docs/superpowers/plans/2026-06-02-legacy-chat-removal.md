# Legacy `/chat` Path Removal — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the legacy `/chat` path and everything used only by it; hardwire the UI to `/chat-v2`, leaving exactly one chat path with no user-visible change.

**Architecture:** Two cohesive deletion tasks — (1) handler/frontend layer (routes, `handleChat`, runtime toggle, `/debug/tools`), then (2) the now-orphaned agent-package layer (the Go tool-loop, tool registry, legacy system-prompt builders, legacy tool wrappers) — followed by deploy + live verification. Go does not flag unused package-level funcs, so deletion is deliberate and grep-verified; each task ends green (`go build ./... && go test ./...`) and the test edits accompany their symbol deletions in the same task.

**Tech Stack:** Go 1.26 (`/opt/homebrew/bin/go`), embedded SPA (`static/app.js`), `net/http`, SQLite.

**Spec:** `docs/superpowers/specs/2026-06-02-legacy-chat-removal-design.md`

**Conventions:** build/test `/opt/homebrew/bin/go`; commit `git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "..."`; branch main.

**Verified keep/delete facts (do not re-derive):**
- KEEP: Pi path (`handler/chat_v2.go`, `agent/pi_runner.go`, `agent/sandbox.go`); `LLMClient`, `NewLLMClient`, `CallLLMNonStreaming`, `cleanTitle`, `GenerateTitle` (used by `chat_v2.go:124`), `GenerateSummary` (`sessions.go` summary path); `App.CreateCourse`, `App.CourseName`, `App.AppCourseName` (used by `db.go`), `readFileWithLog` (used by `tools_skill.go`); all `Tool*` methods claw-cli calls (`ToolUpdatePlan/ToolRewritePlan/ToolRAGSearch/ToolPDFExtract/ToolStudySkill/ToolSaveNote`); S1/S2 surface.
- DELETE (legacy-only): routes `/chat`,`/api/runtime`,`/debug/tools`; `handleChat`; `handler/runtime.go` + `handler/runtime_test.go`; `toolsHandler` + its 2 tests in `debug.go`/`debug_test.go`; `ProcessWithTools`,`CallLLM`,`ParseStream`,`jsonEscape` (`llm.go`); `GetTools`,`ExecuteTool`,`ToolDef`,`ToolFunc`,`ToolCall`,`ToolCreateCourse` (`tools.go`); `LoadSystemPrompt`,`toolsAndRulesPrompt`,`fallbackSystemPrompt` (`agent.go`); `GetSessionSystemPrompt` (`tools_skill.go`); files `agent/tools_confidence.go` (`ToolLogConfidence`) + `agent/tools_knowledge.go` (`ToolKnowledgeCreate`); `static/app.js` `loadRuntimeEndpoint`.
- `pi_runner.go`'s `ToolCall` is a struct *field* of a different type (`piToolCall`) — NOT `agent.ToolCall`; leave it.

---

### Task 1: Remove the handler/frontend entry points

**Files:**
- Modify: `handler/handler.go` (routes), `handler/sessions.go` (`handleChat`), `handler/debug.go` (`toolsHandler`), `static/app.js` (`loadRuntimeEndpoint`)
- Delete: `handler/runtime.go`, `handler/runtime_test.go`
- Modify (tests): `handler/debug_test.go` (drop 2 toolsHandler tests)
- Test: `handler/handler_test.go` (new guard test, or wherever route tests live — create if absent)

After this task the agent-layer legacy symbols (`ProcessWithTools`, `GetTools`, `LoadSystemPrompt`, …) are orphaned but still compile (Go allows unused funcs); Task 2 removes them.

- [ ] **Step 1: Write the guard test (route removal)**

Verified API: routes are registered by `func (h *Handler) Register(mux *http.ServeMux)` (in `handler/handler.go`); the test-handler constructor is `newTestHandler(t) *Handler` (in `handler/testutil_test.go`). There is a `/` catch-all (`handleIndex`) that returns `http.NotFound` for any path != `/`, so once the explicit `/chat` route is removed, `/chat` falls through to `handleIndex` and genuinely 404s. Add to a new `handler/handler_test.go`:

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyChatRouteRemoved(t *testing.T) {
	h := newTestHandler(t)
	mux := http.NewServeMux()
	h.Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/chat", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("/chat should be 404 after removal (falls through to handleIndex), got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run it — expect FAIL (route still present)**

Run: `/opt/homebrew/bin/go test ./handler/ -run TestLegacyChatRouteRemoved -v`
Expected: FAIL (route still registered → not 404).

- [ ] **Step 3: Remove the three routes**

In `handler/handler.go`, delete these lines:
```go
	mux.HandleFunc("/chat", h.handleChat)
	mux.HandleFunc("/api/runtime", h.handleRuntime)
	mux.HandleFunc("/debug/tools", h.toolsHandler)
```
(Keep `/chat-v2` and all others.)

- [ ] **Step 4: Delete `handleChat`**

In `handler/sessions.go`, delete the entire `handleChat` function (starts at `func (h *Handler) handleChat(w http.ResponseWriter, r *http.Request) {`, ~line 291, through its closing brace). If `maybeGenerateSummary` or other helpers in this file are ONLY called by `handleChat`, grep them; the summary path is also used by `/chat-v2` — verify before removing anything beyond `handleChat` itself. Remove now-unused imports in `sessions.go` if the compiler flags them.

- [ ] **Step 5: Delete `handler/runtime.go` and `handler/runtime_test.go`**

```bash
git rm handler/runtime.go handler/runtime_test.go
```

- [ ] **Step 6: Delete `toolsHandler` + its tests**

In `handler/debug.go`, delete the `toolsHandler` method. In `handler/debug_test.go`, delete `TestToolsHandler` and `TestToolsHandlerMethodNotAllowed` (keep the version/schema/band tests). Remove imports left unused.

- [ ] **Step 7: Hardwire the frontend to `/chat-v2`**

In `static/app.js`, the function `loadRuntimeEndpoint()` (around line 89) fetches `/api/runtime` to choose the endpoint. Delete that function and replace its single call site so the chat endpoint is the constant `'/chat-v2'`. Find the call site:
```bash
grep -n "loadRuntimeEndpoint\|chatEndpoint" static/app.js static/chat.js
```
Replace the awaited `loadRuntimeEndpoint()` result with the literal `'/chat-v2'` (e.g. `const chatEndpoint = '/chat-v2';`) and remove the now-dead function. Do not change any other behavior.

- [ ] **Step 8: Run the guard test + full handler suite + build**

Run: `/opt/homebrew/bin/go test ./handler/ -v && /opt/homebrew/bin/go build .`
Expected: `TestLegacyChatRouteRemoved` PASSES; all other handler tests pass; build green (agent-layer orphans still compile).

- [ ] **Step 9: Commit**

```bash
git add -A
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "refactor: remove legacy /chat, /api/runtime, /debug/tools routes + handlers; hardwire UI to /chat-v2"
```

---

### Task 2: Remove the orphaned agent-package legacy code

**Files:**
- Modify: `agent/llm.go`, `agent/tools.go`, `agent/agent.go`, `agent/tools_skill.go`
- Delete: `agent/tools_confidence.go`, `agent/tools_knowledge.go`
- Modify (tests): `agent/agent_test.go`, `agent/tools_dispatch_test.go`, `agent/db_test.go`

This is deliberate deletion; after each group, `go build ./...` must stay green and the grep-gate must shrink. Delete symbols and the tests that reference them together (a test referencing a deleted symbol breaks test-binary compilation).

- [ ] **Step 1: Delete the legacy tool wrappers + their tests**

```bash
git rm agent/tools_confidence.go agent/tools_knowledge.go
```
In `agent/db_test.go` delete `TestToolLogConfidence_DispatchedRoundTrip` (line ~428, the func calling `a.ToolLogConfidence(...)`). In `agent/tools_dispatch_test.go` delete `TestToolKnowledgeCreate_Success` and `TestToolKnowledgeCreate_MissingFields`.

- [ ] **Step 2: Delete the tool registry + dispatcher (`agent/tools.go`)**

Delete from `agent/tools.go`: the `GetTools` func, the `ExecuteTool` method, the `ToolCreateCourse` method, and the now-unused types `ToolDef`, `ToolFunc`, `ToolCall`. KEEP `CourseName`, `AppCourseName` (used by `db.go`). After this, `agent/tools.go` should contain only `CourseName`/`AppCourseName` (and the package clause + imports they need) — if that's all that remains, that's fine; trim unused imports.

In `agent/tools_dispatch_test.go`: delete every test that calls `ExecuteTool` or `GetTools` (`TestExecuteTool_*` ×7, `TestGetTools_NotEmpty`). KEEP `TestCourseName`. The file should end up containing only `TestCourseName` (plus package/imports it uses) — trim unused imports. (If cleaner, move `TestCourseName` into a new `agent/tools_test.go` and `git rm agent/tools_dispatch_test.go`; either is acceptable.)

- [ ] **Step 3: Delete the Go tool-loop in `agent/llm.go`**

Delete from `agent/llm.go`: `ProcessWithTools`, `CallLLM`, `ParseStream`, and `jsonEscape` (verify `jsonEscape` has no remaining caller after `CallLLM`/`ParseStream` go: `grep -n jsonEscape agent/*.go`). KEEP `LLMClient`, `NewLLMClient`, `CallLLMNonStreaming`, `cleanTitle`, `GenerateTitle`, `GenerateSummary`, `Message`. Trim imports the compiler flags (e.g. `net/http`, `io` may become unused — let the build tell you).

- [ ] **Step 4: Delete the legacy system-prompt builders**

In `agent/agent.go`: delete `LoadSystemPrompt`, the `toolsAndRulesPrompt` const, and the `fallbackSystemPrompt` const. KEEP `readFileWithLog` (used by `tools_skill.go`). In `agent/tools_skill.go`: delete `GetSessionSystemPrompt`. In `agent/agent_test.go`: delete `TestLoadSystemPrompt_FallbackWhenNoFiles` and `TestLoadSystemPrompt_ConcatenatesFiles`; KEEP `TestReadFileWithLog_Missing` and `TestReadFileWithLog_Present`.

- [ ] **Step 5: Build + vet + full suite**

Run: `/opt/homebrew/bin/go vet ./... && /opt/homebrew/bin/go test ./... && /opt/homebrew/bin/go build . && /opt/homebrew/bin/go build -o /tmp/cc ./claw-cli`
Expected: all green. If the build flags an unused import or a lingering reference, fix it (remove the import / delete the straggler). If a reference you didn't expect appears, STOP and report — it may mean a symbol is shared and shouldn't be deleted.

- [ ] **Step 6: Grep-gate (no legacy symbols remain in live code)**

Run:
```bash
grep -rn "handleChat\|ProcessWithTools\|GetTools\|ExecuteTool\|LoadSystemPrompt\|toolsAndRulesPrompt\|GetSessionSystemPrompt\|ToolLogConfidence\|ToolKnowledgeCreate\|ToolCreateCourse\|handleRuntime\|toolsHandler\|/api/runtime\|loadRuntimeEndpoint" --include=*.go --include=*.js . | grep -v -E "docs/|specs/|/worktrees/"
```
Expected: NO output (every match would be in docs/specs/worktrees, which are excluded). If anything prints from live code, delete that straggler and re-run Step 5.

- [ ] **Step 7: Commit**

```bash
git add -A
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "refactor: remove orphaned legacy tool-loop, tool registry, and system-prompt builders"
```

---

### Task 3: Build, deploy, live-verify

**Files:** none (operational).

- [ ] **Step 1: Cross-compile the server (claw-cli unchanged in S3, but rebuild it too for safety)**
```bash
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/claw-cli-linux ./claw-cli
ls -la /tmp/study-app-linux /tmp/claw-cli-linux
```

- [ ] **Step 2: Deploy (back up, restart)**
```bash
scp /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
scp /tmp/claw-cli-linux nanoclaw:/home/eduardo/stack/study-app/bin/claw-cli.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak && cp claw-cli claw-cli.bak && mv study-app.new study-app && mv claw-cli.new claw-cli && chmod +x study-app claw-cli && export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service && sleep 3 && systemctl --user is-active study-app.service'
```
Expected: `active`.

- [ ] **Step 3: Confirm `/chat` is gone and `/chat-v2`/health are alive**
```bash
URL=https://study.claw-study.xyz
TOKEN=$(ssh nanoclaw 'grep ^AUTH_TOKEN= ~/stack/study-app/.env | cut -d= -f2')
rtk proxy curl -s -o /dev/null -w "chat=%{http_code}\n" -X POST -H "Authorization: Bearer $TOKEN" "$URL/chat"          # expect 404
rtk proxy curl -s -o /dev/null -w "health=%{http_code}\n" -H "Authorization: Bearer $TOKEN" "$URL/debug/health"        # expect 200
rtk proxy curl -s -o /dev/null -w "runtime=%{http_code}\n" -H "Authorization: Bearer $TOKEN" "$URL/api/runtime"        # expect 404 (removed)
```
Expected: `chat=404`, `health=200`, `runtime=404`.

- [ ] **Step 4: Live `/chat-v2` smoke**

In the app, open a study session and send one message; confirm the SSE stream responds and a plan/PDF tool call still works. Confirm a session summary still generates after enough messages (the kept `GenerateSummary` path). If anything is broken, roll back:
```bash
ssh nanoclaw 'cd ~/stack/study-app/bin && mv study-app study-app.broken && mv study-app.bak study-app && export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service'
```

---

## Self-Review

- **Spec coverage:** routes/handlers → Task 1 Steps 3–6; frontend hardwire → Task 1 Step 7; `llm.go` tool-loop → Task 2 Step 3; `tools.go` registry/types/`ToolCreateCourse` → Task 2 Step 2; `agent.go` prompt builders → Task 2 Step 4; `GetSessionSystemPrompt` → Task 2 Step 4; legacy wrapper files → Task 2 Step 1; test cleanup → distributed across the matching steps; guard test → Task 1 Step 1; grep-gate → Task 2 Step 6; deploy/manual → Task 3. KEEP list honored (no edits to chat_v2/pi_runner/sandbox; `GenerateSummary`/`GenerateTitle`/`CreateCourse`/`CourseName`/`readFileWithLog` and all claw-cli `Tool*` methods explicitly preserved).
- **Placeholders:** none — every deletion names the exact symbol/file; the two "confirm the real API name" notes (Task 1 Step 1 routes accessor; Task 2 import trimming) are verification instructions, not unfinished content, because Go's compiler is the authoritative check there.
- **Type/name consistency:** real names verified against the tree — `ToolKnowledgeCreate` (not ToolCreateKnowledge), `ToolCreateCourse`, `fallbackSystemPrompt`, `readFileWithLog` (kept), `loadRuntimeEndpoint`. `pi_runner.go`'s `ToolCall` field is a different type and is explicitly excluded from deletion.
- **Ordering safety:** Task 1 leaves agent-layer symbols orphaned-but-compiling (Go allows unused funcs), so it ends green independently; Task 2 then removes them with their tests in the same task, also ending green. No intermediate broken state.
