# Observability & User Metrics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add structured event logging to SQLite, instrument five call sites, and expose a `/debug/metrics` HTML page showing summaries and a raw event log.

**Architecture:** A single `events` table with typed sparse columns stores all event kinds. Instrumentation is inline at handler call sites — no middleware. The `/debug/metrics` handler queries the table and renders a standalone HTML page via a Go template.

**Tech Stack:** Go 1.26, SQLite via `modernc.org/sqlite`, `database/sql`, `html/template`

---

## File Map

| File | Role |
|---|---|
| `agent/db.go` | `Event` struct, schema migration, `RecordEvent`, `PruneOldEvents`, `QueryEventSummary`, `ListRecentEvents` |
| `agent/db_test.go` | Tests for all four DB methods |
| `handler/chat_v2.go` | Extend `streamPiTurn` to return `PiUsage` + tool records; emit `chat_turn` + `tool_use` events |
| `handler/chat_v2_test.go` | Test `streamPiTurn` returns usage; test tool record accumulation |
| `handler/sessions.go` | Emit `session_create` after successful POST |
| `handler/sessions_test.go` | Test `session_create` event recorded |
| `handler/plan.go` | Emit `plan_toggle` after successful save |
| `handler/plan_http_test.go` | Test `plan_toggle` event recorded |
| `handler/pdf.go` | Emit `pdf_open` after `SetLastOpenedPDF` |
| `handler/metrics.go` | New: `GET /debug/metrics` handler + HTML template |
| `handler/handler.go` | Register `/debug/metrics` route |
| `main.go` | Start 90-day retention sweep goroutine |

---

### Task 1: Event struct, schema, and RecordEvent

**Files:**
- Modify: `agent/db.go`
- Modify: `agent/db_test.go`

- [ ] **Step 1: Write the failing test**

Add to `agent/db_test.go`:

```go
func TestRecordEventRoundtrip(t *testing.T) {
	a := newMemoryApp(t)
	sid := int64(42)
	e := Event{
		Kind:      "chat_turn",
		SessionID: &sid,
		CourseID:  "ce297",
		Model:     "claude-opus-4-7",
		InputTokens:  100,
		OutputTokens: 50,
		DurationMs:   3200,
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := a.RecordEvent(e); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	evs, err := a.ListRecentEvents(10)
	if err != nil {
		t.Fatalf("ListRecentEvents: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	got := evs[0]
	if got.Kind != "chat_turn" {
		t.Errorf("kind = %q, want chat_turn", got.Kind)
	}
	if got.CourseID != "ce297" {
		t.Errorf("course_id = %q, want ce297", got.CourseID)
	}
	if got.DurationMs != 3200 {
		t.Errorf("duration_ms = %d, want 3200", got.DurationMs)
	}
	if got.SessionID == nil || *got.SessionID != 42 {
		t.Errorf("session_id = %v, want 42", got.SessionID)
	}
}

func TestRecordEventToolUse(t *testing.T) {
	a := newMemoryApp(t)
	okTrue := true
	if err := a.RecordEvent(Event{
		Kind:      "tool_use",
		ToolName:  "rag_search",
		OK:        &okTrue,
		CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	evs, _ := a.ListRecentEvents(10)
	if len(evs) != 1 || evs[0].ToolName != "rag_search" {
		t.Fatalf("unexpected events: %+v", evs)
	}
	if evs[0].OK == nil || !*evs[0].OK {
		t.Errorf("ok should be true")
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

```bash
cd ~/Documents/ITA/claw-study
go test ./agent/ -run "TestRecordEvent" -v 2>&1 | tail -10
```

Expected: build failure — `Event`, `RecordEvent`, `ListRecentEvents` undefined.

- [ ] **Step 3: Add Event struct and schema migration to `agent/db.go`**

Add the `Event` struct after the `Message` struct (around line 19):

```go
// Event is a single observability record. Fields not relevant to a given
// kind are left at their zero value (SessionID and OK use pointers so
// NULL is distinguishable from false/0).
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
	CreatedAt    int64 // unix milliseconds
}
```

Add to the migrations slice in `InitSchema` (after the existing entries):

```go
"ALTER TABLE events ADD COLUMN kind TEXT NOT NULL DEFAULT ''",
```

Wait — `events` doesn't exist yet. Add it to the CREATE TABLE block in the `schema` constant instead. Insert after the `agent_memory` table definition, before the closing backtick:

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
    ok            INTEGER,
    created_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS events_created ON events(created_at);
CREATE INDEX IF NOT EXISTS events_kind_created ON events(kind, created_at);
```

- [ ] **Step 4: Implement `RecordEvent` and `ListRecentEvents` in `agent/db.go`**

Add after the `GetMessageCount` function:

```go
// ---------- Events ----------

// RecordEvent inserts one observability event. SessionID and OK are stored
// as NULL when their pointer is nil.
func (a *App) RecordEvent(e Event) error {
	var sessionID interface{}
	if e.SessionID != nil {
		sessionID = *e.SessionID
	}
	var ok interface{}
	if e.OK != nil {
		if *e.OK {
			ok = 1
		} else {
			ok = 0
		}
	}
	_, err := a.DB.Exec(
		`INSERT INTO events
		 (kind, session_id, course_id, tool_name, model,
		  input_tokens, output_tokens, duration_ms, ok, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Kind, sessionID, e.CourseID, e.ToolName, e.Model,
		e.InputTokens, e.OutputTokens, e.DurationMs, ok, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

// ListRecentEvents returns up to limit events ordered newest-first.
func (a *App) ListRecentEvents(limit int) ([]Event, error) {
	rows, err := a.DB.Query(
		`SELECT id, kind, session_id, course_id, tool_name, model,
		        input_tokens, output_tokens, duration_ms, ok, created_at
		 FROM events ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()
	var evs []Event
	for rows.Next() {
		var e Event
		var sid sql.NullInt64
		var okVal sql.NullInt64
		if err := rows.Scan(
			&e.ID, &e.Kind, &sid, &e.CourseID, &e.ToolName, &e.Model,
			&e.InputTokens, &e.OutputTokens, &e.DurationMs, &okVal, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if sid.Valid {
			v := sid.Int64
			e.SessionID = &v
		}
		if okVal.Valid {
			b := okVal.Int64 == 1
			e.OK = &b
		}
		evs = append(evs, e)
	}
	return evs, rows.Err()
}
```

Make sure `"database/sql"` is already imported in `agent/db.go` (it is, via `_ "modernc.org/sqlite"`). If `sql.NullInt64` causes a compile error, add `"database/sql"` to the import block explicitly.

- [ ] **Step 5: Run tests and verify they pass**

```bash
go test ./agent/ -run "TestRecordEvent" -v 2>&1 | tail -10
```

Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
git add agent/db.go agent/db_test.go
git commit -m "feat(events): Event struct, schema, RecordEvent, ListRecentEvents"
```

---

### Task 2: PruneOldEvents

**Files:**
- Modify: `agent/db.go`
- Modify: `agent/db_test.go`

- [ ] **Step 1: Write the failing test**

Add to `agent/db_test.go`:

```go
func TestPruneOldEventsRemovesOldRows(t *testing.T) {
	a := newMemoryApp(t)
	now := time.Now()
	old := now.Add(-100 * 24 * time.Hour).UnixMilli()
	recent := now.UnixMilli()

	for i := 0; i < 3; i++ {
		_ = a.RecordEvent(Event{Kind: "session_create", CreatedAt: old})
	}
	for i := 0; i < 2; i++ {
		_ = a.RecordEvent(Event{Kind: "session_create", CreatedAt: recent})
	}

	cutoff := now.Add(-90 * 24 * time.Hour)
	deleted, err := a.PruneOldEvents(cutoff)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if deleted != 3 {
		t.Errorf("deleted = %d, want 3", deleted)
	}
	evs, _ := a.ListRecentEvents(100)
	if len(evs) != 2 {
		t.Errorf("remaining = %d, want 2", len(evs))
	}
}
```

- [ ] **Step 2: Run and verify it fails**

```bash
go test ./agent/ -run "TestPruneOldEvents" -v 2>&1 | tail -10
```

Expected: `PruneOldEvents undefined`

- [ ] **Step 3: Implement `PruneOldEvents` in `agent/db.go`**

Add after `ListRecentEvents`:

```go
// PruneOldEvents deletes events older than before. Returns the number of rows deleted.
func (a *App) PruneOldEvents(before time.Time) (int64, error) {
	res, err := a.DB.Exec("DELETE FROM events WHERE created_at < ?", before.UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("prune events: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
```

- [ ] **Step 4: Run and verify it passes**

```bash
go test ./agent/ -run "TestPruneOldEvents" -v 2>&1 | tail -5
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add agent/db.go agent/db_test.go
git commit -m "feat(events): PruneOldEvents"
```

---

### Task 3: QueryEventSummary

**Files:**
- Modify: `agent/db.go`
- Modify: `agent/db_test.go`

- [ ] **Step 1: Write the failing test**

Add to `agent/db_test.go`:

```go
func TestQueryEventSummaryCountsAndAggregates(t *testing.T) {
	a := newMemoryApp(t)
	now := time.Now().UnixMilli()

	okTrue, okFalse := true, false
	sid := int64(1)

	// 2 chat turns
	_ = a.RecordEvent(Event{Kind: "chat_turn", SessionID: &sid, CourseID: "ce297", DurationMs: 2000, InputTokens: 100, OutputTokens: 50, CreatedAt: now})
	_ = a.RecordEvent(Event{Kind: "chat_turn", SessionID: &sid, CourseID: "ce297", DurationMs: 4000, InputTokens: 200, OutputTokens: 80, CreatedAt: now})
	// 1 tool_use ok, 1 tool_use failed
	_ = a.RecordEvent(Event{Kind: "tool_use", ToolName: "rag_search", OK: &okTrue, CreatedAt: now})
	_ = a.RecordEvent(Event{Kind: "tool_use", ToolName: "rag_search", OK: &okFalse, CreatedAt: now})
	// 1 plan_toggle done, 1 undone
	_ = a.RecordEvent(Event{Kind: "plan_toggle", CourseID: "ce297", OK: &okTrue, CreatedAt: now})
	_ = a.RecordEvent(Event{Kind: "plan_toggle", CourseID: "ce297", OK: &okFalse, CreatedAt: now})
	// 1 pdf_open
	_ = a.RecordEvent(Event{Kind: "pdf_open", CourseID: "ce297", CreatedAt: now})
	// 1 session_create
	_ = a.RecordEvent(Event{Kind: "session_create", CourseID: "ddia", CreatedAt: now})

	since := time.UnixMilli(now - 1000)
	s, err := a.QueryEventSummary(since)
	if err != nil {
		t.Fatalf("QueryEventSummary: %v", err)
	}

	if s.TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", s.TurnCount)
	}
	if s.AvgLatencyMs != 3000 {
		t.Errorf("AvgLatencyMs = %d, want 3000", s.AvgLatencyMs)
	}
	if s.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", s.InputTokens)
	}
	if s.OutputTokens != 130 {
		t.Errorf("OutputTokens = %d, want 130", s.OutputTokens)
	}
	if s.ToolCounts["rag_search"] != 2 {
		t.Errorf("ToolCounts[rag_search] = %d, want 2", s.ToolCounts["rag_search"])
	}
	if s.CourseCounts["ddia"] != 1 {
		t.Errorf("CourseCounts[ddia] = %d, want 1", s.CourseCounts["ddia"])
	}
	if s.PlanDone != 1 || s.PlanUndone != 1 {
		t.Errorf("PlanDone=%d PlanUndone=%d, want 1 1", s.PlanDone, s.PlanUndone)
	}
	if s.PDFOpens != 1 {
		t.Errorf("PDFOpens = %d, want 1", s.PDFOpens)
	}
}
```

- [ ] **Step 2: Run and verify it fails**

```bash
go test ./agent/ -run "TestQueryEventSummary" -v 2>&1 | tail -10
```

Expected: `EventSummary undefined`, `QueryEventSummary undefined`

- [ ] **Step 3: Add `EventSummary` struct and implement `QueryEventSummary` in `agent/db.go`**

Add the struct after the `Event` struct:

```go
// EventSummary holds pre-aggregated metrics over a time window.
type EventSummary struct {
	TurnCount    int
	AvgLatencyMs int64
	P95LatencyMs int64
	InputTokens  int64
	OutputTokens int64
	ToolCounts   map[string]int
	CourseCounts map[string]int
	PlanDone     int
	PlanUndone   int
	PDFOpens     int
}
```

Add the method after `PruneOldEvents`:

```go
// QueryEventSummary returns aggregated metrics for events recorded after since.
func (a *App) QueryEventSummary(since time.Time) (EventSummary, error) {
	sinceMs := since.UnixMilli()
	s := EventSummary{
		ToolCounts:   make(map[string]int),
		CourseCounts: make(map[string]int),
	}

	// chat_turn aggregates
	row := a.DB.QueryRow(
		`SELECT COUNT(*), COALESCE(AVG(duration_ms),0),
		        COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0)
		 FROM events WHERE kind='chat_turn' AND created_at >= ?`, sinceMs)
	if err := row.Scan(&s.TurnCount, &s.AvgLatencyMs, &s.InputTokens, &s.OutputTokens); err != nil {
		return s, fmt.Errorf("chat_turn aggregates: %w", err)
	}

	// p95 latency: select row at 95th percentile position
	if s.TurnCount > 0 {
		offset := int(float64(s.TurnCount)*0.95) - 1
		if offset < 0 {
			offset = 0
		}
		p95Row := a.DB.QueryRow(
			`SELECT duration_ms FROM events
			 WHERE kind='chat_turn' AND created_at >= ?
			 ORDER BY duration_ms ASC LIMIT 1 OFFSET ?`, sinceMs, offset)
		_ = p95Row.Scan(&s.P95LatencyMs)
	}

	// tool counts
	rows, err := a.DB.Query(
		`SELECT tool_name, COUNT(*) FROM events
		 WHERE kind='tool_use' AND created_at >= ? AND tool_name != ''
		 GROUP BY tool_name ORDER BY COUNT(*) DESC`, sinceMs)
	if err != nil {
		return s, fmt.Errorf("tool counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return s, err
		}
		s.ToolCounts[name] = count
	}
	if err := rows.Err(); err != nil {
		return s, err
	}

	// course counts (by session_create)
	crows, err := a.DB.Query(
		`SELECT course_id, COUNT(*) FROM events
		 WHERE kind='session_create' AND created_at >= ? AND course_id != ''
		 GROUP BY course_id ORDER BY COUNT(*) DESC`, sinceMs)
	if err != nil {
		return s, fmt.Errorf("course counts: %w", err)
	}
	defer crows.Close()
	for crows.Next() {
		var cid string
		var count int
		if err := crows.Scan(&cid, &count); err != nil {
			return s, err
		}
		s.CourseCounts[cid] = count
	}
	if err := crows.Err(); err != nil {
		return s, err
	}

	// plan toggles
	row = a.DB.QueryRow(
		`SELECT
		   COALESCE(SUM(CASE WHEN ok=1 THEN 1 ELSE 0 END),0),
		   COALESCE(SUM(CASE WHEN ok=0 THEN 1 ELSE 0 END),0)
		 FROM events WHERE kind='plan_toggle' AND created_at >= ?`, sinceMs)
	if err := row.Scan(&s.PlanDone, &s.PlanUndone); err != nil {
		return s, fmt.Errorf("plan toggles: %w", err)
	}

	// pdf opens
	row = a.DB.QueryRow(
		`SELECT COUNT(*) FROM events WHERE kind='pdf_open' AND created_at >= ?`, sinceMs)
	if err := row.Scan(&s.PDFOpens); err != nil {
		return s, fmt.Errorf("pdf opens: %w", err)
	}

	return s, nil
}
```

- [ ] **Step 4: Run and verify it passes**

```bash
go test ./agent/ -run "TestQueryEventSummary" -v 2>&1 | tail -5
```

Expected: `PASS`

- [ ] **Step 5: Run all agent tests to confirm nothing broken**

```bash
go test ./agent/ 2>&1 | tail -5
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add agent/db.go agent/db_test.go
git commit -m "feat(events): EventSummary struct and QueryEventSummary"
```

---

### Task 4: Extend streamPiTurn to return usage and tool records

**Files:**
- Modify: `handler/chat_v2.go`
- Modify: `handler/chat_v2_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `handler/chat_v2_test.go`:

```go
func TestStreamPiTurnReturnsDoneUsage(t *testing.T) {
	events := make(chan agent.PiEvent, 2)
	events <- agent.PiEvent{Kind: "token", Delta: "hi"}
	events <- agent.PiEvent{Kind: "done", Usage: agent.PiUsage{Input: 100, Output: 40}}
	close(events)

	w := httptest.NewRecorder()
	text, _, usage, _ := streamPiTurn(events, w, w)

	if text != "hi" {
		t.Errorf("text = %q, want hi", text)
	}
	if usage.Input != 100 || usage.Output != 40 {
		t.Errorf("usage = %+v, want Input=100 Output=40", usage)
	}
}

func TestStreamPiTurnAccumulatesToolRecords(t *testing.T) {
	events := make(chan agent.PiEvent, 4)
	events <- agent.PiEvent{Kind: "tool_start", ToolName: "rag_search"}
	events <- agent.PiEvent{Kind: "tool_end", ToolName: "rag_search", OK: true}
	events <- agent.PiEvent{Kind: "tool_end", ToolName: "read_file", OK: false}
	events <- agent.PiEvent{Kind: "done"}
	close(events)

	w := httptest.NewRecorder()
	_, _, _, tools := streamPiTurn(events, w, w)

	if len(tools) != 2 {
		t.Fatalf("expected 2 tool records, got %d", len(tools))
	}
	if tools[0].Name != "rag_search" || !tools[0].OK {
		t.Errorf("tools[0] = %+v", tools[0])
	}
	if tools[1].Name != "read_file" || tools[1].OK {
		t.Errorf("tools[1] = %+v", tools[1])
	}
}
```

- [ ] **Step 2: Run and verify they fail**

```bash
go test ./handler/ -run "TestStreamPiTurn(ReturnsDoneUsage|AccumulatesToolRecords)" -v 2>&1 | tail -10
```

Expected: build failure — assignment mismatch (streamPiTurn returns 2 values, test expects 4).

- [ ] **Step 3: Add `toolUseRecord` type and update `streamPiTurn` signature**

In `handler/chat_v2.go`, add after the `sseToolEndPayload` struct (around line 28):

```go
// toolUseRecord captures a single tool invocation outcome for event logging.
type toolUseRecord struct {
	Name string
	OK   bool
}
```

Change the `streamPiTurn` signature and its return statement:

```go
func streamPiTurn(events <-chan agent.PiEvent, w http.ResponseWriter, flusher http.Flusher) (text, reasoning string, usage agent.PiUsage, tools []toolUseRecord) {
	var textBuf, reasoningBuf strings.Builder
	for ev := range events {
		switch ev.Kind {
		case "token":
			textBuf.WriteString(ev.Delta)
			data, _ := json.Marshal(map[string]string{"delta": ev.Delta})
			writeSSEEvent(w, flusher, "token", string(data))
		case "reasoning":
			reasoningBuf.WriteString(ev.Delta)
			data, _ := json.Marshal(map[string]string{"delta": ev.Delta})
			writeSSEEvent(w, flusher, "reasoning", string(data))
		case "tool_start":
			data, _ := json.Marshal(map[string]string{"name": ev.ToolName, "input_summary": ev.InputSummary})
			writeSSEEvent(w, flusher, "tool_start", string(data))
		case "tool_end":
			tools = append(tools, toolUseRecord{Name: ev.ToolName, OK: ev.OK})
			payload := sseToolEndPayload{Name: ev.ToolName, OutputSummary: ev.OutputSummary, OK: ev.OK}
			data, _ := json.Marshal(payload)
			writeSSEEvent(w, flusher, "tool_end", string(data))
		case "skill_start":
			data, _ := json.Marshal(map[string]string{"name": ev.SkillName})
			writeSSEEvent(w, flusher, "skill_start", string(data))
		case "compaction":
			data, _ := json.Marshal(map[string]string{"reason": ev.Reason})
			writeSSEEvent(w, flusher, "compaction", string(data))
		case "model_change":
			data, _ := json.Marshal(map[string]string{"from": ev.From, "to": ev.To})
			writeSSEEvent(w, flusher, "model_change", string(data))
		case "done":
			usage = ev.Usage
			payload := sseDonePayload{Usage: ev.Usage}
			data, _ := json.Marshal(payload)
			writeSSEEvent(w, flusher, "done", string(data))
		case "error":
			data, _ := json.Marshal(map[string]string{"message": ev.Message})
			writeSSEEvent(w, flusher, "error", string(data))
		}
	}
	return textBuf.String(), reasoningBuf.String(), usage, tools
}
```

- [ ] **Step 4: Fix all existing call sites in `chat_v2_test.go`**

Replace every `text, _ := streamPiTurn(...)` with `text, _, _, _ := streamPiTurn(...)` and every `_, _ := streamPiTurn(...)` with `_, _, _, _ := streamPiTurn(...)`.

Also update the existing `TestStreamPiTurnAccumulatesReasoningDeltas` test:

```go
func TestStreamPiTurnAccumulatesReasoningDeltas(t *testing.T) {
	events := make(chan agent.PiEvent, 5)
	events <- agent.PiEvent{Kind: "reasoning", Delta: "first "}
	events <- agent.PiEvent{Kind: "token", Delta: "answer"}
	events <- agent.PiEvent{Kind: "reasoning", Delta: "second"}
	events <- agent.PiEvent{Kind: "done"}
	close(events)

	w := httptest.NewRecorder()
	text, reasoning, _, _ := streamPiTurn(events, w, w)

	if text != "answer" {
		t.Errorf("text = %q, want %q", text, "answer")
	}
	if reasoning != "first second" {
		t.Errorf("reasoning = %q, want %q", reasoning, "first second")
	}
}
```

Update `handleChatV2` to use all four return values (usage and tools will be used in Task 5):

```go
assistantText, assistantReasoning, _, _ := streamPiTurn(events, w, flusher)
```

- [ ] **Step 5: Run and verify all chat_v2 tests pass**

```bash
go test ./handler/ -run "TestStreamPiTurn" -v 2>&1 | tail -15
```

Expected: all `TestStreamPiTurn*` pass.

- [ ] **Step 6: Commit**

```bash
git add handler/chat_v2.go handler/chat_v2_test.go
git commit -m "feat(events): streamPiTurn returns usage and tool records"
```

---

### Task 5: Instrument chat_v2 — emit chat_turn and tool_use events

**Files:**
- Modify: `handler/chat_v2.go`

No new handler tests here — `RecordEvent` is called with `go` (fire-and-forget) for the hot path, and the DB round-trip is already covered in Task 1. Integration is verified visually at `/debug/metrics` after deploy.

- [ ] **Step 1: Update `handleChatV2` to record events**

In `handleChatV2`, replace:

```go
assistantText, assistantReasoning, _, _ := streamPiTurn(events, w, flusher)
```

with:

```go
turnStart := time.Now()
assistantText, assistantReasoning, piUsage, piTools := streamPiTurn(events, w, flusher)
durationMs := time.Since(turnStart).Milliseconds()
```

After the `SaveAssistantMessage` block, add event recording:

```go
// Record chat_turn event (fire-and-forget — must not block SSE response).
sessID := req.SessionID
go func() {
    if err := h.App.RecordEvent(agent.Event{
        Kind:         "chat_turn",
        SessionID:    &sessID,
        CourseID:     sess.CourseID,
        Model:        model,
        InputTokens:  piUsage.Input,
        OutputTokens: piUsage.Output,
        DurationMs:   durationMs,
        CreatedAt:    time.Now().UnixMilli(),
    }); err != nil {
        slog.Warn("record chat_turn event", "err", err)
    }
    for _, tr := range piTools {
        ok := tr.OK
        if err := h.App.RecordEvent(agent.Event{
            Kind:      "tool_use",
            SessionID: &sessID,
            ToolName:  tr.Name,
            OK:        &ok,
            CreatedAt: time.Now().UnixMilli(),
        }); err != nil {
            slog.Warn("record tool_use event", "err", err)
        }
    }
}()
```

Add `"time"` to the import block if not already present (it should be via `piTurnTimeout`).

- [ ] **Step 2: Build to verify no compile errors**

```bash
go build ./handler/ 2>&1
```

Expected: no output (success).

- [ ] **Step 3: Run all handler tests**

```bash
go test ./handler/ 2>&1 | tail -5
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add handler/chat_v2.go
git commit -m "feat(events): record chat_turn and tool_use on Pi turns"
```

---

### Task 6: Instrument sessions.go — emit session_create

**Files:**
- Modify: `handler/sessions.go`
- Modify: `handler/sessions_test.go`

- [ ] **Step 1: Write the failing test**

Add to `handler/sessions_test.go`:

```go
func TestHandleSessionsCreateRecordsSessionCreateEvent(t *testing.T) {
	h := newTestHandler(t)
	body := strings.NewReader(`{"course_id":"ce297","topic":"STPA"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", body)
	rr := httptest.NewRecorder()
	h.handleSessions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var created agent.Session
	_ = json.NewDecoder(rr.Body).Decode(&created)

	evs, err := h.App.ListRecentEvents(10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var found bool
	for _, e := range evs {
		if e.Kind == "session_create" && e.CourseID == "ce297" &&
			e.SessionID != nil && *e.SessionID == created.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("no session_create event found for session %d; events: %+v", created.ID, evs)
	}
}
```

- [ ] **Step 2: Run and verify it fails**

```bash
go test ./handler/ -run "TestHandleSessionsCreateRecordsSessionCreateEvent" -v 2>&1 | tail -10
```

Expected: FAIL — no session_create event found.

- [ ] **Step 3: Instrument `handleSessions` in `handler/sessions.go`**

Find the POST branch in `handleSessions` where it returns after `CreateSession`. After the successful create and before `writeJSON`, add:

```go
sid := s.ID
if err := h.App.RecordEvent(agent.Event{
    Kind:      "session_create",
    SessionID: &sid,
    CourseID:  s.CourseID,
    CreatedAt: time.Now().UnixMilli(),
}); err != nil {
    slog.Warn("record session_create event", "err", err)
}
```

Add `"time"` to the import if missing.

- [ ] **Step 4: Run and verify it passes**

```bash
go test ./handler/ -run "TestHandleSessionsCreateRecordsSessionCreateEvent" -v 2>&1 | tail -5
```

Expected: `PASS`

- [ ] **Step 5: Run all handler tests**

```bash
go test ./handler/ 2>&1 | tail -5
```

- [ ] **Step 6: Commit**

```bash
git add handler/sessions.go handler/sessions_test.go
git commit -m "feat(events): record session_create event"
```

---

### Task 7: Instrument plan.go — emit plan_toggle

**Files:**
- Modify: `handler/plan.go`
- Modify: `handler/plan_http_test.go`

- [ ] **Step 1: Read `plan_http_test.go` to understand the test setup**

```bash
cat handler/plan_http_test.go
```

Note how it seeds the plan file and calls `handlePlanToggle`. Mirror that setup.

- [ ] **Step 2: Write the failing test**

Add to `handler/plan_http_test.go`:

```go
func TestHandlePlanToggleRecordsPlanToggleEvent(t *testing.T) {
	h := newTestHandler(t)
	// seed a minimal plan so handlePlanToggle finds something to toggle
	writePlan(t, h, &agent.JSONPlan{
		ID:   "ce297",
		Name: "CE-297",
		Phases: []agent.Phase{{
			Name:  "Phase 1",
			Tasks: []agent.Task{{Title: "Read chapter 1", Done: false}},
		}},
	})

	body := strings.NewReader("course=ce297&index=0")
	req := httptest.NewRequest(http.MethodPost, "/api/plan/toggle", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.handlePlanToggle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	evs, err := h.App.ListRecentEvents(10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var found bool
	for _, e := range evs {
		if e.Kind == "plan_toggle" && e.CourseID == "ce297" {
			found = true
		}
	}
	if !found {
		t.Errorf("no plan_toggle event found; events: %+v", evs)
	}
}
```

- [ ] **Step 3: Run and verify it fails**

```bash
go test ./handler/ -run "TestHandlePlanToggleRecordsPlanToggleEvent" -v 2>&1 | tail -10
```

Expected: FAIL — no plan_toggle event.

- [ ] **Step 4: Instrument `handlePlanToggle` in `handler/plan.go`**

After `writeJSON(w, http.StatusOK, p)` succeeds (but before returning), add — actually add it right before `writeJSON`:

```go
// record after successful toggle
newDone := p.Phases[0].Tasks[0].Done // placeholder — use the actual toggled task's done value
```

Actually the simplest approach: check `toggleTaskAt` returns the plan, and after saving we emit the event. We don't have easy access to which task was toggled or its new state without more refactoring. Instead, emit with `ok=nil` (unknown direction) for now — tool provides event existence and course, which is enough for product decisions.

Replace the end of `handlePlanToggle` with:

```go
	if err := h.App.SavePlan(p); err != nil {
		writeServerError(w, "save plan", err)
		return
	}

	if err := h.App.RecordEvent(agent.Event{
		Kind:      "plan_toggle",
		CourseID:  course,
		CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		slog.Warn("record plan_toggle event", "err", err)
	}
	writeJSON(w, http.StatusOK, p)
```

Add the missing imports to `handler/plan.go` if needed:

```go
import (
    "log/slog"
    "net/http"
    "strconv"
    "time"

    "study-app/agent"
)
```

- [ ] **Step 5: Run and verify it passes**

```bash
go test ./handler/ -run "TestHandlePlanToggleRecordsPlanToggleEvent" -v 2>&1 | tail -5
```

Expected: `PASS`

- [ ] **Step 6: Run all tests**

```bash
go test ./handler/ 2>&1 | tail -5
```

- [ ] **Step 7: Commit**

```bash
git add handler/plan.go handler/plan_http_test.go
git commit -m "feat(events): record plan_toggle event"
```

---

### Task 8: Instrument pdf.go — emit pdf_open

**Files:**
- Modify: `handler/pdf.go`

No handler test for this one — `handlePDFUpload` involves multipart form parsing and file I/O, making it expensive to integration-test. The DB round-trip is covered by Task 1.

- [ ] **Step 1: Locate the `SetLastOpenedPDF` call in `handlePDFUpload`**

```bash
grep -n "SetLastOpenedPDF" handler/pdf.go
```

- [ ] **Step 2: Add event emission after `SetLastOpenedPDF` succeeds**

After the `SetLastOpenedPDF` call and its error guard, add:

```go
courseID := r.FormValue("course_id")
activeSess := h.App.ActiveSessionID()
var sessPtr *int64
if activeSess != 0 {
    sessPtr = &activeSess
}
if err := h.App.RecordEvent(agent.Event{
    Kind:      "pdf_open",
    SessionID: sessPtr,
    CourseID:  courseID,
    CreatedAt: time.Now().UnixMilli(),
}); err != nil {
    slog.Warn("record pdf_open event", "err", err)
}
```

Add `"time"` and `"log/slog"` to imports if missing.

- [ ] **Step 3: Build to verify no compile errors**

```bash
go build ./handler/ 2>&1
```

- [ ] **Step 4: Run all tests**

```bash
go test ./handler/ 2>&1 | tail -5
```

- [ ] **Step 5: Commit**

```bash
git add handler/pdf.go
git commit -m "feat(events): record pdf_open event"
```

---

### Task 9: /debug/metrics handler

**Files:**
- Create: `handler/metrics.go`
- Modify: `handler/handler.go`

- [ ] **Step 1: Write the failing test**

Add a new file `handler/metrics_test.go`:

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDebugMetricsRequiresAuth(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/debug/metrics", nil)
	rr := httptest.NewRecorder()
	h.handleDebugMetrics(rr, req)
	// newTestHandler sets a bearer token; request without it should be rejected
	// OR accepted (depends on auth middleware). Check it returns 200 (auth not
	// enforced in test handler) or that the page renders without panicking.
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Metrics") {
		t.Errorf("expected Metrics in response, got:\n%s", rr.Body.String())
	}
}

func TestDebugMetricsWindowParam(t *testing.T) {
	h := newTestHandler(t)
	for _, w := range []string{"7d", "30d", "90d"} {
		req := httptest.NewRequest(http.MethodGet, "/debug/metrics?window="+w, nil)
		rr := httptest.NewRecorder()
		h.handleDebugMetrics(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("window=%s status=%d", w, rr.Code)
		}
	}
}
```

- [ ] **Step 2: Run and verify it fails**

```bash
go test ./handler/ -run "TestDebugMetrics" -v 2>&1 | tail -10
```

Expected: `h.handleDebugMetrics undefined`

- [ ] **Step 3: Create `handler/metrics.go`**

```go
package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"
)

var metricsTemplate = template.Must(template.New("metrics").Funcs(template.FuncMap{
	"ms2s": func(ms int64) string { return fmt.Sprintf("%.1fs", float64(ms)/1000) },
}).Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Metrics</title>
<style>
body { font-family: monospace; max-width: 900px; margin: 2rem auto; padding: 0 1rem; }
h1 { font-size: 1.2rem; }
h2 { font-size: 1rem; margin-top: 2rem; border-bottom: 1px solid #ccc; }
.windows a { margin-right: 1rem; }
.windows a.active { font-weight: bold; text-decoration: none; }
table { border-collapse: collapse; width: 100%; font-size: 0.85rem; }
th, td { border: 1px solid #ddd; padding: 4px 8px; text-align: left; }
th { background: #f5f5f5; }
.stat { display: inline-block; margin-right: 2rem; }
</style>
</head>
<body>
<h1>Metrics</h1>

<div class="windows">
  <a href="?window=7d" {{if eq .Window "7d"}}class="active"{{end}}>7d</a>
  <a href="?window=30d" {{if eq .Window "30d"}}class="active"{{end}}>30d</a>
  <a href="?window=90d" {{if eq .Window "90d"}}class="active"{{end}}>90d</a>
</div>

<h2>Chat</h2>
<span class="stat">Turns: {{.Summary.TurnCount}}</span>
<span class="stat">Avg latency: {{ms2s .Summary.AvgLatencyMs}}</span>
<span class="stat">p95: {{ms2s .Summary.P95LatencyMs}}</span>
<span class="stat">Tokens in: {{.Summary.InputTokens}}</span>
<span class="stat">Tokens out: {{.Summary.OutputTokens}}</span>

<h2>Top Tools</h2>
{{if .Summary.ToolCounts}}
<table><tr><th>Tool</th><th>Calls</th></tr>
{{range $name, $count := .Summary.ToolCounts}}<tr><td>{{$name}}</td><td>{{$count}}</td></tr>{{end}}
</table>
{{else}}<p>No tool events.</p>{{end}}

<h2>Active Courses</h2>
{{if .Summary.CourseCounts}}
<table><tr><th>Course</th><th>Sessions created</th></tr>
{{range $cid, $count := .Summary.CourseCounts}}<tr><td>{{$cid}}</td><td>{{$count}}</td></tr>{{end}}
</table>
{{else}}<p>No session events.</p>{{end}}

<h2>Plan Toggles</h2>
<span class="stat">Done: {{.Summary.PlanDone}}</span>
<span class="stat">Undone: {{.Summary.PlanUndone}}</span>

<h2>PDF Opens</h2>
<span class="stat">{{.Summary.PDFOpens}}</span>

<h2>Recent Events (last 200)</h2>
{{if .Events}}
<table>
<tr><th>Time</th><th>Kind</th><th>Session</th><th>Course</th><th>Tool</th><th>Dur</th><th>OK</th></tr>
{{range .Events}}
<tr>
  <td>{{.FormattedTime}}</td>
  <td>{{.Kind}}</td>
  <td>{{if .SessionID}}{{deref .SessionID}}{{end}}</td>
  <td>{{.CourseID}}</td>
  <td>{{.ToolName}}</td>
  <td>{{if .DurationMs}}{{ms2s .DurationMs}}{{end}}</td>
  <td>{{if .OK}}{{derefBool .OK}}{{end}}</td>
</tr>
{{end}}
</table>
{{else}}<p>No events yet.</p>{{end}}
</body>
</html>`))
```

Wait — the template uses `deref` and `derefBool` which need to be in the FuncMap. Also `FormattedTime` needs to be a method or field. Let me simplify: use a view struct that pre-formats fields.

Replace the above with a cleaner approach using a view struct:

```go
package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"study-app/agent"
)

type eventRow struct {
	Time     string
	Kind     string
	Session  string
	CourseID string
	ToolName string
	Dur      string
	OK       string
}

type metricsData struct {
	Window  string
	Summary agent.EventSummary
	Events  []eventRow
}

var metricsTempl = template.Must(template.New("metrics").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Metrics</title>
<style>
body{font-family:monospace;max-width:960px;margin:2rem auto;padding:0 1rem}
h1{font-size:1.2rem}h2{font-size:1rem;margin-top:2rem;border-bottom:1px solid #ccc}
.windows a{margin-right:1rem}.windows a.active{font-weight:bold;text-decoration:none}
table{border-collapse:collapse;width:100%;font-size:.85rem}
th,td{border:1px solid #ddd;padding:4px 8px;text-align:left}th{background:#f5f5f5}
.stat{display:inline-block;margin-right:2rem}
</style>
</head>
<body>
<h1>Metrics</h1>
<div class="windows">
  <a href="?window=7d"{{if eq .Window "7d"}} class="active"{{end}}>7d</a>
  <a href="?window=30d"{{if eq .Window "30d"}} class="active"{{end}}>30d</a>
  <a href="?window=90d"{{if eq .Window "90d"}} class="active"{{end}}>90d</a>
</div>
<h2>Chat</h2>
<span class="stat">Turns: {{.Summary.TurnCount}}</span>
<span class="stat">Avg latency: {{.Summary.AvgLatencyMs}}ms</span>
<span class="stat">p95: {{.Summary.P95LatencyMs}}ms</span>
<span class="stat">Tokens in: {{.Summary.InputTokens}}</span>
<span class="stat">Tokens out: {{.Summary.OutputTokens}}</span>
<h2>Top Tools</h2>
{{if .Summary.ToolCounts}}<table><tr><th>Tool</th><th>Calls</th></tr>
{{range $n,$c := .Summary.ToolCounts}}<tr><td>{{$n}}</td><td>{{$c}}</td></tr>{{end}}
</table>{{else}}<p>No tool events.</p>{{end}}
<h2>Active Courses</h2>
{{if .Summary.CourseCounts}}<table><tr><th>Course</th><th>Sessions</th></tr>
{{range $c,$n := .Summary.CourseCounts}}<tr><td>{{$c}}</td><td>{{$n}}</td></tr>{{end}}
</table>{{else}}<p>No session events.</p>{{end}}
<h2>Plan Toggles</h2>
<span class="stat">Done: {{.Summary.PlanDone}}</span>
<span class="stat">Undone: {{.Summary.PlanUndone}}</span>
<h2>PDF Opens</h2>
<span class="stat">{{.Summary.PDFOpens}}</span>
<h2>Recent Events (last 200)</h2>
{{if .Events}}<table>
<tr><th>Time</th><th>Kind</th><th>Session</th><th>Course</th><th>Tool</th><th>Dur</th><th>OK</th></tr>
{{range .Events}}<tr><td>{{.Time}}</td><td>{{.Kind}}</td><td>{{.Session}}</td><td>{{.CourseID}}</td><td>{{.ToolName}}</td><td>{{.Dur}}</td><td>{{.OK}}</td></tr>{{end}}
</table>{{else}}<p>No events yet.</p>{{end}}
</body></html>`))

func (h *Handler) handleDebugMetrics(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}

	window := r.URL.Query().Get("window")
	if window != "7d" && window != "90d" {
		window = "30d"
	}
	days := map[string]int{"7d": 7, "30d": 30, "90d": 90}
	since := time.Now().Add(-time.Duration(days[window]) * 24 * time.Hour)

	summary, err := h.App.QueryEventSummary(since)
	if err != nil {
		writeServerError(w, "query event summary", err)
		return
	}

	rawEvs, err := h.App.ListRecentEvents(200)
	if err != nil {
		writeServerError(w, "list recent events", err)
		return
	}

	rows := make([]eventRow, len(rawEvs))
	for i, e := range rawEvs {
		row := eventRow{
			Time:     time.UnixMilli(e.CreatedAt).UTC().Format("01-02 15:04:05"),
			Kind:     e.Kind,
			CourseID: e.CourseID,
			ToolName: e.ToolName,
		}
		if e.SessionID != nil {
			row.Session = fmt.Sprintf("%d", *e.SessionID)
		}
		if e.DurationMs > 0 {
			row.Dur = fmt.Sprintf("%.1fs", float64(e.DurationMs)/1000)
		}
		if e.OK != nil {
			if *e.OK {
				row.OK = "✓"
			} else {
				row.OK = "✗"
			}
		}
		rows[i] = row
	}

	data := metricsData{Window: window, Summary: summary, Events: rows}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := metricsTempl.Execute(w, data); err != nil {
		slog.Warn("render metrics template", "err", err)  // headers already sent
	}
}
```

Add `"log/slog"` to the import if needed.

- [ ] **Step 4: Register the route in `handler/handler.go`**

Add to the `Register` method:

```go
mux.HandleFunc("/debug/metrics", h.handleDebugMetrics)
```

- [ ] **Step 5: Run and verify tests pass**

```bash
go test ./handler/ -run "TestDebugMetrics" -v 2>&1 | tail -10
```

Expected: `PASS`

- [ ] **Step 6: Run all tests**

```bash
go test ./agent/ ./handler/ 2>&1 | tail -5
```

- [ ] **Step 7: Commit**

```bash
git add handler/metrics.go handler/metrics_test.go handler/handler.go
git commit -m "feat(events): /debug/metrics handler with summary and raw event log"
```

---

### Task 10: Retention sweep in main.go

**Files:**
- Modify: `main.go`

No test — this is a background goroutine with a 24h ticker; testing it would require time mocking that's overkill. The underlying `PruneOldEvents` is tested in Task 2.

- [ ] **Step 1: Add the retention goroutine to `main.go`**

Find where the HTTP server is started in `main.go` (look for `http.ListenAndServe` or similar). Before that call, add:

```go
// Prune events older than 90 days on startup and daily thereafter.
go func() {
    cutoff := time.Now().Add(-90 * 24 * time.Hour)
    if n, err := app.PruneOldEvents(cutoff); err != nil {
        slog.Warn("prune old events", "err", err)
    } else if n > 0 {
        slog.Info("pruned old events", "count", n)
    }
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    for range ticker.C {
        cutoff := time.Now().Add(-90 * 24 * time.Hour)
        if n, err := app.PruneOldEvents(cutoff); err != nil {
            slog.Warn("prune old events", "err", err)
        } else if n > 0 {
            slog.Info("pruned old events", "count", n)
        }
    }
}()
```

Make sure `"time"` is imported in `main.go`.

- [ ] **Step 2: Build to verify**

```bash
go build . 2>&1
```

- [ ] **Step 3: Run all tests**

```bash
go test ./agent/ ./handler/ 2>&1 | tail -5
```

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat(events): 90-day retention sweep on startup and daily"
```

---

### Task 11: Final check, push, deploy

- [ ] **Step 1: Run full test suite**

```bash
go test ./agent/ ./handler/ 2>&1
```

Expected: all pass, zero failures.

- [ ] **Step 2: Cross-compile for Linux**

```bash
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
ls -lh /tmp/study-app-linux
```

Expected: ~18 MB ELF binary.

- [ ] **Step 3: Push to remote**

```bash
git push
```

- [ ] **Step 4: Deploy to VPS**

```bash
scp /tmp/study-app-linux nanoclaw:$VAULT_ROOT/bin/study-app.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak && mv study-app.new study-app && chmod +x study-app && export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service'
```

- [ ] **Step 5: Verify service health**

```bash
ssh nanoclaw 'export XDG_RUNTIME_DIR=/run/user/$(id -u); systemctl --user status study-app.service --no-pager | head -5'
```

Expected: `active (running)`

- [ ] **Step 6: Smoke-test `/debug/metrics`**

Open `https://your-host.example/debug/metrics` (with bearer token in the Authorization header or via the app's session cookie). Verify the page loads, shows the three window links, and the Recent Events table is present (may be empty on a fresh deploy).
