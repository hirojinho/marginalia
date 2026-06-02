# Confidence Persistence on the Pi Path — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the Pi `/chat-v2` agent a `claw-cli confidence log` write path so Rule-3 confidence values finally persist to `confidence_log` (and cascade into `retrieval_queue`).

**Architecture:** One new `claw-cli` subcommand (`confidence log`) that calls the existing `App.LogConfidence` — which already inserts the row *and* upserts `retrieval_queue`. Then rewrite pedagogy Rule 3 in `agent/sandbox.go` to run that command instead of "the log_confidence tool" (a Go tool Pi cannot invoke). No DB code, no schema change, no legacy-path edits.

**Tech Stack:** Go 1.26 (`/opt/homebrew/bin/go`), SQLite, the `flag` package, `study-app/agent` package.

**Spec:** `docs/superpowers/specs/2026-06-02-confidence-persistence-pi-design.md`

---

### Task 1: `claw-cli confidence log` subcommand

**Files:**
- Modify: `claw-cli/main.go` — `runConfidence` switch (around line 1170) + new `confidenceLog` func (add after `confidenceSchema`, ~line 1268)
- Test: `claw-cli/main_test.go`

Reuses `func (a *App) LogConfidence(sessionID int64, knowledgeComponentID string, value float64, source, rawText string) (int64, error)` (`agent/db.go:1110`), which validates `value ∈ [0.0,1.0]`, validates `source`, inserts into `confidence_log`, and upserts `retrieval_queue`. Test helpers already exist in `main_test.go`: `newTempDB(t)` and `run([]string{...}, &stdout, &stderr, dbPath)`.

- [ ] **Step 1: Write the failing tests**

Add to `claw-cli/main_test.go`:

```go
func TestConfidenceLogWritesRowAndQueue(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "confidence", "log",
		"--session", "1",
		"--kc", "task-abc",
		"--value", "0.8",
		"--raw", "pretty solid",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}

	db, err := agent.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	var n int
	var sess int64
	var kc, source string
	var val float64
	row := db.QueryRow(`SELECT count(*) FROM confidence_log`)
	if err := row.Scan(&n); err != nil || n != 1 {
		t.Fatalf("confidence_log rows = %d (err %v), want 1", n, err)
	}
	if err := db.QueryRow(
		`SELECT session_id, knowledge_component_id, value, source FROM confidence_log`,
	).Scan(&sess, &kc, &val, &source); err != nil {
		t.Fatalf("scan row: %v", err)
	}
	if sess != 1 || kc != "task-abc" || val != 0.8 || source != "tool_call" {
		t.Fatalf("row = (%d,%q,%v,%q)", sess, kc, val, source)
	}

	var qn int
	if err := db.QueryRow(
		`SELECT count(*) FROM retrieval_queue WHERE knowledge_component_id = 'task-abc'`,
	).Scan(&qn); err != nil || qn != 1 {
		t.Fatalf("retrieval_queue rows = %d (err %v), want 1", qn, err)
	}
}

func TestConfidenceLogRejectsOutOfRange(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "confidence", "log",
		"--session", "1", "--kc", "task-x", "--value", "1.5",
	}, &stdout, &stderr, dbPath)
	if code == 0 {
		t.Fatalf("expected non-zero exit for out-of-range value")
	}
	db, _ := agent.OpenDB(dbPath)
	defer db.Close()
	var n int
	_ = db.QueryRow(`SELECT count(*) FROM confidence_log`).Scan(&n)
	if n != 0 {
		t.Fatalf("confidence_log rows = %d, want 0 (no write on rejection)", n)
	}
}

func TestConfidenceLogMissingKCExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "confidence", "log",
		"--session", "1", "--value", "0.5",
	}, &stdout, &stderr, dbPath)
	if code != 2 {
		t.Fatalf("exit code: %d, want 2", code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `/opt/homebrew/bin/go test ./claw-cli/ -run TestConfidenceLog -v`
Expected: FAIL — `confidence log` is an unknown subcommand (exit 2 from the existing `default` case), so `TestConfidenceLogWritesRowAndQueue` fails on `code != 0`.

- [ ] **Step 3: Add the dispatch case**

In `claw-cli/main.go`, in `runConfidence` (the switch around line 1170), add a `case` before `default`:

```go
	case "log":
		return confidenceLog(args[1:], stdout, stderr, dbPath)
```

Also update the usage string on the first line of `runConfidence` to include `log`:

```go
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli confidence <log|trajectory|recent|schema> [args]")
```

- [ ] **Step 4: Implement `confidenceLog`**

Add this function in `claw-cli/main.go` after `confidenceSchema` (mirrors `confidenceTrajectory`'s structure):

```go
func confidenceLog(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("confidence log", flag.ContinueOnError)
	fs.SetOutput(stderr)
	session := fs.Int64("session", 0, "session id (required)")
	kc := fs.String("kc", "", "knowledge_component_id — the active plan task's id (required)")
	value := fs.Float64("value", -1, "confidence value in [0.0, 1.0] (required)")
	raw := fs.String("raw", "", "the user's verbatim reply (optional)")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *session <= 0 || *kc == "" || *value < 0 {
		_, _ = fmt.Fprintln(stderr, "confidence log: --session (≥1), --kc, and --value are required")
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
	id, err := app.LogConfidence(*session, *kc, *value, "tool_call", *raw)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "logged confidence %.2f for %s (row %d)\n", *value, *kc, id)
	return 0
}
```

Note: `--value` defaults to `-1` (an impossible confidence) so an omitted flag is caught by the required-flag check and returns exit 2; an out-of-range value like `1.5` passes the required-flag check and is rejected by `LogConfidence`'s `[0.0,1.0]` validation, returning exit 1.

- [ ] **Step 5: Run tests to verify they pass**

Run: `/opt/homebrew/bin/go test ./claw-cli/ -run TestConfidenceLog -v`
Expected: PASS (all three).

- [ ] **Step 6: Full package test + build**

Run: `/opt/homebrew/bin/go test ./... && /opt/homebrew/bin/go build .`
Expected: all green, server binary builds.

- [ ] **Step 7: Commit**

```bash
git add claw-cli/main.go claw-cli/main_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(claw-cli): add 'confidence log' subcommand for the Pi path"
```

---

### Task 2: Rewrite pedagogy Rule 3 to use `claw-cli confidence log`

**Files:**
- Modify: `agent/sandbox.go:243` (Rule 3 string inside `studyTuningSections`)
- Test: `agent/sandbox_test.go` (add a focused test)

The Pi-facing prompt is assembled in `studyTuningSections` (a method on `*SandboxManager`). It is nil-safe on `sm.Settings` (falls back to `DefaultCourseSettings`), so a zero-value `SandboxManager` can be exercised in a test.

- [ ] **Step 1: Write the failing test**

Add to `agent/sandbox_test.go`:

```go
func TestRule3UsesClawCLIConfidenceLog(t *testing.T) {
	var sm SandboxManager
	out := string(sm.studyTuningSections("ddia"))
	if !strings.Contains(out, "claw-cli confidence log") {
		t.Fatalf("Rule 3 must instruct running 'claw-cli confidence log'; got:\n%s", out)
	}
	if strings.Contains(out, "call the log_confidence tool") {
		t.Fatalf("Rule 3 still references the unreachable 'log_confidence tool'")
	}
}
```

(If `agent/sandbox_test.go` does not import `strings`, add it to the import block.)

- [ ] **Step 2: Run test to verify it fails**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestRule3UsesClawCLIConfidenceLog -v`
Expected: FAIL — current Rule 3 says "call the log_confidence tool" and has no `claw-cli confidence log`.

- [ ] **Step 3: Replace the Rule 3 string**

In `agent/sandbox.go`, replace the Rule 3 line (currently line 243):

```go
		"3. **ALWAYS ask \"How confident are you with this?\"** before moving to a new topic. After the user replies, parse a value in [0.0, 1.0] from their answer and call the log_confidence tool with knowledge_component_id = the active task's id field from the plan, value = your parsed value, and raw = their verbatim reply. If no active task is in context, skip the tool call (prompt-only behavior). Low confidence → return to the previous topic; do not advance.\n" +
```

with:

```go
		"3. **ALWAYS ask \"How confident are you with this?\"** before moving to a new topic. **You must elicit an actual number** — if the reply is vague (e.g. \"I think I'm ok\"), ask again for a value in [0.0, 1.0] before advancing. Once you have a number, persist it by running:\n```\nclaw-cli confidence log --session <SESSION_ID> --kc <active task id> --value <0.0-1.0> --raw \"<their verbatim reply>\"\n```\nwhere <SESSION_ID> is the id in the Session section above and <active task id> is the `id` field of the active task from `claw-cli plan status`. If no active task is in context, skip the command (prompt-only). Low confidence → return to the previous topic; do not advance.\n" +
```

- [ ] **Step 4: Run test to verify it passes**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestRule3UsesClawCLIConfidenceLog -v`
Expected: PASS.

- [ ] **Step 5: Full test + build**

Run: `/opt/homebrew/bin/go test ./... && /opt/homebrew/bin/go build .`
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add agent/sandbox.go agent/sandbox_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(pedagogy): Rule 3 persists confidence via claw-cli on the Pi path"
```

---

### Task 3: Build, deploy, and live-verify

**Files:** none (deploy + acceptance). Follow the deploy cheat sheet exactly.

- [ ] **Step 1: Cross-compile both binaries**

```bash
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/claw-cli-linux ./claw-cli
ls -la /tmp/study-app-linux /tmp/claw-cli-linux
```
Expected: two ELF binaries.

- [ ] **Step 2: Deploy server + claw-cli**

```bash
scp /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
scp /tmp/claw-cli-linux nanoclaw:/home/eduardo/stack/study-app/bin/claw-cli.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak && cp claw-cli claw-cli.bak && mv study-app.new study-app && mv claw-cli.new claw-cli && chmod +x study-app claw-cli && export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service'
```

(`/usr/local/bin/claw-cli` is a root-owned symlink → `~/stack/study-app/bin/claw-cli`, so replacing the target updates the command Pi invokes.)

- [ ] **Step 3: Verify service healthy**

```bash
ssh nanoclaw 'export XDG_RUNTIME_DIR=/run/user/$(id -u); systemctl --user is-active study-app.service'
```
Expected: `active`.

- [ ] **Step 4: Smoke-test the subcommand on the VPS directly**

```bash
ssh nanoclaw 'cd ~/stack/study-app && ./bin/claw-cli confidence log --session 56 --kc smoke-test --value 0.5 --raw "manual smoke" --db data/study.db && sqlite3 data/study.db "SELECT session_id,knowledge_component_id,value,source FROM confidence_log WHERE knowledge_component_id=\"smoke-test\";"'
```
Expected: prints `logged confidence 0.50 ...` then the row. Then clean it up:
```bash
ssh nanoclaw 'cd ~/stack/study-app && sqlite3 data/study.db "DELETE FROM confidence_log WHERE knowledge_component_id=\"smoke-test\"; DELETE FROM retrieval_queue WHERE knowledge_component_id=\"smoke-test\";"'
```

- [ ] **Step 5: Live acceptance via a real Pi turn**

In the study app, open a DDIA session, answer a confidence question with a number, then:
```bash
ssh nanoclaw 'cd ~/stack/study-app && sqlite3 -header -column data/study.db "SELECT session_id, knowledge_component_id, value, source, created_at FROM confidence_log ORDER BY id DESC LIMIT 3;"'
```
Expected: ≥1 real row written by the agent (`source=tool_call`) — the table that was empty before this change.

---

## Self-Review

- **Spec coverage:** §1 (new subcommand) → Task 1. §2 (Rule 3 rewrite) → Task 2. §3 (legacy mirrors left untouched) → honored (no edits to `agent.go`/`CLAUDE.local.md`). Testing §s → Tasks 1–2 unit tests + Task 3 live acceptance. Deploy § → Task 3.
- **Placeholders:** none — all code and commands are concrete.
- **Type consistency:** `confidenceLog(args []string, stdout, stderr io.Writer, dbPath string) int` matches the sibling signatures and the `runConfidence` call site; `LogConfidence(int64, string, float64, string, string)` matches `agent/db.go:1110`; `studyTuningSections(string) []byte` matches the call in `writeAgentsMD`.
