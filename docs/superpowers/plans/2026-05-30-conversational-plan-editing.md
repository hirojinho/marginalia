# Conversational Plan Editing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `claw-cli plan rewrite` so the live Pi tutor can restructure a study plan in-flow (split / add / rename / reorder / remove tasks) via one deterministic, validated write.

**Architecture:** New `planRewrite` function in `claw-cli/main.go`, dispatched from `runPlan`, wrapping the existing `App.ToolRewritePlan` (which validates JSON, requires `plan_json.id == course`, and respects provided task `id`s so session anchors survive). One AGENTS.md guidance edit in `agent/sandbox.go` so the agent uses it (read → edit JSON → rewrite, preserving `id`s). `show/status/toggle` unchanged; rail refresh is free via the existing fingerprint mechanism in `handler/chat_v2.go`. Docs (ADR 0017 + CONTEXT.md) already committed (`285c229`).

**Tech Stack:** Go 1.26 (build with `/opt/homebrew/bin/go`), `flag` CLI, SQLite via `agent.App`, JSON plans on disk under `VAULT_ROOT/data/plans/<course>.json`. Tests in `claw-cli/main_test.go` using `run(...)`, `newTempDB(t)`, and `t.Setenv("VAULT_ROOT", ...)`.

---

## File Structure

- **`claw-cli/main.go`** — add `case "rewrite"` to `runPlan` (line 392) + update its usage string; add `planRewrite` function (mirrors `planToggle` at line 512). *(Task 1)*
- **`claw-cli/main_test.go`** — 4 new tests for `plan rewrite`. *(Task 1)*
- **`agent/sandbox.go`** — extend the `planSection` string in `writeAgentsMD` (line 158) with an "Editing the plan" block + two more `course` args. *(Task 2)*

> **Test harness gotcha (read before Task 1):** `openApp(t, dbPath)` builds the App with a *fresh* `t.TempDir()` vault, NOT the env `VAULT_ROOT`, so it CANNOT see a plan written by `run(...)`. Verify written plans by calling `claw-cli plan show` through `run(...)` (it uses `newAppFromEnv` → env `VAULT_ROOT`) and parsing its JSON output — do NOT use `openApp`/`LoadPlan` for plan-file assertions.

---

## Task 1: `claw-cli plan rewrite` command + tests

**Files:**
- Modify: `claw-cli/main.go` (`case "rewrite"` + usage in `runPlan`; new `planRewrite` func)
- Test: `claw-cli/main_test.go` (append 4 tests after `TestCourseSettingsSetRejectsBadKey` / at end)

- [ ] **Step 1: Write the failing tests**

Append to `claw-cli/main_test.go`:

```go
func TestPlanRewriteValidFileCreatesAndPreservesIDs(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir()) // plans live under VAULT_ROOT/data/plans/
	planJSON := `{"id":"rw-course","name":"RW","phases":[{"title":"P1","tasks":[` +
		`{"id":"keep-123","title":"Existing","done":false},` +
		`{"title":"Fresh","done":false}]}]}`
	planFile := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planFile, []byte(planJSON), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}
	var out, errb bytes.Buffer
	code := run([]string{"clawcli", "plan", "rewrite", "--course", "rw-course", "--plan-file", planFile}, &out, &errb, dbPath)
	if code != 0 {
		t.Fatalf("rewrite exit %d, stderr: %s", code, errb.String())
	}
	// Verify via `plan show` (NOT openApp — openApp uses a different vault).
	out.Reset()
	errb.Reset()
	if code := run([]string{"clawcli", "plan", "show", "--course", "rw-course"}, &out, &errb, dbPath); code != 0 {
		t.Fatalf("show exit %d, stderr: %s", code, errb.String())
	}
	var shown struct {
		Phases []struct {
			Tasks []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"tasks"`
		} `json:"phases"`
	}
	if err := json.Unmarshal(out.Bytes(), &shown); err != nil {
		t.Fatalf("parse show output: %v\n%s", err, out.String())
	}
	if len(shown.Phases) != 1 || len(shown.Phases[0].Tasks) != 2 {
		t.Fatalf("unexpected plan shape: %+v", shown)
	}
	tasks := shown.Phases[0].Tasks
	var existingID, freshID string
	for _, tk := range tasks {
		switch tk.Title {
		case "Existing":
			existingID = tk.ID
		case "Fresh":
			freshID = tk.ID
		}
	}
	if existingID != "keep-123" {
		t.Fatalf("explicit id not preserved, got %q", existingID)
	}
	if freshID == "" || freshID == "keep-123" {
		t.Fatalf("new task did not get a fresh uuid, got %q", freshID)
	}
}

func TestPlanRewriteBadJSONExits1(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir())
	planFile := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(planFile, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var out, errb bytes.Buffer
	code := run([]string{"clawcli", "plan", "rewrite", "--course", "rw-course", "--plan-file", planFile}, &out, &errb, dbPath)
	if code != 1 {
		t.Fatalf("want exit 1 on bad JSON, got %d", code)
	}
	if !strings.Contains(errb.String(), "error") {
		t.Fatalf("stderr: %s", errb.String())
	}
}

func TestPlanRewriteIDMismatchExits1(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir())
	planFile := filepath.Join(t.TempDir(), "mismatch.json")
	if err := os.WriteFile(planFile, []byte(`{"id":"other","name":"X","phases":[]}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var out, errb bytes.Buffer
	code := run([]string{"clawcli", "plan", "rewrite", "--course", "rw-course", "--plan-file", planFile}, &out, &errb, dbPath)
	if code != 1 {
		t.Fatalf("want exit 1 on id mismatch, got %d (stderr: %s)", code, errb.String())
	}
}

func TestPlanRewriteMissingFlagsExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var out, errb bytes.Buffer
	code := run([]string{"clawcli", "plan", "rewrite", "--course", "rw-course"}, &out, &errb, dbPath)
	if code != 2 {
		t.Fatalf("want exit 2 when --plan-file missing, got %d (stderr: %s)", code, errb.String())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./claw-cli/ -run TestPlanRewrite -v`
Expected: FAIL — `plan rewrite` falls through `runPlan`'s default (exit 2), so the valid/bad/mismatch tests fail (the missing-flags one may pass for the wrong reason; that's fine, it'll pass for the right reason after Step 4).

- [ ] **Step 3: Confirm imports**

Run: `grep -nE '"os"|"strings"|"encoding/json"' claw-cli/main.go`
Expected: all three present (they are — `os.ReadFile`, `strings`, and `json.Marshal` are used elsewhere in the file). If any is missing, add it to the import block.

- [ ] **Step 4: Add the `planRewrite` function**

Add to `claw-cli/main.go`, immediately after `planToggle` (ends at line 543):

```go
func planRewrite(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("plan rewrite", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id / plan id (required)")
	planFile := fs.String("plan-file", "", "path to a JSON file holding the full new plan (required)")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *course == "" || *planFile == "" {
		_, _ = fmt.Fprintln(stderr, "plan rewrite: --course and --plan-file are required")
		return 2
	}
	planBytes, err := os.ReadFile(*planFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "plan rewrite: reading %q: %v\n", *planFile, err)
		return 1
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
	argsJSON, _ := json.Marshal(map[string]any{ // string values cannot fail to marshal
		"plan_id":   *course,
		"plan_json": string(planBytes),
	})
	result := app.ToolRewritePlan(argsJSON)
	if strings.HasPrefix(result, "error") {
		_, _ = fmt.Fprintln(stderr, result)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, result)
	return 0
}
```

- [ ] **Step 5: Wire the dispatch + usage string**

In `runPlan` (`claw-cli/main.go:387`), change:

```go
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli plan <show|status|toggle> [args]")
		return 2
	}
	switch args[0] {
	case "show":
		return planShow(args[1:], stdout, stderr, dbPath)
	case "status":
		return planStatus(args[1:], stdout, stderr, dbPath)
	case "toggle":
		return planToggle(args[1:], stdout, stderr, dbPath)
	default:
```

to:

```go
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli plan <show|status|toggle|rewrite> [args]")
		return 2
	}
	switch args[0] {
	case "show":
		return planShow(args[1:], stdout, stderr, dbPath)
	case "status":
		return planStatus(args[1:], stdout, stderr, dbPath)
	case "toggle":
		return planToggle(args[1:], stdout, stderr, dbPath)
	case "rewrite":
		return planRewrite(args[1:], stdout, stderr, dbPath)
	default:
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./claw-cli/ -run TestPlanRewrite -v`
Expected: PASS (all 4).

- [ ] **Step 7: Run the full claw-cli + agent + handler suites**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./claw-cli/ ./agent/ ./handler/`
Expected: all `ok`.

- [ ] **Step 8: Commit**

```bash
cd ~/Documents/ITA/claw-study
git add claw-cli/main.go claw-cli/main_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "$(cat <<'EOF'
feat(plan): claw-cli plan rewrite so the tutor can restructure a plan

Wraps the existing validated ToolRewritePlan (parses, id must match course,
preserves provided task ids → session anchors survive). Errors surface as
exit 1, unlike toggle. Closes the session-46 'can't edit the plan' gap (ADR 0017).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Agent guidance in `writeAgentsMD`

**Files:**
- Modify: `agent/sandbox.go` (the `planSection` `fmt.Sprintf` at line 158-161)

Prose edit (no new unit test — matches how the sibling guidance blocks are maintained; verified by clean build + agent suite + grep, then live at deploy). The current `planSection` template ends with `...never edit a markdown plan file.\n` and is formatted with four `course` args. We append an "Editing the plan" block (adds two `%s`) and add two `course` args.

- [ ] **Step 1: Replace the `planSection` statement**

In `agent/sandbox.go`, replace this exact block (lines 158-161):

```go
		planSection := fmt.Sprintf(
			"\n## Study plan — JSON is the only source of truth\n\nThe canonical plan for course %q lives in `data/plans/%s.json` and is rendered by the UI. **The markdown plans (`study-plan.md`) were retired 2026-05-14 — do NOT read or write any `study-plan.md` file, even if a path appears in your conversation history.** Those files no longer reflect reality.\n\n**Before answering any question about plan tasks, status, or progress, run:**\n```\nclaw-cli plan status --course %s\n```\nThis prints the authoritative current state with a `#N` linear index per task. To mark a task done/undone, run:\n```\nclaw-cli plan toggle --course %s --task <N>\n```\nNever answer plan questions from memory, and never edit a markdown plan file.\n",
			course, course, course, course,
		)
```

with:

```go
		planSection := fmt.Sprintf(
			"\n## Study plan — JSON is the only source of truth\n\nThe canonical plan for course %q lives in `data/plans/%s.json` and is rendered by the UI. **The markdown plans (`study-plan.md`) were retired 2026-05-14 — do NOT read or write any `study-plan.md` file, even if a path appears in your conversation history.** Those files no longer reflect reality.\n\n**Before answering any question about plan tasks, status, or progress, run:**\n```\nclaw-cli plan status --course %s\n```\nThis prints the authoritative current state with a `#N` linear index per task. To mark a task done/undone, run:\n```\nclaw-cli plan toggle --course %s --task <N>\n```\nNever answer plan questions from memory, and never edit a markdown plan file.\n\n**Editing the plan (it is a live document).** When Eduardo asks to restructure it — split one task into several, add, rename, reorder, or remove tasks — edit it directly. Read the full plan JSON:\n```\nclaw-cli plan show --course %s\n```\nEdit that JSON, write the whole plan to a temp file, then submit it:\n```\nclaw-cli plan rewrite --course %s --plan-file <tmp.json>\n```\n**Keep each task's existing `id`** on tasks that continue existing work — a renamed or split-from task keeps its `id` so its session stays attached; leave `id` empty only for genuinely new tasks (they get fresh UUIDs). Make the change, confirm in one line, and resume the study work — do not turn a study session into a plan-editing conversation, and do not restructure unasked.\n",
			course, course, course, course, course, course,
		)
```

- [ ] **Step 2: Build the server binary**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go build -o /tmp/study-app-check .`
Expected: no output (compiles; the `fmt.Sprintf` arg count now matches the six `%`-verbs: `%q`, `%s`×5).

- [ ] **Step 3: Run the agent suite**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./agent/`
Expected: `ok`.

- [ ] **Step 4: Verify the new guidance is present**

Run: `grep -c 'plan rewrite --course' agent/sandbox.go`
Expected: `1` (or more) — the rewrite instruction is in the template.

- [ ] **Step 5: Commit**

```bash
cd ~/Documents/ITA/claw-study
git add agent/sandbox.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "$(cat <<'EOF'
feat(agent): tell the tutor the plan is editable via claw-cli plan rewrite

Adds an "Editing the plan" block to the generated AGENTS.md: read plan show,
edit the JSON, plan rewrite; keep ids on continuing tasks to preserve session
anchors; one-shot, confirm, resume (ADR 0017).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Deploy

**Files:** none. Touches `claw-cli` (new subcommand) AND `study-app` (sandbox.go AGENTS.md) → both binaries. No skill file changed this time.

- [ ] **Step 1: Pre-deploy sync check**

```bash
cd ~/Documents/ITA/claw-study && git fetch origin && git rev-list --left-right --count origin/main...HEAD
```
Expected: confirm ahead/behind. If behind, `git merge origin/main --no-edit` and re-run `/opt/homebrew/bin/go test ./claw-cli/ ./agent/ ./handler/` before deploying (concurrent sessions commit to main).

- [ ] **Step 2: Push (requires explicit user OK — direct-to-main is harness-gated)**

```bash
cd ~/Documents/ITA/claw-study && git push origin main
```

- [ ] **Step 3: Build both linux binaries**

```bash
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/claw-cli-linux ./claw-cli
ls -la /tmp/study-app-linux /tmp/claw-cli-linux
```
Expected: two ELF binaries (~20 MB study-app, ~15 MB claw-cli).

- [ ] **Step 4: Deploy both binaries with backups, restart**

```bash
cd ~/Documents/ITA/claw-study
scp -q /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
scp -q /tmp/claw-cli-linux nanoclaw:/home/eduardo/stack/study-app/bin/claw-cli.new
ssh nanoclaw 'cd ~/stack/study-app/bin && \
  cp study-app study-app.bak.2026-05-30-plan-rewrite && mv study-app.new study-app && chmod +x study-app && \
  cp claw-cli claw-cli.bak.2026-05-30-plan-rewrite && mv claw-cli.new claw-cli && chmod +x claw-cli && \
  export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service && \
  sleep 2 && systemctl --user is-active study-app.service'
```
Expected: `active`.

- [ ] **Step 5: Live smoke test (on a disposable plan)**

```bash
DB=/home/eduardo/stack/study-app/data/study.db
VR=/home/eduardo/stack/study-app
# Seed a tiny plan via rewrite (also proves create-when-absent):
ssh nanoclaw "printf '%s' '{\"id\":\"plan-smoke\",\"name\":\"Smoke\",\"phases\":[{\"title\":\"P\",\"tasks\":[{\"title\":\"One\"}]}]}' > /tmp/p.json && VAULT_ROOT=$VR /usr/local/bin/claw-cli plan rewrite --course plan-smoke --plan-file /tmp/p.json --db $DB"
# Read it back, capture the generated id of task "One":
ssh nanoclaw "VAULT_ROOT=$VR /usr/local/bin/claw-cli plan show --course plan-smoke --db $DB"
# Edit: keep that id (rename "One"->"One renamed") and add a blank-id task, rewrite, show again — expect id preserved + new uuid:
#   (do this by hand from the show output to confirm anchor preservation)
# Cleanup:
ssh nanoclaw "rm -f $VR/data/plans/plan-smoke.json"
```
Expected: rewrite prints `rewrote plan "plan-smoke": ...`; show returns the plan with task "One" bearing a uuid; the rename+add round-trip preserves that uuid and assigns a fresh one to the added task; cleanup removes the file.

- [ ] **Step 6: Update memory**

In `claw_study_experience_redesign.md`, add a bullet that conversational plan editing shipped: `claw-cli plan rewrite` + AGENTS.md block + ADR 0017/CONTEXT, deployed, closing the session-46 gap.

---

## Self-Review Notes

- **Spec coverage:** Task 1 = CLI `plan rewrite` wrapping `ToolRewritePlan` with exit-1-on-error (spec §Design.1, §Testing). Task 2 = AGENTS.md "Editing the plan" block with id-preservation + one-shot discipline (spec §Design.2). §Design.3 (live refresh) needs no work (already in `chat_v2.go`). Docs (ADR 0017 + CONTEXT) already committed `285c229`. Deploy = Task 3 (spec §Deploy notes). The out-of-scope items (bespoke split/add/rename, frontend, orphan-guard, skill edits) are touched by no task.
- **Type/contract consistency:** `planRewrite(args, stdout, stderr, dbPath)` matches the sibling signatures; `ToolRewritePlan` takes `{plan_id, plan_json}` (confirmed `agent/tools_plan.go:138-141`) and returns `"error..."`/`"rewrote plan..."` strings; the `strings.HasPrefix(result, "error")` gate matches those returns. `planShow` emits the plan as JSON with `id`/`title` task fields, which the test parses. The `fmt.Sprintf` in Task 2 has six `%`-verbs and six `course` args.
- **Placeholders:** none — all code and commands are complete.
