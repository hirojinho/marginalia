# Phase 4 — Steering Settings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give each course a small set of declarative Steering knobs (framing, exam style, reading chunk size, stop-after-task, interleaving) that persist to a source-of-truth table, are editable from a settings form *and* from the tutor via a deterministic typed tool, and flow into the generated `AGENTS.md` so the tutor's behavior actually reflects them.

**Architecture:** A new `course_settings` SQLite table (one row per course, lazy — missing rows resolve to behavior-preserving defaults) is the single source of truth. Two writers share one validated DB function: the settings form (`PUT /api/courses/settings`) and a `claw-cli course settings set` subcommand the Pi agent calls. The read path is `writeAgentsMD` (`agent/sandbox.go`), which reads the settings every turn (AGENTS.md is regenerated per turn) and (a) emits a "How to teach this course" section and (b) parameterizes the pedagogy rules in-place (chunk size, interleaving clause, stop-after-task state). The `study-step-complete` skill is edited once to defer to the stated stop flag. Decisions recorded in ADR 0010 (amended) + ADR 0016; CONTEXT.md amended.

**Tech Stack:** Go (`net/http` ServeMux, `database/sql` + SQLite), embedded SPA (vanilla ES modules + `style.css`), `claw-cli` companion binary. Tests: Go `testing`; frontend verified via headless Chrome CDP.

**Deploy note:** This touches `claw-cli`, so deploying rebuilds **both** binaries (`study-app` *and* `claw-cli`), and the edited `study-step-complete/SKILL.md` must be scp'd separately (skills are disk-mounted on the VPS, not carried by the binary).

---

## File Structure

- **Create** `agent/course_settings.go` — `CourseSettings` struct, `DefaultCourseSettings`, `ValidateCourseSettings`, and the `*App` methods `GetCourseSettings`, `UpsertCourseSettings`, `SetCourseSetting`, plus `parseBoolSetting`. (Mirrors `agent/memory.go`'s pattern of type + store methods in one focused file.)
- **Create** `agent/course_settings_test.go` — unit tests for the above.
- **Modify** `agent/db.go` — add the `course_settings` CREATE TABLE to the schema string.
- **Modify** `agent/app.go` — wire `Sandbox.Settings` to `GetCourseSettings` in `NewApp`.
- **Modify** `agent/sandbox.go` — add a `Settings func(string) CourseSettings` field; emit the framing section; parameterize rules 6/9 and add rule 10; add a "Course settings (Steering)" tool section.
- **Modify** `agent/sandbox_test.go` — tests for the parameterized output.
- **Modify** `skills/study-step-complete/SKILL.md` — Step 5 defers to the stop flag.
- **Modify** `claw-cli/main.go` — `course settings get|set` subcommands.
- **Modify** `claw-cli/main_test.go` — subcommand tests.
- **Create** `handler/course_settings.go` — `GET`/`PUT /api/courses/settings`.
- **Modify** `handler/handler.go` — register the route.
- **Create** `handler/course_settings_test.go` — handler tests.
- **Create** `static/settings.js` — the settings modal.
- **Modify** `static/rail.js` — ⚙ button + click handler + import.
- **Modify** `static/style.css` — modal styles.

---

## Task 1: `course_settings` data layer (table, struct, methods, validation)

**Files:**
- Create: `agent/course_settings.go`
- Create: `agent/course_settings_test.go`
- Modify: `agent/db.go` (schema string, after the `knowledge_components` table, before the closing backtick at line ~165)

- [ ] **Step 1: Add the table to the schema string**

In `agent/db.go`, inside the `schema := \`...\`` block, immediately after the `CREATE INDEX IF NOT EXISTS idx_knowledge_components_task ...;` line and before the closing backtick, add:

```sql
	CREATE TABLE IF NOT EXISTS course_settings (
		course_id        TEXT PRIMARY KEY,
		framing          TEXT    NOT NULL DEFAULT '',
		exam_style       TEXT    NOT NULL DEFAULT '',
		chunk_pages      INTEGER NOT NULL DEFAULT 8,
		stop_after_task  INTEGER NOT NULL DEFAULT 1,
		interleaving     INTEGER NOT NULL DEFAULT 1,
		updated_at       INTEGER NOT NULL
	);
```

- [ ] **Step 2: Write the failing tests**

Create `agent/course_settings_test.go`:

```go
package agent

import (
	"testing"
)

func newSettingsApp(t *testing.T) *App {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return NewApp(Config{VaultRoot: t.TempDir()}, db)
}

func TestGetCourseSettingsReturnsDefaultsWhenNoRow(t *testing.T) {
	app := newSettingsApp(t)
	s, err := app.GetCourseSettings("ce297")
	if err != nil {
		t.Fatalf("GetCourseSettings: %v", err)
	}
	if s.CourseID != "ce297" || s.ChunkPages != 8 || !s.StopAfterTask || !s.Interleaving {
		t.Fatalf("defaults wrong: %+v", s)
	}
	if s.Framing != "" || s.ExamStyle != "" {
		t.Fatalf("expected empty text defaults: %+v", s)
	}
}

func TestSetCourseSettingPersistsAndRoundTrips(t *testing.T) {
	app := newSettingsApp(t)
	cases := []struct{ key, val string }{
		{"framing", "exam-prep first"},
		{"exam_style", "conceptual oral"},
		{"chunk_pages", "6"},
		{"stop_after_task", "false"},
		{"interleaving", "off"},
	}
	for _, c := range cases {
		if err := app.SetCourseSetting("ce297", c.key, c.val); err != nil {
			t.Fatalf("set %s=%s: %v", c.key, c.val, err)
		}
	}
	s, err := app.GetCourseSettings("ce297")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s.Framing != "exam-prep first" || s.ExamStyle != "conceptual oral" ||
		s.ChunkPages != 6 || s.StopAfterTask || s.Interleaving {
		t.Fatalf("round-trip wrong: %+v", s)
	}
}

func TestSetCourseSettingRejectsBadValues(t *testing.T) {
	app := newSettingsApp(t)
	if err := app.SetCourseSetting("ce297", "chunk_pages", "999"); err == nil {
		t.Fatal("expected chunk_pages range error")
	}
	if err := app.SetCourseSetting("ce297", "chunk_pages", "notanint"); err == nil {
		t.Fatal("expected chunk_pages parse error")
	}
	if err := app.SetCourseSetting("ce297", "bogus_key", "x"); err == nil {
		t.Fatal("expected unknown-key error")
	}
	if err := app.SetCourseSetting("ce297", "stop_after_task", "maybe"); err == nil {
		t.Fatal("expected bool parse error")
	}
}

func TestUpsertCourseSettingsValidates(t *testing.T) {
	if err := ValidateCourseSettings(CourseSettings{CourseID: "x", ChunkPages: 2}); err == nil {
		t.Fatal("expected chunk_pages<3 to fail validation")
	}
	if err := ValidateCourseSettings(CourseSettings{CourseID: "x", ChunkPages: 8}); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /Users/eduardohiroji/Documents/ITA/claw-study && go test ./agent/ -run 'CourseSetting' -v`
Expected: FAIL — `undefined: CourseSettings`, `app.GetCourseSettings undefined`, etc.

- [ ] **Step 4: Implement the data layer**

Create `agent/course_settings.go`:

```go
package agent

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CourseSettings holds the per-course declarative Steering knobs (ADR 0010,
// 0016). One row per course in the course_settings table; a course with no
// row resolves to DefaultCourseSettings (behavior-preserving). The agent
// never writes these into generated files — both the settings form and the
// claw-cli tool go through UpsertCourseSettings / SetCourseSetting.
type CourseSettings struct {
	CourseID      string `json:"course_id"`
	Framing       string `json:"framing"`
	ExamStyle     string `json:"exam_style"`
	ChunkPages    int    `json:"chunk_pages"`
	StopAfterTask bool   `json:"stop_after_task"`
	Interleaving  bool   `json:"interleaving"`
	UpdatedAt     int64  `json:"updated_at"`
}

// DefaultCourseSettings returns the behavior-preserving defaults that match
// the pre-Phase-4 hardcoded tutor behavior: ~8-page chunks, stop after each
// task, interleaved opener on.
func DefaultCourseSettings(courseID string) CourseSettings {
	return CourseSettings{
		CourseID:      courseID,
		ChunkPages:    8,
		StopAfterTask: true,
		Interleaving:  true,
	}
}

// ValidateCourseSettings enforces the bounds shared by both write paths.
func ValidateCourseSettings(s CourseSettings) error {
	if s.ChunkPages < 3 || s.ChunkPages > 30 {
		return fmt.Errorf("chunk_pages must be between 3 and 30, got %d", s.ChunkPages)
	}
	if len(s.Framing) > 4000 {
		return fmt.Errorf("framing too long (max 4000 chars)")
	}
	if len(s.ExamStyle) > 4000 {
		return fmt.Errorf("exam_style too long (max 4000 chars)")
	}
	return nil
}

// GetCourseSettings returns the stored settings for courseID, or the
// behavior-preserving defaults if no row exists. A DB error returns defaults
// plus the error (callers on the read path may ignore it safely).
func (a *App) GetCourseSettings(courseID string) (CourseSettings, error) {
	s := DefaultCourseSettings(courseID)
	var stop, inter int
	err := a.DB.QueryRow(
		"SELECT framing, exam_style, chunk_pages, stop_after_task, interleaving, updated_at FROM course_settings WHERE course_id = ?",
		courseID,
	).Scan(&s.Framing, &s.ExamStyle, &s.ChunkPages, &stop, &inter, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return s, nil
	}
	if err != nil {
		return DefaultCourseSettings(courseID), fmt.Errorf("query course settings %q: %w", courseID, err)
	}
	s.StopAfterTask = stop != 0
	s.Interleaving = inter != 0
	return s, nil
}

// UpsertCourseSettings writes the full settings row (insert or replace).
// Callers must validate first.
func (a *App) UpsertCourseSettings(s CourseSettings) error {
	if s.CourseID == "" {
		return fmt.Errorf("course id is required")
	}
	stop, inter := 0, 0
	if s.StopAfterTask {
		stop = 1
	}
	if s.Interleaving {
		inter = 1
	}
	_, err := a.DB.Exec(
		`INSERT INTO course_settings (course_id, framing, exam_style, chunk_pages, stop_after_task, interleaving, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(course_id) DO UPDATE SET
		   framing=excluded.framing, exam_style=excluded.exam_style, chunk_pages=excluded.chunk_pages,
		   stop_after_task=excluded.stop_after_task, interleaving=excluded.interleaving, updated_at=excluded.updated_at`,
		s.CourseID, s.Framing, s.ExamStyle, s.ChunkPages, stop, inter, time.Now().UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("upsert course settings: %w", err)
	}
	return nil
}

// SetCourseSetting mutates a single knob by key, validates, and upserts.
// This is the deterministic write path used by the claw-cli tool (ADR 0016).
func (a *App) SetCourseSetting(courseID, key, value string) error {
	if courseID == "" {
		return fmt.Errorf("course id is required")
	}
	s, err := a.GetCourseSettings(courseID)
	if err != nil {
		return err
	}
	switch key {
	case "framing":
		s.Framing = value
	case "exam_style":
		s.ExamStyle = value
	case "chunk_pages":
		n, convErr := strconv.Atoi(strings.TrimSpace(value))
		if convErr != nil {
			return fmt.Errorf("chunk_pages must be an integer, got %q", value)
		}
		s.ChunkPages = n
	case "stop_after_task":
		b, bErr := parseBoolSetting(value)
		if bErr != nil {
			return bErr
		}
		s.StopAfterTask = b
	case "interleaving":
		b, bErr := parseBoolSetting(value)
		if bErr != nil {
			return bErr
		}
		s.Interleaving = b
	default:
		return fmt.Errorf("unknown setting key %q (valid: framing, exam_style, chunk_pages, stop_after_task, interleaving)", key)
	}
	if err := ValidateCourseSettings(s); err != nil {
		return err
	}
	return a.UpsertCourseSettings(s)
}

// parseBoolSetting accepts the human spellings the agent or a form might emit.
func parseBoolSetting(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on", "yes":
		return true, nil
	case "false", "0", "off", "no":
		return false, nil
	default:
		return false, fmt.Errorf("expected a boolean (true/false/on/off), got %q", value)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/eduardohiroji/Documents/ITA/claw-study && go test ./agent/ -run 'CourseSetting' -v`
Expected: PASS (all four tests).

- [ ] **Step 6: Commit**

```bash
cd /Users/eduardohiroji/Documents/ITA/claw-study
git add agent/course_settings.go agent/course_settings_test.go agent/db.go
git commit -m "feat(settings): course_settings table + data layer (ADR 0010/0016)"
```

---

## Task 2: Read path — parameterize AGENTS.md from settings

**Files:**
- Modify: `agent/app.go:101-107` (wire `Sandbox.Settings`)
- Modify: `agent/sandbox.go` (struct field ~19-23; `writeAgentsMD` ~112-194)
- Modify: `agent/sandbox_test.go` (add tests)

- [ ] **Step 1: Write the failing tests**

Add to `agent/sandbox_test.go`:

```go
func readAgentsMD(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	return string(b)
}

func TestWriteAgentsMDParameterizesSteering(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	sm.Settings = func(string) CourseSettings {
		return CourseSettings{
			CourseID: "ce297", Framing: "exam-prep first", ExamStyle: "conceptual oral",
			ChunkPages: 6, StopAfterTask: false, Interleaving: false,
		}
	}
	dir, err := sm.Create(1, "", "ce297", "eduardo")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	md := readAgentsMD(t, dir)
	for _, want := range []string{"~6 pages per chunk", "Stop-after-task is OFF", "exam-prep first", "conceptual oral", "How to teach this course"} {
		if !strings.Contains(md, want) {
			t.Errorf("AGENTS.md missing %q", want)
		}
	}
	if strings.Contains(md, "interleaved spaced retrieval") {
		t.Errorf("interleaving clause should be absent when Interleaving=false")
	}
}

func TestWriteAgentsMDUsesDefaultsWhenNoProvider(t *testing.T) {
	sm := NewSandboxManager(t.TempDir()) // Settings nil → defaults
	dir, err := sm.Create(2, "", "ce297", "eduardo")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	md := readAgentsMD(t, dir)
	for _, want := range []string{"~8 pages per chunk", "Stop-after-task is ON", "interleaved spaced retrieval"} {
		if !strings.Contains(md, want) {
			t.Errorf("AGENTS.md missing default %q", want)
		}
	}
	if strings.Contains(md, "How to teach this course") {
		t.Errorf("framing section should be absent when framing/exam_style empty")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/eduardohiroji/Documents/ITA/claw-study && go test ./agent/ -run 'WriteAgentsMD' -v`
Expected: FAIL — `sm.Settings undefined`.

- [ ] **Step 3: Add the `Settings` field to `SandboxManager`**

In `agent/sandbox.go`, the struct (around line 19) becomes:

```go
// SandboxManager creates, reuses, and cleans up per-session sandboxes.
// Construct with NewSandboxManager; the zero value is invalid.
type SandboxManager struct {
	baseDir string
	outDir  string
	// Settings, if set, supplies per-course Steering settings for AGENTS.md
	// generation. Nil → DefaultCourseSettings is used. Wired in NewApp.
	Settings func(courseID string) CourseSettings
}
```

(Keep the existing `baseDir`/`outDir` fields exactly as they are — only add the `Settings` field. If the struct's field names differ, match them; do not rename.)

- [ ] **Step 4: Wire the provider in `NewApp`**

In `agent/app.go`, replace the `NewApp` body (lines 101-107):

```go
func NewApp(cfg Config, db *sql.DB) *App {
	app := &App{
		DB:      db,
		Config:  cfg,
		Sandbox: NewSandboxManager(cfg.VaultRoot),
	}
	app.Sandbox.Settings = func(courseID string) CourseSettings {
		s, _ := app.GetCourseSettings(courseID) // defaults on error are safe here
		return s
	}
	return app
}
```

- [ ] **Step 5: Emit the framing section and parameterize the rules**

In `agent/sandbox.go` `writeAgentsMD`, first resolve settings. Add this right after the `pdfSection` is appended (after line ~171, before the `// Pedagogical rules go last` comment):

```go
	// Resolve per-course Steering settings (ADR 0010/0016). Nil provider or
	// missing row → behavior-preserving defaults.
	settings := DefaultCourseSettings(course)
	if sm.Settings != nil {
		settings = sm.Settings(course)
	}

	// Framing / exam-style section — only when the learner has set something.
	if course != "" && (settings.Framing != "" || settings.ExamStyle != "") {
		var fb strings.Builder
		fb.WriteString("\n## How to teach this course\n\n")
		fb.WriteString("The learner's Steering settings for this course (set via the settings UI or by his explicit request). Honor them:\n\n")
		if settings.Framing != "" {
			fb.WriteString(fmt.Sprintf("- **Framing / goal:** %s\n", settings.Framing))
		}
		if settings.ExamStyle != "" {
			fb.WriteString(fmt.Sprintf("- **Exam style:** %s\n", settings.ExamStyle))
		}
		content = append(content, []byte(fb.String())...)
	}

	// Tool section: how to change a setting conversationally (ADR 0016).
	steerTool := "\n## Course settings (Steering) — change via tool, never via files\n\n" +
		"Durable course settings live in a database table, surfaced above and in the rules below. " +
		"If Eduardo asks to change one (\"smaller chunks\", \"stop chaining\", \"exam-prep framing\"), make the change with:\n" +
		"```\nclaw-cli course settings set --course " + courseArgOrPlaceholder(course) + " --key <framing|exam_style|chunk_pages|stop_after_task|interleaving> --value <value>\n```\n" +
		"Then confirm in ONE line and resume what you were doing — do not turn the session into a config conversation. " +
		"**Never write settings into AGENTS.md, notes, or any file** — only this tool persists them. The change takes effect next turn.\n"
	content = append(content, []byte(steerTool)...)
```

Add this helper near the bottom of `agent/sandbox.go` (package-level func):

```go
// courseArgOrPlaceholder returns the course id, or a placeholder for
// course-less (Scratch) sessions where the agent must ask which course.
func courseArgOrPlaceholder(course string) string {
	if course == "" {
		return "<course-id — ask which course>"
	}
	return course
}
```

Now replace the static `pedagogySection := "..."` assignment (lines ~175-187) with a built version. The rules 1-5, 7, 8 and the interest-log trailer are unchanged; rules 6, 9, 10 are dynamic:

```go
	// Pedagogical rules go last so they sit closest to the user message in
	// the assembled context — maximum LLM weight. Rules 6/9/10 reflect the
	// course's Steering settings (ADR 0010/0016).
	rule6 := "6. **Session-open retrieval check.** At the start of every chat session, before answering anything else, run ONE recall check. Usually ask him to recall, in his own words, the main idea of his most recent completed task. Occasionally instead pick an OLDER completed task from an earlier phase and ask him to recall that (interleaved spaced retrieval — Rohrer 2007; Cepeda 2008) — useful when earlier material is at risk of fading. Exactly one check either way; keep the opener small. Compare his recall against the actual content silently — note gaps and surface them this turn. Non-negotiable; highest-evidence pedagogic move (Roediger & Karpicke 2006, testing effect).\n"
	if !settings.Interleaving {
		rule6 = "6. **Session-open retrieval check.** At the start of every chat session, before answering anything else, run ONE recall check: ask him to recall, in his own words, the main idea of his most recent completed task. (Interleaving of older tasks is OFF for this course — stay on the most recent.) Keep the opener small. Compare his recall against the actual content silently — note gaps and surface them this turn. Non-negotiable; highest-evidence pedagogic move (Roediger & Karpicke 2006, testing effect).\n"
	}

	rule9 := fmt.Sprintf("9. **The reading is his — read to ground yourself, never to lecture.** A 🔴 Read task is HIS cognitive work, not yours to narrate. Chunk a long reading (~%d pages per chunk) and per chunk loop *predict → he reads → boundary recall*, ending with a full recall + confidence check; a short reading stays whole. **Position-gate (run before every boundary recall): read the page number in the `<reading_state>` block. If it is below the chunk's last page, do NOT advance or accept \"done\" — say where he is and that the chunk isn't finished, e.g.** *\"`<reading_state>` shows you on p.18, but this chunk runs to p.24 — finish it and I'll quiz you.\"* **Only run the boundary recall once the block confirms he reached the chunk's end.** An explain-back does not substitute for the page check — a confident summary can come from skimming. You MAY read the pages (`pdf extract`) to orient, to judge his recall/prediction, and to clarify questions *he* asks — but you must NOT reproduce, quote, summarize, or paraphrase a chunk's content before he has read it. Hand off explicitly: name the page range, ask him to read it, and wait. **Pull vs. push:** a question or \"explain this equation\" pulls grounded content out (always allowed — explicit requests override); dumping content before he reads is the leak. \"Read it interactively / together\" means smaller chunks with more recall, NOT lecturing. (ADR 0012 + 0015; Sweller 1988; Chi et al. 1989, self-explanation; Richland/Kornell/Kao 2009, pretesting effect; Bjork & Bjork 2011, desirable difficulties; Dunlosky et al. 2013, passive rereading is low-utility.)\n\n", settings.ChunkPages)

	stopState, stopGuidance := "ON", "After he completes one task, STOP — affirm the stopping point and do NOT chain, preview, or start the next task. Continuing is opt-in (only if he says \"keep going\")."
	if !settings.StopAfterTask {
		stopState, stopGuidance = "OFF", "After he completes one task, you MAY offer to continue to the next task in the same session (chaining is allowed for this course)."
	}
	rule10 := fmt.Sprintf("10. **Stop-after-task is %s.** %s (This is the `stop_after_task` Steering setting; the study-step-complete skill Step 5 defers to it.)\n", stopState, stopGuidance)

	pedagogySection := "\n## Pedagogical Rules (MANDATORY — apply on every turn)\n\n" +
		"These govern how you teach Eduardo. Break them and the conversation is broken.\n\n" +
		"1. **NEVER lecture continuously.** Max 3–4 sentences, then stop and ask him to explain back, apply, or react. If he hasn't spoken in the last 4 sentences, you're lecturing — stop.\n" +
		"2. **ALWAYS ask \"What do you already know about X?\"** before explaining a new concept. Calibrate to his current model; do not start from zero.\n" +
		"3. **ALWAYS ask \"How confident are you with this?\"** before moving to a new topic. After the user replies, parse a value in [0.0, 1.0] from their answer and call the log_confidence tool with knowledge_component_id = the active task's id field from the plan, value = your parsed value, and raw = their verbatim reply. If no active task is in context, skip the tool call (prompt-only behavior). Low confidence → return to the previous topic; do not advance.\n" +
		"4. **ALWAYS connect new concepts to prior knowledge.** Tie X to something he has already engaged with (earlier course material, Brendi work, prior thesis interests). No standalone introductions.\n" +
		"5. **Progress through Bloom's levels: explain → apply → analyze → evaluate → create.** After he can explain X, ask him to apply it; after application, ask him to analyze (compare / find weaknesses); after analysis, ask him to evaluate; finally, where the topic supports it, ask him to create (synthesize / design / extend). Do not skip levels.\n" +
		rule6 +
		"7. **Pre-Read prediction.** Before opening any new 🔴 Read task, ask him to predict in one sentence what he thinks the key idea will be — then **STOP**. Do not reveal, hint at, confirm, or answer it in the same turn, and never fabricate a prediction on his behalf. Only after he has predicted *and* read the chunk do you compare his prediction against the actual content — the gap is where the learning happens. (Slamecka & Graf 1978, generation effect; Richland, Kornell & Kao 2009, pretesting effect.)\n" +
		"8. **Term budget: max 3 new technical terms per turn.** If a topic requires introducing more, break it across turns with a Rule-3 confidence check in between. (Sweller 1988, intrinsic cognitive load management.)\n" +
		rule9 +
		rule10 +
		"\n### Interest log — surface once per session\n\n" +
		"Once per study session, surface the oldest 1–2 entries from the course's `interests.md` (path is in the course profile section above). Ask: \"Do you want to spend 20 min on this now, or close it?\" Closure is a real option — the log should not become psychic debt. Skip this prompt if the session is clearly tactical (planning, debugging, single-task focus).\n"
	content = append(content, []byte(pedagogySection)...)
```

Note: the original Rule 9 ended with `\n\n` before the interest-log subheading; `rule9` keeps that trailing `\n\n`, and the interest-log block starts with `\n###` — preserving spacing. Verify the diff shows rules 1-5/7/8 text byte-identical to the original.

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd /Users/eduardohiroji/Documents/ITA/claw-study && go test ./agent/ -run 'WriteAgentsMD|Sandbox' -v`
Expected: PASS (new tests + existing sandbox tests still green — `Create(...)` signature is unchanged).

- [ ] **Step 7: Full agent build + vet**

Run: `cd /Users/eduardohiroji/Documents/ITA/claw-study && go build ./... && go vet ./agent/`
Expected: no output (success).

- [ ] **Step 8: Commit**

```bash
cd /Users/eduardohiroji/Documents/ITA/claw-study
git add agent/sandbox.go agent/sandbox_test.go agent/app.go
git commit -m "feat(settings): parameterize AGENTS.md pedagogy rules from course settings"
```

---

## Task 3: `study-step-complete` skill defers to the stop flag

**Files:**
- Modify: `skills/study-step-complete/SKILL.md:96-107` and the Red Flags row at line ~117

This is a disk-mounted skill (no Go test). Verify by inspection + a grep assertion.

- [ ] **Step 1: Rewrite Step 5 to defer to the flag**

Replace the Step 5 block (lines 96-107) with:

```markdown
### Step 5 — Mark a stopping point (honor the `stop_after_task` setting)

Completing one task is a complete session. **Honor the `stop_after_task`
Steering setting stated in the Pedagogical Rules section of AGENTS.md** (Rule
10):

**When `stop_after_task` is ON (the default):** stop here — distributed
practice beats massed sessions (Cepeda 2008).

- Affirm the stop: "Good stopping point. Come back tomorrow and we'll open with a
  quick recall on this." Name in one phrase what next time will open with, so the
  return has a hook.
- Do **not** recommend, preview, or start the next task.
- Continuing is **opt-in**: only if the learner explicitly says "keep going" do
  you proceed, treating the next task as a fresh task in this session.

**When `stop_after_task` is OFF:** you may offer to continue to the next task in
the same session — still affirm the completion first, and still let the learner
decline.
```

- [ ] **Step 2: Update the Red Flags row**

Replace the row (line ~117):

```markdown
| Chaining into the next task by default | Stop after one task; continuing is opt-in (the learner says "keep going") |
```

with:

```markdown
| Chaining when `stop_after_task` is ON | Honor Rule 10 in AGENTS.md; when ON, stop and let continuing be opt-in |
```

- [ ] **Step 3: Verify the edit**

Run: `cd /Users/eduardohiroji/Documents/ITA/claw-study && grep -c "stop_after_task" skills/study-step-complete/SKILL.md`
Expected: `3` (Step 5 heading reference, the two state paragraphs reference it, red-flag row — at least 3 matches).

- [ ] **Step 4: Commit**

```bash
cd /Users/eduardohiroji/Documents/ITA/claw-study
git add skills/study-step-complete/SKILL.md
git commit -m "feat(settings): study-step-complete defers to stop_after_task flag"
```

---

## Task 4: `claw-cli course settings get|set` subcommand

**Files:**
- Modify: `claw-cli/main.go` (`runCourse` dispatch ~546-552; add funcs after `courseInterests` ~582)
- Modify: `claw-cli/main_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `claw-cli/main_test.go`:

```go
func TestCourseSettingsSetAndGet(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "course", "settings", "set",
		"--course", "ce297", "--key", "chunk_pages", "--value", "6",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("set exit %d, stderr: %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"clawcli", "course", "settings", "get", "--course", "ce297",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("get exit %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "chunk_pages: 6") {
		t.Fatalf("expected chunk_pages: 6 in output:\n%s", out)
	}
	if !strings.Contains(out, "stop_after_task: true") {
		t.Fatalf("expected default stop_after_task: true:\n%s", out)
	}
}

func TestCourseSettingsSetRejectsBadKey(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "course", "settings", "set",
		"--course", "ce297", "--key", "nope", "--value", "x",
	}, &stdout, &stderr, dbPath)
	if code == 0 {
		t.Fatalf("expected non-zero exit for bad key; stderr: %s", stderr.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/eduardohiroji/Documents/ITA/claw-study && go test ./claw-cli/ -run 'CourseSettings' -v`
Expected: FAIL — `unknown course subcommand: "settings"` (exit 2).

- [ ] **Step 3: Add the dispatch + commands**

In `claw-cli/main.go`, extend `runCourse`'s switch (and its usage string):

```go
func runCourse(args []string, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli course <interests|settings> [args]")
		return 2
	}
	switch args[0] {
	case "interests":
		return courseInterests(args[1:], stdout, stderr)
	case "settings":
		return courseSettings(args[1:], stdout, stderr, dbPath)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown course subcommand: %q\n", args[0])
		return 2
	}
}
```

Add these functions after `courseInterests` (after line ~582):

```go
func courseSettings(args []string, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli course settings <get|set> [args]")
		return 2
	}
	switch args[0] {
	case "get":
		return courseSettingsGet(args[1:], stdout, stderr, dbPath)
	case "set":
		return courseSettingsSet(args[1:], stdout, stderr, dbPath)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown course settings subcommand: %q\n", args[0])
		return 2
	}
}

func printCourseSettings(w io.Writer, s agent.CourseSettings) {
	_, _ = fmt.Fprintf(w, "course: %s\n", s.CourseID)
	_, _ = fmt.Fprintf(w, "framing: %s\n", s.Framing)
	_, _ = fmt.Fprintf(w, "exam_style: %s\n", s.ExamStyle)
	_, _ = fmt.Fprintf(w, "chunk_pages: %d\n", s.ChunkPages)
	_, _ = fmt.Fprintf(w, "stop_after_task: %v\n", s.StopAfterTask)
	_, _ = fmt.Fprintf(w, "interleaving: %v\n", s.Interleaving)
}

func courseSettingsGet(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("course settings get", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id (required)")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *course == "" {
		_, _ = fmt.Fprintln(stderr, "course settings get: --course is required")
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	app, err := newAppFromEnv(resolvedDB, false)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() { _ = app.Close() }()
	s, err := app.GetCourseSettings(*course)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	printCourseSettings(stdout, s)
	return 0
}

func courseSettingsSet(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("course settings set", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id (required)")
	key := fs.String("key", "", "setting key (framing|exam_style|chunk_pages|stop_after_task|interleaving)")
	value := fs.String("value", "", "setting value")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *course == "" || *key == "" {
		_, _ = fmt.Fprintln(stderr, "course settings set: --course and --key are required")
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	app, err := newAppFromEnv(resolvedDB, false)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() { _ = app.Close() }()
	if err := app.SetCourseSetting(*course, *key, *value); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "set %s = %s for course %s\n", *key, *value, *course)
	s, _ := app.GetCourseSettings(*course)
	printCourseSettings(stdout, s)
	return 0
}
```

(`agent`, `flag`, `fmt`, `io` are already imported in `claw-cli/main.go`.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/eduardohiroji/Documents/ITA/claw-study && go test ./claw-cli/ -run 'CourseSettings' -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
cd /Users/eduardohiroji/Documents/ITA/claw-study
git add claw-cli/main.go claw-cli/main_test.go
git commit -m "feat(settings): claw-cli course settings get|set (ADR 0016 tool path)"
```

---

## Task 5: HTTP settings endpoint (`GET`/`PUT /api/courses/settings`)

**Files:**
- Create: `handler/course_settings.go`
- Modify: `handler/handler.go:55` (register route)
- Create: `handler/course_settings_test.go`

- [ ] **Step 1: Write the failing tests**

Create `handler/course_settings_test.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"study-app/agent"
)

func TestGetCourseSettingsReturnsDefaults(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/courses/settings?course_id=ce297", nil)
	rr := httptest.NewRecorder()
	h.handleCourseSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var s agent.CourseSettings
	if err := json.NewDecoder(rr.Body).Decode(&s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.ChunkPages != 8 || !s.StopAfterTask || !s.Interleaving {
		t.Fatalf("unexpected defaults: %+v", s)
	}
}

func TestPutCourseSettingsPersistsAndValidates(t *testing.T) {
	h := newTestHandler(t)
	body := `{"course_id":"ce297","framing":"exam-prep first","exam_style":"oral","chunk_pages":6,"stop_after_task":false,"interleaving":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/courses/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handleCourseSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body=%s", rr.Code, rr.Body.String())
	}

	got, err := h.App.GetCourseSettings("ce297")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Framing != "exam-prep first" || got.ChunkPages != 6 || got.StopAfterTask || got.Interleaving {
		t.Fatalf("not persisted: %+v", got)
	}
}

func TestPutCourseSettingsRejectsBadChunk(t *testing.T) {
	h := newTestHandler(t)
	body := `{"course_id":"ce297","chunk_pages":999}`
	req := httptest.NewRequest(http.MethodPut, "/api/courses/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handleCourseSettings(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/eduardohiroji/Documents/ITA/claw-study && go test ./handler/ -run 'CourseSettings' -v`
Expected: FAIL — `h.handleCourseSettings undefined`.

- [ ] **Step 3: Implement the handler**

Create `handler/course_settings.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"

	"study-app/agent"
)

// handleCourseSettings serves GET (read, ?course_id=) and PUT (write, JSON
// body) for per-course Steering settings. The same validated write path the
// claw-cli tool uses (ADR 0010/0016).
func (h *Handler) handleCourseSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		courseID := r.URL.Query().Get("course_id")
		if courseID == "" {
			writeError(w, http.StatusBadRequest, "course_id is required")
			return
		}
		s, err := h.App.GetCourseSettings(courseID)
		if err != nil {
			writeServerError(w, "get course settings", err)
			return
		}
		writeJSON(w, http.StatusOK, s)

	case http.MethodPut:
		var s agent.CourseSettings
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if s.CourseID == "" {
			s.CourseID = r.URL.Query().Get("course_id")
		}
		if s.CourseID == "" {
			writeError(w, http.StatusBadRequest, "course_id is required")
			return
		}
		if err := agent.ValidateCourseSettings(s); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.App.UpsertCourseSettings(s); err != nil {
			writeServerError(w, "save course settings", err)
			return
		}
		out, _ := h.App.GetCourseSettings(s.CourseID)
		writeJSON(w, http.StatusOK, out)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
```

- [ ] **Step 4: Register the route**

In `handler/handler.go`, after line 55 (`mux.HandleFunc("/api/courses", h.handleCourses)`), add:

```go
	mux.HandleFunc("/api/courses/settings", h.handleCourseSettings)
```

(Register `/api/courses/settings` — the exact path — so it does not collide with the `/api/courses` exact-match route.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/eduardohiroji/Documents/ITA/claw-study && go test ./handler/ -run 'CourseSettings' -v`
Expected: PASS (all three).

- [ ] **Step 6: Commit**

```bash
cd /Users/eduardohiroji/Documents/ITA/claw-study
git add handler/course_settings.go handler/course_settings_test.go handler/handler.go
git commit -m "feat(settings): GET/PUT /api/courses/settings"
```

---

## Task 6: Frontend — rail ⚙ button + settings modal

**Files:**
- Create: `static/settings.js`
- Modify: `static/rail.js` (import ~line 4; `renderCourseSwitcher` ~124-132; `initRail` ~162-169)
- Modify: `static/style.css` (append modal styles)

- [ ] **Step 1: Create the settings modal module**

Create `static/settings.js`:

```javascript
// settings.js — Course Steering settings modal (ADR 0010/0016).
// Reads GET /api/courses/settings?course_id=, writes PUT on save.
import { apiFetch } from './apiFetch.js';

function escapeHtml(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, (c) =>
    ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

export async function openCourseSettings(courseId) {
  let s;
  try {
    const resp = await apiFetch(`/api/courses/settings?course_id=${encodeURIComponent(courseId)}`);
    if (!resp.ok) throw new Error('HTTP ' + resp.status);
    s = await resp.json();
  } catch (err) {
    alert('Could not load course settings: ' + err.message);
    return;
  }
  renderModal(courseId, s);
}

function renderModal(courseId, s) {
  const overlay = document.createElement('div');
  overlay.className = 'settings-overlay';
  overlay.innerHTML = `
    <div class="settings-modal" role="dialog" aria-modal="true">
      <h2>Course settings — ${escapeHtml(courseId)}</h2>
      <label class="settings-field">Framing / goal
        <textarea id="set-framing" rows="3" placeholder="e.g. exam-prep first, conceptual exam">${escapeHtml(s.framing)}</textarea>
      </label>
      <label class="settings-field">Exam style
        <textarea id="set-exam" rows="2" placeholder="e.g. conceptual oral, problem sets">${escapeHtml(s.exam_style)}</textarea>
      </label>
      <label class="settings-field">Reading chunk size (pages)
        <input id="set-chunk" type="number" min="3" max="30" value="${Number(s.chunk_pages)}">
      </label>
      <label class="settings-check"><input id="set-stop" type="checkbox" ${s.stop_after_task ? 'checked' : ''}> Stop after each task</label>
      <label class="settings-check"><input id="set-inter" type="checkbox" ${s.interleaving ? 'checked' : ''}> Interleave older tasks at session open</label>
      <div id="set-error" class="settings-error"></div>
      <div class="settings-actions">
        <button id="set-cancel">Cancel</button>
        <button id="set-save" class="primary">Save</button>
      </div>
    </div>`;
  document.body.appendChild(overlay);

  const close = () => overlay.remove();
  overlay.querySelector('#set-cancel').addEventListener('click', close);
  overlay.addEventListener('click', (e) => { if (e.target === overlay) close(); });

  overlay.querySelector('#set-save').addEventListener('click', async () => {
    const payload = {
      course_id: courseId,
      framing: overlay.querySelector('#set-framing').value,
      exam_style: overlay.querySelector('#set-exam').value,
      chunk_pages: parseInt(overlay.querySelector('#set-chunk').value, 10),
      stop_after_task: overlay.querySelector('#set-stop').checked,
      interleaving: overlay.querySelector('#set-inter').checked,
    };
    try {
      const r = await apiFetch('/api/courses/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      if (!r.ok) {
        const e = await r.json().catch(() => ({}));
        overlay.querySelector('#set-error').textContent = e.error || ('Save failed (HTTP ' + r.status + ')');
        return;
      }
      close();
    } catch (err) {
      overlay.querySelector('#set-error').textContent = 'Save failed: ' + err.message;
    }
  });
}
```

- [ ] **Step 2: Wire the ⚙ button into the rail**

In `static/rail.js`, add the import near the top (after the existing `import { courseMeta, ... }` on line ~4):

```javascript
import { openCourseSettings } from './settings.js';
```

Replace `renderCourseSwitcher` (lines ~124-132) with:

```javascript
function renderCourseSwitcher() {
  let opts = '';
  for (const id of Object.keys(courseMeta)) {
    if (id === '') continue; // skip the "General" pseudo-course in the switcher
    const sel = id === selectedCourse ? ' selected' : '';
    opts += `<option value="${escapeHtml(id)}"${sel}>${escapeHtml(courseMeta[id].name)}</option>`;
  }
  return `<div class="rail-course-row">` +
    `<select id="rail-course-select" class="rail-course-select" data-action="noop">${opts}</select>` +
    `<button class="rail-settings-btn" data-action="open-settings" title="Course settings" aria-label="Course settings">&#9881;</button>` +
    `</div>`;
}
```

In `initRail` (lines ~162-169), add a click listener alongside the existing `change` listener:

```javascript
export function initRail() {
  // Course switcher: change event (not a click data-action) swaps the rail.
  document.getElementById('session-list').addEventListener('change', (e) => {
    const sel = e.target.closest('#rail-course-select');
    if (!sel) return;
    selectCourse(sel.value);
  });
  // Course settings gear.
  document.getElementById('session-list').addEventListener('click', (e) => {
    const btn = e.target.closest('[data-action="open-settings"]');
    if (!btn) return;
    e.stopPropagation();
    if (selectedCourse) openCourseSettings(selectedCourse);
  });
}
```

- [ ] **Step 3: Add modal styles**

Append to `static/style.css`:

```css
/* ---------- Course settings (Steering) ---------- */
.rail-course-row { display: flex; align-items: center; gap: 6px; }
.rail-course-row .rail-course-select { flex: 1; }
.rail-settings-btn {
  flex: 0 0 auto; background: none; border: none; cursor: pointer;
  font-size: 16px; line-height: 1; padding: 4px 6px; color: inherit; opacity: 0.7;
}
.rail-settings-btn:hover { opacity: 1; }

.settings-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,0.45);
  display: flex; align-items: center; justify-content: center; z-index: 1000;
}
.settings-modal {
  background: var(--bg, #fff); color: inherit; width: min(440px, 92vw);
  max-height: 88vh; overflow-y: auto; padding: 20px 22px; border-radius: 10px;
  box-shadow: 0 12px 40px rgba(0,0,0,0.3); display: flex; flex-direction: column; gap: 12px;
}
.settings-modal h2 { margin: 0 0 4px; font-size: 1.1rem; }
.settings-field { display: flex; flex-direction: column; gap: 4px; font-size: 0.85rem; }
.settings-field textarea, .settings-field input[type="number"] {
  font: inherit; padding: 6px 8px; border: 1px solid rgba(0,0,0,0.2); border-radius: 6px; width: 100%;
}
.settings-check { display: flex; align-items: center; gap: 8px; font-size: 0.9rem; }
.settings-error { color: #b91c1c; font-size: 0.8rem; min-height: 1em; }
.settings-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 4px; }
.settings-actions button { font: inherit; padding: 6px 14px; border-radius: 6px; cursor: pointer; border: 1px solid rgba(0,0,0,0.2); background: transparent; }
.settings-actions button.primary { background: #2563eb; color: #fff; border-color: #2563eb; }
```

- [ ] **Step 4: Browser-verify via headless Chrome CDP**

Build and run a local preview (recipe from Phase 3b — `loadConfig` requires an LLM key even locally; empty `AUTH_TOKEN` disables the gate; static is `//go:embed`-ed so a rebuild is required after JS/CSS changes):

```bash
cd /Users/eduardohiroji/Documents/ITA/claw-study
rm -rf /tmp/claw-set-vault && mkdir -p /tmp/claw-set-vault
VAULT_ROOT=/tmp/claw-set-vault LISTEN_ADDR=127.0.0.1:8099 AUTH_TOKEN= LLM_API_KEY=dummy AGENT_RUNTIME=pi go run . &
sleep 2
```

Launch headless Chrome and drive it (reuse `/tmp/cdp.mjs` if present, else a minimal `Runtime.evaluate`). Verify, via DOM eval against `http://127.0.0.1:8099`:
1. The rail course switcher row contains a `.rail-settings-btn`.
2. Clicking it (`document.querySelector('.rail-settings-btn').click()`) appends a `.settings-overlay` with the five fields.
3. Setting `#set-chunk` to `6`, unchecking `#set-stop`, clicking `#set-save` closes the overlay, and a follow-up `fetch('/api/courses/settings?course_id=<the selected course>')` returns `chunk_pages: 6, stop_after_task: false`.

Expected: all three hold. (No PDF is involved here, so the Phase-3b `openPdf` alert gotcha does not apply.) Stop the server when done: `kill %1`.

- [ ] **Step 5: Commit**

```bash
cd /Users/eduardohiroji/Documents/ITA/claw-study
git add static/settings.js static/rail.js static/style.css
git commit -m "feat(settings): rail gear + course settings modal"
```

---

## Task 7: Full verification + deploy both binaries

**Files:** none (build/deploy only)

- [ ] **Step 1: Full test suite + build + vet**

Run: `cd /Users/eduardohiroji/Documents/ITA/claw-study && go build ./... && go vet ./... && go test ./...`
Expected: build clean, vet clean, all tests PASS.

- [ ] **Step 2: Deploy `study-app`**

Follow the deploy cheat sheet in the `claw-study service` memory. Build the linux binary, back up the live one (`study-app.bak.2026-05-30-phase4-steering`), scp, restart `study-app.service` (remember `export XDG_RUNTIME_DIR=/run/user/$(id -u)` over SSH).

- [ ] **Step 3: Deploy `claw-cli` (REQUIRED — this phase changed it)**

```bash
cd /Users/eduardohiroji/Documents/ITA/claw-study
go build -o /tmp/claw-cli-linux ./claw-cli
```

Back up the live `bin/claw-cli` (`claw-cli.bak.2026-05-30-phase4-steering`), scp `/tmp/claw-cli-linux` to `~/stack/study-app/bin/claw-cli`.

- [ ] **Step 4: Sync the edited skill (disk-mounted, not in the binary)**

scp `skills/study-step-complete/SKILL.md` to `~/stack/study-app/skills/study-step-complete/SKILL.md`.

- [ ] **Step 5: Verify live**

- `systemctl --user is-active study-app.service study-app-tunnel.service` → two `active`.
- Public health returns 401 (bearer auth healthy).
- On the VPS: `bin/claw-cli course settings get --course ce297` prints defaults (`chunk_pages: 8`, `stop_after_task: true`).
- In the browser at `https://study.claw-study.xyz`, open a course's ⚙, set chunk size to 6, save; reopen to confirm it stuck.
- Start a study session on that course and confirm the tutor's reading chunking reflects 6 pages (the `<reading_state>` flow) — i.e. AGENTS.md picked up the setting.

- [ ] **Step 6: Final commit (if any deploy notes/scripts changed)**

```bash
cd /Users/eduardohiroji/Documents/ITA/claw-study
git status   # commit any deploy-script tweaks; code is already committed per-task
```

---

## Self-Review

**1. Spec/decision coverage** (against the grilled design + ADR 0010/0016):
- Scope = declarative knobs only → Tasks 1/5/6 cover framing, exam_style, chunk_pages, stop_after_task, interleaving; no plan-editing. ✓
- Persistence = dedicated `course_settings` typed table, lazy defaults → Task 1. ✓
- Two write paths, one validated function → `UpsertCourseSettings`/`ValidateCourseSettings` shared by handler (Task 5) and `SetCourseSetting` (Task 4). ✓
- Read path in `sandbox.go`, rules parameterized in-place, skill defers to flag → Tasks 2 + 3. ✓
- Defaults = current behavior (chunk 8, stop on, interleave on) so migration is behavior-preserving → `DefaultCourseSettings` + Task 2 default test. ✓
- Agent tool = generic `set_course_setting` (claw-cli `course settings set`), any surface one-shot → Task 4 + the "Course settings (Steering)" AGENTS.md section in Task 2 Step 5. ✓
- UI = rail ⚙ → modal → Task 6. ✓
- "Nothing to remove" finding: no task removes agent config-writing code, because none exists; Task 2's tool section is the only agent-facing addition. ✓
- Deploy rebuilds both binaries + syncs skill → Task 7 Steps 2-4. ✓

**2. Placeholder scan:** No "TBD"/"add validation"/"similar to" — every code step shows full code; every test step shows assertions; every run step shows command + expected. ✓

**3. Type consistency:** `CourseSettings` fields (`CourseID/Framing/ExamStyle/ChunkPages/StopAfterTask/Interleaving/UpdatedAt`) and JSON tags (`course_id/framing/exam_style/chunk_pages/stop_after_task/interleaving`) are identical across Tasks 1, 4, 5, 6. Methods `GetCourseSettings`/`UpsertCourseSettings`/`SetCourseSetting`/`ValidateCourseSettings`/`DefaultCourseSettings` named consistently in every caller. `Sandbox.Settings` type `func(string) CourseSettings` matches the wiring in `NewApp` and the test injections. ✓
