# Phase 1 Pre-Prep Fixes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement task-by-task.

**Goal:** Address the three Phase-2-prep items surfaced by the Phase 1 final review. They silently degrade the Phase 2 agent runtime in ways that look like model failures rather than infra failures, so they ship before any Phase 2 work begins.

**Architecture:** Three targeted fixes inside the existing `agent/`, `claw-cli/`, `seed-memory/` packages, plus a deploy/verify pass on the VPS. No new packages.

**Tech Stack:** Go 1.24+, `unicode/utf8`, `database/sql` transactions, `os.Stat` for mtimes. Tests via stdlib `testing`. Lint enforced via the pre-commit hook installed 2026-05-10 — every new exported symbol needs a doc comment (revive `exported`), code must pass `gofmt -s` and the staged-file lint scan.

---

## The three issues (from Phase 1 cross-task review 2026-05-10)

1. **UTF-8-unsafe `s[:n]` truncation** in three sites: `agent/memory.go` `RecentSessionsForCourse` summary trim, `agent/memory.go` `truncate` helper used by `AssembleAgentsMD`, `claw-cli/main.go` `memorySearch` snippet trim. Eduardo's memory contains Portuguese (`ção`, `é`); a 200-byte cut landing mid-rune emits invalid UTF-8 right before the `…` ellipsis. Pi parses AGENTS.md as UTF-8 and will mojibake or reject. **Fix:** one rune-aware helper in `agent`; all three sites call it.

2. **`claw-cli` CWD-relative defaults break Pi sandbox.** `data/study.db` and `skills` defaults assume invocation from `~/stack/study-app/`. Pi's per-session sandbox at `data/agent-sessions/<id>/` will silently create a fresh empty SQLite (because `OpenDB` creates missing files) and emit a "shell" AGENTS.md with `_(none)_` everywhere. **Fix:** resolve paths against `CLAW_STUDY_ROOT`; error loudly when the resolved DB doesn't exist; never auto-create the DB outside an explicit init step. `seed-memory` remains the only path that bootstraps a DB.

3. **`seed-memory` not transactional.** `DELETE FROM agent_memory WHERE user_id=?` runs first, then a per-row `Save` loop with `log.Fatalf` on error. An interruption between delete and a successful loop nukes production memory. Also: every reseed resets `created_at = time.Now().Unix()`, destroying historical recency. **Fix:** wrap delete + insert loop in a single `BEGIN`/`COMMIT`. Use `os.Stat(path).ModTime()` for `created_at` so reseeds are idempotent.

---

## File structure

| File | Status | Responsibility |
|---|---|---|
| `agent/text.go` | **create** | `TruncateRunes(s string, maxBytes int) string` — UTF-8-aware byte-budgeted truncation with ellipsis |
| `agent/text_test.go` | **create** | Tests: ASCII, multi-byte cuts, exact boundary, empty, sub-ellipsis budget |
| `agent/memory.go` | modify | Replace two byte-slice truncations with `TruncateRunes`; delete the local `truncate` helper |
| `claw-cli/main.go` | modify | New `resolveDBPath` + `resolveSkillsDir` helpers. Add `--db` flag to all three subcommands. Error if resolved DB doesn't exist; error if explicit `--skills-dir` doesn't exist (default skills dir may be missing). Replace `memorySearch`'s byte-slice truncation. |
| `claw-cli/main_test.go` | modify | Tests: env-var DB resolution, missing-DB error on explicit `--db`, missing-skills error on explicit `--skills-dir`, default skills missing is OK |
| `seed-memory/main.go` | modify | Wrap delete + insert loop in transaction; `os.Stat(path).ModTime().Unix()` for `created_at` |
| `seed-memory/main_test.go` | modify | One test exercising mtime-based `created_at` |

---

## Task 1 — `TruncateRunes` helper, swap all three sites

**Files:** `agent/text.go` (create), `agent/text_test.go` (create), `agent/memory.go` (modify), `claw-cli/main.go` (modify, just the `memorySearch` site).

- [ ] **Step 1: Create `agent/text_test.go`**

```go
package agent

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateRunesShortString(t *testing.T) {
	if got := TruncateRunes("hello", 100); got != "hello" {
		t.Fatalf("got %q, want hello", got)
	}
}

func TestTruncateRunesEmptyInput(t *testing.T) {
	if got := TruncateRunes("", 10); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestTruncateRunesAsciiBudgetEnforced(t *testing.T) {
	in := strings.Repeat("a", 300)
	got := TruncateRunes(in, 100)
	if len(got) > 100 {
		t.Fatalf("over budget: %d bytes", len(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("missing ellipsis: %q", got)
	}
}

func TestTruncateRunesMultiByteSafe(t *testing.T) {
	in := strings.Repeat("ção ", 100)
	got := TruncateRunes(in, 50)
	if len(got) > 50 {
		t.Fatalf("over budget: %d bytes", len(got))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8 in output: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("missing ellipsis: %q", got)
	}
}

func TestTruncateRunesExactBoundaryNoTrim(t *testing.T) {
	in := "hello world"
	if got := TruncateRunes(in, len(in)); got != in {
		t.Fatalf("got %q, want unchanged", got)
	}
}

func TestTruncateRunesBudgetSmallerThanEllipsis(t *testing.T) {
	got := TruncateRunes("hello", 2)
	if len(got) > 2 {
		t.Fatalf("over budget: %d bytes", len(got))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8: %q", got)
	}
}
```

- [ ] **Step 2: Run, verify fail**

```bash
cd ~/Documents/ITA/claw-study
/opt/homebrew/bin/go test ./agent -run TestTruncateRunes -v
```

Expected: FAIL — `TruncateRunes` undefined.

- [ ] **Step 3: Create `agent/text.go`**

```go
// Package agent — text helpers. UTF-8-safe operations the memory and CLI
// layers share. Kept in a separate file so future text-handling additions
// (sanitization, case folding) live next to TruncateRunes rather than
// growing memory.go further.
package agent

import "unicode/utf8"

// TruncateRunes returns s if its byte length ≤ maxBytes, else the longest
// rune-boundary-aligned prefix of s that fits within (maxBytes - 3) bytes
// followed by "…" (3 bytes in UTF-8). If maxBytes < 3 the ellipsis is
// dropped and the prefix is truncated to fit the budget directly.
//
// The function never returns invalid UTF-8 even when maxBytes lands inside
// a multi-byte rune.
func TruncateRunes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	const ellipsis = "…"
	if maxBytes < len(ellipsis) {
		return truncateNoEllipsis(s, maxBytes)
	}
	prefix := truncateNoEllipsis(s, maxBytes-len(ellipsis))
	return prefix + ellipsis
}

// truncateNoEllipsis returns the longest valid-UTF-8 prefix of s whose byte
// length is ≤ maxBytes.
func truncateNoEllipsis(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	end := 0
	for end < len(s) {
		_, size := utf8.DecodeRuneInString(s[end:])
		if end+size > maxBytes {
			break
		}
		end += size
	}
	return s[:end]
}
```

- [ ] **Step 4: Run, verify pass**

```bash
/opt/homebrew/bin/go test ./agent -run TestTruncateRunes -v
```

Expected: 6 tests pass.

- [ ] **Step 5: Replace byte-slice truncation in `agent/memory.go`**

Find the two sites:
```bash
grep -n 'd\.Summary\[:200\]\|s\[:n\] + "…"' ~/Documents/ITA/claw-study/agent/memory.go
```

In `RecentSessionsForCourse` replace the existing block:
```go
		if len(d.Summary) > 200 {
			d.Summary = d.Summary[:200] + "…"
		}
```
with:
```go
		d.Summary = TruncateRunes(d.Summary, 200)
```

In `AssembleAgentsMD`: every call to `truncate(body, capX)` (or `truncate(scope.Profile.Body, capProfile)`) becomes `TruncateRunes(...)` with the same args. Then **delete the local `truncate` function** entirely (it lives in `agent/memory.go` near the constants block).

- [ ] **Step 6: Replace byte-slice truncation in `claw-cli/main.go`**

Find:
```bash
grep -n 'snippet := m\.Body\|snippet\[:200\]' ~/Documents/ITA/claw-study/claw-cli/main.go
```

In `memorySearch`, replace:
```go
		snippet := m.Body
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
```
with:
```go
		snippet := agent.TruncateRunes(m.Body, 200)
```

- [ ] **Step 7: Verify all tests still pass**

```bash
/opt/homebrew/bin/go test ./... 2>&1 | tail -10
```

Expected: all green.

- [ ] **Step 8: Commit**

```bash
cd ~/Documents/ITA/claw-study
git add agent/text.go agent/text_test.go agent/memory.go claw-cli/main.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "agent: add TruncateRunes helper, replace UTF-8-unsafe byte slicing"
```

The pre-commit hook will lint the staged Go files. If it blocks on issues in your changes, fix them. If it blocks on pre-existing debt, that's a hook bug — report.

---

## Task 2 — `claw-cli` resolves paths via `CLAW_STUDY_ROOT`, errors on missing explicit paths

**Files:** `claw-cli/main.go`, `claw-cli/main_test.go`.

**Design summary:**

- New helper `resolveDBPath(override, fallback string) (string, error)` — order: explicit `--db` > `dbPath` arg from `main` > `CLAW_STUDY_DB` env > `CLAW_STUDY_ROOT/data/study.db`. Returns error if final path is empty or `os.Stat` fails.
- New helper `resolveSkillsDir(override string) string` — order: explicit `--skills-dir` > `CLAW_STUDY_ROOT/skills` > `skills`. Does NOT Stat (default skills dir is allowed to be missing — Phase 3 hasn't built it yet).
- A separate guard: if `--skills-dir` was passed explicitly AND the path doesn't exist, return exit 1 with a clear error.
- All three subcommands gain a `--db` flag plumbed through `resolveDBPath`. `memoryLoad` keeps its `--skills-dir` flag with the new resolution + guard.
- `main()` pre-seeds `dbPath` from `CLAW_STUDY_DB` env or `CLAW_STUDY_ROOT/data/study.db` (legacy path retained as last fallback). Tests bypass main and rely on the helper's full fallback chain.

- [ ] **Step 1: Append failing tests to `claw-cli/main_test.go`**

You may need to add `"os"` and `"path/filepath"` to imports if not present.

```go
func TestRunMemoryLoadResolvesDBFromEnvRoot(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(dataDir, "study.db")
	db, err := agent.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("seed open: %v", err)
	}
	if err := agent.InitSchema(db); err != nil {
		t.Fatalf("seed init: %v", err)
	}
	db.Close()

	t.Setenv("CLAW_STUDY_ROOT", root)
	t.Setenv("CLAW_STUDY_DB", "")
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "memory", "load", "--course", "ce297"}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# AGENTS.md") {
		t.Fatalf("expected AGENTS.md output, got: %s", stdout.String())
	}
}

func TestRunMemoryLoadErrorsOnExplicitMissingDB(t *testing.T) {
	t.Setenv("CLAW_STUDY_ROOT", "")
	t.Setenv("CLAW_STUDY_DB", "")
	missing := filepath.Join(t.TempDir(), "missing.db")
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "load", "--course", "ce297", "--db", missing,
	}, &stdout, &stderr, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "database not found") || !strings.Contains(stderr.String(), missing) {
		t.Fatalf("expected database-not-found error mentioning %q, got: %s", missing, stderr.String())
	}
}

func TestRunMemoryLoadErrorsOnExplicitMissingSkillsDir(t *testing.T) {
	dbPath := newTempDB(t)
	missing := filepath.Join(t.TempDir(), "no-such-skills")
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "load", "--course", "ce297",
		"--db", dbPath, "--skills-dir", missing,
	}, &stdout, &stderr, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "skills directory not found") || !strings.Contains(stderr.String(), missing) {
		t.Fatalf("expected skills-not-found error, got: %s", stderr.String())
	}
}

func TestRunMemoryLoadDefaultSkillsDirMissingIsOK(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("CLAW_STUDY_ROOT", t.TempDir()) // root has no skills/ subdir
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "load", "--course", "ce297", "--db", dbPath,
	}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "_(none yet)_") {
		t.Fatalf("expected skill section fallback, got: %s", stdout.String())
	}
}
```

- [ ] **Step 2: Run, verify fail**

```bash
/opt/homebrew/bin/go test ./claw-cli -run "TestRunMemoryLoadResolves|TestRunMemoryLoadErrors|TestRunMemoryLoadDefault" -v
```

Expected: FAIL — flags unrecognized OR no error returned.

- [ ] **Step 3: Modify `claw-cli/main.go`**

Add `"path/filepath"` to imports if not already present.

Add these two helpers after the `defaultUserID` constant:

```go
// resolveStudyRoot returns CLAW_STUDY_ROOT or empty.
func resolveStudyRoot() string {
	return os.Getenv("CLAW_STUDY_ROOT")
}

// resolveDBPath returns the effective study.db path. Order of precedence:
// explicit override > fallback (from main) > CLAW_STUDY_DB > CLAW_STUDY_ROOT/data/study.db.
// Returns an error if the final path is empty or does not exist on disk.
// claw-cli never bootstraps a fresh DB — seed-memory is the canonical first-run path.
func resolveDBPath(override, fallback string) (string, error) {
	resolved := override
	if resolved == "" {
		resolved = fallback
	}
	if resolved == "" {
		resolved = os.Getenv("CLAW_STUDY_DB")
	}
	if resolved == "" {
		if root := resolveStudyRoot(); root != "" {
			resolved = filepath.Join(root, "data", "study.db")
		}
	}
	if resolved == "" {
		return "", fmt.Errorf("no database path: pass --db, set CLAW_STUDY_DB, or set CLAW_STUDY_ROOT")
	}
	if _, err := os.Stat(resolved); err != nil {
		return "", fmt.Errorf("database not found at %q: %w", resolved, err)
	}
	return resolved, nil
}

// resolveSkillsDir returns the skills directory path. Order of precedence:
// explicit override > CLAW_STUDY_ROOT/skills > "skills". Does not Stat — a
// defaulted missing dir is OK (Phase 3 has not populated skills/ yet);
// callers must Stat themselves if they want to enforce existence on an
// explicit override.
func resolveSkillsDir(override string) string {
	if override != "" {
		return override
	}
	if root := resolveStudyRoot(); root != "" {
		return filepath.Join(root, "skills")
	}
	return "skills"
}
```

Update `main()` to honor the env precedence (its `dbPath` becomes the "fallback" passed to handlers):

```go
func main() {
	dbPath := os.Getenv("CLAW_STUDY_DB")
	if dbPath == "" {
		if root := os.Getenv("CLAW_STUDY_ROOT"); root != "" {
			dbPath = filepath.Join(root, "data", "study.db")
		} else {
			dbPath = "data/study.db"
		}
	}
	os.Exit(run(os.Args, os.Stdout, os.Stderr, dbPath))
}
```

Add a `--db` flag to `memorySave` and use the helper:

```go
func memorySave(args []string, stdin io.Reader, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("memory save", flag.ContinueOnError)
	fs.SetOutput(stderr)
	kind := fs.String("kind", "", "memory kind (profile|feedback|project|reference)")
	course := fs.String("course", "", "course id (optional)")
	title := fs.String("title", "", "memory title")
	body := fs.String("body", "", "memory body, or `-` to read from stdin")
	dbOverride := fs.String("db", "", "path to study.db (default: $CLAW_STUDY_DB or $CLAW_STUDY_ROOT/data/study.db)")
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

	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	db, err := agent.OpenDB(resolvedDB)
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

Add a `--db` flag to `memorySearch`:

```go
func memorySearch(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("memory search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	query := fs.String("query", "", "search query (required)")
	course := fs.String("course", "", "course id (optional)")
	limit := fs.Int("limit", 20, "max results")
	dbOverride := fs.String("db", "", "path to study.db (default: $CLAW_STUDY_DB or $CLAW_STUDY_ROOT/data/study.db)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *query == "" {
		fmt.Fprintln(stderr, "memory search: --query is required")
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	db, err := agent.OpenDB(resolvedDB)
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
		out = append(out, searchResult{
			ID: m.ID, Kind: m.Kind, CourseID: m.CourseID,
			Title: m.Title, Snippet: agent.TruncateRunes(m.Body, 200),
		})
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"results": out})
	return 0
}
```

(The `m.Body` truncation uses `agent.TruncateRunes` from Task 1.)

Rewrite `memoryLoad` to use both helpers + the explicit-skills-dir guard:

```go
func memoryLoad(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("memory load", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id")
	user := fs.String("user", defaultUserID, "user id")
	skillsDirFlag := fs.String("skills-dir", "", "directory containing SKILL.md files (default: $CLAW_STUDY_ROOT/skills, then 'skills')")
	dbOverride := fs.String("db", "", "path to study.db (default: $CLAW_STUDY_DB or $CLAW_STUDY_ROOT/data/study.db)")
	_ = fs.String("session", "", "session id (informational; unused in v1)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	resolvedSkills := resolveSkillsDir(*skillsDirFlag)
	if *skillsDirFlag != "" {
		if _, err := os.Stat(resolvedSkills); err != nil {
			fmt.Fprintf(stderr, "skills directory not found at %q: %v\n", resolvedSkills, err)
			return 1
		}
	}

	db, err := agent.OpenDB(resolvedDB)
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
	skills, err := agent.ParseSkillsDir(resolvedSkills)
	if err != nil {
		fmt.Fprintf(stderr, "parse skills: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, agent.AssembleAgentsMD(scope, recent, skills, *course))
	return 0
}
```

- [ ] **Step 4: Run all `claw-cli` tests, verify pass**

```bash
/opt/homebrew/bin/go test ./claw-cli -v 2>&1 | tail -30
```

Expected: all 12+ tests pass (8 prior + 4 new). If existing tests fail because they don't set `CLAW_STUDY_DB`/`CLAW_STUDY_ROOT`, **the existing tests are correct** (they pass `dbPath` directly via the `run` harness which is the "fallback" arg) — the failure means the helper logic is wrong; fix it.

- [ ] **Step 5: Commit**

```bash
cd ~/Documents/ITA/claw-study
git add claw-cli/main.go claw-cli/main_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "claw-cli: resolve paths via CLAW_STUDY_ROOT, error on missing explicit paths"
```

---

## Task 3 — `seed-memory` transactional, mtime-based `created_at`

**Files:** `seed-memory/main.go`, `seed-memory/main_test.go`.

- [ ] **Step 1: Append a failing test to `seed-memory/main_test.go`**

```go
func TestCollectUsesFileModTimeForCreatedAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	content := []byte("---\nname: test\ndescription: x\ntype: feedback\n---\nbody\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	// Set the file's mtime to a fixed past timestamp.
	wantTime := int64(1700000000) // 2023-11-14
	mt := time.Unix(wantTime, 0)
	if err := os.Chtimes(path, mt, mt); err != nil {
		t.Fatal(err)
	}

	rows, err := collect(dir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].CreatedAt != wantTime {
		t.Fatalf("CreatedAt = %d, want %d", rows[0].CreatedAt, wantTime)
	}
}
```

Add `"time"` to imports if not already there.

- [ ] **Step 2: Run, verify fail**

```bash
/opt/homebrew/bin/go test ./seed-memory -run TestCollectUsesFileModTime -v
```

Expected: FAIL — `CreatedAt` is `time.Now().Unix()` not the file's mtime.

- [ ] **Step 3: Modify `seed-memory/main.go`**

In `collect`, change the row construction:

Find:
```go
		out = append(out, agent.Memory{
			CourseID:  course,
			Kind:      kind,
			Title:     fm["name"],
			Body:      strings.TrimSpace(body),
			CreatedAt: time.Now().Unix(),
		})
```

Replace with:
```go
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		out = append(out, agent.Memory{
			CourseID:  course,
			Kind:      kind,
			Title:     fm["name"],
			Body:      strings.TrimSpace(body),
			CreatedAt: info.ModTime().Unix(),
		})
```

Wrap the delete + insert loop in a transaction. In `main`, find:

```go
	if _, err := db.Exec(`DELETE FROM agent_memory WHERE user_id = ?`, userID); err != nil {
		log.Fatalf("clear: %v", err)
	}
	for _, r := range rows {
		r.UserID = userID
		if _, err := store.Save(r); err != nil {
			log.Fatalf("save %q: %v", r.Title, err)
		}
	}
```

Replace with:

```go
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.Printf("rollback: %v", rollbackErr)
			}
		}
	}()
	if _, err := tx.Exec(`DELETE FROM agent_memory WHERE user_id = ?`, userID); err != nil {
		log.Fatalf("clear: %v", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO agent_memory (user_id, course_id, kind, title, body, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		log.Fatalf("prepare: %v", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		r.UserID = userID
		var courseID any
		if r.CourseID != "" {
			courseID = r.CourseID
		}
		if _, err := stmt.Exec(r.UserID, courseID, r.Kind, r.Title, r.Body, r.CreatedAt, r.CreatedAt); err != nil {
			log.Fatalf("insert %q: %v", r.Title, err)
		}
	}
	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}
	committed = true
```

(Note: this stops using `store.Save` because `Save` always sets `UpdatedAt = time.Now().Unix()` — bypass it for the seed path so `updated_at` matches `created_at`. The `store` import remains used elsewhere if the file references it; verify with `grep store seed-memory/main.go`.)

If `store` is now unused in `main.go`, remove the `store := agent.NewMemoryStore(db)` line and adjust imports.

- [ ] **Step 4: Run all `seed-memory` tests, verify pass**

```bash
/opt/homebrew/bin/go test ./seed-memory -v
```

Expected: 4 tests pass (3 prior + 1 new).

- [ ] **Step 5: Smoke run dry-mode and commit-mode against a temp DB**

```bash
cd ~/Documents/ITA/claw-study
/opt/homebrew/bin/go run ./seed-memory \
  --source "$HOME/.claude/projects/-Users-eduardohiroji-Documents-ITA-Mestrado/memory" \
  --db /tmp/seed-tx-test.db
```

Expected log: `seeded 24 rows` (or similar — actual count depends on memory dir state). The DB at `/tmp/seed-tx-test.db` should contain 24 rows with `created_at` reflecting actual file mtimes (varied, not all `time.Now()`):

```bash
sqlite3 /tmp/seed-tx-test.db "SELECT COUNT(DISTINCT created_at) FROM agent_memory" 2>/dev/null || \
  python3 -c "import sqlite3; c=sqlite3.connect('/tmp/seed-tx-test.db'); print(c.execute('SELECT COUNT(DISTINCT created_at) FROM agent_memory').fetchone())"
```

Expected: a number > 1 (varied timestamps confirm mtime is being read, not a single `time.Now()`).

- [ ] **Step 6: Commit**

```bash
cd ~/Documents/ITA/claw-study
git add seed-memory/main.go seed-memory/main_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "seed-memory: transactional reseed, file-mtime-based created_at"
```

---

## Task 4 — Cross-compile, deploy, verify on VPS

**Files:** none (deploy + verify only).

This is the production deploy. The fixes touch the `agent` package (so `study-app` server must rebuild), `claw-cli`, and `seed-memory`. Same hot-swap pattern as Phase 1 Task 8.

- [ ] **Step 1: Cross-compile all three binaries**

```bash
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/claw-cli-linux ./claw-cli
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/seed-memory-linux ./seed-memory
ls -la /tmp/study-app-linux /tmp/claw-cli-linux /tmp/seed-memory-linux
```

- [ ] **Step 2: Copy + hot-swap server**

```bash
scp /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
scp /tmp/claw-cli-linux nanoclaw:/home/eduardo/stack/study-app/bin/claw-cli
scp /tmp/seed-memory-linux nanoclaw:/home/eduardo/stack/study-app/bin/seed-memory
ssh nanoclaw 'cd ~/stack/study-app/bin && \
  cp study-app study-app.bak && mv study-app.new study-app && \
  chmod +x study-app claw-cli seed-memory && \
  export XDG_RUNTIME_DIR=/run/user/$(id -u) && \
  systemctl --user restart study-app.service && sleep 3 && \
  systemctl --user is-active study-app.service'
```

Expected: `active`. If not, check logs and rollback per Phase 1 Task 8 procedure.

- [ ] **Step 3: Set `CLAW_STUDY_ROOT` for ergonomics**

The new `claw-cli` paths default to `$CLAW_STUDY_ROOT/data/study.db`. Append to `~/stack/study-app/.env` if not already there:

```bash
ssh nanoclaw 'grep -q CLAW_STUDY_ROOT ~/stack/study-app/.env || \
  echo "CLAW_STUDY_ROOT=/home/eduardo/stack/study-app" >> ~/stack/study-app/.env'
```

(The systemd unit reads `.env`. The CLI inherits via shell when run interactively — verify the next step still works without it being set in your interactive shell, since the test below runs ad-hoc.)

- [ ] **Step 4: Smoke-test `claw-cli` from the canonical cwd (current behavior unchanged)**

```bash
ssh nanoclaw 'cd /home/eduardo/stack/study-app && ./bin/claw-cli memory load --course ce297 --user eduardo' > /tmp/agents-ce297-after-fixes.md
wc -c /tmp/agents-ce297-after-fixes.md
```

Expected: ≤ 3072 bytes. Compare to the Phase 1 deploy log's 2673 bytes — should be in the same ballpark (a few bytes different due to the new TruncateRunes ellipsis-byte accounting).

- [ ] **Step 5: Smoke-test `claw-cli` from a different cwd via `CLAW_STUDY_ROOT`**

```bash
ssh nanoclaw 'cd /tmp && CLAW_STUDY_ROOT=/home/eduardo/stack/study-app /home/eduardo/stack/study-app/bin/claw-cli memory load --course ce297 --user eduardo' > /tmp/agents-from-tmp.md
wc -c /tmp/agents-from-tmp.md
```

Expected: identical (or near-identical) byte count to Step 4. **This is the Pi-sandbox case** the fix was for.

- [ ] **Step 6: Smoke-test loud failure on missing explicit path**

```bash
ssh nanoclaw 'cd /tmp && /home/eduardo/stack/study-app/bin/claw-cli memory load --course ce297 --db /tmp/no-such.db' 2>&1
```

Expected: stderr `database not found at "/tmp/no-such.db"`, exit 1.

- [ ] **Step 7: Re-run seed to confirm transactional + mtime-based updates work in prod**

```bash
ssh nanoclaw 'cd /home/eduardo/stack/study-app && ./bin/seed-memory --source ./data/memory --db ./data/study.db'
```

Expected: `seeded N rows`. Verify the `created_at` distribution:

```bash
ssh nanoclaw 'python3 -c "import sqlite3; c=sqlite3.connect(\"/home/eduardo/stack/study-app/data/study.db\"); print(c.execute(\"SELECT MIN(created_at), MAX(created_at), COUNT(DISTINCT created_at) FROM agent_memory WHERE user_id=\\\"eduardo\\\"\").fetchone())"'
```

Expected: distinct values > 5 (mtime-based, not single `time.Now()`).

- [ ] **Step 8: Append a deploy note to the Phase 1 deploy log**

Edit `docs/specs/proposals/phase1-deploy-log.md` to add a "Pre-prep deploy" section recording:
- Commit range for the three fix commits
- New AGENTS.md byte count
- Whether the cross-cwd test passed
- Confirmation that distinct created_at count > 5

- [ ] **Step 9: Commit + push**

```bash
cd ~/Documents/ITA/claw-study
git add docs/specs/proposals/phase1-deploy-log.md
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "docs: phase 1 pre-prep deploy log"
git push origin main
```

---

## Self-review checklist

- ✅ Three issues each addressed in their own task with TDD
- ✅ All tests passing locally
- ✅ Live AGENTS.md generation still works post-deploy
- ✅ Cross-cwd invocation now works (Pi sandbox case)
- ✅ Loud error on missing explicit `--db` path
- ✅ Distinct `created_at` values confirm mtime-based seed
- ✅ Pre-commit hook didn't fight us (or, if it did, the issues were real and got fixed)
- ✅ Phase 1 deploy log updated
- ✅ All commits pushed
