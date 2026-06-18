# Phase 3a — Task-Anchored Sessions (Backend) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Sessions a `task_id` anchor and the supporting data-layer operations so the (later) plan-spine UI can resolve a Task → its one Session, list Scratch, and show per-task "has-work" — plus a one-time clean-break migration of existing sessions.

**Architecture:** Add three nullable/defaulted columns to `sessions` (`task_id`, `archived`, `hidden`) via the existing idempotent-ALTER pattern. Expose them on the `Session` struct and through `ListSessions` (which now hides `hidden` rows). Add `GetSessionByTask` + `CreateSessionForTask` and a get-or-create `/api/sessions/for-task` endpoint (the lazy-creation hook — the row is created on the first message, not on task click). A guarded, one-time migration hides synthetic sessions and archives pre-redesign task-less ones. No FK — `task_id` is a soft TEXT ref to a plan task's UUID (ADR 0014). Frontend (3b) computes the "Detached" bucket by cross-referencing `task_id` against the live plan.

**Tech Stack:** Go (`modernc.org/sqlite`), stdlib `net/http`, table-driven Go tests with the existing `newMemoryApp(t)` helper.

**Spec:** [ADR 0014](../../adr/0014-phase3-task-anchored-sessions-data-model.md). Glossary: [CONTEXT.md](../../CONTEXT.md) (*Session*, *Scratch*).

**Sync constraint:** This phase changes only `agent/*.go` and `handler/*.go` — all carried by the `study-app` binary. No disk-mounted file (`CLAUDE.local.md`, `skills/*`) changes. Deploy = build + scp + swap + restart (Task 6). The `claw-cli` binary is unaffected by 3a.

---

## File Structure

- `agent/db.go` — schema columns, idempotent ALTERs, one-time Phase-3 migration, `GetSessionByTask`, `CreateSessionForTask`, updated `ListSessions`/`GetSession` scans.
- `agent/types.go` — `Session` struct gains `TaskID *string`, `Archived bool`.
- `agent/db_test.go` — tests for the new query methods and the migration.
- `handler/sessions.go` — `createSession` accepts optional `task_id`; new `handleSessionForTask`.
- `handler/handler.go` — register `/api/sessions/for-task`.
- `handler/sessions_test.go` — test for the get-or-create endpoint.

---

## Task 1: Schema columns + Session struct + ListSessions hides hidden

**Files:**
- Modify: `agent/types.go` (`Session` struct)
- Modify: `agent/db.go` (`InitSchema` migrations block; `ListSessions`; `GetSession`)
- Test: `agent/db_test.go`

- [ ] **Step 1: Write the failing test**

Add to `agent/db_test.go`:

```go
func TestSessionTaskIDRoundTrips(t *testing.T) {
	a := newMemoryApp(t)
	s, err := a.CreateSession("ce297", "STPA")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// A freshly created session has no task anchor and is not archived.
	got, err := a.GetSession(s.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TaskID != nil {
		t.Errorf("TaskID = %v, want nil", got.TaskID)
	}
	if got.Archived {
		t.Errorf("Archived = true, want false")
	}
}

func TestListSessionsExcludesHidden(t *testing.T) {
	a := newMemoryApp(t)
	visible, err := a.CreateSession("ce297", "real work")
	if err != nil {
		t.Fatalf("create visible: %v", err)
	}
	hidden, err := a.CreateSession("verifier-stats", "stats-verifier")
	if err != nil {
		t.Fatalf("create hidden: %v", err)
	}
	if _, err := a.DB.Exec("UPDATE sessions SET hidden = 1 WHERE id = ?", hidden.ID); err != nil {
		t.Fatalf("mark hidden: %v", err)
	}

	list, err := a.ListSessions()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, s := range list {
		if s.ID == hidden.ID {
			t.Errorf("hidden session %d appeared in ListSessions", hidden.ID)
		}
	}
	var sawVisible bool
	for _, s := range list {
		if s.ID == visible.ID {
			sawVisible = true
		}
	}
	if !sawVisible {
		t.Errorf("visible session %d missing from ListSessions", visible.ID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `/opt/homebrew/bin/go test ./agent/ -run 'TestSessionTaskIDRoundTrips|TestListSessionsExcludesHidden' -v`
Expected: FAIL — `got.TaskID` undefined (struct field missing) / compile error.

- [ ] **Step 3: Add the struct fields**

In `agent/types.go`, change the `Session` struct to add `TaskID` and `Archived` (place `TaskID` right after `CourseID`, `Archived` after `LastPage`):

```go
type Session struct {
	ID        int64   `json:"id"`
	CourseID  string  `json:"course_id"`
	TaskID    *string `json:"task_id"`
	Topic     string  `json:"topic"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	LastPdfID *int64  `json:"last_pdf_id"`
	LastPage  int     `json:"last_page"`
	Archived  bool    `json:"archived"`
	PdfName   string  `json:"pdf_name,omitempty"`
	Summary   string  `json:"summary"`
	SummaryAt int     `json:"summary_at"`
}
```

- [ ] **Step 4: Add the columns + idempotent migrations**

In `agent/db.go`, inside `InitSchema`, append to the `migrations` slice (after the existing `confidence_log` rename line):

```go
		"ALTER TABLE sessions ADD COLUMN task_id TEXT",
		"ALTER TABLE sessions ADD COLUMN archived INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE sessions ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0",
```

(The existing loop already suppresses the `duplicate column` sentinel, so this is idempotent.)

- [ ] **Step 5: Update `ListSessions` to select the new columns and hide `hidden`**

In `agent/db.go`, replace the `ListSessions` query + scan. The full method becomes:

```go
func (a *App) ListSessions() ([]Session, error) {
	rows, err := a.DB.Query("SELECT id, course_id, task_id, topic, created_at, updated_at, last_pdf_id, last_page, archived FROM sessions WHERE hidden = 0 ORDER BY updated_at DESC")
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.CourseID, &s.TaskID, &s.Topic, &s.CreatedAt, &s.UpdatedAt, &s.LastPdfID, &s.LastPage, &s.Archived); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		if s.LastPdfID != nil {
			if name, err := a.PDFOriginalName(*s.LastPdfID); err == nil {
				s.PdfName = name
			}
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}
	return sessions, nil
}
```

- [ ] **Step 6: Update `GetSession` to select the new columns**

In `agent/db.go`, replace the `GetSession` query + scan (keep the `PdfName` enrichment):

```go
func (a *App) GetSession(id int64) (Session, error) {
	var s Session
	err := a.DB.QueryRow(
		"SELECT id, course_id, task_id, topic, created_at, updated_at, last_pdf_id, last_page, archived FROM sessions WHERE id = ?",
		id,
	).Scan(&s.ID, &s.CourseID, &s.TaskID, &s.Topic, &s.CreatedAt, &s.UpdatedAt, &s.LastPdfID, &s.LastPage, &s.Archived)
	if err != nil {
		return Session{}, err
	}
	if s.LastPdfID != nil {
		if name, err := a.PDFOriginalName(*s.LastPdfID); err == nil {
			s.PdfName = name
		}
	}
	return s, nil
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `/opt/homebrew/bin/go test ./agent/ -run 'TestSessionTaskIDRoundTrips|TestListSessionsExcludesHidden' -v`
Expected: PASS.

- [ ] **Step 8: Run the full agent + handler suite (guard against scan-arity regressions)**

Run: `/opt/homebrew/bin/go build ./... && /opt/homebrew/bin/go test ./agent/ ./handler/`
Expected: build OK, `ok study-app/agent`, `ok study-app/handler`.

- [ ] **Step 9: Commit**

```bash
git add agent/types.go agent/db.go agent/db_test.go
git -c user.email=you@example.com -c user.name=your-name commit -m "feat(sessions): add task_id/archived/hidden columns; ListSessions hides hidden (ADR 0014)"
```

---

## Task 2: GetSessionByTask + CreateSessionForTask

**Files:**
- Modify: `agent/db.go` (new methods near `CreateSession`)
- Test: `agent/db_test.go`

The 1:1 rule (ADR 0014): a (course, task) pair has at most one Session. `GetSessionByTask` finds it; `CreateSessionForTask` creates an anchored one.

- [ ] **Step 1: Write the failing test**

Add to `agent/db_test.go`:

```go
func TestCreateAndGetSessionByTask(t *testing.T) {
	a := newMemoryApp(t)

	// No session for the task yet.
	if _, ok, err := a.GetSessionByTask("ddia", "task-uuid-1"); err != nil || ok {
		t.Fatalf("expected (not found), got ok=%v err=%v", ok, err)
	}

	created, err := a.CreateSessionForTask("ddia", "task-uuid-1", "DDIA 3.3 Weak Isolation")
	if err != nil {
		t.Fatalf("create for task: %v", err)
	}
	if created.TaskID == nil || *created.TaskID != "task-uuid-1" {
		t.Fatalf("TaskID = %v, want task-uuid-1", created.TaskID)
	}
	if created.CourseID != "ddia" {
		t.Errorf("CourseID = %q, want ddia", created.CourseID)
	}

	got, ok, err := a.GetSessionByTask("ddia", "task-uuid-1")
	if err != nil || !ok {
		t.Fatalf("expected (found), got ok=%v err=%v", ok, err)
	}
	if got.ID != created.ID {
		t.Errorf("GetSessionByTask returned id %d, want %d", got.ID, created.ID)
	}

	// A different task in the same course is a different (or no) session.
	if _, ok, _ := a.GetSessionByTask("ddia", "task-uuid-2"); ok {
		t.Errorf("unexpected session for task-uuid-2")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestCreateAndGetSessionByTask -v`
Expected: FAIL — `GetSessionByTask`/`CreateSessionForTask` undefined.

- [ ] **Step 3: Implement the two methods**

In `agent/db.go`, add directly after the existing `CreateSession` method (after its closing `}` and the `return s, nil`):

```go
// CreateSessionForTask creates a Session anchored to a plan task's UUID
// (ADR 0014, 1:1 Task↔Session). topic defaults to "General" when empty.
func (a *App) CreateSessionForTask(courseID, taskID, topic string) (Session, error) {
	if topic == "" {
		topic = "General"
	}
	now := time.Now().Format(time.RFC3339)
	res, err := a.DB.Exec(
		"INSERT INTO sessions (course_id, task_id, topic, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		courseID, taskID, topic, now, now,
	)
	if err != nil {
		return Session{}, fmt.Errorf("insert task session: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Session{}, fmt.Errorf("last insert id: %w", err)
	}
	if err := a.setMetaInt("last_session", id); err != nil {
		return Session{}, fmt.Errorf("set last_session: %w", err)
	}
	a.SetActiveSessionIDInMemory(id)
	return a.GetSession(id)
}

// GetSessionByTask returns the (single) Session anchored to (courseID, taskID).
// The bool is false when none exists. Hidden sessions are ignored.
func (a *App) GetSessionByTask(courseID, taskID string) (Session, bool, error) {
	var id int64
	err := a.DB.QueryRow(
		"SELECT id FROM sessions WHERE course_id = ? AND task_id = ? AND hidden = 0 ORDER BY id LIMIT 1",
		courseID, taskID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, fmt.Errorf("get session by task: %w", err)
	}
	s, err := a.GetSession(id)
	if err != nil {
		return Session{}, false, err
	}
	return s, true, nil
}
```

- [ ] **Step 4: Ensure imports**

`agent/db.go` must import `database/sql` and `errors` (for `sql.ErrNoRows` / `errors.Is`). Check the top `import` block; if either is missing, add it. Verify with:

Run: `/opt/homebrew/bin/go build ./agent/`
Expected: build OK (no "undefined: sql" / "undefined: errors").

- [ ] **Step 5: Run test to verify it passes**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestCreateAndGetSessionByTask -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add agent/db.go agent/db_test.go
git -c user.email=you@example.com -c user.name=your-name commit -m "feat(sessions): CreateSessionForTask + GetSessionByTask (1:1 anchor, ADR 0014)"
```

---

## Task 3: `/api/sessions/for-task` get-or-create endpoint (lazy hook)

**Files:**
- Modify: `handler/sessions.go` (new `handleSessionForTask`)
- Modify: `handler/handler.go` (route registration)
- Test: `handler/sessions_test.go`

This is the lazy-creation hook: the frontend opens a task's workspace with no row; on the first message it POSTs here to get-or-create the anchored Session, then chats with the returned id. GET is a pure lookup (returns `{"id": null}` when none).

- [ ] **Step 1: Inspect the test harness**

Read the top of `handler/sessions_test.go` to find the existing handler-test constructor (how `*Handler` + an in-memory `App` are built, e.g. a `newTestHandler(t)` helper). Reuse that exact helper in the new test below — do not invent a new one. Run:

Run: `grep -n "func newTestHandler\|func newHandler\|httptest" handler/sessions_test.go handler/testutil_test.go`
Expected: a helper name to reuse (substitute it for `newTestHandler(t)` in Step 2 if it differs).

- [ ] **Step 2: Write the failing test**

Add to `handler/sessions_test.go` (replace `newTestHandler` with the helper found in Step 1 if different):

```go
func TestHandleSessionForTask_GetThenCreate(t *testing.T) {
	h := newTestHandler(t)

	// GET before anything exists → {"id": null}
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/for-task?course=ddia&task_id=t-1", nil)
	rec := httptest.NewRecorder()
	h.handleSessionForTask(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, `"id":null`) {
		t.Fatalf("GET body = %s, want {\"id\":null}", got)
	}

	// POST get-or-create → creates and returns a session anchored to the task
	body := strings.NewReader(`{"course_id":"ddia","task_id":"t-1","topic":"DDIA 3.3"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/sessions/for-task", body)
	rec = httptest.NewRecorder()
	h.handleSessionForTask(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d, want 200", rec.Code)
	}
	var created agent.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.TaskID == nil || *created.TaskID != "t-1" {
		t.Fatalf("created.TaskID = %v, want t-1", created.TaskID)
	}

	// POST again → returns the SAME session (get-or-create, 1:1)
	body = strings.NewReader(`{"course_id":"ddia","task_id":"t-1","topic":"ignored"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/sessions/for-task", body)
	rec = httptest.NewRecorder()
	h.handleSessionForTask(rec, req)
	var second agent.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	if second.ID != created.ID {
		t.Errorf("second POST id = %d, want same as first %d", second.ID, created.ID)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `/opt/homebrew/bin/go test ./handler/ -run TestHandleSessionForTask_GetThenCreate -v`
Expected: FAIL — `h.handleSessionForTask` undefined.

- [ ] **Step 4: Implement the handler**

In `handler/sessions.go`, add (after `handleSessionMessages`, before `getSessionStats`):

```go
// handleSessionForTask resolves the single Session anchored to a (course,
// task) pair. GET is a pure lookup ({"id": null} when none). POST is
// get-or-create — the lazy-creation hook the workspace calls on the first
// message (ADR 0014). Both require course_id and task_id.
func (h *Handler) handleSessionForTask(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		course := r.URL.Query().Get("course")
		taskID := r.URL.Query().Get("task_id")
		if course == "" || taskID == "" {
			writeError(w, http.StatusBadRequest, "course and task_id are required")
			return
		}
		s, ok, err := h.App.GetSessionByTask(course, taskID)
		if err != nil {
			writeServerError(w, "get session by task", err)
			return
		}
		if !ok {
			writeJSON(w, http.StatusOK, map[string]interface{}{"id": nil})
			return
		}
		writeJSON(w, http.StatusOK, s)

	case http.MethodPost:
		var body struct {
			CourseID string `json:"course_id"`
			TaskID   string `json:"task_id"`
			Topic    string `json:"topic"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		if body.CourseID == "" || body.TaskID == "" {
			writeError(w, http.StatusBadRequest, "course_id and task_id are required")
			return
		}
		if s, ok, err := h.App.GetSessionByTask(body.CourseID, body.TaskID); err != nil {
			writeServerError(w, "get session by task", err)
			return
		} else if ok {
			writeJSON(w, http.StatusOK, s)
			return
		}
		s, err := h.App.CreateSessionForTask(body.CourseID, body.TaskID, body.Topic)
		if err != nil {
			writeServerError(w, "create session for task", err)
			return
		}
		sid := s.ID
		if err := h.App.RecordEvent(agent.Event{
			Kind:      "session_create",
			SessionID: &sid,
			CourseID:  s.CourseID,
			CreatedAt: time.Now().UnixMilli(),
		}); err != nil {
			slog.Warn("record session_create event", "err", err)
		}
		writeJSON(w, http.StatusOK, s)

	default:
		methodNotAllowed(w, r, http.MethodGet, http.MethodPost)
	}
}
```

- [ ] **Step 5: Register the route**

In `handler/handler.go`, inside `Register`, add after the `/api/sessions/stats` line:

```go
	mux.HandleFunc("/api/sessions/for-task", h.handleSessionForTask)
```

- [ ] **Step 6: Run test to verify it passes**

Run: `/opt/homebrew/bin/go test ./handler/ -run TestHandleSessionForTask_GetThenCreate -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add handler/sessions.go handler/handler.go handler/sessions_test.go
git -c user.email=you@example.com -c user.name=your-name commit -m "feat(api): /api/sessions/for-task get-or-create (lazy task-session hook, ADR 0014)"
```

---

## Task 4: One-time clean-break migration (hide synthetic, archive pre-redesign)

**Files:**
- Modify: `agent/db.go` (new `migratePhase3Sessions` + call from `InitSchema`)
- Test: `agent/db_test.go`

ADR 0014: existing sessions are *not* retro-anchored. Provable junk is **hidden**; remaining pre-redesign task-less sessions are **archived** (shown later in Scratch's "Before the redesign" group). Runs **once**, guarded by a meta flag so new Scratch chats are never archived.

- [ ] **Step 1: Write the failing test**

Add to `agent/db_test.go`:

```go
func TestMigratePhase3Sessions(t *testing.T) {
	a := newMemoryApp(t)

	// Seed a representative mix BEFORE running the migration.
	realID := mustSession(t, a, "ce297", "Ch.8 Event Tree Analysis") // real, task-less → archive
	verifierID := mustSession(t, a, "verifier-stats", "stats-verifier") // synthetic → hide
	smokeID := mustSession(t, a, "ce297", "phase5 smoke v3")            // smoke → hide
	emptyID := mustSession(t, a, "ddia", "General")                    // 0 messages → hide
	// Give the "real" one a message so it is NOT treated as empty.
	if err := a.SaveMessage(realID, "user", "hello"); err != nil {
		t.Fatalf("seed message: %v", err)
	}

	n, err := a.migratePhase3Sessions()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if n == 0 {
		t.Fatalf("expected migration to touch rows, got 0")
	}

	assertHidden := func(id int64, want bool) {
		var hidden int
		if err := a.DB.QueryRow("SELECT hidden FROM sessions WHERE id = ?", id).Scan(&hidden); err != nil {
			t.Fatalf("scan hidden %d: %v", id, err)
		}
		if (hidden == 1) != want {
			t.Errorf("session %d hidden=%d, want hidden=%v", id, hidden, want)
		}
	}
	assertArchived := func(id int64, want bool) {
		var archived int
		if err := a.DB.QueryRow("SELECT archived FROM sessions WHERE id = ?", id).Scan(&archived); err != nil {
			t.Fatalf("scan archived %d: %v", id, err)
		}
		if (archived == 1) != want {
			t.Errorf("session %d archived=%d, want archived=%v", id, archived, want)
		}
	}

	assertHidden(verifierID, true)
	assertHidden(smokeID, true)
	assertHidden(emptyID, true)
	assertHidden(realID, false)
	assertArchived(realID, true)
	assertArchived(verifierID, false) // hidden rows are not also archived

	// Idempotency: a session created AFTER migration is left untouched.
	freshID := mustSession(t, a, "ddia", "new scratch")
	if err := a.SaveMessage(freshID, "user", "post-migration"); err != nil {
		t.Fatalf("seed fresh message: %v", err)
	}
	again, err := a.migratePhase3Sessions()
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if again != 0 {
		t.Errorf("second migration touched %d rows, want 0 (guard failed)", again)
	}
	assertArchived(freshID, false)
}

// mustSession creates a session or fails the test.
func mustSession(t *testing.T, a *App, course, topic string) int64 {
	t.Helper()
	s, err := a.CreateSession(course, topic)
	if err != nil {
		t.Fatalf("create session (%s/%s): %v", course, topic, err)
	}
	return s.ID
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestMigratePhase3Sessions -v`
Expected: FAIL — `a.migratePhase3Sessions` undefined.

- [ ] **Step 3: Implement the migration**

In `agent/db.go`, add this method (place it after `InitSchema`):

```go
// migratePhase3Sessions runs the one-time ADR-0014 clean-break migration:
// hide provably-synthetic sessions, then archive the remaining pre-redesign
// task-less sessions. Guarded by the "phase3_session_migration" meta flag so
// Scratch chats created later are never archived. Returns rows changed.
func (a *App) migratePhase3Sessions() (int64, error) {
	done, err := a.getMetaInt("phase3_session_migration")
	if err != nil {
		return 0, fmt.Errorf("read migration flag: %w", err)
	}
	if done != 0 {
		return 0, nil
	}

	var changed int64

	// 1. Hide synthetic / junk: verifier output, smoke tests, smoke courses,
	//    and any zero-message session.
	hideRes, err := a.DB.Exec(`
		UPDATE sessions SET hidden = 1
		WHERE hidden = 0 AND (
			course_id = 'verifier-stats'
			OR topic LIKE 'phase5 smoke%'
			OR course_id LIKE 'postship-smoke-%'
			OR id NOT IN (SELECT DISTINCT session_id FROM messages)
		)`)
	if err != nil {
		return 0, fmt.Errorf("hide synthetic sessions: %w", err)
	}
	if n, _ := hideRes.RowsAffected(); n > 0 {
		changed += n
	}

	// 2. Archive remaining pre-redesign, task-less, visible sessions.
	archRes, err := a.DB.Exec(`
		UPDATE sessions SET archived = 1
		WHERE hidden = 0 AND archived = 0 AND task_id IS NULL`)
	if err != nil {
		return 0, fmt.Errorf("archive pre-redesign sessions: %w", err)
	}
	if n, _ := archRes.RowsAffected(); n > 0 {
		changed += n
	}

	if err := a.setMetaInt("phase3_session_migration", 1); err != nil {
		return 0, fmt.Errorf("set migration flag: %w", err)
	}
	return changed, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestMigratePhase3Sessions -v`
Expected: PASS.

- [ ] **Step 5: Wire the migration into startup**

The migration must run once at boot, AFTER the schema/ALTERs exist. `InitSchema(db)` is a package function with no `*App`; the migration needs `*App` (for `getMetaInt`/`setMetaInt`). Call it from wherever the App is constructed and `InitSchema` is invoked at startup. Find that site:

Run: `grep -rn "InitSchema\|LoadActiveSessionID" agent/app.go agent/*.go main.go 2>/dev/null | grep -v _test`

Then, at that startup site (the same place `LoadActiveSessionID()` is called on the constructed `*App`), add:

```go
	if n, err := app.migratePhase3Sessions(); err != nil {
		slog.Warn("phase3 session migration", "err", err)
	} else if n > 0 {
		slog.Info("phase3 session migration applied", "rows", n)
	}
```

(Use the actual `*App` variable name at that site — likely `app` or `a`. If `slog` is not imported there, add `"log/slog"`.)

- [ ] **Step 6: Build + full suite**

Run: `/opt/homebrew/bin/go build ./... && /opt/homebrew/bin/go test ./agent/ ./handler/`
Expected: build OK; both packages `ok`.

- [ ] **Step 7: Commit**

```bash
git add agent/db.go agent/db_test.go agent/app.go
git -c user.email=you@example.com -c user.name=your-name commit -m "feat(sessions): one-time Phase-3 clean-break migration — hide synthetic, archive pre-redesign (ADR 0014)"
```

(Adjust the `git add` path in Step 7 to the file actually edited in Step 5 if it is not `agent/app.go`.)

---

## Task 5: Let `createSession` accept an optional `task_id` (anchored create via the main endpoint)

**Files:**
- Modify: `handler/sessions.go` (`createSession`)
- Test: `handler/sessions_test.go`

Belt-and-suspenders so the existing `POST /api/sessions` can also create an anchored session (e.g. if 3b reuses it). Pure-Scratch creates (no `task_id`) keep working unchanged.

- [ ] **Step 1: Write the failing test**

Add to `handler/sessions_test.go`:

```go
func TestCreateSessionWithTaskID(t *testing.T) {
	h := newTestHandler(t)
	body := strings.NewReader(`{"course_id":"ddia","task_id":"t-9","topic":"anchored"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", body)
	rec := httptest.NewRecorder()
	h.handleSessions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var s agent.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.TaskID == nil || *s.TaskID != "t-9" {
		t.Errorf("TaskID = %v, want t-9", s.TaskID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `/opt/homebrew/bin/go test ./handler/ -run TestCreateSessionWithTaskID -v`
Expected: FAIL — `s.TaskID` is nil (current `createSession` ignores `task_id`).

- [ ] **Step 3: Update `createSession`**

In `handler/sessions.go`, replace the `createSession` body's decode + create section:

```go
func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CourseID string `json:"course_id"`
		TaskID   string `json:"task_id"`
		Topic    string `json:"topic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var s agent.Session
	var err error
	if body.TaskID != "" {
		s, err = h.App.CreateSessionForTask(body.CourseID, body.TaskID, body.Topic)
	} else {
		s, err = h.App.CreateSession(body.CourseID, body.Topic)
	}
	if err != nil {
		writeServerError(w, "create session", err)
		return
	}
	sid := s.ID
	if err := h.App.RecordEvent(agent.Event{
		Kind:      "session_create",
		SessionID: &sid,
		CourseID:  s.CourseID,
		CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		slog.Warn("record session_create event", "err", err)
	}
	writeJSON(w, http.StatusOK, s)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `/opt/homebrew/bin/go test ./handler/ -run TestCreateSessionWithTaskID -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add handler/sessions.go handler/sessions_test.go
git -c user.email=you@example.com -c user.name=your-name commit -m "feat(api): POST /api/sessions accepts optional task_id (ADR 0014)"
```

---

## Task 6: Deploy (binary only) — REQUIRES EXPLICIT DEPLOY APPROVAL

**Files:** none (build + ship `study-app`; `claw-cli` and disk files unchanged in 3a).

> Do not run this task without the user's explicit "deploy" go-ahead (prod boundary). The one-time migration runs automatically on the new binary's first boot — it is idempotent (meta-flag guarded) and only hides/archives; it deletes nothing and is reversible by clearing the columns.

- [ ] **Step 1: Verify clean + green on `main`**

Run: `git status -sb && /opt/homebrew/bin/go vet ./... && /opt/homebrew/bin/go test ./...`
Expected: on `main`, working tree clean (after commits), vet silent, all packages `ok`.

- [ ] **Step 2: Build the linux binary**

Run: `GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .`
Expected: exit 0.

- [ ] **Step 3: Ship + swap + restart**

Run:
```bash
scp /tmp/study-app-linux nanoclaw:$VAULT_ROOT/bin/study-app.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak.2026-05-30-phase3a && mv study-app.new study-app && chmod +x study-app && systemctl --user restart study-app.service && sleep 3 && systemctl --user is-active study-app.service && systemctl --user is-active study-app-tunnel.service'
```
Expected: both `active`.

- [ ] **Step 4: Verify migration applied + health**

Run:
```bash
curl -s -o /dev/null -w "%{http_code}\n" https://your-host.example/
ssh nanoclaw 'cd ~/stack/study-app && echo -n "migration flag: "; sqlite3 data/study.db "SELECT value FROM meta WHERE key=\"phase3_session_migration\";"; echo -n "hidden count: "; sqlite3 data/study.db "SELECT COUNT(*) FROM sessions WHERE hidden=1;"; echo -n "archived count: "; sqlite3 data/study.db "SELECT COUNT(*) FROM sessions WHERE archived=1;"; journalctl --user -u study-app.service -n 20 --no-pager | grep -i "phase3 session migration"'
```
Expected: HTTP `401` (auth = healthy); `migration flag: 1`; `hidden count` ≈ 6 (2 verifier + 4 smoke, plus any 0-msg); `archived count` > 0; a log line `phase3 session migration applied rows=N`.

- [ ] **Step 5: Push**

Run: `git push origin main`
Expected: `ok main`.

---

## Self-Review

**1. Spec coverage (ADR 0014):**
- (1) 1:1 Session↔Task → Task 2 (`GetSessionByTask` enforces single lookup) + Task 3 (get-or-create returns same row). ✓
- (2) Soft `task_id` TEXT ref, no FK → Task 1 (`ADD COLUMN task_id TEXT`, no constraint). ✓
- (3) Scratch = `task_id NULL` → no new entity; `CreateSession` (unchanged) yields a Scratch row; surfaced via `ListSessions`. ✓
- (4) Studying/Authoring/Steering → these are UI/behaviour concerns, not backend schema; out of 3a scope (3b + Phase 4). ✓ (noted)
- (5) Clean-break migration (hide synthetic, archive pre-redesign, new sessions only) → Task 4. ✓
- (6) Lazy creation (row on first message) → Task 3's POST get-or-create is the first-message hook; no eager create. ✓
- (7) Learned resource (`last_pdf_id`) → already shipped in Phase 2; no 3a work. ✓
- "Detached" bucket → computed in 3b by cross-referencing `task_id` vs live plan; no backend column (orphan = `task_id` present but absent from plan). ✓
- "Rail shows only human courses / hide synthetic" → Task 1 (`ListSessions WHERE hidden=0`) + Task 4 (set `hidden`). ✓

**2. Placeholder scan:** No TBD/TODO; every code step shows complete code; commands have expected output. Two steps (Task 4 Step 5, Task 3 Step 1) require a `grep` to locate an exact site/helper name — these are deliberate "confirm the local name" steps with the command given, not placeholders. ✓

**3. Type consistency:** `Session.TaskID *string` and `Session.Archived bool` defined in Task 1 are used consistently in Tasks 2/3/5. `CreateSessionForTask(courseID, taskID, topic string) (Session, error)` and `GetSessionByTask(courseID, taskID string) (Session, bool, error)` defined in Task 2 are called with matching signatures in Tasks 3 and 5. `handleSessionForTask` defined in Task 3 is registered in Task 3 Step 5. `migratePhase3Sessions() (int64, error)` defined in Task 4 is called in Task 4 Step 5. ✓

**Out-of-scope (3b, separate plan):** the rail/workspace/Scratch UI, the Detached-bucket cross-reference, retiring the flat session list in `static/sessions.js`, and routing the chat-send flow through `/api/sessions/for-task`.
