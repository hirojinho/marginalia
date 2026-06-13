---
id: 2026-06-13-r9-full-pipeline
title: R9 Full Practice-Testing Pipeline — question generation + answer capture + grading + SM-2
max_wall_clock_minutes: 120
max_diff_lines: 400
max_retries: 1
max_tokens: 60000
requires_visual_approval: false
allow_web_search: false
---

## Goal

Wire the already-shipped R9 infra (`retrieval_probe`, `LogProbe`, `GetProbeQuestion`, `GradeProbeAnswer`) into a complete practice-testing pipeline the agent can drive. Pi generates targeted short-answer questions from Knowledge Component bodies, the learner answers in chat, Pi grades on the SM-2 0–5 scale, and the result feeds the SM-2 scheduler for authentic spaced repetition.

This closes the "biggest gap" (ROADMAP §Practice-testing gap) — replacing the current Rule-6 free-recall with structured, persisted practice testing backed by the strongest evidence in learning science (Roediger & Karpicke 2006).

The spec has 4 tasks. Task 1 adds `claw-cli probe` subcommands so Pi can persist and read probes. Task 2 adds a `GenerateProbeQuestion` LLM function so Pi can delegate question generation to a cheap model (rather than burning Pi context). Task 3 wires the `probe` tool and grading rubric into the AGENTS.md pedagogy block. Task 4 adds test coverage.

The learner-facing experience stays in chat (no new UI components) — Pi asks the question, the learner types the answer, Pi grades and reports. A future spec can add a dedicated probe-answer panel when HTML snippets are available.

---

## Implementation plan

### Task 1 — `claw-cli probe` subcommand (`claw-cli/main.go`)

Add a `probe` case to the top-level switch in `runWithStdin` (alongside `retrieve`, `knowledge`, etc.):

```go
case "probe":
    return runProbe(args[2:], stdout, stderr, dbPath)
```

Then implement `runProbe` with three sub-subcommands:

#### `claw-cli probe store`

Stores a newly-generated question (question-only probe — learner hasn't answered yet).

```
claw-cli probe store --kc <knowledge_component_id> --question "<text>" --expected "<text>"
```

Implementation:
```go
func runProbe(args []string, stdout, stderr io.Writer, dbPath string) int {
    if len(args) < 1 {
        _, _ = fmt.Fprintln(stderr, "usage: claw-cli probe <store|show|record> [args]")
        return 2
    }
    switch args[0] {
    case "store":
        return runProbeStore(args[1:], stdout, stderr, dbPath)
    case "show":
        return runProbeShow(args[1:], stdout, stderr, dbPath)
    case "record":
        return runProbeRecord(args[1:], stdout, stderr, dbPath)
    default:
        _, _ = fmt.Fprintf(stderr, "unknown probe subcommand: %q\n", args[0])
        return 2
    }
}
```

**`runProbeStore`:**
```go
func runProbeStore(args []string, stdout, stderr io.Writer, dbPath string) int {
    fs := flag.NewFlagSet("probe store", flag.ContinueOnError)
    fs.SetOutput(stderr)
    kc := fs.String("kc", "", "knowledge_component_id")
    question := fs.String("question", "", "the generated question")
    expected := fs.String("expected", "", "expected answer (KC body at generation time)")
    dbOverride := fs.String("db", "", "path to study.db")
    if err := fs.Parse(args); err != nil {
        return 2
    }
    if *kc == "" || *question == "" || *expected == "" {
        _, _ = fmt.Fprintln(stderr, "probe store: --kc, --question, and --expected are required")
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
    // sessionID 0: not anchored to a specific session (question generation happens
    // before the learner answers — the answer may come in any session)
    probeID, err := app.LogProbe(*kc, *question, *expected, "", 0, 0)
    if err != nil {
        _, _ = fmt.Fprintf(stderr, "error: %v\n", err)
        return 1
    }
    _, _ = fmt.Fprintf(stdout, `{"probe_id":%d}`, probeID)
    return 0
}
```

Output: `{"probe_id":42}` on stdout (JSON, for Pi to capture the ID).

#### `claw-cli probe show`

Shows details for a probe. If `--kc` is passed, returns the most recent cached question for that KC (so Pi can check if one exists).

```
claw-cli probe show <probe_id>          → single probe by ID
claw-cli probe show --kc <kc_id>        → most recent question for a KC (or "null")
```

Implementation:
```go
func runProbeShow(args []string, stdout, stderr io.Writer, dbPath string) int {
    fs := flag.NewFlagSet("probe show", flag.ContinueOnError)
    fs.SetOutput(stderr)
    kc := fs.String("kc", "", "knowledge_component_id (returns most recent question)")
    dbOverride := fs.String("db", "", "path to study.db")
    if err := fs.Parse(args); err != nil {
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
    if *kc != "" {
        probeID, question, err := app.GetProbeQuestion(*kc)
        if err != nil {
            _, _ = fmt.Fprintf(stderr, "error: %v\n", err)
            return 1
        }
        if probeID == 0 {
            _, _ = fmt.Fprintln(stdout, "null")
            return 0
        }
        _, _ = fmt.Fprintf(stdout, `{"probe_id":%d,"question":%q}`, probeID, question)
        return 0
    }
    // Show single probe by ID (args after flag parsing)
    positional := fs.Args()
    if len(positional) == 0 {
        _, _ = fmt.Fprintln(stderr, "probe show: provide a probe_id or --kc")
        return 2
    }
    probeID, err := strconv.ParseInt(positional[0], 10, 64)
    if err != nil {
        _, _ = fmt.Fprintln(stderr, "probe show: probe_id must be an integer")
        return 2
    }
    // Query the probe row directly
    var question, expected, learnerAnswer string
    var grade sql.NullInt64
    err = app.DB.QueryRow(
        `SELECT question, expected_answer, learner_answer, grade FROM retrieval_probe WHERE id = ?`,
        probeID,
    ).Scan(&question, &expected, &learnerAnswer, &grade)
    if err == sql.ErrNoRows {
        _, _ = fmt.Fprintln(stdout, "null")
        return 0
    }
    if err != nil {
        _, _ = fmt.Fprintf(stderr, "error: %v\n", err)
        return 1
    }
    gradeVal := "null"
    if grade.Valid {
        gradeVal = strconv.Itoa(int(grade.Int64))
    }
    learnerOut := "null"
    if learnerAnswer != "" {
        learnerOut = fmt.Sprintf("%q", learnerAnswer)
    }
    _, _ = fmt.Fprintf(stdout, `{"probe_id":%d,"question":%q,"expected_answer":%q,"learner_answer":%s,"grade":%s}`,
        probeID, question, expected, learnerOut, gradeVal)
    return 0
}
```

#### `claw-cli probe record`

Records a graded answer and feeds SM-2.

```
claw-cli probe record --probe-id <id> --answer "<text>" --grade <0-5>
```

Implementation:
```go
func runProbeRecord(args []string, stdout, stderr io.Writer, dbPath string) int {
    fs := flag.NewFlagSet("probe record", flag.ContinueOnError)
    fs.SetOutput(stderr)
    probeID := fs.Int64("probe-id", 0, "probe ID")
    answer := fs.String("answer", "", "learner's verbatim answer")
    grade := fs.Int("grade", -1, "SM-2 grade 0–5")
    dbOverride := fs.String("db", "", "path to study.db")
    if err := fs.Parse(args); err != nil {
        return 2
    }
    if *probeID == 0 || *answer == "" || *grade < 0 || *grade > 5 {
        _, _ = fmt.Fprintln(stderr, "probe record: --probe-id (>0), --answer, and --grade (0–5) are required")
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
    // Read the existing probe row for kc_id, question, expected_answer
    var kcID, question, expected string
    err = app.DB.QueryRow(
        `SELECT knowledge_component_id, question, expected_answer FROM retrieval_probe WHERE id = ?`,
        *probeID,
    ).Scan(&kcID, &question, &expected)
    if err != nil {
        _, _ = fmt.Fprintf(stderr, "error: probe %d not found: %v\n", *probeID, err)
        return 1
    }
    // LogProbe with answer and grade feeds SM-2
    _, err = app.LogProbe(kcID, question, expected, *answer, *grade, 0)
    if err != nil {
        _, _ = fmt.Fprintf(stderr, "error: %v\n", err)
        return 1
    }
    _, _ = fmt.Fprintln(stdout, `{"status":"recorded"}`)
    return 0
}
```

**Note**: `LogProbe` creates a NEW row when `learnerAnswer != ""`. The question-only row (from `probe store`) is kept as a generation record. The graded row is the authoritative practice-test result that feeds SM-2.

### Task 2 — `GenerateProbeQuestion` in `agent/llm.go`

Add a lightweight LLM function so Pi can delegate question generation to a cheap model rather than burning its own context window. The function prompts the LLM to generate a targeted short-answer question from a KC body.

```go
// GenerateProbeQuestion prompts an LLM to generate a targeted short-answer
// practice-testing question from a Knowledge Component body. The question
// probes ONE specific concept, mechanism, or relationship — not an open-ended
// "recall everything" prompt. Returns the generated question text.
func (c *LLMClient) GenerateProbeQuestion(ctx context.Context, kcBody string) (question string, err error) {
    systemMsg := `You are generating a targeted short-answer practice-testing question from a knowledge component. The question must probe ONE specific concept, mechanism, or relationship — never ask an open-ended "recall everything about X."

Return ONLY the question text. Do not include the answer, commentary, numbering, or markdown formatting.`

    msgs := []Message{
        {Role: "system", Content: systemMsg},
        {Role: "user", Content: "Knowledge component: " + kcBody},
    }
    body := map[string]interface{}{
        "model":      c.Model,
        "stream":     false,
        "max_tokens": 256,
    }
    msgsI := make([]interface{}, 0, len(msgs))
    for _, m := range msgs {
        msgsI = append(msgsI, map[string]string{"role": m.Role, "content": m.Content})
    }
    body["messages"] = msgsI
    payload, err := json.Marshal(body)
    if err != nil {
        return "", fmt.Errorf("marshal request: %w", err)
    }
    req, err := http.NewRequestWithContext(ctx, "POST", c.APIURL+"/chat/completions", bytes.NewReader(payload))
    if err != nil {
        return "", fmt.Errorf("build request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.APIKey)
    httpClient := &http.Client{Timeout: 15 * time.Second}
    resp, err := httpClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        respBody, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("api status %d: %s", resp.StatusCode, string(respBody))
    }
    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("read response: %w", err)
    }
    var result struct {
        Choices []struct {
            Message struct {
                Content string `json:"content"`
            } `json:"message"`
        } `json:"choices"`
    }
    if err := json.Unmarshal(respBody, &result); err != nil {
        return "", fmt.Errorf("unmarshal response: %w", err)
    }
    if len(result.Choices) == 0 {
        return "", fmt.Errorf("no response from LLM")
    }
    return strings.TrimSpace(result.Choices[0].Message.Content), nil
}
```

Add the function signature and import updates as needed.

When this function is unavailable (Pi's bash tool doesn't call Go functions — only `claw-cli`), the AGENTS.md instructions provide the fallback: Pi generates the question itself. The Go function exists for future use (e.g., a server-side probe endpoint or `claw-cli probe generate` v2) but is not wired into `claw-cli` in this spec. Skip tests for this function (no test harness for LLM calls in the existing test suite).

### Task 3 — Update AGENTS.md pedagogy block (`agent/sandbox.go`)

Modify Rule 6 in `studyTuningSections` (around line 232) to include the probe tool loop. The current Rule 6 tells Pi to ask retrieval questions in free-text. Replace it with a structured probe flow:

Replace the entire Rule 6 block (both the interleaving-on and interleaving-off variants) with a version that:
1. Still runs `claw-cli retrieve due` first
2. For each due KC, generates a question using the probe tools
3. Grades the answer using the SM-2 rubric
4. Records the probe

The new Rule 6 for the interleaving-on case:

```go
rule6 := "6. **Session-open retrieval with practice-testing probes (MANDATORY).** Before answering anything else, run `claw-cli retrieve due`. **(a)** If items are due → for each due KC (at most 2):\n" +
    "  (i) Read the KC body: `claw-cli knowledge show <kc_id>`\n" +
    "  (ii) Check for an existing cached question: `claw-cli probe show --kc <kc_id>`. If it returns a cached question, use it — skip generation.\n" +
    "  (iii) If no cached question exists, generate a targeted short-answer question from the KC body. The question must ask for ONE specific concept, mechanism, or relationship — not \"recall everything about X.\" Store it: `claw-cli probe store --kc <kc_id> --question \"<question>\" --expected \"<kc body>\"` (the --expected is the KC body at generation time).\n" +
    "  (iv) Present the question to Eduardo in chat. Ask for his answer.\n" +
    "  (v) When he answers, grade his answer against the expected answer on the SM-2 0–5 scale:\n" +
    "    - 0 = complete blackout — nothing correct or relevant\n" +
    "    - 1 = wrong, but would recognize the correct answer when shown\n" +
    "    - 2 = wrong, but the correct answer seems easy to recall (tip of the tongue)\n" +
    "    - 3 = correct, but with serious difficulty or major gaps\n" +
    "    - 4 = correct, after hesitation or minor gaps\n" +
    "    - 5 = perfect, immediate recall — complete and precise\n" +
    "  (vi) Record the probe: `claw-cli probe record --probe-id <id> --answer \"<his verbatim text>\" --grade <0-5>`\n" +
    "  (vii) Tell him the grade plus a one-sentence justification:\n" +
    "    - Grade 0–2: name what was missed, show the correct answer concisely.\n" +
    "    - Grade 3–4: note the specific gap or hesitation.\n" +
    "    - Grade 5: affirm. Do not linger.\n" +
    "  (viii) If multiple KCs are due, repeat from (i) for the next one.\n" +
    "**(b)** If nothing is due BUT a task has been completed → you MUST still open with 2–3 targeted short-answer questions about the highest-priority concepts from the most recent completed task. Generate questions from your knowledge of the material; do NOT use the probe tools (probes are for KCs only). Score per the SM-2 rubric above; name at most 1–2 missed points. An empty queue is the normal case (SM-2 future-dates fresh items); it does NOT license skipping the recall. **(c)** ONLY if no task is completed at all (a brand-new course, or his very first task) do you skip to the Rule 7 pre-read prediction. Never invent or assume a completed task; never claim he has read, finished, or recalled anything without evidence — if unsure whether he has started, ask.\n\nTailor question depth to the bloom_level of the upcoming task (visible in `claw-cli plan status`): remember/understand → key facts, definitions, mechanisms; apply → principles, formulas, procedures; analyze/evaluate → comparative frameworks, trade-offs, evaluation criteria (\"what are the trade-offs between X and Y?\" not \"what is X?\"); create → skip scored recall, the creation is the retrieval. If bloom_level is missing (older plans), default to understand-level. Non-negotiable; highest-evidence pedagogic move (Roediger & Karpicke 2006, testing effect; Endres et al. 2020, targeted short-answer preserves testing effect).\n"
```

The interleaving-off variant is identical except it appends ` (Interleaving of older tasks is OFF for this course — for branch (b) stay on the most recent.)` at the end of the parenthetical note.

### Task 4 — Test coverage

#### `claw-cli/main_test.go` — probe subcommand tests

Add tests for the three probe subcommands. Follow the existing test pattern (the CLI test file uses a test helper that runs `runProbe` with captured stdout/stderr and a temp DB).

```go
func TestProbeStore(t *testing.T) {
    // Setup: create a KC in a temp DB
    // Run: runProbeStore with valid args
    // Assert: stdout contains {"probe_id":<N>}
    // Assert: stderr is empty
    // Assert: DB has the probe row with correct fields
}

func TestProbeStoreMissingFlags(t *testing.T) {
    // Missing --kc → stderr message, exit 2
    // Missing --question → stderr message, exit 2
    // Missing --expected → stderr message, exit 2
}

func TestProbeShowByKC(t *testing.T) {
    // Setup: store 2 probes for same KC
    // Run: probe show --kc <kc_id>
    // Assert: returns the most recent question
}

func TestProbeShowByKCNone(t *testing.T) {
    // Run: probe show --kc nonexistent
    // Assert: stdout == "null"
}

func TestProbeShowByID(t *testing.T) {
    // Setup: store a probe
    // Run: probe show <probe_id>
    // Assert: returns full probe details
}

func TestProbeRecord(t *testing.T) {
    // Setup: store a probe (question-only)
    // Run: probe record --probe-id <id> --answer "X is Z" --grade 4
    // Assert: stdout == {"status":"recorded"}
    // Assert: DB has a new row with learner_answer + grade set
}

func TestProbeRecordInvalidGrade(t *testing.T) {
    // --grade 6 → stderr message, exit 2
    // --grade -1 → stderr message, exit 2
}

func TestProbeRecordMissingProbe(t *testing.T) {
    // --probe-id 99999 → stderr message, exit 1
}
```

---

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
# Features should NOT exist on current main.
# Exit 1 when features are absent (expected pre-state → gate proceeds).

# 1. No probe subcommand on main.
if grep -q '"probe"' claw-cli/main.go; then
  echo "UNEXPECTED: probe subcommand already in claw-cli"
  exit 0
fi

# 2. No GenerateProbeQuestion on main.
if grep -q 'GenerateProbeQuestion' agent/llm.go; then
  echo "UNEXPECTED: GenerateProbeQuestion already in llm.go"
  exit 0
fi

# 3. Rule 6 does not reference claw-cli probe on main.
if grep -q 'claw-cli probe' agent/sandbox.go; then
  echo "UNEXPECTED: Rule 6 already references claw-cli probe"
  exit 0
fi

exit 1
```

### Post-acceptance (must PASS after implementation)

```bash
# 1. probe subcommand is registered.
grep -q '"probe"' claw-cli/main.go && echo "PASS: probe subcommand"

# 2. probe store exists.
grep -q 'func runProbeStore' claw-cli/main.go && echo "PASS: probe store"

# 3. probe show exists with --kc support.
grep -q 'func runProbeShow' claw-cli/main.go && echo "PASS: probe show"
grep -q 'GetProbeQuestion' claw-cli/main.go && echo "PASS: probe show uses GetProbeQuestion"

# 4. probe record exists.
grep -q 'func runProbeRecord' claw-cli/main.go && echo "PASS: probe record"

# 5. probe record feeds SM-2 (calls LogProbe → UpsertRetrievalItem).
grep -q 'LogProbe' claw-cli/main.go && echo "PASS: probe record calls LogProbe"

# 6. GenerateProbeQuestion exists.
grep -q 'func.*GenerateProbeQuestion' agent/llm.go && echo "PASS: GenerateProbeQuestion"

# 7. Rule 6 references probe tools.
grep -q 'claw-cli probe' agent/sandbox.go && echo "PASS: Rule 6 probe references"

# 8. Build + vet + tests.
go build ./... && echo "PASS: build"
go vet ./... && echo "PASS: vet"
go test ./... -count=1 && echo "PASS: tests"

# 9. No existing test regressions.
go test ./... -count=1 -run 'TestConfidence|TestKnowledge|TestSession|TestCourse|TestPlan|TestLogProbe|TestProbeQuestion|TestRetriev' && echo "PASS: no regressions"
```

### Human-eyeball notes

- `claw-cli probe store --kc K-001 --question "What is X?" --expected "X is Y"` prints `{"probe_id":1}`
- `claw-cli probe show --kc K-001` prints `{"probe_id":1,"question":"What is X?"}`
- `claw-cli probe record --probe-id 1 --answer "X is Z" --grade 3` prints `{"status":"recorded"}`
- `claw-cli probe show 1` (after record) shows the original row unchanged; a new row exists with the answer+grade
- Opening the app (existing features) works unchanged — no new UI, no new endpoints
- The AGENTS.md for a study session shows the new Rule 6 with probe tool instructions

---

## Done criteria

- [ ] `claw-cli probe store` subcommand (stores question-only probe)
- [ ] `claw-cli probe show` subcommand (by ID or by KC)
- [ ] `claw-cli probe record` subcommand (records graded answer, feeds SM-2)
- [ ] `GenerateProbeQuestion` in `agent/llm.go` (future use; not wired to CLI yet)
- [ ] Rule 6 in `agent/sandbox.go` updated with probe tool loop + SM-2 rubric
- [ ] Test coverage: `TestProbeStore`, `TestProbeStoreMissingFlags`, `TestProbeShowByKC`, `TestProbeShowByKCNone`, `TestProbeShowByID`, `TestProbeRecord`, `TestProbeRecordInvalidGrade`, `TestProbeRecordMissingProbe`
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass
- [ ] No existing test regressions
- [ ] Pre-baseline fails on current main; post-acceptance passes on the branch

---

## Edge cases handled

- **KC has no body**: Pi skips probe generation for that KC (no content to test against). Logged as a skip in the retrieval round.
- **Probe store with duplicate question**: `LogProbe` uses `INSERT OR IGNORE` for question-only rows — duplicates are silently ignored and the existing row's ID is returned.
- **Learner declines to answer**: Pi records grade 0 ("complete blackout") and shows the correct answer. The 0 feeds SM-2 (schedules immediate re-review).
- **Pi can't reach the DB** (claw-cli errors): Pi falls back to the old free-recall behavior (Rule 6b) without persistence — same as before this spec. Reports the failure so it can be fixed.
- **Probe exists but KC body has changed since generation**: The `expected_answer` field in `retrieval_probe` is a snapshot from generation time. If the KC body has evolved, the grading may be slightly off — acceptable trade-off. The next probe generation will use the updated body.
- **No KCs exist yet for a course**: `claw-cli retrieve due` returns empty. Pi proceeds to Rule 6b (task-based recall) or Rule 6c (skip to pre-testing). No probe tools called.

---

## Rollback notes

Additive: new CLI subcommand + sandbox prompt update + LLM function. Revert the commit. No schema changes (the `retrieval_probe` table already exists). The `probe` subcommand is the only new write path — if rolled back, the table sits idle until re-added.
