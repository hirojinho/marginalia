# Observability & User Metrics

**Date:** 2026-05-12  
**Status:** Approved

## Goal

Make per-user behavior observable to drive UX improvements: which courses are active, which tools are useful, where users get stuck, chat latency trends.

## Events Table

Single `events` table in SQLite with typed sparse columns. One row per event regardless of kind.

```sql
CREATE TABLE IF NOT EXISTS events (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    kind          TEXT    NOT NULL,
    session_id    INTEGER,
    course_id     TEXT,
    tool_name     TEXT,
    model         TEXT,
    input_tokens  INTEGER,
    output_tokens INTEGER,
    duration_ms   INTEGER,
    ok            INTEGER,   -- 1=true, 0=false, NULL=n/a
    created_at    INTEGER NOT NULL  -- unix milliseconds
);
CREATE INDEX IF NOT EXISTS events_created ON events(created_at);
CREATE INDEX IF NOT EXISTS events_kind_created ON events(kind, created_at);
```

Added via the existing idempotent migrations slice in `InitSchema`.

## Event Kinds

| Kind | Emitted from | Columns used |
|---|---|---|
| `chat_turn` | `handleChatV2` after `streamPiTurn` returns | session_id, course_id, model, input_tokens, output_tokens, duration_ms |
| `tool_use` | `streamPiTurn` on each `tool_end` PiEvent | session_id, tool_name, duration_ms, ok |
| `plan_toggle` | `handlePlanToggle` after successful save | course_id, ok (new done value as 1/0) |
| `pdf_open` | `handlePDFUpload` after `SetLastOpenedPDF` | session_id (active session), course_id (pdf's course_id) |
| `session_create` | `handleSessions` POST after successful create | session_id, course_id |

**`chat_turn` details:**
- `duration_ms` = wall time from start of `handleChatV2` to `streamPiTurn` return
- `input_tokens` / `output_tokens` from the `done` PiEvent's `Usage` field (already in `sseDonePayload`)
- `model` from `h.App.Config.AgentModel` (or `Config.Model` fallback)
- `course_id` from the session's course

**`tool_use` details:**
- `streamPiTurn` already receives `tool_end` events with `ToolName` and `OK`
- `duration_ms` is not available per-tool from Pi events — leave NULL

## DB API (`agent/db.go`)

```go
type Event struct {
    ID           int64
    Kind         string
    SessionID    *int64
    CourseID     string
    ToolName     string
    Model        string
    InputTokens  int
    OutputTokens int
    DurationMs   int64
    OK           *bool
    CreatedAt    int64 // unix ms
}

func (a *App) RecordEvent(e Event) error
func (a *App) PruneOldEvents(before time.Time) (int64, error)
func (a *App) QueryEventSummary(since time.Time) (EventSummary, error)
func (a *App) ListRecentEvents(limit int) ([]Event, error)
```

`RecordEvent` is called with `go a.RecordEvent(e)` in hot paths (chat turn, tool use) so it never blocks the HTTP response. Errors are logged but not surfaced.

`EventSummary` carries:
- Turn count, avg latency ms, p95 latency ms (chat_turns)
- Tool call counts map[string]int (top tools)
- Course activity map[string]int (session_creates by course)
- Plan toggle counts: done_count, undone_count
- PDF open count

## Retention Sweep

On app start and every 24h via `time.Ticker`: call `PruneOldEvents(time.Now().Add(-90 * 24 * time.Hour))`. Log rows deleted at INFO. Best-effort — errors logged, not fatal. Lives in `main.go`.

## `/debug/metrics` Endpoint

- Route: `GET /debug/metrics`
- Auth: same bearer token middleware as all other endpoints
- Response: server-rendered HTML (Go template inline in handler)
- No JS required — debug page only, not part of SPA

**Page layout:**

```
/debug/metrics

── Summary ──────────────────────────────────────
  [7d]  [30d]  [90d]   (window selector — form GET param)

  Chat turns: 42  |  Avg latency: 3.2s  |  p95: 8.1s
  Token usage: 18,400 in / 9,200 out

  Top tools:
    read_file     31
    rag_search    18
    ...

  Active courses:
    ce297    12 sessions
    ddia      7 sessions

  Plan toggles: 24 done / 3 undone
  PDF opens: 9

── Recent events (last 200) ─────────────────────
  [table: created_at | kind | session | course | tool | dur | ok]
```

Window selection via `?window=7d|30d|90d` query param (default 30d).

## Files Touched

| File | Change |
|---|---|
| `agent/db.go` | `Event` struct, 4 DB methods, schema + migration |
| `agent/db_test.go` | Tests for RecordEvent, PruneOldEvents, QueryEventSummary |
| `handler/metrics.go` | New file: `/debug/metrics` handler + HTML template |
| `handler/handler.go` | Register `/debug/metrics` route |
| `handler/chat_v2.go` | Emit `chat_turn` + `tool_use` events |
| `handler/plan.go` | Emit `plan_toggle` event |
| `handler/pdf.go` | Emit `pdf_open` event |
| `handler/sessions.go` | Emit `session_create` event |
| `main.go` | Start retention sweep goroutine |

Estimated ~200–250 lines of new code.

## Out of Scope

- Per-user breakdown (single-user app)
- Backfilling historical data
- Alerting or anomaly detection
- Exporting events (CSV, etc.)
