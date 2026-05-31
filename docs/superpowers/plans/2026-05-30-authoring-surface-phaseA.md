# Authoring Surface — Phase A Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a conversational "+ new course" Authoring surface: a task-less `authoring` session where the tutor designs a course + plan and creates it via the deterministic writes we already shipped, re-binding the chat to the course it produces.

**Architecture:** Add a `sessions.mode` discriminator (`study|scratch|authoring`) threaded from the session row → chat handler → sandbox → generated AGENTS.md, which branches to an Authoring frame. A "+ new course" rail button creates a task-less `authoring` session via the (previously unused) `POST /api/sessions`. `claw-cli course create --session <id>` re-tags the chat. Reuses `claw-cli course create` / `plan rewrite` (already live).

**Tech Stack:** Go 1.26 (`/opt/homebrew/bin/go`), SQLite, vanilla-JS SPA (`static/`). Tests: `:memory:` DB via `OpenDB`+`InitSchema` (see `agent/course_settings_test.go:newSettingsApp`), `SandboxManager` via `NewSandboxManager` (see `agent/sandbox_test.go`), CLI via `run(...)`/`newTempDB`/`openApp` (`claw-cli/main_test.go`).

---

## File Structure

- **`agent/types.go`** — add `Mode` to `Session` struct. *(Task 1)*
- **`agent/db.go`** — `mode` ALTER migration; `MigrateSessionMode`; `mode` in `ListSessions`/`GetSession` SELECT+scan; `mode` param on `CreateSession`; new `UpdateSessionCourse`. *(Tasks 1, 2, 3)*
- **`main.go`** — call `MigrateSessionMode` after `MigratePhase3Sessions`. *(Task 1)*
- **`handler/sessions.go`** — `mode` in `createSession` request body. *(Task 2)*
- **`claw-cli/main.go`** — `--session` flag on `courseCreate`. *(Task 3)*
- **`agent/sandbox.go`** — `mode` param on `Create` + `writeAgentsMD`; Authoring frame. *(Task 4)*
- **`handler/chat_v2.go`** — pass `sess.Mode` to `Sandbox.Create`. *(Task 4)*
- **`static/rail.js`, `static/app.js`** — "+ new course" button + handler. *(Task 5)*
- Tests in `agent/db_test.go`, `agent/sandbox_test.go`, `handler/sessions_test.go` (or existing), `claw-cli/main_test.go`.

---

## Task 1: `sessions.mode` column, migration, struct, scans

**Files:** `agent/types.go`, `agent/db.go`, `main.go`, test in `agent/db_test.go`

- [ ] **Step 1: Write the failing test** — append to `agent/db_test.go`:

```go
func TestMigrateSessionModeBackfills(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	app := NewApp(Config{VaultRoot: t.TempDir()}, db)
	now := "2026-05-30T00:00:00Z"
	// one task-less row and one task-anchored row, both default mode 'study'
	if _, err := db.Exec("INSERT INTO sessions (course_id, topic, created_at, updated_at) VALUES ('c','scratchy',?,?)", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO sessions (course_id, task_id, topic, created_at, updated_at) VALUES ('c','t1','studyy',?,?)", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := app.MigrateSessionMode(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var scratchMode, studyMode string
	if err := db.QueryRow("SELECT mode FROM sessions WHERE task_id IS NULL").Scan(&scratchMode); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT mode FROM sessions WHERE task_id = 't1'").Scan(&studyMode); err != nil {
		t.Fatal(err)
	}
	if scratchMode != "scratch" {
		t.Fatalf("task-less row should be scratch, got %q", scratchMode)
	}
	if studyMode != "study" {
		t.Fatalf("task row should stay study, got %q", studyMode)
	}
	// idempotent: second run changes nothing
	n, err := app.MigrateSessionMode()
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if n != 0 {
		t.Fatalf("second run should change 0 rows, changed %d", n)
	}
}

func TestGetSessionReturnsMode(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	app := NewApp(Config{VaultRoot: t.TempDir()}, db)
	now := "2026-05-30T00:00:00Z"
	res, err := db.Exec("INSERT INTO sessions (course_id, topic, mode, created_at, updated_at) VALUES ('c','t','authoring',?,?)", now, now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	s, err := app.GetSession(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s.Mode != "authoring" {
		t.Fatalf("expected mode authoring, got %q", s.Mode)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./agent/ -run 'TestMigrateSessionMode|TestGetSessionReturnsMode' -v`
Expected: compile error / FAIL — `MigrateSessionMode` undefined, `s.Mode` undefined, no `mode` column.

- [ ] **Step 3: Add `Mode` to the `Session` struct** — in `agent/types.go`, in `type Session struct` (line 83), add after the `Archived bool ...` line:

```go
	Mode      string  `json:"mode"`
```

- [ ] **Step 4: Add the column migration** — in `agent/db.go`, in the `migrations` slice (line 182-190), add this line (anywhere in the slice):

```go
		"ALTER TABLE sessions ADD COLUMN mode TEXT NOT NULL DEFAULT 'study'",
```

- [ ] **Step 5: Add `mode` to the two full-session SELECTs + scans** — in `agent/db.go`:

In `ListSessions` (line 388) change the query and scan:
```go
	rows, err := a.DB.Query("SELECT id, course_id, task_id, topic, created_at, updated_at, last_pdf_id, last_page, archived, mode FROM sessions WHERE hidden = 0 ORDER BY updated_at DESC")
```
```go
		if err := rows.Scan(&s.ID, &s.CourseID, &s.TaskID, &s.Topic, &s.CreatedAt, &s.UpdatedAt, &s.LastPdfID, &s.LastPage, &s.Archived, &s.Mode); err != nil {
```

In `GetSession` (line 416) change the query and scan:
```go
		"SELECT id, course_id, task_id, topic, created_at, updated_at, last_pdf_id, last_page, archived, mode FROM sessions WHERE id = ?",
```
```go
	).Scan(&s.ID, &s.CourseID, &s.TaskID, &s.Topic, &s.CreatedAt, &s.UpdatedAt, &s.LastPdfID, &s.LastPage, &s.Archived, &s.Mode)
```

- [ ] **Step 6: Add `MigrateSessionMode`** — in `agent/db.go`, after `MigratePhase3Sessions` (ends line 258):

```go
// MigrateSessionMode backfills the sessions.mode column added 2026-05-30:
// task-less rows that still carry the column default ('study') become 'scratch'.
// Guarded by the "session_mode_migration" meta flag so authoring sessions
// (task-less, mode='authoring') created later are never reset. Returns rows changed.
func (a *App) MigrateSessionMode() (int64, error) {
	done, err := a.getMetaInt("session_mode_migration")
	if err != nil {
		return 0, fmt.Errorf("read mode migration flag: %w", err)
	}
	if done != 0 {
		return 0, nil
	}
	res, err := a.DB.Exec("UPDATE sessions SET mode = 'scratch' WHERE task_id IS NULL AND mode = 'study'")
	if err != nil {
		return 0, fmt.Errorf("backfill scratch mode: %w", err)
	}
	changed, _ := res.RowsAffected()
	if err := a.setMetaInt("session_mode_migration", 1); err != nil {
		return 0, fmt.Errorf("set mode migration flag: %w", err)
	}
	return changed, nil
}
```

- [ ] **Step 7: Wire it in `main.go`** — after the `MigratePhase3Sessions` block (line 124-128), add:

```go
	if n, err := app.MigrateSessionMode(); err != nil {
		slog.Warn("session mode migration", "err", err)
	} else if n > 0 {
		slog.Info("session mode migration applied", "rows", n)
	}
```

- [ ] **Step 8: Run tests + build** — `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./agent/ -run 'TestMigrateSessionMode|TestGetSessionReturnsMode' -v && /opt/homebrew/bin/go build .`
Expected: both new tests PASS, build clean.

- [ ] **Step 9: Run full agent suite** — `/opt/homebrew/bin/go test ./agent/` — Expected: `ok` (the new `mode` scan column doesn't break existing session tests).

- [ ] **Step 10: Commit**
```bash
cd ~/Documents/ITA/claw-study
git add agent/types.go agent/db.go main.go agent/db_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "$(cat <<'EOF'
feat(sessions): add mode discriminator (study|scratch|authoring) + migration

Schema column + guarded backfill (task-less existing rows → scratch), threaded
into Session struct and the session SELECTs. Foundation for the Authoring
surface (ADR 0018).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: `CreateSession` mode param + `POST /api/sessions` mode

**Files:** `agent/db.go` (`CreateSession`), all `CreateSession` callers, `handler/sessions.go`, test in `agent/db_test.go`

- [ ] **Step 1: Write the failing test** — append to `agent/db_test.go`:

```go
func TestCreateSessionPersistsMode(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	app := NewApp(Config{VaultRoot: t.TempDir()}, db)
	s, err := app.CreateSession("", "Design a new course", "authoring")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.Mode != "authoring" {
		t.Fatalf("returned session mode = %q, want authoring", s.Mode)
	}
	got, err := app.GetSession(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "authoring" {
		t.Fatalf("persisted mode = %q, want authoring", got.Mode)
	}
}
```

- [ ] **Step 2: Run to verify it fails** — `/opt/homebrew/bin/go test ./agent/ -run TestCreateSessionPersistsMode -v` — Expected: FAIL (CreateSession takes 2 args, not 3).

- [ ] **Step 3: Change `CreateSession`** — in `agent/db.go` (line 311), update signature, default, INSERT, and returned struct:

```go
func (a *App) CreateSession(courseID, topic, mode string) (Session, error) {
	if topic == "" {
		topic = "General"
	}
	if mode == "" {
		mode = "scratch"
	}
	now := time.Now().Format(time.RFC3339)
	res, err := a.DB.Exec(
		"INSERT INTO sessions (course_id, topic, mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		courseID, topic, mode, now, now,
	)
	if err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Session{}, fmt.Errorf("last insert id: %w", err)
	}
	if err := a.setMetaInt("last_session", id); err != nil {
		return Session{}, fmt.Errorf("set last_session: %w", err)
	}
	a.SetActiveSessionIDInMemory(id)
	return Session{
		ID:        id,
		CourseID:  courseID,
		Topic:     topic,
		Mode:      mode,
		CreatedAt: now,
		UpdatedAt: now,
		LastPage:  1,
	}, nil
}
```

- [ ] **Step 4: Update all `CreateSession` callers** — run `grep -rn 'CreateSession(' --include=*.go . | grep -v CreateSessionForTask | grep -v 'func (a \*App) CreateSession'` to list call sites. For each non-test caller, add the mode argument. The known production caller is `handler/sessions.go` (updated in Step 5). For any test caller, pass `"scratch"`.

- [ ] **Step 5: Add `mode` to the create-session handler** — in `handler/sessions.go` `createSession` (line 46), add `Mode` to the body struct and pass it:

```go
	var body struct {
		CourseID string `json:"course_id"`
		TaskID   string `json:"task_id"`
		Topic    string `json:"topic"`
		Mode     string `json:"mode"`
	}
```
and change the task-less branch (line 73):
```go
		s, err = h.App.CreateSession(body.CourseID, body.Topic, body.Mode)
```
(The task-anchored branch keeps `CreateSessionForTask`, which inserts with the column default `study`.)

- [ ] **Step 6: Run tests + build** — `/opt/homebrew/bin/go test ./agent/ ./handler/ -run 'TestCreateSession|Session' && /opt/homebrew/bin/go build .` — Expected: pass + clean. Then full: `/opt/homebrew/bin/go test ./agent/ ./handler/` — Expected: `ok`.

- [ ] **Step 7: Commit**
```bash
cd ~/Documents/ITA/claw-study
git add agent/db.go handler/sessions.go agent/db_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "$(cat <<'EOF'
feat(sessions): CreateSession takes a mode; POST /api/sessions accepts mode

Lets the frontend start a task-less session with an explicit mode
(scratch default, or authoring). Threads mode into the inserted row.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: `UpdateSessionCourse` + `claw-cli course create --session`

**Files:** `agent/db.go` (`UpdateSessionCourse`), `claw-cli/main.go` (`courseCreate`), test in `claw-cli/main_test.go`

- [ ] **Step 1: Write the failing test** — append to `claw-cli/main_test.go`:

```go
func TestCourseCreateWithSessionRetags(t *testing.T) {
	dbPath := newTempDB(t)
	// create a task-less authoring session up front
	var sid int64
	func() {
		app := openApp(t, dbPath)
		defer func() { _ = app.Close() }()
		s, err := app.CreateSession("", "Design a new course", "authoring")
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		sid = s.ID
	}()
	var out, errb bytes.Buffer
	code := run([]string{
		"clawcli", "course", "create", "--id", "retag-course", "--name", "Retag",
		"--session", strconv.FormatInt(sid, 10),
	}, &out, &errb, dbPath)
	if code != 0 {
		t.Fatalf("create exit %d, stderr: %s", code, errb.String())
	}
	app := openApp(t, dbPath)
	defer func() { _ = app.Close() }()
	s, err := app.GetSession(sid)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if s.CourseID != "retag-course" {
		t.Fatalf("session not re-tagged, course_id = %q", s.CourseID)
	}
}
```
(Ensure `strconv` is imported in the test file; add it if missing.)

- [ ] **Step 2: Run to verify it fails** — `/opt/homebrew/bin/go test ./claw-cli/ -run TestCourseCreateWithSessionRetags -v` — Expected: FAIL (no `--session` flag, course_id unchanged).

- [ ] **Step 3: Add `UpdateSessionCourse`** — in `agent/db.go`, after `UpdateSessionTopic` (ends ~line 503):

```go
// UpdateSessionCourse re-tags session id to a course and bumps updated_at.
// Used when an Authoring session creates the course it was designing (ADR 0018).
func (a *App) UpdateSessionCourse(id int64, courseID string) error {
	now := time.Now().Format(time.RFC3339)
	res, err := a.DB.Exec("UPDATE sessions SET course_id = ?, updated_at = ? WHERE id = ?", courseID, now, id)
	if err != nil {
		return fmt.Errorf("update session course: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("session %d not found", id)
	}
	return nil
}
```

- [ ] **Step 4: Add `--session` to `courseCreate`** — in `claw-cli/main.go` (line 636), add the flag and the re-tag call. Add after the `examStyle` flag (line 642):
```go
	session := fs.Int64("session", 0, "optional session id to re-tag to the new course (Authoring)")
```
and after the existing settings handling, just before `return 0` (line 685):
```go
	if *session > 0 {
		if err := app.UpdateSessionCourse(*session, *id); err != nil {
			_, _ = fmt.Fprintf(stderr, "course %s created, but failed to re-tag session %d: %v\n", *id, *session, err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "re-tagged session %d to course %s\n", *session, *id)
	}
```

- [ ] **Step 5: Run tests + build** — `/opt/homebrew/bin/go test ./claw-cli/ -run 'TestCourseCreate' -v && /opt/homebrew/bin/go test ./agent/` — Expected: all PASS (existing `course create` tests still green; new re-tag test passes).

- [ ] **Step 6: Commit**
```bash
cd ~/Documents/ITA/claw-study
git add agent/db.go claw-cli/main.go claw-cli/main_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "$(cat <<'EOF'
feat(course): course create --session re-tags an Authoring chat to the new course

Adds App.UpdateSessionCourse and an optional --session flag so a course-less
Authoring session binds to the course it just produced (ADR 0018).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Mode-aware Authoring frame in `writeAgentsMD`

**Files:** `agent/sandbox.go` (`Create`, `writeAgentsMD`), `handler/chat_v2.go`, `agent/sandbox_test.go`

- [ ] **Step 1: Write the failing test** — append to `agent/sandbox_test.go`:

```go
func TestWriteAgentsMDAuthoringFrame(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	// authoring session: course-less
	path, err := sm.Create(101, "", "", "", "authoring")
	if err != nil {
		t.Fatalf("create authoring: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(path, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "course create --session") {
		t.Fatalf("authoring frame missing the create+retag instruction:\n%s", body)
	}
	// study session must NOT carry the authoring frame
	path2, err := sm.Create(102, "", "ce297", "", "study")
	if err != nil {
		t.Fatalf("create study: %v", err)
	}
	body2, err := os.ReadFile(filepath.Join(path2, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read2: %v", err)
	}
	if strings.Contains(string(body2), "course create --session") {
		t.Fatalf("study session should not have the authoring frame:\n%s", body2)
	}
}
```
(Ensure `os`, `strings`, `path/filepath` are imported in `agent/sandbox_test.go`; add any missing.)

- [ ] **Step 2: Run to verify it fails** — `/opt/homebrew/bin/go test ./agent/ -run TestWriteAgentsMDAuthoringFrame -v` — Expected: FAIL (Create takes 4 args; no authoring frame).

- [ ] **Step 3: Add `mode` to `Create`** — in `agent/sandbox.go` (line 46), change the signature and the `writeAgentsMD` call (line 66):
```go
func (sm *SandboxManager) Create(sessionID int64, clawCLIPath, course, userID, mode string) (string, error) {
```
```go
	if err := sm.writeAgentsMD(agentsMD, clawCLIPath, sessionID, course, userID, mode); err != nil {
```

- [ ] **Step 4: Add `mode` to `writeAgentsMD` + the Authoring frame** — in `agent/sandbox.go` change the signature (line 117):
```go
func (sm *SandboxManager) writeAgentsMD(path, clawCLIPath string, sessionID int64, course, userID, mode string) error {
```
Then, immediately before the function appends the pedagogical rules / at the end of the content assembly (just before the final `os.WriteFile`/return of the function), append the Authoring frame when `mode == "authoring"`:
```go
	if mode == "authoring" {
		authoringFrame := "\n## You are in an Authoring session (designing a course)\n\n" +
			"This is not a study session — Eduardo wants to design a course/plan with you, generatively. " +
			"Use the `course-study-path` skill: grill the intent, research the resources, and build the study plan.\n\n" +
			"When the course is ready, create it (this also re-tags THIS chat to the new course):\n" +
			"```\nclaw-cli course create --id <kebab-slug> --name \"<display name>\" --session " + fmt.Sprintf("%d", sessionID) + "\n```\n" +
			"Then seed the plan's tasks (read it back, edit JSON, submit the whole plan):\n" +
			"```\nclaw-cli plan rewrite --course <kebab-slug> --plan-file <tmp.json>\n```\n" +
			"Pick a stable kebab-case id (ids are permanent). Keep task `id`s stable on later edits. " +
			"Confirm what you created in one or two lines and ask Eduardo to review.\n"
		content = append(content, []byte(authoringFrame)...)
	}
```
(Place this after the existing sections are appended to `content` and before it is written to disk. Confirm `content` is the byte slice the function writes; if the function writes incrementally, append this block at the equivalent final point. `fmt` is already imported.)

- [ ] **Step 5: Pass `sess.Mode` from the chat handler** — in `handler/chat_v2.go` (line 80), change the `Sandbox.Create` call:
```go
	sandboxPath, err := h.App.Sandbox.Create(
		req.SessionID,
		h.App.Config.ClawCLIPath,
		sess.CourseID,
		h.App.Config.UserID,
		sess.Mode,
	)
```

- [ ] **Step 6: Update existing `sm.Create(...)` calls** — the 4-arg calls in `agent/sandbox_test.go` (e.g. `sm.Create(42, "", "", "")`) now need a 5th arg. Run `grep -rn 'sm.Create(\|Sandbox.Create(\|\.Create(' --include=*.go agent handler | grep -v 'sm.Create(101\|sm.Create(102'` and add `, "study"` to each existing 4-arg call (study is the behavior-neutral default for those tests).

- [ ] **Step 7: Run tests + build** — `/opt/homebrew/bin/go test ./agent/ -run TestWriteAgentsMDAuthoringFrame -v && /opt/homebrew/bin/go build . && /opt/homebrew/bin/go test ./agent/ ./handler/` — Expected: new test PASS, build clean, all packages `ok`.

- [ ] **Step 8: Commit**
```bash
cd ~/Documents/ITA/claw-study
git add agent/sandbox.go handler/chat_v2.go agent/sandbox_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "$(cat <<'EOF'
feat(agent): Authoring frame in AGENTS.md for mode=authoring sessions

writeAgentsMD now takes the session mode; an authoring session gets a
course-design frame (course-study-path + course create --session + plan
rewrite) instead of the study frame (ADR 0018).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Frontend "+ new course" entry

**Files:** `static/rail.js`, `static/app.js`

This is the only frontend task. Vanilla JS, no framework. Read the current `static/rail.js` (`renderCourseSwitcher` builds the `.rail-course-row` with the course `<select>` and the ⚙ settings button) and `static/app.js` (rail action wiring via `data-action` click delegation) before editing — match those patterns exactly.

- [ ] **Step 1: Add a "+ new course" button to the rail course row**

In `static/rail.js` `renderCourseSwitcher`, add a button beside the existing course `<select>` and ⚙ button, e.g.:
```js
`<button class="rail-newcourse-btn" data-action="new-course" title="Design a new course">+ course</button>`
```
Match the existing class/markup style of the ⚙ `rail-settings-btn` in the same row.

- [ ] **Step 2: Wire the click handler**

Wherever rail `data-action` clicks are dispatched (the same delegation that handles `open-settings` / task clicks — find it in `static/app.js` or `static/rail.js`), add a `case 'new-course'` (or `if (action === 'new-course')`) that calls a new async function `startNewCourseAuthoring()`:

```js
async function startNewCourseAuthoring() {
  const resp = await apiFetch('/api/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ course_id: '', task_id: '', mode: 'authoring', topic: 'Design a new course' }),
  });
  if (!resp.ok) { showErrorBanner('Could not start a new course chat.'); return; }
  const session = await resp.json();
  await apiFetch('/api/sessions/active', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: session.id }),
  });
  setActiveSessionId(session.id);   // use the existing active-session setter
  clearChatTranscript();            // use the existing transcript-clear used when switching sessions
  await loadRailData();             // refresh the rail
  focusChatInput();                 // use the existing chat-focus helper if present
}
```
Use the EXACT helper names that already exist in the codebase for: the authenticated fetch (`apiFetch`), setting the active session, clearing/loading the chat transcript on session switch, and the error banner. Read `static/app.js` / `static/chat.js` to find them (e.g. `getActiveSessionId`/`setActiveSessionId`, `switchSession`, `loadActiveSession`). Prefer reusing an existing "switch to session N" routine over re-implementing — if a `switchSession(id)` exists, call it instead of manually clearing+activating.

- [ ] **Step 3: Build + local preview**

Static assets are `//go:embed`-ed, so a JS change needs a rebuild to preview. Use the documented local recipe:
```bash
cd ~/Documents/ITA/claw-study
VAULT_ROOT=/tmp/claw-authoring-vault LISTEN_ADDR=127.0.0.1:8099 AUTH_TOKEN= LLM_API_KEY=dummy AGENT_RUNTIME=pi /opt/homebrew/bin/go run .
```
(loadConfig requires an LLM key even locally; empty AUTH_TOKEN disables the gate.)

- [ ] **Step 4: Browser-verify via Chrome CDP**

Drive headless Chrome (the reusable driver `/tmp/cdp.mjs` from prior frontend work, or `--remote-debugging-port=9222` + `Runtime.evaluate`). Verify: clicking "+ course" creates a session (network POST `/api/sessions` with `mode:"authoring"`), the chat clears, and `/api/sessions` now lists a session with `mode:"authoring"` and empty `task_id`. Capture the eval result showing the new session's `mode`.

- [ ] **Step 5: Commit**
```bash
cd ~/Documents/ITA/claw-study
git add static/rail.js static/app.js
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "$(cat <<'EOF'
feat(rail): "+ new course" opens a conversational Authoring chat

Adds a rail button that POSTs a task-less mode=authoring session and switches
to it — the human front door to course design (ADR 0018, Phase A).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Deploy

**Files:** none. Touches `claw-cli` (`course create --session`) AND `study-app` (everything else) → both binaries. Migration runs on boot.

- [ ] **Step 1: Pre-deploy sync** — `cd ~/Documents/ITA/claw-study && git fetch origin && git rev-list --left-right --count origin/main...HEAD`. If behind, `git merge origin/main --no-edit` and re-run `/opt/homebrew/bin/go test ./...` before deploying.

- [ ] **Step 2: Push (requires explicit user OK — direct-to-main is harness-gated)** — `git push origin main`.

- [ ] **Step 3: Build both binaries**
```bash
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/claw-cli-linux ./claw-cli
ls -la /tmp/study-app-linux /tmp/claw-cli-linux
```

- [ ] **Step 4: Deploy both with backups + restart**
```bash
cd ~/Documents/ITA/claw-study
scp -q /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
scp -q /tmp/claw-cli-linux nanoclaw:/home/eduardo/stack/study-app/bin/claw-cli.new
ssh nanoclaw 'cd ~/stack/study-app/bin && \
  cp study-app study-app.bak.2026-05-30-authoring-A && mv study-app.new study-app && chmod +x study-app && \
  cp claw-cli claw-cli.bak.2026-05-30-authoring-A && mv claw-cli.new claw-cli && chmod +x claw-cli && \
  export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service && \
  sleep 2 && systemctl --user is-active study-app.service'
```
Expected: `active`. Check `journalctl --user -u study-app.service --since "1 min ago"` shows the mode migration line (or no error).

- [ ] **Step 5: Live smoke**
```bash
DB=/home/eduardo/stack/study-app/data/study.db
# create a throwaway authoring session, confirm mode persists:
ssh nanoclaw "sqlite3 -json $DB \"SELECT id, mode, task_id FROM sessions ORDER BY id DESC LIMIT 3;\""
# exercise the re-tag end-to-end via CLI (the agent's exact path):
ssh nanoclaw "VAULT_ROOT=/home/eduardo/stack/study-app /usr/local/bin/claw-cli course create --id authoring-smoke --name 'Authoring Smoke' --db $DB --session 999999"  # bad session → exit 1, clear msg
# (Then verify the UI: open https://study.claw-study.xyz, click "+ course", confirm a chat opens.)
# cleanup throwaway course:
ssh nanoclaw "sqlite3 $DB \"DELETE FROM courses WHERE id='authoring-smoke';\""
```
Expected: sessions carry a `mode`; `course create --session 999999` exits 1 with "failed to re-tag session 999999" (no such session); UI "+ course" opens a chat. Clean up.

- [ ] **Step 6: Update memory** — in `claw_study_experience_redesign.md`, add a bullet: Authoring surface Phase A shipped (sessions.mode + migration, "+ new course" rail entry → task-less authoring session, mode-aware AGENTS.md Authoring frame, `course create --session` re-tag), ADR 0018, deployed; Phase B (existing-course design entry + rail Design section) still pending.

---

## Self-Review Notes

- **Spec coverage:** Task 1 = `sessions.mode` + migration + struct/scan (spec §Design.1). Task 2 = `CreateSession` mode + `POST /api/sessions` mode (spec §Design.2). Task 3 = re-tag via `course create --session` + `UpdateSessionCourse` (spec §Design.5). Task 4 = mode-aware Authoring frame + threading (spec §Design.4). Task 5 = "+ new course" entry (spec §Design.3). Task 6 = deploy (spec §Deploy). Interim rail (spec §Design.6) needs no work. ADR 0018 + spec already committed.
- **Type/signature consistency across tasks:** `CreateSession(courseID, topic, mode string)` (Task 2) is used in Task 3's test and Task 5's payload feeds it via the handler. `Create(sessionID, clawCLIPath, course, userID, mode string)` and `writeAgentsMD(..., mode string)` (Task 4) match the chat_v2 call. `UpdateSessionCourse(id int64, courseID string)` (Task 3) is called by `course create --session`. `Session.Mode` (Task 1) is read in Tasks 3/4 tests and `sess.Mode` in chat_v2. The `mode` SELECT columns (Task 1) populate `GetSession`/`ListSessions` used everywhere.
- **Placeholders:** none — all code complete. Frontend (Task 5) intentionally instructs reuse of existing helper names (read from the codebase) rather than inventing them, since the exact helper names must match `static/`.
