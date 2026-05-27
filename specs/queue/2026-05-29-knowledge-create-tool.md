---
id: 2026-05-29-knowledge-create-tool
title: Agent knowledge_create tool + capture-flow pedagogy rule + /debug/tools
max_wall_clock_minutes: 60
max_diff_lines: 250
max_retries: 1
max_tokens: 200000
requires_visual_approval: false
allow_web_search: false
---

## Goal

Give the agent a `knowledge_create` tool so a Knowledge Component can be captured
during a study session, and add the pedagogy rule that drives it. The governing
principle (see `docs/adr/0007-knowledge-component-as-atomic-note.md`): **the
learner authors the body in their own words; the agent never writes it.** The
agent proposes a short title and elicits the body; the tool stores the learner's
text verbatim. Also adds a `GET /debug/tools` endpoint (lists registered tool
names) so the tool's registration is verifiable over HTTP.

**Depends on ticket `2026-05-28-knowledge-components-entity`** (the
`knowledge_components` table and `CreateKnowledgeComponent` method), which is
queued ahead of this one and will be live on prod when this ticket runs.

Gate note (honest): the *elicitation behaviour* is model-driven and cannot be
bash-verified. The deterministic surface here is "the tool is registered and its
handler creates a component." That is what the verifier and `go test` check; the
pedagogy-rule wording is an eyeball check.

## Implementation plan

### Step 1 — Tool handler (`agent/tools_knowledge.go`, new file)

Mirror `agent/tools_confidence.go`. Add:

```go
func (a *App) ToolKnowledgeCreate(args json.RawMessage) string {
    var p struct {
        Title        string `json:"title"`
        Body         string `json:"body"`
        SourceTaskID string `json:"source_task_id"`
    }
    if err := json.Unmarshal(args, &p); err != nil {
        return "error: " + err.Error()
    }
    if p.Title == "" || p.Body == "" {
        return "error: title and body are both required"
    }
    id, err := a.CreateKnowledgeComponent(p.Title, p.Body, p.SourceTaskID, a.ActiveSessionID())
    if err != nil {
        return "error: " + err.Error()
    }
    return fmt.Sprintf("created knowledge component %s (%q)", id, p.Title)
}
```

(`CreateKnowledgeComponent` and `ActiveSessionID` already exist — from the
prior ticket and `agent/app.go:123` respectively.)

### Step 2 — Register the tool (`agent/tools.go`)

1. In the `GetTools` list (the `log_confidence` entry is around lines 159–171),
   add a `knowledge_create` tool definition with properties `title` (string),
   `body` (string), `source_task_id` (string), `"required": []string{"title","body"}`.
   Description must state the body is the LEARNER's own words, passed through
   verbatim — the agent must not author it.
2. In `ExecuteTool`'s switch (around line 200), add
   `case "knowledge_create": return a.ToolKnowledgeCreate(args)`.

### Step 3 — Capture-flow pedagogy rule (`agent/sandbox.go`)

In the `pedagogySection` (the numbered rules around lines 162–173), add a new
rule after Rule 8 (before the `### Interest log` subsection at line 172). Keep
it one paragraph, in the same voice:

> **9. Capture atomic knowledge components — the learner writes them.** When a
> discrete idea has been understood, propose a SHORT title for that one idea
> (Zettelkasten-atomic: one idea, nothing removable) and ask him to state the
> idea in his own words. Pass his verbatim words as `body` to the
> `knowledge_create` tool, with your proposed (or his edited) `title` and
> `source_task_id` = the active plan task's id. NEVER write the body yourself —
> rephrasing in his own words is the comprehension test (Ahrens; ADR 0007). One
> atom per idea; if his note bundles several, ask him to split it.

### Step 4 — `/debug/tools` endpoint (`handler/debug.go`, `handler/handler.go`)

1. In `handler/debug.go`, add `toolsHandler`: GET only (reuse the
   `methodNotAllowed` guard, as `versionHandler` does at debug.go:11); collect
   the tool names from the registered tool list (call the same function/method
   that builds tools — `GetTools` in `agent/tools.go` around line 52; via
   `h.App` if it is a method, or `agent.GetTools()` if it is a package
   function); respond 200 with `writeJSON(w, http.StatusOK, map[string][]string{"tools": names})`.
2. In `handler/handler.go` `Register` (the `/debug/*` block around lines 63–65),
   add `mux.HandleFunc("/debug/tools", h.toolsHandler)`.

### Step 5 — Tests

1. `agent/` test: with `newMemoryApp(t)`, call `ToolKnowledgeCreate` with a JSON
   payload containing title+body, assert the returned string contains "created
   knowledge component", and assert the component is retrievable via
   `GetKnowledgeComponent`. Assert that a payload missing title or body returns
   an `"error:"` string and creates nothing.
2. `handler/` test (extend `handler/debug_test.go` or new file): GET
   `/debug/tools` returns 200 and the JSON `tools` array contains
   `"knowledge_create"` and `"log_confidence"`; POST returns 405.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
set -euo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

# On current main /debug/tools does not exist (404) and the knowledge_create
# tool is not registered → assertion fails.
resp="$(curl -s -H "Authorization: Bearer $STAGING_TOKEN" "$STAGING_URL/debug/tools")"

if printf '%s' "$resp" | grep -q '"knowledge_create"'; then
  echo "OK: knowledge_create tool is registered"
  exit 0
else
  echo "FAIL: knowledge_create not registered (response: $resp)"
  exit 1
fi
```

### Post-acceptance (must PASS after implementation)

**Same script as above.** After implementation `/debug/tools` lists the
registered tools including `knowledge_create` → assertion passes (exit 0).

### Human-eyeball notes (NOT part of the gate)

- `go test ./...` covers the tool handler (create + error paths) and the
  endpoint. The bash verifier only confirms the tool is wired and reachable.
- The capture-flow behaviour (agent proposes title, learner writes body, body
  stored verbatim) is a prompt rule — confirm by running a real session and
  checking `claw-cli knowledge list` shows a component whose body is in your own
  words, with `source_task_id` set to the active task.

## Done criteria

- [ ] `knowledge_create` tool registered and dispatched; handler creates a component and never authors the body.
- [ ] Rule 9 (capture flow) present in the AGENTS.md pedagogy section.
- [ ] `GET /debug/tools` lists registered tool names; POST → 405.
- [ ] `go build ./...` and `go test ./...` pass.
- [ ] Pre-baseline fails on current main; post-acceptance passes.

## Rollback notes

Pure code addition — `git revert` fully undoes it. No schema or data changes
(the component rows it would create are written through the prior ticket's
table and are harmless if left).
