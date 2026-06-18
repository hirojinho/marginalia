# Phase 1 — `claw-cli` skeleton + memory subcommands — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a `claw-cli` Go binary with `memory load|save|search` subcommands, an `agent_memory` SQLite table, and a `seed-memory` one-shot importer that pulls Eduardo's existing memory store from `~/.claude/projects/<project-slug>/` into the database. End state: `claw-cli memory load --course ce297` produces a ≤3 KB AGENTS.md ready for Pi to consume.

**Architecture:** New table `agent_memory(id, user_id, course_id, kind, title, body, created_at, updated_at)` added inline to `InitSchema` in `agent/db.go`. New `MemoryStore` struct in `agent/memory.go` with `Save / Search / LoadByScope` methods on `*sql.DB` (does not depend on the heavyweight `*App`). New binary `claw-cli/main.go` (package `main`, parallel to existing `convert/main.go`) using stdlib `flag` for subcommand dispatch. AGENTS.md assembly reads from the DB plus the existing `sessions` table for recent activity, plus optional `skills/` dir frontmatter. Seed binary `seed-memory/main.go` walks the source memory dir, parses YAML frontmatter, maps `type` → `kind`, derives `course_id` from path (`courses/<id>/...` → `<id>`, else `NULL`), and reseeds idempotently. Inline migration pattern matches the existing `db.go`.

**Tech Stack:** Go 1.24.1, `modernc.org/sqlite`, stdlib `flag` / `encoding/json` / `gopkg.in/yaml.v3`-equivalent (use a tiny hand-rolled frontmatter parser to avoid adding a dep — frontmatter format is fixed and simple). Tests via `testing` + `t.TempDir()` + `:memory:` DBs (existing pattern in `agent/db_test.go`).

**Design decisions locked here (do not relitigate during implementation):**

1. **CLI framework:** stdlib `flag`, manual subcommand dispatch. No new deps.
2. **Migration:** append `CREATE TABLE IF NOT EXISTS agent_memory ...` to `InitSchema`'s schema string.
3. **`memory search` backend:** SQLite `LIKE '%q%'` over `title || ' ' || body`, ordered by `updated_at DESC`, capped at 20.
4. **Skill list when `skills/` empty/missing:** emit `_(none yet)_`.
5. **Recency slice:** last 2 rows from `sessions` `WHERE course_id=? ORDER BY updated_at DESC LIMIT 2`. Topic + first 200 chars of summary.
6. **JSON shapes:** `save` → `{"id":N,"kind":"...","title":"..."}`; `search` → `{"results":[{"id":N,"kind":"...","course_id":"...","title":"...","snippet":"..."}]}`; `load` writes plain markdown to stdout.
7. **Seed form:** standalone `seed-memory` Go binary; idempotent (`DELETE FROM agent_memory WHERE user_id=?` then re-insert). Only imports `kind IN ('user','feedback')` plus everything under `courses/<id>/`. Skips `project`/`reference` files at the top level (ops/meta noise).
8. **AGENTS.md template:** five `## ` sections in fixed order. Per-section character caps: profile 500, course 800, feedback 1200, recent 500, skills 500. Total cap 3072 bytes. If over, drop sections from the bottom (skills first, then recent).
9. **`user_id`:** hardcoded `"eduardo"` for v1.
10. **Timestamps:** `created_at` and `updated_at` as Unix epoch seconds (`INTEGER`).
11. **Frontmatter parser:** hand-rolled, accepts only top-of-file `---\n<key>: <value>\n...\n---\n` block with simple `key: value` lines. No nesting, no arrays, no quoting. Sufficient for the existing memory file format.

---

## File structure

| File | Status | Responsibility |
|---|---|---|
| `agent/db.go` | modify (lines 47-81 schema string) | Append `agent_memory` table to `InitSchema` |
| `agent/db_test.go` | modify | Add test that `agent_memory` table exists after `InitSchema` |
| `agent/memory.go` | **create** | `MemoryStore` + `Memory` type + `Save`/`Search`/`LoadByScope`/recency-helper/skill-frontmatter-parser/AGENTS.md assembler |
| `agent/memory_test.go` | **create** | Unit tests: store CRUD, scope filter, AGENTS.md cap, empty case, recency slice, skill parser |
| `claw-cli/main.go` | **create** | Subcommand dispatcher + `memory save|search|load` wiring |
| `claw-cli/main_test.go` | **create** | End-to-end tests via `os/exec`-style subcommand-table tests |
| `seed-memory/main.go` | **create** | Walk source memory dir, parse frontmatter, reseed |
| `seed-memory/main_test.go` | **create** | Frontmatter parser + filename→course_id derivation tests |

---

## Task 1 — `agent_memory` schema in `InitSchema`

**Files:**
- Modify: `agent/db.go` lines 47-81 (the `schema` string in `InitSchema`)
- Test: `agent/db_test.go` (append new test)

- [ ] **Step 1: Write the failing test** in `agent/db_test.go` (append at end of file)

```go
func TestInitSchemaCreatesAgentMemoryTable(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	var name string
	row := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='agent_memory'`)
	if err := row.Scan(&name); err != nil {
		t.Fatalf("agent_memory table missing: %v", err)
	}
	if name != "agent_memory" {
		t.Fatalf("got %q, want agent_memory", name)
	}
	// Index too
	row = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='agent_memory_scope'`)
	if err := row.Scan(&name); err != nil {
		t.Fatalf("agent_memory_scope index missing: %v", err)
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

```
cd ~/Documents/ITA/claw-study
go test ./agent -run TestInitSchemaCreatesAgentMemoryTable -v
```

Expected: FAIL — `agent_memory table missing`.

- [ ] **Step 3: Add the table to `InitSchema`'s schema string** in `agent/db.go`. After the `messages` table block (line 80, before the closing backtick), append:

```sql
	CREATE TABLE IF NOT EXISTS agent_memory (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id     TEXT NOT NULL,
		course_id   TEXT,
		kind        TEXT NOT NULL,
		title       TEXT,
		body        TEXT NOT NULL,
		created_at  INTEGER NOT NULL,
		updated_at  INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS agent_memory_scope ON agent_memory (user_id, course_id, kind);
```

- [ ] **Step 4: Run tests, verify pass**

```
go test ./agent -run TestInitSchema -v
```

Expected: PASS for the new test and any pre-existing schema tests.

- [ ] **Step 5: Run the full agent test suite to verify no regression**

```
go test ./agent -v
```

Expected: all previously-passing tests still pass.

- [ ] **Step 6: Commit**

```
git -c user.email=you@example.com -c user.name=your-name \
  add agent/db.go agent/db_test.go
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "agent: add agent_memory table to InitSchema"
```

---

## Task 2 — `MemoryStore` save / search / scope-load

**Files:**
- Create: `agent/memory.go`
- Test: `agent/memory_test.go`

- [ ] **Step 1: Write the failing tests** at `agent/memory_test.go`

```go
package agent

import (
	"strings"
	"testing"
)

func newMemoryDB(t *testing.T) *MemoryStore {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	return NewMemoryStore(db)
}

func TestMemoryStoreSaveAssignsID(t *testing.T) {
	store := newMemoryDB(t)
	saved, err := store.Save(Memory{
		UserID:   "eduardo",
		CourseID: "ce297",
		Kind:     "feedback",
		Title:    "no abbreviations",
		Body:     "spell out terms",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.ID == 0 {
		t.Fatalf("expected non-zero id")
	}
	if saved.CreatedAt == 0 || saved.UpdatedAt == 0 {
		t.Fatalf("expected timestamps set, got %+v", saved)
	}
}

func TestMemoryStoreSearchMatchesTitleAndBody(t *testing.T) {
	store := newMemoryDB(t)
	for _, m := range []Memory{
		{UserID: "eduardo", Kind: "feedback", Title: "abbreviations", Body: "spell out"},
		{UserID: "eduardo", Kind: "feedback", Title: "density", Body: "match existing density"},
		{UserID: "eduardo", Kind: "profile", Title: "user", Body: "Eduardo studies safety"},
	} {
		if _, err := store.Save(m); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	results, err := store.Search("eduardo", "density", "", 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].Title != "density" {
		t.Fatalf("got %+v", results)
	}
	results, err = store.Search("eduardo", "safety", "", 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].Kind != "profile" {
		t.Fatalf("got %+v", results)
	}
}

func TestMemoryStoreSearchCourseFilter(t *testing.T) {
	store := newMemoryDB(t)
	for _, m := range []Memory{
		{UserID: "eduardo", CourseID: "ce297", Kind: "feedback", Title: "ce297-rule", Body: "x"},
		{UserID: "eduardo", CourseID: "dsa-interview", Kind: "feedback", Title: "dsa-rule", Body: "x"},
		{UserID: "eduardo", Kind: "feedback", Title: "global-rule", Body: "x"},
	} {
		if _, err := store.Save(m); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	results, err := store.Search("eduardo", "rule", "ce297", 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	titles := []string{}
	for _, r := range results {
		titles = append(titles, r.Title)
	}
	// Course filter: ce297 OR NULL only
	if got := strings.Join(titles, ","); !strings.Contains(got, "ce297-rule") || !strings.Contains(got, "global-rule") || strings.Contains(got, "dsa-rule") {
		t.Fatalf("course filter wrong, got titles: %s", got)
	}
}

func TestMemoryStoreLoadByScope(t *testing.T) {
	store := newMemoryDB(t)
	mems := []Memory{
		{UserID: "eduardo", Kind: "profile", Title: "user", Body: "PROFILE_BODY"},
		{UserID: "eduardo", CourseID: "ce297", Kind: "project", Title: "ce297-context", Body: "COURSE_BODY"},
		{UserID: "eduardo", Kind: "feedback", Title: "global-style", Body: "FB_GLOBAL"},
		{UserID: "eduardo", CourseID: "ce297", Kind: "feedback", Title: "ce297-style", Body: "FB_COURSE"},
		{UserID: "eduardo", CourseID: "dsa-interview", Kind: "feedback", Title: "dsa-style", Body: "FB_DSA"},
	}
	for _, m := range mems {
		if _, err := store.Save(m); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	scope, err := store.LoadByScope("eduardo", "ce297")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if scope.Profile == nil || !strings.Contains(scope.Profile.Body, "PROFILE_BODY") {
		t.Fatalf("profile missing: %+v", scope.Profile)
	}
	if len(scope.CourseProjects) != 1 || scope.CourseProjects[0].Body != "COURSE_BODY" {
		t.Fatalf("course projects: %+v", scope.CourseProjects)
	}
	// Feedback: course-scoped + global, NOT dsa
	if len(scope.Feedback) != 2 {
		t.Fatalf("expected 2 feedback rows, got %+v", scope.Feedback)
	}
}
```

- [ ] **Step 2: Run tests, verify they fail to compile** (`MemoryStore` undefined)

```
go test ./agent -run TestMemoryStore -v
```

Expected: FAIL — undefined `MemoryStore`, `Memory`, `NewMemoryStore`.

- [ ] **Step 3: Create `agent/memory.go`** with the minimal implementation

```go
package agent

import (
	"database/sql"
	"fmt"
	"time"
)

// Memory is one row in agent_memory. Title may be empty.
type Memory struct {
	ID        int64
	UserID    string
	CourseID  string // empty string == NULL in DB
	Kind      string // "profile" | "feedback" | "project" | "reference"
	Title     string
	Body      string
	CreatedAt int64 // Unix seconds
	UpdatedAt int64
}

// Scope is the set of memories relevant to a single agent turn,
// already filtered by user + course.
type Scope struct {
	Profile        *Memory
	CourseProjects []Memory // kind='project' AND course_id=?
	Feedback       []Memory // kind='feedback' AND (course_id=? OR course_id IS NULL)
}

type MemoryStore struct {
	db *sql.DB
}

func NewMemoryStore(db *sql.DB) *MemoryStore {
	return &MemoryStore{db: db}
}

// Save inserts a new memory row. Returns the row with ID + timestamps populated.
func (s *MemoryStore) Save(m Memory) (Memory, error) {
	now := time.Now().Unix()
	if m.CreatedAt == 0 {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	var courseID any
	if m.CourseID != "" {
		courseID = m.CourseID
	}
	res, err := s.db.Exec(
		`INSERT INTO agent_memory (user_id, course_id, kind, title, body, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.UserID, courseID, m.Kind, m.Title, m.Body, m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		return Memory{}, fmt.Errorf("memory save: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Memory{}, fmt.Errorf("memory save: last id: %w", err)
	}
	m.ID = id
	return m, nil
}

// Search runs a LIKE scan over title+body. courseID="" means no course filter
// (returns rows from all courses + global). Otherwise restricts to course OR NULL.
func (s *MemoryStore) Search(userID, query, courseID string, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 20
	}
	pattern := "%" + query + "%"
	var rows *sql.Rows
	var err error
	if courseID == "" {
		rows, err = s.db.Query(
			`SELECT id, user_id, IFNULL(course_id,''), kind, IFNULL(title,''), body, created_at, updated_at
			 FROM agent_memory
			 WHERE user_id = ? AND (title LIKE ? OR body LIKE ?)
			 ORDER BY updated_at DESC LIMIT ?`,
			userID, pattern, pattern, limit,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, user_id, IFNULL(course_id,''), kind, IFNULL(title,''), body, created_at, updated_at
			 FROM agent_memory
			 WHERE user_id = ? AND (course_id = ? OR course_id IS NULL)
			   AND (title LIKE ? OR body LIKE ?)
			 ORDER BY updated_at DESC LIMIT ?`,
			userID, courseID, pattern, pattern, limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("memory search: %w", err)
	}
	defer rows.Close()
	out := []Memory{}
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.UserID, &m.CourseID, &m.Kind, &m.Title, &m.Body, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("memory scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// LoadByScope returns the curated set of memories for AGENTS.md assembly.
func (s *MemoryStore) LoadByScope(userID, courseID string) (Scope, error) {
	var scope Scope

	// Profile (always one global row, kind='profile')
	row := s.db.QueryRow(
		`SELECT id, user_id, IFNULL(course_id,''), kind, IFNULL(title,''), body, created_at, updated_at
		 FROM agent_memory
		 WHERE user_id = ? AND kind = 'profile'
		 ORDER BY updated_at DESC LIMIT 1`,
		userID,
	)
	var p Memory
	if err := row.Scan(&p.ID, &p.UserID, &p.CourseID, &p.Kind, &p.Title, &p.Body, &p.CreatedAt, &p.UpdatedAt); err == nil {
		scope.Profile = &p
	} else if err != sql.ErrNoRows {
		return scope, fmt.Errorf("scope profile: %w", err)
	}

	// Course-scoped projects
	if courseID != "" {
		rows, err := s.db.Query(
			`SELECT id, user_id, IFNULL(course_id,''), kind, IFNULL(title,''), body, created_at, updated_at
			 FROM agent_memory
			 WHERE user_id = ? AND course_id = ? AND kind = 'project'
			 ORDER BY updated_at DESC`,
			userID, courseID,
		)
		if err != nil {
			return scope, fmt.Errorf("scope course: %w", err)
		}
		for rows.Next() {
			var m Memory
			if err := rows.Scan(&m.ID, &m.UserID, &m.CourseID, &m.Kind, &m.Title, &m.Body, &m.CreatedAt, &m.UpdatedAt); err != nil {
				rows.Close()
				return scope, fmt.Errorf("scope course scan: %w", err)
			}
			scope.CourseProjects = append(scope.CourseProjects, m)
		}
		rows.Close()
	}

	// Feedback: course-scoped OR global
	var fbRows *sql.Rows
	var err error
	if courseID == "" {
		fbRows, err = s.db.Query(
			`SELECT id, user_id, IFNULL(course_id,''), kind, IFNULL(title,''), body, created_at, updated_at
			 FROM agent_memory WHERE user_id = ? AND kind = 'feedback' AND course_id IS NULL
			 ORDER BY updated_at DESC`,
			userID,
		)
	} else {
		fbRows, err = s.db.Query(
			`SELECT id, user_id, IFNULL(course_id,''), kind, IFNULL(title,''), body, created_at, updated_at
			 FROM agent_memory WHERE user_id = ? AND kind = 'feedback'
			   AND (course_id = ? OR course_id IS NULL)
			 ORDER BY updated_at DESC`,
			userID, courseID,
		)
	}
	if err != nil {
		return scope, fmt.Errorf("scope feedback: %w", err)
	}
	defer fbRows.Close()
	for fbRows.Next() {
		var m Memory
		if err := fbRows.Scan(&m.ID, &m.UserID, &m.CourseID, &m.Kind, &m.Title, &m.Body, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return scope, fmt.Errorf("scope feedback scan: %w", err)
		}
		scope.Feedback = append(scope.Feedback, m)
	}
	return scope, fbRows.Err()
}
```

- [ ] **Step 4: Run tests, verify they pass**

```
go test ./agent -run TestMemoryStore -v
```

Expected: 4 tests pass.

- [ ] **Step 5: Commit**

```
git -c user.email=you@example.com -c user.name=your-name \
  add agent/memory.go agent/memory_test.go
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "agent: add MemoryStore with save/search/load-by-scope"
```

---

## Task 3 — Recent-sessions helper + skill-frontmatter parser + AGENTS.md assembler

**Files:**
- Modify: `agent/memory.go` (append helpers + assembler)
- Modify: `agent/memory_test.go` (append tests)

- [ ] **Step 1: Write the failing tests** at the end of `agent/memory_test.go`

```go
func TestRecentSessionsForCourse(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	// Seed a few sessions
	now := time.Now().Format(time.RFC3339)
	for _, s := range []struct{ course, topic, summary string }{
		{"ce297", "STAMP intro", "Read Leveson ch 4."},
		{"ce297", "STPA step 2", "Hazards enumerated for elevator example."},
		{"ce297", "STPA step 3", "Control structure drafted."},
		{"dsa-interview", "Bus Routes", "BFS over stop->buses graph."},
	} {
		if _, err := db.Exec(
			`INSERT INTO sessions (course_id, topic, created_at, updated_at, summary, summary_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			s.course, s.topic, now, now, s.summary, time.Now().Unix(),
		); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	got, err := RecentSessionsForCourse(db, "ce297", 2)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	// Most recent first
	if got[0].Topic != "STPA step 3" {
		t.Fatalf("expected most recent first, got %q", got[0].Topic)
	}
}

func TestParseSkillFrontmatter(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`---
name: study-notes
description: Use when finishing a reading task
---
body here
`)
	skillDir := dir + "/study-notes"
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(skillDir+"/SKILL.md", content, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	skills, err := ParseSkillsDir(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(skills) != 1 || skills[0].Name != "study-notes" || !strings.Contains(skills[0].Description, "finishing") {
		t.Fatalf("got %+v", skills)
	}
}

func TestParseSkillsDirMissingReturnsEmpty(t *testing.T) {
	skills, err := ParseSkillsDir("/no/such/path")
	if err != nil {
		t.Fatalf("expected nil err for missing dir, got %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected empty, got %+v", skills)
	}
}

func TestAssembleAgentsMDIncludesAllSections(t *testing.T) {
	scope := Scope{
		Profile:        &Memory{Body: "Eduardo is an ITA master's student."},
		CourseProjects: []Memory{{Title: "Course arc", Body: "STAMP vs Avizienis."}},
		Feedback:       []Memory{{Title: "no abbreviations", Body: "spell out terms"}},
	}
	recent := []SessionDigest{{Topic: "STPA step 3", Summary: "Control structure drafted."}}
	skills := []SkillMeta{{Name: "study-notes", Description: "Use when finishing a reading task"}}
	out := AssembleAgentsMD(scope, recent, skills, "ce297")
	for _, want := range []string{
		"## User profile", "ITA master's student",
		"## Course context: ce297", "STAMP",
		"## Active feedback rules", "no abbreviations",
		"## Recent sessions", "STPA step 3",
		"## Available skills", "study-notes",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
	if len(out) > 3072 {
		t.Fatalf("over cap: %d bytes", len(out))
	}
}

func TestAssembleAgentsMDEmpty(t *testing.T) {
	out := AssembleAgentsMD(Scope{}, nil, nil, "")
	if !strings.Contains(out, "_(none yet)_") {
		t.Fatalf("expected fallback skill section, got:\n%s", out)
	}
}

func TestAssembleAgentsMDDropsBottomSectionsWhenOverCap(t *testing.T) {
	huge := strings.Repeat("x", 4000)
	scope := Scope{
		Profile:  &Memory{Body: huge},        // will get truncated to 500
		Feedback: []Memory{{Body: huge}},     // will get truncated to 1200
	}
	recent := []SessionDigest{{Topic: "Recent", Summary: huge}}
	skills := []SkillMeta{{Name: "alpha", Description: huge}}
	out := AssembleAgentsMD(scope, recent, skills, "ce297")
	if len(out) > 3072 {
		t.Fatalf("over cap: %d bytes", len(out))
	}
	// Profile + feedback survive (top sections); recent and skills can drop
	if !strings.Contains(out, "## User profile") {
		t.Fatalf("profile dropped, must survive")
	}
}
```

You will need to add `os` and `time` to the test imports.

- [ ] **Step 2: Run tests, verify they fail**

```
go test ./agent -run "TestRecentSessions|TestParseSkill|TestAssembleAgents" -v
```

Expected: FAIL — undefined `RecentSessionsForCourse`, `ParseSkillsDir`, `AssembleAgentsMD`, `SessionDigest`, `SkillMeta`.

- [ ] **Step 3: Append to `agent/memory.go`**

```go
import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)
// (merge with existing imports — keep alphabetical order)

// SessionDigest is a topic+summary pair used in the AGENTS.md recent slice.
type SessionDigest struct {
	Topic   string
	Summary string
}

// RecentSessionsForCourse returns the most recent N sessions for a course,
// newest first. Summary is truncated to 200 chars.
func RecentSessionsForCourse(db *sql.DB, courseID string, limit int) ([]SessionDigest, error) {
	if limit <= 0 {
		limit = 2
	}
	rows, err := db.Query(
		`SELECT topic, summary FROM sessions
		 WHERE course_id = ?
		 ORDER BY updated_at DESC LIMIT ?`,
		courseID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("recent sessions: %w", err)
	}
	defer rows.Close()
	out := []SessionDigest{}
	for rows.Next() {
		var d SessionDigest
		if err := rows.Scan(&d.Topic, &d.Summary); err != nil {
			return nil, fmt.Errorf("recent sessions scan: %w", err)
		}
		if len(d.Summary) > 200 {
			d.Summary = d.Summary[:200] + "…"
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SkillMeta is the minimal frontmatter needed for AGENTS.md.
type SkillMeta struct {
	Name        string
	Description string
}

// ParseSkillsDir walks dir/<skill>/SKILL.md, parses frontmatter,
// returns sorted skill list. Missing dir returns empty slice (not an error).
func ParseSkillsDir(dir string) ([]SkillMeta, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("skills dir: %w", err)
	}
	var out []SkillMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name(), "SKILL.md")
		fm, err := parseFrontmatter(path)
		if err != nil {
			continue // skip skills without parseable frontmatter
		}
		out = append(out, SkillMeta{Name: fm["name"], Description: fm["description"]})
	}
	return out, nil
}

// parseFrontmatter reads a markdown file and returns the YAML-ish
// "key: value" pairs from the leading `---` block. No nesting.
func parseFrontmatter(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return nil, fmt.Errorf("no frontmatter in %s", path)
	}
	out := map[string]string{}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			return out, nil
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		out[key] = val
	}
	return nil, fmt.Errorf("unterminated frontmatter in %s", path)
}

// AGENTS.md assembly — fixed section order with hard caps + bottom-drop overflow strategy.
const (
	agentsMDTotalCap = 3072
	capProfile       = 500
	capCourse        = 800
	capFeedback      = 1200
	capRecent        = 500
	capSkills        = 500
)

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// AssembleAgentsMD builds the AGENTS.md text. Drops sections from the bottom
// (skills first, then recent) when the total exceeds the 3072-byte cap.
func AssembleAgentsMD(scope Scope, recent []SessionDigest, skills []SkillMeta, courseID string) string {
	type section struct{ title, body string }
	var sections []section

	// 1. User profile
	if scope.Profile != nil {
		sections = append(sections, section{"## User profile", truncate(scope.Profile.Body, capProfile)})
	} else {
		sections = append(sections, section{"## User profile", "_(none)_"})
	}

	// 2. Course context
	if courseID != "" {
		var b strings.Builder
		for _, m := range scope.CourseProjects {
			if m.Title != "" {
				b.WriteString("- **")
				b.WriteString(m.Title)
				b.WriteString("**: ")
			} else {
				b.WriteString("- ")
			}
			b.WriteString(m.Body)
			b.WriteString("\n")
		}
		body := truncate(b.String(), capCourse)
		if body == "" {
			body = "_(none)_"
		}
		sections = append(sections, section{"## Course context: " + courseID, body})
	}

	// 3. Feedback
	{
		var b strings.Builder
		for _, m := range scope.Feedback {
			if m.Title != "" {
				b.WriteString("- **")
				b.WriteString(m.Title)
				b.WriteString("**: ")
			} else {
				b.WriteString("- ")
			}
			b.WriteString(m.Body)
			b.WriteString("\n")
		}
		body := truncate(b.String(), capFeedback)
		if body == "" {
			body = "_(none)_"
		}
		sections = append(sections, section{"## Active feedback rules", body})
	}

	// 4. Recent sessions
	{
		var b strings.Builder
		for _, d := range recent {
			b.WriteString("- ")
			b.WriteString(d.Topic)
			if d.Summary != "" {
				b.WriteString(" — ")
				b.WriteString(d.Summary)
			}
			b.WriteString("\n")
		}
		body := truncate(b.String(), capRecent)
		if body == "" {
			body = "_(none)_"
		}
		sections = append(sections, section{"## Recent sessions", body})
	}

	// 5. Skills
	{
		var b strings.Builder
		for _, sk := range skills {
			b.WriteString("- `")
			b.WriteString(sk.Name)
			b.WriteString("` — ")
			b.WriteString(sk.Description)
			b.WriteString("\n")
		}
		body := truncate(b.String(), capSkills)
		if body == "" {
			body = "_(none yet)_"
		}
		sections = append(sections, section{"## Available skills", body})
	}

	// Render with bottom-drop overflow
	render := func(secs []section) string {
		var b strings.Builder
		b.WriteString("# AGENTS.md\n\n")
		for _, s := range secs {
			b.WriteString(s.title)
			b.WriteString("\n\n")
			b.WriteString(s.body)
			if !strings.HasSuffix(s.body, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
		return b.String()
	}

	out := render(sections)
	for len(out) > agentsMDTotalCap && len(sections) > 1 {
		sections = sections[:len(sections)-1]
		out = render(sections)
	}
	return out
}
```

- [ ] **Step 4: Run tests, verify pass**

```
go test ./agent -run "TestRecentSessions|TestParseSkill|TestAssembleAgents" -v
```

Expected: 5 tests pass.

- [ ] **Step 5: Run full agent suite**

```
go test ./agent -v
```

Expected: all pass.

- [ ] **Step 6: Commit**

```
git -c user.email=you@example.com -c user.name=your-name \
  add agent/memory.go agent/memory_test.go
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "agent: AGENTS.md assembler with recent-sessions + skills frontmatter"
```

---

## Task 4 — `claw-cli` skeleton + `memory save`

**Files:**
- Create: `claw-cli/main.go`
- Create: `claw-cli/main_test.go`

- [ ] **Step 1: Write the failing tests** in `claw-cli/main_test.go`

```go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"study-app/agent"
)

func newTempDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "study.db")
	db, err := agent.OpenDB(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := agent.InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	db.Close()
	return path
}

func TestRunUnknownSubcommandExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "wat"}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit code: %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown") {
		t.Fatalf("stderr: %s", stderr.String())
	}
}

func TestRunMemorySaveJSONOutput(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "save",
		"--kind", "feedback",
		"--course", "ce297",
		"--title", "no abbreviations",
		"--body", "spell out Software Control Category not SCC",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout.String())
	}
	if got["kind"] != "feedback" || got["title"] != "no abbreviations" {
		t.Fatalf("got %+v", got)
	}
	if id, ok := got["id"].(float64); !ok || id == 0 {
		t.Fatalf("expected non-zero id, got %v", got["id"])
	}
}

func TestRunMemorySaveBodyFromStdin(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("body from stdin\nsecond line")
	code := runWithStdin([]string{
		"clawcli", "memory", "save",
		"--kind", "feedback",
		"--title", "stdin-test",
		"--body", "-", // sentinel: read from stdin
	}, stdin, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
}

func TestRunMissingRequiredFlagExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "save",
		"--kind", "feedback",
		// missing --body
	}, &stdout, &stderr, dbPath)
	if code != 2 {
		t.Fatalf("exit code: %d", code)
	}
}

// Helper to silence "unused" if os import is not yet needed in test file
var _ = os.Stdout
```

- [ ] **Step 2: Run, verify fail (compile error)**

```
go test ./claw-cli -v
```

Expected: FAIL — `claw-cli` package doesn't exist.

- [ ] **Step 3: Create `claw-cli/main.go`**

```go
// claw-cli is the agent's command-line surface into claw-study state.
// It is invoked by Pi via the bash tool. All subcommands write JSON
// (or markdown for `memory load`) to stdout. Errors go to stderr with
// non-zero exit codes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"study-app/agent"
)

const defaultUserID = "eduardo"

func main() {
	dbPath := os.Getenv("CLAW_STUDY_DB")
	if dbPath == "" {
		dbPath = "data/study.db"
	}
	os.Exit(run(os.Args, os.Stdout, os.Stderr, dbPath))
}

func run(args []string, stdout, stderr io.Writer, dbPath string) int {
	return runWithStdin(args, os.Stdin, stdout, stderr, dbPath)
}

func runWithStdin(args []string, stdin io.Reader, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: claw-cli <subcommand> [args]")
		return 2
	}
	switch args[1] {
	case "memory":
		return runMemory(args[2:], stdin, stdout, stderr, dbPath)
	default:
		fmt.Fprintf(stderr, "unknown subcommand: %q\n", args[1])
		return 2
	}
}

func runMemory(args []string, stdin io.Reader, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: claw-cli memory <save|search|load> [args]")
		return 2
	}
	switch args[0] {
	case "save":
		return memorySave(args[1:], stdin, stdout, stderr, dbPath)
	default:
		fmt.Fprintf(stderr, "unknown memory subcommand: %q\n", args[0])
		return 2
	}
}

func memorySave(args []string, stdin io.Reader, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("memory save", flag.ContinueOnError)
	fs.SetOutput(stderr)
	kind := fs.String("kind", "", "memory kind (profile|feedback|project|reference)")
	course := fs.String("course", "", "course id (optional)")
	title := fs.String("title", "", "memory title")
	body := fs.String("body", "", "memory body, or `-` to read from stdin")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *kind == "" || *body == "" {
		fmt.Fprintln(stderr, "memory save: --kind and --body are required")
		return 2
	}

	bodyText := *body
	if bodyText == "-" {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "read stdin: %v\n", err)
			return 1
		}
		bodyText = string(raw)
	}

	db, err := agent.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return 1
	}
	defer db.Close()
	if err := agent.InitSchema(db); err != nil {
		fmt.Fprintf(stderr, "init schema: %v\n", err)
		return 1
	}
	store := agent.NewMemoryStore(db)
	saved, err := store.Save(agent.Memory{
		UserID:   defaultUserID,
		CourseID: *course,
		Kind:     *kind,
		Title:    *title,
		Body:     bodyText,
	})
	if err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{
		"id":    saved.ID,
		"kind":  saved.Kind,
		"title": saved.Title,
	})
	return 0
}
```

- [ ] **Step 4: Run tests, verify pass**

```
go test ./claw-cli -v
```

Expected: 4 tests pass.

- [ ] **Step 5: Verify the binary builds**

```
go build -o /tmp/claw-cli ./claw-cli
/tmp/claw-cli wat
echo "exit: $?"
```

Expected: stderr `unknown subcommand: "wat"`, exit 2.

- [ ] **Step 6: Commit**

```
git -c user.email=you@example.com -c user.name=your-name \
  add claw-cli/main.go claw-cli/main_test.go
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "claw-cli: skeleton + memory save subcommand"
```

---

## Task 5 — `memory search` subcommand

**Files:**
- Modify: `claw-cli/main.go` (add search dispatcher + handler)
- Modify: `claw-cli/main_test.go` (append tests)

- [ ] **Step 1: Write the failing tests** at the end of `claw-cli/main_test.go`

```go
func TestRunMemorySearchReturnsResults(t *testing.T) {
	dbPath := newTempDB(t)
	// Seed via `save`
	var sb, eb bytes.Buffer
	for _, body := range []string{"density rule", "abbreviations rule", "unrelated text"} {
		sb.Reset(); eb.Reset()
		code := run([]string{
			"clawcli", "memory", "save",
			"--kind", "feedback", "--title", body, "--body", body,
		}, &sb, &eb, dbPath)
		if code != 0 {
			t.Fatalf("seed: %s", eb.String())
		}
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "search", "--query", "rule",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	var got struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout.String())
	}
	if len(got.Results) != 2 {
		t.Fatalf("expected 2 hits, got %d:\n%s", len(got.Results), stdout.String())
	}
}

func TestRunMemorySearchMissingQueryExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "memory", "search"}, &stdout, &stderr, dbPath)
	if code != 2 {
		t.Fatalf("exit: %d", code)
	}
}
```

- [ ] **Step 2: Run, verify fail**

```
go test ./claw-cli -run TestRunMemorySearch -v
```

Expected: FAIL — `unknown memory subcommand: "search"`.

- [ ] **Step 3: Wire `search`** in `claw-cli/main.go`. Modify `runMemory`'s switch to add `search`, and append `memorySearch`:

In `runMemory`:
```go
	case "search":
		return memorySearch(args[1:], stdout, stderr, dbPath)
```

Append:
```go
type searchResult struct {
	ID       int64  `json:"id"`
	Kind     string `json:"kind"`
	CourseID string `json:"course_id,omitempty"`
	Title    string `json:"title,omitempty"`
	Snippet  string `json:"snippet"`
}

func memorySearch(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("memory search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	query := fs.String("query", "", "search query (required)")
	course := fs.String("course", "", "course id (optional)")
	limit := fs.Int("limit", 20, "max results")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *query == "" {
		fmt.Fprintln(stderr, "memory search: --query is required")
		return 2
	}
	db, err := agent.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return 1
	}
	defer db.Close()
	if err := agent.InitSchema(db); err != nil {
		fmt.Fprintf(stderr, "init schema: %v\n", err)
		return 1
	}
	store := agent.NewMemoryStore(db)
	rows, err := store.Search(defaultUserID, *query, *course, *limit)
	if err != nil {
		fmt.Fprintf(stderr, "search: %v\n", err)
		return 1
	}
	out := make([]searchResult, 0, len(rows))
	for _, m := range rows {
		snippet := m.Body
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		out = append(out, searchResult{
			ID: m.ID, Kind: m.Kind, CourseID: m.CourseID,
			Title: m.Title, Snippet: snippet,
		})
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"results": out})
	return 0
}
```

- [ ] **Step 4: Run tests, verify pass**

```
go test ./claw-cli -v
```

Expected: all tests (Task 4 + Task 5) pass.

- [ ] **Step 5: Commit**

```
git -c user.email=you@example.com -c user.name=your-name \
  add claw-cli/main.go claw-cli/main_test.go
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "claw-cli: memory search subcommand"
```

---

## Task 6 — `memory load` subcommand (AGENTS.md generator)

**Files:**
- Modify: `claw-cli/main.go` (add `load` dispatcher + handler)
- Modify: `claw-cli/main_test.go` (append tests)

- [ ] **Step 1: Write the failing tests** at the end of `claw-cli/main_test.go`

```go
func TestRunMemoryLoadProducesAgentsMD(t *testing.T) {
	dbPath := newTempDB(t)
	// Seed: profile + ce297-scoped + global feedback
	for _, args := range [][]string{
		{"--kind", "profile", "--title", "user", "--body", "Eduardo studies safety at ITA"},
		{"--kind", "project", "--course", "ce297", "--title", "course-arc", "--body", "STAMP vs Avizienis"},
		{"--kind", "feedback", "--course", "ce297", "--title", "no-abbrev", "--body", "spell out Software Control Category"},
		{"--kind", "feedback", "--title", "density", "--body", "match existing density"},
	} {
		var sb, eb bytes.Buffer
		full := append([]string{"clawcli", "memory", "save"}, args...)
		if code := run(full, &sb, &eb, dbPath); code != 0 {
			t.Fatalf("seed: %s", eb.String())
		}
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "load", "--course", "ce297", "--user", "eduardo",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"# AGENTS.md", "## User profile", "Eduardo studies safety",
		"## Course context: ce297", "STAMP",
		"## Active feedback rules", "no-abbrev", "density",
		"## Available skills", "_(none yet)_",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
	if len(out) > 3072 {
		t.Fatalf("over cap: %d", len(out))
	}
}

func TestRunMemoryLoadEmptyDBStillProducesShell(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "memory", "load", "--course", "ce297"}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# AGENTS.md") {
		t.Fatalf("expected AGENTS.md header even on empty db")
	}
}
```

- [ ] **Step 2: Run, verify fail**

```
go test ./claw-cli -run TestRunMemoryLoad -v
```

Expected: FAIL — unknown subcommand `load`.

- [ ] **Step 3: Wire `load`** in `claw-cli/main.go`. In `runMemory`'s switch:

```go
	case "load":
		return memoryLoad(args[1:], stdout, stderr, dbPath)
```

Append:
```go
func memoryLoad(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("memory load", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id")
	user := fs.String("user", defaultUserID, "user id")
	skillsDir := fs.String("skills-dir", "skills", "directory containing SKILL.md files")
	_ = fs.String("session", "", "session id (informational; unused in v1)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	db, err := agent.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return 1
	}
	defer db.Close()
	if err := agent.InitSchema(db); err != nil {
		fmt.Fprintf(stderr, "init schema: %v\n", err)
		return 1
	}
	store := agent.NewMemoryStore(db)
	scope, err := store.LoadByScope(*user, *course)
	if err != nil {
		fmt.Fprintf(stderr, "load scope: %v\n", err)
		return 1
	}
	var recent []agent.SessionDigest
	if *course != "" {
		recent, err = agent.RecentSessionsForCourse(db, *course, 2)
		if err != nil {
			fmt.Fprintf(stderr, "recent sessions: %v\n", err)
			return 1
		}
	}
	skills, err := agent.ParseSkillsDir(*skillsDir)
	if err != nil {
		fmt.Fprintf(stderr, "parse skills: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, agent.AssembleAgentsMD(scope, recent, skills, *course))
	return 0
}
```

- [ ] **Step 4: Run tests, verify pass**

```
go test ./claw-cli -v
```

Expected: all tests pass (Tasks 4-6).

- [ ] **Step 5: Hand-verify size cap with the seeded fixture**

```
go run ./claw-cli memory load --course ce297 --user eduardo > /tmp/agents.md
wc -c /tmp/agents.md
head -50 /tmp/agents.md
```

(Will print "data/study.db" not found error since no seed yet — that's expected, just confirming the binary path. Skip if it errors — we'll verify post-seed in Task 8.)

- [ ] **Step 6: Commit**

```
git -c user.email=you@example.com -c user.name=your-name \
  add claw-cli/main.go claw-cli/main_test.go
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "claw-cli: memory load subcommand emits AGENTS.md"
```

---

## Task 7 — `seed-memory` importer

**Files:**
- Create: `seed-memory/main.go`
- Create: `seed-memory/main_test.go`

- [ ] **Step 1: Write the failing tests** in `seed-memory/main_test.go`

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeriveCourseIDFromPath(t *testing.T) {
	cases := []struct{ in, root, want string }{
		{"/m/courses/ce297/safety.md", "/m", "ce297"},
		{"/m/courses/dsa-interview/interests.md", "/m", "dsa-interview"},
		{"/m/feedback_dsa_descriptive_names.md", "/m", ""},
		{"/m/user_profile.md", "/m", ""},
		{"/m/study_tracks/phase1.md", "/m", ""},
	}
	for _, c := range cases {
		got := deriveCourseID(c.in, c.root)
		if got != c.want {
			t.Fatalf("deriveCourseID(%q, %q) = %q, want %q", c.in, c.root, got, c.want)
		}
	}
}

func TestParseFrontmatterMinimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	content := []byte(`---
name: test
description: a test
type: feedback
---
body line 1
body line 2
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	fm, body, err := parseFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm["type"] != "feedback" || fm["name"] != "test" {
		t.Fatalf("frontmatter: %+v", fm)
	}
	if body == "" || !contains(body, "body line 1") {
		t.Fatalf("body: %q", body)
	}
}

func TestParseFileNoFrontmatterReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-fm.md")
	if err := os.WriteFile(path, []byte("just body, no frontmatter"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := parseFile(path); err == nil {
		t.Fatalf("expected error")
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run, verify fail (no package)**

```
go test ./seed-memory -v
```

Expected: FAIL — package missing.

- [ ] **Step 3: Create `seed-memory/main.go`**

```go
// seed-memory imports Eduardo's existing memory store at
// ~/.claude/projects/<project-slug>/
// into the agent_memory SQLite table. Idempotent: deletes all rows for
// the user before reseeding.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"study-app/agent"
)

const userID = "eduardo"

func main() {
	source := flag.String("source", os.ExpandEnv("$HOME/.claude/projects/<project-slug>"), "source memory directory")
	dbPath := flag.String("db", "data/study.db", "study.db path")
	dryRun := flag.Bool("dry-run", false, "print what would be inserted; do not write")
	flag.Parse()

	db, err := agent.OpenDB(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := agent.InitSchema(db); err != nil {
		log.Fatalf("init schema: %v", err)
	}
	store := agent.NewMemoryStore(db)

	rows, err := collect(*source)
	if err != nil {
		log.Fatalf("collect: %v", err)
	}
	log.Printf("collected %d candidate rows from %s", len(rows), *source)

	if *dryRun {
		for _, r := range rows {
			fmt.Printf("[%s] course=%q title=%q (%d bytes)\n", r.Kind, r.CourseID, r.Title, len(r.Body))
		}
		return
	}

	if _, err := db.Exec(`DELETE FROM agent_memory WHERE user_id = ?`, userID); err != nil {
		log.Fatalf("clear: %v", err)
	}
	for _, r := range rows {
		r.UserID = userID
		if _, err := store.Save(r); err != nil {
			log.Fatalf("save %q: %v", r.Title, err)
		}
	}
	log.Printf("seeded %d rows", len(rows))
}

// collect walks root and returns rows ready for Save (UserID empty).
func collect(root string) ([]agent.Memory, error) {
	var out []agent.Memory
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		if filepath.Base(path) == "MEMORY.md" {
			return nil // index file, not a memory itself
		}
		fm, body, err := parseFile(path)
		if err != nil {
			return nil // skip files without frontmatter
		}
		kind := mapKind(fm["type"])
		course := deriveCourseID(path, root)
		// Filter: keep all course-scoped rows (any kind), and global rows of kind profile/feedback only.
		if course == "" && kind != "profile" && kind != "feedback" {
			return nil
		}
		out = append(out, agent.Memory{
			CourseID:  course,
			Kind:      kind,
			Title:     fm["name"],
			Body:      strings.TrimSpace(body),
			CreatedAt: time.Now().Unix(),
		})
		return nil
	})
	return out, err
}

// mapKind translates frontmatter `type` to agent_memory.kind.
func mapKind(t string) string {
	switch t {
	case "user":
		return "profile"
	case "feedback":
		return "feedback"
	case "project":
		return "project"
	case "reference":
		return "reference"
	default:
		return "project" // safe fallback — won't be loaded for AGENTS.md global feedback
	}
}

// deriveCourseID returns "ce297" for paths under courses/ce297/, "" otherwise.
func deriveCourseID(path, root string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) >= 2 && parts[0] == "courses" {
		return parts[1]
	}
	return ""
}

// parseFile returns the frontmatter map and the body of a markdown file.
func parseFile(path string) (map[string]string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024), 1024*1024)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return nil, "", fmt.Errorf("no frontmatter in %s", path)
	}
	fm := map[string]string{}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		fm[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
	}
	if len(fm) == 0 {
		return nil, "", fmt.Errorf("empty frontmatter in %s", path)
	}
	var body strings.Builder
	for sc.Scan() {
		body.WriteString(sc.Text())
		body.WriteString("\n")
	}
	if err := sc.Err(); err != nil {
		return nil, "", err
	}
	return fm, body.String(), nil
}
```

- [ ] **Step 4: Run unit tests, verify pass**

```
go test ./seed-memory -v
```

Expected: 3 tests pass.

- [ ] **Step 5: Smoke-run dry-mode against the real source dir**

```
go run ./seed-memory --dry-run --source "$HOME/.claude/projects/<project-slug>" --db /tmp/seed-smoke.db
```

Expected: prints ~25-35 candidate rows: 1 `[profile]`, ~13 `[feedback]` global, ~6 course-scoped under `ce297`, etc. No errors.

- [ ] **Step 6: Commit**

```
git -c user.email=you@example.com -c user.name=your-name \
  add seed-memory/main.go seed-memory/main_test.go
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "seed-memory: import frontmatter-tagged memory into agent_memory"
```

---

## Task 8 — Cross-compile, deploy, and verify on VPS

**Files:** none. Pure deployment + verification.

- [ ] **Step 1: Cross-compile both binaries**

```
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/claw-cli-linux ./claw-cli
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/seed-memory-linux ./seed-memory
ls -la /tmp/claw-cli-linux /tmp/seed-memory-linux
```

Expected: two ELF binaries, ~10-18 MB each.

- [ ] **Step 2: Copy binaries to VPS**

```
scp /tmp/claw-cli-linux nanoclaw:$VAULT_ROOT/bin/claw-cli
scp /tmp/seed-memory-linux nanoclaw:$VAULT_ROOT/bin/seed-memory
ssh nanoclaw 'chmod +x $VAULT_ROOT/bin/claw-cli $VAULT_ROOT/bin/seed-memory'
```

- [ ] **Step 3: Sync the source memory dir to the VPS**

The VPS doesn't have `~/.claude/projects/<project-slug>/` — that lives on the laptop. Tar it and copy:

```
cd ~/.claude/projects/<project-slug>/
tar czf /tmp/memory-source.tgz memory/
scp /tmp/memory-source.tgz nanoclaw:$VAULT_ROOT/data/
ssh nanoclaw 'cd $VAULT_ROOT/data && tar xzf memory-source.tgz && rm memory-source.tgz && ls memory | wc -l'
```

Expected: file count ~30+.

- [ ] **Step 4: Apply schema migration on the live DB**

The running `study-app.service` already calls `InitSchema` at startup. Restart so the new `agent_memory` table is created:

```
ssh nanoclaw 'export XDG_RUNTIME_DIR=/run/user/$(id -u); systemctl --user restart study-app.service'
ssh nanoclaw 'sleep 2; sqlite3 $VAULT_ROOT/data/study.db "SELECT name FROM sqlite_master WHERE type=\"table\" AND name=\"agent_memory\";"'
```

Expected: prints `agent_memory`.

(If the service is running an older binary that doesn't have `InitSchema` updated, redeploy `study-app` per the deploy cheat sheet first. Check with `ssh nanoclaw 'sha256sum $VAULT_ROOT/bin/study-app'` against a freshly-built binary.)

Actually — we likely need to redeploy `study-app` itself since the schema change is in the `agent` package. Do that:

```
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
scp /tmp/study-app-linux nanoclaw:$VAULT_ROOT/bin/study-app.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak && mv study-app.new study-app && chmod +x study-app && export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service'
ssh nanoclaw 'sleep 2; sqlite3 $VAULT_ROOT/data/study.db "SELECT name FROM sqlite_master WHERE type=\"table\" AND name=\"agent_memory\";"'
```

- [ ] **Step 5: Run the seed importer**

```
ssh nanoclaw 'cd $VAULT_ROOT && ./bin/seed-memory --source ./data/memory --db ./data/study.db'
```

Expected: log line `seeded N rows` (N ≈ 25-35).

- [ ] **Step 6: Generate AGENTS.md and verify the cap**

```
ssh nanoclaw 'cd $VAULT_ROOT && ./bin/claw-cli memory load --course ce297 --user eduardo' > /tmp/agents-ce297.md
wc -c /tmp/agents-ce297.md
head -60 /tmp/agents-ce297.md
```

Expected:
- File size ≤ 3072 bytes
- Contains `# AGENTS.md`, `## User profile`, `## Course context: ce297`, `## Active feedback rules`, `## Recent sessions`, `## Available skills`
- Body of profile, course context, feedback all populated from real data
- Skills section says `_(none yet)_` (Phase 3 will populate)

- [ ] **Step 7: Smoke `memory search`**

```
ssh nanoclaw 'cd $VAULT_ROOT && ./bin/claw-cli memory search --query "abbreviations"'
```

Expected: JSON `{"results":[...]}` with at least one feedback entry hit.

- [ ] **Step 8: Smoke `memory save` and re-verify load**

```
ssh nanoclaw 'cd $VAULT_ROOT && ./bin/claw-cli memory save --kind feedback --course ce297 --title "phase1-smoke" --body "test entry written by claw-cli during Phase 1 smoke"'
ssh nanoclaw 'cd $VAULT_ROOT && ./bin/claw-cli memory load --course ce297 --user eduardo' | grep -c phase1-smoke
```

Expected: save emits `{"id":N,"kind":"feedback","title":"phase1-smoke"}`; the grep returns ≥1 (entry surfaces in feedback section).

- [ ] **Step 9: Push and commit deploy notes**

Append a `Phase 1 deploy log` section to `docs/specs/proposals/pi-rpc-handshake-notes.md` (or a new sibling file) recording:
- Commit SHA range for Phase 1
- Final `wc -c /tmp/agents-ce297.md` byte count
- Number of seeded rows
- Any deviation from the plan

```
git -c user.email=you@example.com -c user.name=your-name \
  add docs/specs/proposals/
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "docs: phase 1 deploy log"
git push origin main
```

---

## Self-review (controller's pre-flight check before dispatch)

- ✅ Spec scope: `claw-cli` skeleton (Tasks 4-6), `memory load|save|search` (Tasks 4-6), schema migration (Task 1), seed script (Task 7), AGENTS.md ≤ 3 KB verification (Task 8). Loader unit tests cover empty, course-only filtering, recency cutoff, skill-list assembly (Tasks 2-3 and 6).
- ✅ No placeholders. All test bodies written out, all SQL written out, all flag plumbing written out.
- ✅ Type consistency: `Memory`, `Scope`, `SessionDigest`, `SkillMeta`, `MemoryStore`, `RecentSessionsForCourse`, `ParseSkillsDir`, `AssembleAgentsMD`, `searchResult` — all defined and referenced consistently.
- ✅ Commit hygiene: 8 commits, one per task, descriptive messages.
- ⚠️ Task 8 redeploys `study-app.service` — that's a production-impact step. Subagent must do it explicitly and verify with the smoke commands; not an automatic side-effect.