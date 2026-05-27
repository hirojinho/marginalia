---
id: 2026-05-26-r4-confidence-log
title: R4 — confidence_log table + log_confidence tool + claw-cli confidence subcommand
max_wall_clock_minutes: 60
max_diff_lines: 350
max_retries: 1
max_tokens: 200000
requires_visual_approval: false
allow_web_search: false
created_at: 2026-05-26
created_by: laptop-cc + eduardo
---

## Goal

R4 from the ROADMAP "Pedagogy backlog" — the substrate that R1 (retrieval queue) will consume. Persist the user's Rule-3 confidence answers ("How confident are you?") to a `confidence_log` table keyed by **Knowledge Component**. In claw-study the KC is the plan-task: 2026-05-25's `stable-task-ids` ticket gave every task a UUID specifically so this could anchor against it.

Literature anchor: Bayesian Knowledge Tracing (Corbett & Anderson 1995, *User Modeling and User-Adapted Interaction*; Williams CS — *What is BKT?*) treats student knowledge as a per-KC latent variable; the confidence trajectory is the observable noisy signal. Without stable KC identity over time, BKT can't update its posterior — that's why this couldn't ship before stable-task-ids.

**Out of scope for R4** (deferred to R-Later / R1):
- HTTP endpoint for the trajectory — pure CLI substrate for now
- Frontend dashboard (R-Later per ROADMAP)
- Spaced-repetition scheduling on top of the data (that's R1+R2)
- Retroactive backfill of confidence from chat history (not a v1 concern)

## References

- `agent/db.go:79-130` — schema-init block (pdfs, sessions, messages, agent_memory, events). Add `confidence_log` here.
- `agent/db.go:104-149` — ALTER-based migration pattern (`messages.reasoning` was added that way). New tables use `CREATE TABLE IF NOT EXISTS` — same idempotency.
- `agent/tools.go:93-104` — `update_plan` tool registration. Pattern for adding `log_confidence`.
- `agent/tools.go:182` — tool dispatch switch. Add `case "log_confidence": return a.ToolLogConfidence(args)`.
- `agent/sandbox.go:166` — Rule 3 system-prompt text. Will be amended to instruct the agent to call `log_confidence` after eliciting the answer.
- `claw-cli/main.go:143-160` — top-level subcommand switch (memory, rag, plan, course, note, pdf, web, skill, session). Add `case "confidence":`.
- `claw-cli/main.go:384-388` — `plan show/status/toggle` is the closest existing pattern for a "list / fetch" subcommand.
- `agent/types.go` — add `ConfidencePoint` and a new `ConfidenceSource` enum-as-string. New types live next to existing ones (`Session`, `Plan`, `Task`).
- `2026-05-25-stable-task-ids` (shipped commit `1006975`) — the UUIDs `kc_id` references live on `Task.ID` in `data/plans/<plan>.json`.

## Implementation plan

1. **Add the table** in `agent/db.go` schema-init block:
   ```sql
   CREATE TABLE IF NOT EXISTS confidence_log (
       id          INTEGER PRIMARY KEY AUTOINCREMENT,
       session_id  INTEGER REFERENCES sessions(id) ON DELETE CASCADE,
       kc_id       TEXT    NOT NULL,
       value       REAL    NOT NULL CHECK (value >= 0.0 AND value <= 1.0),
       source      TEXT    NOT NULL CHECK (source IN ('tool_call','inferred','manual','verifier')),
       created_at  INTEGER NOT NULL,
       raw_text    TEXT
   );
   CREATE INDEX IF NOT EXISTS idx_confidence_log_kc ON confidence_log(kc_id, created_at);
   CREATE INDEX IF NOT EXISTS idx_confidence_log_session ON confidence_log(session_id, created_at);
   ```
   `kc_id` is `TEXT` not an FK because tasks live in plan JSON files, not SQLite. The join happens at read time in claw-cli/handler code.

2. **Add `ConfidencePoint` type** in `agent/types.go`:
   ```go
   type ConfidencePoint struct {
       ID         int64  `json:"id"`
       SessionID  int64  `json:"session_id"`
       KCID       string `json:"kc_id"`
       Value      float64 `json:"value"`
       Source     string `json:"source"`
       CreatedAt  int64  `json:"created_at"`
       RawText    string `json:"raw_text,omitempty"`
   }
   ```

3. **Add DB methods** in `agent/db.go` (after the existing GetSessionStats block from yesterday):
   - `func (a *App) LogConfidence(sessionID int64, kcID string, value float64, source, rawText string) (int64, error)` — INSERT, returns new row id. Validate value in [0.0, 1.0] before INSERT; validate source in the allowed set.
   - `func (a *App) GetConfidenceTrajectory(kcID string, limit int) ([]ConfidencePoint, error)` — `SELECT ... FROM confidence_log WHERE kc_id = ? ORDER BY created_at DESC LIMIT ?`
   - `func (a *App) GetRecentConfidence(sinceMs int64, limit int) ([]ConfidencePoint, error)` — `SELECT ... WHERE created_at >= ? ORDER BY created_at DESC LIMIT ?`

4. **Add the `log_confidence` agent tool** in `agent/tools.go`:
   ```go
   {Type: "function", Function: ToolFunc{
       Name:        "log_confidence",
       Description: "Log the user's confidence value (0.0-1.0) for the current plan task after they answer Rule 3 ('how confident are you'). Pass kc_id = the active plan task's id field. raw is the user's verbatim reply for audit.",
       Parameters: map[string]interface{}{
           "type": "object",
           "properties": map[string]interface{}{
               "kc_id": map[string]interface{}{"type": "string", "description": "Plan task UUID (from the active plan's task.id field)"},
               "value": map[string]interface{}{"type": "number", "description": "Confidence in [0.0, 1.0]. Convert verbal answers (e.g. '3 out of 5' → 0.6)."},
               "raw":   map[string]interface{}{"type": "string", "description": "User's verbatim reply, for audit/debug"},
           },
           "required": []string{"kc_id", "value"},
       },
   }},
   ```
   Add `case "log_confidence": return a.ToolLogConfidence(args)` to the dispatch switch (line 182 area).

5. **Implement `ToolLogConfidence`** in a new file `agent/tools_confidence.go` (pattern: `agent/tools_plan.go`). Takes `args json.RawMessage`, decodes to a struct, calls `a.LogConfidence(...)` with `source="tool_call"` and the current session ID (the App keeps an `ActiveSessionID` accessible — grep for how `update_plan` knows context; if no session context is plumbed, use 0 and add a TODO). On success, return `"logged confidence X for kc Y"`. On error, return `"error: ..."`.

6. **Amend Rule 3** in `agent/sandbox.go:166`:
   - **OLD:** `"3. **ALWAYS ask \"How confident are you with this?\"** before moving to a new topic..."`
   - **NEW:** `"3. **ALWAYS ask \"How confident are you with this?\"** before moving to a new topic. After the user replies, parse a value in [0.0, 1.0] from their answer and call the log_confidence tool with kc_id = the active task's id field from the plan, value = your parsed value, and raw = their verbatim reply. If no active task is in context, skip the tool call (prompt-only behavior)."`

7. **Add `claw-cli confidence` subcommand** in `claw-cli/main.go` (new switch case at line ~152, alongside `course`):
   - `claw-cli confidence trajectory <kc_id> [--limit N]` — calls `app.GetConfidenceTrajectory(kcID, limit)`, prints `created_at\tvalue\tsource\traw_text` (TSV). Default limit 50.
   - `claw-cli confidence recent [--since 7d] [--limit N]` — calls `app.GetRecentConfidence(sinceMs, limit)`. Parse `7d` / `24h` / `30m` durations via `time.ParseDuration` (which doesn't support `d`; hand-roll the d→hour suffix; reject anything else).
   - `claw-cli confidence schema` — runs a `SELECT count(*) FROM confidence_log` query; prints `OK` on success, error otherwise. **This is the verifier surface** — see Verification recipe.

   Mirror the existing dispatch style at `claw-cli/main.go:147-159`.

8. **Tests** — add unit tests in `agent/db_test.go` (existence) and a new `agent/tools_confidence_test.go`:
   - `TestLogConfidence_ValidRow` — insert + readback via GetConfidenceTrajectory
   - `TestLogConfidence_RejectsOutOfRange` — value=-0.1 and value=1.5 both return error
   - `TestLogConfidence_RejectsInvalidSource` — DB rejects via CHECK constraint
   - `TestGetConfidenceTrajectory_OrderingAndLimit` — insert 5, limit 3, assert correct 3 rows in descending created_at order
   - `TestToolLogConfidence_DispatchedRoundTrip` — call `a.ExecuteTool("log_confidence", json)` and confirm a row landed

9. **Run `go test ./...` locally before declaring done.** All existing tests must pass.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
set -euo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

# The CLI is invoked on the staging app's binary via the staging-up.sh
# helper's CLAW_CLI_PATH env (see staging-up.sh). The gate-runner is
# expected to expose it as $STAGING_CLI; if missing, fall back to
# $HOME/stack/study-app/bin/claw-cli (prod CLI against staging DB env).
CLI="${STAGING_CLI:-$HOME/stack/study-app/bin/claw-cli}"

# Run the new schema-check subcommand. On current main the `confidence`
# subcommand does not exist → claw-cli prints "unknown subcommand" and
# exits non-zero. After implementation, the new schema check exits 0.
if "$CLI" confidence schema 2>&1; then
  echo "OK: confidence schema check passed"
  exit 0
else
  echo "FAIL: confidence schema check failed (table or subcommand missing)"
  exit 1
fi
```

### Post-acceptance (must PASS after Pi's implementation)

**Same script as above.** Pre-baseline expects exit 1 (subcommand absent on current main); post-acceptance expects exit 0 (subcommand exists and the table is present after migration ran on staging start-up).

### Human-eyeball notes (NOT part of the gate)

- After deploy, open a chat session on the CE-297 course, discuss a topic, answer Rule 3's confidence question, then on the laptop: `ssh nanoclaw 'sqlite3 ~/stack/study-app/data/study.db "SELECT * FROM confidence_log ORDER BY id DESC LIMIT 3"'`. Should show your answer with `source='tool_call'`.
- Also test `claw-cli confidence trajectory <task-id>` against a known task UUID from `data/plans/ce297.json`.

## Done criteria

- [ ] `confidence_log` table exists in SQLite with the expected columns + CHECK constraints + two indexes
- [ ] `App.LogConfidence`, `App.GetConfidenceTrajectory`, `App.GetRecentConfidence` implemented
- [ ] `log_confidence` agent tool registered and dispatched
- [ ] `agent/sandbox.go:166` Rule 3 updated to instruct the tool call
- [ ] `claw-cli confidence trajectory|recent|schema` subcommand works
- [ ] All five unit tests pass
- [ ] `go test ./...` all green
- [ ] Gate verifier exits 0 on the new binary (subcommand exists + schema query succeeds)
- [ ] Diff stays under 350 lines

## Rollback notes

The `confidence_log` table will be created in SQLite on first start after this ticket lands. A `git revert` removes the code that reads/writes the table, but the table itself persists (harmless — unused). No data corruption. Any rows logged via the tool after this ticket ships **become orphaned data** on rollback but are not lost — a future re-roll-forward will read them.

No FK destructive risk: `kc_id TEXT` is loose-coupled to plan-task UUIDs in JSON files; even if the task is deleted from the plan, the row stays (the audit trail is intentional).
