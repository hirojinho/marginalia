---
id: 2026-06-08-kc-capture-box
title: Knowledge Component HTML capture box — learner-authored KC panel in the left rail
max_wall_clock_minutes: 60
max_diff_lines: 250
max_retries: 1
max_tokens: 60000
requires_visual_approval: true
allow_web_search: false
---

## Goal

Add a "Knowledge" section to the left rail where the learner can browse their
Knowledge Components and author new ones directly — typing title and body into
an inline form and submitting, without going through the chat. This closes the
frontend gap on the KC arc (entity, create tool, and retrieval loop all shipped;
the learner still has to type KC bodies into the chat so the agent can call
`claw-cli knowledge create`). After this spec, the agent in chat still proposes
titles and elicits bodies ("state that idea in your own words"), but the learner
can also self-serve: open the KC panel and write one directly.

Per ADR 0007: the body is authored by the learner in their own words — the
agent never writes it. The capture box makes this a first-class UI action
instead of a chat-mediated back-and-forth.

## Implementation plan

### Step 1 — Add HTTP endpoints for KC CRUD (`handler/knowledge.go` + test)

Create `handler/knowledge.go` with:

**`GET /api/knowledge`** — list all Knowledge Components, most-recent-first.
Returns JSON array of `KnowledgeComponent` (already defined in `agent/types.go`).
Accepts optional query param `?limit=N` (default 50).

**`POST /api/knowledge`** — create a new Knowledge Component.
Accepts JSON body: `{"title": "...", "body": "...", "source_task_id": "..." (optional), "source_session_id": N (optional)}`.
Returns 201 with `{"id": "<uuid>"}` on success.
Validates: title and body are required, non-empty, max 500 chars each.
Uses `app.CreateKnowledgeComponent` (already exists in `agent/db.go`).

Register both routes in `handler/handler.go`'s `Register` method.

Create `handler/knowledge_test.go` with:
- `TestHandleKnowledgeList` — seeds a few KCs via the app, hits GET, checks count and structure
- `TestHandleKnowledgeCreate` — POST a valid KC, verify 201 + id; POST with empty title, verify 400
- `TestHandleKnowledgeCreateMaxLength` — POST with >500 char title, verify 400

### Step 2 — Add KC section to the left rail (`static/rail.js` + `static/style.css`)

In `renderRail()`, after the plan div and before `renderOther()`, add a new section:

```html
<div class="rail-bucket rail-knowledge">
  <div class="rail-other-label">Knowledge <button class="rail-settings-btn" data-action="add-kc" title="Add Knowledge Component">+</button></div>
  <div id="kc-list"></div>
</div>
```

**On rail load**: after `loadRailData()` resolves, fetch `GET /api/knowledge?limit=25` and populate `#kc-list` with one `<div class="rail-other-item kc-item">` per KC. Each item shows the title. Clicking a KC expands it inline to show the body.

**"+" button**: opens an inline form inside `#kc-list`:
```html
<div class="kc-form">
  <input type="text" class="kc-title-input" placeholder="One-idea title..." maxlength="500">
  <textarea class="kc-body-input" placeholder="State the idea in your own words..." maxlength="500" rows="3"></textarea>
  <div class="kc-form-actions">
    <button class="kc-cancel-btn">Cancel</button>
    <button class="kc-submit-btn">Save</button>
  </div>
</div>
```

On submit: POST to `/api/knowledge` with the form values. On success, re-fetch the KC list and re-render. On error, show the error inline.

Add minimal CSS in `static/style.css` for `.rail-knowledge`, `.kc-item`, `.kc-form`, `.kc-title-input`, `.kc-body-input`, `.kc-form-actions`.

**KC click-to-expand**: clicking a KC item toggles an inline body display. The body is shown in a `<div class="kc-body">` with the text. Clicking again collapses it.

Export a function `refreshKCList()` so the chat panel can trigger a refresh after the agent creates a KC via `claw-cli knowledge create` (future use — no chat.js changes in this spec).

### Step 3 — Verify build and no regressions

No changes to chat.js, app.js, or the SSE flow. The capture box is an additive, standalone panel.

## Verification recipe

### Pre-baseline (must FAIL on current main)

The gate expects this script to exit non-zero on current main (features absent).
Exit 0 signals "feature already exists" → gate fails the run.

```bash
# Pre-baseline: features should NOT exist on current main.
# Exit 1 when features are absent (expected pre-state → gate proceeds).
# Exit 0 when any feature IS present (unexpected → gate fails, correct).

# 1. No handler/knowledge.go on main.
if [ -f handler/knowledge.go ]; then
  echo "UNEXPECTED: handler/knowledge.go already exists on main"
  exit 0
fi

# 2. No /api/knowledge route registered in handler/handler.go.
if grep -q '/api/knowledge' handler/handler.go; then
  echo "UNEXPECTED: /api/knowledge route already registered on main"
  exit 0
fi

# 3. No rail-knowledge section in static/rail.js.
if grep -q 'rail-knowledge' static/rail.js; then
  echo "UNEXPECTED: rail-knowledge already in rail.js on main"
  exit 0
fi

# 4. No rail-knowledge styles in static/style.css.
if grep -q 'rail-knowledge' static/style.css; then
  echo "UNEXPECTED: rail-knowledge already in style.css on main"
  exit 0
fi

# All features absent — expected. Signal "fail" (non-zero) so gate proceeds.
exit 1
```

### Post-acceptance (must PASS after implementation)

```bash
# 1. GET /api/knowledge returns 200 with a JSON array.
curl -sf http://localhost:8080/api/knowledge | python3 -c "import sys,json; d=json.load(sys.stdin); assert isinstance(d, list), 'not an array'; print('PASS: GET')"

# 2. POST /api/knowledge creates a KC and returns 201.
RESP=$(curl -sf -w "\n%{http_code}" -X POST \
  -H "Content-Type: application/json" \
  -d '{"title":"Test KC","body":"This is a test."}' \
  http://localhost:8080/api/knowledge)
HTTP_CODE=$(echo "$RESP" | tail -1)
ID=$(echo "$RESP" | head -1 | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
[ "$HTTP_CODE" = "201" ] && [ -n "$ID" ] && echo "PASS: POST 201 id=$ID" || echo "FAIL: HTTP $HTTP_CODE"

# 3. Created KC appears in list.
curl -sf "http://localhost:8080/api/knowledge?limit=5" | python3 -c "
import sys,json; ids=[d['id'] for d in json.load(sys.stdin)]
assert '$ID' in ids, f'$ID not found'; print('PASS: KC in list')
"

# 4. Empty title returns 400.
[ "$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d '{"title":"","body":"x"}' http://localhost:8080/api/knowledge)" = "400" ] && echo "PASS: empty title 400"

# 5. Empty body returns 400.
[ "$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d '{"title":"x","body":""}' http://localhost:8080/api/knowledge)" = "400" ] && echo "PASS: empty body 400"

# 6. Build + vet + tests.
go build ./... && echo "PASS: build"
go vet ./... && echo "PASS: vet"
go test ./... -count=1 && echo "PASS: tests"

# 7. Frontend elements present.
curl -sf http://localhost:8080/ | grep -q 'rail-knowledge' && echo "PASS: rail-knowledge in HTML"
curl -sf http://localhost:8080/ | grep -q 'data-action="add-kc"' && echo "PASS: add-kc button in HTML"
```

### Human-eyeball notes

- Open the app, select a course. The left rail should have a "Knowledge" section below the plan with a "+" button.
- Click "+" — a form appears with title and body fields.
- Type a title and body, click Save — the KC appears in the list.
- Click the KC — it expands to show the body.
- The existing agent-mediated flow (agent calls `claw-cli knowledge create` in chat) continues to work unchanged.
- KCs are shown across all courses (no course filter in this version).

## Done criteria

- [ ] `GET /api/knowledge` endpoint registered and returns KC list.
- [ ] `POST /api/knowledge` endpoint registered, validates, and creates KCs.
- [ ] Both endpoints have test coverage (list, create, validation).
- [ ] "Knowledge" section rendered in the left rail with "+" button.
- [ ] Inline form appears on "+" click; submits to POST endpoint; list refreshes.
- [ ] KC items are click-to-expand (show body inline).
- [ ] Minimal CSS for the new elements (not broken visually).
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass.
- [ ] Pre-baseline fails on current main; post-acceptance passes on the branch.

## Rollback notes

Pure additive. Revert the commit. No schema migration, no data impact — the
`knowledge_components` table already exists and the new endpoints only read/write it.
