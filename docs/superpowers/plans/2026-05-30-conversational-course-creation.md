# Conversational Course Creation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `claw-cli course create` subcommand so the live Pi tutor can create a course (with optional initial framing/exam_style) during the conversational Authoring flow.

**Architecture:** New `courseCreate` function in `claw-cli/main.go`, dispatched from `runCourse`, composing the already-validated `CreateCourse` + `SetCourseSetting` App methods (no new DB logic). Two prose wiring edits — `agent/sandbox.go` `writeAgentsMD` (so the agent discovers the command) and `skills/course-study-path/SKILL.md` (so the Authoring flow creates the course before saving its plan). No frontend changes; the legacy `/chat` `create_course` Go tool is untouched.

**Tech Stack:** Go 1.26 (build with `/opt/homebrew/bin/go`), `flag` package CLI, SQLite via `agent.App`. Tests are Go table-free unit tests in `claw-cli/main_test.go` using the existing `run(...)`, `newTempDB(t)`, `openApp(t, dbPath)` helpers.

---

## File Structure

- **`claw-cli/main.go`** — add `courseCreate(args, stdout, stderr, dbPath)` (mirrors `courseSettingsSet` at line 643), a package-level `courseIDRe` kebab regex, a `case "create"` in `runCourse` (line 546), and update the `runCourse` usage string. *(Task 1)*
- **`claw-cli/main_test.go`** — 4 new tests for `course create`. *(Task 1)*
- **`agent/sandbox.go`** — append a `## Creating a course` subsection in `writeAgentsMD` right after the `steerTool` block (line 206). *(Task 2)*
- **`skills/course-study-path/SKILL.md`** — add a "create the course first" step to Self-Study Mode Phase 4 (line 89) and Plan Mode Phase 1 (line 112). *(Task 3)*

---

## Task 1: `claw-cli course create` command + tests

**Files:**
- Modify: `claw-cli/main.go` (add regex var near top; add `case "create"` in `runCourse` at `claw-cli/main.go:546`; add `courseCreate` func after `courseInterests`/before `courseSettings`)
- Test: `claw-cli/main_test.go` (append 4 tests after `TestCourseSettingsSetRejectsBadKey` at line 654)

- [ ] **Step 1: Write the failing tests**

Append to `claw-cli/main_test.go`:

```go
func TestCourseCreateInsertsRow(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "course", "create",
		"--id", "new-course", "--name", "Brand New Course",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("create exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Created course") {
		t.Fatalf("expected confirmation, got: %s", stdout.String())
	}
	app := openApp(t, dbPath)
	defer func() { _ = app.Close() }()
	c, err := app.GetCourse("new-course")
	if err != nil {
		t.Fatalf("GetCourse: %v", err)
	}
	if c.ID != "new-course" || c.Name != "Brand New Course" {
		t.Fatalf("course not persisted, got %+v", c)
	}
}

func TestCourseCreateWithSettings(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "course", "create",
		"--id", "framed-course", "--name", "Framed",
		"--framing", "exam-prep lens", "--exam-style", "short essays",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("create exit %d, stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"clawcli", "course", "settings", "get", "--course", "framed-course",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("get exit %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "framing: exam-prep lens") {
		t.Fatalf("framing not persisted:\n%s", out)
	}
	if !strings.Contains(out, "exam_style: short essays") {
		t.Fatalf("exam_style not persisted:\n%s", out)
	}
}

func TestCourseCreateDuplicateExits1(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	run([]string{"clawcli", "course", "create", "--id", "dup-course", "--name", "First"}, &stdout, &stderr, dbPath)
	stdout.Reset()
	stderr.Reset()
	code := run([]string{"clawcli", "course", "create", "--id", "dup-course", "--name", "Second"}, &stdout, &stderr, dbPath)
	if code != 1 {
		t.Fatalf("want exit 1 on duplicate, got %d", code)
	}
	if !strings.Contains(stderr.String(), "course already exists") {
		t.Fatalf("stderr: %s", stderr.String())
	}
}

func TestCourseCreateInvalidIDExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "course", "create", "--id", "Bad ID", "--name", "X"}, &stdout, &stderr, dbPath)
	if code != 2 {
		t.Fatalf("want exit 2 on invalid id, got %d (stderr: %s)", code, stderr.String())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./claw-cli/ -run TestCourseCreate -v`
Expected: compile error / FAIL — `courseCreate` does not exist yet and `course create` falls through to `runCourse`'s default (exit 2), so the insert/settings tests fail.

- [ ] **Step 3: Add the kebab regex var**

Confirm `regexp` is already imported in `claw-cli/main.go` (run `grep -n '"regexp"' claw-cli/main.go`). If absent, add `"regexp"` to the import block.

Add this package-level var near the top of `claw-cli/main.go` (e.g. just after the imports, beside other package-level vars):

```go
// courseIDRe is the kebab-case rule shared with POST /api/courses (handler/courses.go).
var courseIDRe = regexp.MustCompile(`^[a-z0-9-]+$`)
```

- [ ] **Step 4: Add the `courseCreate` function**

Add this function to `claw-cli/main.go` (place it next to `courseInterests`/`courseSettings`, e.g. immediately before `func courseSettings`):

```go
func courseCreate(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("course create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "course id (kebab-case, required)")
	name := fs.String("name", "", "course display name (required)")
	framing := fs.String("framing", "", "optional initial framing (course_settings)")
	examStyle := fs.String("exam-style", "", "optional initial exam_style (course_settings)")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *id == "" || *name == "" {
		_, _ = fmt.Fprintln(stderr, "course create: --id and --name are required")
		return 2
	}
	if !courseIDRe.MatchString(*id) {
		_, _ = fmt.Fprintln(stderr, "course create: --id must be kebab-case (lowercase letters, digits, hyphens only)")
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
	if err := app.CreateCourse(*id, *name); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "Created course %q (id: %s)\n", *name, *id)
	if *framing != "" {
		if err := app.SetCourseSetting(*id, "framing", *framing); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "set framing for course %s\n", *id)
	}
	if *examStyle != "" {
		if err := app.SetCourseSetting(*id, "exam_style", *examStyle); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "set exam_style for course %s\n", *id)
	}
	return 0
}
```

- [ ] **Step 5: Wire the dispatch**

In `runCourse` (`claw-cli/main.go:546`), add a `create` case and update the usage string. Change:

```go
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
```

to:

```go
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli course <interests|settings|create> [args]")
		return 2
	}
	switch args[0] {
	case "interests":
		return courseInterests(args[1:], stdout, stderr)
	case "settings":
		return courseSettings(args[1:], stdout, stderr, dbPath)
	case "create":
		return courseCreate(args[1:], stdout, stderr, dbPath)
	default:
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./claw-cli/ -run TestCourseCreate -v`
Expected: PASS (all 4 tests).

- [ ] **Step 7: Run the full claw-cli + agent suites to check for regressions**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./claw-cli/ ./agent/ ./handler/`
Expected: all packages `ok`.

- [ ] **Step 8: Commit**

```bash
cd ~/Documents/ITA/claw-study
git add claw-cli/main.go claw-cli/main_test.go
git -c user.email=you@example.com -c user.name=your-name commit -m "$(cat <<'EOF'
feat(course): claw-cli course create with optional framing/exam_style

Composes CreateCourse + SetCourseSetting (no new write logic); kebab-case
validation matches POST /api/courses. Brings the Pi path to parity with
the legacy /chat create_course tool.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Agent discovery in `writeAgentsMD`

**Files:**
- Modify: `agent/sandbox.go` (after the `steerTool` append at `agent/sandbox.go:206`)

This is a prose edit — no new test (matches how the existing `steerTool`/`## Slides / PDFs` guidance blocks are maintained; verified at deploy via `strings`/disk grep). The change is verified by a clean build + a grep of the produced string in the binary.

- [ ] **Step 1: Add the create-course guidance block**

In `agent/sandbox.go`, immediately after this line (line 206):

```go
	content = append(content, []byte(steerTool)...)
```

insert:

```go
	// How to create a course conversationally (Authoring; ADR 0014).
	createCourse := "\n## Creating a course\n\n" +
		"If Eduardo starts studying a subject that is not already one of his courses, create it before saving any plan or memory for it:\n" +
		"```\nclaw-cli course create --id <kebab-case-slug> --name \"<display name>\" [--framing \"<how to teach it>\"] [--exam-style \"<assessment style>\"]\n```\n" +
		"Pick a stable kebab-case id (lowercase letters, digits, hyphens) — ids are permanent. " +
		"This only registers the course; build the study plan with the course-study-path skill as usual.\n"
	content = append(content, []byte(createCourse)...)
```

- [ ] **Step 2: Build the server binary to verify it compiles**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go build .`
Expected: no output (success).

- [ ] **Step 3: Run the agent suite**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./agent/`
Expected: `ok`.

- [ ] **Step 4: Commit**

```bash
cd ~/Documents/ITA/claw-study
git add agent/sandbox.go
git -c user.email=you@example.com -c user.name=your-name commit -m "$(cat <<'EOF'
feat(agent): tell the tutor it can create courses via claw-cli course create

Adds a "## Creating a course" block to the generated AGENTS.md so the Pi
agent knows to register a new course before saving its plan/memory.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Authoring-flow wiring in `course-study-path` skill

**Files:**
- Modify: `skills/course-study-path/SKILL.md` (Self-Study Mode Phase 4 at line 89; Plan Mode Phase 1 at line 112)

Prose edit to a disk-mounted skill file (no test; verified by reading the file back and, at deploy, scp + disk grep).

- [ ] **Step 1: Add a create-course step to Self-Study Mode Phase 4**

In `skills/course-study-path/SKILL.md`, the section starting at line 89 reads:

```markdown
### Phase 4 — Create Files

Write the following files to `out/` so the user can copy them to the right place on their local machine:
```

Change it to:

```markdown
### Phase 4 — Create Files

**First, register the course if it does not already exist.** Check the known courses; if `<topic>` is new, run `claw-cli course create --id <topic> --name "<display name>"` (optionally `--framing`/`--exam-style`) before the memory-save step below — otherwise `claw-cli memory save --course <topic>` points at a course the app does not list.

Write the following files to `out/` so the user can copy them to the right place on their local machine:
```

- [ ] **Step 2: Add a create-course step to Plan Mode Phase 1**

In the same file, the section at line 112 reads:

```markdown
### Phase 1 — Course Profile (first invocation only)

If no course profile exists in memory or context, collect it before creating tasks:
```

Change it to:

```markdown
### Phase 1 — Course Profile (first invocation only)

If this is a brand-new course (not in the known-courses list), register it first with `claw-cli course create --id <slug> --name "<display name>"` (optionally `--framing`/`--exam-style`), then collect the profile.

If no course profile exists in memory or context, collect it before creating tasks:
```

- [ ] **Step 3: Verify the edits**

Run: `cd ~/Documents/ITA/claw-study && grep -n 'claw-cli course create' skills/course-study-path/SKILL.md`
Expected: two matches (one in Self-Study Phase 4, one in Plan Mode Phase 1).

- [ ] **Step 4: Commit**

```bash
cd ~/Documents/ITA/claw-study
git add skills/course-study-path/SKILL.md
git -c user.email=you@example.com -c user.name=your-name commit -m "$(cat <<'EOF'
docs(skill): course-study-path creates the course before saving its plan

Closes the gap where the Authoring flow ran memory save / plan writes
against a course that was never registered in the DB.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Deploy

**Files:** none (build + ship). Touches `claw-cli` AND `study-app` (sandbox.go) AND a disk-mounted skill — all three deploy paths.

- [ ] **Step 1: Pre-deploy sync check**

```bash
cd ~/Documents/ITA/claw-study && git fetch origin && git rev-list --left-right --count origin/main...HEAD
```
Expected: confirm whether ahead/behind. If behind, `git merge origin/main --no-edit` and re-run tests before deploying (this repo has concurrent sessions committing to `main`).

- [ ] **Step 2: Push (requires explicit user OK — direct-to-main is blocked by the harness)**

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
Expected: two ELF binaries (~18 MB study-app).

- [ ] **Step 4: Deploy study-app, claw-cli, and the skill**

```bash
# study-app
scp /tmp/study-app-linux nanoclaw:$VAULT_ROOT/bin/study-app.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak.2026-05-30-course-create && mv study-app.new study-app && chmod +x study-app'
# claw-cli
scp /tmp/claw-cli-linux nanoclaw:$VAULT_ROOT/bin/claw-cli.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp claw-cli claw-cli.bak.2026-05-30-course-create && mv claw-cli.new claw-cli && chmod +x claw-cli'
# skill (disk-mounted, not carried by the binary)
scp skills/course-study-path/SKILL.md nanoclaw:$VAULT_ROOT/skills/course-study-path/SKILL.md
# restart
ssh nanoclaw 'export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service'
```

- [ ] **Step 5: Live smoke test**

```bash
ssh nanoclaw 'export XDG_RUNTIME_DIR=/run/user/$(id -u); systemctl --user is-active study-app.service'
# create via the new command, confirm it lands, then clean up:
ssh nanoclaw '/usr/local/bin/claw-cli course create --id smoke-course --name "Smoke Test" --framing "verify path"'
ssh nanoclaw '/usr/local/bin/claw-cli course settings get --course smoke-course'   # expect framing: verify path
TOKEN=$(ssh nanoclaw 'grep ^AUTH_TOKEN= ~/stack/study-app/.env | cut -d= -f2')
rtk proxy curl -s -H "Authorization: Bearer $TOKEN" https://your-host.example/api/courses | grep smoke-course   # expect it listed
# cleanup the smoke course row:
ssh nanoclaw 'sqlite3 ~/stack/study-app/data/study.db "DELETE FROM courses WHERE id=\"smoke-course\"; DELETE FROM course_settings WHERE course_id=\"smoke-course\";"'
```
Expected: `active`; create prints confirmation + `set framing`; settings get shows `framing: verify path`; `/api/courses` lists `smoke-course`; cleanup removes it.

- [ ] **Step 6: Update memory**

Update `claw_study_experience_redesign.md`: flip the "GAP — no working course-creation surface in the default (Pi) path" bullet to SHIPPED, noting `claw-cli course create` + the two wiring edits + deploy.

---

## Self-Review Notes

- **Spec coverage:** Task 1 = CLI command (spec §Design.1, incl. optional settings reusing `SetCourseSetting`). Task 2 = `writeAgentsMD` discovery (spec §Design.2a). Task 3 = `course-study-path` wiring (spec §Design.2b). Task 4 = deploy notes (spec §Deploy). Testing (spec §Testing) is in Task 1 Steps 1–7. Boundary (no plan/memory scaffolding; legacy tool untouched; no UI) is honored — no task touches those.
- **Type consistency:** `courseCreate` / `courseIDRe` / `CreateCourse(id, name)` / `SetCourseSetting(id, "framing"|"exam_style", value)` / `GetCourse(id)` match the real signatures read from `agent/db.go` and `agent/course_settings.go`. Exit codes: 2 = flag/validation errors, 1 = runtime errors — consistent with `courseSettingsSet`.
- **Placeholders:** none — all code and commands are complete.
