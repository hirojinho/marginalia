---
id: 2026-05-21-session-stats-endpoint
title: Add /api/sessions/stats endpoint returning per-session message + reasoning counters
estimated_complexity: small
max_wall_clock_minutes: 30
max_diff_lines: 200
max_retries: 1
max_tokens: 100000
requires_visual_approval: false
allow_web_search: false
model: deepseek-v4-flash
thinking: off
created_at: 2026-05-21
created_by: laptop-cc + eduardo
---

## Goal

Add `GET /api/sessions/stats?id=N` returning per-session aggregate counters as JSON:

```json
{
  "session_id": 42,
  "message_count": 12,
  "user_message_count": 6,
  "assistant_message_count": 6,
  "first_message_at": "2026-05-20T14:03:11Z",
  "last_message_at": "2026-05-21T09:17:44Z",
  "total_reasoning_chars": 4821
}
```

**Why:** lays the groundwork for the R-Later pedagogy dashboard (confidence trajectories / learning curves per ROADMAP "Later") which needs per-session aggregates that the current API does not expose. Also useful for the morning digest: lets the overnight pipeline summarize "sessions touched yesterday" without joining `messages` ad-hoc. This ticket is also the second vertical-slice validation of the overnight pipeline (after `2026-05-21-debug-version-endpoint.md`).

## References

- Existing handler patterns: `handler/sessions.go` already implements `listSessions`, `deleteSession`, `renameSession`, `handleSessionMessages`. Match their style — handler method on `*Handler`, query-param `id` parsing via `parseInt64`, `writeJSON` / `writeError` helpers.
- Existing DB query patterns: `agent/db.go:GetSessionHistory` (line 355) shows the messages-table query shape. The `messages` table already has columns `role`, `content`, `reasoning`, `created_at` (schema at `agent/db.go:104`; `reasoning` added via ALTER at line 149).
- Route registration site: `handler/handler.go:Register` (lines 42-64). Add new route alongside other `/api/sessions/...` entries (lines 48-50).
- Test patterns: `handler/sessions_test.go` uses `newTestHandler(t)` (`handler/testutil_test.go`) which wires an in-memory DB. Empty-session and create-then-query patterns demonstrated at lines 13-62.

## Implementation plan

1. **Add `GetSessionStats` DB method** in `agent/db.go` after `GetSessionHistory` (around line 370). Signature:
   ```go
   func (a *App) GetSessionStats(sessionID int64) (SessionStats, error)
   ```
   Where `SessionStats` is a new struct in `agent/types.go` next to the existing `Session` / `Message` types:
   ```go
   type SessionStats struct {
       SessionID             int64   `json:"session_id"`
       MessageCount          int     `json:"message_count"`
       UserMessageCount      int     `json:"user_message_count"`
       AssistantMessageCount int     `json:"assistant_message_count"`
       FirstMessageAt        *string `json:"first_message_at"`
       LastMessageAt         *string `json:"last_message_at"`
       TotalReasoningChars   int     `json:"total_reasoning_chars"`
   }
   ```
   Pointer-string for the timestamps so empty sessions serialize them as `null`, not `""`.

   Implementation: one SQL query against `messages` aggregating by `session_id`. Use `COUNT(*)`, `SUM(CASE WHEN role = 'user' THEN 1 ELSE 0 END)`, `SUM(CASE WHEN role = 'assistant' THEN 1 ELSE 0 END)`, `MIN(created_at)`, `MAX(created_at)`, and `SUM(LENGTH(reasoning))`. Use `sql.NullString` for the MIN/MAX scans; convert to `*string` (nil if not Valid) on the way out. `LENGTH()` in SQLite is byte length; that's fine for this counter — the field is named `total_reasoning_chars` but documented in the JSON as "chars" meaning UTF-8 byte count. Do not switch to rune counting; SQLite has no built-in rune count and byte-length is sufficient.

   Before running the aggregate, verify the session exists with a `SELECT 1 FROM sessions WHERE id = ?` check. If no row, return `sql.ErrNoRows` wrapped: `fmt.Errorf("session not found: %w", sql.ErrNoRows)`.

2. **Add `getSessionStats` handler method** in `handler/sessions.go` after `handleSessionMessages` (around line 180). Signature:
   ```go
   func (h *Handler) getSessionStats(w http.ResponseWriter, r *http.Request)
   ```
   - Method gate: `if methodNotAllowed(w, r, http.MethodGet) { return }`
   - Parse `id` from query via `parseInt64(r.URL.Query().Get("id"), "id")`; 400 on parse error matching existing pattern.
   - Call `h.App.GetSessionStats(id)`.
   - On `errors.Is(err, sql.ErrNoRows)`: return 404 via `writeError(w, http.StatusNotFound, "session not found")`.
   - On other error: `writeServerError(w, "get session stats", err)`.
   - On success: `writeJSON(w, http.StatusOK, stats)`.

3. **Register the route** in `handler/handler.go:Register` (line 50 area). Add:
   ```go
   mux.HandleFunc("/api/sessions/stats", h.getSessionStats)
   ```
   Place immediately after the existing `/api/sessions/messages` line so the file groups all `/api/sessions/*` routes together. Auth is provided by `AuthMiddleware` which wraps the whole mux per the existing setup; no per-route auth wiring needed.

4. **Add unit tests in `handler/sessions_test.go`** at the end of the file. Three test functions:

   - `TestHandleSessionStatsEmpty`: create a session via the POST flow used in `TestHandleSessionsCreateThenList`; GET `/api/sessions/stats?id=<created.ID>`; assert 200, `message_count == 0`, `user_message_count == 0`, `assistant_message_count == 0`, `total_reasoning_chars == 0`, `first_message_at == nil` (pointer is nil after JSON decode into a `*string`), `last_message_at == nil`.

   - `TestHandleSessionStatsWithMessages`: create a session; directly insert 3 messages via `h.App.SaveMessage(id, "user", "hi")` (twice) and `h.App.SaveAssistantMessage(id, "answer", "thinking-1234")` (once). GET stats; assert `message_count == 3`, `user_message_count == 2`, `assistant_message_count == 1`, `total_reasoning_chars == len("thinking-1234")` (= 12), `first_message_at` and `last_message_at` both non-nil ISO8601-shaped strings.

   - `TestHandleSessionStatsNotFound`: GET `/api/sessions/stats?id=999999` against an empty handler; assert 404 and JSON error body contains `"session not found"`.

   - `TestHandleSessionStatsMissingID`: GET `/api/sessions/stats` (no `id` param); assert 400.

5. **No CHANGELOG entry** — that's done by the deploy step in the overnight pipeline, not part of this spec.

## Verification recipe

### Pre-baseline (must FAIL on current main)

This is the **single canonical verifier**; the gate-runner runs it twice with different binaries. Against current main, step 2 (`curl -sf` to `/api/sessions/stats`) returns a 404 so the script exits non-zero — that's the desired pre-baseline state.

```bash
set -euo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

AUTH="Authorization: Bearer $STAGING_TOKEN"

# 1) Create a session we can query stats on.
create_resp=$(curl -sf -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"course_id":"verifier-stats","topic":"stats-verifier"}' \
  "$STAGING_URL/api/sessions")
session_id=$(echo "$create_resp" | jq -r '.id')
[ -n "$session_id" ] && [ "$session_id" != "null" ] || { echo "FAIL: create did not return id"; exit 1; }

# 2) Hit the new endpoint. On current main this 404s; curl -sf returns non-zero.
stats=$(curl -sf -H "$AUTH" "$STAGING_URL/api/sessions/stats?id=$session_id")

# 3) Structural assertions on the JSON.
echo "$stats" | jq -e ".session_id == $session_id"                     > /dev/null || { echo "FAIL: session_id mismatch"; exit 1; }
echo "$stats" | jq -e '.message_count == 0'                            > /dev/null || { echo "FAIL: message_count not 0"; exit 1; }
echo "$stats" | jq -e '.user_message_count == 0'                       > /dev/null || { echo "FAIL: user_message_count not 0"; exit 1; }
echo "$stats" | jq -e '.assistant_message_count == 0'                  > /dev/null || { echo "FAIL: assistant_message_count not 0"; exit 1; }
echo "$stats" | jq -e '.total_reasoning_chars == 0'                    > /dev/null || { echo "FAIL: total_reasoning_chars not 0"; exit 1; }
echo "$stats" | jq -e '.first_message_at == null'                      > /dev/null || { echo "FAIL: first_message_at not null"; exit 1; }
echo "$stats" | jq -e '.last_message_at == null'                       > /dev/null || { echo "FAIL: last_message_at not null"; exit 1; }

# 4) 404 path: unknown id.
not_found_status=$(curl -s -o /dev/null -w "%{http_code}" -H "$AUTH" \
  "$STAGING_URL/api/sessions/stats?id=999999999")
[ "$not_found_status" = "404" ] || { echo "FAIL: expected 404 for unknown id, got $not_found_status"; exit 1; }

# 5) 400 path: missing id.
bad_req_status=$(curl -s -o /dev/null -w "%{http_code}" -H "$AUTH" \
  "$STAGING_URL/api/sessions/stats")
[ "$bad_req_status" = "400" ] || { echo "FAIL: expected 400 for missing id, got $bad_req_status"; exit 1; }

# 6) Cleanup — delete the verifier session so we don't accumulate junk on staging.
curl -sf -X DELETE -H "$AUTH" "$STAGING_URL/api/sessions?id=$session_id" > /dev/null || true

echo "OK"
```

On current main this fails at step 2 (the endpoint returns 404 because the route is not registered yet; `curl -sf` exits non-zero). On the new binary it must pass all six steps.

### Post-acceptance (must PASS after Pi's implementation)

**Same script as above.** This is by design — one verifier, two contexts. Pi's gate-runner runs it twice with different binaries; we don't author two scripts that could drift from each other. After Pi's implementation lands and the new binary is deployed to staging, all six steps must complete and the script exits 0.

### Human-eyeball notes (NOT part of the gate)

- After deploy, manually `curl https://study.claw-study.xyz/api/sessions/stats?id=<real-session>` with the prod token and confirm a real session with chat history returns plausible counters (non-zero, monotonically related to what you see in the UI). Sanity check only; redundant with the gate.

## Done criteria

- [ ] `handler/sessions_test.go` includes the four new test functions and they pass locally
- [ ] `agent/types.go` declares `SessionStats`
- [ ] `agent/db.go` declares `GetSessionStats` and wraps `sql.ErrNoRows` for missing sessions
- [ ] Route `/api/sessions/stats` registered in `handler/handler.go:Register`
- [ ] All existing tests still pass (`go test ./...`)
- [ ] Diff stays under 200 lines (per frontmatter cap)
- [ ] After deploy, the canonical verifier passes against prod

## Rollback notes

No data migration. Binary swap + `git revert` is sufficient — only additions: one new type, one new DB method, one new handler method, one new mux registration, four new tests.
