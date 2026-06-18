# Phase 2 — `claw-cli` domain subcommands — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement task-by-task.

**Goal:** Extend `claw-cli` with the domain subcommands Pi will invoke at runtime: `rag search`, `plan show`, `plan toggle`, `course interests`, `note save`, `pdf extract`, `web fetch`, `skill dispatch`. Each wraps an existing `agent/tools_*.go` function. End state: every entry in the `claw-cli` surface table from `docs/specs/agent-runtime-pi.md` is reachable from the CLI and writes JSON or markdown to stdout for Pi to parse.

**Architecture:** New helper `newAppFromEnv(resolvedDB string) (*agent.App, error)` in `claw-cli/main.go` constructs a fully-configured `*agent.App` from environment variables (mirroring the server's `loadConfig`). Required env vars vary by subcommand: read-only DB ops (plan, pdf, note save) work without `LLM_API_KEY`; `rag search` needs the embedding API key. The helper takes a strict mode flag — `requireAPIKey bool` — so subcommands declare what they need. Each subcommand handler is structured the same way: parse flags → resolve DB → build App → marshal flags into the `json.RawMessage` shape the existing tool method expects → call `app.toolXxx(args)` → write the returned string to stdout.

**Tech Stack:** Go 1.24+, stdlib `flag` + `encoding/json`. Tests via `:memory:` SQLite + `t.TempDir()`. Lint-enforced via the pre-commit hook (`STYLE.md` rules apply: `verb noun: %w` error wraps, doc comments on new exported symbols, `_, _ = fmt.Fprintf` for stderr writes).

**Pre-prep status:** All three Phase-2-prep fixes (`9c346cb`, `0c1e160`, `65621aa`) are deployed. `claw-cli` resolves paths via `CLAW_STUDY_ROOT`. UTF-8-safe truncation in place. `seed-memory` transactional + mtime-based.

---

## Reference: subcommand → tool mapping

| Subcommand | Tool method | Args struct | Needs API key |
|---|---|---|---|
| `rag search --query Q [--course C] [--top-k N]` | `(*App).toolRAGSearch` | `{query, course, top_k}` | **yes** (embedding) |
| `plan show --course C` | `(*App).LoadPlan` (direct) | — | no |
| `plan toggle --course C --task N` | `(*App).toolUpdatePlan` | `{plan_id, action="toggle", task_index}` | no |
| `course interests --course C` | `(*App).toolReadFile` (direct path) | — | no |
| `note save --course C --kind K --content TEXT` | `(*App).toolSaveNote` | `{course, kind, content}` | no |
| `pdf extract --id N [--pages R]` | `(*App).toolPDFExtract` | `{pdf_id, pages}` | no |
| `web fetch URL` | `agent.toolWebFetch` (free function) | `{url}` | no |
| `skill dispatch --skill NAME --topic T --course C` | `(*App).toolStudySkill` | `{skill, topic, course, count?}` | no |

(Some tool methods may have slightly different field names — implementer reads the source to confirm before encoding.)

---

## File structure

| File | Status | Responsibility |
|---|---|---|
| `claw-cli/main.go` | modify | Add `newAppFromEnv`, dispatcher cases for 7 new top-level subcommands (`rag`, `plan`, `course`, `note`, `pdf`, `web`, `skill`), each with their own subcommand handlers |
| `claw-cli/main_test.go` | modify | One end-to-end test per new subcommand using real DB + temp filesystem |

---

## Task 1 — `newAppFromEnv` helper + `rag search` subcommand

**Files:** `claw-cli/main.go`, `claw-cli/main_test.go`.

The first task establishes the `*agent.App` constructor pattern. All later subcommands reuse it.

- [ ] **Step 1: Read existing tool source** so the args JSON shape is correct:

```bash
cat ~/Documents/ITA/claw-study/agent/tools_rag.go
```

Confirm the args struct: `{query, course, top_k}` — JSON tags `"query"`, `"course"`, `"top_k"`.

- [ ] **Step 2: Append failing tests** to `claw-cli/main_test.go`:

```go
func TestRunRagSearchRequiresAPIKey(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("OPENCODE_API_KEY", "")
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "rag", "search", "--query", "x", "--db", dbPath}, &stdout, &stderr, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "LLM_API_KEY") {
		t.Fatalf("expected LLM_API_KEY error, got: %s", stderr.String())
	}
}

func TestRunRagSearchMissingQueryExits2(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("LLM_API_KEY", "stub")
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "rag", "search", "--db", dbPath}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}
```

(End-to-end RAG search against a real embedding API isn't worth testing in CI — too slow, requires network. The two tests above cover the CLI plumbing; the underlying `(*App).Search` is already covered by `agent/vectorstore_test.go`.)

- [ ] **Step 3: Run, verify fail** (`unknown subcommand: "rag"`).

```bash
cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./claw-cli -run TestRunRag -v
```

- [ ] **Step 4: Add the helper + dispatcher + handler** in `claw-cli/main.go`.

After the existing `resolveSkillsDir` helper, add:

```go
// newAppFromEnv constructs a fully-configured *agent.App from environment
// variables, mirroring the server's loadConfig. The dbPath argument is the
// already-resolved absolute path from resolveDBPath; this helper does NOT
// re-resolve. requireAPIKey controls whether a missing LLM_API_KEY/
// OPENCODE_API_KEY is a hard error: read-only subcommands pass false; RAG
// and any subcommand that hits the embedding API pass true.
func newAppFromEnv(dbPath string, requireAPIKey bool) (*agent.App, error) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENCODE_API_KEY")
	}
	if apiKey == "" && requireAPIKey {
		return nil, fmt.Errorf("LLM_API_KEY or OPENCODE_API_KEY must be set")
	}
	vaultRoot := os.Getenv("VAULT_ROOT")
	if vaultRoot == "" {
		vaultRoot = resolveStudyRoot()
	}
	if vaultRoot == "" {
		vaultRoot = "/workspace"
	}
	apiURL := os.Getenv("LLM_API_URL")
	if apiURL == "" {
		apiURL = os.Getenv("OPENCODE_API_URL")
	}
	if apiURL == "" {
		apiURL = "https://opencode.ai/zen/go/v1"
	}
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "qwen3.6-plus"
	}
	embeddingModel := os.Getenv("EMBEDDING_MODEL")
	if embeddingModel == "" {
		embeddingModel = "nomic-ai/nomic-embed-text-v1.5"
	}
	cfg := agent.Config{
		VaultRoot:      vaultRoot,
		APIKey:         apiKey,
		APIURL:         apiURL,
		Model:          model,
		EmbeddingModel: embeddingModel,
	}
	db, err := agent.OpenDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := agent.InitSchema(db); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return agent.NewApp(cfg, db), nil
}
```

In `runWithStdin`'s switch, add cases for the new top-level subcommands. Group them:

```go
	switch args[1] {
	case "memory":
		return runMemory(args[2:], stdin, stdout, stderr, dbPath)
	case "rag":
		return runRag(args[2:], stdout, stderr, dbPath)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown subcommand: %q\n", args[1])
		return 2
	}
```

Then add `runRag` and the `ragSearch` handler:

```go
func runRag(args []string, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli rag <search> [args]")
		return 2
	}
	switch args[0] {
	case "search":
		return ragSearch(args[1:], stdout, stderr, dbPath)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown rag subcommand: %q\n", args[0])
		return 2
	}
}

func ragSearch(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("rag search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	query := fs.String("query", "", "search query (required)")
	course := fs.String("course", "", "course id filter (optional)")
	topK := fs.Int("top-k", 3, "number of results (1-10)")
	dbOverride := fs.String("db", "", "path to study.db (default: $CLAW_STUDY_DB or $CLAW_STUDY_ROOT/data/study.db)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *query == "" {
		_, _ = fmt.Fprintln(stderr, "rag search: --query is required")
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	app, err := newAppFromEnv(resolvedDB, true)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer app.Close()
	argsJSON, _ := json.Marshal(map[string]any{
		"query":  *query,
		"course": *course,
		"top_k":  *topK,
	})
	_, _ = fmt.Fprintln(stdout, app.ToolRAGSearch(argsJSON))
	return 0
}
```

**Note:** `toolRAGSearch` is currently lowercase (private). Either:
(a) Rename the source to `ToolRAGSearch` (capitalize) — exports it, passes lint (`exported` rule wants doc comment), small refactor that touches the test file.
(b) Add a small exported wrapper `func (a *App) ToolRAGSearch(args json.RawMessage) string { return a.toolRAGSearch(args) }` next to it.
(c) Add a method `func (a *App) RagSearch(query, course string, topK int) string` that does the JSON-shaped work internally — cleaner long-term.

**Pick (a): rename `toolRAGSearch` to `ToolRAGSearch`** in `agent/tools_rag.go` and update any callers (mostly `agent/tools.go` registry — grep for it). Add a doc comment. Same approach for the other tools in subsequent tasks. Document the rename in the commit message; it's a small public-API expansion.

After the rename, lint will require a doc comment on the now-exported method. Add:

```go
// ToolRAGSearch executes a vector search over the indexed corpus and
// returns a human-readable list of hits. Args JSON shape:
//   {"query": "...", "course": "ce297", "top_k": 5}
// `course` is optional; `top_k` defaults to 3 and is clamped to [1, 10].
func (a *App) ToolRAGSearch(args json.RawMessage) string {
	// ... existing body ...
}
```

- [ ] **Step 5: Update existing callers of `toolRAGSearch`** (the tool dispatcher).

```bash
grep -rn 'toolRAGSearch' ~/Documents/ITA/claw-study/agent/
```

There's likely one site in `agent/tools.go` (the dispatcher map). Update to `ToolRAGSearch`.

- [ ] **Step 6: Run all tests, verify pass**

```bash
/opt/homebrew/bin/go test ./... 2>&1 | tail -10
```

The two new CLI tests pass; existing agent tests still green (rename was internal-to-package).

- [ ] **Step 7: Commit**

```bash
cd ~/Documents/ITA/claw-study
git add agent/tools_rag.go agent/tools.go claw-cli/main.go claw-cli/main_test.go
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "claw-cli: rag search subcommand + newAppFromEnv helper"
```

---

## Task 2 — `plan show` + `plan toggle`

Both wrap `agent/tools_plan.go`. Show is read-only and uses `(*App).LoadPlan` directly (no LLM tool path); toggle uses `ToolUpdatePlan` (renamed from `toolUpdatePlan`).

- [ ] **Step 1: Append tests** to `claw-cli/main_test.go`:

```go
func TestRunPlanShowEmptyPlanReturnsError(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir()) // empty vault → no plan files
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "plan", "show", "--course", "ce297", "--db", dbPath}, &stdout, &stderr, "")
	// Plan not found is a soft error: exit 1 with a clear message.
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "plan not found") {
		t.Fatalf("expected plan-not-found error, got: %s", stderr.String())
	}
}

func TestRunPlanToggleMissingTaskExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "plan", "toggle", "--course", "ce297", "--db", dbPath}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2 (missing --task)", code)
	}
}
```

- [ ] **Step 2: Add the dispatcher case** in `runWithStdin`'s switch:

```go
	case "plan":
		return runPlan(args[2:], stdout, stderr, dbPath)
```

- [ ] **Step 3: Add handlers**:

```go
func runPlan(args []string, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli plan <show|toggle> [args]")
		return 2
	}
	switch args[0] {
	case "show":
		return planShow(args[1:], stdout, stderr, dbPath)
	case "toggle":
		return planToggle(args[1:], stdout, stderr, dbPath)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown plan subcommand: %q\n", args[0])
		return 2
	}
}

func planShow(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("plan show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id (required)")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *course == "" {
		_, _ = fmt.Fprintln(stderr, "plan show: --course is required")
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
	defer app.Close()
	plan := app.LoadPlan(*course)
	if plan == nil {
		_, _ = fmt.Fprintf(stderr, "plan not found for course %q\n", *course)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(plan)
	return 0
}

func planToggle(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("plan toggle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id / plan id (required)")
	taskIndex := fs.Int("task", -1, "task index in plan (required, ≥0)")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *course == "" || *taskIndex < 0 {
		_, _ = fmt.Fprintln(stderr, "plan toggle: --course and --task (≥0) are required")
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
	defer app.Close()
	argsJSON, _ := json.Marshal(map[string]any{
		"plan_id":    *course,
		"action":     "toggle",
		"task_index": *taskIndex,
	})
	_, _ = fmt.Fprintln(stdout, app.ToolUpdatePlan(argsJSON))
	return 0
}
```

- [ ] **Step 4: Rename `toolUpdatePlan` → `ToolUpdatePlan`** in `agent/tools_plan.go`. Add a doc comment. Update the dispatcher in `agent/tools.go`.

- [ ] **Step 5: Run + commit**:

```bash
/opt/homebrew/bin/go test ./... 2>&1 | tail -10
git add agent/tools_plan.go agent/tools.go claw-cli/main.go claw-cli/main_test.go
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "claw-cli: plan show + plan toggle subcommands"
```

---

## Task 3 — `course interests`

Reads `<VaultRoot>/memory/courses/<id>/interests.md` directly. No tool method needed.

- [ ] **Step 1: Test**:

```go
func TestRunCourseInterestsReturnsFile(t *testing.T) {
	dbPath := newTempDB(t)
	vault := t.TempDir()
	dir := filepath.Join(vault, "memory", "courses", "ce297")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	body := "# CE-297 interests\n\nFormal methods angle on safety.\n"
	if err := os.WriteFile(filepath.Join(dir, "interests.md"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VAULT_ROOT", vault)

	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "course", "interests", "--course", "ce297", "--db", dbPath}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Formal methods angle on safety") {
		t.Fatalf("expected file contents in stdout, got: %s", stdout.String())
	}
}

func TestRunCourseInterestsMissingFileExits1(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "course", "interests", "--course", "no-such", "--db", dbPath}, &stdout, &stderr, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
}
```

- [ ] **Step 2: Dispatcher case + handler**:

```go
	case "course":
		return runCourse(args[2:], stdout, stderr, dbPath)
```

```go
func runCourse(args []string, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli course <interests> [args]")
		return 2
	}
	switch args[0] {
	case "interests":
		return courseInterests(args[1:], stdout, stderr, dbPath)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown course subcommand: %q\n", args[0])
		return 2
	}
}

func courseInterests(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("course interests", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id (required)")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *course == "" {
		_, _ = fmt.Fprintln(stderr, "course interests: --course is required")
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
	defer app.Close()
	path := app.VaultPath("memory", "courses", *course, "interests.md")
	body, err := os.ReadFile(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "interests not found at %q: %v\n", path, err)
		return 1
	}
	_, _ = stdout.Write(body)
	return 0
}
```

- [ ] **Step 3: Run + commit**:

```bash
/opt/homebrew/bin/go test ./claw-cli -v
git add claw-cli/main.go claw-cli/main_test.go
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "claw-cli: course interests subcommand"
```

---

## Task 4 — `note save`

Wraps `(*App).toolSaveNote` (rename → `ToolSaveNote`).

- [ ] **Step 1: Read source** to confirm args shape:

```bash
grep -A 20 'toolSaveNote' ~/Documents/ITA/claw-study/agent/tools_file.go
```

Args struct should be `{course, kind, content}` or similar. Confirm before encoding.

- [ ] **Step 2: Test**:

```go
func TestRunNoteSaveWritesFile(t *testing.T) {
	dbPath := newTempDB(t)
	vault := t.TempDir()
	t.Setenv("VAULT_ROOT", vault)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "note", "save", "--course", "ce297",
		"--kind", "fleeting", "--content", "test note from CLI",
		"--db", dbPath,
	}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	// Confirm a file was written somewhere under vault/memory/courses/ce297/fleeting/
	matches, _ := filepath.Glob(filepath.Join(vault, "memory", "courses", "ce297", "fleeting", "*.md"))
	if len(matches) == 0 {
		t.Fatalf("expected fleeting note written under vault, found none")
	}
}
```

- [ ] **Step 3: Dispatcher + handler**:

```go
	case "note":
		return runNote(args[2:], stdout, stderr, dbPath)
```

```go
func runNote(args []string, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli note <save> [args]")
		return 2
	}
	switch args[0] {
	case "save":
		return noteSave(args[1:], stdout, stderr, dbPath)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown note subcommand: %q\n", args[0])
		return 2
	}
}

func noteSave(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("note save", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id (required)")
	kind := fs.String("kind", "fleeting", "note kind (default: fleeting)")
	content := fs.String("content", "", "note body (required)")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *course == "" || *content == "" {
		_, _ = fmt.Fprintln(stderr, "note save: --course and --content are required")
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
	defer app.Close()
	argsJSON, _ := json.Marshal(map[string]any{
		"course":  *course,
		"kind":    *kind,
		"content": *content,
	})
	_, _ = fmt.Fprintln(stdout, app.ToolSaveNote(argsJSON))
	return 0
}
```

- [ ] **Step 4: Rename `toolSaveNote` → `ToolSaveNote`** in `agent/tools_file.go`. Doc comment. Update dispatcher in `agent/tools.go`.

- [ ] **Step 5: Run + commit**:

```bash
/opt/homebrew/bin/go test ./... 2>&1 | tail -10
git add agent/tools_file.go agent/tools.go claw-cli/main.go claw-cli/main_test.go
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "claw-cli: note save subcommand"
```

---

## Task 5 — `pdf extract`

Wraps `(*App).toolPDFExtract` (rename → `ToolPDFExtract`).

- [ ] **Step 1: Read source** for args shape (likely `{pdf_id, pages}`).

- [ ] **Step 2: Test** — seed a fake PDF row in the test DB, call `pdf extract --id <id>`, expect non-error output. (Real PDF extraction needs an actual file; the test uses a stubbed path and accepts that the tool returns a "not found" string but exits 0 — exit code 0 means CLI plumbing worked, not that extraction succeeded.)

```go
func TestRunPDFExtractInvalidIDReturnsErrorString(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "pdf", "extract", "--id", "999", "--db", dbPath}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	// stdout will contain whatever the tool returns for missing PDFs — likely an "error" string,
	// but the CLI plumbing exits 0 because it's the tool's text output, not a CLI failure.
	if stdout.Len() == 0 {
		t.Fatalf("expected some stdout output")
	}
}
```

- [ ] **Step 3: Dispatcher + handler** — same shape as note save. Args JSON: `{"pdf_id": *id, "pages": *pages}`. Flags: `--id` (int, required), `--pages` (string, optional), `--db`.

- [ ] **Step 4: Rename `toolPDFExtract` → `ToolPDFExtract`** + doc comment + dispatcher update.

- [ ] **Step 5: Commit**.

---

## Task 6 — `web fetch`

Wraps the free function `agent.toolWebFetch` (capitalize → `ToolWebFetch`).

- [ ] **Step 1: Test** — fetch a stable URL. Use `t.Skip` if the test machine has no network:

```go
func TestRunWebFetchOK(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "web", "fetch", "https://example.com", "--db", dbPath}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Example Domain") {
		t.Fatalf("expected example.com content, got: %s", stdout.String())
	}
}

func TestRunWebFetchMissingURLExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "web", "fetch", "--db", dbPath}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2 (missing URL)", code)
	}
}
```

- [ ] **Step 2: Dispatcher + handler**. URL is positional, not a flag:

```go
func webFetch(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("web fetch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		_, _ = fmt.Fprintln(stderr, "web fetch: URL argument required")
		return 2
	}
	url := fs.Arg(0)
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	// web fetch doesn't actually need the DB but resolveDBPath enforces existence,
	// which is a useful pre-flight signal that the env is correctly set.
	_ = resolvedDB
	argsJSON, _ := json.Marshal(map[string]any{"url": url})
	_, _ = fmt.Fprintln(stdout, agent.ToolWebFetch(argsJSON))
	return 0
}
```

- [ ] **Step 3: Rename `toolWebFetch` → `ToolWebFetch`** + doc comment + dispatcher.

- [ ] **Step 4: Commit**.

---

## Task 7 — `skill dispatch`

Wraps `(*App).toolStudySkill` (rename → `ToolStudySkill`).

- [ ] **Step 1: Read source** for args (likely `{skill, topic, course, count}`).

- [ ] **Step 2: Test** — skill dispatch generates a prompt locally (no LLM call) so it's testable with stubbed VAULT_ROOT:

```go
func TestRunSkillDispatchReturnsPrompt(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "skill", "dispatch",
		"--skill", "orientation", "--topic", "STAMP", "--course", "ce297",
		"--db", dbPath,
	}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatalf("expected non-empty prompt output")
	}
}

func TestRunSkillDispatchMissingFlagsExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "skill", "dispatch", "--db", dbPath}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}
```

- [ ] **Step 3: Handler** — flags `--skill`, `--topic`, `--course`, optional `--count`, `--db`. Marshal to JSON, call `app.ToolStudySkill(argsJSON)`.

- [ ] **Step 4: Rename + commit**.

---

## Task 8 — Cross-compile, deploy, smoke-test on VPS

**Files:** none.

This is the production deploy. Same pattern as Phase 1 Task 8 + pre-prep Task 4.

- [ ] **Step 1: Cross-compile** (`study-app` because the agent package now has 5 newly-exported methods; `claw-cli` because that's the new surface; `seed-memory` is unchanged but consistency keeps the deploy atomic).

```bash
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/claw-cli-linux ./claw-cli
ls -la /tmp/study-app-linux /tmp/claw-cli-linux
```

- [ ] **Step 2: Hot-swap server + claw-cli**

```bash
scp /tmp/study-app-linux nanoclaw:$VAULT_ROOT/bin/study-app.new
scp /tmp/claw-cli-linux nanoclaw:$VAULT_ROOT/bin/claw-cli
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak && mv study-app.new study-app && chmod +x study-app claw-cli && export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service && sleep 3 && systemctl --user is-active study-app.service'
```

Expected: `active`. Rollback procedure same as Phase 1 Task 8 if needed.

- [ ] **Step 3: Smoke each new subcommand**

```bash
ssh nanoclaw 'cd $VAULT_ROOT && set -e
echo "=== plan show ==="; ./bin/claw-cli plan show --course ce297 | head -10 || echo "no plan"
echo "=== plan toggle (dry — wrong index intentional) ==="; ./bin/claw-cli plan toggle --course ce297 --task 999
echo "=== course interests ==="; ./bin/claw-cli course interests --course ce297 | head -5 || echo "no file"
echo "=== note save ==="; ./bin/claw-cli note save --course ce297 --kind fleeting --content "phase2 smoke"
echo "=== pdf extract (id=999, expected error) ==="; ./bin/claw-cli pdf extract --id 999 | head -3
echo "=== web fetch ==="; ./bin/claw-cli web fetch https://example.com | head -10
echo "=== skill dispatch ==="; ./bin/claw-cli skill dispatch --skill orientation --topic STAMP --course ce297 | head -20
'
```

Expected: every subcommand prints something to stdout (whether the tool's success or its error string), no shell-level "command not found" or unhandled crashes. Capture the output for the deploy log.

- [ ] **Step 4: Smoke `rag search` (needs real API key from .env)**

```bash
ssh nanoclaw 'cd $VAULT_ROOT && set -a && source .env && set +a && ./bin/claw-cli rag search --query "STAMP" --course ce297 --top-k 3 | head -30'
```

Expected: top-3 RAG hits printed. If there's no indexed corpus yet, the tool returns `"No relevant results found for: STAMP"` — that's still a valid CLI exit 0.

- [ ] **Step 5: Verify the live `/chat` UI still works** (the renames to public methods could in principle break the in-server tool dispatcher):

```bash
TOKEN=$(ssh nanoclaw 'grep ^AUTH_TOKEN= ~/stack/study-app/.env | cut -d= -f2')
curl -s -o /dev/null -w "%{http_code}\n" -H "Authorization: Bearer $TOKEN" https://your-host.example/debug/health
```

Expected: 200.

- [ ] **Step 6: Append to `docs/specs/proposals/phase1-deploy-log.md`** (yes, same file — chronological log). Section title: `## Phase 2 deploy 2026-05-10`. Record:
  - Commit range
  - Each subcommand's smoke result (1 line each)
  - rag search latency / hit count
  - /debug/health 200 confirmation

- [ ] **Step 7: Commit + push**

```bash
cd ~/Documents/ITA/claw-study
git add docs/specs/proposals/phase1-deploy-log.md
git -c user.email=you@example.com -c user.name=your-name \
  commit -m "docs: phase 2 deploy log"
git push origin main
```

---

## Self-review checklist

- ✅ All 8 subcommands present in `claw-cli`: rag search, plan show, plan toggle, course interests, note save, pdf extract, web fetch, skill dispatch
- ✅ Each subcommand has at least one CLI-plumbing test (network/LLM-dependent ones may use `t.Skip(testing.Short())`)
- ✅ All renames `toolXxx` → `ToolXxx` propagated to dispatchers and tests in `agent/`
- ✅ `newAppFromEnv` correctly distinguishes API-required vs. read-only paths
- ✅ Pre-commit hook didn't fight us (or, if it did, the issues were real and got fixed)
- ✅ Each subcommand uses `_, _ = fmt.Fprintf` for stderr writes (errcheck rule for new code)
- ✅ Live `/chat` UI still 200 after deploy
- ✅ All commits pushed
- ⚠️ Pre-existing 86 Go violations untouched (not in scope)
